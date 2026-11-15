package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func stageWriterWorkflow(t *testing.T, s *stageHTTP, actor string) service.CollaborationWorkflowRequest {
	t.Helper()
	ctx := auth.WithAuthenticatedPrincipal(t.Context(), stagePrincipal(actor))
	a, err := s.svc.ResolveIssueAccess(ctx, nil, parseUUID(s.issue))
	if err != nil {
		t.Fatal(err)
	}
	return service.CollaborationWorkflowRequest{CollaborationQueueRequest: service.CollaborationQueueRequest{
		IssueID: parseUUID(s.issue), RequestID: uuid.NewString(), ExpectedLifecycleEpoch: a.LifecycleEpoch,
		ExpectedParticipantEpoch: a.ParticipantEpoch.Int64, ExpectedAccountEpoch: a.AccountEpoch.Int64}, ExpectedCollaborationRevision: a.Revision}
}

// The ordinary artifact is packaged/approved by the existing fixture. Its root
// and scope are established by real collaboration services, never by a second
// completion implementation or a client-supplied approval marker.
func stageWriterCompletionFixture(t *testing.T, mode string) (*completionFixture, *stageHTTP, service.CollaborationStageCandidateRef) {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCompletionFixture(t)
	if mode != "enforce" {
		policy := f.policy(t, http.MethodPut, deliveryPolicyRequest{1, mode, deliveryEvidencePolicy, deliveryPolicyStage, "Ordinary-child M1 mode"}, http.StatusOK)
		f.request.Project.ExpectedPolicyRevision = policy.Revision
	}
	fx := testutil.New(testPool, f.ws, f.actor)
	var creationEnabled bool
	fx.QueryRow(t, `SELECT creation_enabled FROM collaboration_execution_gate WHERE id=1`).Scan(&creationEnabled)
	fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE collaboration_execution_gate SET creation_enabled=$1 WHERE id=1`, creationEnabled)
	})
	// The artifact fixture inserts its issue through dbfx; align the retained
	// counter before the real collaboration creation allocates another number.
	fx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, f.ws)
	created := testutil.Call(t, f.h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, f.ws, uuid.NewString(), f.actor,
		map[string]any{"source_workspace_id": f.ws, "handler_user_id": f.actor, "title": "Independent ordinary-child stage root", "description": "Synthetic root requirements", "project_id": f.project}, false)).Want(http.StatusCreated).Map()
	root, ok := created["id"].(string)
	if !ok {
		t.Fatal("fixture collaboration creation returned no root identity")
	}
	for _, table := range []string{"issue_handoff", "collaboration_stage_submission", "collaboration_stage_child", "collaboration_stage_revocation", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "collaboration_event", "collaboration_notification", "issue_participant", "collaboration_session", "issue_collaboration"} {
		if table == "collaboration_stage_child" {
			fx.Cleanup(t, `DELETE FROM `+table+` WHERE root_issue_id=$1`, root)
		} else {
			fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, root)
		}
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, root)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, f.ws)
	fx.Cleanup(t, `DELETE FROM project_stage_policy WHERE project_id=$1`, f.project)
	fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, f.actor)
	fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, root)
	testutil.Call(t, f.h.UpdateIssue, testutil.WithURLParams(f.artifactFixture.request(http.MethodPut, "/api/issues/"+f.request.Target.IssueID,
		map[string]any{"parent_issue_id": root}), "id", f.request.Target.IssueID)).Want(http.StatusOK)
	fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.Target.IssueID).Scan(&f.request.Target.ExpectedIssueRevision)
	f.request.Evidence.ExpectedIssueRevision = f.request.Target.ExpectedIssueRevision
	f.req.IssueRevision = f.request.Target.ExpectedIssueRevision
	s := &stageHTTP{h: &f.h, fx: fx, svc: service.NewCollaborationService(f.h.Queries, f.h.TxStarter, f.h.IssueService),
		actor: f.actor, recipient: f.actor, issue: root, project: f.project}
	s.revise(t, true)
	users := make([]string, 3)
	for i := range users {
		users[i] = fx.User(t, "Synthetic stage signer", "writer-signer-"+uuid.NewString()+"@example.invalid")
		fx.Member(t, f.ws, users[i], "admin")
		fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, users[i])
		_, err := s.svc.InviteParticipant(f.artifactFixture.request(http.MethodPost, "/unused", nil).Context(), service.CollaborationParticipantParams{
			CollaborationWorkflowRequest: stageWriterWorkflow(t, s, f.actor), TargetUserID: parseUUID(users[i])})
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx := f.artifactFixture.request(http.MethodPost, "/unused", nil).Context()
	_, err := s.svc.ConfigureStageRoles(ctx, service.CollaborationStageRolesParams{CollaborationWorkflowRequest: stageWriterWorkflow(t, s, f.actor),
		ExpectedPolicyRevision: 1, ExpectedRoleRevision: 0, Roles: service.CollaborationStageRoleSet{BusinessOwner: parseUUID(f.actor), Implementer: parseUUID(f.actor),
			ProductApprover: parseUUID(f.actor), CodeReviewer: parseUUID(users[0]), QAApprover: parseUUID(users[1]), ReleaseApprover: parseUUID(users[2])}})
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": f.ws, "installation_id": int64(1), "account_login": "synthetic-writer", "account_type": "User"})
	fx.Exec(t, `UPDATE github_pull_request SET snapshot_head_sha=head_sha,snapshot_fetched_at=clock_timestamp()-interval '1 minute',snapshot_complete=true WHERE id=$1`, f.pr)
	fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{"pr_id": f.pr, "head_sha": f.artifactFixture.req.SourceRevision,
		"ordinal": int32(0), "name": "Synthetic writer unit tests", "status": "completed", "conclusion": "success"}, "pr_id=$1", f.pr)
	view, err := s.svc.StageView(ctx, parseUUID(root))
	if err != nil {
		t.Fatal(err)
	}
	var contentDigest string
	for _, item := range view.Children.Items {
		if item.IssueID == f.request.Target.IssueID && item.ContentDigest != nil {
			contentDigest = *item.ContentDigest
		}
	}
	if contentDigest == "" {
		t.Fatal("fixture child is not source-readable")
	}
	code := service.CollaborationStageCodeRef{Provider: "github", PullRequestID: parseUUID(f.pr), HeadSHA: f.artifactFixture.req.SourceRevision}
	_, err = s.svc.ConfirmStageScope(ctx, service.CollaborationStageScopeParams{CollaborationWorkflowRequest: stageWriterWorkflow(t, s, f.actor),
		ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedScopeRevision: 0, Reason: "Explicit synthetic child/code scope", Children: []service.CollaborationStageScopeChild{
			{IssueID: parseUUID(f.request.Target.IssueID), ContentDigest: contentDigest, Code: &code}}})
	if err != nil {
		t.Fatal(err)
	}
	createdCode, err := s.svc.CreateStageCodeCandidate(ctx, service.CollaborationStageCodeParams{CollaborationWorkflowRequest: stageWriterWorkflow(t, s, f.actor),
		ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ChildIssueID: parseUUID(f.request.Target.IssueID), PullRequest: code,
		Artifact: &service.CollaborationStageArtifactSelection{ApprovalID: parseUUID(f.request.Evidence.ApprovalID)}})
	if err != nil || createdCode.StageConfirmation == nil {
		t.Fatalf("fixture code candidate: %+v %v", createdCode, err)
	}
	version, err := strconv.ParseInt(createdCode.StageConfirmation.Version, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return f, s, service.CollaborationStageCandidateRef{CandidateID: parseUUID(createdCode.StageConfirmation.CandidateID), Version: version, ContentDigest: createdCode.StageConfirmation.ContentDigest}
}

func stageWriterBinding(grant service.DeliveryCompletionAuthorizationView) map[string]any {
	return map[string]any{"authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "expected_issue_revision": grant.IssueRevision}
}

func TestStageWriterContinuationLegacyRootWritersM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, mode)
			s.revise(t, true)
			ordinary := s.fx.Issue(t, "Unattempted legacy batch companion", testutil.Cols{"project_id": s.project})
			runtime := s.fx.Runtime(t, "Legacy writer caller runtime")
			agent := s.fx.Agent(t, "Legacy writer caller", runtime)
			task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running", "originator_user_id": s.actor, "accountable_user_id": s.actor})
			before := stageExitHTTPState(t, s)
			for _, actor := range []string{"legacy_client", "agent_actor", "task_credential"} {
				r := stageExitHTTPRequest(s, http.MethodPut, "/api/issues/"+s.issue, s.issue, map[string]any{"status": "done"})
				r.Header.Set("X-Client-Version", "0.6.7")
				if actor != "legacy_client" {
					testutil.WithHeaders(r, "X-Agent-ID", agent, "X-Task-ID", task)
					principal := stagePrincipal(s.actor)
					principal.AgentActor = true
					if actor == "task_credential" {
						principal.Kind = auth.CredentialTask
						r.Header.Set("X-Actor-Source", "task_token")
					}
					r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
				}
				if out := testutil.Call(t, s.h.UpdateIssue, r).Want(http.StatusConflict).Map(); out["code"] != "unsupported_collaboration_action" {
					t.Fatalf("%s ordinary PUT reached managed root completion: %v", actor, out)
				}
			}
			move := map[string]any{"status": "done", "before_id": nil, "after_id": nil}
			if out := testutil.Call(t, s.h.MoveIssue, stageExitHTTPRequest(s, http.MethodPost, "/api/issues/"+s.issue+"/move", s.issue, move)).Want(http.StatusConflict).Map(); out["code"] != "unsupported_collaboration_action" {
				t.Fatalf("Move created another root completion entry: %v", out)
			}
			if out := testutil.Call(t, s.h.BatchUpdateIssues, stageExitHTTPRequest(s, http.MethodPost, "/api/issues/batch-update", "",
				map[string]any{"issue_ids": []string{ordinary, s.issue}, "updates": map[string]any{"status": "done"}})).Want(http.StatusConflict).Map(); out["code"] != "unsupported_collaboration_action" {
				t.Fatalf("mixed root batch did not refuse before its ordinary prefix: %v", out)
			}
			if stageExitHTTPState(t, s) != before {
				t.Fatal("legacy PUT/Agent/Move/root batch mutated issue, queue, handoff or execution facts")
			}
		})
	}
}

func TestStageWriterContinuationOrdinaryCompletionAndMoveM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		for _, method := range []string{"put", "move"} {
			t.Run(mode+"/"+method, func(t *testing.T) {
				f, s, candidate := stageWriterCompletionFixture(t, mode)
				body := map[string]any{"status": "done"}
				call, verb, path := f.h.UpdateIssue, http.MethodPut, "/api/issues/"+f.request.Target.IssueID
				if method == "move" {
					call, verb, path = f.h.MoveIssue, http.MethodPost, path+"/move"
					body["before_id"], body["after_id"] = nil, nil
				}
				request := func() *http.Request {
					return testutil.WithURLParams(f.artifactFixture.request(verb, path, body), "id", f.request.Target.IssueID)
				}
				var grant service.DeliveryCompletionAuthorizationView
				if mode == "enforce" {
					out := testutil.Call(t, call, request()).Want(http.StatusPreconditionRequired).Map()
					if out["code"] != "delivery_completion_authorization_required" || s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.request.Target.IssueID) != 0 {
						t.Fatalf("stage replaced ordinary child M1 admission: %v", out)
					}
					if method == "move" {
						var position float64
						s.fx.QueryRow(t, `SELECT position FROM issue WHERE id=$1`, f.request.Target.IssueID).Scan(&position)
						f.request.Target.TargetPosition = &position
					}
					grant = f.create(t, uuid.NewString(), http.StatusCreated)
					body["completion"] = stageWriterBinding(grant)
				}
				testutil.Call(t, call, request()).Want(http.StatusOK)
				var semantic int64
				s.fx.QueryRow(t, `SELECT semantic_version FROM collaboration_stage_child WHERE child_issue_id=$1`, f.request.Target.IssueID).Scan(&semantic)
				evidence, err := s.svc.StageCodeEvidence(f.artifactFixture.request(http.MethodGet, "/unused", nil).Context(), parseUUID(s.issue), candidate.CandidateID)
				if err != nil || evidence.Candidate.State != "current" || semantic != 1 ||
					s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, s.issue) != 1 ||
					s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.request.Target.IssueID) != 1 {
					t.Fatalf("ordinary child completion changed its semantic identity or imposed/advanced root workflow: semantic=%d evidence=%+v err=%v", semantic, evidence.Candidate, err)
				}
				if mode == "enforce" {
					testutil.Call(t, call, request()).Want(http.StatusOK)
					if s.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID) != 1 {
						t.Fatal("ordinary child completion replay consumed another grant or lost its exact terminal")
					}
				}
			})
		}
	}
}

func TestStageWriterContinuationBatchCommittedPrefix(t *testing.T) {
	f, s, _ := stageWriterCompletionFixture(t, "enforce")
	second := s.fx.Issue(t, "Rejected ordinary child in batch", testutil.Cols{"parent_issue_id": s.issue, "project_id": s.project})
	third := s.fx.Issue(t, "Unattempted ordinary child in batch", testutil.Cols{"parent_issue_id": s.issue, "project_id": s.project})
	grant := f.create(t, uuid.NewString(), http.StatusCreated)
	binding := stageWriterBinding(grant)
	binding["issue_id"] = grant.IssueID
	body := map[string]any{"issue_ids": []string{grant.IssueID, second, third}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{binding}}
	var result BatchUpdateIssuesResponse
	testutil.Call(t, f.h.BatchUpdateIssues, f.artifactFixture.request(http.MethodPost, "/api/issues/batch-update", body)).Want(http.StatusPreconditionRequired).JSON(&result)
	if result.Updated != 1 || len(result.Results) != 3 || result.Results[0].State != "committed" || result.Results[0].IssueID != grant.IssueID ||
		result.Results[1].State != "rejected" || result.Results[1].Code != "delivery_completion_authorization_required" || result.Results[2].State != "not_attempted" {
		t.Fatalf("stage-enabled ordinary batch lost its actual committed prefix: %+v", result)
	}
	if s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='done'`, []string{grant.IssueID, second, third}) != 1 ||
		s.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID) != 1 ||
		s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, s.issue) != 1 {
		t.Fatal("batch committed a rejected/unattempted child or completed its stage root")
	}
}

