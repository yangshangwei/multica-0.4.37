package handler

import (
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"testing"
)

func TestIterationInboxEventPreservesRecipientRouting(t *testing.T) {
	h := &Handler{Bus: events.New()}
	item := db.InboxItem{ID: parseUUID(uuid.NewString()), WorkspaceID: parseUUID(uuid.NewString()), RecipientType: "member", RecipientID: parseUUID(uuid.NewString()), Type: "iteration", Severity: "info", Title: "Iteration", Details: []byte(`{"iteration_id":"period-id","kind":"end"}`)}
	var captured []events.Event
	h.Bus.SubscribeAll(func(event events.Event) { captured = append(captured, event) })
	h.PublishIterationInbox(item)
	if len(captured) != 1 || captured[0].Type != protocol.EventInboxNew || captured[0].ActorType != "system" || captured[0].WorkspaceID != uuidToString(item.WorkspaceID) {
		t.Fatalf("unexpected event: %+v", captured)
	}
	payload := captured[0].Payload.(map[string]any)["item"].(map[string]any)
	if payload["id"] != uuidToString(item.ID) || payload["recipient_id"] != uuidToString(item.RecipientID) || payload["type"] != "iteration" {
		t.Fatalf("inbox routing payload: %+v", payload)
	}
}
