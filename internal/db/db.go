package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// MaxDeliveryAttempts is the maximum number of times a federation delivery
// will be retried before being marked as failed.
const MaxDeliveryAttempts = 5

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// ==================== USERS ====================

func (d *DB) CreateUser(ctx context.Context, u *models.User) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO users (id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		u.ID, u.Username, u.Domain, u.DisplayName, u.Bio, u.AvatarURL, u.BannerURL,
		u.PasswordHash, u.PrivateKey, u.PublicKey, u.APID, u.InboxURL, u.OutboxURL,
		u.IsAdmin, u.IsLocked, u.IsSilenced, u.ForcePassChange, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (d *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return scanUser(d.pool.QueryRow(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at
		FROM users WHERE id=$1`, id))
}

func (d *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return scanUser(d.pool.QueryRow(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at
		FROM users WHERE username=$1`, username))
}

func (d *DB) GetUserByAPID(ctx context.Context, apID string) (*models.User, error) {
	return scanUser(d.pool.QueryRow(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at
		FROM users WHERE ap_id=$1`, apID))
}

func scanUser(row pgx.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Username, &u.Domain, &u.DisplayName, &u.Bio,
		&u.AvatarURL, &u.BannerURL, &u.PasswordHash, &u.PrivateKey, &u.PublicKey,
		&u.APID, &u.InboxURL, &u.OutboxURL,
		&u.IsAdmin, &u.IsLocked, &u.IsSilenced, &u.ForcePassChange,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (d *DB) UpdateUser(ctx context.Context, u *models.User) error {
	u.UpdatedAt = time.Now()
	_, err := d.pool.Exec(ctx, `
		UPDATE users SET display_name=$2, bio=$3, avatar_url=$4, banner_url=$5,
			password_hash=$6, is_admin=$7, is_locked=$8, is_silenced=$9,
			force_pass_change=$10, updated_at=$11
		WHERE id=$1`,
		u.ID, u.DisplayName, u.Bio, u.AvatarURL, u.BannerURL,
		u.PasswordHash, u.IsAdmin, u.IsLocked, u.IsSilenced,
		u.ForcePassChange, u.UpdatedAt,
	)
	return err
}

func (d *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

func (d *DB) ListUsers(ctx context.Context, limit, offset int) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Domain, &u.DisplayName, &u.Bio,
			&u.AvatarURL, &u.BannerURL, &u.PasswordHash, &u.PrivateKey, &u.PublicKey,
			&u.APID, &u.InboxURL, &u.OutboxURL,
			&u.IsAdmin, &u.IsLocked, &u.IsSilenced, &u.ForcePassChange,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (d *DB) SearchUsers(ctx context.Context, query string, limit int) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url, banner_url,
			password_hash, private_key, public_key, ap_id, inbox_url, outbox_url,
			is_admin, is_locked, is_silenced, force_pass_change, created_at, updated_at
		FROM users WHERE username ILIKE $1 OR display_name ILIKE $1
		ORDER BY username LIMIT $2`, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Domain, &u.DisplayName, &u.Bio,
			&u.AvatarURL, &u.BannerURL, &u.PasswordHash, &u.PrivateKey, &u.PublicKey,
			&u.APID, &u.InboxURL, &u.OutboxURL,
			&u.IsAdmin, &u.IsLocked, &u.IsSilenced, &u.ForcePassChange,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ==================== SSH KEYS ====================

func (d *DB) CreateSSHKey(ctx context.Context, k *models.SSHKey) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO ssh_keys (id, user_id, name, public_key, fingerprint, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		k.ID, k.UserID, k.Name, k.PublicKey, k.Fingerprint, k.CreatedAt,
	)
	return err
}

func (d *DB) GetSSHKeyByFingerprint(ctx context.Context, fp string) (*models.SSHKey, error) {
	k := &models.SSHKey{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, user_id, name, public_key, fingerprint, created_at
		FROM ssh_keys WHERE fingerprint=$1`, fp).Scan(
		&k.ID, &k.UserID, &k.Name, &k.PublicKey, &k.Fingerprint, &k.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

func (d *DB) ListSSHKeysByUser(ctx context.Context, userID uuid.UUID) ([]*models.SSHKey, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, user_id, name, public_key, fingerprint, created_at
		FROM ssh_keys WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.SSHKey
	for rows.Next() {
		k := &models.SSHKey{}
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.PublicKey, &k.Fingerprint, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (d *DB) DeleteSSHKey(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM ssh_keys WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

func (d *DB) DeleteSSHKeyByFingerprint(ctx context.Context, fp string, userID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM ssh_keys WHERE fingerprint=$1 AND user_id=$2`, fp, userID)
	return err
}

// ==================== SESSIONS ====================

func (d *DB) CreateSession(ctx context.Context, s *models.Session) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, remote_addr, created_at, last_seen_at, ended)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.UserID, s.RemoteAddr, s.CreatedAt, s.LastSeenAt, s.Ended,
	)
	return err
}

