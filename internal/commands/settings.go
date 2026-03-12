package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/auth"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleSettings handles settings subcommands.
func HandleSettings(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: settings <update_password|add_key|add_key_url|remove_key|list_keys|sessions|export>")
	}

	switch ctx.Args[0] {
	case "update_password":
		return settingsUpdatePassword(ctx)
	case "add_key":
		return settingsAddKey(ctx)
	case "add_key_url":
		return settingsAddKeyURL(ctx)
	case "remove_key":
		return settingsRemoveKey(ctx)
	case "list_keys":
		return settingsListKeys(ctx)
	case "sessions":
		return settingsSessions(ctx)
	case "export":
		return settingsExport(ctx)
	default:
		return fmt.Errorf("unknown settings subcommand: %s", ctx.Args[0])
	}
}

func settingsUpdatePassword(ctx *Context) error {
	fmt.Fprintf(ctx.W, "Current password: ")
	oldPass, err := readMaskedLine(ctx)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	ok, err := auth.VerifyPassword(oldPass, ctx.User.PasswordHash)
	if err != nil || !ok {
		return fmt.Errorf("\033[31mincorrect current password\033[0m")
	}

	fmt.Fprintf(ctx.W, "New password: ")
	newPass, err := readMaskedLine(ctx)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	if len(newPass) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	fmt.Fprintf(ctx.W, "Confirm new password: ")
	confirmPass, err := readMaskedLine(ctx)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	if newPass != confirmPass {
		return fmt.Errorf("\033[31mpasswords do not match\033[0m")
	}

	hash, err := auth.HashPassword(newPass)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx.User.PasswordHash = hash
	ctx.User.ForcePassChange = false
	if err := ctx.DB.UpdateUser(ctx.Ctx, ctx.User); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Password updated successfully\033[0m\n")
	return nil
}

// readMaskedLine reads a line from ctx.W (SSH session) without echoing characters.
// It writes '*' per character as a best-effort mask and handles backspace.
func readMaskedLine(ctx *Context) (string, error) {
	type readerWriter interface {
		Read([]byte) (int, error)
		Write([]byte) (int, error)
	}
	rw, ok := ctx.W.(readerWriter)
	if !ok {
		return "", fmt.Errorf("output does not support reading")
	}

	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := rw.Read(buf)
		if n == 0 || err != nil {
			break
		}
		b := buf[0]
		if b == '\r' || b == '\n' {
			fmt.Fprintf(ctx.W, "\r\n")
			break
		}
		if b == 0x7f || b == 0x08 { // backspace
			if len(line) > 0 {
				line = line[:len(line)-1]
				fmt.Fprintf(ctx.W, "\b \b")
			}
			continue
		}
		if b == 0x03 { // ctrl-c
			fmt.Fprintf(ctx.W, "\r\n")
			return "", fmt.Errorf("cancelled")
		}
		line = append(line, b)
		fmt.Fprintf(ctx.W, "*")
	}
	return string(line), nil
}

