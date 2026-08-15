package groups_test

import (
	"testing"

	appgroups "voice-rooms/internal/groups"
)

func TestMessageHasContentAllowsTextAndAttachments(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		attachments  int
		wantAccepted bool
	}{
		{name: "text only", body: "hello", wantAccepted: true},
		{name: "text and attachment", body: "photo", attachments: 1, wantAccepted: true},
		{name: "attachment only", attachments: 1, wantAccepted: true},
		{name: "whitespace and attachment", body: "  ", attachments: 1, wantAccepted: true},
		{name: "empty", wantAccepted: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := appgroups.MessageHasContent(tc.body, tc.attachments); got != tc.wantAccepted {
				t.Fatalf("MessageHasContent(%q, %d)=%v, want %v", tc.body, tc.attachments, got, tc.wantAccepted)
			}
		})
	}
}
