package commands

import (
	"fmt"
	"strings"
)

// HandleProfile handles profile subcommands.
func HandleProfile(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return profileShow(ctx, "")
	}

	switch ctx.Args[0] {
	case "show":
		target := ""
		if len(ctx.Args) > 1 {
			target = ctx.Args[1]
		}
		return profileShow(ctx, target)
	case "set":
		return profileSet(ctx)
	default:
		return fmt.Errorf("unknown profile subcommand: %s", ctx.Args[0])
	}
}

func profileShow(ctx *Context, target string) error {
	if target == "" {
		return renderProfile(ctx, ctx.User.Username, ctx.Config.Domain)
	}

	// Parse @user or @user@domain
	handle := strings.TrimPrefix(target, "@")
	parts := strings.SplitN(handle, "@", 2)
	username := parts[0]
	domain := ctx.Config.Domain
	if len(parts) == 2 {
		domain = parts[1]
	}

	return renderProfile(ctx, username, domain)
}

func renderProfile(ctx *Context, username, domain string) error {
	if domain == ctx.Config.Domain {
		user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
		if err != nil {
			return fmt.Errorf("lookup user: %w", err)
		}
		if user == nil {
			return fmt.Errorf("user not found: @%s", username)
		}

		followers, err := ctx.DB.ListFollowers(ctx.Ctx, user.ID)
		if err != nil {
			return fmt.Errorf("fetch followers: %w", err)
		}
		following, err := ctx.DB.ListFollowing(ctx.Ctx, user.ID)
		if err != nil {
			return fmt.Errorf("fetch following: %w", err)
		}
		posts, err := ctx.DB.ListPostsByUser(ctx.Ctx, user.ID, 1000, 0)
		if err != nil {
			return fmt.Errorf("count posts: %w", err)
		}

		adminBadge := ""
		if user.IsAdmin {
			adminBadge = " \033[33m[admin]\033[0m"
		}
		lockedBadge := ""
		if user.IsLocked {
			lockedBadge = " \033[31m[suspended]\033[0m"
		}

		displayName := user.DisplayName
		if displayName == "" {
			displayName = user.Username
		}

		fmt.Fprintf(ctx.W, "\033[1m@%s@%s\033[0m%s%s\n", user.Username, user.Domain, adminBadge, lockedBadge)
		fmt.Fprintf(ctx.W, "Display Name: %s\n", displayName)
		fmt.Fprintf(ctx.W, "Bio: %s\n", user.Bio)
		if user.AvatarURL != "" {
			fmt.Fprintf(ctx.W, "Avatar: %s\n", user.AvatarURL)
		}
		if user.BannerURL != "" {
			fmt.Fprintf(ctx.W, "Banner: %s\n", user.BannerURL)
		}
		fmt.Fprintf(ctx.W, "Created: %s\n", user.CreatedAt.Format("2006-01-02"))
		fmt.Fprintf(ctx.W, "\033[36mFollowers: %d | Following: %d\033[0m\n", len(followers), len(following))
		fmt.Fprintf(ctx.W, "Posts: %d\n\n", len(posts))
		return nil
	}

	// Remote user
	actor, err := ctx.DB.GetRemoteActorByUsernameAndDomain(ctx.Ctx, username, domain)
	if err != nil {
		return fmt.Errorf("lookup remote actor: %w", err)
	}
	if actor == nil {
		return fmt.Errorf("remote user not found: @%s@%s (try 'user lookup @%s@%s')", username, domain, username, domain)
	}

	displayName := actor.DisplayName
	if displayName == "" {
		displayName = actor.Username
	}

	fmt.Fprintf(ctx.W, "\033[1m@%s@%s\033[0m \033[33m[remote]\033[0m\n", actor.Username, actor.Domain)
	fmt.Fprintf(ctx.W, "Display Name: %s\n", displayName)
	fmt.Fprintf(ctx.W, "Bio: %s\n", actor.Bio)
	if actor.AvatarURL != "" {
		fmt.Fprintf(ctx.W, "Avatar: %s\n", actor.AvatarURL)
	}
	fmt.Fprintf(ctx.W, "AP ID: %s\n\n", actor.APID)
	return nil
}

func profileSet(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: profile set <display_name|bio|avatar_url|banner_url> \"value\"")
	}

	field := ctx.Args[1]
	value := ctx.Args[2]

	user := ctx.User
	switch field {
	case "display_name":
		user.DisplayName = value
	case "bio":
		user.Bio = value
	case "avatar_url":
		user.AvatarURL = value
	case "banner_url":
		user.BannerURL = value
	default:
		return fmt.Errorf("unknown field: %s (valid: display_name, bio, avatar_url, banner_url)", field)
	}

	if err := ctx.DB.UpdateUser(ctx.Ctx, user); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Profile updated: %s = %s\033[0m\n", field, value)
	return nil
}
