package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleMod handles moderation admin commands.
func HandleMod(ctx *Context) error {
	if !ctx.User.IsAdmin {
		return fmt.Errorf("\033[31maccess denied: admin only\033[0m")
	}

	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: mod <suspend|unsuspend|silence|reports|resolve|note> [args...]")
	}

	switch ctx.Args[0] {
	case "suspend":
		return modSetLocked(ctx, true)
	case "unsuspend":
		return modSetLocked(ctx, false)
	case "silence":
		return modSilence(ctx)
	case "reports":
		return modReports(ctx)
	case "resolve":
		return modResolve(ctx)
	case "note":
		return modNote(ctx)
	default:
		return fmt.Errorf("unknown mod subcommand: %s", ctx.Args[0])
	}
}

func modSetLocked(ctx *Context, locked bool) error {
	if len(ctx.Args) < 2 {
		action := "suspend"
		if !locked {
			action = "unsuspend"
		}
		return fmt.Errorf("usage: mod %s USERNAME", action)
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
		return fmt.Errorf("you cannot suspend yourself")
	}

	user.IsLocked = locked
	if err := ctx.DB.UpdateUser(ctx.Ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	action := "suspended"
	if !locked {
		action = "unsuspended"
	}

	// Audit log
	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "mod." + action,
		Target:    username,
		Details:   "{}",
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ User @%s %s\033[0m\n", username, action)
	return nil
}

func modSilence(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: mod silence USERNAME")
	}

	username := ctx.Args[1]
	user, err := ctx.DB.GetUserByUsername(ctx.Ctx, username)
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", username)
	}

	user.IsSilenced = true
	if err := ctx.DB.UpdateUser(ctx.Ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "mod.silence",
		Target:    username,
		Details:   "{}",
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ User @%s silenced\033[0m\n", username)
	return nil
}

func modReports(ctx *Context) error {
	reports, err := ctx.DB.ListReports(ctx.Ctx, models.ReportOpen)
	if err != nil {
		return fmt.Errorf("list reports: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mOpen Reports (%d):\033[0m\n", len(reports))
	if len(reports) == 0 {
		fmt.Fprintf(ctx.W, "No open reports.\n\n")
		return nil
	}
	for _, r := range reports {
		target := "(unknown)"
		if r.TargetUserID != nil {
			u, err := ctx.DB.GetUserByID(ctx.Ctx, *r.TargetUserID)
			if err == nil && u != nil {
				target = "@" + u.Username
			}
		} else if r.TargetPostID != nil {
			target = "post:" + r.TargetPostID.String()[:8]
		}
		fmt.Fprintf(ctx.W, "  \033[36m[%s]\033[0m target=%s reason=%q - %s\n",
			r.ID.String()[:8], target, r.Reason, r.CreatedAt.Format("2006-01-02 15:04"))
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func modResolve(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: mod resolve REPORT_ID")
	}

	prefix := ctx.Args[1]
	reports, err := ctx.DB.ListReports(ctx.Ctx, models.ReportOpen)
	if err != nil {
		return fmt.Errorf("list reports: %w", err)
	}

	var targetID *uuid.UUID
	for _, r := range reports {
		if strings.HasPrefix(r.ID.String(), prefix) {
			id := r.ID
			targetID = &id
			break
		}
	}
	if targetID == nil {
		return fmt.Errorf("report not found: %s", prefix)
	}

	if err := ctx.DB.ResolveReport(ctx.Ctx, *targetID, models.ReportResolved); err != nil {
		return fmt.Errorf("resolve report: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Report resolved\033[0m\n")
	return nil
}

func modNote(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: mod note USERNAME \"note\"")
	}

	username := ctx.Args[1]
	note := ctx.Args[2]

	_ = ctx.DB.CreateAuditLog(ctx.Ctx, &models.AuditLog{
		ID:        uuid.New(),
		ActorID:   &ctx.User.ID,
		Action:    "mod.note",
		Target:    username,
		Details:   fmt.Sprintf(`{"note":%q}`, note),
		CreatedAt: time.Now(),
	})

	fmt.Fprintf(ctx.W, "\033[32m✓ Note added for @%s\033[0m\n", username)
	return nil
}
