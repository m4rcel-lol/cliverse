package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/activitypub"
	"github.com/m4rcel-lol/cliverse/internal/auth"
	"github.com/m4rcel-lol/cliverse/internal/commands"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/federation"
	"github.com/m4rcel-lol/cliverse/internal/models"
	internalssh "github.com/m4rcel-lol/cliverse/internal/ssh"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("starting CLIverse", zap.String("version", version))

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	database, err := db.New(ctx, cfg.DatabaseDSN)
	cancel()
	if err != nil {
		logger.Fatal("connect db", zap.Error(err))
	}
	defer database.Close()

	bootstrapAdmin(cfg, database, logger)

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("parse redis url", zap.Error(err))
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	rateLimiter := auth.NewRateLimiter(redisClient)

	deliverer := activitypub.NewDeliverer(database, logger)
	inboxProcessor := activitypub.NewInboxProcessor(database, cfg, logger, deliverer)
	worker := federation.NewWorker(database, cfg, logger, inboxProcessor)

	dispatch := commands.NewDispatcher(cfg, database, logger, version, startTime)
	shell := internalssh.NewShell(cfg, database, logger, dispatch)
	sshServer := internalssh.New(cfg, database, shell, logger, rateLimiter)

	r := chi.NewRouter()
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(httprate.LimitByIP(100, time.Minute))

	r.Get("/.well-known/webfinger", activitypub.WebFingerHandler(cfg, database))
	r.Get("/.well-known/nodeinfo", activitypub.NodeInfoWellKnownHandler(cfg))
	r.Get("/nodeinfo/2.0", activitypub.NodeInfoHandler(cfg, database))
	r.Get("/users/{username}", activitypub.ActorHandler(cfg, database))
	r.Post("/users/{username}/inbox", activitypub.InboxHandler(cfg, database, logger))
	r.Get("/users/{username}/outbox", activitypub.OutboxHandler(cfg, database))
	r.Get("/users/{username}/followers", activitypub.FollowersHandler(cfg, database))
	r.Get("/users/{username}/following", activitypub.FollowingHandler(cfg, database))

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      r,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go worker.Start(workerCtx)

	go func() {
		logger.Info("starting HTTP server", zap.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server", zap.Error(err))
		}
	}()

	go func() {
		logger.Info("starting SSH server", zap.Int("port", cfg.SSHPort))
		if err := sshServer.Start(); err != nil {
			logger.Fatal("ssh server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	workerCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := httpServer.Shutdown(shutCtx); err != nil {
		logger.Error("http server shutdown", zap.Error(err))
	}
	if err := sshServer.Stop(shutCtx); err != nil {
		logger.Error("ssh server shutdown", zap.Error(err))
	}
}

// bootstrapAdmin creates the initial admin user on a fresh instance if
// ADMIN_PASSWORD_HASH is set and no admin user exists yet.
func bootstrapAdmin(cfg *config.Config, database *db.DB, logger *zap.Logger) {
	if cfg.AdminPasswordHash == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hasAdmin, err := database.HasAdminUser(ctx)
	if err != nil {
		logger.Error("bootstrap admin: check existing admins", zap.Error(err))
		return
	}
	if hasAdmin {
		return
	}

	privKey, pubKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		logger.Error("bootstrap admin: generate key pair", zap.Error(err))
		return
	}

	now := time.Now()
	domain := cfg.Domain
	username := cfg.AdminUsername
	user := &models.User{
		ID:              uuid.New(),
		Username:        username,
		Domain:          domain,
		PasswordHash:    cfg.AdminPasswordHash,
		PrivateKey:      privKey,
		PublicKey:       pubKey,
		APID:            fmt.Sprintf("https://%s/users/%s", domain, username),
		InboxURL:        fmt.Sprintf("https://%s/users/%s/inbox", domain, username),
		OutboxURL:       fmt.Sprintf("https://%s/users/%s/outbox", domain, username),
		IsAdmin:         true,
		ForcePassChange: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := database.CreateUser(ctx, user); err != nil {
		logger.Error("bootstrap admin: create user", zap.Error(err), zap.String("username", username))
		return
	}

	logger.Info("bootstrap: created initial admin user", zap.String("username", username))
}
