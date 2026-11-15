package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This is a retained M1 fixture for read-authorization coverage, not proof of
// the M1 approval HTTP journey. It uses actual packager bytes/canonical spec;
// all subsequent stage/comment/claim/report/QA calls use their real services.
func stageArtifactReadFixture(t *testing.T, f *stageReviewFixture) {
	t.Helper()
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.reviewer)
	runtime := f.fx.Runtime(t, "Synthetic artifact runtime", testutil.Cols{"owner_id": f.creator})
	agent := f.fx.Agent(t, "Synthetic artifact operator", runtime, testutil.Cols{"owner_id": f.creator, "autonomy_level": "operator"})
	task := f.fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": f.child, "runtime_id": runtime, "originator_user_id": f.creator, "accountable_user_id": f.creator, "claim_generation": int64(1)})
	entry := artifact.Entry{Path: "candidate.txt", SizeBytes: 9, SHA256: hexHash([]byte("candidate"))}
	input, err := artifact.InputDigest([]artifact.Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.StagingManifest{WorkspaceID: f.workspace, AgentID: agent, TaskID: task, TaskGeneration: 1, ResourceID: f.child, SourceRevision: f.head, InputManifestDigest: input}
	manifestDigest, err := artifact.StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	req := artifact.Request{AgentID: agent, TaskID: task, ResourceKind: "issue", ResourceID: f.child, Action: artifact.Action, StagingID: uuid.NewString(), SourceRevision: f.head, InputManifestDigest: input, StagingManifestDigest: manifestDigest, Entries: []artifact.Entry{entry}, TargetEnvironment: artifact.Environment, PolicyVersion: artifact.Policy}
	base := t.TempDir()
	stage, output := filepath.Join(base, "stage"), filepath.Join(base, "output")
	files := filepath.Join(stage, req.StagingID, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(output, 0700); err != nil {
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
	packager, err := artifact.New(artifact.Config{StagingRoot: stage, OutputRoot: output, Identity: "stage-read-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	now, err := f.svc.Queries.ArtifactExecutionDatabaseTime(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	binding := artifact.Binding{WorkspaceID: f.workspace, InitiatorKind: "human", InitiatorID: f.creator, OriginatorUserID: f.creator, RuntimeID: runtime, TaskGeneration: 1}
	spec, err := packager.Prepare(f.ctx, req, binding, now.Time)
	if err != nil {
		t.Fatal(err)
	}
	canonical, digest, err := artifact.CanonicalSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	requestDigest, err := artifact.RequestDigest(req, binding)
	if err != nil {
		t.Fatal(err)
	}
	execution := uuid.NewString()
	outcome := packager.Pack(f.ctx, execution, spec)
	if outcome.Status != "succeeded" {
		t.Fatalf("synthetic artifact packaging: %+v", outcome)
	}
	approval := f.fx.Insert(t, "agent_approval_request", testutil.Cols{"workspace_id": f.workspace, "agent_id": agent, "task_id": task, "issue_id": f.child, "risk_class": "production_release", "summary": "Retained synthetic artifact", "plan": "Read authority fixture", "status": "executed", "decided_by": f.creator, "decided_at": now.Time, "executed_at": now.Time})
	finished, err := f.svc.Queries.ArtifactExecutionDatabaseTime(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	receipt, _ := json.Marshal(map[string]any{"finished_at": finished.Time.UTC(), "execution_id": execution, "approval_id": approval, "spec_digest": digest, "actor_kind": "human", "actor_id": f.creator, "resource_id": f.child, "action": req.Action, "declared_revision": req.SourceRevision, "input_digest": input, "environment": req.TargetEnvironment, "policy": req.PolicyVersion, "result": outcome.Status, "code": outcome.Code, "artifact_id": outcome.ArtifactID, "output_digest": outcome.OutputDigest, "output_bytes": outcome.OutputSize, "claimed_at": now.Time.UTC()})
	f.fx.InsertNoID(t, "agent_artifact_execution", testutil.Cols{"execution_id": execution, "approval_id": approval, "workspace_id": f.workspace, "initiator_kind": "human", "initiator_id": f.creator, "originator_user_id": f.creator, "agent_id": agent, "task_id": task, "runtime_id": runtime, "task_generation": int64(1), "resource_kind": "issue", "resource_id": f.child, "action": req.Action, "idempotency_key": uuid.NewString(), "request_digest": requestDigest, "canonical_spec": canonical, "spec_digest": digest, "source_revision": f.head, "staging_id": req.StagingID, "staging_manifest_digest": manifestDigest, "input_manifest_digest": input, "invocation_digest": spec.InvocationDigest, "policy_version": req.PolicyVersion, "target_environment": req.TargetEnvironment, "root_config_identity": spec.RootIdentity, "created_at": now.Time, "expires_at": now.Time.Add(15 * time.Minute), "updated_at": finished.Time, "claimed_kind": "human", "claimed_by": f.creator, "claim_nonce": uuid.NewString(), "claimed_at": now.Time, "started_at": now.Time, "finished_at": finished.Time, "result_status": "succeeded", "result_code": outcome.Code, "receipt_json": json.RawMessage(receipt), "receipt_digest": hexHash(receipt), "output_artifact_id": execution, "output_artifact_digest": outcome.OutputDigest, "output_size": outcome.OutputSize}, "execution_id=$1", execution)
	f.artifact = &CollaborationStageArtifactSelection{ApprovalID: util.MustParseUUID(approval)}
}

func hexHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func stageArtifactReportFixture(t *testing.T) (*stageRuntimeFixture, protocol.CollaborationTestReportReceipt) {
	t.Helper()
	f := newStageRuntimeFixture(t, true, stageArtifactReadFixture)
	run, claim, permit := startStageReportRuntime(t, f)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	p := stageRuntimeReport(subject.Subject, claim.Identity)
	ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
	if err != nil {
		t.Fatal(err)
	}
	return f, ack
}

func TestStageArtifactReportReadAuthorityMatchesR4(t *testing.T) {
	f, ack := stageArtifactReportFixture(t)
	run := util.MustParseUUID(ack.RunID)
	var err error
	read := func(ctx context.Context) (CollaborationTestReportDetail, error) {
		return f.svc.TestReport(ctx, f.issue.Issue.ID, run, util.MustParseUUID(ack.ReportID))
	}
	for _, ctx := range []context.Context{f.ctx, f.stage.reviewerCtx} {
		if _, err = read(ctx); err != nil {
			t.Fatalf("artifact initiator/manager lost read: %v", err)
		}
		if page, err := f.svc.TestReports(ctx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20}); err != nil || len(page.Items) != 1 {
			t.Fatalf("authorized report list: %+v %v", page, err)
		}
	}
	denied := f.stage.qaCtx
	view := f.stage.evidence(t, denied, f.candidate)
	if view.Candidate.Baseline != nil || len(view.TestReports) != 0 {
		t.Fatal("R4 did not enforce artifact read boundary")
	}
	if _, err = read(denied); !collaborationErrorCode(err, "not_found") {
		t.Errorf("ordinary source member read artifact report detail: %v", err)
	}
	if _, err = f.svc.TestReports(denied, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20}); !collaborationErrorCode(err, "not_found") {
		t.Errorf("ordinary source member read artifact report list: %v", err)
	}
	qa := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, denied), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "return", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{}, Reason: "Synthetic QA return"}
	if _, err = f.svc.DecideStage(denied, qa); !collaborationErrorCode(err, "stage_evidence_unavailable") {
		t.Errorf("QA cited an unreadable artifact report: %v", err)
	}
	qa.CollaborationWorkflowRequest = f.stage.req(t, denied)
	qa.Evidence = CollaborationStageReviewEvidence{}
	if _, err = f.svc.DecideStage(denied, qa); err != nil {
		t.Fatalf("reason-only return was narrowed: %v", err)
	}
	f.stage.code(t, &f.candidate)
	if detail, err := read(f.ctx); err != nil || detail.Applicability != "stale" {
		t.Fatalf("superseded original artifact report disappeared: %+v %v", detail, err)
	}
	f.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, f.stage.pr, strings.Repeat("c", 40))
	if detail, err := read(f.ctx); err != nil || detail.Applicability != "stale" {
		t.Fatalf("authorized stale history disappeared: %+v %v", detail, err)
	}
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.reviewer)
	if _, err = read(f.stage.reviewerCtx); !collaborationErrorCode(err, "not_found") {
		t.Errorf("revoked manager read retained artifact report: %v", err)
	}
	if _, err = f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20}); !collaborationErrorCode(err, "not_found") {
		t.Errorf("revoked manager listed retained artifact report: %v", err)
	}
}

