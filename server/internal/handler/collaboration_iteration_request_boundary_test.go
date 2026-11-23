package handler

import (
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func iterationRequestBoundaryState(t *testing.T, issue, period string) string {
	t.Helper()
	var snapshot string
	dbfx.QueryRow(t, `SELECT jsonb_build_object(
		'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
		'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
		'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$1),
		'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$1),
		'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$1),
		'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$1),
		'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1),
		'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id WHERE s.issue_id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1),
		'period',(SELECT to_jsonb(i) FROM iteration i WHERE i.id=$2),
		'participations',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.iteration_id,p.issue_id) FROM iteration_participation p WHERE p.issue_id=$1),
		'iteration_events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.sequence) FROM iteration_event e WHERE e.iteration_id=$2)
	)::text`, issue, period).Scan(&snapshot)
	return snapshot
}

func TestCollaborationIterationPUTRejectsMetadataBeforeEffects(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	h := lifecycleHTTPHandler(t)
	period := lifecycleHTTPCreate(t, h, "Dedicated iteration request period")
	issue := collaborationEditFixture(t)
	privateEmail := "PRIVATE_ITERATION_REQUEST_ACCOUNT_" + uuid.NewString() + "@example.test"
	external := dbfx.User(t, "External request-boundary handler", privateEmail)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": external}, "user_id=$1", external)
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, external)
	project := dbfx.Project(t, "Current request-boundary project")
	privateParentBody := "PRIVATE_ITERATION_REQUEST_PARENT_BODY_" + uuid.NewString()
	parent := dbfx.Issue(t, "Current request-boundary parent", testutil.Cols{"description": privateParentBody})
	metadata := collaborationEditBodyValues()
	metadata["project_id"], metadata["parent_issue_id"], metadata["stage"] = project, parent, 2
	testutil.Call(t, h.UpdateCollaborationIssue, collaborationEditRequest(issue, testUserID, uuid.NewString(), metadata)).Want(http.StatusOK)
	beforeIssue, err := h.Queries.GetIssue(t.Context(), parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	requestBody := func() map[string]any {
		body := collaborationEditBodyValues()
		body["expected_collaboration_revision"], body["expected_revision"] = "2", strconv.FormatInt(beforeIssue.Revision, 10)
		body["current_iteration_id"], body["expected_iteration_revision"] = period, "1"
		return body
	}
	request := func(key string, body any) *http.Request {
		r := collaborationEditRequest(issue, testUserID, key, body)
		r.Method, r.URL.Path = http.MethodPut, "/api/collaboration/issues/"+issue+"/iteration"
		return r
	}
	privateTitle := "PRIVATE_MIXED_ITERATION_TITLE_" + uuid.NewString()
	assertPrivateResponse := func(response *testutil.Response) {
		t.Helper()
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("iteration request boundary response can be cached")
		}
		for _, private := range []string{privateEmail, privateParentBody, privateTitle, `"email"`, `"username"`, `"source_options"`} {
			if strings.Contains(response.Text(), private) {
				t.Fatalf("iteration request boundary response leaked private/request data: %q", private)
			}
		}
	}
	for _, extra := range []struct {
		key   string
		value any
	}{{"title", privateTitle}, {"project_id", project}, {"status", "done"}, {"stage", 3}} {
		t.Run(extra.key, func(t *testing.T) {
			before := iterationRequestBoundaryState(t, issue, period)
			body := requestBody()
			body[extra.key] = extra.value
			r := request(uuid.NewString(), body)
			response := testutil.Call(t, h.UpdateCollaborationIteration, r).Want(http.StatusBadRequest)
			if response.Map()["code"] != "invalid_request" || r.Method != http.MethodPut || r.URL.Path != "/api/collaboration/issues/"+issue+"/iteration" ||
				r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
				t.Fatal("mixed request bypassed the dedicated iteration boundary or retained ambient scope")
			}
			assertPrivateResponse(response)
			if iterationRequestBoundaryState(t, issue, period) != before {
				t.Fatal("mixed iteration PUT partially changed source fields, membership, history, operation or AI")
			}
		})
	}
	key := uuid.NewString()
	response := testutil.Call(t, h.UpdateCollaborationIteration, request(key, requestBody())).Want(http.StatusOK)
	if response.Map()["code"] != "iteration_changed" || response.Map()["collaboration_revision"] != "3" {
		t.Fatal("legal dedicated iteration PUT did not commit its exact membership result")
	}
	assertPrivateResponse(response)
	afterIssue, err := h.Queries.GetIssue(t.Context(), parseUUID(issue))
	if err != nil || afterIssue.CurrentIterationID != parseUUID(period) || afterIssue.Revision != beforeIssue.Revision+1 {
		t.Fatalf("legal dedicated iteration membership did not commit once: %+v %v", afterIssue, err)
	}
	unchanged := afterIssue
	unchanged.CurrentIterationID, unchanged.Revision, unchanged.UpdatedAt = beforeIssue.CurrentIterationID, beforeIssue.Revision, beforeIssue.UpdatedAt
	unchanged.LastActivityAt = beforeIssue.LastActivityAt
	if !reflect.DeepEqual(unchanged, beforeIssue) || dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND iteration_id=$2 AND current_joined_at IS NOT NULL`, issue, period) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE issue_id=$1 AND iteration_id=$2 AND actor->>'user_id'=$3`, issue, period, testUserID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue) != 0 {
		t.Fatal("dedicated iteration PUT changed unrelated metadata/responsibility or triggered AI")
	}
	committed := iterationRequestBoundaryState(t, issue, period)
	response = testutil.Call(t, h.UpdateCollaborationIteration, request(key, requestBody())).Want(http.StatusOK)
	assertPrivateResponse(response)
	if response.Map()["code"] != "iteration_changed" || response.Map()["collaboration_revision"] != "3" || iterationRequestBoundaryState(t, issue, period) != committed {
		t.Fatal("dedicated iteration replay recomputed membership/history")
	}
	var detail CollaborationIssueResponse
	response = testutil.Call(t, h.GetCollaborationIssue, collaborationFieldsReadRequest(issue, external, "")).Want(http.StatusOK)
	response.JSON(&detail)
	assertPrivateResponse(response)
	if detail.SourceStructure == nil || detail.SourceStructure.Iteration == nil || detail.SourceStructure.Iteration.ID != period ||
		detail.SourceStructure.Project == nil || detail.SourceStructure.Project.ID != project || detail.SourceStructure.ParentIssue == nil || detail.SourceStructure.ParentIssue.ID != parent {
		t.Fatal("dedicated iteration request lost external minimal source references")
	}
}
