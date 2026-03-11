package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
	gossh "golang.org/x/crypto/ssh"
)

const (
	argon2Time    = 3
	argon2Memory  = 65536
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 32

	rateLimitMax     = 10
	rateLimitWindow  = 15 * time.Minute
)

// HashPassword hashes a password using Argon2id and returns the formatted hash string.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads, saltB64, hashB64), nil
}

// VerifyPassword checks a plaintext password against an Argon2id hash string.
func VerifyPassword(password, hash string) (bool, error) {
	parts := strings.Split(hash, "$")
	// parts: ["", "argon2id", "v=19", "m=65536,t=3,p=4", "SALT", "HASH"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads)
	if err != nil {
		return false, fmt.Errorf("parse hash params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}

	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(storedHash)))

	// Constant-time comparison
	if len(computed) != len(storedHash) {
		return false, nil
	}
	var diff byte
	for i := range computed {
		diff |= computed[i] ^ storedHash[i]
	}
	return diff == 0, nil
}

// CalcFingerprint parses a public key string (authorized_keys format) and returns
// its SHA256 fingerprint in the format "SHA256:BASE64...".
func CalcFingerprint(pubKeyStr string) (string, error) {
	pubKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(pubKeyStr))
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}

	h := sha256.Sum256(pubKey.Marshal())
	fp := "SHA256:" + base64.StdEncoding.EncodeToString(h[:])
	return strings.TrimRight(fp, "="), nil
}

// GenerateSessionToken generates 32 random bytes and returns them as a hex string.
func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// RateLimiter uses Redis to track and limit login attempts per key.
type RateLimiter struct {
	redis *redis.Client
}

// NewRateLimiter creates a new RateLimiter backed by the provided Redis client.
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{redis: redisClient}
}

// CheckLoginAttempts returns true if the key is under the rate limit, false if blocked.
func (r *RateLimiter) CheckLoginAttempts(ctx context.Context, key string) (bool, error) {
	redisKey := "login_attempts:" + key
	count, err := r.redis.Get(ctx, redisKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("redis get: %w", err)
	}
	return count < rateLimitMax, nil
}

// RecordLoginAttempt increments the attempt counter for the key, setting a TTL on first use.
func (r *RateLimiter) RecordLoginAttempt(ctx context.Context, key string) error {
	redisKey := "login_attempts:" + key
	pipe := r.redis.Pipeline()
	pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, rateLimitWindow)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline: %w", err)
	}
	return nil
}

// GenerateRSAKeyPair generates a 2048-bit RSA key pair and returns PEM-encoded strings.
func GenerateRSAKeyPair() (privateKeyPEM string, publicKeyPEM string, err error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generate rsa key: %w", err)
	}

	privDER := x509.MarshalPKCS1PrivateKey(privKey)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	privateKeyPEM = string(pem.EncodeToMemory(privBlock))

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal public key: %w", err)
	}
	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	publicKeyPEM = string(pem.EncodeToMemory(pubBlock))

	return privateKeyPEM, publicKeyPEM, nil
}
