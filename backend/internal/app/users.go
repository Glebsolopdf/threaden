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
			return CreatedUser{}, ErrUnauthorized
		}
		return CreatedUser{}, err
	}
	if bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) != nil {
		return CreatedUser{}, ErrUnauthorized
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

func (s *Service) DeleteAvatar(ctx context.Context, user model.User) (model.User, error) {
	updated, err := s.store.UpdateProfile(ctx, user.ID, user.DisplayName, defaultAvatar)
	return updated, translateStoreError(err)
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
