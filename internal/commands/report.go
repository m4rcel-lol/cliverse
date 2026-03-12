package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleReport handles report subcommands for users.
func HandleReport(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: report <user|post> TARGET \"reason\"")
	}

	switch ctx.Args[0] {
	case "user":
		return reportUser(ctx)
	case "post":
		return reportPost(ctx)
	default:
		return fmt.Errorf("unknown report subcommand: %s. Use: user, post", ctx.Args[0])
	}
}

func reportUser(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: report user @username \"reason\"")
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
		return fmt.Errorf("you cannot report yourself")
	}

	reason := ctx.Args[2]
	if len(strings.TrimSpace(reason)) == 0 {
		return fmt.Errorf("reason cannot be empty")
	}

	report := &models.Report{
		ID:           uuid.New(),
		ReporterID:   ctx.User.ID,
		TargetUserID: &target.ID,
		Reason:       reason,
		Status:       models.ReportOpen,
		CreatedAt:    time.Now(),
	}
	if err := ctx.DB.CreateReport(ctx.Ctx, report); err != nil {
		return fmt.Errorf("submit report: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Report submitted against @%s. Thank you.\033[0m\n", username)
	return nil
}

func reportPost(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: report post POST_ID \"reason\"")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}
	if post.AuthorID == ctx.User.ID {
		return fmt.Errorf("you cannot report your own post")
	}

	reason := ctx.Args[2]
	if len(strings.TrimSpace(reason)) == 0 {
		return fmt.Errorf("reason cannot be empty")
	}

	report := &models.Report{
		ID:           uuid.New(),
		ReporterID:   ctx.User.ID,
		TargetPostID: &post.ID,
		Reason:       reason,
		Status:       models.ReportOpen,
		CreatedAt:    time.Now(),
	}
	if err := ctx.DB.CreateReport(ctx.Ctx, report); err != nil {
		return fmt.Errorf("submit report: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Report submitted for post [%s]. Thank you.\033[0m\n", ctx.Args[1])
	return nil
}