// This export exists only in the test build. The external service_test package
// can exercise real HTTP adapters using the same retained artifact/run fixture.
func StageArtifactReportTransportFixture(t *testing.T) (*CollaborationService, context.Context, context.Context, protocol.CollaborationTestReportReceipt, func()) {
	f, ack := stageArtifactReportFixture(t)
	return f.svc, f.stage.reviewerCtx, f.stage.qaCtx, ack, func() {
		if err := stageDemoteArtifactManager(t, f, f.stage.reviewer); err != nil {
			t.Fatal(err)
		}
	}
}

func stageDemoteArtifactManager(t *testing.T, f *stageRuntimeFixture, user string) error {
	t.Helper()
	member, err := f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: util.MustParseUUID(f.workspace), UserID: util.MustParseUUID(user)})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), "SET LOCAL lock_timeout='50ms'"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.New(tx).UpdateMemberRole(context.Background(), db.UpdateMemberRoleParams{ID: member.ID, Role: "member"}); err != nil {
		return err
	}
	return tx.Commit(context.Background())
}

func TestStageArtifactReportQAReferenceUsesCurrentManager(t *testing.T) {
	f, ack := stageArtifactReportFixture(t)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.qa)
	params := func() CollaborationStageReviewParams {
		return CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "return", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{}, Reason: "Synthetic manager QA reviews retained failure"}
	}
	p := params()
	if _, err := f.svc.DecideStage(f.stage.qaCtx, p); err != nil {
		t.Fatalf("authorized manager report reference: %v", err)
	}
	if err := stageDemoteArtifactManager(t, f, f.stage.qa); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.DecideStage(f.stage.qaCtx, params()); !collaborationErrorCode(err, "stage_evidence_unavailable") {
		t.Fatalf("revoked manager used cached QA reference: %v", err)
	}
	p = params()
	p.Evidence = CollaborationStageReviewEvidence{}
	if _, err := f.svc.DecideStage(f.stage.qaCtx, p); err != nil {
		t.Fatalf("reason-only return after artifact revocation: %v", err)
	}
}

