package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationBoundaryIssue(t *testing.T, workspaceID string) string {
	t.Helper()
	issueID := dbfx.Issue(t, "Protected collaboration", testutil.Cols{"workspace_id": workspaceID, "assignee_type": "member", "assignee_id": testUserID})
	dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issueID}, "issue_id=$1", issueID)
	dbfx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issueID})
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issueID, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issueID, testUserID)
	return issueID
}

func TestLegacyIssueWritesCannotBypassCollaborationLifecycle(t *testing.T) {
	issueID := collaborationBoundaryIssue(t, testWorkspaceID)
	for _, tc := range []struct {
		method, body string
		handler      http.HandlerFunc
	}{
		{http.MethodPut, `{"status":"done"}`, testHandler.UpdateIssue},
		{http.MethodPut, `{"assignee_id":null}`, testHandler.UpdateIssue},
		{http.MethodDelete, ``, testHandler.DeleteIssue},
	} {
		r := withURLParam(newRequest(tc.method, "/api/issues/"+issueID, json.RawMessage(tc.body)), "id", issueID)
		testutil.Call(t, tc.handler, r).Want(http.StatusConflict)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issueID).Scan(&status)
	if status != "todo" {
		t.Fatalf("legacy mutation changed collaboration status to %s", status)
	}
}

// System status writers such as PR-merge auto-advance never move a
// collaboration issue (R19/R35), even when their own policy would allow it.
func TestSystemStatusWriterSkipsCollaborationIssue(t *testing.T) {
	ctx := context.Background()
	allow := func(context.Context, *db.Queries, db.Issue) (bool, error) { return true, nil }
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id=$1`, shared)
	issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(shared))
	if err != nil {
		t.Fatalf("load collaboration issue: %v", err)
	}
	if _, _, changed, err := service.WriteIssueStatus(ctx, testHandler.TxStarter, testHandler.Queries, issue, "done", "pull_request_close_aggregate", allow); err != nil || changed {
		t.Fatalf("system writer moved a collaboration issue: changed=%v err=%v", changed, err)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, shared).Scan(&status)
	if status != "in_progress" {
		t.Fatalf("collaboration status changed to %s", status)
	}

	// The same writer and policy still move an ordinary issue.
	ordinary := dbfx.Issue(t, "Ordinary merge target", testutil.Cols{"status": "in_progress"})
	issue, err = testHandler.Queries.GetIssue(ctx, parseUUID(ordinary))
	if err != nil {
		t.Fatalf("load ordinary issue: %v", err)
	}
	if _, updated, changed, err := service.WriteIssueStatus(ctx, testHandler.TxStarter, testHandler.Queries, issue, "done", "pull_request_close_aggregate", allow); err != nil || !changed || updated.Status != "done" {
		t.Fatalf("ordinary issue not advanced: changed=%v status=%s err=%v", changed, updated.Status, err)
	}
}

// Deleting an ordinary parent or project must not rewrite a collaboration
// issue's source structure behind the collaboration workflow (R49).
func TestLegacyDeleteCannotDetachCollaborationIssue(t *testing.T) {
	parent := dbfx.Issue(t, "Ordinary parent", testutil.Cols{"status": "in_progress"})
	project := dbfx.Project(t, "Ordinary project")
	child := collaborationBoundaryIssue(t, testWorkspaceID)
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id=$2, project_id=$3 WHERE id=$1`, child, parent, project)
	// A refused delete must not have cancelled the parent's own running work.
	runtime := dbfx.Runtime(t, "Parent delete runtime")
	agent := dbfx.Agent(t, "Parent delete agent", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": parent, "status": "running", "runtime_id": runtime})

	for _, call := range []*testutil.Response{
		testutil.Call(t, testHandler.DeleteIssue, withURLParam(newRequest(http.MethodDelete, "/api/issues/"+parent, nil), "id", parent)),
		testutil.Call(t, testHandler.BatchDeleteIssues, newRequest(http.MethodPost, "/api/issues/batch-delete",
			json.RawMessage(fmt.Sprintf(`{"issue_ids":[%q]}`, parent)))),
	} {
		if body := call.Want(http.StatusConflict).Map(); body["code"] != "issue_has_collaboration_children" {
			t.Fatalf("parent delete refusal: %v", body)
		}
	}
	if body := testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest(http.MethodDelete, "/api/projects/"+project, nil), "id", project)).Want(http.StatusConflict).Map(); body["code"] != "project_has_collaboration_issues" {
		t.Fatalf("project delete refusal: %v", body)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='running'`, task); n != 1 {
		t.Fatal("a refused delete cancelled the parent's running task")
	}

	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id=$2 AND project_id=$3`, child, parent, project); n != 1 {
		t.Fatal("legacy delete rewrote the collaboration issue's parent or project")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, parent) + dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, project); n != 2 {
		t.Fatal("a refused delete still removed the parent or project")
	}
}

