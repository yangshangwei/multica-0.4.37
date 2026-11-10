package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These tests authenticate through NewRouter. Parent collaboration/account rows
// and the ordinary producer queue are test fixtures; the child is created by
// POST /issues, and artifact approval, packaging, authorization and completion
// all use actual routes, including the source owner's explicit policy transition
// from observe to enforce. This does not exercise a real provider or deployment.
type completionCollaborationRouterFixture struct {
	*deliveryPolicyRouterFixture
	parent, initiator, initiatorToken, recipient, recipientToken string
	admin, adminToken, organization, evaluationKey               string
}

func newCompletionCollaborationRouterFixture(t *testing.T, kind string) *completionCollaborationRouterFixture {
	t.Helper()
	mode, passwordVersion := "legacy", int64(0)
	if kind == "admin" {
		mode, passwordVersion = "password", 1
	}
	t.Setenv("MULTICA_AUTH_MODE", mode)
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", fmt.Sprint(kind == "admin"))
	t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
	t.Setenv("MULTICA_CLOUD_URL", "")
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")

	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	workspaceID := pgtype.UUID{Bytes: uuid.MustParse(testWorkspaceID), Valid: true}
	existingCatalog, err := db.New(testPool).ListIssueStatusEntries(t.Context(), db.ListIssueStatusEntriesParams{WorkspaceID: workspaceID, IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.New(testPool).SeedIssueStatusEntries(t.Context(), workspaceID); err != nil {
		t.Fatal(err)
	}
	if len(existingCatalog) == 0 {
		fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, testWorkspaceID)
	}
	account := func(name string) (string, string) {
		t.Helper()
		id := fx.User(t, name, "completion-shared-"+uuid.NewString()+"@example.test")
		if passwordVersion != 0 {
			fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": id, "username": strings.ReplaceAll(uuid.NewString(), "-", ""), "password_hash": "unused-signed-jwt-fixture", "session_version": passwordVersion, "must_change_password": false}, "user_id=$1", id)
		}
		return id, controlledArtifactJWT(t, id, passwordVersion, time.Now().Add(time.Hour))
	}
	issuer, issuerToken := account("Source evidence issuer")
	initiator, initiatorToken := account("Outside collaboration initiator")
	recipient, recipientToken := account("Outside collaboration recipient")
	fx.Member(t, testWorkspaceID, issuer, "owner")
	fx = testutil.New(testPool, testWorkspaceID, issuer)
	project := fx.Project(t, "Exact shared delivery source")
	assignee := recipient
	if kind == "self" {
		assignee = initiator
	}
	parent := fx.Issue(t, "Shared completion target", testutil.Cols{
		"creator_id": initiator, "assignee_type": "member", "assignee_id": assignee, "project_id": project,
	})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": parent}, "issue_id=$1", parent)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": parent})
	for _, actor := range []string{initiator, recipient} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": parent, "user_id": actor}, "issue_id=$1 AND user_id=$2", parent, actor)
		fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
	}
	for _, table := range []string{"issue_handoff", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, parent)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1`, parent)
	fx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, testWorkspaceID)

	source := &controlledArtifactRouterFixture{fx: fx, router: controlledArtifactRouter(t), user: issuer, token: issuerToken}
	child := source.call(t, issuerToken, "POST", "/api/issues", map[string]any{
		"title": "Actual ordinary source child " + uuid.NewString(), "parent_issue_id": parent, "status": "in_progress",
	}).Want(201).Map()
	childID, ok := child["id"].(string)
	if !ok || childID == "" || child["parent_issue_id"] != parent || child["project_id"] != project {
		t.Fatal("actual child route did not retain the exact parent/project")
	}
	fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, childID)
	if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, parent) != 0 {
		t.Fatal("shared parent acquired an ordinary producer task")
	}

	runtime := fx.Runtime(t, "Ordinary child packaging runtime")
	agent := fx.Agent(t, "Ordinary child packaging operator", runtime, testutil.Cols{"autonomy_level": "operator"})
	task := fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": childID, "runtime_id": runtime, "originator_user_id": issuer, "accountable_user_id": issuer, "claim_generation": 1})
	taskToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	source.taskToken = taskToken
	source.tokenID = fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(taskToken), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": issuer, "expires_at": time.Now().Add(time.Hour), "auth_version": passwordVersion})
	entry := artifact.Entry{Path: "candidate.txt", SizeBytes: 9, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("candidate")))}
	input, err := artifact.InputDigest([]artifact.Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.StagingManifest{WorkspaceID: testWorkspaceID, AgentID: agent, TaskID: task, TaskGeneration: 1, ResourceID: childID, SourceRevision: strings.Repeat("a", 40), InputManifestDigest: input}
	digest, err := artifact.StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	source.request = artifact.Request{AgentID: agent, TaskID: task, ResourceKind: "issue", ResourceID: childID, Action: artifact.Action, StagingID: uuid.NewString(), SourceRevision: manifest.SourceRevision, StagingManifestDigest: digest, InputManifestDigest: input, Entries: []artifact.Entry{entry}, TargetEnvironment: artifact.Environment, PolicyVersion: artifact.Policy}
	base := t.TempDir()
	stage := filepath.Join(base, "stage")
	source.output = filepath.Join(base, "output")
	files := filepath.Join(stage, source.request.StagingID, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(source.output, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, source.request.StagingID, "manifest.json"), raw, 0400); err != nil {
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
	t.Setenv("MULTICA_ARTIFACT_STAGING_ROOT", stage)
	t.Setenv("MULTICA_ARTIFACT_OUTPUT_ROOT", source.output)
	t.Setenv("MULTICA_ARTIFACT_CONFIG_ID", "shared-completion-router")
	source.router = controlledArtifactRouter(t)
	approval := source.create(t, taskToken)
	path := "/api/agent-approvals/" + approval.Approval.ID
	source.call(t, issuerToken, "POST", path+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": approval.SpecDigest}).Want(200)
	source.call(t, taskToken, "POST", path+"/artifact-package/execution", map[string]string{"expected_spec_digest": approval.SpecDigest}).Want(200).JSON(&approval)
	fx.Cleanup(t, `DELETE FROM agent_artifact_execution WHERE task_id=$1`, task)
	fx.Cleanup(t, `DELETE FROM agent_approval_request WHERE task_id=$1`, task)
	if approval.Status != "succeeded" || approval.ReceiptDigest == "" {
		t.Fatal("actual source-child packaging did not succeed")
	}
	var childRevision, projectRevision int64
	var artifactDigest string
	fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, childID).Scan(&childRevision)
	fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&projectRevision)
	fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, approval.Approval.ID).Scan(&artifactDigest)
	pr := fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": 1, "repo_owner": "synthetic", "repo_name": "shared-source-" + uuid.NewString(), "pr_number": 1, "title": "Database observed source", "state": "open", "html_url": "https://example.invalid/pull/1", "head_sha": source.request.SourceRevision, "pr_created_at": time.Now(), "pr_updated_at": time.Now()})
	fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": childID, "pull_request_id": pr}, "issue_id=$1 AND pull_request_id=$2", childID, pr)
	evidence := &deliveryEvidenceRouterFixture{controlledArtifactRouterFixture: source, approval: approval, pr: pr, body: map[string]any{
		"policy_version": service.DeliveryEvidencePolicy, "expected_issue_revision": childRevision,
		"expected_project_id": project, "expected_project_revision": projectRevision,
		"approval_id": approval.Approval.ID, "expected_source_revision": source.request.SourceRevision,
		"expected_spec_digest": approval.SpecDigest, "expected_artifact_digest": artifactDigest,
	}}
	f := &completionCollaborationRouterFixture{deliveryPolicyRouterFixture: &deliveryPolicyRouterFixture{deliveryEvidenceRouterFixture: evidence, project: project}, parent: parent, initiator: initiator, initiatorToken: initiatorToken, recipient: recipient, recipientToken: recipientToken, evaluationKey: uuid.NewString()}
	f.archive(t) // Existing helper verifies actual tar entry, bytes, SHA and EOF.
	f.call(t, issuerToken, "PUT", f.policyPath(), f.policyBody(0, "observe")).Want(200)
	f.call(t, issuerToken, "POST", f.evaluationsPath(), f.evaluationBody(1), "Idempotency-Key", f.evaluationKey).Want(200)
	var policy service.DeliveryPolicy
	f.call(t, issuerToken, "PUT", f.policyPath(), f.policyBody(1, "enforce")).Want(200).JSON(&policy)
	if policy.Mode != "enforce" || policy.Revision != 2 || policy.CreatedBy == nil || *policy.CreatedBy != issuer {
		t.Fatal("source owner did not explicitly enable the expected policy revision")
	}
	// The source issuer was already an owner; public activation must not change
	// that fixture role or grant membership to the outside consumer.
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role='owner'`, testWorkspaceID, issuer) != 1 {
		t.Fatal("policy activation changed the source issuer's membership")
	}
	if kind == "submission" {
		body := f.workflowBody(t, recipientToken, "submit_acceptance")
		body["note"], body["basis"] = "Exact external result", "Actual source-child tar checked"
		var receipt struct {
			Confirmation struct {
				SubmissionID string `json:"submission_id"`
			} `json:"delivery_confirmation"`
		}
		f.call(t, recipientToken, "POST", f.pathFor("/acceptance-submissions"), body).Want(200).JSON(&receipt)
		if receipt.Confirmation.SubmissionID == "" {
			t.Fatal("authenticated submission did not identify exact result")
		}
	}
	if kind == "admin" {
		admin := fx.User(t, "Password platform administrator", "completion-admin-"+uuid.NewString()+"@example.test")
		username := strings.ReplaceAll(uuid.NewString(), "-", "")
		hash, err := auth.HashPassword(t.Context(), platformRouterPassword)
		if err != nil {
			t.Fatal(err)
		}
		fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": admin, "username": username, "password_hash": hash, "session_version": 1, "must_change_password": false}, "user_id=$1", admin)
		fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": admin, "role": "super_admin"}, "user_id=$1", admin)
		if fx.Count(t, `SELECT count(*) FROM organization WHERE internal=true AND state='active'`) == 0 {
			fx.Insert(t, "organization", testutil.Cols{"name": "Task-owned completion organization"})
		}
		org, err := db.New(testPool).GetInternalOrganization(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		f.organization = uuid.UUID(org.ID.Bytes).String()
		if fx.Count(t, `SELECT count(*) FROM organization_workspace WHERE workspace_id=$1`, testWorkspaceID) == 0 {
			fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": testWorkspaceID, "organization_id": org.ID}, "workspace_id=$1", testWorkspaceID)
		}
		var login struct {
			Token string `json:"token"`
		}
		testutil.Call(t, f.router.ServeHTTP, testutil.JSONRequest("POST", "/auth/login", map[string]string{"username": username, "password": platformRouterPassword})).Want(200).JSON(&login)
		if login.Token == "" {
			t.Fatal("actual password login returned no credential")
		}
		f.admin, f.adminToken = admin, login.Token
		fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, initiator)
		fx.Cleanup(t, `DELETE FROM admin_audit_event WHERE target_id=$1`, parent)
		fx.Cleanup(t, `DELETE FROM admin_operation WHERE target_id=$1`, parent)
	}
	return f
}

