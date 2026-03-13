# CLIverse

CLIverse is a fully federated **Fediverse instance with a CLI-based interface**. Instead of a web browser, users connect via SSH and interact through a text-based terminal shell. It speaks ActivityPub, so it federates with Mastodon, Pleroma, and the rest of the Fediverse. I really love ActivityPub ❤️

## Features

- **ActivityPub federation** — compatible with Mastodon, Pleroma, and the wider Fediverse
- **SSH access** — log in with a password or SSH public key (no browser required)
- **SSH key import** — import keys from a URL (e.g. `ssh.example.com/example`)
- **Post & interact** — create posts, reply to threads, like, boost, and bookmark
- **Timelines** — home, local, global (federated), and mentions
- **Social graph** — follow/unfollow local and remote users, manage follow requests
- **User safety** — block users, mute users, report abuse
- **Notifications** — follows, mentions, likes, boosts, replies
- **Drafts** — save posts for later and publish when ready
- **Search** — find users and posts
- **Moderation** — admin tools for suspending accounts, silencing users, domain policies
- **NodeInfo** — standard `/.well-known/nodeinfo` for instance discoverability
- **Rate limiting** — SSH login rate limiting backed by Redis
- **MOTD** — configurable message of the day shown on login
- **Config validation** — startup checks for required settings
- **Post sanitization** — HTML tags stripped from user-created posts
- **Build versioning** — embedded version via `-ldflags` at build time

## Quick Start (Docker Compose)

```bash
# 1. Copy and edit the environment file
cp .env.example .env
$EDITOR .env

# 2. Generate an Argon2id password hash for the initial admin account
#    (requires Go 1.23+)
go run ./cmd/hash-password
# Enter your desired admin password at the prompt.
# Copy the printed hash into ADMIN_PASSWORD_HASH in your .env file.

# 3. Start the stack (app + worker + PostgreSQL + Redis)
docker compose up -d
# Point your reverse proxy at 127.0.0.1:8080 for HTTP traffic.

# 4. Connect via SSH as the admin user (default username: admin)
ssh -p 6969 admin@<your-domain>
```

On first startup, if `ADMIN_PASSWORD_HASH` is set in `.env` and no admin user exists yet,
the application automatically creates the initial admin account (username controlled by
`ADMIN_USERNAME`, defaulting to `admin`). The admin can then create additional users with
`admin create_user <username>`.

## Development

A **Makefile** is provided for common tasks:

```bash
make build          # Compile the main server binary
make build-worker   # Compile the federation worker binary
make all            # Build both binaries
make test           # Run all tests with race detection
make vet            # Run go vet
make lint           # vet + test
make fmt            # Format all Go source files
make clean          # Remove build artifacts
make docker         # Build Docker images with Compose
make docker-up      # Start all services
make docker-down    # Stop all services
```

## Requirements

- Docker & Docker Compose
- A domain name with DNS pointing to your server
- Ports 22 (or 6969), 80, and 443 open (handle TLS termination with your own reverse proxy)

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable             | Default                   | Description                        |
|----------------------|---------------------------|------------------------------------|
| `DOMAIN`             | `localhost`               | Public domain name of the instance |
| `INSTANCE_NAME`      | `CLIverse`                | Display name shown to other servers|
| `INSTANCE_DESC`      | `A CLIverse Fediverse instance` | Instance description          |
| `SSH_PORT`           | `6969`                    | SSH listen port                    |
| `HTTP_PORT`          | `8080`                    | Internal HTTP listen port          |
| `DATABASE_DSN`       | (postgres://...)          | PostgreSQL connection string       |
| `REDIS_URL`          | `redis://localhost:6379/0`| Redis connection URL               |
| `SESSION_SECRET`     | (changeme)                | Secret for session tokens          |
| `ADMIN_USERNAME`     | `admin`                   | Username for the bootstrap admin   |
| `ADMIN_PASSWORD_HASH`| _(empty)_                 | Argon2id hash; triggers admin bootstrap on first start |
| `MAX_POST_LENGTH`    | `500`                     | Maximum characters per post        |
| `SSH_IDLE_TIMEOUT`   | `30m`                     | SSH session idle timeout           |

## Shell Commands

Once connected via SSH, type `help` to see all commands. Key commands:

```
post global "Hello Fediverse!"         # Create a public post
post local "Local-only post"           # Create an unlisted local post
post reply <ID> "Nice!"               # Reply to a post
timeline home                          # View home timeline
timeline global                        # View federated timeline
follow add @user@mastodon.social       # Follow a remote user
block add @user                        # Block a user
mute add @user                         # Mute a user
report user @user "spam"               # Report a user
search users "alice"                   # Search for users
notif list                             # View notifications
settings add_key "ssh-ed25519 …"       # Add an SSH key directly
settings add_key_url ssh.mreow.org/m   # Import SSH keys from a URL
info                                   # Show instance info
uptime                                 # Show server uptime
version                                # Show build version
whoami                                 # Show your handle and role
ping                                   # Connectivity check
clear                                  # Clear the terminal
admin create_user alice                # Create a user account
admin create_user alice ssh.mreow.org  # Create user + import SSH keys
admin add_key_url alice ssh.mreow.org  # Import SSH keys for a user
help                                   # Full command list
```

## Federation

CLIverse implements:
- **ActivityPub** inbox/outbox with HTTP Signatures (RSA-SHA256)
- **WebFinger** (`/.well-known/webfinger`) for actor discovery
- **NodeInfo** (`/.well-known/nodeinfo`) for instance metadata
- Activities: `Follow`, `Accept`, `Reject`, `Create`, `Delete`, `Like`, `Announce`, `Undo`

## Architecture

```
cmd/cliverse/    — HTTP + SSH server entry point
cmd/worker/      — Background federation worker
internal/
  activitypub/   — ActivityPub protocol (inbox, outbox, signatures, nodeinfo)
  auth/          — Argon2id password hashing, SSH key fingerprints, rate limiting
  commands/      — CLI command handlers (post, timeline, follow, block, mute, …)
  config/        — Environment-based configuration with validation
  db/            — PostgreSQL data layer
  federation/    — Background delivery worker
  models/        — Data model definitions
  ssh/           — SSH server and interactive shell
migrations/      — PostgreSQL schema
```

## License

MIT
