package commands

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// HandleNotif handles notification subcommands.
func HandleNotif(ctx *Context) error {
	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: notif <list|read ID|clear>")
	}

	switch ctx.Args[0] {
	case "list":
		return notifList(ctx)
	case "read":
		return notifRead(ctx)
	case "clear":
		return notifClear(ctx)
	default:
		return fmt.Errorf("unknown notif subcommand: %s", ctx.Args[0])
	}
}

func notifList(ctx *Context) error {
	notifs, err := ctx.DB.ListNotifications(ctx.Ctx, ctx.User.ID, 20, 0)
	if err != nil {
		return fmt.Errorf("list notifications: %w", err)
	}

	unread, _ := ctx.DB.CountUnreadNotifications(ctx.Ctx, ctx.User.ID)

	fmt.Fprintf(ctx.W, "\033[1m── Notifications ──\033[0m  \033[33m%d unread\033[0m\n", unread)

	if len(notifs) == 0 {
		fmt.Fprintf(ctx.W, "No notifications.\n")
		return nil
	}

	for _, n := range notifs {
		prefix := n.ID.String()[:8]
		unreadTag := ""
		if !n.Read {
			unreadTag = " \033[33m[UNREAD]\033[0m"
		}

		actorHandle := "unknown"
		if n.ActorID != nil {
			actor, err := ctx.DB.GetUserByID(ctx.Ctx, *n.ActorID)
			if err == nil && actor != nil {
				actorHandle = fmt.Sprintf("@%s@%s", actor.Username, actor.Domain)
			}
		}

		var detail string
		switch n.Type {
		case "follow":
			detail = fmt.Sprintf("%s followed you", actorHandle)
		case "mention":
			postRef := ""
			if n.PostID != nil {
				postRef = " in a post"
			}
			detail = fmt.Sprintf("%s mentioned you%s", actorHandle, postRef)
		case "like":
			detail = fmt.Sprintf("%s liked your post", actorHandle)
		case "boost":
			detail = fmt.Sprintf("%s boosted your post", actorHandle)
		case "reply":
			detail = fmt.Sprintf("%s replied to your post", actorHandle)
		default:
			detail = fmt.Sprintf("%s - %s", n.Type, actorHandle)
		}

		fmt.Fprintf(ctx.W, "\033[36m[%s]\033[0m %s | %s - %s%s\n",
			prefix, n.Type, detail, n.CreatedAt.Format("2006-01-02 15:04"), unreadTag)
	}
	return nil
}

func notifRead(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: notif read ID")
	}

	notifs, err := ctx.DB.ListNotifications(ctx.Ctx, ctx.User.ID, 100, 0)
	if err != nil {
		return fmt.Errorf("list notifications: %w", err)
	}

	prefix := ctx.Args[1]
	var targetID *uuid.UUID
	for _, n := range notifs {
		if strings.HasPrefix(n.ID.String(), prefix) {
			id := n.ID
			targetID = &id
			break
		}
	}

	if targetID == nil {
		return fmt.Errorf("notification not found: %s", prefix)
	}

	if err := ctx.DB.MarkNotificationRead(ctx.Ctx, *targetID, ctx.User.ID); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Notification marked as read\033[0m\n")
	return nil
}

func notifClear(ctx *Context) error {
	if err := ctx.DB.ClearNotifications(ctx.Ctx, ctx.User.ID); err != nil {
		return fmt.Errorf("clear notifications: %w", err)
	}
	fmt.Fprintf(ctx.W, "\033[32m✓ All notifications cleared\033[0m\n")
	return nil
}