func (f *completionCollaborationRouterFixture) pathFor(suffix string) string {
	return "/api/collaboration/issues/" + f.parent + suffix
}

func (f *completionCollaborationRouterFixture) workflowBody(t *testing.T, token, action string) map[string]any {
	t.Helper()
	var detail struct {
		Revision string `json:"collaboration_revision"`
		Epoch    string `json:"lifecycle_epoch"`
		Write    struct {
			Participant string `json:"participant_epoch"`
			Account     string `json:"account_epoch"`
		} `json:"write_context"`
	}
	f.call(t, token, "GET", f.pathFor(""), nil).Want(200).JSON(&detail)
	var options service.CollaborationStatusOptions
	f.call(t, token, "GET", f.pathFor("/status-options?action="+action), nil).Want(200).JSON(&options)
	body := map[string]any{"expected_collaboration_revision": detail.Revision, "expected_lifecycle_epoch": detail.Epoch, "expected_participant_epoch": detail.Write.Participant, "expected_account_epoch": detail.Write.Account, "status_options_version": options.StatusOptionsVersion}
	if action == "submit_acceptance" {
		body["expected_status"] = options.DefaultStatus
	} else {
		body["action"], body["status"] = action, options.DefaultStatus
	}
	return body
}

func (f *completionCollaborationRouterFixture) authorization(t *testing.T, kind string) (service.DeliveryCompletionAuthorizationRequest, map[string]any) {
	t.Helper()
	var revision, projectRevision int64
	var status string
	f.fx.QueryRow(t, `SELECT revision,status FROM issue WHERE id=$1`, f.parent).Scan(&revision, &status)
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&projectRevision)
	target := service.DeliveryCompletionTarget{Kind: "collaboration_" + kind, IssueID: f.parent, ExpectedIssueRevision: revision, ExpectedStatus: status, TargetStatus: "done"}
	consumer := service.DeliveryCompletionConsumer{Kind: "human", UserID: f.initiator}
	var body map[string]any
	if kind == "admin" {
		var detail service.AdminCollaborationDetail
		f.call(t, f.adminToken, "GET", "/api/admin/collaboration/issues/"+f.parent, nil).Want(200).JSON(&detail)
		for _, option := range detail.Actions {
			if option.Action == "complete" {
				body = map[string]any{"action": "complete", "expected_collaboration_revision": fmt.Sprint(detail.CollaborationRevision), "expected_status": detail.Status, "target_status": option.DefaultStatus, "status_options_version": option.StatusOptionsVersion, "reason": "Explicit disabled-creator completion"}
			}
		}
		if body == nil {
			t.Fatal("actual administrator has no complete action")
		}
		target.ExpectedCollaborationRevision = fmt.Sprint(detail.CollaborationRevision)
		var epoch int64
		f.fx.QueryRow(t, `SELECT lifecycle_epoch FROM issue_collaboration WHERE issue_id=$1`, f.parent).Scan(&epoch)
		target.ExpectedLifecycleEpoch = fmt.Sprint(epoch)
		target.StatusOptionsVersion = body["status_options_version"].(string)
		target.AdministrationReason = body["reason"].(string)
		consumer = service.DeliveryCompletionConsumer{Kind: "admin", UserID: f.admin, OrganizationID: f.organization}
	} else {
		body = f.workflowBody(t, f.initiatorToken, "complete")
		target.ExpectedCollaborationRevision = body["expected_collaboration_revision"].(string)
		target.ExpectedLifecycleEpoch = body["expected_lifecycle_epoch"].(string)
		target.ExpectedParticipantEpoch = body["expected_participant_epoch"].(string)
		target.ExpectedAccountEpoch = body["expected_account_epoch"].(string)
		target.StatusOptionsVersion = body["status_options_version"].(string)
		if kind == "self" {
			target.Result, target.Basis = "Exact self result", "Source-child archive verified"
			target.AttachmentIDs = []string{}
			body["note"], body["basis"] = target.Result, target.Basis
		} else {
			var submission string
			f.fx.QueryRow(t, `SELECT id::text FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' ORDER BY created_at DESC LIMIT 1`, f.parent).Scan(&submission)
			target.SubmissionID = &submission
			body["submission_id"] = submission
		}
	}
	var expires time.Time
	f.fx.QueryRow(t, `SELECT clock_timestamp()+interval '1 minute'`).Scan(&expires)
	return service.DeliveryCompletionAuthorizationRequest{ContractVersion: service.DeliveryCompletionContract, SourceWorkspaceID: testWorkspaceID, OperationID: uuid.NewString(), Target: target, Project: service.DeliveryCompletionProject{ID: f.project, ExpectedRevision: projectRevision, ExpectedPolicyRevision: 2}, Evidence: service.DeliveryCompletionEvidence{IssueID: f.request.ResourceID, ExpectedIssueRevision: f.body["expected_issue_revision"].(int64), ApprovalID: f.approval.Approval.ID, ExpectedSourceRevision: f.request.SourceRevision, ExpectedSpecDigest: f.approval.SpecDigest, ExpectedArtifactDigest: f.body["expected_artifact_digest"].(string)}, Consumer: consumer, ExpiresAt: expires.UTC().Truncate(time.Microsecond), Reason: "Source human accepts this exact collaboration candidate"}, body
}

