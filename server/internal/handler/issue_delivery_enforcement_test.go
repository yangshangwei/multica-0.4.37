package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func ordinaryDeliveryPolicy(t *testing.T, mode string) string {
	t.Helper()
	project := dbfx.Project(t, "Ordinary delivery "+mode)
	// Public enforce activation remains closed until every writer is covered.
	// This fixture installs the actual persisted policy for writer-level tests.
	dbfx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "revision": int64(1), "policy_version": deliveryEvidencePolicy, "mode": mode, "stage": "done", "reason": "writer acceptance fixture", "created_by": testUserID}, "project_id=$1", project)
	return project
}
func ordinaryDeliveryRequest(method, id string, body any) *http.Request {
	r := testutil.WithURLParams(newRequest(method, "/api/issues/"+id, body), "id", id)
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
}
func ordinaryDeliveryIssue(t *testing.T, id string) db.Issue {
	t.Helper()
	issue, err := testHandler.Queries.GetIssueInWorkspace(t.Context(), db.GetIssueInWorkspaceParams{ID: parseUUID(id), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	return issue
}

func TestOrdinaryDeliveryCompletionMissingAuthorization(t *testing.T) {
	project := ordinaryDeliveryPolicy(t, "enforce")
	for _, endpoint := range []string{"update", "move"} {
		t.Run(endpoint, func(t *testing.T) {
			id := dbfx.Issue(t, "missing completion authorization", testutil.Cols{"project_id": project})
			body := map[string]any{"status": "done"}
			call := testHandler.UpdateIssue
			if endpoint == "move" {
				body["before_id"] = nil
				body["after_id"] = nil
				call = testHandler.MoveIssue
			}
			testutil.Call(t, call, ordinaryDeliveryRequest("PUT", id, body)).Want(http.StatusPreconditionRequired)
			if issue := ordinaryDeliveryIssue(t, id); issue.Status != "todo" {
				t.Fatalf("refused completion changed status to %s", issue.Status)
			}
		})
	}
}
func TestOrdinaryDeliveryCreateDonePolicy(t *testing.T) {
	for _, mode := range []string{"enforce", "observe", "off"} {
		t.Run(mode, func(t *testing.T) {
			project := ordinaryDeliveryPolicy(t, mode)
			var createdParent IssueResponse
			testutil.Call(t, testHandler.CreateIssue, ordinaryDeliveryRequest("POST", "", map[string]any{"title": "creation policy parent " + uuid.NewString(), "project_id": project})).Want(http.StatusCreated).JSON(&createdParent)
			parent := createdParent.ID
			dbfx.Cleanup(t, "DELETE FROM issue WHERE id=$1", parent)
			title := "create done " + uuid.NewString()
			want := http.StatusCreated
			if mode == "enforce" {
				want = http.StatusConflict
			}
			var out IssueResponse
			testutil.Call(t, testHandler.CreateIssue, ordinaryDeliveryRequest("POST", "", map[string]any{"title": title, "status": "done", "parent_issue_id": parent})).Want(want).JSON(&out)
			if mode == "enforce" {
				if n := dbfx.Count(t, "SELECT count(*) FROM issue WHERE title=$1", title); n != 0 {
					t.Fatalf("refused create left %d issues", n)
				}
			} else {
				if out.Status != "done" || out.ProjectID == nil || *out.ProjectID != project {
					t.Fatalf("legacy create changed: %+v", out)
				}
				dbfx.Cleanup(t, "DELETE FROM issue WHERE id=$1", out.ID)
			}
		})
	}
}
func TestOrdinaryDeliveryBatchCommittedPrefix(t *testing.T) {
	enforced := ordinaryDeliveryPolicy(t, "enforce")
	ids := []string{dbfx.Issue(t, "batch accepted prefix"), dbfx.Issue(t, "batch refused middle", testutil.Cols{"project_id": enforced}), dbfx.Issue(t, "batch unattempted suffix")}
	var out struct {
		Updated int `json:"updated"`
		Results []struct {
			Index   int    `json:"index"`
			IssueID string `json:"issue_id"`
			State   string `json:"state"`
			Code    string `json:"code"`
		} `json:"results"`
	}
	testutil.Call(t, testHandler.BatchUpdateIssues, ordinaryDeliveryRequest("POST", "batch-update", map[string]any{"issue_ids": ids, "updates": map[string]any{"status": "done"}})).Want(http.StatusPreconditionRequired).JSON(&out)
	if out.Updated != 1 || len(out.Results) != 3 {
		t.Fatalf("lost committed prefix: %+v", out)
	}
	for i, want := range []string{"committed", "rejected", "not_attempted"} {
		if out.Results[i].Index != i || out.Results[i].IssueID != ids[i] || out.Results[i].State != want {
			t.Fatalf("wrong item %d: %+v", i, out.Results[i])
		}
		status := "todo"
		if i == 0 {
			status = "done"
		}
		if got := ordinaryDeliveryIssue(t, ids[i]).Status; got != status {
			t.Fatalf("item %d status=%s want=%s", i, got, status)
		}
	}
}
func TestOrdinaryDeliveryProjectChangeBoundaries(t *testing.T) {
	project := ordinaryDeliveryPolicy(t, "enforce")
	t.Run("reason_required", func(t *testing.T) {
		id := dbfx.Issue(t, "no implicit policy exit", testutil.Cols{"project_id": project})
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": nil})).Want(http.StatusPreconditionRequired)
		if !ordinaryDeliveryIssue(t, id).ProjectID.Valid {
			t.Fatal("missing reason detached issue")
		}
	})
	t.Run("done_requires_prior_reopen", func(t *testing.T) {
		id := dbfx.Issue(t, "done migration", testutil.Cols{"project_id": project, "status": "done"})
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": nil, "project_change_reason": "approved move"})).Want(http.StatusConflict)
		if !ordinaryDeliveryIssue(t, id).ProjectID.Valid {
			t.Fatal("done migration detached issue")
		}
	})
	t.Run("move_and_completion_are_separate", func(t *testing.T) {
		id := dbfx.Issue(t, "combined migration", testutil.Cols{"project_id": project})
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": nil, "status": "done", "project_change_reason": "approved move"})).Want(http.StatusConflict)
		current := ordinaryDeliveryIssue(t, id)
		if !current.ProjectID.Valid || current.Status != "todo" {
			t.Fatal("combined migration partially applied")
		}
	})
	t.Run("deletion_requires_explicit_policy_off", func(t *testing.T) {
		r := testutil.WithURLParams(ordinaryDeliveryRequest("DELETE", project, nil), "id", project)
		testutil.Call(t, testHandler.DeleteProject, r).Want(http.StatusConflict)
		if _, err := testHandler.Queries.GetProjectInWorkspace(context.Background(), db.GetProjectInWorkspaceParams{ID: parseUUID(project), WorkspaceID: parseUUID(testWorkspaceID)}); err != nil {
			t.Fatal("refused deletion removed project", err)
		}
	})
}

func ordinaryAuthorizedCompletion(t *testing.T) (*deliveryPolicyFixture, service.DeliveryCompletionBinding) {
	t.Helper()
	f := newDeliveryPolicyFixture(t, false)
	dbfx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.ws, "project_id": f.project, "revision": int64(1), "policy_version": deliveryEvidencePolicy, "mode": "enforce", "stage": "done", "reason": "actual completion writer fixture", "created_by": f.actor}, "project_id=$1", f.project)
	current, err := f.h.Queries.GetIssue(t.Context(), parseUUID(f.artifactFixture.req.ResourceID))
	if err != nil {
		t.Fatal(err)
	}
	request := service.DeliveryCompletionAuthorizationRequest{
		ContractVersion: service.DeliveryCompletionContract, SourceWorkspaceID: f.ws, OperationID: uuid.NewString(),
		Target:   service.DeliveryCompletionTarget{Kind: "ordinary", IssueID: f.artifactFixture.req.ResourceID, ExpectedIssueRevision: current.Revision, ExpectedStatus: current.Status, TargetStatus: "done"},
		Project:  service.DeliveryCompletionProject{ID: f.project, ExpectedRevision: *f.req.ProjectRevision, ExpectedPolicyRevision: 1},
		Evidence: service.DeliveryCompletionEvidence{IssueID: f.artifactFixture.req.ResourceID, ExpectedIssueRevision: current.Revision, ApprovalID: f.req.ApprovalID, ExpectedSourceRevision: f.req.SourceRevision, ExpectedSpecDigest: f.req.SpecDigest, ExpectedArtifactDigest: f.req.ArtifactDigest},
		Consumer: service.DeliveryCompletionConsumer{Kind: "human", UserID: f.actor}, ExpiresAt: time.Now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond), Reason: "Human accepts this exact actual archive",
	}
	var grant service.DeliveryCompletionAuthorizationView
	testutil.Call(t, f.h.CreateDeliveryCompletionAuthorization, f.request("POST", "/api/delivery-completion-authorizations", request)).Want(201).JSON(&grant)
	if grant.AuthorizationID == "" || grant.State != "issued" {
		t.Fatalf("unexpected authorization: %+v", grant)
	}
	dbfx.Cleanup(t, "DELETE FROM delivery_completion_authorization WHERE id=$1", grant.AuthorizationID)
	dbfx.Cleanup(t, "DELETE FROM delivery_completion_terminal WHERE authorization_id=$1", grant.AuthorizationID)
	return f, service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: grant.OperationID, ExpectedIssueRevision: current.Revision}
}
func ordinaryAuthorizedRequest(f *deliveryPolicyFixture, body any) *http.Request {
	return testutil.WithURLParams(f.request("PUT", "/api/issues/"+f.artifactFixture.req.ResourceID, body), "id", f.artifactFixture.req.ResourceID)
}
func TestOrdinaryDeliveryActualConsumptionAndReplay(t *testing.T) {
	f, binding := ordinaryAuthorizedCompletion(t)
	body := map[string]any{"status": "done", "expected_revision": binding.ExpectedIssueRevision, "completion": binding}
	var done IssueResponse
	testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, body)).Want(200).JSON(&done)
	if done.Status != "done" || done.CompletionReceipt == nil || done.CompletionReceipt.Replayed || done.CompletionReceipt.AuthorizationID != binding.AuthorizationID {
		t.Fatalf("state and receipt did not commit: %+v", done)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed' AND issue_revision_after=$2", binding.AuthorizationID, done.Revision); n != 1 {
		t.Fatalf("exact durable consumption count=%d", n)
	}
	var edited IssueResponse
	testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, map[string]any{"title": "post-completion annotation"})).Want(200).JSON(&edited)
	if edited.Status != "done" || edited.CompletionReceipt != nil {
		t.Fatal("post-completion edit created a new acceptance")
	}
	var reopened IssueResponse
	testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, map[string]any{"status": "todo"})).Want(200).JSON(&reopened)
	var replay IssueResponse
	testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, body)).Want(200).JSON(&replay)
	if replay.Status != "todo" || replay.Title != edited.Title || replay.Revision != reopened.Revision || replay.CompletionReceipt == nil || !replay.CompletionReceipt.Replayed || replay.CompletionReceipt.CompletedRevision != done.Revision {
		t.Fatalf("replay rewrote current state or lost original receipt: %+v", replay)
	}
	testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, map[string]any{"status": "done"})).Want(428)
	if n := dbfx.Count(t, "SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1", binding.AuthorizationID); n != 1 {
		t.Fatal("replay consumed again")
	}
}
func TestOrdinaryDeliveryMixedCompletionRefusesActualChanges(t *testing.T) {
	for _, kind := range []string{"title", "attachment"} {
		t.Run(kind, func(t *testing.T) {
			f, binding := ordinaryAuthorizedCompletion(t)
			body := map[string]any{"status": "done", "completion": binding}
			attachment := ""
			if kind == "title" {
				body["title"] = "unreviewed title"
			} else {
				attachment = dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": f.ws, "uploader_type": "member", "uploader_id": f.actor, "filename": "new.txt", "url": "https://example.invalid/new.txt", "content_type": "text/plain", "size_bytes": int64(3)})
				body["attachment_ids"] = []string{attachment}
			}
			testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, body)).Want(409)
			current, err := f.h.Queries.GetIssue(t.Context(), parseUUID(f.artifactFixture.req.ResourceID))
			if err != nil {
				t.Fatal(err)
			}
			if current.Status == "done" || current.Revision != binding.ExpectedIssueRevision {
				t.Fatal("mixed refusal left partial change")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1", binding.AuthorizationID); n != 0 {
				t.Fatal("mixed refusal consumed authorization")
			}
			if attachment != "" && dbfx.Count(t, "SELECT count(*) FROM attachment WHERE id=$1 AND issue_id IS NULL", attachment) != 1 {
				t.Fatal("mixed refusal linked new attachment")
			}
			testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, map[string]any{"status": "done", "title": current.Title, "completion": binding})).Want(200)
		})
	}
}
func TestOrdinaryDeliveryBatchUnknownCommitRecovery(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			f, binding := ordinaryAuthorizedCompletion(t)
			h := f.h
			fault := &artifactFaultStarter{txStarter: h.TxStarter, failAt: 1, after: after}
			h.TxStarter = fault
			suffix := dbfx.Issue(t, "unknown suffix", testutil.Cols{"workspace_id": f.ws, "creator_id": f.actor})
			body := map[string]any{"issue_ids": []string{f.artifactFixture.req.ResourceID, suffix}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{{"issue_id": f.artifactFixture.req.ResourceID, "authorization_id": binding.AuthorizationID, "operation_id": binding.OperationID, "expected_issue_revision": binding.ExpectedIssueRevision}}}
			var response BatchUpdateIssuesResponse
			testutil.Call(t, h.BatchUpdateIssues, f.request("POST", "/api/issues/batch-update", body)).Want(503).JSON(&response)
			if fault.count != 1 || response.Updated != 0 || len(response.Results) != 2 || response.Results[0].State != "unknown" || response.Results[1].State != "not_attempted" {
				t.Fatalf("unknown commit retried or misreported: %+v attempts=%d", response, fault.count)
			}
			count := dbfx.Count(t, "SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1", binding.AuthorizationID)
			want := 0
			if after {
				want = 1
			}
			if count != want {
				t.Fatalf("actual commit witness=%d want%d", count, want)
			}
			var stored service.DeliveryCompletionAuthorizationView
			get := testutil.WithURLParams(f.request("GET", "/api/delivery-completion-authorizations/"+binding.AuthorizationID, nil), "authorizationId", binding.AuthorizationID)
			testutil.Call(t, f.h.GetDeliveryCompletionAuthorization, get).Want(200).JSON(&stored)
			if after {
				if stored.CompletionReceipt == nil {
					t.Fatal("committed outcome not recoverable")
				}
				var replay IssueResponse
				testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f, map[string]any{"status": "done", "completion": binding})).Want(200).JSON(&replay)
				if replay.CompletionReceipt == nil || !replay.CompletionReceipt.Replayed {
					t.Fatal("original completion not replayed")
				}
			} else if stored.CompletionReceipt != nil {
				t.Fatal("rolled-back completion fabricated receipt")
			}
			var status string
			dbfx.QueryRow(t, "SELECT status FROM issue WHERE id=$1", suffix).Scan(&status)
			if status != "todo" {
				t.Fatal("unattempted suffix changed")
			}
		})
	}
}
func TestOrdinaryDeliveryCustomAndCommentChild(t *testing.T) {
	project := ordinaryDeliveryPolicy(t, "enforce")
	custom := createTestCustomStatus(t, "custom_delivery_done", "done")
	t.Run("custom_done_requires_authorization", func(t *testing.T) {
		id := dbfx.Issue(t, "custom completion", testutil.Cols{"project_id": project})
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"status": custom.Key})).Want(428)
	})
	t.Run("archived_done_content_edit_remains_available", func(t *testing.T) {
		id := dbfx.Issue(t, "archived completed", testutil.Cols{"project_id": project, "status": custom.Key})
		dbfx.Exec(t, "UPDATE issue_status SET archived_at=clock_timestamp() WHERE id=$1", custom.ID)
		var out IssueResponse
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"title": "updated annotation"})).Want(200).JSON(&out)
		if out.Status != custom.Key || out.CompletionReceipt != nil {
			t.Fatal("ordinary annotation changed completion history")
		}
		dbfx.Exec(t, "UPDATE issue_status SET archived_at=NULL WHERE id=$1", custom.ID)
	})
	t.Run("comment_child_inherits_enforced_project", func(t *testing.T) {
		var parent IssueResponse
		testutil.Call(t, testHandler.CreateIssue, ordinaryDeliveryRequest("POST", "", map[string]any{"title": "comment parent " + uuid.NewString(), "project_id": project})).Want(201).JSON(&parent)
		dbfx.Cleanup(t, "DELETE FROM issue WHERE id=$1", parent.ID)
		comment := dbfx.Comment(t, parent.ID, "completed source discussion")
		req := testutil.WithURLParams(ordinaryDeliveryRequest("GET", parent.ID, nil), "commentId", comment)
		var preview sourceContextPreviewResponse
		testutil.Call(t, testHandler.PreviewCommentSubIssue, req).Want(200).JSON(&preview)
		title := "refused comment child " + uuid.NewString()
		request := testutil.WithURLParams(ordinaryDeliveryRequest("POST", parent.ID, map[string]any{"mode": "manual", "capture_token": preview.CaptureToken, "issue": map[string]any{"title": title, "status": "done"}}), "commentId", comment)
		testutil.Call(t, testHandler.CreateCommentSubIssue, request).Want(409)
		if dbfx.Count(t, "SELECT count(*) FROM issue WHERE title=$1", title) != 0 {
			t.Fatal("refused comment child persisted")
		}
	})
}
func TestOrdinaryDeliveryControlledProjectExit(t *testing.T) {
	project := ordinaryDeliveryPolicy(t, "enforce")
	t.Run("member_cannot_exit", func(t *testing.T) {
		member := dbfx.User(t, "migration member", "migration-"+uuid.NewString()+"@example.invalid")
		dbfx.Member(t, testWorkspaceID, member, "member")
		id := dbfx.Issue(t, "member exit", testutil.Cols{"project_id": project})
		req := ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": nil, "project_change_reason": "would bypass gate"})
		req.Header.Set("X-User-ID", member)
		req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: member, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
		testutil.Call(t, testHandler.UpdateIssue, req).Want(403)
		if !ordinaryDeliveryIssue(t, id).ProjectID.Valid {
			t.Fatal("member detached enforce issue")
		}
	})
	t.Run("owner_reason_is_retained", func(t *testing.T) {
		id := dbfx.Issue(t, "owner exit", testutil.Cols{"project_id": project})
		dbfx.Cleanup(t, "DELETE FROM delivery_project_change WHERE issue_id=$1", id)
		before := ordinaryDeliveryIssue(t, id)
		testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": nil, "project_change_reason": " Approved scope correction "})).Want(200)
		if ordinaryDeliveryIssue(t, id).ProjectID.Valid {
			t.Fatal("owner correction did not detach")
		}
		var reason, actor string
		var revision int64
		dbfx.QueryRow(t, "SELECT reason,actor_user_id::text,issue_revision FROM delivery_project_change WHERE issue_id=$1 AND from_project_id=$2 AND to_project_id IS NULL", id, project).Scan(&reason, &actor, &revision)
		if reason != "Approved scope correction" || actor != testUserID || revision != before.Revision {
			t.Fatal("scope correction lost original actor/reason/version")
		}
	})
}
func TestOrdinaryDeliveryStrictCompletionControls(t *testing.T) {
	id := dbfx.Issue(t, "strict control")
	key, op := uuid.NewString(), uuid.NewString()
	for _, raw := range []string{
		`{"status":"done","completion":null}`,
		fmt.Sprintf(`{"status":"done","completion":{"authorization_id":%q,"operation_id":%q}}`, key, op),
		fmt.Sprintf(`{"status":"done","completion":{"authorization_id":%q,"operation_id":%q,"expected_issue_revision":1,"expected_issue_revision":1}}`, key, op),
		fmt.Sprintf(`{"status":"done","completion":{"authorization_id":%q,"operation_id":%q,"expected_issue_revision":1},"completion":{"authorization_id":%q,"operation_id":%q,"expected_issue_revision":1}}`, key, op, key, op),
		`{"status":"done","Completion":{}}`,
		`{"project_change_reason":null}`,
	} {
		t.Run(raw, func(t *testing.T) {
			r := ordinaryDeliveryRequest("PUT", id, nil)
			r.Body = io.NopCloser(strings.NewReader(raw))
			testutil.Call(t, testHandler.UpdateIssue, r).Want(400)
		})
	}
	testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"status": "done", "skip": true})).Want(409)
	if ordinaryDeliveryIssue(t, id).Status != "todo" {
		t.Fatal("invalid controls changed issue")
	}
	// A bound request is per item; duplicate/foreign maps and one shared binding reject.
	binding := map[string]any{"authorization_id": key, "operation_id": op, "expected_issue_revision": int64(1)}
	for _, body := range []map[string]any{
		{"issue_ids": []string{id}, "updates": map[string]any{"status": "done", "completion": binding}},
		{"issue_ids": []string{id}, "updates": map[string]any{"status": "done"}, "completion_bindings": []any{map[string]any{"issue_id": uuid.NewString(), "authorization_id": key, "operation_id": op, "expected_issue_revision": int64(1)}}},
	} {
		testutil.Call(t, testHandler.BatchUpdateIssues, ordinaryDeliveryRequest("POST", "batch-update", body)).Want(400)
	}
}