func (d *DB) UpdateSessionLastSeen(ctx context.Context, id uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE sessions SET last_seen_at=NOW() WHERE id=$1`, id)
	return err
}

func (d *DB) EndSession(ctx context.Context, id uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE sessions SET ended=TRUE WHERE id=$1`, id)
	return err
}

func (d *DB) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]*models.Session, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, user_id, remote_addr, created_at, last_seen_at, ended
		FROM sessions WHERE user_id=$1 AND ended=FALSE ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		s := &models.Session{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.RemoteAddr, &s.CreatedAt, &s.LastSeenAt, &s.Ended); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// ==================== POSTS ====================

func (d *DB) CreatePost(ctx context.Context, p *models.Post) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO posts (id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.LocalID, p.AuthorID, p.Content, p.Visibility, p.InReplyToID,
		p.ActivityPubID, p.RemoteID, p.Deleted, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (d *DB) GetPostByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	p := &models.Post{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE id=$1 AND deleted=FALSE`, id).Scan(
		&p.ID, &p.LocalID, &p.AuthorID, &p.Content, &p.Visibility, &p.InReplyToID,
		&p.ActivityPubID, &p.RemoteID, &p.Deleted, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) GetPostByLocalID(ctx context.Context, localID string) (*models.Post, error) {
	p := &models.Post{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE local_id=$1 AND deleted=FALSE`, localID).Scan(
		&p.ID, &p.LocalID, &p.AuthorID, &p.Content, &p.Visibility, &p.InReplyToID,
		&p.ActivityPubID, &p.RemoteID, &p.Deleted, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) GetPostByAPID(ctx context.Context, apID string) (*models.Post, error) {
	p := &models.Post{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE ap_id=$1`, apID).Scan(
		&p.ID, &p.LocalID, &p.AuthorID, &p.Content, &p.Visibility, &p.InReplyToID,
		&p.ActivityPubID, &p.RemoteID, &p.Deleted, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) DeletePost(ctx context.Context, id uuid.UUID, authorID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE posts SET deleted=TRUE, updated_at=NOW() WHERE id=$1 AND author_id=$2`,
		id, authorID)
	return err
}

func (d *DB) ListPostsByUser(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE author_id=$1 AND deleted=FALSE
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, authorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) GetHomeTimeline(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT p.id, p.local_id, p.author_id, p.content, p.visibility, p.in_reply_to_id,
			p.ap_id, p.remote_id, p.deleted, p.created_at, p.updated_at
		FROM posts p
		WHERE p.deleted=FALSE
		  AND p.visibility IN ('public','unlisted')
		  AND p.author_id IN (
			SELECT following_id FROM follows
			WHERE follower_id=$1 AND state='accepted' AND following_id IS NOT NULL
		  )
		ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) GetLocalTimeline(ctx context.Context, limit, offset int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT p.id, p.local_id, p.author_id, p.content, p.visibility, p.in_reply_to_id,
			p.ap_id, p.remote_id, p.deleted, p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.deleted=FALSE AND p.visibility='public'
		  AND p.remote_id IS NULL
		ORDER BY p.created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) GetGlobalTimeline(ctx context.Context, limit, offset int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE deleted=FALSE AND visibility='public'
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) GetMentionsTimeline(ctx context.Context, userID uuid.UUID, domain string, limit, offset int) ([]*models.Post, error) {
	u, err := d.GetUserByID(ctx, userID)
	if err != nil || u == nil {
		return nil, err
	}
	pattern := "%@" + u.Username + "%"
	rows, err := d.pool.Query(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE deleted=FALSE AND content ILIKE $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		pattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) GetThread(ctx context.Context, postID uuid.UUID) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		WITH RECURSIVE thread AS (
			SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
				ap_id, remote_id, deleted, created_at, updated_at
			FROM posts WHERE id=$1
			UNION ALL
			SELECT p.id, p.local_id, p.author_id, p.content, p.visibility, p.in_reply_to_id,
				p.ap_id, p.remote_id, p.deleted, p.created_at, p.updated_at
			FROM posts p
			JOIN thread t ON p.in_reply_to_id = t.id
		)
		SELECT * FROM thread WHERE deleted=FALSE ORDER BY created_at`,
		postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (d *DB) SearchPosts(ctx context.Context, query string, limit int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, local_id, author_id, content, visibility, in_reply_to_id,
			ap_id, remote_id, deleted, created_at, updated_at
		FROM posts WHERE deleted=FALSE AND visibility='public' AND content ILIKE $1
		ORDER BY created_at DESC LIMIT $2`,
		"%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func scanPosts(rows pgx.Rows) ([]*models.Post, error) {
	var posts []*models.Post
	for rows.Next() {
		p := &models.Post{}
		if err := rows.Scan(
			&p.ID, &p.LocalID, &p.AuthorID, &p.Content, &p.Visibility, &p.InReplyToID,
			&p.ActivityPubID, &p.RemoteID, &p.Deleted, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// ==================== FOLLOWS ====================

func (d *DB) CreateFollow(ctx context.Context, f *models.Follow) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO follows (id, follower_id, following_id, following_remote_id, state, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		f.ID, f.FollowerID, f.FollowingID, f.FollowingRemoteID, f.State, f.CreatedAt,
	)
	return err
}

func (d *DB) GetFollow(ctx context.Context, followerID, followingID uuid.UUID) (*models.Follow, error) {
	f := &models.Follow{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, follower_id, following_id, following_remote_id, state, created_at
		FROM follows WHERE follower_id=$1 AND following_id=$2`,
		followerID, followingID).Scan(
		&f.ID, &f.FollowerID, &f.FollowingID, &f.FollowingRemoteID, &f.State, &f.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return f, err
}

func (d *DB) UpdateFollowState(ctx context.Context, id uuid.UUID, state string) error {
	_, err := d.pool.Exec(ctx, `UPDATE follows SET state=$2 WHERE id=$1`, id, state)
	return err
}

func (d *DB) DeleteFollow(ctx context.Context, followerID, followingID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM follows WHERE follower_id=$1 AND following_id=$2`,
		followerID, followingID)
	return err
}

func (d *DB) ListFollowing(ctx context.Context, userID uuid.UUID) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT u.id, u.username, u.domain, u.display_name, u.bio, u.avatar_url, u.banner_url,
			u.password_hash, u.private_key, u.public_key, u.ap_id, u.inbox_url, u.outbox_url,
			u.is_admin, u.is_locked, u.is_silenced, u.force_pass_change, u.created_at, u.updated_at
		FROM users u
		JOIN follows f ON f.following_id = u.id
		WHERE f.follower_id=$1 AND f.state='accepted'
		ORDER BY u.username`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (d *DB) ListFollowers(ctx context.Context, userID uuid.UUID) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT u.id, u.username, u.domain, u.display_name, u.bio, u.avatar_url, u.banner_url,
			u.password_hash, u.private_key, u.public_key, u.ap_id, u.inbox_url, u.outbox_url,
			u.is_admin, u.is_locked, u.is_silenced, u.force_pass_change, u.created_at, u.updated_at
		FROM users u
		JOIN follows f ON f.follower_id = u.id
		WHERE f.following_id=$1 AND f.state='accepted'
		ORDER BY u.username`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (d *DB) ListPendingFollowRequests(ctx context.Context, userID uuid.UUID) ([]*models.Follow, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, follower_id, following_id, following_remote_id, state, created_at
		FROM follows WHERE following_id=$1 AND state='pending'
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []*models.Follow
	for rows.Next() {
		f := &models.Follow{}
		if err := rows.Scan(&f.ID, &f.FollowerID, &f.FollowingID, &f.FollowingRemoteID, &f.State, &f.CreatedAt); err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}
	return follows, rows.Err()
}

func (d *DB) GetFollowByID(ctx context.Context, id uuid.UUID) (*models.Follow, error) {
	f := &models.Follow{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, follower_id, following_id, following_remote_id, state, created_at
		FROM follows WHERE id=$1`, id).Scan(
		&f.ID, &f.FollowerID, &f.FollowingID, &f.FollowingRemoteID, &f.State, &f.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return f, err
}

func scanUsers(rows pgx.Rows) ([]*models.User, error) {
	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Domain, &u.DisplayName, &u.Bio,
			&u.AvatarURL, &u.BannerURL, &u.PasswordHash, &u.PrivateKey, &u.PublicKey,
			&u.APID, &u.InboxURL, &u.OutboxURL,
			&u.IsAdmin, &u.IsLocked, &u.IsSilenced, &u.ForcePassChange,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ==================== LIKES ====================

func (d *DB) CreateLike(ctx context.Context, l *models.Like) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO likes (id, user_id, post_id, ap_id, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		l.ID, l.UserID, l.PostID, l.APID, l.CreatedAt,
	)
	return err
}

func (d *DB) GetLike(ctx context.Context, userID, postID uuid.UUID) (*models.Like, error) {
	l := &models.Like{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, user_id, post_id, ap_id, created_at
		FROM likes WHERE user_id=$1 AND post_id=$2`,
		userID, postID).Scan(&l.ID, &l.UserID, &l.PostID, &l.APID, &l.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return l, err
}

func (d *DB) DeleteLike(ctx context.Context, userID, postID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM likes WHERE user_id=$1 AND post_id=$2`, userID, postID)
	return err
}

func (d *DB) CountLikes(ctx context.Context, postID uuid.UUID) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM likes WHERE post_id=$1`, postID).Scan(&count)
	return count, err
}

// ==================== BOOSTS ====================

func (d *DB) CreateBoost(ctx context.Context, b *models.Boost) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO boosts (id, user_id, post_id, ap_id, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		b.ID, b.UserID, b.PostID, b.APID, b.CreatedAt,
	)
	return err
}

func (d *DB) GetBoost(ctx context.Context, userID, postID uuid.UUID) (*models.Boost, error) {
	b := &models.Boost{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, user_id, post_id, ap_id, created_at
		FROM boosts WHERE user_id=$1 AND post_id=$2`,
		userID, postID).Scan(&b.ID, &b.UserID, &b.PostID, &b.APID, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func (d *DB) DeleteBoost(ctx context.Context, userID, postID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM boosts WHERE user_id=$1 AND post_id=$2`, userID, postID)
	return err
}

func (d *DB) CountBoosts(ctx context.Context, postID uuid.UUID) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM boosts WHERE post_id=$1`, postID).Scan(&count)
	return count, err
}

// ==================== BOOKMARKS ====================

func (d *DB) CreateBookmark(ctx context.Context, bm *models.Bookmark) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO bookmarks (id, user_id, post_id, created_at)
		VALUES ($1,$2,$3,$4)`,
		bm.ID, bm.UserID, bm.PostID, bm.CreatedAt,
	)
	return err
}