func TestStageArtifactReportAuthorityLocksAcrossReadAndQA(t *testing.T) {
	for _, path := range []string{"list", "detail", "r4", "qa_reference"} {
		t.Run(path, func(t *testing.T) {
			f, ack := stageArtifactReportFixture(t)
			user, ctx := f.stage.reviewer, f.stage.reviewerCtx
			if path == "qa_reference" {
				user, ctx = f.stage.qa, f.stage.qaCtx
				f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, user)
			}
			query := "GetCollaborationStageCodeHead"
			if path == "qa_reference" {
				query = "CreateCollaborationStageReviewDecision"
			}
			hits := 0
			var demotion error
			// R4 reaches its code head before the artifact observer; report attempt
			// resolution is the first canonical row read after the observer and gate.
			if path == "r4" {
				query = "GetCollaborationAttempt"
			}
			f.svc.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: query, before: func() { hits++; demotion = stageDemoteArtifactManager(t, f, user) }}
			var err error
			switch path {
			case "list":
				_, err = f.svc.TestReports(ctx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), CollaborationTestReportsQuery{Limit: 20})
			case "detail":
				_, err = f.svc.TestReport(ctx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), util.MustParseUUID(ack.ReportID))
			case "r4":
				_, err = f.svc.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
			case "qa_reference":
				p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "return", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{}, Reason: "Synthetic concurrent QA review"}
				_, err = f.svc.DecideStage(ctx, p)
			}
			f.svc.TxStarter = f.pool
			var locked *pgconn.PgError
			if hits != 1 || !errors.As(demotion, &locked) || locked.Code != "55P03" || err != nil {
				t.Fatalf("%s did not retain authority: hook=%d demotion=%v read=%v", path, hits, demotion, err)
			}
			if err = stageDemoteArtifactManager(t, f, user); err != nil {
				t.Fatalf("demotion after completed read: %v", err)
			}
			if _, err = f.svc.TestReport(ctx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), util.MustParseUUID(ack.ReportID)); !collaborationErrorCode(err, "not_found") {
				t.Fatalf("cached ID after successful demotion: %v", err)
			}
		})
	}
}

