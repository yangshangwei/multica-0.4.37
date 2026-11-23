package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This combines real local synthetic code execution with service-owned records.
// PR/CI observations, managed preparation/consent/settlement and human accounts
// are fixtures, including the retired test-only ReconfigureTarget helper. This
// does not attest a production configuration Switch, daemon checkout, remote CI
// or payment-system repair.
func TestStageCodeRevisionRegressionPreservesFailedHistory(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("code revision regression requires an explicitly owned DATABASE_URL")
	}
	repo := tc12NewRepository(t)
	a := repo.commit(t, "a", "Retain a duplicate-callback defect for regression proof")
	f := newStageRuntimeFixture(t, false, func(t *testing.T, s *stageReviewFixture) {
		tc12SetObservedHead(t, s, a.Commit)
	})
	tc12RegisterSource(t, f, a)
	first := tc12ExecuteReport(t, f, repo, a, "failed", 1)
	if first.Detail.Applicability != "execution_failed" || first.Detail.FailedCaseCount != 1 {
		t.Fatalf("failed execution not linked to report: %+v", first.Detail)
	}
	qaReturn := tc12QAParams(t, f, first.Ack.ReportID, "return", "")
	returned, err := f.svc.DecideStage(f.stage.qaCtx, qaReturn)
	if err != nil || returned.StageConfirmation == nil || returned.StageConfirmation.Decision != "returned" {
		t.Fatalf("exact failed report QA return: %+v %v", returned, err)
	}
	returnReceipt, err := f.svc.GetWorkflowOperation(f.stage.qaCtx, f.issue.Issue.ID, qaReturn.RequestID)
	if err != nil || returnReceipt.Status != returned.Status || returnReceipt.Code != returned.Code || returnReceipt.CollaborationRevision != returned.CollaborationRevision || !reflect.DeepEqual(returned.StageConfirmation, returnReceipt.StageConfirmation) {
		t.Fatalf("original QA return receipt: %+v %v", returnReceipt, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "development", returned.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	oldCandidate := f.candidate
	var oldOperations, oldEvents []string
	f.fx.QueryRow(t, `SELECT array_agg(id::text ORDER BY id) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID).Scan(&oldOperations)
	f.fx.QueryRow(t, `SELECT array_agg(id::text ORDER BY id) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID).Scan(&oldEvents)
	oldFacts := tc12History(t, f, first, oldCandidate, oldOperations, oldEvents)
	repo.save(t, "a-platform-history-before.json", oldFacts)
	repo.save(t, "a-human-return.json", map[string]any{"request": qaReturn, "receipt": returnReceipt})

	// Only the implementation changes. The regression oracle and input stay fixed.
	repo.write(t, "business.py", tc12FixedBusiness)
	b := repo.commit(t, "b", "Prevent repeated fulfillment of one synthetic callback")
	if b.Parent != a.Commit || b.Commit == a.Commit || b.Tree == a.Tree || b.BundleSHA256 == a.BundleSHA256 ||
		b.Files["business.py"] == a.Files["business.py"] || b.Files["runner.py"] != a.Files["runner.py"] || b.Files["input.json"] != a.Files["input.json"] {
		t.Fatal("revision B must change only business code with A as its direct parent")
	}
	if changed := repo.gitOK(t, "diff", "--name-only", a.Commit, b.Commit); changed != "business.py\n" {
		t.Fatalf("repair changed the regression oracle or input: %q", changed)
	}
	patch := repo.gitOK(t, "diff", "--no-ext-diff", "--no-textconv", a.Commit, b.Commit, "--", "business.py")
	repo.save(t, "code-change.json", map[string]any{"before": a, "after": b, "patch": patch, "patch_sha256": tc12Hash([]byte(patch))})
	tc12SetObservedHead(t, f.stage, b.Commit)
	next := f.stage.code(t, &oldCandidate)
	if next.CandidateID == oldCandidate.CandidateID || next.Version != oldCandidate.Version+1 || next.ContentDigest == oldCandidate.ContentDigest {
		t.Fatal("repair must append a distinct versioned code candidate")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_candidate WHERE candidate_id=$1 AND parent_candidate_id=$2 AND version=$3`, next.CandidateID, oldCandidate.CandidateID, next.Version) != 1 {
		t.Fatal("new candidate lost its exact predecessor")
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, next, "code_review")); err != nil {
		t.Fatal(err)
	}
	review, err := f.svc.DecideStage(f.stage.reviewerCtx, f.stage.reviewParams(t, next, "approve"))
	if err != nil || review.StageConfirmation == nil {
		t.Fatalf("fresh review of B: %+v %v", review, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, next, "qa", review.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	f.candidate = next
	// A settled run does not imply that its target is paused for reconfiguration.
	stopped, err := f.svc.StopQueue(f.ctx, CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget,
		Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f.collaborationQueueFixture, f.targetID))}})
	if err != nil || stopped.QueueState != "paused" {
		t.Fatalf("explicit target stop before source revision change: %+v %v", stopped, err)
	}
	repo.save(t, "b-explicit-target-stop.json", stopped)
	tc12RegisterSource(t, f, b)
	second := tc12ExecuteReport(t, f, repo, b, "passed", 0)
	if second.Run == first.Run || second.Command == first.Command || second.Ack.ReportID == first.Ack.ReportID ||
		second.Report.RequestID == first.Report.RequestID || second.Claim.Identity.LaunchNonce == first.Claim.Identity.LaunchNonce ||
		second.Report.Report.Subject.BindingDigest == first.Report.Report.Subject.BindingDigest || second.Detail.Applicability != "usable" || second.Detail.FailedCaseCount != 0 {
		t.Fatal("regression must have a distinct bound run/report and usable passing result")
	}
	if second.Report.Report.SuiteID != first.Report.Report.SuiteID || second.Report.Report.Cases[0].ID != first.Report.Report.Cases[0].ID {
		t.Fatal("regression no longer targets the original failing case")
	}

	// The stale report refusal records its own recoverable operation, but no
	// decision, event, run, candidate or other business state may change.
	wrong := tc12QAParams(t, f, first.Ack.ReportID, "approve", review.StageConfirmation.DecisionID)
	beforeWrong := tc13QAFacts(t, f)
	refusal, err := f.svc.DecideStage(f.stage.qaCtx, wrong)
	if !collaborationErrorCode(err, "stage_evidence_changed") || refusal.Status != 409 || refusal.Code != "stage_evidence_changed" {
		t.Fatalf("B borrowed A's report: %+v %v", refusal, err)
	}
	afterWrong := tc13QAFacts(t, f)
	var unchangedOperations, addedOperation string
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM collaboration_operation x WHERE scope_kind='issue' AND scope_id=$1 AND NOT (actor_user_id=$2 AND request_id=$3)`, f.issue.Issue.ID, f.stage.qa, wrong.RequestID).Scan(&unchangedOperations)
	if unchangedOperations != beforeWrong["collaboration_operation"] {
		t.Fatal("wrong-revision request changed another original operation")
	}
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM collaboration_operation x WHERE scope_kind='issue' AND scope_id=$1 AND actor_user_id=$2 AND request_id=$3`, f.issue.Issue.ID, f.stage.qa, wrong.RequestID).Scan(&addedOperation)
	var added []json.RawMessage
	if err = json.Unmarshal([]byte(addedOperation), &added); err != nil || len(added) != 1 {
		t.Fatalf("wrong-revision request did not record exactly one refusal: %v", err)
	}
	addedRow, err := f.svc.Queries.GetCollaborationOperation(f.stage.qaCtx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(f.stage.qa), RequestID: wrong.RequestID})
	if err != nil || addedRow.Kind != stageKindDecide || addedRow.ResultCode.String != "stage_evidence_changed" {
		t.Fatalf("wrong-revision refusal identity: %v", err)
	}
	storedRefusal, err := decodeWorkflowReceipt(addedRow)
	if err != nil || !reflect.DeepEqual(storedRefusal, refusal) {
		t.Fatalf("stored refusal differs from the exact response: %v", err)
	}
	repo.save(t, "wrong-revision-operations.json", map[string]any{"before": json.RawMessage(beforeWrong["collaboration_operation"]), "after": json.RawMessage(afterWrong["collaboration_operation"]), "added": json.RawMessage(addedOperation)})
	delete(beforeWrong, "collaboration_operation")
	delete(afterWrong, "collaboration_operation")
	if !reflect.DeepEqual(beforeWrong, afterWrong) {
		t.Fatal("wrong-revision QA request changed business facts")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND request_id=$2 AND result_code='stage_evidence_changed'`, f.issue.Issue.ID, wrong.RequestID) != 1 {
		t.Fatal("wrong-revision refusal lost its one original operation")
	}
	recoveredRefusal, err := f.svc.GetWorkflowOperation(f.stage.qaCtx, f.issue.Issue.ID, wrong.RequestID)
	if err != nil || recoveredRefusal.Status != refusal.Status || recoveredRefusal.Code != refusal.Code || recoveredRefusal.CollaborationRevision != refusal.CollaborationRevision || !reflect.DeepEqual(recoveredRefusal.StageConfirmation, refusal.StageConfirmation) {
		t.Fatalf("wrong-revision refusal recovery: %+v %v", recoveredRefusal, err)
	}
	repo.save(t, "wrong-revision-refusal.json", map[string]any{"request": wrong, "receipt": refusal, "business_before": beforeWrong, "business_after": afterWrong})

	qaApprove := tc12QAParams(t, f, second.Ack.ReportID, "approve", review.StageConfirmation.DecisionID)
	approved, err := f.svc.DecideStage(f.stage.qaCtx, qaApprove)
	if err != nil || approved.StageConfirmation == nil || approved.StageConfirmation.Decision != "approved" || approved.StageConfirmation.CandidateID != util.UUIDToString(next.CandidateID) {
		t.Fatalf("fresh QA approval of repaired code: %+v %v", approved, err)
	}
	decision, err := f.svc.Queries.GetCollaborationStageDecision(f.ctx, db.GetCollaborationStageDecisionParams{IssueID: f.issue.Issue.ID, DecisionID: util.MustParseUUID(approved.StageConfirmation.DecisionID)})
	if err != nil || decision.ActorUserID != util.MustParseUUID(f.stage.qa) || decision.CandidateID != next.CandidateID {
		t.Fatalf("QA approval identity: %v", err)
	}
	var payload stageReviewPayload
	if err = json.Unmarshal(decision.Payload, &payload); err != nil || payload.Evidence.TestReportID == nil || *payload.Evidence.TestReportID != second.Ack.ReportID || !reflect.DeepEqual(payload.DependsOn, []string{review.StageConfirmation.DecisionID}) {
		t.Fatalf("QA approved another report or review dependency: %v", err)
	}
	if f.stage.view(t).Stage.Current != "qa" {
		t.Fatal("report or approval silently advanced beyond QA")
	}

	afterB := tc12History(t, f, first, oldCandidate, oldOperations, oldEvents)
	repo.save(t, "a-platform-history-after-b.json", afterB)
	if !reflect.DeepEqual(oldFacts, afterB) {
		t.Fatal("repair/regression rewrote the original failed history")
	}
	oldDetail, err := f.svc.TestReport(f.ctx, f.issue.Issue.ID, first.Run, util.MustParseUUID(first.Ack.ReportID))
	if err != nil || oldDetail.Applicability != "stale" {
		t.Fatalf("old failed report must remain readable and now stale: %+v %v", oldDetail, err)
	}
	repo.save(t, "a-historical-detail.json", oldDetail)
	oldDetail.Applicability = first.Detail.Applicability
	if !reflect.DeepEqual(oldDetail, first.Detail) {
		t.Fatal("historical report changed beyond its live applicability")
	}
	replayed, err := f.svc.RecordRuntimeTestReport(context.Background(), first.Run, first.Permit.BusinessToken, first.Report)
	if err != nil || replayed != first.Ack {
		t.Fatalf("old report same-body/key replay: %+v %v", replayed, err)
	}
	reportReceipt, err := f.svc.RuntimeTestReportReceipt(context.Background(), first.Run, first.Permit.BusinessToken, first.Report.RequestID)
	if err != nil || reportReceipt != first.Ack {
		t.Fatalf("old report original receipt: %+v %v", reportReceipt, err)
	}
	qaRecovered, err := f.svc.GetWorkflowOperation(f.stage.qaCtx, f.issue.Issue.ID, qaReturn.RequestID)
	if err != nil || !reflect.DeepEqual(qaRecovered, returnReceipt) {
		t.Fatalf("old QA return recovery: %+v %v", qaRecovered, err)
	}
	qaReplayed, err := f.svc.DecideStage(f.stage.qaCtx, qaReturn)
	if err != nil || !reflect.DeepEqual(qaReplayed, returned) {
		t.Fatalf("old QA return same-body/key replay: %+v %v", qaReplayed, err)
	}
	afterRecovery := tc12History(t, f, first, oldCandidate, oldOperations, oldEvents)
	repo.save(t, "a-platform-history-after-recovery.json", afterRecovery)
	if !reflect.DeepEqual(oldFacts, afterRecovery) || f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, first.Run) != 1 {
		t.Fatal("original recovery duplicated or rewrote failed history")
	}
	for _, revision := range []tc12Revision{a, b} {
		if tc12FileHash(t, filepath.Join(repo.root, revision.Label+".bundle")) != revision.BundleSHA256 {
			t.Fatal("retained source bundle changed")
		}
	}
	repo.save(t, "final-proof.json", map[string]any{
		"scope":    "local synthetic code execution and service association; fixture PR/CI, managed preparation/consent/settlement (retired test-only reconfiguration); no production Switch, daemon checkout or real payment",
		"issue_id": util.UUIDToString(f.issue.Issue.ID), "candidate_a": oldCandidate, "candidate_b": next,
		"a": first.safe(), "b": second.safe(), "qa_return": returnReceipt, "qa_approval": approved, "qa_approval_row": decision,
		"old_operation_ids": oldOperations, "old_event_ids": oldEvents, "historical_collections": len(oldFacts), "history_unchanged": true,
		"report_replay": replayed, "report_receipt": reportReceipt, "qa_recovery": qaRecovered, "qa_replay": qaReplayed,
	})
	t.Logf("TC12 proof retained at %s; failed=%s repaired=%s; one source-bound case, two independent runs", repo.root, a.Commit, b.Commit)
}

