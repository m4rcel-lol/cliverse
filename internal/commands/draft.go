package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleDraft handles draft subcommands.
func HandleDraft(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: draft <new|list|post|delete> [args...]")
	}

	switch ctx.Args[0] {
	case "new":
		return draftNew(ctx)
	case "list":
		return draftList(ctx)
	case "post":
		return draftPost(ctx)
	case "delete":
		return draftDelete(ctx)
	default:
		return fmt.Errorf("unknown draft subcommand: %s", ctx.Args[0])
	}
}

func draftNew(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: draft new \"content\"")
	}

	content := ctx.Args[1]
	if len(content) == 0 {
		return fmt.Errorf("draft content cannot be empty")
	}

	now := time.Now()
	draft := &models.Draft{
		ID:         uuid.New(),
		UserID:     ctx.User.ID,
		Content:    content,
		Visibility: models.VisibilityPublic,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := ctx.DB.CreateDraft(ctx.Ctx, draft); err != nil {
		return fmt.Errorf("save draft: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Draft saved [%s]\033[0m\n", draft.ID.String()[:8])
	return nil
}

func draftList(ctx *Context) error {
	drafts, err := ctx.DB.ListDrafts(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return fmt.Errorf("list drafts: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mDrafts (%d):\033[0m\n", len(drafts))
	if len(drafts) == 0 {
		fmt.Fprintf(ctx.W, "No drafts. Create one with 'draft new \"content\"'.\n")
		return nil
	}
	for _, d := range drafts {
		preview := d.Content
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		fmt.Fprintf(ctx.W, "  \033[36m[%s]\033[0m %s - %s\n",
			d.ID.String()[:8], preview, d.UpdatedAt.Format("2006-01-02 15:04"))
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func draftPost(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: draft post ID")
	}

	draft, err := findDraft(ctx, ctx.Args[1])
	if err != nil {
		return err
	}

	// Create the post from draft content
	localID := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	apID := fmt.Sprintf("https://%s/users/%s/posts/%s", ctx.Config.Domain, ctx.User.Username, localID)
	now := time.Now()
	post := &models.Post{
		ID:            uuid.New(),
		LocalID:       localID,
		AuthorID:      ctx.User.ID,
		Content:       draft.Content,
		Visibility:    draft.Visibility,
		ActivityPubID: apID,
		Deleted:       false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := ctx.DB.CreatePost(ctx.Ctx, post); err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	if err := ctx.DB.DeleteDraft(ctx.Ctx, draft.ID, ctx.User.ID); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Draft posted as [%s]\033[0m\n", localID)
	return nil
}

func draftDelete(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: draft delete ID")
	}

	draft, err := findDraft(ctx, ctx.Args[1])
	if err != nil {
		return err
	}

	if err := ctx.DB.DeleteDraft(ctx.Ctx, draft.ID, ctx.User.ID); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Draft [%s] deleted\033[0m\n", ctx.Args[1])
	return nil
}

func findDraft(ctx *Context, prefix string) (*models.Draft, error) {
	drafts, err := ctx.DB.ListDrafts(ctx.Ctx, ctx.User.ID)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	for _, d := range drafts {
		if strings.HasPrefix(d.ID.String(), prefix) {
			return d, nil
		}
	}
	return nil, fmt.Errorf("draft not found: %s", prefix)
}
