package livekit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
	livekitproto "github.com/livekit/protocol/livekit"
	"github.com/livekit/psrpc"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"

	"voice-rooms/internal/model"
)

type Gateway struct {
	publicURL string
	client    *lksdk.RoomServiceClient
	apiKey    string
	apiSecret string
}

func New(internalURL, publicURL, apiKey, apiSecret string) *Gateway {
	return &Gateway{
		publicURL: publicURL,
		client:    lksdk.NewRoomServiceClient(internalURL, apiKey, apiSecret),
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

func (g *Gateway) PublicURL() string { return g.publicURL }

func (g *Gateway) JoinToken(roomCode string, user model.User, ttl time.Duration) (string, error) {
	canPublish := true
	canSubscribe := true
	canPublishData := false
	grant := &auth.VideoGrant{
		RoomJoin:          true,
		Room:              roomCode,
		CanPublish:        &canPublish,
		CanSubscribe:      &canSubscribe,
		CanPublishData:    &canPublishData,
		CanPublishSources: []string{"microphone"},
	}
	token, err := auth.NewAccessToken(g.apiKey, g.apiSecret).
		SetIdentity(user.ID).
		SetName(user.DisplayName).
		SetVideoGrant(grant).
		SetValidFor(ttl).
		ToJWT()
	if err != nil {
		return "", fmt.Errorf("sign LiveKit token: %w", err)
	}
	return token, nil
}

func (g *Gateway) DeleteRoom(ctx context.Context, roomCode string) error {
	_, err := g.client.DeleteRoom(ctx, &livekitproto.DeleteRoomRequest{Room: roomCode})
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete LiveKit room: %w", err)
	}
	return nil
}

func (g *Gateway) RemoveParticipant(ctx context.Context, roomCode, identity string) error {
	_, err := g.client.RemoveParticipant(ctx, &livekitproto.RoomParticipantIdentity{
		Room: roomCode, Identity: identity,
	})
	if isNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove LiveKit participant: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	code, ok := psrpc.GetErrorCode(err)
	if ok && code == psrpc.NotFound {
		return true
	}
	var twirpErr twirp.Error
	return errors.As(err, &twirpErr) && twirpErr.Code() == twirp.NotFound
}