func (f *completionCollaborationRouterFixture) grant(t *testing.T, req service.DeliveryCompletionAuthorizationRequest) service.DeliveryCompletionAuthorizationView {
	t.Helper()
	var grant service.DeliveryCompletionAuthorizationView
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, req).Want(201).JSON(&grant)
	return grant
}

func (f *completionCollaborationRouterFixture) complete(t *testing.T, req service.DeliveryCompletionAuthorizationRequest, body map[string]any, grant *service.DeliveryCompletionAuthorizationView) *testutil.Response {
	t.Helper()
	if grant != nil {
		body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	}
	path, token := f.pathFor("/lifecycle"), f.initiatorToken
	if req.Target.Kind == "collaboration_admin" {
		path, token = "/api/admin/collaboration/issues/"+f.parent+"/lifecycle", f.adminToken
	}
	return f.call(t, token, "POST", path, body, "Idempotency-Key", req.OperationID)
}

func (f *completionCollaborationRouterFixture) upload(t *testing.T) (string, string) {
	t.Helper()
	draft := uuid.NewString()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for key, value := range map[string]string{"draft_id": draft, "issue_id": f.parent} {
		if err := form.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := form.CreateFormFile("file", "private-result.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("Actual encrypted self-result evidence")); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := http.NewRequest("POST", "/api/collaboration/attachments/drafts", &body)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+f.initiatorToken)
	r.Header.Set("Idempotency-Key", uuid.NewString())
	var uploaded service.CollaborationAttachmentView
	testutil.Call(t, f.router.ServeHTTP, r).Want(201).JSON(&uploaded)
	id := uuid.UUID(uploaded.ID.Bytes).String()
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, id)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, id)
	return draft, id
}

func TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery(t *testing.T) {
	for _, kind := range []string{"self", "submission", "admin"} {
		t.Run(kind, func(t *testing.T) {
			f := newCompletionCollaborationRouterFixture(t, kind)
			req, body := f.authorization(t, kind)
			grant := f.grant(t, req)
			archive := f.archive(t)
			if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.initiator) != 0 || f.initiator == f.user {
				t.Fatal("initiator is not distinct from the source issuer or unexpectedly owns source membership")
			}
			f.complete(t, req, body, &grant).Want(200)
			var beforeRevision, afterRevision int64
			var operation, status string
			f.fx.QueryRow(t, `SELECT issue_revision_before,issue_revision_after,operation_id::text,status_after FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID).Scan(&beforeRevision, &afterRevision, &operation, &status)
			if beforeRevision != req.Target.ExpectedIssueRevision || afterRevision <= beforeRevision || operation != req.OperationID || status != "done" {
				t.Fatal("actual completed version and original grant did not commit together")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.parent) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='closed'`, f.parent) != 1 {
				t.Fatal("real collaboration lifecycle did not close")
			}
			var collaborationOperation, adminOperation, resultSubmission pgtype.UUID
			var terminalActor string
			f.fx.QueryRow(t, `SELECT collaboration_operation_id,admin_operation_id,result_submission_id,actor_user_id::text FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID).Scan(&collaborationOperation, &adminOperation, &resultSubmission, &terminalActor)
			if !collaborationOperation.Valid || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE id=$1 AND scope_id=$2 AND request_id=$3`, collaborationOperation, f.parent, req.OperationID) != 1 {
				t.Fatal("completion terminal is not linked to the actual original workflow operation")
			}
			accepted := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, f.parent)
			if kind == "admin" {
				if accepted != 0 || resultSubmission.Valid || !adminOperation.Valid || terminalActor != f.admin || f.fx.Count(t, `SELECT count(*) FROM admin_operation WHERE id=$1 AND target_id=$2 AND actor_id=$3`, adminOperation, f.parent, f.admin) != 1 || f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.admin) != 0 {
					t.Fatal("R38 did not preserve its distinct real administrator without membership/acceptance")
				}
			} else {
				if accepted != 1 || !resultSubmission.Valid || adminOperation.Valid || terminalActor != f.initiator || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND actor_user_id=$2 AND payload->'delivery_decision'->>'kind'='accepted' AND payload->'delivery_decision'->>'submission_id'=$3`, f.parent, f.initiator, uuid.UUID(resultSubmission.Bytes).String()) != 1 {
					t.Fatal("actual initiator decision, terminal and exact submission were not recorded together")
				}
				if kind == "submission" && uuid.UUID(resultSubmission.Bytes).String() != *req.Target.SubmissionID {
					t.Fatal("external completion accepted a different submission")
				}
				if kind == "self" && f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND issue_id=$2 AND actor_user_id=$3 AND from_user_id=$3 AND to_user_id=$3 AND purpose='acceptance' AND note=$4`, resultSubmission, f.parent, f.initiator, req.Target.Result) != 1 {
					t.Fatal("self completion did not produce its own actual frozen result")
				}
			}
			count := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.parent)
			f.complete(t, req, body, &grant).Want(200)
			token, recovery := f.initiatorToken, f.pathFor("/operations/"+req.OperationID)
			if kind == "admin" {
				token, recovery = f.adminToken, "/api/admin/collaboration/issues/"+f.parent+"/operations/"+req.OperationID
			}
			f.call(t, token, "GET", recovery, nil).Want(200)
			read := f.call(t, token, "GET", completionAuthorizationRouterPath+"/"+grant.AuthorizationID, nil).Want(200)
			if read.Header().Get("Cache-Control") != "no-store" || !strings.Contains(read.Text(), `"state":"consumed"`) {
				t.Fatal("authenticated consumer did not recover minimal consumed fact")
			}
			for _, private := range []string{"observation", "request_json", f.request.ResourceID, f.body["expected_artifact_digest"].(string), f.evaluationKey, f.token, f.taskToken, "candidate.txt"} {
				if strings.Contains(read.Text(), private) {
					t.Fatal("narrow consumer recovery leaked private source evidence")
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.parent) != count || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 || !bytes.Equal(archive, f.archive(t)) {
				t.Fatal("original-key recovery changed business history or source bytes")
			}
		})
	}
}

