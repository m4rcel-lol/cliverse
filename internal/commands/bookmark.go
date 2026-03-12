package commands

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleBookmark handles bookmark subcommands.
func HandleBookmark(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: bookmark <add|remove|list> [ID]")
	}

	switch ctx.Args[0] {
	case "add":
		return bookmarkAdd(ctx)
	case "remove":
		return bookmarkRemove(ctx)
	case "list":
		return bookmarkList(ctx)
	default:
		return fmt.Errorf("unknown bookmark subcommand: %s", ctx.Args[0])
	}
}

func bookmarkAdd(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: bookmark add ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	bm := &models.Bookmark{
		ID:        uuid.New(),
		UserID:    ctx.User.ID,
		PostID:    post.ID,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateBookmark(ctx.Ctx, bm); err != nil {
		return fmt.Errorf("add bookmark: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Post [%s] bookmarked\033[0m\n", ctx.Args[1])
	return nil
}

func bookmarkRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: bookmark remove ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	if err := ctx.DB.DeleteBookmark(ctx.Ctx, ctx.User.ID, post.ID); err != nil {
		return fmt.Errorf("remove bookmark: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Bookmark removed\033[0m\n")
	return nil
}

func bookmarkList(ctx *Context) error {
	posts, err := ctx.DB.ListBookmarks(ctx.Ctx, ctx.User.ID, 20, 0)
	if err != nil {
		return fmt.Errorf("list bookmarks: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mBookmarks (%d):\033[0m\n", len(posts))
	if len(posts) == 0 {
		fmt.Fprintf(ctx.W, "No bookmarks yet.\n")
		return nil
	}
	for _, p := range posts {
		author, err := ctx.DB.GetUserByID(ctx.Ctx, p.AuthorID)
		if err != nil || author == nil {
			continue
		}
		likes, _ := ctx.DB.CountLikes(ctx.Ctx, p.ID)
		boosts, _ := ctx.DB.CountBoosts(ctx.Ctx, p.ID)
		printPost(ctx, p, author, likes, boosts)
	}
	return nil
}

// HandleFav handles fav (like) subcommands.
func HandleFav(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: fav <add|remove> ID")
	}

	switch ctx.Args[0] {
	case "add":
		return favAdd(ctx)
	case "remove":
		return favRemove(ctx)
	default:
		return fmt.Errorf("unknown fav subcommand: %s", ctx.Args[0])
	}
}

func favAdd(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: fav add ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	existing, err := ctx.DB.GetLike(ctx.Ctx, ctx.User.ID, post.ID)
	if err != nil {
		return fmt.Errorf("check like: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("already liked this post")
	}

	apID := fmt.Sprintf("https://%s/users/%s/likes/%s", ctx.Config.Domain, ctx.User.Username, uuid.New().String())
	like := &models.Like{
		ID:        uuid.New(),
		UserID:    ctx.User.ID,
		PostID:    post.ID,
		APID:      apID,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateLike(ctx.Ctx, like); err != nil {
		return fmt.Errorf("add like: %w", err)
	}

	// Notify post author if different user
	if post.AuthorID != ctx.User.ID {
		notif := &models.Notification{
			ID:        uuid.New(),
			UserID:    post.AuthorID,
			Type:      models.NotifLike,
			ActorID:   &ctx.User.ID,
			PostID:    &post.ID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		_ = ctx.DB.CreateNotification(ctx.Ctx, notif)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Post [%s] liked ♥\033[0m\n", ctx.Args[1])
	return nil
}

func favRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: fav remove ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	if err := ctx.DB.DeleteLike(ctx.Ctx, ctx.User.ID, post.ID); err != nil {
		return fmt.Errorf("remove like: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Like removed\033[0m\n")
	return nil
}

// HandleBoostCmd handles boost subcommands.
func HandleBoostCmd(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: boost <add|remove> ID")
	}

	switch ctx.Args[0] {
	case "add":
		return boostAdd(ctx)
	case "remove":
		return boostRemove(ctx)
	default:
		return fmt.Errorf("unknown boost subcommand: %s", ctx.Args[0])
	}
}

func boostAdd(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: boost add ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	existing, err := ctx.DB.GetBoost(ctx.Ctx, ctx.User.ID, post.ID)
	if err != nil {
		return fmt.Errorf("check boost: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("already boosted this post")
	}

	apID := fmt.Sprintf("https://%s/users/%s/boosts/%s", ctx.Config.Domain, ctx.User.Username, uuid.New().String())
	boost := &models.Boost{
		ID:        uuid.New(),
		UserID:    ctx.User.ID,
		PostID:    post.ID,
		APID:      apID,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateBoost(ctx.Ctx, boost); err != nil {
		return fmt.Errorf("add boost: %w", err)
	}

	// Notify post author if different user
	if post.AuthorID != ctx.User.ID {
		notif := &models.Notification{
			ID:        uuid.New(),
			UserID:    post.AuthorID,
			Type:      models.NotifBoost,
			ActorID:   &ctx.User.ID,
			PostID:    &post.ID,
			Read:      false,
			CreatedAt: time.Now(),
		}
		_ = ctx.DB.CreateNotification(ctx.Ctx, notif)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Post [%s] boosted ↺\033[0m\n", ctx.Args[1])
	return nil
}

func boostRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: boost remove ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	if err := ctx.DB.DeleteBoost(ctx.Ctx, ctx.User.ID, post.ID); err != nil {
		return fmt.Errorf("remove boost: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Boost removed\033[0m\n")
	return nil
}