func TestOrdinaryDeliveryBatchCommittedParentNotification(t *testing.T) {
	f := newStagedBatchFixture(t)
	project := ordinaryDeliveryPolicy(t, "enforce")
	refused := dbfx.Issue(t, "refused after committed stage", testutil.Cols{"project_id": project})
	ids := []string{f.stage1[0].ID, f.stage1[1].ID, refused, f.stage2[0].ID, f.stage2[1].ID}
	var out BatchUpdateIssuesResponse
	testutil.Call(t, testHandler.BatchUpdateIssues, ordinaryDeliveryRequest("POST", "batch-update", map[string]any{"issue_ids": ids, "updates": map[string]any{"status": "done"}})).Want(428).JSON(&out)
	if out.Updated != 2 || len(out.Results) != 5 || out.Results[2].State != "rejected" || out.Results[3].State != "not_attempted" {
		t.Fatalf("wrong stage prefix outcome: %+v", out)
	}
	if got := countSystemCommentsOn(t, f.parent.ID); got != 1 {
		t.Fatalf("committed prefix should notify parent once, got%d", got)
	}
	content, _, _, _ := systemCommentOn(t, f.parent.ID)
	if !strings.Contains(content, "Stage 1") || !strings.Contains(content, "Stage 2") {
		t.Fatalf("notification lost actual final stage state: %s", content)
	}
	if got := countPendingTasksForAgent(t, f.parent.ID, f.agentID); got != 1 {
		t.Fatalf("committed prefix should enqueue one parent wake, got%d", got)
	}
	if triggerCommentIDForAgentTask(t, f.parent.ID, f.agentID) != systemCommentIDOn(t, f.parent.ID) {
		t.Fatal("parent wake did not bind actual committed prefix notification")
	}
	for _, child := range f.stage2 {
		if ordinaryDeliveryIssue(t, child.ID).Status != "in_progress" {
			t.Fatal("unattempted stage changed")
		}
	}
}
func TestOrdinaryDeliveryEnteringEnforceProject(t *testing.T) {
	project := ordinaryDeliveryPolicy(t, "enforce")
	for _, status := range []string{"todo", "done"} {
		t.Run(status, func(t *testing.T) {
			id := dbfx.Issue(t, "enter enforce "+status, testutil.Cols{"status": status})
			dbfx.Cleanup(t, "DELETE FROM delivery_project_change WHERE issue_id=$1", id)
			want := 200
			if status == "done" {
				want = 409
			}
			testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"project_id": project})).Want(want)
			current := ordinaryDeliveryIssue(t, id)
			if status == "done" {
				if current.ProjectID.Valid {
					t.Fatal("completed task entered enforce without prior reopen")
				}
			} else {
				if uuidToString(current.ProjectID) != project {
					t.Fatal("ordinary open task could not enter enforce project")
				}
				if dbfx.Count(t, "SELECT count(*) FROM delivery_project_change WHERE issue_id=$1 AND from_project_id IS NULL AND to_project_id=$2", id, project) != 1 {
					t.Fatal("enforce entry adjustment not retained")
				}
				testutil.Call(t, testHandler.UpdateIssue, ordinaryDeliveryRequest("PUT", id, map[string]any{"status": "done"})).Want(428)
			}
		})
	}
}
