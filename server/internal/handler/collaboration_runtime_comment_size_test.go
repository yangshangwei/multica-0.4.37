package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// The runtime CLI posts with json.Marshal, which writes each <, > and & as six
// bytes. The comment and squad activity limits apply to decoded text, so the
// raw read bound must admit any legal text at its worst-case escaping and the
// service then enforces the decoded limit.
func TestCollaborationRuntimeCommentBodyAcceptsCodeWithinTheContentLimit(t *testing.T) {
	post := func(payload map[string]any) *http.Request {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/runtime/comments", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	comment := func(w http.ResponseWriter, r *http.Request) {
		var body collaborationRuntimeCommentBody
		if decodeCollaborationRuntimeCommentBody(w, r, &body) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
	activity := func(w http.ResponseWriter, r *http.Request) {
		var body collaborationRuntimeSquadActivityBody
		if decodeCollaborationRuntimeSquadActivityBody(w, r, &body) {
			w.WriteHeader(http.StatusNoContent)
		}
	}

	snippet := strings.Repeat("<div><span>&nbsp;</span></div>\n", 1600) // 49,600 bytes decoded
	testutil.Call(t, comment, post(map[string]any{"comment_id": uuid.NewString(), "parent_id": nil, "content": snippet})).Want(http.StatusNoContent)

	worstComment := strings.Repeat("<", service.CollaborationCommentMaxBytes)
	testutil.Call(t, comment, post(map[string]any{"comment_id": uuid.NewString(), "parent_id": uuid.NewString(), "content": worstComment})).Want(http.StatusNoContent)

	worstReason := strings.Repeat("&", service.CollaborationSquadActivityReasonMaxBytes)
	testutil.Call(t, activity, post(map[string]any{"event_id": uuid.NewString(), "outcome": "action", "reason": worstReason})).Want(http.StatusNoContent)

	// The read stays bounded: a body far beyond any legal comment is refused
	// before decoding.
	oversized := strings.Repeat("<", 2*service.CollaborationCommentMaxBytes)
	testutil.Call(t, comment, post(map[string]any{"comment_id": uuid.NewString(), "parent_id": nil, "content": oversized})).Want(http.StatusRequestEntityTooLarge)
}
