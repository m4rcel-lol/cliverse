package commands

import (
	"fmt"
)

// HandleHelp displays help information for all commands or a specific command.
func HandleHelp(ctx *Context) error {
	if len(ctx.Args) > 0 {
		return helpCommand(ctx, ctx.Args[0])
	}
	return helpAll(ctx)
}

func helpAll(ctx *Context) error {
	fmt.Fprintf(ctx.W, "\033[1m\033[36m")
	fmt.Fprintf(ctx.W, "  ██████╗██╗     ██╗██╗   ██╗███████╗██████╗ ███████╗███████╗\n")
	fmt.Fprintf(ctx.W, " ██╔════╝██║     ██║██║   ██║██╔════╝██╔══██╗██╔════╝██╔════╝\n")
	fmt.Fprintf(ctx.W, " ██║     ██║     ██║██║   ██║█████╗  ██████╔╝███████╗█████╗  \n")
	fmt.Fprintf(ctx.W, " ██║     ██║     ██║╚██╗ ██╔╝██╔══╝  ██╔══██╗╚════██║██╔══╝  \n")
	fmt.Fprintf(ctx.W, " ╚██████╗███████╗██║ ╚████╔╝ ███████╗██║  ██║███████║███████╗\n")
	fmt.Fprintf(ctx.W, "  ╚═════╝╚══════╝╚═╝  ╚═══╝  ╚══════╝╚═╝  ╚═╝╚══════╝╚══════╝\n")
	fmt.Fprintf(ctx.W, "\033[0m")
	fmt.Fprintf(ctx.W, "\n\033[1mAvailable Commands\033[0m  (type 'help COMMAND' for details)\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m📝 Posts\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mpost\033[0m          Create, reply, delete, list posts\n")
	fmt.Fprintf(ctx.W, "  \033[36mdraft\033[0m         Save and manage post drafts\n")
	fmt.Fprintf(ctx.W, "  \033[36mthread\033[0m        View post threads and replies\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m📰 Timelines\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mtimeline\033[0m      Browse home, local, global, or mentions\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m🔔 Notifications\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mnotif\033[0m         List, read, and clear notifications\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m👤 Social\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mprofile\033[0m       View and edit your profile\n")
	fmt.Fprintf(ctx.W, "  \033[36mfollow\033[0m        Follow, unfollow, manage follow requests\n")
	fmt.Fprintf(ctx.W, "  \033[36mblock\033[0m         Block / unblock users\n")
	fmt.Fprintf(ctx.W, "  \033[36mmute\033[0m          Mute / unmute users\n")
	fmt.Fprintf(ctx.W, "  \033[36mreport\033[0m        Report a user or post\n")
	fmt.Fprintf(ctx.W, "  \033[36muser\033[0m          Look up local or remote users\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m❤️  Interactions\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mfav\033[0m           Like / unlike posts\n")
	fmt.Fprintf(ctx.W, "  \033[36mboost\033[0m         Boost / unboost posts\n")
	fmt.Fprintf(ctx.W, "  \033[36mbookmark\033[0m      Bookmark / unbookmark posts\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m🔍 Search\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36msearch\033[0m        Search for users or posts\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m⚙️  Settings\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36msettings\033[0m      Password, SSH keys, sessions, export\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m🛡️  Moderation (admin)\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mmod\033[0m           Suspend, silence, review reports\n")
	fmt.Fprintf(ctx.W, "  \033[36mfed\033[0m           Domain block/allow policies\n")
	fmt.Fprintf(ctx.W, "  \033[36madmin\033[0m         User management, stats, system config\n\n")

	fmt.Fprintf(ctx.W, "\033[1m\033[33m💻 Shell\033[0m\n")
	fmt.Fprintf(ctx.W, "  \033[36mhelp\033[0m          Show this help\n")
	fmt.Fprintf(ctx.W, "  \033[36minfo\033[0m          Show instance information\n")
	fmt.Fprintf(ctx.W, "  \033[36muptime\033[0m        Show server uptime\n")
	fmt.Fprintf(ctx.W, "  \033[36mversion\033[0m       Show software version\n")
	fmt.Fprintf(ctx.W, "  \033[36mwhoami\033[0m        Show your handle and role\n")
	fmt.Fprintf(ctx.W, "  \033[36mping\033[0m          Connectivity check\n")
	fmt.Fprintf(ctx.W, "  \033[36mclear\033[0m         Clear the screen\n")
	fmt.Fprintf(ctx.W, "  \033[36mexit\033[0m / \033[36mquit\033[0m  Disconnect\n\n")

	return nil
}