func TestLegacyMixedBatchRefusesBeforeAnyCollaborationSideEffects(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	ordinary := dbfx.Issue(t, "Ordinary batch companion")
	for _, tc := range []struct {
		path, body string
		handler    http.HandlerFunc
	}{
		{"batch-update", fmt.Sprintf(`{"issue_ids":[%q,%q],"updates":{"status":"done"}}`, ordinary, shared), testHandler.BatchUpdateIssues},
		{"batch-delete", fmt.Sprintf(`{"issue_ids":[%q,%q]}`, ordinary, shared), testHandler.BatchDeleteIssues},
	} {
		testutil.Call(t, tc.handler, newRequest(http.MethodPost, "/api/issues/"+tc.path, json.RawMessage(tc.body))).Want(http.StatusConflict)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='todo'`, []string{ordinary, shared}); n != 2 {
		t.Fatalf("mixed batch partially changed/deleted issues: %d", n)
	}
}

func TestWorkspaceWithCollaborationHistoryCannotBeDeleted(t *testing.T) {
	for _, status := range []string{"todo", "done", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
			h := collaborationFileHandler(t)
			ws := dbfx.Workspace(t, "Retained collaboration workspace", "collab-retained-"+uuid.NewString())
			dbfx.Member(t, ws, testUserID, "owner")
			issue := collaborationBoundaryIssue(t, ws)
			external := dbfx.User(t, "External retained resource owner", "retained-owner-"+uuid.NewString()+"@example.test")
			execution := dbfx.Workspace(t, "Retained private execution workspace", "retained-execution-"+uuid.NewString())
			dbfx.Member(t, execution, external, "owner")
			dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
			for _, user := range []string{testUserID, external} {
				dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
			}
			for _, table := range []string{"comment", "collaboration_event", "collaboration_notification", "collaboration_resource_offer", "collaboration_grant"} {
				dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
			}
			dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
			dbfx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue)
			dbfx.Cleanup(t, `DELETE FROM collaboration_target WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue)
			var owned []string
			t.Cleanup(func() {
				if _, err := testPool.Exec(context.Background(), `DELETE FROM collaboration_config_invalidation WHERE resource_id=ANY($1::uuid[])`, owned); err != nil {
					t.Errorf("cleanup retained resource configuration intents: %v", err)
				}
			})
			fx := testutil.New(testPool, execution, external)
			runtime := fx.Runtime(t, "Retained private offline runtime", testutil.Cols{"status": "offline"})
			agent := fx.Agent(t, "Retained private agent", runtime)
			owned = []string{runtime, agent}
			installation := uuid.NewString()
			binding := fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": external,
				"execution_workspace_id": execution, "runtime_id": runtime, "installation_id": installation, "public_key": make([]byte, 32)})

			// Persist actual offer/adoption/owner consent without granting the
			// source organizer membership in the private execution workspace.
			offerBody := switchHTTPBody()
			offerBody["execution_workspace_id"], offerBody["kind"], offerBody["resource_id"] = execution, "agent", agent
			offerBody["roster"] = []map[string]string{{"role": "agent", "agent_id": agent, "runtime_id": runtime, "owner_user_id": external, "execution_workspace_id": execution}}
			var offered, adopted, consented service.CollaborationQueueReceipt
			testutil.Call(t, h.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, issue, "", external, uuid.NewString(), offerBody)).Want(http.StatusCreated).JSON(&offered)
			if offered.OfferID == nil {
				t.Fatal("retained resource offer missing its identity")
			}
			adoptBody := switchHTTPBody()
			adoptBody["expected_offer_version"], adoptBody["expected_target_set_version"] = "1", "1"
			testutil.Call(t, h.AdoptCollaborationResourceOffer, withURLParams(collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), adoptBody),
				"offerId", *offered.OfferID)).Want(http.StatusCreated).JSON(&adopted)
			consentBody := switchHTTPBody()
			consentBody["agent_id"], consentBody["expected_target_epoch"], consentBody["expected_config_version"] = agent, "1", "1"
			consentBody["identity"] = map[string]string{"kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": "1"}
			testutil.Call(t, h.ConsentCollaborationTarget, withURLParams(collaborationContentRequest(http.MethodPost, issue, "", external, uuid.NewString(), consentBody),
				"targetId", uuidToString(adopted.TargetID))).Want(http.StatusCreated).JSON(&consented)
			if !consented.GrantID.Valid || dbfx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND issue_id=$2 AND target_id=$3
				AND owner_user_id=$4 AND execution_workspace_id=$5 AND agent_id=$6 AND runtime_id=$7 AND binding_id=$8 AND revoked_at IS NULL`,
				consented.GrantID, issue, adopted.TargetID, external, execution, agent, runtime, binding) != 1 {
				t.Fatal("refusal fixture lacks the actual resource owner's target grant")
			}

			draft, attachment := collaborationUploadedFile(t, h, issue, external)
			var comment service.CollaborationCommentReceipt
			testutil.Call(t, h.CreateCollaborationComment, collaborationContentRequest(http.MethodPost, issue, "", external, uuid.NewString(),
				map[string]any{"content": "Retained shared discussion", "expected_lifecycle_epoch": "1", "draft_id": draft,
					"attachment_ids": []string{uuidToString(attachment.ID)}})).Want(http.StatusCreated).JSON(&comment)
			if dbfx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id=$2 AND uploader_user_id=$3
				AND source_workspace_id=$4 AND encryption_key IS NOT NULL AND delivered_at IS NOT NULL`, attachment.ID, issue, external, ws) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND removed_at IS NULL
					AND ((reference_kind='issue' AND reference_id=$2) OR (reference_kind='comment' AND reference_id=$3))`, attachment.ID, issue, comment.CommentID) != 2 {
				t.Fatal("refusal fixture lacks a delivered encrypted attachment and its issue/comment references")
			}
			if dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role='owner'`, ws, testUserID) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role='owner'`, execution, external) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM member WHERE (workspace_id=$1 AND user_id=$2) OR (workspace_id=$3 AND user_id=$4)`, ws, external, execution, testUserID) != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=ANY($2::uuid[]) AND revoked_at IS NULL`, issue, []string{testUserID, external}) != 2 {
				t.Fatal("refusal fixture does not preserve separate source membership, external participation and execution ownership")
			}
			dbfx.Exec(t, `UPDATE issue SET status=$2 WHERE id=$1`, issue, status)
			for _, viewer := range []string{testUserID, external} {
				if response := testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, issue, "", uuidToString(attachment.ID), viewer, nil)).Want(http.StatusOK); response.Text() != "controlled private bytes" {
					t.Fatal("retained attachment fixture has no readable private blob")
				}
			}
			snapshot := func() string {
				var result string
				dbfx.QueryRow(t, `SELECT jsonb_build_object(
					'workspace',(SELECT to_jsonb(w) FROM workspace w WHERE w.id=$1),
					'execution_workspace',(SELECT to_jsonb(w) FROM workspace w WHERE w.id=$3),
					'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$2),
					'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$2),
					'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$2),
					'accounts',(SELECT jsonb_agg(to_jsonb(u) ORDER BY u.id) FROM "user" u WHERE u.id=ANY($4::uuid[])),
					'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.workspace_id,m.user_id) FROM member m WHERE m.workspace_id IN ($1,$3)),
					'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$2),
					'account_fences',(SELECT jsonb_agg(to_jsonb(f) ORDER BY f.user_id) FROM collaboration_account_fence f WHERE f.user_id=ANY($4::uuid[])),
					'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$2)),
					'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$2),
					'enrollments',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM collaboration_runtime_enrollment e WHERE e.id IN (SELECT binding_id FROM collaboration_grant WHERE issue_id=$2)),
					'agents',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM agent a WHERE a.id IN (SELECT agent_id FROM collaboration_grant WHERE issue_id=$2)),
					'runtimes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM agent_runtime r WHERE r.id IN (SELECT runtime_id FROM collaboration_grant WHERE issue_id=$2)),
					'offers',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_resource_offer o WHERE o.issue_id=$2),
					'attachments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM collaboration_attachment a WHERE a.issue_id=$2),
					'attachment_references',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.attachment_id,r.reference_kind,r.reference_id,r.reference_revision)
						FROM collaboration_attachment_reference r WHERE r.attachment_id IN (SELECT id FROM collaboration_attachment WHERE issue_id=$2)),
					'comments',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM comment c WHERE c.issue_id=$2),
					'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$2),
					'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$2)
					)::text`, ws, issue, execution, []string{testUserID, external}).Scan(&result)
				return result
			}
			before := snapshot()
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(before), &fields); err != nil {
				t.Fatal(err)
			}
			for _, relation := range []string{"accounts", "members", "participants", "account_fences", "targets", "grants", "enrollments", "agents", "runtimes", "offers", "attachments", "attachment_references", "comments", "events", "operations"} {
				var rows []json.RawMessage
				if err := json.Unmarshal(fields[relation], &rows); err != nil || len(rows) == 0 {
					t.Fatalf("refusal snapshot lacks seeded %s evidence: %v", relation, err)
				}
			}
			r := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+ws, nil), "id", ws)
			response := testutil.Call(t, h.DeleteWorkspace, r).Want(http.StatusConflict).Map()
			if response["code"] != "workspace_has_collaboration_records" || snapshot() != before {
				t.Fatal("source deletion bypassed the history guard or partially removed retained data")
			}
			for _, viewer := range []string{testUserID, external} {
				if response := testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, issue, "", uuidToString(attachment.ID), viewer, nil)).Want(http.StatusOK); response.Text() != "controlled private bytes" {
					t.Fatal("refused source deletion removed private blob bytes or either current read basis")
				}
			}
		})
	}
}

func TestCollaborationPositionUsesSourceAuthorityWithoutChangingCollaborationVersion(t *testing.T) {
	id := collaborationBoundaryIssue(t, testWorkspaceID)
	r := withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, json.RawMessage(`{"position":-42}`)), "id", id)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	testutil.Call(t, testHandler.UpdateIssue, r).Want(http.StatusOK)
	var position float64
	var revision int64
	dbfx.QueryRow(t, `SELECT i.position,c.revision FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1`, id).Scan(&position, &revision)
	if position != -42 || revision != 1 {
		t.Fatalf("layout change: position=%v collaboration_revision=%d", position, revision)
	}
	r = withURLParam(newRequest(http.MethodPut, "/api/issues/"+id, json.RawMessage(`{"position":-50}`)), "id", id)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.UpdateIssue, r).Want(http.StatusForbidden)
}

func TestLegacyCollaborationCommentAndRerunPathsStayInert(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, issue, "Retained comment")
	for _, tc := range []struct {
		method, path, param, id string
		body                    any
		handler                 http.HandlerFunc
	}{
		{http.MethodPost, "/api/issues/" + issue + "/comments", "id", issue, map[string]any{"content": "Do not enqueue this"}, testHandler.CreateComment},
		{http.MethodPost, "/api/issues/" + issue + "/rerun", "id", issue, nil, testHandler.RerunIssue},
		{http.MethodPut, "/api/comments/" + comment, "commentId", comment, map[string]any{"content": "Do not overwrite"}, testHandler.UpdateComment},
		{http.MethodDelete, "/api/comments/" + comment, "commentId", comment, nil, testHandler.DeleteComment},
		{http.MethodPost, "/api/comments/" + comment + "/resolve", "commentId", comment, nil, testHandler.ResolveComment},
	} {
		r := withURLParam(newRequest(tc.method, tc.path, tc.body), tc.param, tc.id)
		testutil.Call(t, tc.handler, r).Want(http.StatusConflict)
	}
	var content string
	dbfx.QueryRow(t, `SELECT content FROM comment WHERE id=$1`, comment).Scan(&content)
	if content != "Retained comment" {
		t.Fatal("legacy route changed retained collaboration comment")
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("legacy route enqueued ordinary AI work")
	}
}

func TestLegacyIssueDetailDoesNotExposeCollaborationToMachine(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	r := withURLParam(newRequest(http.MethodGet, "/api/issues/"+issue, nil), "id", issue)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.GetIssue, r).Want(http.StatusNotFound)
	r = withURLParam(newRequest(http.MethodGet, "/api/issues/"+issue, nil), "id", issue)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	testutil.Call(t, testHandler.GetIssue, r).Want(http.StatusOK)
}

func TestCollaborationMachineSearchAndListKeepOnlyReferences(t *testing.T) {
	shared := collaborationBoundaryIssue(t, testWorkspaceID)
	ordinary := dbfx.Issue(t, "Ordinary searchable control")
	secret := "private-description-" + uuid.NewString()
	commentSecret := "private-comment-" + uuid.NewString()
	title := "Reference-" + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET title=$2,description=$3,metadata='{"private":"value"}' WHERE id=$1`, shared, title, secret)
	dbfx.Exec(t, `UPDATE issue SET description=$2 WHERE id=$1`, ordinary, secret)
	dbfx.Comment(t, shared, commentSecret)
	type page struct {
		Issues []map[string]json.RawMessage `json:"issues"`
		Total  int64                        `json:"total"`
	}
	call := func(t *testing.T, kind auth.CredentialKind, handler http.HandlerFunc, path string) (page, string) {
		t.Helper()
		r := newRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Actor-Source", "jwt")
		if kind != "missing_context" {
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
		}
		var out page
		response := testutil.Call(t, handler, r).Want(http.StatusOK)
		response.JSON(&out)
		return out, response.Body.String()
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "", "future_machine", "missing_context"} {
		t.Run(string(kind), func(t *testing.T) {
			for _, query := range []string{secret, commentSecret} {
				out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(query))
				want := int64(0)
				if query == secret {
					want = 1
				}
				if out.Total != want || int64(len(out.Issues)) != want || strings.Contains(body, shared) {
					t.Fatalf("private text changed machine search matches/count: %s", body)
				}
			}
			out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(title))
			if out.Total != 1 || len(out.Issues) != 1 {
				t.Fatalf("title reference missing: %s", body)
			}
			for _, endpoint := range []struct {
				path    string
				handler http.HandlerFunc
			}{
				{"/api/issues/search?q=" + url.QueryEscape(title), testHandler.SearchIssues},
				{"/api/issues", testHandler.ListIssues},
				{"/api/issues?open_only=true", testHandler.ListIssues},
			} {
				out, body = call(t, kind, endpoint.handler, endpoint.path)
				found := false
				for _, item := range out.Issues {
					if string(item["id"]) != fmt.Sprintf("%q", shared) {
						continue
					}
					found = true
					if string(item["collaboration_reference"]) != "true" || string(item["can_open"]) != "false" {
						t.Fatalf("machine did not receive a closed reference: %s", body)
					}
					for key := range item {
						switch key {
						case "id", "workspace_id", "number", "identifier", "title", "collaboration_reference", "can_open":
						default:
							t.Fatalf("reference exposed private field %q: %s", key, body)
						}
					}
				}
				if !found {
					t.Fatalf("machine list lost title reference: %s", body)
				}
			}
			for _, filter := range []struct {
				path string
				want int64
			}{
				{"/api/issues?metadata=" + url.QueryEscape(`{"private":"value"}`), 0},
				{"/api/issues?open_only=true&metadata=" + url.QueryEscape(`{"private":"value"}`), 0},
				{"/api/issues?assignee_id=" + testUserID, 0},
				// The ordinary todo control remains visible; only the shared
				// task is excluded from private status filtering.
				{"/api/issues?status=todo", 1},
			} {
				out, body = call(t, kind, testHandler.ListIssues, filter.path)
				if strings.Contains(body, shared) || out.Total != filter.want || int64(len(out.Issues)) != filter.want {
					t.Fatalf("private filter changed permitted results at %s: %s", filter.path, body)
				}
				if filter.want == 1 && string(out.Issues[0]["id"]) != fmt.Sprintf("%q", ordinary) {
					t.Fatalf("private filter returned something other than the ordinary control: %s", body)
				}
			}
			_, body = call(t, kind, testHandler.ListGroupedIssues, "/api/issues/grouped?group_by=assignee")
			if strings.Contains(body, shared) || strings.Contains(body, title) {
				t.Fatalf("private grouping revealed collaboration: %s", body)
			}
		})
	}
	for _, kind := range []auth.CredentialKind{auth.CredentialJWT, auth.CredentialPAT} {
		out, body := call(t, kind, testHandler.SearchIssues, "/api/issues/search?q="+url.QueryEscape(secret))
		if out.Total != 2 || !strings.Contains(body, shared) || !strings.Contains(body, secret) {
			t.Fatalf("human content search changed: %s", body)
		}
	}
}