func tc12SetObservedHead(t *testing.T, s *stageReviewFixture, commit string) {
	t.Helper()
	s.head = commit
	s.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$2,snapshot_head_sha=$2,snapshot_fetched_at=clock_timestamp()-interval '1 minute',snapshot_complete=true WHERE id=$1`, s.pr, commit)
	s.fx.Exec(t, `UPDATE github_pull_request_check_run SET head_sha=$2 WHERE pr_id=$1`, s.pr, commit)
}

func tc12RegisterSource(t *testing.T, f *stageRuntimeFixture, revision tc12Revision) {
	t.Helper()
	p := preparationRegistrationRequest(f.managedCollaborationFixture)
	p.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: revision.Commit, BundleDigest: revision.BundleSHA256}
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	refreshPreparationConsent(t, f.managedCollaborationFixture)
}

type tc12Run struct {
	Run, Command pgtype.UUID
	Input        CollaborationCreateCommentParams
	Comment      CollaborationCommentReceipt
	Claim        protocol.CollaborationRunClaimResponse
	Permit       protocol.CollaborationRunPermitResponse
	Report       protocol.CollaborationTestReportRequest
	Ack          protocol.CollaborationTestReportReceipt
	Detail       CollaborationTestReportDetail
}

func (r tc12Run) safe() map[string]any {
	return map[string]any{"run_id": util.UUIDToString(r.Run), "command_id": util.UUIDToString(r.Command), "input": r.Input, "comment": r.Comment,
		"identity": r.Claim.Identity, "preparation": r.Claim.Preparation, "report": r.Report, "ack": r.Ack, "detail": r.Detail}
}

func tc12ExecuteReport(t *testing.T, f *stageRuntimeFixture, repo *tc12Repository, revision tc12Revision, outcome string, wantExit int) tc12Run {
	t.Helper()
	r := tc12Run{Input: f.input(t)}
	var err error
	r.Comment, err = f.svc.CreateComment(f.ctx, r.Input)
	if err != nil || len(r.Comment.Targets) != 1 || !r.Comment.Targets[0].CommandID.Valid {
		t.Fatalf("typed regression input: %+v %v", r.Comment, err)
	}
	r.Command = r.Comment.Targets[0].CommandID
	// Target Resume requires the enclosing issue to be active first.
	f.resumeManaged(t)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if target.QueueState == "paused" {
		resume, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: CollaborationStopQueueParams{
			CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{targetControl(target)}}, ExpectedExecutionEpoch: f.gateEpoch})
		if err != nil {
			t.Fatal(err)
		}
		repo.save(t, revision.Label+"-explicit-target-resume.json", resume)
	}
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("one actual regression run: %+v %v", list, err)
	}
	r.Run = util.MustParseUUID(list.RunIDs[0])
	_, r.Claim = claimPreparedCandidate(t, f.managedCollaborationFixture, r.Run)
	_, r.Permit = permitPreparedClaim(t, f.managedCollaborationFixture, r.Run, r.Claim)
	if r.Claim.Preparation.Git == nil || r.Claim.Preparation.Git.CommitOID != revision.Commit || r.Claim.Preparation.Git.BundleDigest != revision.BundleSHA256 || !reflect.DeepEqual(r.Claim.Preparation.CommandIDs, []string{util.UUIDToString(r.Command)}) {
		t.Fatal("claim preparation lost the actual revision or typed command")
	}
	subject, err := f.svc.RuntimeTestSubject(context.Background(), r.Run, r.Permit.BusinessToken)
	if err != nil || subject.Subject.SourceRevision != revision.Commit || subject.Subject.CandidateID != util.UUIDToString(f.candidate.CandidateID) || subject.Subject.CandidateVersion != stageVersion(f.candidate.Version) || subject.Subject.CandidateDigest != f.candidate.ContentDigest {
		t.Fatalf("source-bound test subject: %+v %v", subject, err)
	}
	result := repo.runRevision(t, revision, wantExit)
	reported, settled := "passed", "success"
	if result.Exit != 0 {
		reported, settled = "failed", "failure"
	}
	if reported != outcome {
		t.Fatal("observed local result differs from the expected regression phase")
	}
	provenance, err := json.Marshal(map[string]any{"revision": revision, "execution": result})
	if err != nil {
		t.Fatal(err)
	}
	r.Report = protocol.CollaborationTestReportRequest{Version: 1, Identity: r.Claim.Identity, RequestID: uuid.NewString(), Report: protocol.CollaborationTestReport{
		Subject: subject.Subject, SuiteID: "local-code-revision-regression", Cases: []protocol.CollaborationTestCase{{ID: "duplicate-callback-once", Outcome: reported, LogIndex: 0}},
		Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: result.Stdout}, {Kind: "inline", Body: string(provenance)}}}}
	r.Ack, err = f.svc.RecordRuntimeTestReport(context.Background(), r.Run, r.Permit.BusinessToken, r.Report)
	if err != nil || !r.Ack.Accepted {
		t.Fatalf("record actual execution: %+v %v", r.Ack, err)
	}
	finishSquadRun(t, f.managedCollaborationFixture, r.Run, r.Claim, r.Permit.BusinessToken, r.Permit.AttemptNonce, settled)
	r.Detail, err = f.svc.TestReport(f.ctx, f.issue.Issue.ID, r.Run, util.MustParseUUID(r.Ack.ReportID))
	if err != nil || r.Detail.ReportedResult != outcome || r.Detail.SourceKind != "runtime_reported" || !reflect.DeepEqual(r.Detail.Subject, subject.Subject) || !reflect.DeepEqual(r.Detail.Cases, r.Report.Report.Cases) || len(r.Detail.Logs) != 2 || r.Detail.Logs[0].Body != result.Stdout || r.Detail.Logs[1].Body != string(provenance) {
		t.Fatalf("actual result/case/log/source association: %+v %v", r.Detail, err)
	}
	view := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
	if len(view.TestReports) != 1 || view.TestReports[0].ReportID != r.Ack.ReportID || view.TestReports[0].RunID != util.UUIDToString(r.Run) {
		t.Fatalf("automatic candidate report association: %+v", view.TestReports)
	}
	stored, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, r.Run)
	if err != nil || !stored.SettledAt.Valid || stored.BusinessOutcome.String != settled || !collaborationRunProvenEnded(stored) {
		t.Fatalf("report run not settled with observed execution result: %v", err)
	}
	repo.save(t, revision.Label+"-platform-report.json", r.safe())
	return r
}

func tc12QAParams(t *testing.T, f *stageRuntimeFixture, report, decision, review string) CollaborationStageReviewParams {
	t.Helper()
	p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: f.stage.policyRevision,
		ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		Step: "qa", Decision: decision, Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &report}, DependsOn: []string{}, Reason: "Human QA checks the exact local regression report"}
	if decision == "approve" {
		expiry := f.stage.evidence(t, f.stage.qaCtx, f.candidate).CheckedAt.Add(time.Minute).Truncate(time.Microsecond)
		p.ExpiresAt, p.DependsOn = &expiry, []string{review}
	}
	return p
}

func tc12History(t *testing.T, f *stageRuntimeFixture, run tc12Run, candidate CollaborationStageCandidateRef, operations, events []string) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for table, selection := range map[string]struct {
		predicate string
		arg       any
	}{
		"collaboration_run": {"id=$1", run.Run}, "collaboration_attempt": {"run_id=$1", run.Run}, "collaboration_run_event": {"run_id=$1", run.Run},
		"collaboration_termination": {"run_id=$1", run.Run}, "collaboration_business_capability": {"run_id=$1", run.Run},
		"collaboration_command": {"id=$1", run.Command}, "collaboration_test_subject": {"command_id=$1", run.Command},
		"collaboration_stage_candidate": {"candidate_id=$1", candidate.CandidateID}, "collaboration_stage_decision": {"candidate_id=$1", candidate.CandidateID},
		"collaboration_operation": {"id=ANY($1::uuid[])", operations}, "collaboration_event": {"id=ANY($1::uuid[])", events}, "comment": {"id=$1", run.Comment.CommentID},
	} {
		var raw string
		f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x WHERE `+selection.predicate, selection.arg).Scan(&raw)
		if raw == "[]" {
			t.Fatalf("missing retained historical collection %s", table)
		}
		out[table] = json.RawMessage(raw)
	}
	return out
}

