package commands

import (
	"fmt"
)

// HandleSearch handles search subcommands.
func HandleSearch(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: search <users|posts> \"query\"")
	}

	switch ctx.Args[0] {
	case "users":
		return searchUsers(ctx)
	case "posts":
		return searchPosts(ctx)
	default:
		return fmt.Errorf("unknown search subcommand: %s", ctx.Args[0])
	}
}

func searchUsers(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: search users \"query\"")
	}

	query := ctx.Args[1]
	users, err := ctx.DB.SearchUsers(ctx.Ctx, query, 20)
	if err != nil {
		return fmt.Errorf("search users: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mUser search results for %q (%d):\033[0m\n", query, len(users))
	if len(users) == 0 {
		fmt.Fprintf(ctx.W, "No users found.\n\n")
		return nil
	}
	for _, u := range users {
		displayName := u.DisplayName
		if displayName == "" {
			displayName = u.Username
		}
		statusTags := ""
		if u.IsAdmin {
			statusTags += " \033[33m[admin]\033[0m"
		}
		if u.IsLocked {
			statusTags += " \033[31m[suspended]\033[0m"
		}
		fmt.Fprintf(ctx.W, "  \033[36m@%s@%s\033[0m%s - %s\n", u.Username, u.Domain, statusTags, displayName)
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func searchPosts(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: search posts \"query\"")
	}

	query := ctx.Args[1]
	posts, err := ctx.DB.SearchPosts(ctx.Ctx, query, 20)
	if err != nil {
		return fmt.Errorf("search posts: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mPost search results for %q (%d):\033[0m\n", query, len(posts))
	if len(posts) == 0 {
		fmt.Fprintf(ctx.W, "No posts found.\n\n")
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
