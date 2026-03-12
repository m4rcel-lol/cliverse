package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandlePost handles all post subcommands.
func HandlePost(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: post <global|local|reply|delete|show|list> [args...]")
	}

	switch ctx.Args[0] {
	case "global":
		return postCreate(ctx, models.VisibilityPublic, false)
	case "local":
		return postCreate(ctx, models.VisibilityPublic, true)
	case "reply":
		return postReply(ctx)
	case "delete":
		return postDelete(ctx)
	case "show":
		return postShow(ctx)
	case "list":
		return postList(ctx)
	default:
		return fmt.Errorf("unknown post subcommand: %s", ctx.Args[0])
	}
}

func postCreate(ctx *Context, visibility string, localOnly bool) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: post %s \"message\"", ctx.Args[0])
	}

	content := ctx.Args[1]
	if len(content) == 0 {
		return fmt.Errorf("post content cannot be empty")
	}
	if len(content) > ctx.Config.MaxPostLength {
		return fmt.Errorf("post too long: %d chars (max %d)", len(content), ctx.Config.MaxPostLength)
	}

	localID := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	apID := fmt.Sprintf("https://%s/users/%s/posts/%s", ctx.Config.Domain, ctx.User.Username, localID)
	if localOnly {
		apID = "" // No federation for local-only posts
	}

	now := time.Now()
	post := &models.Post{
		ID:            uuid.New(),
		LocalID:       localID,
		AuthorID:      ctx.User.ID,
		Content:       content,
		Visibility:    visibility,
		ActivityPubID: apID,
		Deleted:       false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := ctx.DB.CreatePost(ctx.Ctx, post); err != nil {
		return fmt.Errorf("create post: %w", err)
	}

	scope := "global"
	if localOnly {
		scope = "local-only"
	}
	fmt.Fprintf(ctx.W, "\033[32m✓ Posted [%s] (%s)\033[0m\n", localID, scope)
	return nil
}

func postReply(ctx *Context) error {
	if len(ctx.Args) < 3 {
		return fmt.Errorf("usage: post reply ID \"message\"")
	}

	parentLocalID := ctx.Args[1]
	content := ctx.Args[2]

	parent, err := ctx.DB.GetPostByLocalID(ctx.Ctx, parentLocalID)
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if parent == nil {
		return fmt.Errorf("post not found: %s", parentLocalID)
	}

	if len(content) > ctx.Config.MaxPostLength {
		return fmt.Errorf("post too long: %d chars (max %d)", len(content), ctx.Config.MaxPostLength)
	}

	localID := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	apID := fmt.Sprintf("https://%s/users/%s/posts/%s", ctx.Config.Domain, ctx.User.Username, localID)

	now := time.Now()
	post := &models.Post{
		ID:            uuid.New(),
		LocalID:       localID,
		AuthorID:      ctx.User.ID,
		Content:       content,
		Visibility:    models.VisibilityPublic,
		InReplyToID:   &parent.ID,
		ActivityPubID: apID,
		Deleted:       false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := ctx.DB.CreatePost(ctx.Ctx, post); err != nil {
		return fmt.Errorf("create reply: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Reply posted [%s] → [%s]\033[0m\n", localID, parentLocalID)
	return nil
}

func postDelete(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: post delete ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}
	if post.AuthorID != ctx.User.ID && !ctx.User.IsAdmin {
		return fmt.Errorf("you can only delete your own posts")
	}

	authorID := post.AuthorID
	if err := ctx.DB.DeletePost(ctx.Ctx, post.ID, authorID); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Post [%s] deleted\033[0m\n", ctx.Args[1])
	return nil
}

func postShow(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: post show ID")
	}

	post, err := ctx.DB.GetPostByLocalID(ctx.Ctx, ctx.Args[1])
	if err != nil {
		return fmt.Errorf("lookup post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("post not found: %s", ctx.Args[1])
	}

	author, err := ctx.DB.GetUserByID(ctx.Ctx, post.AuthorID)
	if err != nil || author == nil {
		return fmt.Errorf("lookup author: %w", err)
	}

	likes, _ := ctx.DB.CountLikes(ctx.Ctx, post.ID)
	boosts, _ := ctx.DB.CountBoosts(ctx.Ctx, post.ID)

	printPost(ctx, post, author, likes, boosts)
	return nil
}

func postList(ctx *Context) error {
	posts, err := ctx.DB.ListPostsByUser(ctx.Ctx, ctx.User.ID, 20, 0)
	if err != nil {
		return fmt.Errorf("list posts: %w", err)
	}

	if len(posts) == 0 {
		fmt.Fprintf(ctx.W, "No posts yet. Use 'post global \"message\"' to create one.\n")
		return nil
	}

	fmt.Fprintf(ctx.W, "\033[1mYour recent posts (%d):\033[0m\n", len(posts))
	for _, p := range posts {
		likes, _ := ctx.DB.CountLikes(ctx.Ctx, p.ID)
		boosts, _ := ctx.DB.CountBoosts(ctx.Ctx, p.ID)
		printPost(ctx, p, ctx.User, likes, boosts)
	}
	return nil
}

// printPost formats and writes a single post to the writer.
func printPost(ctx *Context, p *models.Post, author *models.User, likes, boosts int) {
	handle := fmt.Sprintf("@%s@%s", author.Username, author.Domain)
	fmt.Fprintf(ctx.W, "\033[36m[%s]\033[0m \033[1m%s\033[0m - %s\n",
		p.LocalID, handle, p.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(ctx.W, "%s\n", p.Content)
	fmt.Fprintf(ctx.W, "\033[33mLikes: %d | Boosts: %d\033[0m\n\n", likes, boosts)
}