const tc12BrokenBusiness = `def apply_callbacks(events):
    state = "pending"
    fulfilled = 0
    for event in events:
        if event["status"] == "succeeded":
            state = "paid"
            fulfilled += 1
    return {"state": state, "fulfilled": fulfilled}
`

const tc12FixedBusiness = `def apply_callbacks(events):
    state = "pending"
    fulfilled = 0
    seen = set()
    for event in events:
        if event["status"] == "succeeded" and event["id"] not in seen:
            seen.add(event["id"])
            state = "paid"
            fulfilled += 1
    return {"state": state, "fulfilled": fulfilled}
`

const tc12Runner = `import json
import sys
from business import apply_callbacks

with open(sys.argv[1], encoding="utf-8") as source:
    events = json.load(source)
actual = apply_callbacks(events)
expected = {"state": "paid", "fulfilled": 1}
print(json.dumps({"case": "duplicate-callback-once", "actual": actual, "expected": expected}, sort_keys=True))
sys.exit(0 if actual == expected else 1)
`

type tc12Repository struct {
	root, dir, git, python string
	env                    []string
	commands               int
}

type tc12Revision struct {
	Label        string            `json:"label"`
	Commit       string            `json:"commit"`
	Parent       string            `json:"parent"`
	Tree         string            `json:"tree"`
	Files        map[string]string `json:"files_sha256"`
	BundleSHA256 string            `json:"bundle_sha256"`
}

