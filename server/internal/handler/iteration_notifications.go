package handler

import (
	"encoding/json"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// PublishIterationInbox is called only after the inbox/outbox transaction commits.
// The ordinary inbox event retains recipient routing and client cache semantics.
func (h *Handler) PublishIterationInbox(item db.InboxItem) {
	raw, err := json.Marshal(inboxToResponse(item))
	if err != nil {
		return
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return
	}
	h.publish(protocol.EventInboxNew, uuidToString(item.WorkspaceID), "system", "", map[string]any{"item": data})
}