func TestDeliveryCompletionCollaborationRouterPrivateEvaluationAndAuthentication(t *testing.T) {
	f := newCompletionCollaborationRouterFixture(t, "self")
	req, body := f.authorization(t, "self")
	grant := f.grant(t, req)
	evaluations := f.evaluationRows(t)
	private := f.call(t, f.initiatorToken, "GET", f.evaluationsPath()+"/"+f.evaluationKey, nil)
	if private.Code != 403 && private.Code != 404 {
		t.Fatal("outside initiator acquired issuer private evaluation", private.Code)
	}
	missing := f.call(t, f.initiatorToken, "GET", "/api/issues/"+uuid.NewString()+"/delivery-evidence/evaluations/"+uuid.NewString(), nil)
	if missing.Code != private.Code || missing.Text() != private.Text() {
		t.Fatal("outside evaluation refusal disclosed resource existence")
	}
	for _, value := range []string{f.evaluationKey, f.request.ResourceID, f.body["expected_artifact_digest"].(string), "observation"} {
		if strings.Contains(private.Text(), value) {
			t.Fatal("outside evaluation refusal disclosed private evidence")
		}
	}
	body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	for _, token := range []string{"", f.taskToken} {
		request := testutil.WithHeaders(testutil.JSONRequest("POST", f.pathFor("/lifecycle"), body), "X-User-ID", f.initiator, "X-Actor-Source", "member", "Idempotency-Key", req.OperationID)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := testutil.Call(t, f.router.ServeHTTP, request)
		want := http.StatusUnauthorized
		if token != "" {
			want = http.StatusForbidden
		}
		response.Want(want)
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 || f.evaluationRows(t) != evaluations {
		t.Fatal("refused authentication consumed an authorization or changed private evaluation")
	}
}

func TestDeliveryCompletionCollaborationRouterCurrentIdentityAndChild(t *testing.T) {
	for _, changed := range []string{"disabled_consumer", "revoked_participant", "detached_child", "revoked_issuer"} {
		t.Run(changed, func(t *testing.T) {
			f := newCompletionCollaborationRouterFixture(t, "self")
			req, body := f.authorization(t, "self")
			grant := f.grant(t, req)
			switch changed {
			case "disabled_consumer":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.initiator)
			case "revoked_participant":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=clock_timestamp(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.parent, f.initiator)
			case "detached_child":
				f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]any{"parent_issue_id": nil}).Want(200)
			case "revoked_issuer":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
			}
			handoffs := f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.parent)
			events := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.parent)
			response := f.complete(t, req, body, &grant)
			if response.Code < 400 || response.Code >= 500 {
				t.Errorf("%s should return a definite 4xx refusal; got %d: %s", changed, response.Code, response.Text())
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.parent) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.parent) != handoffs || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.parent) != events || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatal("identity/source refusal left partial completion")
			}
		})
	}
}

func TestDeliveryCompletionCollaborationRouterSelfAttachmentRemainsUnboundUntilAccepted(t *testing.T) {
	f := newCompletionCollaborationRouterFixture(t, "self")
	draft, attachment := f.upload(t)
	req, body := f.authorization(t, "self")
	req.Target.DraftID, req.Target.AttachmentIDs = &draft, []string{attachment}
	body["draft_id"], body["attachment_ids"] = draft, []string{attachment}
	unbound := func() {
		t.Helper()
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL AND delivered_at IS NULL`, attachment) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1`, attachment) != 0 || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.parent) != 0 {
			t.Fatal("grant or refusal partially delivered a self-result attachment")
		}
	}
	unbound()
	f.complete(t, req, body, nil).Want(428)
	unbound()
	req.OperationID = uuid.NewString()
	grant := f.grant(t, req)
	unbound()
	f.complete(t, req, body, &grant).Want(200)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id=$2 AND delivered_at IS NOT NULL`, attachment, f.parent) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 {
		t.Fatal("accepted self-result did not bind the actual uploaded evidence atomically")
	}
}
