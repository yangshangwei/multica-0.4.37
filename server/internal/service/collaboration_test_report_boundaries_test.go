package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

func startStageReportRuntime(t *testing.T, f *stageRuntimeFixture) (pgtype.UUID, protocol.CollaborationRunClaimResponse, protocol.CollaborationRunPermitResponse) {
	t.Helper()
	if _, err := f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("runs: %+v %v", list, err)
	}
	run := util.MustParseUUID(list.RunIDs[0])
	_, claim := claimPreparedCandidate(t, f.managedCollaborationFixture, run)
	_, permit := permitPreparedClaim(t, f.managedCollaborationFixture, run, claim)
	return run, claim, permit
}

func stageRuntimeReport(subject protocol.CollaborationTestSubject, identity protocol.CollaborationRunIdentity) protocol.CollaborationTestReportRequest {
	return protocol.CollaborationTestReportRequest{Version: 1, Identity: identity, RequestID: uuid.NewString(), Report: protocol.CollaborationTestReport{Subject: subject, SuiteID: "synthetic regression", Cases: []protocol.CollaborationTestCase{{ID: "callback", Outcome: "failed", LogIndex: 0}}, Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: "actual synthetic failure"}}}}
}

func TestStageTestSubjectClaimRequiresMatchingGit(t *testing.T) {
	for _, kind := range []string{"missing", "mismatch"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageRuntimeFixture(t, kind != "missing")
			if kind == "mismatch" {
				p := preparationRegistrationRequest(f.managedCollaborationFixture)
				p.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: strings.Repeat("c", 40), BundleDigest: strings.Repeat("d", 64)}
				if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err != nil {
					t.Fatal(err)
				}
				refreshPreparationConsent(t, f.managedCollaborationFixture)
			}
			if _, err := f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
				t.Fatal(err)
			}
			f.resumeManaged(t)
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 1 {
				t.Fatalf("runs: %+v %v", list, err)
			}
			run := util.MustParseUUID(list.RunIDs[0])
			p := preparationClaimRequest(f.managedCollaborationFixture)
			p.PreparationVersion = 1
			p.BundleDigest = strings.Repeat("a", 64)
			_, err = f.svc.ClaimRuntimeRun(f.daemonCtx, run, p)
			want := "test_source_unbound"
			if kind == "mismatch" {
				want = "test_subject_mismatch"
			}
			if !collaborationErrorCode(err, want) {
				t.Fatalf("claim %s: %v", kind, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1`, run) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND prompt_snapshot IS NOT NULL`, run) != 0 {
				t.Fatal("refused source left a partial claim")
			}
		})
	}
}

func TestStageTestReportOriginalCandidateConcurrentRecoveryAndPrivacy(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	run, claim, permit := startStageReportRuntime(t, f)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	p := stageRuntimeReport(subject.Subject, claim.Identity)
	syntheticSecret := "mcb_" + strings.Repeat("a", 64)
	p.Report.Cases[0].ID = "case " + syntheticSecret
	p.Report.Logs[0].Body = "output " + syntheticSecret
	changed := p
	changed.Report.Subject.CandidateID = uuid.NewString()
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, changed); !collaborationErrorCode(err, "test_subject_mismatch") {
		t.Fatalf("caller rebound subject: %v", err)
	}
	f.stage.code(t, &f.candidate)
	var group sync.WaitGroup
	acks := make(chan protocol.CollaborationTestReportReceipt, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
			acks <- ack
			errs <- err
		}()
	}
	group.Wait()
	close(acks)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first protocol.CollaborationTestReportReceipt
	for ack := range acks {
		if first.ReportID == "" {
			first = ack
		} else if ack != first {
			t.Fatal("concurrent receipt identity changed")
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, run) != 1 {
		t.Fatal("concurrent duplicate report")
	}
	detail, err := f.svc.TestReport(f.ctx, f.issue.Issue.ID, run, util.MustParseUUID(first.ReportID))
	if err != nil || detail.Applicability != "stale" || detail.Subject.CandidateID != subject.Subject.CandidateID {
		t.Fatalf("old candidate report relabelled: %+v %v", detail, err)
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), syntheticSecret) || detail.Cases[0].ID != redact.Text(p.Report.Cases[0].ID) || detail.Logs[0].Body != redact.Text(p.Report.Logs[0].Body) {
		t.Fatal("inline report free text was not redacted")
	}
	if _, err = f.svc.TestReport(f.stage.handlerCtx, f.issue.Issue.ID, run, util.MustParseUUID(first.ReportID)); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("external source read: %v", err)
	}
	if _, err = f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
	if err != nil || replay != first {
		t.Fatalf("stopped original ACK: %+v %v", replay, err)
	}
	if _, err = f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken); err == nil {
		t.Fatal("Stop retained subject content access")
	}
	if _, err = f.svc.RuntimeTestReportReceipt(context.Background(), run, permit.BusinessToken, uuid.NewString()); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("unknown original key: %v", err)
	}
	if _, err = f.svc.RuntimeTestReportReceipt(context.Background(), run, claim.TerminationToken, p.RequestID); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("termination credential read business receipt: %v", err)
	}
}

