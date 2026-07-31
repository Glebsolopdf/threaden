package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"voice-rooms/internal/model"
)

const legacySessionTTL = 7 * 24 * time.Hour

func (s *Store) CreateUser(ctx context.Context, user model.User, passwordHash []byte, tokenHash [sha256.Size]byte) error {
	return s.CreateUserWithSession(ctx, user, passwordHash, tokenHash, user.CreatedAt.Add(legacySessionTTL))
}

func (s *Store) CreateUserWithSession(
	ctx context.Context,
	user model.User,
	passwordHash []byte,
	tokenHash [sha256.Size]byte,
	expiresAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users(id, email, display_name, avatar, password_hash, token_hash, created_at, last_seen_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.DisplayName, user.Avatar, passwordHash, tokenHash[:], user.CreatedAt.Unix(), user.CreatedAt.Unix())
	if err != nil {
		if isConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO sessions(token_hash, user_id, created_at, last_seen_at, expires_at, reviewed_at)
		VALUES(?, ?, ?, ?, ?, ?)`, tokenHash[:], user.ID, user.CreatedAt.Unix(), user.CreatedAt.Unix(), expiresAt.Unix(), user.CreatedAt.Unix()); err != nil {
		if isConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert initial session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create user: %w", err)
	}
	return nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (model.User, []byte, error) {
	var user model.User
	var passwordHash []byte
	var created int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, avatar, password_hash, created_at
		FROM users WHERE email = ?`, email).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.Avatar, &passwordHash, &created)
	if err == sql.ErrNoRows {
		return model.User{}, nil, ErrNotFound
	}
	if err != nil {
		return model.User{}, nil, fmt.Errorf("find user by email: %w", err)
	}
	user.CreatedAt = time.Unix(created, 0).UTC()
	return user, passwordHash, nil
}

// UserByTokenHash remains for store-level compatibility checks. Authentication
// must use UserBySessionHash so expiry and idle timeout are enforced.
func (s *Store) UserByTokenHash(ctx context.Context, hash [sha256.Size]byte) (model.User, error) {
	return s.userBySessionQuery(ctx, hash, "", nil)
}

func (s *Store) UserBySessionHash(
	ctx context.Context,
	hash [sha256.Size]byte,
	now time.Time,
	idleCutoff time.Time,
) (model.User, error) {
	condition := `AND sess.expires_at > ? AND sess.last_seen_at > ?`
	return s.userBySessionQuery(ctx, hash, condition, []any{now.Unix(), idleCutoff.Unix()})
}

func (s *Store) userBySessionQuery(ctx context.Context, hash [sha256.Size]byte, condition string, args []any) (model.User, error) {
	var user model.User
	var created int64
	queryArgs := []any{hash[:]}
	queryArgs = append(queryArgs, args...)
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.avatar, u.created_at
		FROM sessions sess
		JOIN users u ON u.id = sess.user_id
		WHERE sess.token_hash = ? `+condition, queryArgs...).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.Avatar, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by session: %w", err)
	}
	user.CreatedAt = time.Unix(created, 0).UTC()
	return user, nil
}

func (s *Store) CreateSession(
	ctx context.Context,
	userID string,
	tokenHash [sha256.Size]byte,
	now time.Time,
	expiresAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create session: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET token_hash = ?, last_seen_at = ? WHERE id = ?`, tokenHash[:], now.Unix(), userID)
	if err != nil {
		return fmt.Errorf("update legacy session marker: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO sessions(token_hash, user_id, created_at, last_seen_at, expires_at)
		VALUES(?, ?, ?, ?, ?)`, tokenHash[:], userID, now.Unix(), now.Unix(), expiresAt.Unix()); err != nil {
		if isConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit session: %w", err)
	}
	return nil
}

func (s *Store) UpdateSessionToken(ctx context.Context, userID string, tokenHash [sha256.Size]byte, now time.Time) error {
	return s.CreateSession(ctx, userID, tokenHash, now, now.Add(legacySessionTTL))
}

func (s *Store) TouchSession(ctx context.Context, hash [sha256.Size]byte, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin touch session: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE token_hash = ? AND user_id = ?`, now.Unix(), hash[:], userID)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, now.Unix(), userID); err != nil {
		return fmt.Errorf("touch user: %w", err)
	}
	return tx.Commit()
}

func (s *Store) RevokeSession(ctx context.Context, hash [sha256.Size]byte) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash[:]); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) SecurityState(ctx context.Context, userID string, hash [sha256.Size]byte, now time.Time) (model.SecurityState, error) {
	var created int64
	if err := s.db.QueryRowContext(ctx, `SELECT created_at FROM sessions WHERE user_id = ? AND token_hash = ?`, userID, hash[:]).Scan(&created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SecurityState{}, ErrNotFound
		}
		return model.SecurityState{}, fmt.Errorf("get session security state: %w", err)
	}
	state := model.SecurityState{CanManage: now.Sub(time.Unix(created, 0)) >= 24*time.Hour}
	if !state.CanManage {
		return state, nil
	}
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id = ? AND created_at > ? AND reviewed_at IS NULL)`, userID, created).Scan(&state.Alert)
	return state, err
}

func (s *Store) SecuritySessions(ctx context.Context, userID string, hash [sha256.Size]byte, now time.Time, idleTTL time.Duration) ([]model.Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT lower(substr(hex(token_hash), 1, 16)), created_at, last_seen_at, token_hash = ? FROM sessions WHERE user_id = ? AND expires_at > ? AND last_seen_at > ? ORDER BY created_at DESC`, hash[:], userID, now.Unix(), now.Add(-idleTTL).Unix())
	if err != nil {
		return nil, fmt.Errorf("list security sessions: %w", err)
	}
	defer rows.Close()
	items := []model.Session{}
	for rows.Next() {
		var item model.Session
		var created, seen int64
		if err := rows.Scan(&item.ID, &created, &seen, &item.Current); err != nil {
			return nil, err
		}
		item.CreatedAt, item.LastSeenAt = time.Unix(created, 0).UTC(), time.Unix(seen, 0).UTC()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReviewNewSessions(ctx context.Context, userID string, hash [sha256.Size]byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET reviewed_at = created_at WHERE user_id = ? AND created_at > (SELECT created_at FROM sessions WHERE token_hash = ?)`, userID, hash[:])
	return err
}

func (s *Store) RevokeSecuritySession(ctx context.Context, userID, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND lower(substr(hex(token_hash), 1, 16)) = ?`, userID, id)
	if err != nil {
		return fmt.Errorf("revoke security session: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ChangePassword(ctx context.Context, userID string, passwordHash []byte) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func (s *Store) UpdateProfile(ctx context.Context, userID, displayName, avatar string) (model.User, error) {
	var user model.User
	var created int64
	err := s.db.QueryRowContext(ctx, `
		UPDATE users SET display_name = ?, avatar = ? WHERE id = ?
		RETURNING id, email, display_name, avatar, created_at`,
		displayName, avatar, userID).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Avatar, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("update profile: %w", err)
	}
	user.CreatedAt = time.Unix(created, 0).UTC()
	return user, nil
}

func (s *Store) OwnedRoomCodes(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code FROM rooms WHERE owner_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("list owned rooms: %w", err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan owned room: %w", err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
