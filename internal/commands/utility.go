package commands

import (
	"fmt"
	"time"
)

// HandleInfo shows instance information.
func HandleInfo(ctx *Context) error {
	userCount, _ := ctx.DB.CountLocalUsers(ctx.Ctx, ctx.Config.Domain)
	postCount, _ := ctx.DB.CountLocalPosts(ctx.Ctx)

	fmt.Fprintf(ctx.W, "\033[1m\033[36m── Instance Info ──\033[0m\n")
	fmt.Fprintf(ctx.W, "  Name:    %s\n", ctx.Config.InstanceName)
	fmt.Fprintf(ctx.W, "  Domain:  %s\n", ctx.Config.Domain)
	fmt.Fprintf(ctx.W, "  Desc:    %s\n", ctx.Config.InstanceDesc)
	fmt.Fprintf(ctx.W, "  Version: %s\n", ctx.Dispatcher.Version())
	fmt.Fprintf(ctx.W, "  Users:   %d\n", userCount)
	fmt.Fprintf(ctx.W, "  Posts:   %d\n", postCount)
	fmt.Fprintf(ctx.W, "  Uptime:  %s\n", formatDuration(time.Since(ctx.Dispatcher.StartTime())))
	fmt.Fprintln(ctx.W)
	return nil
}

// HandleUptime shows how long the server has been running.
func HandleUptime(ctx *Context) error {
	uptime := time.Since(ctx.Dispatcher.StartTime())
	fmt.Fprintf(ctx.W, "Server uptime: %s\n", formatDuration(uptime))
	fmt.Fprintf(ctx.W, "Started at:    %s\n", ctx.Dispatcher.StartTime().UTC().Format("2006-01-02 15:04:05 UTC"))
	return nil
}

// HandleVersion displays the build version.
func HandleVersion(ctx *Context) error {
	fmt.Fprintf(ctx.W, "CLIverse %s\n", ctx.Dispatcher.Version())
	return nil
}

// HandleClear sends ANSI clear-screen sequences.
func HandleClear(ctx *Context) error {
	fmt.Fprint(ctx.W, "\033[2J\033[H")
	return nil
}

// HandlePing returns a quick connectivity check.
func HandlePing(ctx *Context) error {
	fmt.Fprintf(ctx.W, "\033[32mpong!\033[0m\n")
	return nil
}

// HandleWhoami shows the current user's handle and role.
func HandleWhoami(ctx *Context) error {
	role := "user"
	if ctx.User.IsAdmin {
		role = "admin"
	}
	fmt.Fprintf(ctx.W, "@%s@%s (%s)\n", ctx.User.Username, ctx.Config.Domain, role)
	return nil
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
