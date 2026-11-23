package handler

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func completionCollaborationPolicy(t *testing.T, fx *testutil.Fixture, issue, actor string) string {
	t.Helper()
	project := fx.Project(t, "Collaboration completion evidence")
	fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, project, issue)
	fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": fx.WorkspaceID, "project_id": project, "revision": 1, "policy_version": "delivery-artifact-observe-v1", "mode": "enforce", "stage": "done", "reason": "Internal completion writer fixture", "created_by": actor}, "workspace_id=$1 AND project_id=$2", fx.WorkspaceID, project)
	return project
}
func completionCollaborationBody(t *testing.T, h *Handler, issue, actor string) map[string]any {
	t.Helper()
	var options service.CollaborationStatusOptions
	testutil.Call(t, h.GetCollaborationStatusOptions, deliveryHTTPRequest("GET", issue, "/status-options?action=complete", actor, "", nil)).Want(200).JSON(&options)
	var rev, epoch, participant, account int64
	testPool.QueryRow(t.Context(), `SELECT c.revision,c.lifecycle_epoch,p.epoch,a.epoch FROM issue_collaboration c JOIN issue_participant p ON p.issue_id=c.issue_id AND p.user_id=$2 JOIN collaboration_account_fence a ON a.user_id=p.user_id WHERE c.issue_id=$1`, issue, actor).Scan(&rev, &epoch, &participant, &account)
	return map[string]any{"action": "complete", "status": "done", "status_options_version": options.StatusOptionsVersion, "expected_collaboration_revision": fmt.Sprint(rev), "expected_lifecycle_epoch": fmt.Sprint(epoch), "expected_participant_epoch": fmt.Sprint(participant), "expected_account_epoch": fmt.Sprint(account)}
}
func TestDeliveryCompletionCollaborationRequiresAuthorization(t *testing.T) {
	for _, kind := range []string{"self", "submission"} {
		t.Run(kind, func(t *testing.T) {
			h, fx, actor, recipient, issue := deliveryHTTPFixture(t)
			completionCollaborationPolicy(t, fx, issue, actor)
			if kind == "self" {
				fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, issue, actor)
			} else {
				submitted := testutil.Call(t, h.SubmitCollaborationAcceptance, deliveryHTTPRequest("POST", issue, "/acceptance-submissions", recipient, uuid.NewString(), deliveryHTTPSubmissionBody(t, h, issue, recipient, "Exact external result", "Verified checks"))).Want(200).Map()
				_ = submitted
			}
			body := completionCollaborationBody(t, h, issue, actor)
			if kind == "self" {
				body["note"] = "Explicit self result"
				body["basis"] = "Verified checks"
			} else {
				var id string
				fx.QueryRow(t, `SELECT id FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' ORDER BY created_at DESC LIMIT 1`, issue).Scan(&id)
				body["submission_id"] = id
			}
			beforeHandoffs := fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, issue)
			beforeEvents := fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue)
			testutil.Call(t, h.ChangeCollaborationLifecycle, deliveryHTTPRequest("POST", issue, "/lifecycle", actor, uuid.NewString(), body)).Want(http.StatusPreconditionRequired)
			if fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, issue) != 0 || fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, issue) != beforeHandoffs || fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, issue) != beforeEvents {
				t.Fatal("missing authorization wrote completion or partial delivery")
			}
		})
	}
}
func TestDeliveryCompletionR38RequiresSourceAuthorization(t *testing.T) {
	f := newAdminCollaborationHTTPFixture(t)
	fx := testutil.New(testPool, f.workspace, f.creator.User.ID)
	completionCollaborationPolicy(t, fx, f.issue, f.creator.User.ID)
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator.User.ID)
	var detail service.AdminCollaborationDetail
	path := "/api/admin/collaboration/issues/" + f.issue
	f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
	f.call(t, "POST", path+"/lifecycle", adminCollaborationHTTPBody(detail, "complete"), uuid.NewString()).Want(428)
	if fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue) != 0 || fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='delivery_decision'`, f.issue) != 0 {
		t.Fatal("administration bypassed evidence or invented acceptance")
	}
}

type completionSharedFixture struct {
	h                                            *Handler
	fx                                           *testutil.Fixture
	actor, issuer, issue, child, project, output string
	source                                       *artifactFixture
	evidence                                     service.DeliveryCompletionEvidence
	admin                                        *adminCollaborationHTTPFixture
}

func (f *completionSharedFixture) request(method, path, actor string, body any) *http.Request {
	r := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "X-User-ID", actor, "X-Workspace-ID", f.fx.WorkspaceID)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	if auth.PasswordMode() {
		var version int64
		testPool.QueryRow(r.Context(), `SELECT session_version FROM user_password_credential WHERE user_id=$1`, actor).Scan(&version)
		r = r.WithContext(auth.WithPasswordSession(r.Context(), auth.PasswordSession{UserID: actor, Version: version, Kind: "jwt"}))
	}
	return r
}
func newCompletionSharedFixture(t *testing.T, kind string) *completionSharedFixture {
	t.Helper()
	f := &completionSharedFixture{}
	var recipient string
	if kind == "admin" {
		f.admin = newAdminCollaborationHTTPFixture(t)
		f.h = testHandler
		f.actor = f.admin.creator.User.ID
		f.issue = f.admin.issue
		issuer := passwordRegister(t, fmt.Sprintf("sourceissuer%d", time.Now().UnixNano()))
		f.issuer = issuer.User.ID
		f.fx = testutil.New(testPool, f.admin.workspace, f.issuer)
		f.fx.Member(t, f.admin.workspace, f.issuer, "owner")
	} else {
		f.h, f.fx, f.actor, recipient, f.issue = deliveryHTTPFixture(t)
		f.issuer = f.fx.User(t, "Independent source evaluator", "source-"+uuid.NewString()+"@example.test")
		f.fx.Member(t, f.fx.WorkspaceID, f.issuer, "owner")
	}
	f.project = f.fx.Project(t, "Shared delivery project")
	f.fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, f.project, f.issue)
	if kind == "self" {
		f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue, f.actor)
	}
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, f.fx.WorkspaceID)
	childReq := f.request("POST", "/api/issues", f.issuer, map[string]any{"title": "Actual source child " + uuid.NewString(), "parent_issue_id": f.issue, "status": "in_progress"})
	child := testutil.Call(t, f.h.CreateIssue, childReq).Want(201).Map()
	f.child = child["id"].(string)
	f.fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, f.child)
	sf := testutil.New(testPool, f.fx.WorkspaceID, f.issuer)
	runtime := sf.Runtime(t, "Source packaging runtime")
	agent := sf.Agent(t, "Source packaging operator", runtime, testutil.Cols{"autonomy_level": "operator"})
	task := sf.Task(t, agent, testutil.Cols{"status": "running", "issue_id": f.child, "runtime_id": runtime, "originator_user_id": f.issuer, "accountable_user_id": f.issuer, "claim_generation": int64(1)})
	entry := artifact.Entry{Path: "candidate.txt", SizeBytes: 9, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("candidate")))}
	input, err := artifact.InputDigest([]artifact.Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.StagingManifest{WorkspaceID: f.fx.WorkspaceID, AgentID: agent, TaskID: task, TaskGeneration: 1, ResourceID: f.child, SourceRevision: strings.Repeat("a", 40), InputManifestDigest: input}
	digest, err := artifact.StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	req := artifact.Request{AgentID: agent, TaskID: task, ResourceKind: "issue", ResourceID: f.child, Action: artifact.Action, StagingID: uuid.NewString(), SourceRevision: manifest.SourceRevision, StagingManifestDigest: digest, InputManifestDigest: input, Entries: []artifact.Entry{entry}, TargetEnvironment: artifact.Environment, PolicyVersion: artifact.Policy}
	base := t.TempDir()
	stage := filepath.Join(base, "stage")
	f.output = filepath.Join(base, "output")
	files := filepath.Join(stage, req.StagingID, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(f.output, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(manifest)
	if err = os.WriteFile(filepath.Join(stage, req.StagingID, "manifest.json"), raw, 0400); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(files, "candidate.txt"), []byte("candidate"), 0400); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{files, filepath.Dir(files), stage} {
		if err = os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, dir := range []string{stage, filepath.Dir(files), files} {
			_ = os.Chmod(dir, 0700)
		}
	})
	packager, err := artifact.New(artifact.Config{StagingRoot: stage, OutputRoot: f.output, Identity: "collaboration-source-test"})
	if err != nil {
		t.Fatal(err)
	}
	h := *f.h
	h.ArtifactPackager = packager
	f.h = &h
	f.source = &artifactFixture{h: h, req: req, actor: f.issuer, ws: f.fx.WorkspaceID, key: uuid.NewString()}
	create := f.request("POST", "/api/agent-approvals/artifact-package", f.issuer, req)
	create.Header.Set("Idempotency-Key", f.source.key)
	var approval artifactResponse
	testutil.Call(t, f.h.CreateArtifactPackage, create).Want(201).JSON(&approval)
	decision := testutil.WithURLParams(f.request("POST", "/decision", f.issuer, map[string]string{"decision": "approve", "expected_spec_digest": approval.SpecDigest}), "approvalId", approval.Approval.ID)
	testutil.Call(t, f.h.DecideAgentApproval, decision).Want(200)
	execution := testutil.WithURLParams(f.request("POST", "/execution", f.issuer, map[string]string{"expected_spec_digest": approval.SpecDigest}), "approvalId", approval.Approval.ID)
	testutil.Call(t, f.h.ExecuteArtifactPackage, execution).Want(200).JSON(&approval)
	sf.Cleanup(t, `DELETE FROM agent_artifact_execution WHERE task_id=$1`, task)
	sf.Cleanup(t, `DELETE FROM agent_approval_request WHERE task_id=$1`, task)
	if approval.Status != "succeeded" {
		t.Fatal("actual source artifact did not succeed")
	}
	data, err := os.ReadFile(filepath.Join(f.output, approval.ExecutionID, "artifact.tar"))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(data))
	header, err := tr.Next()
	if err != nil || header.Name != "candidate.txt" {
		t.Fatal("real tar entry mismatch", err)
	}
	content, err := io.ReadAll(tr)
	if err != nil || string(content) != "candidate" {
		t.Fatal("real tar content mismatch", err)
	}
	var artifactDigest string
	sf.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, approval.Approval.ID).Scan(&artifactDigest)
	if fmt.Sprintf("%x", sha256.Sum256(data)) != artifactDigest {
		t.Fatal("actual tar digest differs")
	}
	deliveryPR(t, f.source, req.SourceRevision, time.Now().UTC())
	var revision int64
	sf.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.child).Scan(&revision)
	f.evidence = service.DeliveryCompletionEvidence{IssueID: f.child, ExpectedIssueRevision: revision, ApprovalID: approval.Approval.ID, ExpectedSourceRevision: req.SourceRevision, ExpectedSpecDigest: approval.SpecDigest, ExpectedArtifactDigest: artifactDigest}
	sf.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.fx.WorkspaceID, "project_id": f.project, "revision": 1, "policy_version": deliveryEvidencePolicy, "mode": "enforce", "stage": "done", "reason": "Explicit source policy fixture", "created_by": f.issuer}, "project_id=$1", f.project)
	if kind == "submission" {
		testutil.Call(t, f.h.SubmitCollaborationAcceptance, deliveryHTTPRequest("POST", f.issue, "/acceptance-submissions", recipient, uuid.NewString(), deliveryHTTPSubmissionBody(t, f.h, f.issue, recipient, "Precise submitted result", "Source artifact checks"))).Want(200)
	}
	if kind != "admin" {
		sf.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.fx.WorkspaceID, f.actor)
	} else {
		sf.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.actor)
	}
	return f
}
func (f *completionSharedFixture) authorization(t *testing.T, kind string) (service.DeliveryCompletionAuthorizationRequest, map[string]any) {
	t.Helper()
	var revision, projectRevision int64
	var status string
	f.fx.QueryRow(t, `SELECT revision,status FROM issue WHERE id=$1`, f.issue).Scan(&revision, &status)
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&projectRevision)
	operation := uuid.NewString()
	target := service.DeliveryCompletionTarget{Kind: "collaboration_" + kind, IssueID: f.issue, ExpectedIssueRevision: revision, ExpectedStatus: status, TargetStatus: "done"}
	consumer := service.DeliveryCompletionConsumer{Kind: "human", UserID: f.actor}
	body := map[string]any{}
	if kind == "admin" {
		var detail service.AdminCollaborationDetail
		f.admin.call(t, "GET", "/api/admin/collaboration/issues/"+f.issue, nil, "").Want(200).JSON(&detail)
		body = adminCollaborationHTTPBody(detail, "complete")
		target.ExpectedCollaborationRevision = fmt.Sprint(detail.CollaborationRevision)
		var epoch int64
		f.fx.QueryRow(t, `SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1`, f.issue).Scan(&epoch)
		target.ExpectedLifecycleEpoch = fmt.Sprint(epoch)
		target.StatusOptionsVersion = body["status_options_version"].(string)
		target.AdministrationReason = body["reason"].(string)
		var org string
		f.fx.QueryRow(t, `SELECT organization_id FROM organization_workspace WHERE workspace_id=$1`, f.fx.WorkspaceID).Scan(&org)
		consumer = service.DeliveryCompletionConsumer{Kind: "admin", UserID: f.admin.admin.User.ID, OrganizationID: org}
	} else {
		body = completionCollaborationBody(t, f.h, f.issue, f.actor)
		target.ExpectedCollaborationRevision = body["expected_collaboration_revision"].(string)
		target.ExpectedLifecycleEpoch = body["expected_lifecycle_epoch"].(string)
		target.ExpectedParticipantEpoch = body["expected_participant_epoch"].(string)
		target.ExpectedAccountEpoch = body["expected_account_epoch"].(string)
		target.StatusOptionsVersion = body["status_options_version"].(string)
		if kind == "self" {
			target.Result = "Explicit accepted self result"
			target.Basis = "Exact source tar checked"
			target.AttachmentIDs = []string{}
			body["note"] = target.Result
			body["basis"] = target.Basis
		} else {
			var submission string
			f.fx.QueryRow(t, `SELECT id FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' ORDER BY created_at DESC LIMIT 1`, f.issue).Scan(&submission)
			target.SubmissionID = &submission
			body["submission_id"] = submission
		}
	}
	return service.DeliveryCompletionAuthorizationRequest{ContractVersion: service.DeliveryCompletionContract, SourceWorkspaceID: f.fx.WorkspaceID, OperationID: operation, Target: target, Project: service.DeliveryCompletionProject{ID: f.project, ExpectedRevision: projectRevision, ExpectedPolicyRevision: 1}, Evidence: f.evidence, Consumer: consumer, ExpiresAt: time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond), Reason: "Source evaluator approves exact shared result"}, body
}
func TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery(t *testing.T) {
	for _, kind := range []string{"self", "submission", "admin"} {
		t.Run(kind, func(t *testing.T) {
			f := newCompletionSharedFixture(t, kind)
			req, body := f.authorization(t, kind)
			before := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue)
			request := f.request("POST", "/api/delivery-completion-authorizations", f.issuer, req)
			request.Header.Set("Idempotency-Key", uuid.NewString())
			var grant service.DeliveryCompletionAuthorizationView
			testutil.Call(t, f.h.CreateDeliveryCompletionAuthorization, request).Want(201).JSON(&grant)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue) != before {
				t.Fatal("source grant created collaboration events")
			}
			body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
			invoke := func() *testutil.Response {
				if kind == "admin" {
					return f.admin.call(t, "POST", "/api/admin/collaboration/issues/"+f.issue+"/lifecycle", body, req.OperationID)
				}
				r := testutil.WithURLParams(f.request("POST", "/api/collaboration/issues/"+f.issue+"/lifecycle", f.actor, body), "id", f.issue)
				r.Header.Set("Idempotency-Key", req.OperationID)
				return testutil.Call(t, f.h.ChangeCollaborationLifecycle, r)
			}
			invoke().Want(200)
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID) != 1 {
				t.Fatal("workflow and exact grant were not committed together")
			}
			eventCount := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue)
			invoke().Want(200)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue) != eventCount {
				t.Fatal("original workflow replay wrote new events")
			}
			if kind == "admin" && f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, f.issue) != 0 {
				t.Fatal("administrator fabricated initiator acceptance")
			}
			actor := f.actor
			if kind == "admin" {
				actor = f.admin.admin.User.ID
			}
			get := testutil.WithURLParams(f.request("GET", "/api/delivery-completion-authorizations/"+grant.AuthorizationID, actor, nil), "authorizationId", grant.AuthorizationID)
			if kind == "admin" {
				get.Header.Del("X-Workspace-ID")
			}
			response := testutil.Call(t, f.h.GetDeliveryCompletionAuthorization, get).Want(200)
			for _, private := range []string{"observation", "issuer_pat_hash", "request_json", f.child, f.evidence.ExpectedArtifactDigest, "Source artifact checks"} {
				if strings.Contains(response.Text(), private) {
					t.Fatal("consumer learned private source evidence", private)
				}
			}
		})
	}
}