func TestStageArtifactReportRevocationBeforeAuthorityLockDiscardsSnapshot(t *testing.T) {
	for _, path := range []string{"list", "detail"} {
		t.Run(path, func(t *testing.T) {
			f, ack := stageArtifactReportFixture(t)
			query := "GetCollaborationRecordedRun"
			hits := 0
			f.svc.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: query, before: func() {
				hits++
				if err := stageDemoteArtifactManager(t, f, f.stage.reviewer); err != nil {
					t.Fatal(err)
				}
			}}
			var err error
			if path == "list" {
				var page CollaborationTestReportsPage
				page, err = f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), CollaborationTestReportsQuery{Limit: 20})
				if !reflect.DeepEqual(page, CollaborationTestReportsPage{}) {
					t.Fatalf("revoked snapshot exposed a page: %+v", page)
				}
			} else {
				var detail CollaborationTestReportDetail
				detail, err = f.svc.TestReport(f.stage.reviewerCtx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), util.MustParseUUID(ack.ReportID))
				if !reflect.DeepEqual(detail, CollaborationTestReportDetail{}) {
					t.Fatalf("revoked snapshot exposed detail: %+v", detail)
				}
			}
			f.svc.TxStarter = f.pool
			var serialization *pgconn.PgError
			if hits != 1 || !errors.As(err, &serialization) || serialization.Code != "40001" {
				t.Fatalf("permission changed after snapshot was accepted: hooks=%d error=%v", hits, err)
			}
			if _, err = f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, util.MustParseUUID(ack.RunID), CollaborationTestReportsQuery{Limit: 20}); !collaborationErrorCode(err, "not_found") {
				t.Fatalf("fresh revoked read: %v", err)
			}
		})
	}
}

func TestStageArtifactReportCachedCursorDoesNotRestoreAuthority(t *testing.T) {
	f := newStageRuntimeFixture(t, true, stageArtifactReadFixture)
	f.fx.Exec(t, `UPDATE agent_runtime SET runtime_mode='local' WHERE id=$1`, f.runtime.ID)
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := hostCapabilityRegistration(f.managedCollaborationFixture)
	p.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: f.stage.head, BundleDigest: strings.Repeat("d", 64)}
	if _, err = f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err != nil {
		t.Fatal(err)
	}
	renewHostConsent(t, f.managedCollaborationFixture)
	gate, err := f.svc.Queries.GetCollaborationGate(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=$1 WHERE id=1`, gate.HostExecutionEnabled)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	f.gateEpoch = gate.Epoch + 1
	if _, err = f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
		t.Fatal(err)
	}
	run, claim := claimHostRun(t, f.managedCollaborationFixture, p.Capabilities)
	var firstReport string
	for attempt := 1; attempt <= 2; attempt++ {
		launch := hostPermitRequest(claim, attempt, "provider")
		permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, launch)
		if err != nil {
			t.Fatal(err)
		}
		subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
		if err != nil {
			t.Fatal(err)
		}
		ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, stageRuntimeReport(subject.Subject, claim.Identity))
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 1 {
			firstReport = ack.ReportID
			if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), run, claim.TerminationToken, hostLaunchEnded(launch, true)); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("manager did not receive a real continuation cursor: %+v %v", page, err)
	}
	cached := CollaborationTestReportsQuery{Limit: 1, Cursor: *page.NextCursor}
	older, err := f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, run, cached)
	if err != nil || len(older.Items) != 1 || older.Items[0].ReportID != firstReport {
		t.Fatalf("authorized original report page: %+v %v", older, err)
	}
	if err = stageDemoteArtifactManager(t, f, f.stage.reviewer); err != nil {
		t.Fatal(err)
	}
	denied, err := f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, run, cached)
	if !collaborationErrorCode(err, "not_found") || !reflect.DeepEqual(denied, CollaborationTestReportsPage{}) {
		t.Fatalf("cached cursor restored artifact access: %+v %v", denied, err)
	}
}

func TestStageArtifactReportEmptyListStillChecksFrozenSource(t *testing.T) {
	f := newStageRuntimeFixture(t, true, stageArtifactReadFixture)
	run, _, _ := startStageReportRuntime(t, f)
	allowed, err := f.svc.TestReports(f.stage.reviewerCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20})
	if err != nil || len(allowed.Items) != 0 || allowed.NextCursor != nil {
		t.Fatalf("authorized empty report page: %+v %v", allowed, err)
	}
	for _, cursor := range []string{"", "cached-but-unreadable"} {
		denied, err := f.svc.TestReports(f.stage.qaCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20, Cursor: cursor})
		if !collaborationErrorCode(err, "not_found") || !reflect.DeepEqual(denied, CollaborationTestReportsPage{}) {
			t.Fatalf("unreadable frozen artifact revealed an empty page/cursor result: %+v %v", denied, err)
		}
	}
}

func TestStageTestReportUnboundRunKeepsAuthorizedEmptyList(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	input := f.input(t)
	input.TestSubject = nil
	if _, err := f.svc.CreateComment(f.ctx, input); err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("unbound run: %+v %v", list, err)
	}
	run := util.MustParseUUID(list.RunIDs[0])
	page, err := f.svc.TestReports(f.stage.qaCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 20})
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != nil {
		t.Fatalf("unbound authorized empty run changed: %+v %v", page, err)
	}
}
