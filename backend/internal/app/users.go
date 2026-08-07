package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"

	"golang.org/x/crypto/bcrypt"
)

type CreatedUser struct {
	model.User
	SessionToken string `json:"-"`
}

func (s *Service) Register(ctx context.Context, email, password string) (CreatedUser, error) {
	idBytes := make([]byte, 16)
	if _, err := io.ReadFull(s.random, idBytes); err != nil {
		return CreatedUser{}, fmt.Errorf("generate user id: %w", err)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return CreatedUser{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now().UTC()
	user := model.User{
		ID: "usr_" + hex.EncodeToString(idBytes), Email: email, DisplayName: emailName(email),
		Avatar: defaultAvatar, CreatedAt: now,
	}
	token, tokenHash, err := s.newSessionToken()
	if err != nil {
		return CreatedUser{}, err
	}
	if err := s.store.CreateUserWithSession(ctx, user, passwordHash, tokenHash, now.Add(s.sessionTTL)); err != nil {
		if store.Is(err, store.ErrConflict) {
			return CreatedUser{}, ErrEmailTaken
		}
		return CreatedUser{}, err
	}
	return CreatedUser{User: user, SessionToken: token}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (CreatedUser, error) {
	user, passwordHash, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		if translateStoreError(err) == ErrNotFound {
			return CreatedUser{}, ErrInvalidCredentials
		}
		return CreatedUser{}, err
	}
	if bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) != nil {
		return CreatedUser{}, ErrInvalidCredentials
	}
	token, tokenHash, err := s.newSessionToken()
	if err != nil {
		return CreatedUser{}, err
	}
	now := s.now().UTC()
	if err := s.store.CreateSession(ctx, user.ID, tokenHash, now, now.Add(s.sessionTTL)); err != nil {
		return CreatedUser{}, translateStoreError(err)
	}
	return CreatedUser{User: user, SessionToken: token}, nil
}

func (s *Service) newSessionToken() (string, [sha256.Size]byte, error) {
	tokenBytes := make([]byte, 32)
	if _, err := io.ReadFull(s.random, tokenBytes); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	return token, sha256.Sum256([]byte(token)), nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (model.User, error) {
	if token == "" {
		return model.User{}, ErrUnauthorized
	}
	now := s.now().UTC()
	hash := sha256.Sum256([]byte(token))
	user, err := s.store.UserBySessionHash(ctx, hash, now, now.Add(-s.sessionIdleTTL))
	if err != nil {
		if translateStoreError(err) == ErrNotFound {
			return model.User{}, ErrUnauthorized
		}
		return model.User{}, err
	}
	if err := s.store.TouchSession(ctx, hash, user.ID, now); err != nil {
		if store.Is(err, store.ErrNotFound) {
			return model.User{}, ErrUnauthorized
		}
		return model.User{}, err
	}
	state, err := s.store.SecurityState(ctx, user.ID, hash, now)
	if err != nil {
		return model.User{}, translateStoreError(err)
	}
	user.Security = &state
	return user, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.store.RevokeSession(ctx, sha256.Sum256([]byte(token))); err != nil {
		return err
	}
	return nil
}

func (s *Service) UpdateProfile(ctx context.Context, user model.User, displayName, avatar string) (model.User, error) {
	updated, err := s.store.UpdateProfile(ctx, user.ID, displayName, avatar)
	return updated, translateStoreError(err)
}

func (s *Service) Welcome(ctx context.Context, userID string) (model.WelcomeStats, error) {
	return s.store.WelcomeStats(ctx, userID, s.now().UTC())
}

func (s *Service) DeleteAvatar(ctx context.Context, user model.User) (model.User, error) {
	updated, err := s.store.UpdateProfile(ctx, user.ID, user.DisplayName, defaultAvatar)
	return updated, translateStoreError(err)
}

func (s *Service) ChangePassword(ctx context.Context, user model.User, token, password string) error {
	state, err := s.store.SecurityState(ctx, user.ID, sha256.Sum256([]byte(token)), s.now().UTC())
	if err != nil {
		return translateStoreError(err)
	}
	if !state.CanManage {
		return ErrForbidden
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.store.ChangePassword(ctx, user.ID, hash)
}

func (s *Service) SecuritySessions(ctx context.Context, user model.User, token string) ([]model.Session, error) {
	hash := sha256.Sum256([]byte(token))
	state, err := s.store.SecurityState(ctx, user.ID, hash, s.now().UTC())
	if err != nil {
		return nil, translateStoreError(err)
	}
	if !state.CanManage {
		return nil, ErrForbidden
	}
	if err := s.store.ReviewNewSessions(ctx, user.ID, hash); err != nil {
		return nil, err
	}
	return s.store.SecuritySessions(ctx, user.ID, hash, s.now().UTC(), s.sessionIdleTTL)
}

func (s *Service) RevokeSecuritySession(ctx context.Context, user model.User, token, id string) error {
	hash := sha256.Sum256([]byte(token))
	state, err := s.store.SecurityState(ctx, user.ID, hash, s.now().UTC())
	if err != nil {
		return translateStoreError(err)
	}
	if !state.CanManage {
		return ErrForbidden
	}
	return translateStoreError(s.store.RevokeSecuritySession(ctx, user.ID, id))
}

func (s *Service) DeleteUser(ctx context.Context, user model.User) error {
	codes, err := s.store.OwnedRoomCodes(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, code := range codes {
		if err := s.voice.DeleteRoom(ctx, code); err != nil {
			return err
		}
	}
	return translateStoreError(s.store.DeleteUser(ctx, user.ID))
}

func (s *Service) CleanupBannedUser(ctx context.Context, user model.User) error {
	roomNames, err := s.store.CleanupBannedUser(ctx, user.ID, s.now().UTC(), 100)
	if err != nil {
		return err
	}
	for _, roomName := range roomNames {
		if err := s.voice.RemoveParticipant(ctx, roomName, user.ID); err != nil {
			return err
		}
	}
	return nil
}

func emailName(email string) string {
	for i, r := range email {
		if r == '@' && i > 0 {
			return email[:i]
		}
	}
	return email
}

const defaultAvatar = ""

var ErrEmailTaken = errors.New("email already registered")
