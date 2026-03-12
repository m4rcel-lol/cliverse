package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleMute handles mute subcommands.
func HandleMute(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: mute <add|remove|list> [@user]")
	}

	switch ctx.Args[0] {
	case "add":
		return muteAdd(ctx)
	case "remove":
		return muteRemove(ctx)
	case "list":
		return muteList(ctx)
	default:
		return fmt.Errorf("unknown mute subcommand: %s. Use: add, remove, list", ctx.Args[0])
	}
}

func muteAdd(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: mute add @user")
	}

	handle := strings.TrimPrefix(ctx.Args[1], "@")
	username := strings.SplitN(handle, "@", 2)[0]

	target, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if target == nil {
		return fmt.Errorf("user not found: @%s", username)
	}
	if target.ID == ctx.User.ID {
		return fmt.Errorf("you cannot mute yourself")
	}

	existing, err := ctx.DB.GetMute(ctx.Ctx, ctx.User.ID, target.ID)
	if err != nil {
		return fmt.Errorf("check existing mute: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("you already mute @%s", username)
	}

	mute := &models.Mute{
		ID:        uuid.New(),
		MuterID:   ctx.User.ID,
		MutedID:   target.ID,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateMute(ctx.Ctx, mute); err != nil {
		return fmt.Errorf("create mute: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ @%s muted (you will no longer see their posts in your timeline)\033[0m\n", username)
	return nil
}

func muteRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: mute remove @user")
	}

	handle := strings.TrimPrefix(ctx.Args[1], "@")
	username := strings.SplitN(handle, "@", 2)[0]

	target, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if target == nil {
		return fmt.Errorf("user not found: @%s", username)
	}

	existing, err := ctx.DB.GetMute(ctx.Ctx, ctx.User.ID, target.ID)
	if err != nil {
		return fmt.Errorf("check mute: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("you are not muting @%s", username)
	}

	if err := ctx.DB.DeleteMute(ctx.Ctx, ctx.User.ID, target.ID); err != nil {
		return fmt.Errorf("remove mute: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ @%s unmuted\033[0m\n", username)
	return nil
}

func muteList(ctx *Context) error {
	users, err := ctx.DB.ListMutes(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("list mutes: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mMuted users (%d):\033[0m\n", len(users))
	if len(users) == 0 {
		fmt.Fprintf(ctx.W, "You have not muted anyone.\n\n")
		return nil
	}
	for _, u := range users {
		displayName := u.DisplayName
		if displayName == "" {
			displayName = u.Username
		}
		fmt.Fprintf(ctx.W, "  \033[36m@%s@%s\033[0m - %s\n", u.Username, u.Domain, displayName)
	}
	fmt.Fprintln(ctx.W)
	return nil
}