func TestStageWriterContinuationDuplicateBatchBindingsBeforeEffects(t *testing.T) {
	for _, repeated := range []string{"authorization", "operation"} {
		t.Run(repeated, func(t *testing.T) {
			f, s, _ := stageWriterCompletionFixture(t, "enforce")
			other := s.fx.Issue(t, "Duplicate-binding ordinary child", testutil.Cols{"parent_issue_id": s.issue, "project_id": s.project})
			grant := f.create(t, uuid.NewString(), http.StatusCreated)
			authorization, operation := uuid.NewString(), uuid.NewString()
			if repeated == "authorization" {
				authorization = grant.AuthorizationID
			} else {
				operation = grant.OperationID
			}
			var revision int64
			s.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, other).Scan(&revision)
			first := stageWriterBinding(grant)
			first["issue_id"] = grant.IssueID
			body := map[string]any{"issue_ids": []string{grant.IssueID, other}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{
				first, {"issue_id": other, "authorization_id": authorization, "operation_id": operation, "expected_issue_revision": revision}}}
			testutil.Call(t, f.h.BatchUpdateIssues, f.artifactFixture.request(http.MethodPost, "/api/issues/batch-update", body)).Want(http.StatusBadRequest)
			if s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='done'`, []string{grant.IssueID, other}) != 0 ||
				s.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatalf("duplicate %s was detected after an ordinary-child committed prefix", repeated)
			}
		})
	}
}

func TestStageWriterContinuationHumanCreateAndCommentChildM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			s := stageHTTPFixture(t)
			stageExitHTTPM1(t, s, mode)
			s.revise(t, true)
			s.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "key": "writer_http_done", "name": "Writer HTTP done",
				"category": "done", "color": "#000000", "is_system": false, "position": 100})
			runtime := s.fx.Runtime(t, "Machine create caller")
			agent := s.fx.Agent(t, "Machine create caller", runtime)
			task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running", "originator_user_id": s.actor, "accountable_user_id": s.actor})
			for _, status := range []string{"done", "writer_http_done"} {
				before := s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, s.issue)
				body := map[string]any{"title": "Manual create-as-" + status, "status": status, "parent_issue_id": s.issue}
				r := stageExitHTTPRequest(s, http.MethodPost, "/api/issues", "", body)
				want := http.StatusCreated
				if mode == "enforce" {
					want = http.StatusConflict
				}
				out := testutil.Call(t, s.h.CreateIssue, r).Want(want).Map()
				if want == http.StatusConflict && (out["code"] != "delivery_create_done_forbidden" || s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, s.issue) != before) {
					t.Fatalf("ordinary human create-as-done M1 changed: %v", out)
				}
				body["title"] = "Unapproved machine create-as-" + status
				r = stageExitHTTPRequest(s, http.MethodPost, "/api/issues", "", body)
				testutil.WithHeaders(r, "X-Agent-ID", agent, "X-Task-ID", task)
				principal := stagePrincipal(s.actor)
				principal.AgentActor = true
				r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
				if out := testutil.Call(t, s.h.CreateIssue, r).Want(http.StatusConflict).Map(); out["code"] != "stage_proposal_required" {
					t.Fatalf("machine create-as-done acquired human manual/approval authority: %v", out)
				}
			}
			manual := testutil.Call(t, s.h.CreateIssue, stageExitHTTPRequest(s, http.MethodPost, "/api/issues", "",
				map[string]any{"title": "Manual source child", "parent_issue_id": s.issue})).Want(http.StatusCreated).Map()
			source := manual["id"].(string)
			comment := s.fx.Comment(t, source, "Synthetic ordinary child discussion")
			s.fx.Cleanup(t, `DELETE FROM issue_source_context WHERE anchor_comment_id=$1`, comment)
			var preview sourceContextPreviewResponse
			testutil.Call(t, s.h.PreviewCommentSubIssue, testutil.WithURLParams(stageExitHTTPRequest(s, http.MethodGet, "/api/comments/"+comment+"/sub-issue-preview", "", nil), "commentId", comment)).Want(http.StatusOK).JSON(&preview)
			for _, status := range []string{"done", "todo"} {
				want := http.StatusCreated
				if status == "done" && mode == "enforce" {
					want = http.StatusConflict
				}
				title := "Manual comment child " + status
				out := testutil.Call(t, s.h.CreateCommentSubIssue, testutil.WithURLParams(stageExitHTTPRequest(s, http.MethodPost, "/api/comments/"+comment+"/sub-issues", "",
					map[string]any{"mode": "manual", "capture_token": preview.CaptureToken, "issue": map[string]any{"title": title, "status": status}}), "commentId", comment)).Want(want).Map()
				if want == http.StatusConflict && (out["code"] != "delivery_create_done_forbidden" ||
					s.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, s.fx.WorkspaceID, title) != 0 ||
					s.fx.Count(t, `SELECT count(*) FROM issue_source_context WHERE anchor_comment_id=$1`, comment) != 0) {
					t.Fatalf("comment-to-child refusal left creation/capture effects: %v", out)
				}
			}
			if s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, s.issue) != 0 ||
				s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1`, s.issue) != 0 ||
				s.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agent) != 1 {
				t.Fatal("manual creation fabricated AI approval/mapping or unapproved machine creation dispatched")
			}
		})
	}
}