func settingsAddKey(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: settings add_key \"ssh-ed25519 AAAA...\"")
	}

	keyStr := ctx.Args[1]
	if !strings.HasPrefix(keyStr, "ssh-") && !strings.HasPrefix(keyStr, "ecdsa-") {
		return fmt.Errorf("invalid SSH key format: must start with ssh-* or ecdsa-*")
	}

	fp, err := auth.CalcFingerprint(keyStr)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	// Check for duplicates
	existing, err := ctx.DB.GetSSHKeyByFingerprint(ctx.Ctx, fp)
	if err != nil {
		return fmt.Errorf("check key: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("SSH key already registered (fingerprint: %s)", fp)
	}

	key := &models.SSHKey{
		ID:          uuid.New(),
		UserID:      ctx.User.ID,
		Name:        sshKeyName(keyStr, fp),
		PublicKey:   keyStr,
		Fingerprint: fp,
		CreatedAt:   time.Now(),
	}
	if err := ctx.DB.CreateSSHKey(ctx.Ctx, key); err != nil {
		return fmt.Errorf("add key: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ SSH key added\033[0m\n")
	fmt.Fprintf(ctx.W, "  Fingerprint: %s\n", fp)
	return nil
}

func settingsAddKeyURL(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: settings add_key_url URL\n  Example: settings add_key_url ssh.mreow.org/m")
	}

	keyURL := ctx.Args[1]
	keys, err := auth.FetchSSHKeysFromURL(keyURL)
	if err != nil {
		return fmt.Errorf("fetch SSH keys: %w", err)
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

		key := &models.SSHKey{
			ID:          uuid.New(),
			UserID:      ctx.User.ID,
			Name:        sshKeyName(keyStr, fp),
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
		return fmt.Errorf("no new keys imported (all duplicates or invalid)")
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Imported %d SSH key(s) from %s\033[0m\n", imported, keyURL)
	return nil
}

func settingsRemoveKey(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: settings remove_key FINGERPRINT")
	}

	fp := ctx.Args[1]
	key, err := ctx.DB.GetSSHKeyByFingerprint(ctx.Ctx, fp)
	if err != nil {
		return fmt.Errorf("lookup key: %w", err)
	}
	if key == nil || key.UserID != ctx.User.ID {
		return fmt.Errorf("SSH key not found: %s", fp)
	}

	if err := ctx.DB.DeleteSSHKeyByFingerprint(ctx.Ctx, fp, ctx.User.ID); err != nil {
		return fmt.Errorf("remove key: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ SSH key removed\033[0m\n")
	return nil
}

func settingsListKeys(ctx *Context) error {
	keys, err := ctx.DB.ListSSHKeysByUser(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("list keys: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mSSH Keys (%d):\033[0m\n", len(keys))
	if len(keys) == 0 {
		fmt.Fprintf(ctx.W, "  No SSH keys registered. Add one with 'settings add_key'.\n")
		return nil
	}
	for _, k := range keys {
		fmt.Fprintf(ctx.W, "  \033[36m%s\033[0m\n", k.Name)
		fmt.Fprintf(ctx.W, "    Fingerprint: %s\n", k.Fingerprint)
		fmt.Fprintf(ctx.W, "    Added: %s\n", k.CreatedAt.Format("2006-01-02"))
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func settingsSessions(ctx *Context) error {
	sessions, err := ctx.DB.ListActiveSessions(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mActive Sessions (%d):\033[0m\n", len(sessions))
	for _, s := range sessions {
		current := ""
		if s.ID.String() == ctx.SessionID {
			current = " \033[32m[current]\033[0m"
		}
		fmt.Fprintf(ctx.W, "  [%s] %s - started %s, last seen %s%s\n",
			s.ID.String()[:8],
			s.RemoteAddr,
			s.CreatedAt.Format("2006-01-02 15:04"),
			s.LastSeenAt.Format("15:04"),
			current,
		)
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func settingsExport(ctx *Context) error {
	u := ctx.User
	posts, _ := ctx.DB.ListPostsByUser(ctx.Ctx, u.ID, 1000, 0)
	keys, _ := ctx.DB.ListSSHKeysByUser(ctx.Ctx, u.ID)

	fmt.Fprintf(ctx.W, "\033[1mAccount Export for @%s@%s\033[0m\n", u.Username, ctx.Config.Domain)
	fmt.Fprintf(ctx.W, "{\n")
	fmt.Fprintf(ctx.W, "  \"username\": %q,\n", u.Username)
	fmt.Fprintf(ctx.W, "  \"domain\": %q,\n", u.Domain)
	fmt.Fprintf(ctx.W, "  \"display_name\": %q,\n", u.DisplayName)
	fmt.Fprintf(ctx.W, "  \"bio\": %q,\n", u.Bio)
	fmt.Fprintf(ctx.W, "  \"created_at\": %q,\n", u.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(ctx.W, "  \"ssh_keys\": [\n")
	for i, k := range keys {
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}
		fmt.Fprintf(ctx.W, "    {\"name\": %q, \"fingerprint\": %q, \"created_at\": %q}%s\n",
			k.Name, k.Fingerprint, k.CreatedAt.Format(time.RFC3339), comma)
	}
	fmt.Fprintf(ctx.W, "  ],\n")
	fmt.Fprintf(ctx.W, "  \"posts\": [\n")
	for i, p := range posts {
		comma := ","
		if i == len(posts)-1 {
			comma = ""
		}
		fmt.Fprintf(ctx.W, "    {\"local_id\": %q, \"content\": %q, \"visibility\": %q, \"created_at\": %q}%s\n",
			p.LocalID, p.Content, p.Visibility, p.CreatedAt.Format(time.RFC3339), comma)
	}
	fmt.Fprintf(ctx.W, "  ]\n")
	fmt.Fprintf(ctx.W, "}\n")
	return nil
}