func (f *completionSharedFixture) grant(t *testing.T, req service.DeliveryCompletionAuthorizationRequest) service.DeliveryCompletionAuthorizationView {
	t.Helper()
	r := f.request("POST", "/api/delivery-completion-authorizations", f.issuer, req)
	r.Header.Set("Idempotency-Key", uuid.NewString())
	var out service.DeliveryCompletionAuthorizationView
	testutil.Call(t, f.h.CreateDeliveryCompletionAuthorization, r).Want(201).JSON(&out)
	return out
}
func (f *completionSharedFixture) complete(t *testing.T, req service.DeliveryCompletionAuthorizationRequest, body map[string]any, grant *service.DeliveryCompletionAuthorizationView) *testutil.Response {
	t.Helper()
	if grant != nil {
		body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	}
	r := testutil.WithURLParams(f.request("POST", "/api/collaboration/issues/"+f.issue+"/lifecycle", f.actor, body), "id", f.issue)
	r.Header.Set("Idempotency-Key", req.OperationID)
	return testutil.Call(t, f.h.ChangeCollaborationLifecycle, r)
}
func TestDeliveryCompletionCollaborationGrantAndRefusalDoNotBindAttachments(t *testing.T) {
	f := newCompletionSharedFixture(t, "self")
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	f.h.Storage = storage.NewLocalStorageFromEnv()
	draft, file := collaborationUploadedFile(t, f.h, f.issue, f.actor)
	req, body := f.authorization(t, "self")
	id := uuidToString(file.ID)
	req.Target.DraftID = &draft
	req.Target.AttachmentIDs = []string{id}
	body["draft_id"] = draft
	body["attachment_ids"] = []string{id}
	unbound := func() {
		t.Helper()
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL AND delivered_at IS NULL`, id) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1`, id) != 0 {
			t.Fatal("read-only grant/refusal attached a private draft")
		}
	}
	unbound()
	f.complete(t, req, body, nil).Want(428)
	unbound()
	req.OperationID = uuid.NewString()
	grant := f.grant(t, req)
	unbound()
	f.complete(t, req, body, &grant).Want(200)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id=$2 AND delivered_at IS NOT NULL`, id, f.issue) != 1 {
		t.Fatal("accepted self result did not bind exact selected attachment")
	}
}
func TestDeliveryCompletionCollaborationCurrentBindingsRefuse(t *testing.T) {
	for _, kind := range []string{"child_detached", "child_project_changed", "issuer_revoked", "participant_revoked", "self_result_changed", "policy_changed", "source_attachment_changed"} {
		t.Run(kind, func(t *testing.T) {
			f := newCompletionSharedFixture(t, "self")
			req, body := f.authorization(t, "self")
			var attachment string
			if kind == "source_attachment_changed" {
				t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
				f.h.Storage = storage.NewLocalStorageFromEnv()
				draft, file := collaborationUploadedFile(t, f.h, f.issue, f.actor)
				attachment = uuidToString(file.ID)
				req.Target.DraftID = &draft
				req.Target.AttachmentIDs = []string{attachment}
				body["draft_id"] = draft
				body["attachment_ids"] = []string{attachment}
			}
			grant := f.grant(t, req)
			switch kind {
			case "child_detached":
				f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL,revision=revision+1 WHERE id=$1`, f.child)
			case "child_project_changed":
				f.fx.Exec(t, `UPDATE issue SET project_id=NULL,revision=revision+1 WHERE id=$1`, f.child)
			case "issuer_revoked":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.fx.WorkspaceID, f.issuer)
			case "participant_revoked":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue, f.actor)
			case "self_result_changed":
				body["note"] = "Different unreviewed self result"
			case "policy_changed":
				f.fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.fx.WorkspaceID, "project_id": f.project, "revision": 2, "policy_version": deliveryEvidencePolicy, "mode": "enforce", "stage": "done", "reason": "new policy", "created_by": f.issuer}, "project_id=$1 AND revision=2", f.project)
			case "source_attachment_changed":
				f.fx.Exec(t, `UPDATE collaboration_attachment SET version=version+1 WHERE id=$1`, attachment)
			}
			before := f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue)
			response := f.complete(t, req, body, &grant)
			if response.Code < 400 {
				t.Fatal("invalid current binding completed", kind, response.Text())
			}
			if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue) != before {
				t.Fatal("refusal retained partial completion")
			}
		})
	}
}
func TestDeliveryCompletionCollaborationSourceProjectMove(t *testing.T) {
	f := newCompletionSharedFixture(t, "self")
	f.fx.Member(t, f.fx.WorkspaceID, f.actor, "member")
	base := completionCollaborationBody(t, f.h, f.issue, f.actor)
	for _, key := range []string{"action", "status", "status_options_version"} {
		delete(base, key)
	}
	base["project_id"] = nil
	base["project_change_reason"] = "Explicit project scope adjustment"
	call := func(actor string) *testutil.Response {
		r := testutil.WithURLParams(f.request("PUT", "/api/collaboration/issues/"+f.issue, actor, base), "id", f.issue)
		r.Header.Set("Idempotency-Key", uuid.NewString())
		return testutil.Call(t, f.h.UpdateCollaborationIssue, r)
	}
	call(f.actor).Want(403)
	call(f.issuer).Want(403)
	f.fx.Exec(t, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, f.fx.WorkspaceID, f.actor)
	call(f.actor).Want(200)
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id IS NULL`, f.issue) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_project_change WHERE issue_id=$1 AND actor_user_id=$2 AND reason=$3`, f.issue, f.actor, base["project_change_reason"]) != 1 {
		t.Fatal("authorized source move did not retain its scope adjustment")
	}
}

