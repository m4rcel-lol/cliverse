package commands

import (
	"fmt"

	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleTimeline handles timeline subcommands.
func HandleTimeline(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: timeline <home|local|global|mentions>")
	}

	switch ctx.Args[0] {
	case "home":
		return timelineHome(ctx)
	case "local":
		return timelineLocal(ctx)
	case "global":
		return timelineGlobal(ctx)
	case "mentions":
		return timelineMentions(ctx)
	default:
		return fmt.Errorf("unknown timeline subcommand: %s", ctx.Args[0])
	}
}

func timelineHome(ctx *Context) error {
	posts, err := ctx.DB.GetHomeTimeline(ctx.Ctx, ctx.User.ID, 20, 0)
	if err != nil {
		return fmt.Errorf("fetch home timeline: %w", err)
	}
	return renderTimeline(ctx, "Home Timeline", posts)
}

func timelineLocal(ctx *Context) error {
	posts, err := ctx.DB.GetLocalTimeline(ctx.Ctx, 20, 0)
	if err != nil {
		return fmt.Errorf("fetch local timeline: %w", err)
	}
	return renderTimeline(ctx, "Local Timeline", posts)
}

func timelineGlobal(ctx *Context) error {
	posts, err := ctx.DB.GetGlobalTimeline(ctx.Ctx, 20, 0)
	if err != nil {
		return fmt.Errorf("fetch global timeline: %w", err)
	}
	return renderTimeline(ctx, "Federated Timeline", posts)
}

func timelineMentions(ctx *Context) error {
	posts, err := ctx.DB.GetMentionsTimeline(ctx.Ctx, ctx.User.ID, ctx.Config.Domain, 20, 0)
	if err != nil {
		return fmt.Errorf("fetch mentions: %w", err)
	}
	return renderTimeline(ctx, "Mentions", posts)
}

func renderTimeline(ctx *Context, title string, posts []*models.Post) error {
	fmt.Fprintf(ctx.W, "\033[1m── %s ──\033[0m\n", title)
	if len(posts) == 0 {
		fmt.Fprintf(ctx.W, "Nothing here yet.\n\n")
		return nil
	}
	for _, p := range posts {
		author, err := ctx.DB.GetUserByID(ctx.Ctx, p.AuthorID)
		if err != nil {
			continue
		}
		if author == nil {
			continue
		}
		likes, _ := ctx.DB.CountLikes(ctx.Ctx, p.ID)
		boosts, _ := ctx.DB.CountBoosts(ctx.Ctx, p.ID)
		printPost(ctx, p, author, likes, boosts)
	}
	return nil
}