func (d *DB) DeleteBookmark(ctx context.Context, userID, postID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM bookmarks WHERE user_id=$1 AND post_id=$2`, userID, postID)
	return err
}

func (d *DB) ListBookmarks(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Post, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT p.id, p.local_id, p.author_id, p.content, p.visibility, p.in_reply_to_id,
			p.ap_id, p.remote_id, p.deleted, p.created_at, p.updated_at
		FROM posts p
		JOIN bookmarks b ON b.post_id = p.id
		WHERE b.user_id=$1 AND p.deleted=FALSE
		ORDER BY b.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ==================== NOTIFICATIONS ====================

func (d *DB) CreateNotification(ctx context.Context, n *models.Notification) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO notifications (id, user_id, type, actor_id, remote_actor_id, post_id, read, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		n.ID, n.UserID, n.Type, n.ActorID, n.RemoteActorID, n.PostID, n.Read, n.CreatedAt,
	)
	return err
}

func (d *DB) ListNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Notification, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, user_id, type, actor_id, remote_actor_id, post_id, read, created_at
		FROM notifications WHERE user_id=$1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []*models.Notification
	for rows.Next() {
		n := &models.Notification{}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.RemoteActorID, &n.PostID, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

func (d *DB) MarkNotificationRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE notifications SET read=TRUE WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

func (d *DB) ClearNotifications(ctx context.Context, userID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `UPDATE notifications SET read=TRUE WHERE user_id=$1`, userID)
	return err
}

func (d *DB) CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read=FALSE`, userID).Scan(&count)
	return count, err
}

