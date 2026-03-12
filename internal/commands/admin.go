package commands

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/auth"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// validUsername matches alphanumeric characters and underscores only.
var validUsername = regexp.MustCompile(`^[a-z0-9_]+$`)

const randomPasswordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

// HandleAdmin handles admin subcommands.
func HandleAdmin(ctx *Context) error {
	if !ctx.User.IsAdmin {
		return fmt.Errorf("\033[31maccess denied: admin only\033[0m")
	}

	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: admin <create_user|delete_user|reset_password|add_key|add_key_url|list_users|health|stats|broadcast|maintenance|logs>")
	}

	switch ctx.Args[0] {
	case "create_user":
		return adminCreateUser(ctx)
	case "delete_user":
		return adminDeleteUser(ctx)
	case "reset_password":
		return adminResetPassword(ctx)
	case "add_key":
		return adminAddKey(ctx)
	case "add_key_url":
		return adminAddKeyURL(ctx)
	case "list_users":
		return adminListUsers(ctx)
	case "health":
		return adminHealth(ctx)
	case "stats":
		return adminStats(ctx)
	case "broadcast":
		return adminBroadcast(ctx)
	case "maintenance":
		return adminMaintenance(ctx)
	case "logs":
		return adminLogs(ctx)
	default:
		return fmt.Errorf("unknown admin subcommand: %s", ctx.Args[0])
	}
}

