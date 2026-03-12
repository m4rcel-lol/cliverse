package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleBlock handles block subcommands.
func HandleBlock(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: block <add|remove|list> [@user]")
	}

	switch ctx.Args[0] {
	case "add":
		return blockAdd(ctx)
	case "remove":
		return blockRemove(ctx)
	case "list":
		return blockList(ctx)
	default:
		return fmt.Errorf("unknown block subcommand: %s. Use: add, remove, list", ctx.Args[0])
	}
}

func blockAdd(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: block add @user[@domain]")
	}

	handle := strings.TrimPrefix(ctx.Args[1], "@")
	parts := strings.SplitN(handle, "@", 2)
	username := parts[0]

	domain := ctx.Config.Domain
	if len(parts) == 2 {
		domain = parts[1]
	}

	if domain != ctx.Config.Domain {
		return fmt.Errorf("blocking remote users is not yet supported; use 'fed block %s' to block their entire domain", domain)
	}

	target, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if target == nil {
		return fmt.Errorf("user not found: @%s", username)
	}
	if target.ID == ctx.User.ID {
		return fmt.Errorf("you cannot block yourself")
	}

	existing, err := ctx.DB.GetBlock(ctx.Ctx, ctx.User.ID, target.ID)
	if err != nil {
		return fmt.Errorf("check existing block: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("you already block @%s", username)
	}

	block := &models.Block{
		ID:        uuid.New(),
		BlockerID: ctx.User.ID,
		BlockedID: target.ID,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateBlock(ctx.Ctx, block); err != nil {
		return fmt.Errorf("create block: %w", err)
	}

	// Also unfollow them if following
	_ = ctx.DB.DeleteFollow(ctx.Ctx, ctx.User.ID, target.ID)
	_ = ctx.DB.DeleteFollow(ctx.Ctx, target.ID, ctx.User.ID)

	fmt.Fprintf(ctx.W, "\033[32m✓ @%s blocked\033[0m\n", username)
	return nil
}

func blockRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: block remove @user")
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

	existing, err := ctx.DB.GetBlock(ctx.Ctx, ctx.User.ID, target.ID)
	if err != nil {
		return fmt.Errorf("check block: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("you are not blocking @%s", username)
	}

	if err := ctx.DB.DeleteBlock(ctx.Ctx, ctx.User.ID, target.ID); err != nil {
		return fmt.Errorf("remove block: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ @%s unblocked\033[0m\n", username)
	return nil
}

func blockList(ctx *Context) error {
	users, err := ctx.DB.ListBlocks(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("list blocks: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mBlocked users (%d):\033[0m\n", len(users))
	if len(users) == 0 {
		fmt.Fprintf(ctx.W, "You have not blocked anyone.\n\n")
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