func TestStageWriterContinuationChildSemanticEditsInvalidateCandidate(t *testing.T) {
	for _, field := range []string{"title", "description", "parent", "project", "delete"} {
		t.Run(field, func(t *testing.T) {
			f, s, candidate := stageWriterCompletionFixture(t, "enforce")
			before, err := s.svc.StageCodeEvidence(f.artifactFixture.request(http.MethodGet, "/unused", nil).Context(), parseUUID(s.issue), candidate.CandidateID)
			if err != nil || before.Candidate.State != "current" {
				t.Fatalf("fixture has no current exact code candidate: %+v %v", before.Candidate, err)
			}
			body := map[string]any{}
			switch field {
			case "title":
				body["title"] = "Changed ordinary child requirements"
			case "description":
				body["description"] = "Changed ordinary child content"
			case "parent":
				body["parent_issue_id"] = nil
			case "project":
				body["project_id"], body["project_change_reason"] = nil, "Explicit human ordinary-child adjustment"
			}
			if field == "delete" {
				testutil.Call(t, f.h.DeleteIssue, testutil.WithURLParams(f.artifactFixture.request(http.MethodDelete, "/api/issues/"+f.request.Target.IssueID, nil), "id", f.request.Target.IssueID)).Want(http.StatusNoContent)
			} else {
				testutil.Call(t, f.h.UpdateIssue, testutil.WithURLParams(f.artifactFixture.request(http.MethodPut, "/api/issues/"+f.request.Target.IssueID, body), "id", f.request.Target.IssueID)).Want(http.StatusOK)
				var semantic int64
				s.fx.QueryRow(t, `SELECT semantic_version FROM collaboration_stage_child WHERE child_issue_id=$1`, f.request.Target.IssueID).Scan(&semantic)
				if semantic != 2 {
					t.Fatalf("actual %s writer did not advance semantic_version: %d", field, semantic)
				}
			}
			if s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND root_issue_id=$2`, f.request.Target.IssueID, s.issue) != 1 {
				t.Fatal("ordinary child structure/deletion erased its immutable root mapping")
			}
			evidence, err := s.svc.StageCodeEvidence(f.artifactFixture.request(http.MethodGet, "/unused", nil).Context(), parseUUID(s.issue), candidate.CandidateID)
			if err != nil || field != "delete" && evidence.Candidate.State != "stale" {
				t.Fatalf("actual %s writer left the old candidate applicable: %+v %v", field, evidence.Candidate, err)
			}
			if field == "delete" && (evidence.Candidate.Baseline != nil || evidence.BlockedEvidence == 0) {
				t.Fatal("deleted artifact source remained readable or usable in the old candidate")
			}
			// State names the immutable current head. A deleted private source
			// withholds its baseline; prove non-applicability at the real advance
			// writer instead of inferring permission from that head label.
			var revision int64
			s.fx.QueryRow(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`, s.issue).Scan(&revision)
			advance := s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1",
				"expected_stage_revision": int64Text(revision), "target_stage": "code_review", "decision_ids": []string{},
				"candidate": map[string]any{"candidate_id": uuidToString(candidate.CandidateID), "version": int64Text(candidate.Version), "content_digest": candidate.ContentDigest}})
			out := testutil.Call(t, s.h.AdvanceCollaborationStage, s.request(http.MethodPost, "/stage/advance", s.actor, uuid.NewString(), advance)).Want(http.StatusConflict).Map()
			switch out["code"] {
			case "stage_candidate_stale", "stage_evidence_unavailable", "stage_needs_confirmation", "stage_scope_unconfirmed":
			default:
				t.Fatalf("%s did not invalidate the old exact candidate at its actual consumption writer: %v", field, out)
			}
			if s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo'`, s.issue) != 1 ||
				s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_task WHERE issue_id=$1 AND stage='development'`, s.issue) != 1 {
				t.Fatal("ordinary child write completed or advanced its collaboration root")
			}
		})
	}
}

func TestStageWriterContinuationGitHubVCSRootM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		for _, provider := range []string{"github", "forgejo"} {
			t.Run(mode+"/"+provider, func(t *testing.T) {
				s := stageHTTPFixture(t)
				stageExitHTTPM1(t, s, mode)
				s.revise(t, true)
				issue, err := s.h.Queries.GetIssue(t.Context(), parseUUID(s.issue))
				if err != nil {
					t.Fatal(err)
				}
				identifier := issueToResponse(issue, s.h.getIssuePrefix(t.Context(), issue.WorkspaceID)).Identifier
				secret := "synthetic-root-webhook-" + uuid.NewString()
				owner, repo, head := "synthetic-writer", uuid.NewString(), strings.Repeat("c", 40)
				installation := time.Now().UnixNano()
				connection := ""
				if provider == "github" {
					t.Setenv("GITHUB_WEBHOOK_SECRET", secret)
					s.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "installation_id": installation, "account_login": owner, "account_type": "User"})
				} else {
					box := withVCSBox(t)
					s.h.VCSSecretBox, s.h.cfg.VCSIntegrationEnabled = box, true
					sealed, err := box.Seal([]byte(secret))
					if err != nil {
						t.Fatal(err)
					}
					connection = s.fx.Insert(t, "vcs_connection", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "provider": provider,
						"instance_url": "https://" + uuid.NewString() + ".example.invalid", "account_login": owner,
						"access_token_encrypted": base64.StdEncoding.EncodeToString(sealed), "webhook_secret_encrypted": base64.StdEncoding.EncodeToString(sealed)})
				}
				clock := time.Now().UTC().Format(time.RFC3339Nano)
				raw, err := json.Marshal(map[string]any{"action": "closed", "installation": map[string]any{"id": installation},
					"repository": map[string]any{"name": repo, "owner": map[string]any{"login": owner, "username": owner}},
					"pull_request": map[string]any{"number": 1, "title": "Synthetic delivery " + identifier, "body": "Closes " + identifier,
						"state": "closed", "merged": true, "draft": false, "html_url": "https://example.invalid/pr/1",
						"created_at": clock, "updated_at": clock, "merged_at": clock, "closed_at": clock,
						"head": map[string]any{"ref": "delivery/" + identifier, "sha": head}, "user": map[string]any{"login": owner, "username": owner}}})
				if err != nil {
					t.Fatal(err)
				}
				mac := hmac.New(sha256.New, []byte(secret))
				_, _ = mac.Write(raw)
				signature := hex.EncodeToString(mac.Sum(nil))
				before := stageExitHTTPState(t, s)
				if provider == "github" {
					r := testutil.WithHeaders(testutil.JSONRequest(http.MethodPost, "/api/webhooks/github", raw), "X-GitHub-Event", "pull_request",
						"X-Hub-Signature-256", "sha256="+signature, "X-GitHub-Delivery", uuid.NewString())
					testutil.Call(t, s.h.HandleGitHubWebhook, r).Want(http.StatusAccepted)
				} else {
					r := vcsWebhookReq(connection, map[string]string{"X-Gitea-Event": "pull_request", "X-Gitea-Signature": signature, "X-Gitea-Delivery": uuid.NewString()}, raw)
					testutil.Call(t, s.h.HandleVCSWebhook, r).Want(http.StatusAccepted)
				}
				if stageExitHTTPState(t, s) != before {
					t.Fatalf("signed %s merge completed or mutated a managed root under M1 %s", provider, mode)
				}
				// Drive the actual shared writer as well; an accepted webhook
				// alone cannot establish that its auto-advance branch ran.
				s.h.advanceIssueToDone(t.Context(), issue, s.fx.WorkspaceID, nil)
				if stageExitHTTPState(t, s) != before || s.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal terminal
 JOIN delivery_completion_authorization a ON a.id=terminal.authorization_id WHERE a.issue_id=$1`, s.issue) != 0 {
					t.Fatalf("%s shared system completion writer bypassed root ownership", provider)
				}
			})
		}
	}
}