func adminCreateUser(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: admin create_user USERNAME [SSH_KEY_URL]")
	}

	username := strings.ToLower(ctx.Args[1])
	if len(username) < 2 || len(username) > 30 {
		return fmt.Errorf("username must be 2-30 characters")
	}
	if !validUsername.MatchString(username) {
		return fmt.Errorf("username may only contain lowercase letters, numbers, and underscores")
	}

	existing, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("check username: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("username already taken: %s", username)
	}

	password, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	privKey, pubKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}

	now := time.Now()
	domain := ctx.Config.Domain
	user := &models.User{
		ID:              uuid.New(),
		Username:        username,
		Domain:          domain,
		PasswordHash:    hash,
		PrivateKey:      privKey,
		PublicKey:       pubKey,
		APID:            fmt.Sprintf("https://%s/users/%s", domain, username),
		InboxURL:        fmt.Sprintf("https://%s/users/%s/inbox", domain, username),
		OutboxURL:       fmt.Sprintf("https://%s/users/%s/outbox", domain, username),
		ForcePassChange: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := ctx.DB.CreateUser(ctx.Ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "admin.create_user",
		Target:    username,
		Details:   "{}",
		CreatedAt: now,
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ User @%s created\033[0m\n", username)
	fmt.Fprintf(ctx.W, "  Temporary password: \033[33m%s\033[0m\n", password)
	fmt.Fprintf(ctx.W, "  \033[33m(User must change password on first login)\033[0m\n")

	// If an SSH key URL was provided, fetch and import keys.
	if len(ctx.Args) >= 3 {
		keyURL := ctx.Args[2]
		imported, err := importSSHKeysFromURL(ctx, user, keyURL)
		if err != nil {
			fmt.Fprintf(ctx.W, "  \033[33m⚠ Could not import SSH keys from URL: %s\033[0m\n", err)
		} else {
			fmt.Fprintf(ctx.W, "  \033[32m✓ Imported %d SSH key(s) from %s\033[0m\n", imported, keyURL)
		}
	}

	return nil
}

func adminDeleteUser(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: admin delete_user USERNAME")
	}

	username := ctx.Args[1]
	user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}
	if user.ID == ctx.User.ID {
		return fmt.Errorf("you cannot delete your own account")
	}

	if err := ctx.DB.DeleteUser(ctx.Ctx, user.ID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "admin.delete_user",
		Target:    username,
		Details:   "{}",
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ User @%s deleted\033[0m\n", username)
	return nil
}

func adminResetPassword(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: admin reset_password USERNAME")
	}

	username := ctx.Args[1]
	user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}

	password, err := generateRandomPassword(16)
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user.PasswordHash = hash
	user.ForcePassChange = true
	if err := ctx.DB.UpdateUser(ctx.Ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "admin.reset_password",
		Target:    username,
		Details:   "{}",
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ Password reset for @%s\033[0m\n", username)
	fmt.Fprintf(ctx.W, "  New temporary password: \033[33m%s\033[0m\n", password)
	return nil
}

func adminAddKey(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: admin add_key USERNAME \"ssh-ed25519 AAAA...\"")
	}

	username := ctx.Args[1]
	keyStr := ctx.Args[2]

	user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}

	fp, err := auth.CalcFingerprint(keyStr)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	// Check for duplicate key.
	existing, err := ctx.DB.GetSSHKeyByFingerprint(ctx.Ctx, fp)
	if err != nil {
		return fmt.Errorf("check key: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("SSH key already registered (fingerprint: %s)", fp)
	}

	parts := strings.Fields(keyStr)
	name := fp
	if len(parts) > 0 {
		name = parts[0]
		if len(fp) > 8 {
			name += " " + fp[len(fp)-8:]
		}
	}

	key := &models.SSHKey{
		ID:          uuid.New(),
		UserID:      user.ID,
		Name:        name,
		PublicKey:   keyStr,
		Fingerprint: fp,
		CreatedAt:   time.Now(),
	}
	if err := ctx.DB.CreateSSHKey(ctx.Ctx, key); err != nil {
		return fmt.Errorf("add key: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ SSH key added for @%s\033[0m\n", username)
	fmt.Fprintf(ctx.W, "  Fingerprint: %s\n", fp)
	return nil
}

func adminAddKeyURL(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: admin add_key_url USERNAME URL\n  Example: admin add_key_url alice ssh.mreow.org/m")
	}

	username := ctx.Args[1]
	keyURL := ctx.Args[2]

	user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}

	imported, err := importSSHKeysFromURL(ctx, user, keyURL)
	if err != nil {
		return fmt.Errorf("import SSH keys: %w", err)
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "admin.add_key_url",
		Target:    username,
		Details:   fmt.Sprintf(`{"url":%q,"imported":%d}`, keyURL, imported),
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ Imported %d SSH key(s) for @%s from %s\033[0m\n", imported, username, keyURL)
	return nil
}

// importSSHKeysFromURL fetches SSH public keys from the given URL and adds
// them to the user's account, skipping any keys that are already registered.
func importSSHKeysFromURL(ctx *Context, user *models.User, rawURL string) (int, error) {
	keys, err := auth.FetchSSHKeysFromURL(rawURL)
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, keyStr := range keys {
		fp, err := auth.CalcFingerprint(keyStr)
		if err != nil {
			continue
		}

		// Skip duplicates.
		existing, err := ctx.DB.GetSSHKeyByFingerprint(ctx.Ctx, fp)
		if err != nil {
			continue
		}
		if existing != nil {
			fmt.Fprintf(ctx.W, "  \033[33mSkipped duplicate key: %s\033[0m\n", fp)
			continue
		}

		parts := strings.Fields(keyStr)
		name := fp
		if len(parts) > 0 {
			name = parts[0]
			if len(fp) > 8 {
				name += " " + fp[len(fp)-8:]
			}
		}

		key := &models.SSHKey{
			ID:          uuid.New(),
			UserID:      user.ID,
			Name:        name,
			PublicKey:   keyStr,
			Fingerprint: fp,
			CreatedAt:   time.Now(),
		}
		if err := ctx.DB.CreateSSHKey(ctx.Ctx, key); err != nil {
			continue
		}
		fmt.Fprintf(ctx.W, "  Key added: %s\n", fp)
		imported++
	}

	if imported == 0 {
		return 0, fmt.Errorf("no new keys imported (all duplicates or invalid)")
	}
	return imported, nil
}

func adminListUsers(ctx *Context) error {
	users, err := ctx.DB.ListUsers(ctx.Ctx, 100, 0)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mAll Users (%d):\033[0m\n", len(users))
	for _, u := range users {
		status := "\033[32m●\033[0m"
		if u.IsLocked {
			status = "\033[31m✗\033[0m"
		} else if u.IsSilenced {
			status = "\033[33m~\033[0m"
		}
		adminTag := ""
		if u.IsAdmin {
			adminTag = " \033[33m[admin]\033[0m"
		}
		fmt.Fprintf(ctx.W, "  %s @%s%s - joined %s\n",
			status, u.Username, adminTag, u.CreatedAt.Format("2006-01-02"))
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func adminHealth(ctx *Context) error {
	fmt.Fprintf(ctx.W, "\033[1mInstance Health Check\033[0m\n")

	// DB ping via GetSystemConfig as a lightweight check
	_, err := ctx.DB.GetSystemConfig(ctx.Ctx, "health_check")
	if err != nil {
		fmt.Fprintf(ctx.W, "  Database: \033[31m✗ error: %v\033[0m\n", err)
	} else {
		fmt.Fprintf(ctx.W, "  Database: \033[32m✓ OK\033[0m\n")
	}

	fmt.Fprintln(ctx.W)
	return nil
}

func adminStats(ctx *Context) error {
	users, _ := ctx.DB.ListUsers(ctx.Ctx, 10000, 0)
	posts, _ := ctx.DB.GetGlobalTimeline(ctx.Ctx, 10000, 0)
	policies, _ := ctx.DB.ListDomainPolicies(ctx.Ctx)

	fmt.Fprintf(ctx.W, "\033[1mInstance Stats — %s\033[0m\n", ctx.Config.InstanceName)
	fmt.Fprintf(ctx.W, "  Domain:   %s\n", ctx.Config.Domain)
	fmt.Fprintf(ctx.W, "  Users:    %d\n", len(users))
	fmt.Fprintf(ctx.W, "  Posts:    %d\n", len(posts))
	fmt.Fprintf(ctx.W, "  Policies: %d\n", len(policies))
	fmt.Fprintln(ctx.W)
	return nil
}

func adminBroadcast(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: admin broadcast \"message\"")
	}

	message := ctx.Args[1]
	users, err := ctx.DB.ListUsers(ctx.Ctx, 10000, 0)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	count := 0
	for _, u := range users {
		if u.ID == ctx.User.ID {
			continue
		}
		notif := &models.Notification{
			ID:        uuid.New(),
			UserID:    u.ID,
			Type:      "admin_broadcast",
			ActorID:   &ctx.User.ID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		_ = ctx.DB.CreateNotification(ctx.Ctx, notif)
		count++
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "admin.broadcast",
		Target:    "all",
		Details:   fmt.Sprintf(`{"message":%q}`, message),
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ Broadcast sent to %d users\033[0m\n", count)
	return nil
}

func adminMaintenance(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: admin maintenance <on|off>")
	}

	value := "false"
	label := "off"
	if ctx.Args[1] == "on" {
		value = "true"
		label = "on"
	}

	if err := ctx.DB.SetSystemConfig(ctx.Ctx, "maintenance_mode", value); err != nil {
		return fmt.Errorf("set maintenance mode: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Maintenance mode: %s\033[0m\n", label)
	return nil
}

func adminLogs(ctx *Context) error {
	logs, err := ctx.DB.ListAuditLogs(ctx.Ctx, 50)
	if err != nil {
		return fmt.Errorf("list audit logs: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mAudit Logs (last %d):\033[0m\n", len(logs))
	for _, l := range logs {
		actor := "(system)"
		if l.ActorID != nil {
			u, err := ctx.DB.GetUserByID(ctx.Ctx, *l.ActorID)
			if err == nil && u != nil {
				actor = "@" + u.Username
			}
		}
		target := l.Target
		if target == "" {
			target = "-"
		}
		fmt.Fprintf(ctx.W, "  \033[36m%s\033[0m \033[1m%s\033[0m → %s %s\n",
			l.CreatedAt.Format("2006-01-02 15:04"), actor, l.Action, target)
	}
	if len(logs) == 0 {
		fmt.Fprintf(ctx.W, "No audit logs.\n")
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func generateRandomPassword(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(randomPasswordChars))))
		if err != nil {
			return "", err
		}
		b[i] = randomPasswordChars[n.Int64()]
	}
	return string(b), nil
}
