package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"voice-rooms/internal/model"
)

type NewGroup struct{ ID, Visibility, OwnerID, Name, Avatar, InviteToken string }

func (s *Store) CreateGroup(ctx context.Context, in NewGroup, now time.Time, maxOwned int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin group: %w", err)
	}
	defer tx.Rollback()
	if maxOwned > 0 {
		var owned int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE owner_id=?`, in.OwnerID).Scan(&owned); err != nil {
			return fmt.Errorf("count owned groups: %w", err)
		}
		if owned >= maxOwned {
			return ErrGroupLimit
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO groups(id,visibility,owner_id,name,avatar,invite_token,created_at,last_activity_at) VALUES(?,?,?,?,?,?,?,?)`, in.ID, in.Visibility, in.OwnerID, in.Name, in.Avatar, in.InviteToken, now.Unix(), now.Unix())
	if err != nil {
		if isConstraint(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert group: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO group_members(group_id,user_id,joined_at) VALUES(?,?,?)`, in.ID, in.OwnerID, now.Unix()); err != nil {
		return fmt.Errorf("insert owner: %w", err)
	}
	return tx.Commit()
}

func (s *Store) IsGroupMember(ctx context.Context, groupID, userID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID).Scan(&n)
	return n > 0, err
}
func (s *Store) GroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM group_members WHERE group_id=?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) JoinGroup(ctx context.Context, groupID, userID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO group_members(group_id,user_id,joined_at) SELECT id,?,? FROM groups WHERE id=?`, userID, now.Unix(), groupID)
	if err != nil {
		if isConstraint(err) {
			return nil
		}
		return fmt.Errorf("join group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanGroup(row interface{ Scan(...any) error }) (model.Group, error) {
	var g model.Group
	var created, activity int64
	err := row.Scan(&g.ID, &g.Visibility, &g.Owner.ID, &g.Owner.DisplayName, &g.Owner.Avatar, &g.Name, &g.Avatar, &g.InviteToken, &created, &activity, &g.MemberCount)
	if err == sql.ErrNoRows {
		return g, ErrNotFound
	}
	if err != nil {
		return g, err
	}
	g.CreatedAt = time.Unix(created, 0).UTC()
	g.LastActivityAt = time.Unix(activity, 0).UTC()
	return g, nil
}

const groupFields = `g.id,g.visibility,u.id,u.display_name,u.avatar,g.name,g.avatar,g.invite_token,g.created_at,g.last_activity_at,(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id=g.id)`

func (s *Store) Group(ctx context.Context, id string) (model.Group, error) {
	g, e := scanGroup(s.db.QueryRowContext(ctx, `SELECT `+groupFields+` FROM groups g JOIN users u ON u.id=g.owner_id WHERE g.id=?`, id))
	if e != nil {
		return g, e
	}
	return s.hydrateGroup(ctx, g)
}
func (s *Store) GroupByInvite(ctx context.Context, token string) (model.Group, error) {
	g, e := scanGroup(s.db.QueryRowContext(ctx, `SELECT `+groupFields+` FROM groups g JOIN users u ON u.id=g.owner_id WHERE g.invite_token=?`, token))
	if e != nil {
		return g, e
	}
	return s.hydrateGroup(ctx, g)
}
func (s *Store) UserGroups(ctx context.Context, userID string) ([]model.Group, error) {
	return s.listGroups(ctx, `JOIN group_members own ON own.group_id=g.id WHERE own.user_id=? ORDER BY g.last_activity_at DESC`, userID)
}
func (s *Store) DiscoverGroups(ctx context.Context, q string, minMembers, limit, offset int) ([]model.Group, error) {
	return s.listGroups(ctx, `WHERE g.visibility='public' AND lower(g.name) LIKE ? AND (SELECT COUNT(*) FROM group_members gm WHERE gm.group_id=g.id) >= ? ORDER BY g.last_activity_at DESC LIMIT ? OFFSET ?`, "%"+strings.ToLower(q)+"%", minMembers, limit, offset)
}
func (s *Store) listGroups(ctx context.Context, tail string, args ...any) ([]model.Group, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT `+groupFields+` FROM groups g JOIN users u ON u.id=g.owner_id `+tail, args...)
	if e != nil {
		return nil, e
	}
	result := []model.Group{}
	for rows.Next() {
		g, e := scanGroup(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		result = append(result, g)
	}
	if e := rows.Err(); e != nil {
		rows.Close()
		return nil, e
	}
	if e := rows.Close(); e != nil {
		return nil, e
	}
	for i := range result {
		var e error
		result[i], e = s.hydrateGroup(ctx, result[i])
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
func (s *Store) hydrateGroup(ctx context.Context, g model.Group) (model.Group, error) {
	g.VoiceRooms = []model.GroupVoiceRoom{}
	rows, e := s.db.QueryContext(ctx, `SELECT vr.id,vr.group_id,vr.name,vr.created_at,(SELECT COUNT(*) FROM group_voice_members vm WHERE vm.voice_room_id=vr.id) FROM group_voice_rooms vr WHERE vr.group_id=? ORDER BY vr.created_at`, g.ID)
	if e != nil {
		return g, e
	}
	for rows.Next() {
		var r model.GroupVoiceRoom
		var t int64
		if e = rows.Scan(&r.ID, &r.GroupID, &r.Name, &t, &r.ParticipantCount); e != nil {
			return g, e
		}
		r.CreatedAt = time.Unix(t, 0).UTC()
		g.VoiceRooms = append(g.VoiceRooms, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return g, err
	}
	if err := rows.Close(); err != nil {
		return g, err
	}
	var m model.GroupMessage
	var created, authorCreated int64
	err := s.db.QueryRowContext(ctx, `SELECT m.id,m.group_id,m.body,m.created_at,u.id,u.display_name,u.avatar,u.created_at FROM group_messages m JOIN users u ON u.id=m.author_id WHERE m.group_id=? ORDER BY m.created_at DESC LIMIT 1`, g.ID).Scan(&m.ID, &m.GroupID, &m.Body, &created, &m.Author.ID, &m.Author.DisplayName, &m.Author.Avatar, &authorCreated)
	if err == nil {
		m.CreatedAt = time.Unix(created, 0).UTC()
		m.Author.CreatedAt = time.Unix(authorCreated, 0).UTC()
		g.LastMessage = &m
	}
	if err != nil && err != sql.ErrNoRows {
		return g, err
	}
	return g, nil
}

func (s *Store) Messages(ctx context.Context, groupID string, cutoff time.Time, limit int, reader string) ([]model.GroupMessage, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT m.id,m.group_id,m.body,m.created_at,u.id,u.display_name,u.avatar,u.created_at,CASE WHEN m.author_id=? AND EXISTS(SELECT 1 FROM group_message_reads r WHERE r.message_id=m.id AND r.user_id<>m.author_id) THEN 1 ELSE 0 END FROM group_messages m JOIN users u ON u.id=m.author_id WHERE m.group_id=? AND m.created_at>=? ORDER BY m.created_at DESC LIMIT ?`, reader, groupID, cutoff.Unix(), limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.GroupMessage{}
	for rows.Next() {
		var m model.GroupMessage
		var a, c, read int64
		if e = rows.Scan(&m.ID, &m.GroupID, &m.Body, &c, &m.Author.ID, &m.Author.DisplayName, &m.Author.Avatar, &a, &read); e != nil {
			return nil, e
		}
		m.CreatedAt = time.Unix(c, 0).UTC()
		m.Author.CreatedAt = time.Unix(a, 0).UTC()
		m.Read = read != 0
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out, nil
}
func (s *Store) Message(ctx context.Context, id string) (model.GroupMessage, error) {
	var m model.GroupMessage
	var created, authorCreated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT m.id,m.group_id,m.body,m.created_at,u.id,u.display_name,u.avatar,u.created_at
		FROM group_messages m JOIN users u ON u.id=m.author_id
		WHERE m.id=?`, id).Scan(
		&m.ID, &m.GroupID, &m.Body, &created,
		&m.Author.ID, &m.Author.DisplayName, &m.Author.Avatar, &authorCreated)
	if err == sql.ErrNoRows {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	m.CreatedAt = time.Unix(created, 0).UTC()
	m.Author.CreatedAt = time.Unix(authorCreated, 0).UTC()
	return m, nil
}
func (s *Store) AddMessage(ctx context.Context, m model.GroupMessage) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `INSERT INTO group_messages(id,group_id,author_id,body,created_at) VALUES(?,?,?,?,?)`, m.ID, m.GroupID, m.Author.ID, m.Body, m.CreatedAt.Unix()); e != nil {
		return fmt.Errorf("message: %w", e)
	}
	if _, e = tx.ExecContext(ctx, `UPDATE groups SET last_activity_at=? WHERE id=?`, m.CreatedAt.Unix(), m.GroupID); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) DeleteExpiredMessages(ctx context.Context, cutoff time.Time) error {
	_, e := s.db.ExecContext(ctx, `DELETE FROM group_messages WHERE created_at<?`, cutoff.Unix())
	return e
}
