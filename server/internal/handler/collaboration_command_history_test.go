package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationCommandHistoryHTTPRejectsMalformedQuery(t *testing.T) {
	bare := &Handler{}
	for _, query := range []string{"limit=0", "limit=101", "limit=01", "limit=%2B1", "limit=1.0", "limit=1&limit=2", "limit=", "target_id=" + uuid.NewString(), "cursor=", "cursor=a&cursor=b", "cursor=%zz", "limit=1;cursor=x", "cursor=" + strings.Repeat("a", 1025)} {
		r := collaborationContentRequest(http.MethodGet, uuid.NewString(), "", testUserID, "", nil)
		r.URL.RawQuery = query
		response := testutil.Call(t, bare.GetCollaborationCommandHistory, r).Want(http.StatusBadRequest)
		if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" || r.Header.Get("X-Workspace-ID") != "" {
			t.Fatal("invalid history query kept ambient workspace or cache/referrer state")
		}
	}
	r := collaborationContentRequest(http.MethodGet, "not-a-uuid", "", testUserID, "", nil)
	testutil.Call(t, bare.GetCollaborationCommandHistory, r).Want(http.StatusBadRequest)
}

func TestCollaborationCommandHistoryHTTPPagingAndPrivateBoundary(t *testing.T) {
	f := newConfigHTTPFixture(t)
	target := f.register(t, collaboration.TargetAgent, f.agent)
	external := dbfx.User(t, "External command history viewer", "command-history-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue, "user_id": external}, "issue_id=$1 AND user_id=$2", f.issue, external)
	currentChannel, err := f.svc.Queries.EnsureCollaborationChannel(f.ctx, db.EnsureCollaborationChannelParams{SessionID: parseUUID(f.session), RootAgentID: parseUUID(f.agent)})
	if err != nil {
		t.Fatal(err)
	}
	channel := uuidToString(currentChannel.ID)
	dbfx.Exec(t, `UPDATE collaboration_channel SET next_command_seq=3 WHERE id=$1`, channel)
	if _, err := f.svc.Queries.BindCollaborationTargetChannel(f.ctx, db.BindCollaborationTargetChannelParams{SessionID: parseUUID(f.session), TargetID: target.ID, ChannelID: parseUUID(channel)}); err != nil {
		t.Fatal(err)
	}
	batch := dbfx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": f.session, "channel_id": channel, "effective_target_id": target.ID, "seq": 1})
	var latest string
	for i := 1; i <= 2; i++ {
		latest = dbfx.Insert(t, "collaboration_command", testutil.Cols{"session_id": f.session, "target_id": target.ID, "channel_id": channel, "batch_id": batch,
			"input_event_id": uuid.NewString(), "seq": i, "author_user_id": external, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1,
			"content": "PRIVATE_ORIGINAL_COMMAND", "state": "cancelled", "reason": "legacy_removed", "created_at": time.Now().Add(time.Duration(i) * time.Second)})
	}
	r := collaborationContentRequest(http.MethodGet, f.issue, "", external, "", nil)
	r.URL.RawQuery = "limit=1"
	var first service.CollaborationCommandHistoryPage
	response := testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusOK)
	response.JSON(&first)
	if first.ProtocolVersion != 1 || uuidToString(first.IssueID) != f.issue || len(first.Items) != 1 || uuidToString(first.Items[0].ID) != latest || first.NextCursor == nil || first.Items[0].Content != "PRIVATE_ORIGINAL_COMMAND" || first.Items[0].Author == nil || !first.Items[0].Author.External || first.Items[0].Cancellation == nil || first.Items[0].Cancellation.EventID != nil {
		t.Fatalf("history HTTP omitted immutable input or fabricated legacy cancellation details: %+v", first)
	}
	if !strings.Contains(response.Body.String(), `"seq":"2"`) || strings.Contains(response.Body.String(), "@example.test") || strings.Contains(response.Body.String(), "grant_id") || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatal("history HTTP rounded identity, leaked account/resource fields, or retained cache/ambient workspace")
	}
	r = collaborationContentRequest(http.MethodGet, f.issue, "", external, "", nil)
	r.URL.RawQuery = "cursor=" + *first.NextCursor
	var second service.CollaborationCommandHistoryPage
	testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusOK).JSON(&second)
	if len(second.Items) != 1 || second.Items[0].Seq != 1 || second.NextCursor != nil {
		t.Fatalf("history HTTP continuation duplicated or omitted input: %+v", second)
	}
	other := collaborationBoundaryIssue(t, testWorkspaceID)
	r = collaborationContentRequest(http.MethodGet, other, "", testUserID, "", nil)
	r.URL.RawQuery = "cursor=" + *first.NextCursor
	testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusBadRequest)
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud} {
		r = collaborationContentRequest(http.MethodGet, f.issue, "", testUserID, "", nil)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		denied := testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusNotFound)
		if strings.Contains(denied.Body.String(), "PRIVATE_ORIGINAL_COMMAND") {
			t.Fatal("machine denial leaked command content")
		}
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue, external)
	r = collaborationContentRequest(http.MethodGet, f.issue, "", external, "", nil)
	r.URL.RawQuery = "cursor=" + *first.NextCursor
	denied := testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusNotFound)
	if strings.Contains(denied.Body.String(), "PRIVATE_ORIGINAL_COMMAND") || denied.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("warm history read retained revoked content")
	}
	dbfx.Member(t, testWorkspaceID, external, "member")
	testutil.Call(t, testHandler.GetCollaborationCommandHistory, r).Want(http.StatusOK)
}