// ==================== REMOTE ACTORS ====================

func (d *DB) CreateRemoteActor(ctx context.Context, a *models.RemoteActor) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO remote_actors (id, username, domain, display_name, bio, avatar_url,
			ap_id, inbox_url, outbox_url, public_key, followers_url, following_url, fetched_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (ap_id) DO UPDATE SET
			display_name=EXCLUDED.display_name, bio=EXCLUDED.bio, avatar_url=EXCLUDED.avatar_url,
			inbox_url=EXCLUDED.inbox_url, outbox_url=EXCLUDED.outbox_url,
			public_key=EXCLUDED.public_key, fetched_at=EXCLUDED.fetched_at`,
		a.ID, a.Username, a.Domain, a.DisplayName, a.Bio, a.AvatarURL,
		a.APID, a.InboxURL, a.OutboxURL, a.PublicKey, a.FollowersURL, a.FollowingURL, a.FetchedAt,
	)
	return err
}

func (d *DB) GetRemoteActorByAPID(ctx context.Context, apID string) (*models.RemoteActor, error) {
	a := &models.RemoteActor{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url,
			ap_id, inbox_url, outbox_url, public_key, followers_url, following_url, fetched_at
		FROM remote_actors WHERE ap_id=$1`, apID).Scan(
		&a.ID, &a.Username, &a.Domain, &a.DisplayName, &a.Bio, &a.AvatarURL,
		&a.APID, &a.InboxURL, &a.OutboxURL, &a.PublicKey, &a.FollowersURL, &a.FollowingURL, &a.FetchedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (d *DB) GetRemoteActorByUsernameAndDomain(ctx context.Context, username, domain string) (*models.RemoteActor, error) {
	a := &models.RemoteActor{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, username, domain, display_name, bio, avatar_url,
			ap_id, inbox_url, outbox_url, public_key, followers_url, following_url, fetched_at
		FROM remote_actors WHERE username=$1 AND domain=$2`, username, domain).Scan(
		&a.ID, &a.Username, &a.Domain, &a.DisplayName, &a.Bio, &a.AvatarURL,
		&a.APID, &a.InboxURL, &a.OutboxURL, &a.PublicKey, &a.FollowersURL, &a.FollowingURL, &a.FetchedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// ==================== FEDERATION DELIVERIES ====================

func (d *DB) CreateDelivery(ctx context.Context, fd *models.FederationDelivery) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO federation_deliveries (id, recipient_url, payload, attempts, last_attempt, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		fd.ID, fd.RecipientURL, fd.Payload, fd.Attempts, fd.LastAttempt, fd.Status, fd.CreatedAt,
	)
	return err
}

func (d *DB) GetPendingDeliveries(ctx context.Context, limit int) ([]*models.FederationDelivery, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, recipient_url, payload, attempts, last_attempt, status, created_at
		FROM federation_deliveries
		WHERE status='pending' AND attempts < $2
		ORDER BY created_at ASC LIMIT $1`, limit, MaxDeliveryAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*models.FederationDelivery
	for rows.Next() {
		fd := &models.FederationDelivery{}
		if err := rows.Scan(&fd.ID, &fd.RecipientURL, &fd.Payload, &fd.Attempts, &fd.LastAttempt, &fd.Status, &fd.CreatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, fd)
	}
	return deliveries, rows.Err()
}

func (d *DB) UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, status string, attempts int) error {
	now := time.Now()
	_, err := d.pool.Exec(ctx, `
		UPDATE federation_deliveries SET status=$2, attempts=$3, last_attempt=$4 WHERE id=$1`,
		id, status, attempts, now)
	return err
}

// ==================== DOMAIN POLICIES ====================

func (d *DB) CreateDomainPolicy(ctx context.Context, p *models.DomainPolicy) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO domain_policies (id, domain, action, reason, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (domain) DO UPDATE SET action=EXCLUDED.action, reason=EXCLUDED.reason`,
		p.ID, p.Domain, p.Action, p.Reason, p.CreatedAt,
	)
	return err
}

func (d *DB) GetDomainPolicy(ctx context.Context, domain string) (*models.DomainPolicy, error) {
	p := &models.DomainPolicy{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, domain, action, reason, created_at
		FROM domain_policies WHERE domain=$1`, domain).Scan(
		&p.ID, &p.Domain, &p.Action, &p.Reason, &p.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (d *DB) ListDomainPolicies(ctx context.Context) ([]*models.DomainPolicy, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, domain, action, reason, created_at
		FROM domain_policies ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*models.DomainPolicy
	for rows.Next() {
		p := &models.DomainPolicy{}
		if err := rows.Scan(&p.ID, &p.Domain, &p.Action, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (d *DB) DeleteDomainPolicy(ctx context.Context, domain string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM domain_policies WHERE domain=$1`, domain)
	return err
}

// ==================== REPORTS ====================

func (d *DB) CreateReport(ctx context.Context, r *models.Report) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO reports (id, reporter_id, target_user_id, target_post_id, reason, status, notes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		r.ID, r.ReporterID, r.TargetUserID, r.TargetPostID, r.Reason, r.Status, r.Notes, r.CreatedAt,
	)
	return err
}

func (d *DB) ListReports(ctx context.Context, status string) ([]*models.Report, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, reporter_id, target_user_id, target_post_id, reason, status, notes, created_at, resolved_at
		FROM reports WHERE status=$1 ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*models.Report
	for rows.Next() {
		r := &models.Report{}
		if err := rows.Scan(&r.ID, &r.ReporterID, &r.TargetUserID, &r.TargetPostID,
			&r.Reason, &r.Status, &r.Notes, &r.CreatedAt, &r.ResolvedAt); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (d *DB) ResolveReport(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	_, err := d.pool.Exec(ctx, `
		UPDATE reports SET status=$2, resolved_at=$3 WHERE id=$1`,
		id, status, now)
	return err
}

// ==================== AUDIT LOG ====================

func (d *DB) CreateAuditLog(ctx context.Context, l *models.AuditLog) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, actor_id, action, target, details, ip_addr, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		l.ID, l.ActorID, l.Action, l.Target, l.Details, l.IPAddr, l.CreatedAt,
	)
	return err
}

func (d *DB) ListAuditLogs(ctx context.Context, limit int) ([]*models.AuditLog, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, actor_id, action, target, details, ip_addr, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		l := &models.AuditLog{}
		if err := rows.Scan(&l.ID, &l.ActorID, &l.Action, &l.Target, &l.Details, &l.IPAddr, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ==================== DRAFTS ====================

func (d *DB) CreateDraft(ctx context.Context, dr *models.Draft) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO drafts (id, user_id, content, visibility, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		dr.ID, dr.UserID, dr.Content, dr.Visibility, dr.CreatedAt, dr.UpdatedAt,
	)
	return err
}

func (d *DB) GetDraft(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Draft, error) {
	dr := &models.Draft{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, user_id, content, visibility, created_at, updated_at
		FROM drafts WHERE id=$1 AND user_id=$2`, id, userID).Scan(
		&dr.ID, &dr.UserID, &dr.Content, &dr.Visibility, &dr.CreatedAt, &dr.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return dr, err
}

func (d *DB) ListDrafts(ctx context.Context, userID uuid.UUID) ([]*models.Draft, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, user_id, content, visibility, created_at, updated_at
		FROM drafts WHERE user_id=$1 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []*models.Draft
	for rows.Next() {
		dr := &models.Draft{}
		if err := rows.Scan(&dr.ID, &dr.UserID, &dr.Content, &dr.Visibility, &dr.CreatedAt, &dr.UpdatedAt); err != nil {
			return nil, err
		}
		drafts = append(drafts, dr)
	}
	return drafts, rows.Err()
}

func (d *DB) DeleteDraft(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM drafts WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

// ==================== SYSTEM CONFIG ====================

func (d *DB) GetSystemConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := d.pool.QueryRow(ctx, `SELECT value FROM system_config WHERE key=$1`, key).Scan(&value)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) SetSystemConfig(ctx context.Context, key, value string) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO system_config (key, value, updated_at) VALUES ($1,$2,NOW())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`,
		key, value)
	return err
}

// ==================== INBOX EVENTS ====================

func (d *DB) CreateInboxEvent(ctx context.Context, e *models.InboxEvent) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO inbox_events (id, sender_ap_id, activity_type, payload, processed, error, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.ID, e.SenderAPID, e.ActivityType, e.Payload, e.Processed, e.Error, e.CreatedAt,
	)
	return err
}

func (d *DB) GetUnprocessedInboxEvents(ctx context.Context, limit int) ([]*models.InboxEvent, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, sender_ap_id, activity_type, payload, processed, error, created_at
		FROM inbox_events WHERE processed=FALSE
		ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.InboxEvent
	for rows.Next() {
		e := &models.InboxEvent{}
		if err := rows.Scan(&e.ID, &e.SenderAPID, &e.ActivityType, &e.Payload, &e.Processed, &e.Error, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (d *DB) MarkInboxEventProcessed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE inbox_events SET processed=TRUE, error=$2 WHERE id=$1`, id, errMsg)
	return err
}

// ==================== BLOCKS ====================

func (d *DB) CreateBlock(ctx context.Context, b *models.Block) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO blocks (id, blocker_id, blocked_id, created_at)
		VALUES ($1,$2,$3,$4)`,
		b.ID, b.BlockerID, b.BlockedID, b.CreatedAt,
	)
	return err
}

func (d *DB) GetBlock(ctx context.Context, blockerID, blockedID uuid.UUID) (*models.Block, error) {
	b := &models.Block{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, blocker_id, blocked_id, created_at
		FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`,
		blockerID, blockedID).Scan(&b.ID, &b.BlockerID, &b.BlockedID, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (d *DB) DeleteBlock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`,
		blockerID, blockedID)
	return err
}

func (d *DB) ListBlocks(ctx context.Context, blockerID uuid.UUID) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT u.id, u.username, u.domain, u.display_name, u.bio, u.avatar_url, u.banner_url,
			u.password_hash, u.private_key, u.public_key, u.ap_id, u.inbox_url, u.outbox_url,
			u.is_admin, u.is_locked, u.is_silenced, u.force_pass_change, u.created_at, u.updated_at
		FROM blocks bl JOIN users u ON u.id = bl.blocked_id
		WHERE bl.blocker_id=$1 ORDER BY bl.created_at DESC`, blockerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (d *DB) IsBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	b, err := d.GetBlock(ctx, blockerID, blockedID)
	if err != nil {
		return false, err
	}
	return b != nil, nil
}

// ==================== MUTES ====================

func (d *DB) CreateMute(ctx context.Context, m *models.Mute) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO mutes (id, muter_id, muted_id, created_at)
		VALUES ($1,$2,$3,$4)`,
		m.ID, m.MuterID, m.MutedID, m.CreatedAt,
	)
	return err
}

func (d *DB) GetMute(ctx context.Context, muterID, mutedID uuid.UUID) (*models.Mute, error) {
	m := &models.Mute{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, muter_id, muted_id, created_at
		FROM mutes WHERE muter_id=$1 AND muted_id=$2`,
		muterID, mutedID).Scan(&m.ID, &m.MuterID, &m.MutedID, &m.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (d *DB) DeleteMute(ctx context.Context, muterID, mutedID uuid.UUID) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM mutes WHERE muter_id=$1 AND muted_id=$2`,
		muterID, mutedID)
	return err
}

func (d *DB) ListMutes(ctx context.Context, muterID uuid.UUID) ([]*models.User, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT u.id, u.username, u.domain, u.display_name, u.bio, u.avatar_url, u.banner_url,
			u.password_hash, u.private_key, u.public_key, u.ap_id, u.inbox_url, u.outbox_url,
			u.is_admin, u.is_locked, u.is_silenced, u.force_pass_change, u.created_at, u.updated_at
		FROM mutes mu JOIN users u ON u.id = mu.muted_id
		WHERE mu.muter_id=$1 ORDER BY mu.created_at DESC`, muterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (d *DB) CountLocalUsers(ctx context.Context, domain string) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE domain=$1`, domain).Scan(&count)
	return count, err
}

func (d *DB) CountLocalPosts(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM posts WHERE deleted=FALSE`).Scan(&count)
	return count, err
}