func TestCollaborationMachineReferencesDoNotRevealStatusOrPrivateOrdering(t *testing.T) {
	first := collaborationBoundaryIssue(t, testWorkspaceID)
	second := collaborationBoundaryIssue(t, testWorkspaceID)
	title := "Stable reference " + uuid.NewString()
	dbfx.Exec(t, `UPDATE issue SET title=$2 WHERE id=ANY($1::uuid[])`, []string{first, second}, title)
	for _, endpoint := range []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/api/issues?sort=updated_at", testHandler.ListIssues},
		{"/api/issues?open_only=true", testHandler.ListIssues},
		{"/api/issues/search?q=" + url.QueryEscape(title), testHandler.SearchIssues},
	} {
		request := func() *http.Request {
			r := newRequest(http.MethodGet, endpoint.path, nil)
			return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
		}
		before := testutil.Call(t, endpoint.handler, request()).Want(http.StatusOK).Body.String()
		dbfx.Exec(t, `UPDATE issue SET status='done',updated_at=now()+interval '1 day',position=-1000 WHERE id=$1`, first)
		after := testutil.Call(t, endpoint.handler, request()).Want(http.StatusOK).Body.String()
		if before != after {
			t.Fatalf("private status/order changed reference window at %s: before=%s after=%s", endpoint.path, before, after)
		}
		dbfx.Exec(t, `UPDATE issue SET status='todo',updated_at=now(),position=0 WHERE id=$1`, first)
	}
}
