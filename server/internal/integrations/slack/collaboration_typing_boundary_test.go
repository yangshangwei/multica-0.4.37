package slack

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/slack-go/slack"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type collaborationTypingQueries struct {
	*fakeOutboundQueries
	installationReads int
}

func (q *collaborationTypingQueries) GetChannelInstallation(ctx context.Context, input db.GetChannelInstallationParams) (db.ChannelInstallation, error) {
	q.installationReads++
	return q.fakeOutboundQueries.GetChannelInstallation(ctx, input)
}

// Typing owns local tracked reaction state, not a task/content read capability.
// These synthetic identities test that exact boundary through the real Slack
// HTTP client and production subscriptions. The outbound reply suite owns the
// real stored collaboration-source and immutable-delivery classification.
func TestCollaborationOutboundSlackTypingPrivateEventsOnlyClearTrackedOrdinaryState(t *testing.T) {
	const private = "IM_BOUNDARY_PRIVATE_BODY\nIM_BOUNDARY_PRIVATE_COMMENT\nIM_BOUNDARY_PRIVATE_FILE\nIM_BOUNDARY_PRIVATE_RECORD"
	q := &collaborationTypingQueries{fakeOutboundQueries: &fakeOutboundQueries{inst: db.ChannelInstallation{ID: uid(1), Status: "active", Config: slackInstallConfigJSON()}}}
	var mu sync.Mutex
	var wire []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error("local Slack reaction request was not form encoded")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		wire = append(wire, r.Method+" "+r.URL.Path+" "+r.PostForm.Encode())
		mu.Unlock()
		if r.URL.Path != "/reactions.add" && r.URL.Path != "/reactions.remove" {
			t.Error("typing lifecycle attempted a non-reaction Slack request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("channel") != "C_OWN_BOUNDARY" || r.PostForm.Get("name") != typingEmoji || r.PostForm.Get("timestamp") == "" {
			t.Error("typing lifecycle changed its tracked ordinary message identity")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(api.Close)
	creds := 0
	manager := NewTypingIndicatorManager(q, func([]byte) ([]byte, error) { creds++; return []byte("xoxb-local-unused"), nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	manager.newAPI = func(c credentials) reactionAPI {
		return slack.New(c.BotToken, slack.OptionAPIURL(api.URL+"/"), slack.OptionHTTPClient(api.Client()))
	}
	bus := events.New()
	manager.Register(bus)
	privateEvent := func(kind, task, session string) events.Event {
		return events.Event{Type: kind, TaskID: task, ChatSessionID: session, Payload: map[string]any{"task_id": task, "chat_session_id": session, "content": private, "error": private, "attachments": []map[string]any{{"filename": "private-fixture.txt", "content": private}}}}
	}
	identities := []struct{ name, task, session string }{
		{"shared_source_run", util.UUIDToString(uid(8)), util.UUIDToString(uid(9))},
		{"ordinary_child_without_tracked_reaction", util.UUIDToString(uid(10)), util.UUIDToString(uid(11))},
	}
	kinds := []string{protocol.EventChatDone, protocol.EventTaskFailed, protocol.EventTaskCancelled}
	for _, candidate := range identities {
		for _, kind := range kinds {
			t.Run(candidate.name+"/"+kind, func(t *testing.T) {
				bus.Publish(privateEvent(kind, candidate.task, candidate.session))
				if q.installationReads != 0 || creds != 0 || len(wire) != 0 || len(manager.states) != 0 {
					t.Fatal("untracked private event read credentials or touched Slack wire")
				}
			})
		}
	}
	ordinary := uid(7)
	for i, kind := range kinds {
		manager.Add(t.Context(), q.inst, ordinary, "C_OWN_BOUNDARY", freshTS())
		if len(wire) != i*2+1 || !strings.Contains(wire[i*2], "/reactions.add ") || creds != i*2+1 {
			t.Fatal("ordinary typing positive did not reach actual local Slack HTTP")
		}
		for _, candidate := range identities {
			bus.Publish(privateEvent(kind, candidate.task, candidate.session))
		}
		if len(wire) != i*2+1 || q.installationReads != i || len(manager.states[util.UUIDToString(ordinary)]) != 1 {
			t.Fatal("private event consumed an unrelated tracked ordinary reaction")
		}
		// A legitimate lifecycle clears only the already tracked message; the
		// private extra payload fields cannot become a reply or reaction value.
		event := privateEvent(kind, util.UUIDToString(uid(12)), util.UUIDToString(ordinary))
		bus.Publish(event)
		bus.Publish(event)
		if len(wire) != (i+1)*2 || !strings.Contains(wire[i*2+1], "/reactions.remove ") || q.installationReads != i+1 || creds != (i+1)*2 || len(manager.states) != 0 {
			t.Fatal("ordinary lifecycle subscriber did not clear its exact reaction once")
		}
	}
	for _, request := range wire {
		for _, sentinel := range strings.Split(private, "\n") {
			if strings.Contains(request, sentinel) {
				t.Fatal("typing lifecycle serialized protected content into Slack wire")
			}
		}
	}
}
