package livekit

import (
	"testing"
	"time"

	"github.com/livekit/protocol/auth"
	livekitproto "github.com/livekit/protocol/livekit"

	"voice-rooms/internal/model"
)

func TestJoinTokenClaims(t *testing.T) {
	gateway := New("ws://internal", "wss://voice.example.com", "devkey", "secret")
	raw, err := gateway.JoinToken("AB12", model.User{ID: "usr_1", DisplayName: "Gleb"}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := auth.ParseAPIToken(raw)
	if err != nil {
		t.Fatal(err)
	}
	registered, grants, err := verifier.Verify("secret")
	if err != nil {
		t.Fatal(err)
	}
	if grants.Identity != "usr_1" || grants.Name != "Gleb" {
		t.Fatalf("wrong participant claims: %+v", grants)
	}
	video := grants.Video
	if video == nil || !video.RoomJoin || video.Room != "AB12" ||
		!video.GetCanPublish() || !video.GetCanSubscribe() || video.GetCanPublishData() {
		t.Fatalf("wrong room grants: %+v", video)
	}
	sources := video.GetCanPublishSources()
	wantSources := map[livekitproto.TrackSource]bool{
		livekitproto.TrackSource_MICROPHONE:         true,
		livekitproto.TrackSource_SCREEN_SHARE:       true,
		livekitproto.TrackSource_SCREEN_SHARE_AUDIO: true,
	}
	if len(sources) != len(wantSources) {
		t.Fatalf("unexpected publish sources: %v", sources)
	}
	for _, source := range sources {
		if !wantSources[source] {
			t.Fatalf("unexpected publish source: %v", source)
		}
	}
	if video.RoomAdmin || video.RoomCreate || video.RoomList || video.RoomRecord || video.IngressAdmin || video.Agent || video.Hidden || video.Recorder {
		t.Fatalf("token contains an administrative grant: %+v", video)
	}
	remaining := time.Until(registered.ExpiresAt.Time)
	if remaining < 4*time.Minute || remaining > 5*time.Minute {
		t.Fatalf("unexpected token TTL: %s", remaining)
	}
}