func TestDeliveryCompletionCollaborationChildLockAndRollback(t *testing.T) {
	for _, mode := range []string{"child_busy", "late_terminal_database_error"} {
		t.Run(mode, func(t *testing.T) {
			f := newCompletionSharedFixture(t, "self")
			req, body := f.authorization(t, "self")
			grant := f.grant(t, req)
			beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue)
			beforeHandoffs := f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue)
			var held pgx.Tx
			if mode == "child_busy" {
				var err error
				held, err = testPool.Begin(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer held.Rollback(t.Context())
				if _, err = held.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, f.child); err != nil {
					t.Fatal(err)
				}
			} else {
				f.h.TxStarter = completionTerminalFailureStarter{testPool}
			}
			response := f.complete(t, req, body, &grant)
			if response.Code < 400 {
				t.Fatal("busy/failed transaction reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue) != beforeEvents || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue) != beforeHandoffs || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue) != 0 {
				t.Fatal("partial collaboration state survived failed completion transaction")
			}
		})
	}
}

type completionTerminalFailureStarter struct {
	base interface {
		Begin(context.Context) (pgx.Tx, error)
	}
}

func (s completionTerminalFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return completionTerminalFailureTx{tx}, nil
}

type completionTerminalFailureTx struct{ pgx.Tx }

func (t completionTerminalFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "INSERT INTO delivery_completion_terminal") {
		return t.Tx.QueryRow(ctx, `SELECT 1/0`)
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}
func TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration(t *testing.T) {
	f := newCompletionSharedFixture(t, "admin")
	req, _ := f.authorization(t, "admin")
	grant := f.grant(t, req)
	closed := *f.h
	closed.cfg.PlatformAdminEnabled = false
	post := f.request("POST", "/api/delivery-completion-authorizations", f.issuer, req)
	post.Header.Set("Idempotency-Key", uuid.NewString())
	testutil.Call(t, closed.CreateDeliveryCompletionAuthorization, post).Want(403)
	get := func(actor string) *http.Request {
		return testutil.WithURLParams(f.request("GET", "/api/delivery-completion-authorizations/"+grant.AuthorizationID, actor, nil), "authorizationId", grant.AuthorizationID)
	}
	testutil.Call(t, closed.GetDeliveryCompletionAuthorization, get(f.admin.admin.User.ID)).Want(403)
	// Source access is independently authorized; disabling administration does
	// not erase the source evaluator's own retained minimal authorization fact.
	testutil.Call(t, closed.GetDeliveryCompletionAuthorization, get(f.issuer)).Want(200)
}
