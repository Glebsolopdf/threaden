package events

import (
	"context"

	"voice-rooms/internal/groups/hub"
)

type MembershipReader interface {
	GroupMemberIDs(context.Context, string) ([]string, error)
}

type Publisher interface {
	Publish([]string, hub.Event)
}

func PublishMessageDeleted(ctx context.Context, members MembershipReader, publisher Publisher, groupID, messageID string) error {
	ids, err := members.GroupMemberIDs(ctx, groupID)
	if err != nil {
		return err
	}
	publisher.Publish(ids, hub.Event{Type: "message_deleted", GroupID: groupID, Data: hub.MessageDeletedEvent{MessageID: messageID}})
	return nil
}