func TestStageTestSubjectDistinctCandidatesNeverChooseNewest(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	if _, err := f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
		t.Fatal(err)
	}
	qa := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "return", Candidate: f.candidate, DependsOn: []string{}, Reason: "Synthetic new test source"}
	returned, err := f.svc.DecideStage(f.stage.qaCtx, qa)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "development", returned.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	next := f.stage.code(t, &f.candidate)
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, next, "code_review")); err != nil {
		t.Fatal(err)
	}
	approved, err := f.svc.DecideStage(f.stage.reviewerCtx, f.stage.reviewParams(t, next, "approve"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, next, "qa", approved.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	f.candidate = next
	if _, err = f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
		t.Fatal(err)
	}
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("runs: %+v %v", list, err)
	}
	run := util.MustParseUUID(list.RunIDs[0])
	request := preparationClaimRequest(f.managedCollaborationFixture)
	request.PreparationVersion = 1
	request.BundleDigest = strings.Repeat("a", 64)
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, run, request); !collaborationErrorCode(err, "test_subject_ambiguous") {
		t.Fatalf("distinct subjects selected newest: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_subject'`, run) != 0 {
		t.Fatal("ambiguous run froze a subject")
	}
}

func TestStageTestReportLegacyRunCannotAcquireSubject(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "legacy input")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, run, claim)
	if _, err := f.svc.RuntimeTestSubject(context.Background(), run, token); !collaborationErrorCode(err, "test_subject_unavailable") {
		t.Fatalf("legacy run received a subject: %v", err)
	}
	subject := protocol.CollaborationTestSubject{CandidateID: uuid.NewString(), CandidateVersion: "1", CandidateDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64), SourceRevision: strings.Repeat("c", 40)}
	if _, err := f.svc.RecordRuntimeTestReport(context.Background(), run, token, stageRuntimeReport(subject, claim.Identity)); !collaborationErrorCode(err, "test_subject_unavailable") {
		t.Fatalf("legacy producer filled source: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, run) != 0 {
		t.Fatal("legacy run produced typed report")
	}
}

func TestStageTestReportLostCommitRecoversOneOriginalEvent(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	run, claim, permit := startStageReportRuntime(t, f)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	p := stageRuntimeReport(subject.Subject, claim.Identity)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	_, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
	f.svc.TxStarter = f.pool
	if err == nil || fault.begins.Load() != 1 {
		t.Fatalf("unknown commit retried or appeared successful: begins=%d err=%v", fault.begins.Load(), err)
	}
	ack, err := f.svc.RuntimeTestReportReceipt(context.Background(), run, permit.BusinessToken, p.RequestID)
	if err != nil || !ack.Accepted || ack.RequestID != p.RequestID {
		t.Fatalf("original receipt: %+v %v", ack, err)
	}
	replay, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
	if err != nil || replay != ack || f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, run) != 1 {
		t.Fatalf("original intent duplicated: %+v %v", replay, err)
	}
}

