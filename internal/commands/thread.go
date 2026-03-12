package commands

import (
	"fmt"
)

// HandleThread handles thread view command.
func HandleThread(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: thread view ID")
	}

	if ctx.Args[0] != "view" {
		return fmt.Errorf("unknown thread subcommand: %s (use 'thread view ID')", ctx.Args[0])
	}
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: thread view ID")
	}

	localID := ctx.Args[1]
	root, err := ctx.DB.GetPostByLocalID(ctx.Ctx, localID)
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if root == nil {
		return fmt.Errorf("post not found: %s", localID)
	}

	posts, err := ctx.DB.GetThread(ctx.Ctx, root.ID)
	if err != nil {
		return fmt.Errorf("fetch thread: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1m── Thread starting from [%s] ──\033[0m\n", localID)
	if len(posts) == 0 {
		fmt.Fprintf(ctx.W, "No posts in thread.\n\n")
		return nil
	}

	for i, p := range posts {
		author, err := ctx.DB.GetUserByID(ctx.Ctx, p.AuthorID)
		if err != nil || author == nil {
			continue
		}
		likes, _ := ctx.DB.CountLikes(ctx.Ctx, p.ID)
		boosts, _ := ctx.DB.CountBoosts(ctx.Ctx, p.ID)

		indent := ""
		if i > 0 {
			indent = "  " // indent replies
		}
		handle := fmt.Sprintf("@%s@%s", author.Username, author.Domain)
		fmt.Fprintf(ctx.W, "%s\033[36m[%s]\033[0m \033[1m%s\033[0m - %s\n",
			indent, p.LocalID, handle, p.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Fprintf(ctx.W, "%s%s\n", indent, p.Content)
		fmt.Fprintf(ctx.W, "%s\033[33mLikes: %d | Boosts: %d\033[0m\n\n", indent, likes, boosts)
	}
	return nil
}