type tc12Execution struct {
	Argv       []string `json:"argv"`
	Cwd        string   `json:"cwd"`
	ToolSHA256 string   `json:"tool_sha256"`
	Exit       int      `json:"exit"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr"`
	PID        int      `json:"pid"`
	Waited     bool     `json:"waited"`
}

func tc12NewRepository(t *testing.T) *tc12Repository {
	t.Helper()
	root := t.TempDir()
	if retained := os.Getenv("PAY241_TC12_EVIDENCE_DIR"); retained != "" {
		if !filepath.IsAbs(retained) {
			t.Fatal("PAY241_TC12_EVIDENCE_DIR must be an absolute private directory")
		}
		root = filepath.Join(retained, t.Name())
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	r := &tc12Repository{root: root, dir: filepath.Join(root, "repository")}
	for _, path := range []string{r.dir, filepath.Join(root, "home"), filepath.Join(root, "commands")} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	tool := func(name string) string {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	r.git, r.python = tool("git"), tool("python3")
	r.env = []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.Join(root, "home"), "LANG=C", "LC_ALL=C", "TZ=UTC", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "PYTHONDONTWRITEBYTECODE=1", "PYTHONNOUSERSITE=1",
		"GIT_AUTHOR_NAME=PAY241 synthetic fixture", "GIT_AUTHOR_EMAIL=pay241-fixture@example.test", "GIT_COMMITTER_NAME=PAY241 synthetic fixture", "GIT_COMMITTER_EMAIL=pay241-fixture@example.test", "GIT_AUTHOR_DATE=2026-11-20T00:00:00Z", "GIT_COMMITTER_DATE=2026-11-20T00:00:00Z"}
	r.save(t, "tools.json", map[string]any{"git": r.git, "git_sha256": tc12FileHash(t, r.git), "python": r.python, "python_sha256": tc12FileHash(t, r.python), "environment": r.env, "git_version": r.command(t, r.git, "--version"), "python_version": r.command(t, r.python, "--version")})
	r.gitOK(t, "init", "--template=", "--initial-branch=proof")
	r.write(t, "business.py", tc12BrokenBusiness)
	r.write(t, "runner.py", tc12Runner)
	r.write(t, "input.json", "[{\"id\":\"callback-1\",\"status\":\"succeeded\"},{\"id\":\"callback-1\",\"status\":\"succeeded\"},{\"id\":\"callback-1\",\"status\":\"succeeded\"}]\n")
	return r
}

func (r *tc12Repository) write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *tc12Repository) save(t *testing.T, name string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(r.root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(raw, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("retain evidence: write=%v close=%v", writeErr, closeErr)
	}
}

func (r *tc12Repository) command(t *testing.T, tool string, args ...string) tc12Execution {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir, cmd.Env = r.dir, r.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := tc12Execution{Argv: append([]string{tool}, args...), Cwd: r.dir, ToolSHA256: tc12FileHash(t, tool), Stdout: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		result.PID, result.Exit, result.Waited = cmd.ProcessState.Pid(), cmd.ProcessState.ExitCode(), true
	}
	r.commands++
	r.save(t, fmt.Sprintf("commands/%03d.json", r.commands), result)
	var exited *exec.ExitError
	if ctx.Err() != nil || err != nil && !errors.As(err, &exited) || !result.Waited {
		t.Fatalf("local command did not terminate normally: %v", err)
	}
	return result
}

func (r *tc12Repository) gitOK(t *testing.T, args ...string) string {
	t.Helper()
	args = append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "commit.gpgsign=false"}, args...)
	result := r.command(t, r.git, args...)
	if result.Exit != 0 {
		t.Fatalf("local Git command failed: %s", result.Stderr)
	}
	return result.Stdout
}