func TestStageTestSubjectBlockedTargetHasNoBinding(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	p := f.input(t)
	p.TargetVersions[0].ExpectedTargetEpoch++
	out, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || len(out.Targets) != 1 || out.Targets[0].State != "blocked" || out.Targets[0].CommandID.Valid {
		t.Fatalf("blocked target: %+v %v", out, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_test_subject WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("blocked target acquired test subject")
	}
}

func TestStageTestReportHostCheckpointExactLogAndRedaction(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
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
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, launch)
	if err != nil {
		t.Fatal(err)
	}
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	report := stageRuntimeReport(subject.Subject, claim.Identity)
	report.Report.Logs = []protocol.CollaborationTestLog{{Kind: "record", EventID: uuid.NewString(), MessageIndex: 0, BodySHA256: strings.Repeat("a", 64)}}
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report); !collaborationErrorCode(err, "test_report_log_not_retained") {
		t.Fatalf("unretained log accepted: %v", err)
	}
	checkpoint := checkpointFor(claim, permit)
	checkpoint.Messages[0].Body = "secret mcb_" + strings.Repeat("a", 64)
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), run, permit.BusinessToken, checkpoint); err != nil {
		t.Fatal(err)
	}
	e, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: checkpointKey(checkpoint)})
	if err != nil {
		t.Fatal(err)
	}
	report.Report.Logs[0].EventID = util.UUIDToString(e.ID)
	report.Report.Logs[0].BodySHA256 = protocol.CollaborationKnowledgeHash(checkpoint.Messages[0].Body)
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report); !collaborationErrorCode(err, "test_report_log_not_retained") {
		t.Fatalf("raw unredacted hash accepted: %v", err)
	}
	body := redact.Text(checkpoint.Messages[0].Body)
	report.Report.Logs[0].BodySHA256 = protocol.CollaborationKnowledgeHash(body)
	ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := f.svc.TestReport(f.ctx, f.issue.Issue.ID, run, util.MustParseUUID(ack.ReportID))
	if err != nil || detail.Logs[0].Body != body || detail.Logs[0].EventID != util.UUIDToString(e.ID) || detail.Applicability != "execution_unsettled" {
		t.Fatalf("exact redacted log: %+v %v", detail, err)
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), "mcb_"+strings.Repeat("a", 64)) {
		t.Fatal("raw credential leaked")
	}
	old, err := f.svc.RunRecords(WithCollaborationExecutionViewVersion(f.ctx, 6), f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range old.Items {
		if item.Kind == "test_report" || item.Kind == "test_subject" {
			t.Fatal("typed report changed old records union")
		}
	}
	replay, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil || !reflect.DeepEqual(ack, replay) {
		t.Fatalf("record original-key retry: %+v %v", replay, err)
	}
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), run, claim.TerminationToken, hostLaunchEnded(launch, true)); err != nil {
		t.Fatal(err)
	}
	secondLaunch := hostPermitRequest(claim, 2, "provider")
	secondPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, secondLaunch)
	if err != nil {
		t.Fatal(err)
	}
	second := report
	second.RequestID = uuid.NewString()
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, secondPermit.BusinessToken, second); !collaborationErrorCode(err, "test_report_log_not_retained") {
		t.Fatalf("cross-attempt checkpoint accepted: %v", err)
	}
	second.Report.Logs = []protocol.CollaborationTestLog{{Kind: "inline", Body: "second attempt output"}}
	secondACK, err := f.svc.RecordRuntimeTestReport(context.Background(), run, secondPermit.BusinessToken, second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.TestReports(f.ctx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].ReportID != secondACK.ReportID || page.NextCursor == nil {
		t.Fatalf("bounded report page: %+v %v", page, err)
	}
	if _, err = f.svc.TestReports(f.stage.qaCtx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "invalid_test_report_query") {
		t.Fatalf("foreign viewer cursor accepted: %v", err)
	}
	rawCursor, _ := base64.RawURLEncoding.DecodeString(*page.NextCursor)
	var cursor collaborationTestReportCursor
	_ = json.Unmarshal(rawCursor, &cursor)
	cursor.Through = "999999"
	rawCursor, _ = json.Marshal(cursor)
	if _, err = f.svc.TestReports(f.ctx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(rawCursor)}); !collaborationErrorCode(err, "invalid_test_report_query") {
		t.Fatalf("tampered watermark accepted: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), run, claim.TerminationToken, hostLaunchEnded(secondLaunch, true)); err != nil {
		t.Fatal(err)
	}
	thirdPermit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, hostPermitRequest(claim, 3, "provider"))
	if err != nil {
		t.Fatal(err)
	}
	third := second
	third.RequestID = uuid.NewString()
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, thirdPermit.BusinessToken, third); err != nil {
		t.Fatal(err)
	}
	older, err := f.svc.TestReports(f.ctx, f.issue.Issue.ID, run, CollaborationTestReportsQuery{Limit: 1, Cursor: *page.NextCursor})
	if err != nil || len(older.Items) != 1 || older.Items[0].ReportID != ack.ReportID || older.NextCursor != nil {
		t.Fatalf("new report entered an old watermark: %+v %v", older, err)
	}
	if replay, err = f.svc.RuntimeTestReportReceipt(context.Background(), run, permit.BusinessToken, report.RequestID); err != nil || replay != ack {
		t.Fatalf("old attempt receipt after retry: %+v %v", replay, err)
	}
}
