package handler

import (
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/inboxstate"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// PublishIterationInbox is called only after the inbox/outbox transaction commits.
// The ordinary inbox event retains recipient routing and client cache semantics.
func (h *Handler) PublishIterationInbox(delivery inboxstate.Delivery) {
	h.publishInboxDelivery(delivery, "system", "")
}

func (h *Handler) publishInboxDelivery(delivery inboxstate.Delivery, actorType, actorID string) {
	if !delivery.Created {
		return
	}
	if delivery.Suppressed {
		h.publish(protocol.EventInboxUpdated, uuidToString(delivery.State.WorkspaceID), actorType, actorID, inboxstate.Payload(delivery.State))
		return
	}
	item := delivery.Item
	raw, err := json.Marshal(inboxToResponse(item))
	if err != nil {
		return
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return
	}
	data["snooze_revision"] = delivery.State.Revision
	data["snoozed_until"] = util.TimestampToPtr(delivery.State.SnoozedUntil)
	data["reminded_at"] = util.TimestampToPtr(delivery.State.RemindedAt)
	data["reminder_reason"] = util.TextToPtr(delivery.State.ReminderReason)
	h.publish(protocol.EventInboxNew, uuidToString(item.WorkspaceID), actorType, actorID, map[string]any{"item": data})
}