func (r *tc12Repository) commit(t *testing.T, label, intent string) tc12Revision {
	t.Helper()
	r.gitOK(t, "add", "--", "business.py", "runner.py", "input.json")
	r.gitOK(t, "commit", "--no-gpg-sign", "-m", intent+"\n\nPreserve the local synthetic revision as reproducible acceptance input.\n\nConstraint: No remote repository or payment service\nConfidence: high\nScope-risk: narrow\nNot-tested: Real payment behavior")
	v := tc12Revision{Label: label, Commit: strings.TrimSpace(r.gitOK(t, "rev-parse", "HEAD")), Tree: strings.TrimSpace(r.gitOK(t, "rev-parse", "HEAD^{tree}")), Files: map[string]string{}}
	parents := strings.Fields(r.gitOK(t, "rev-list", "--parents", "-n", "1", "HEAD"))
	if len(parents) == 2 {
		v.Parent = parents[1]
	} else if len(parents) != 1 {
		t.Fatal("synthetic source unexpectedly has multiple parents")
	}
	for _, name := range []string{"business.py", "runner.py", "input.json"} {
		v.Files[name] = tc12FileHash(t, filepath.Join(r.dir, name))
		if tc12Hash([]byte(r.gitOK(t, "show", "HEAD:"+name))) != v.Files[name] {
			t.Fatal("executed source differs from its committed blob")
		}
	}
	bundle := filepath.Join(r.root, label+".bundle")
	r.gitOK(t, "bundle", "create", bundle, "HEAD")
	r.gitOK(t, "bundle", "verify", bundle)
	v.BundleSHA256 = tc12FileHash(t, bundle)
	if r.gitOK(t, "status", "--porcelain") != "" || r.gitOK(t, "remote") != "" {
		t.Fatal("synthetic source must be clean and have no remote")
	}
	r.save(t, label+"-revision.json", v)
	return v
}