func helpCommand(ctx *Context, cmd string) error {
	helps := map[string]string{
		"post": `\033[1mpost\033[0m - Create and manage posts

  post global "message"       Publish a public post
  post local "message"        Publish a local-only post (no federation)
  post reply ID "message"     Reply to a post by its short ID
  post delete ID              Delete your own post
  post show ID                Show a single post with stats
  post list                   List your 20 most recent posts`,

		"timeline": `\033[1mtimeline\033[0m - Browse timelines

  timeline home       Posts from people you follow
  timeline local      Posts from local users only
  timeline global     All public posts (federated)
  timeline mentions   Posts mentioning you`,

		"notif": `\033[1mnotif\033[0m - Notifications

  notif list          List recent notifications (20)
  notif read ID       Mark a notification as read
  notif clear         Mark all notifications as read`,

		"profile": `\033[1mprofile\033[0m - View and edit profiles

  profile show                Show your profile
  profile show @user          Show a local user's profile
  profile show @user@domain   Show a remote user's profile
  profile set display_name "Name"
  profile set bio "Bio text"
  profile set avatar_url "https://..."
  profile set banner_url "https://..."`,

		"follow": `\033[1mfollow\033[0m - Manage follows

  follow add @user@domain     Follow a user (local or remote)
  follow remove @user@domain  Unfollow a user
  follow list                 List people you follow
  follow followers            List your followers
  follow requests             Show pending follow requests
  follow accept ID            Accept a follow request
  follow reject ID            Reject a follow request`,

		"block": `\033[1mblock\033[0m - Block users

  block add @user      Block a local user
  block remove @user   Unblock a user
  block list           List all blocked users`,

		"mute": `\033[1mmute\033[0m - Mute users

  mute add @user      Mute a user (hides their posts from your timeline)
  mute remove @user   Unmute a user
  mute list           List all muted users`,

		"report": `\033[1mreport\033[0m - Report content

  report user @username "reason"   Report a user
  report post POST_ID "reason"     Report a specific post`,

		"fav": `\033[1mfav\033[0m - Like posts

  fav add ID      Like a post
  fav remove ID   Remove your like`,

		"boost": `\033[1mboost\033[0m - Boost posts

  boost add ID      Boost a post
  boost remove ID   Remove your boost`,

		"bookmark": `\033[1mbookmark\033[0m - Bookmark posts

  bookmark add ID      Bookmark a post
  bookmark remove ID   Remove a bookmark
  bookmark list        List your bookmarks`,

		"search": `\033[1msearch\033[0m - Search

  search users "query"   Search local users by name
  search posts "query"   Search public posts by content`,

		"thread": `\033[1mthread\033[0m - View threads

  thread view ID   Show a post and all its replies`,

		"draft": `\033[1mdraft\033[0m - Manage drafts

  draft new "content"   Save a new draft
  draft list            List your drafts
  draft post ID         Publish a draft as a post
  draft delete ID       Delete a draft`,

		"user": `\033[1muser\033[0m - User lookup

  user show @user@domain    Show a user profile
  user lookup @user@domain  Fetch a remote user via WebFinger`,

		"settings": `\033[1msettings\033[0m - Account settings

  settings update_password             Change your password
  settings add_key "ssh-ed25519 …"     Add an SSH public key
  settings add_key_url URL             Import SSH keys from a URL (e.g. ssh.mreow.org/m)
  settings remove_key FINGERPRINT      Remove an SSH key
  settings list_keys                   List your SSH keys
  settings sessions                    List your active sessions
  settings export                      Export your account data as JSON`,

		"fed": `\033[1mfed\033[0m - Federation policies (admin only)

  fed list              List all domain policies
  fed block DOMAIN      Block a domain
  fed allow DOMAIN      Explicitly allow a domain
  fed remove DOMAIN     Remove a domain policy
  fed status DOMAIN     Check a domain's policy`,

		"mod": `\033[1mmod\033[0m - Moderation (admin only)

  mod suspend USERNAME    Suspend a user account
  mod unsuspend USERNAME  Re-enable a suspended account
  mod silence USERNAME    Silence a user
  mod reports             List open reports
  mod resolve ID          Resolve an open report
  mod note USER "note"    Add an audit log note`,

		"admin": `\033[1madmin\033[0m - Administration (admin only)

  admin create_user USERNAME [SSH_KEY_URL]  Create a new user account (optionally import SSH keys from URL)
  admin delete_user USERNAME                Permanently delete a user
  admin reset_password USERNAME             Generate a new temporary password
  admin add_key USERNAME "key"              Add SSH key for a user
  admin add_key_url USERNAME URL            Import SSH keys for a user from a URL (e.g. ssh.mreow.org/m)
  admin list_users                          List all users with status
  admin health                              Check DB/Redis health
  admin stats                               Show instance statistics
  admin broadcast "message"                 Send notification to all users
  admin maintenance on/off                  Toggle maintenance mode
  admin logs                                Show last 50 audit log entries`,

		"info": `\033[1minfo\033[0m - Instance information

  Shows instance name, domain, version, user and post counts, and uptime.`,

		"uptime": `\033[1muptime\033[0m - Server uptime

  Shows how long the server has been running and when it started.`,

		"version": `\033[1mversion\033[0m - Software version

  Displays the CLIverse build version string.`,

		"clear": `\033[1mclear\033[0m - Clear screen

  Clears the terminal screen.`,

		"ping": `\033[1mping\033[0m - Connectivity check

  Quick test that the server is responding.`,

		"whoami": `\033[1mwhoami\033[0m - Current user

  Shows your @handle and role (user or admin).`,
	}

	text, ok := helps[cmd]
	if !ok {
		return fmt.Errorf("no help available for '%s'", cmd)
	}

	fmt.Fprintf(ctx.W, "%s\n\n", text)
	return nil
}