func (r *tc12Repository) runRevision(t *testing.T, revision tc12Revision, wantExit int) tc12Execution {
	t.Helper()
	if strings.TrimSpace(r.gitOK(t, "rev-parse", "HEAD")) != revision.Commit || r.gitOK(t, "status", "--porcelain") != "" {
		t.Fatal("execution source is not the recorded clean revision")
	}
	result := r.command(t, r.python, "-B", "-s", "runner.py", "input.json")
	fulfilled := 1
	if wantExit == 1 {
		fulfilled = 3
	}
	wantOutput := fmt.Sprintf("{\"actual\": {\"fulfilled\": %d, \"state\": \"paid\"}, \"case\": \"duplicate-callback-once\", \"expected\": {\"fulfilled\": 1, \"state\": \"paid\"}}\n", fulfilled)
	if result.Exit != wantExit || result.Stdout != wantOutput || result.Stderr != "" {
		t.Fatalf("actual synthetic regression: exit=%d stdout=%q stderr=%q", result.Exit, result.Stdout, result.Stderr)
	}
	for name, digest := range revision.Files {
		if tc12FileHash(t, filepath.Join(r.dir, name)) != digest {
			t.Fatal("regression command modified source or oracle")
		}
	}
	if r.gitOK(t, "status", "--porcelain") != "" {
		t.Fatal("regression command left untracked or modified source")
	}
	r.save(t, revision.Label+"-execution.json", result)
	return result
}

func tc12FileHash(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return tc12Hash(raw)
}

func tc12Hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
