package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type stageRuntimeFixture struct {
	*managedCollaborationFixture
	stage     *stageReviewFixture
	candidate CollaborationStageCandidateRef
}

func newStageRuntimeFixture(t *testing.T, git bool, setup ...func(*testing.T, *stageReviewFixture)) *stageRuntimeFixture {
	t.Helper()
	m := newManagedCollaborationFixture(t)
	principal := func(user string) context.Context {
		return auth.WithAuthenticatedPrincipal(collaborationPasswordContext(util.MustParseUUID(user), 1), stageCredential(user).Principal)
	}
	m.ctx = principal(m.creator)
	f := &collaborationStageFixture{collaborationQueueFixture: m.collaborationQueueFixture, project: m.fx.Project(t, "Synthetic test source")}
	m.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, m.issue.Issue.ID, f.project)
	for _, table := range []string{"collaboration_test_subject", "collaboration_stage_child", "collaboration_stage_revocation", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		m.fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, m.workspace)
	}
	for i, dest := range []*string{&f.reviewer, &f.qa, &f.release} {
		*dest = m.fx.User(t, "Synthetic approver", uuid.NewString()+"@example.test")
		m.fx.Member(t, m.workspace, *dest, "member")
		m.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": *dest, "username": "report" + uuid.NewString()[:8], "password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", *dest)
		m.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, *dest)
		if _, err := m.svc.InviteParticipant(m.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, m.ctx), TargetUserID: util.MustParseUUID(*dest)}); err != nil {
			t.Fatal(i, err)
		}
	}
	f.reviewerCtx = principal(f.reviewer)
	f.qaCtx = principal(f.qa)
	f.releaseCtx = principal(f.release)
	f.handlerCtx = principal(f.handler)
	if _, err := f.svc.ReviseStagePolicy(f.ctx, StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(f.workspace), ProjectID: util.MustParseUUID(f.project), RequestID: uuid.NewString(), Enabled: true, Reason: "Synthetic test policy", Credential: completionRequestCredential(f.ctx)}); err != nil {
		t.Fatal(err)
	}
	f.policyRevision = 1
	f.configure(t)
	child := m.fx.Issue(t, "Synthetic tested child", testutil.Cols{"parent_issue_id": m.issue.Issue.ID, "project_id": f.project})
	f.confirmScope(t, CollaborationStageScopeChild{IssueID: util.MustParseUUID(child), ContentDigest: stageReadableDigest(t, stageChildView(t, f.view(t), child))})
	head := strings.Repeat("b", 40)
	pr := f.githubPR(t, child, head, false)
	m.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": m.workspace, "installation_id": int64(1), "account_login": "synthetic-report", "account_type": "User"})
	m.fx.Exec(t, `UPDATE github_pull_request SET snapshot_head_sha=head_sha,snapshot_fetched_at=clock_timestamp()-interval '1 minute',snapshot_complete=true WHERE id=$1`, pr)
	m.fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{"pr_id": pr, "head_sha": head, "ordinal": int32(0), "name": "synthetic tests", "status": "completed", "conclusion": "success"}, "pr_id=$1", pr)
	s := &stageReviewFixture{collaborationStageFixture: f, child: child, pr: pr, head: head}
	for _, prepare := range setup {
		prepare(t, s)
	}
	c, _, approval := s.approved(t)
	if _, err := s.svc.AdvanceStage(s.ctx, s.advanceParams(t, c, "qa", approval.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	p := preparationRegistrationRequest(m)
	if git {
		p.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: head, BundleDigest: strings.Repeat("d", 64)}
	}
	if _, err := m.svc.RegisterRuntimeCapabilities(m.daemonCtx, p); err != nil {
		t.Fatal(err)
	}
	m.runtime, _ = m.svc.Queries.GetAgentRuntime(m.ctx, m.runtime.ID)
	refreshPreparationConsent(t, m)
	return &stageRuntimeFixture{managedCollaborationFixture: m, stage: s, candidate: c}
}

func (f *stageRuntimeFixture) input(t *testing.T) CollaborationCreateCommentParams {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	p := CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: "Run synthetic tests [@Agent](mention://agent/" + util.UUIDToString(f.grant.AgentID) + ")"},
		TargetVersions: []CollaborationCommentTargetVersion{{TargetID: f.targetID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion}},
		TestSubject:    &protocol.CollaborationTestSubjectRef{CandidateID: util.UUIDToString(f.candidate.CandidateID), CandidateVersion: stageVersion(f.candidate.Version), CandidateDigest: f.candidate.ContentDigest}}
	p.ExecutionViewVersion = 6
	return p
}

func TestStageTestReportActualInputClaimAndReceipt(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	p := f.input(t)
	comment, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || len(comment.Targets) != 1 || !comment.Targets[0].CommandID.Valid {
		t.Fatalf("typed input: %+v %v", comment, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_test_subject WHERE command_id=$1`, comment.Targets[0].CommandID) != 1 {
		t.Fatal("actual command not bound")
	}
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("runs: %+v %v", list, err)
	}
	run := util.MustParseUUID(list.RunIDs[0])
	_, claim := claimPreparedCandidate(t, f.managedCollaborationFixture, run)
	permitRequest, permit := permitPreparedClaim(t, f.managedCollaborationFixture, run, claim)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil || subject.Subject.CandidateID != p.TestSubject.CandidateID || subject.Subject.SourceRevision != f.stage.head {
		t.Fatalf("frozen subject: %+v %v", subject, err)
	}
	report := protocol.CollaborationTestReportRequest{Version: 1, Identity: claim.Identity, RequestID: uuid.NewString(), Report: protocol.CollaborationTestReport{Subject: subject.Subject, SuiteID: "synthetic regression", Cases: []protocol.CollaborationTestCase{{ID: "callback-delay", Outcome: "failed", LogIndex: 0}}, Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: "expected paid, observed pending"}}}}
	ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil || !ack.Accepted {
		t.Fatalf("report: %+v %v", ack, err)
	}
	before, _ := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, run)
	if before.BusinessOutcome.Valid {
		t.Fatal("report ended business turn")
	}
	changed := report
	changed.RequestID = uuid.NewString()
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, changed); !collaborationErrorCode(err, "test_report_already_recorded") {
		t.Fatalf("second report: %v", err)
	}
	changed = report
	changed.Report.SuiteID = "changed intent"
	if _, err = f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, changed); !collaborationErrorCode(err, "test_report_conflict") {
		t.Fatalf("same key changed body: %v", err)
	}
	finishSquadRun(t, f.managedCollaborationFixture, run, claim, permit.BusinessToken, permitRequest.AttemptNonce, "success")
	replay, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil || !reflect.DeepEqual(ack, replay) {
		t.Fatalf("settled exact recovery: %+v %v", replay, err)
	}
	receipt, err := f.svc.RuntimeTestReportReceipt(context.Background(), run, permit.BusinessToken, report.RequestID)
	if err != nil || !reflect.DeepEqual(ack, receipt) {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	raw, _ := json.Marshal(receipt)
	if strings.Contains(string(raw), "callback") || strings.Contains(string(raw), "subject") {
		t.Fatal("receipt leaked report content")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, run) != 1 {
		t.Fatal("report history was overwritten or duplicated")
	}
	if f.stage.view(t).Stage.Current != "qa" {
		t.Fatal("report silently advanced stage")
	}
	qa := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: f.stage.policyRevision, ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "return", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{}, Reason: "Synthetic QA returns the failed callback case"}
	qa.Evidence.CIDigest = f.stage.evidence(t, f.stage.reviewerCtx, f.candidate).Observed.CI.CIDigest
	f.fx.Exec(t, `UPDATE github_pull_request_check_run SET conclusion='failure' WHERE pr_id=$1`, f.stage.pr)
	decision, err := f.svc.DecideStage(f.stage.qaCtx, qa)
	if err != nil || decision.StageConfirmation == nil {
		t.Fatalf("QA return of exact failed report: %+v %v", decision, err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "development", decision.StageConfirmation.DecisionID)); err != nil {
		t.Fatalf("QA rework: %v", err)
	}
	detail, err := f.svc.TestReport(f.ctx, f.issue.Issue.ID, run, util.MustParseUUID(ack.ReportID))
	if err != nil || detail.ReportedResult != "failed" || detail.FailedCaseCount != 1 || detail.Cases[0].ID != "callback-delay" || detail.Logs[0].Body != "expected paid, observed pending" {
		t.Fatalf("retained failure detail: %+v %v", detail, err)
	}
}

func TestStageTestSubjectRequiresTypedTarget(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	p := f.input(t)
	p.Content = "Plain prose cannot choose a test target"
	if _, err := f.svc.CreateComment(f.ctx, p); !collaborationErrorCode(err, "test_subject_target_required") {
		t.Fatalf("unbound comment accepted: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("refusal created a comment")
	}
}

func TestStageQAPassedReportNeedsSuccessfulSettledExecution(t *testing.T) {
	for _, kind := range []string{"success", "failed_execution", "unsettled", "failed_case"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageRuntimeFixture(t, true)
			if _, err := f.svc.CreateComment(f.ctx, f.input(t)); err != nil {
				t.Fatal(err)
			}
			f.resumeManaged(t)
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 1 {
				t.Fatalf("run: %+v %v", list, err)
			}
			run := util.MustParseUUID(list.RunIDs[0])
			_, claim := claimPreparedCandidate(t, f.managedCollaborationFixture, run)
			permitRequest, permit := permitPreparedClaim(t, f.managedCollaborationFixture, run, claim)
			subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
			if err != nil {
				t.Fatal(err)
			}
			outcome := "passed"
			if kind == "failed_case" {
				outcome = "failed"
			}
			p := protocol.CollaborationTestReportRequest{Version: 1, Identity: claim.Identity, RequestID: uuid.NewString(), Report: protocol.CollaborationTestReport{Subject: subject.Subject, SuiteID: "synthetic QA", Cases: []protocol.CollaborationTestCase{{ID: "callback", Outcome: outcome, LogIndex: 0}}, Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: "Synthetic case output"}}}}
			ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, p)
			if err != nil {
				t.Fatal(err)
			}
			if kind != "unsettled" {
				result := "success"
				if kind == "failed_execution" {
					result = "failure"
				}
				finishSquadRun(t, f.managedCollaborationFixture, run, claim, permit.BusinessToken, permitRequest.AttemptNonce, result)
			}
			code, err := f.svc.Queries.GetCollaborationStageReviewHead(f.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "code_review"})
			if err != nil {
				t.Fatal(err)
			}
			view := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
			if len(view.TestReports) != 1 || view.TestReports[0].ReportID != ack.ReportID {
				t.Fatalf("automatic report association: %+v", view.TestReports)
			}
			expires := view.CheckedAt.Add(time.Minute)
			qa := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "approve", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{util.UUIDToString(code.DecisionID)}, Reason: "Synthetic human QA", ExpiresAt: &expires}
			decision, err := f.svc.DecideStage(f.stage.qaCtx, qa)
			if kind != "success" {
				if !collaborationErrorCode(err, "stage_test_report_unusable") || view.Signable[1].Allowed {
					t.Fatalf("%s passed QA: %+v %v", kind, decision, err)
				}
				return
			}
			if err != nil || decision.StageConfirmation == nil || !view.Signable[1].Allowed {
				t.Fatalf("QA approve: %+v %v", decision, err)
			}
			if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "release", decision.StageConfirmation.DecisionID)); err != nil {
				t.Fatalf("QA to release: %v", err)
			}
			if f.stage.view(t).Stage.Current != "release" {
				t.Fatal("valid QA did not advance")
			}
			if _, err = f.svc.RevokeStageDecision(f.stage.reviewerCtx, CollaborationStageRevocationParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.reviewerCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, DecisionID: code.DecisionID, Reason: "Synthetic original approval withdrawal"}); err != nil {
				t.Fatal(err)
			}
			current := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
			for _, d := range current.Decisions {
				if d.Step == "qa" && d.State == "valid" {
					t.Fatal("QA approval survived withdrawal of its original dependency")
				}
			}
		})
	}
}

func TestStageTestSubjectMemberFenceAtActualCommentWrite(t *testing.T) {
	f := newStageRuntimeFixture(t, true)
	hits := 0
	var removal error
	f.svc.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: "CreateCollaborationComment", before: func() { hits++; removal = stageTryDeleteMember(t, f.collaborationFixture, f.creator) }}
	_, err := f.svc.CreateComment(f.ctx, f.input(t))
	f.svc.TxStarter = f.pool
	var locked *pgconn.PgError
	if hits != 1 || !errors.As(removal, &locked) || locked.Code != "55P03" || err != nil {
		t.Fatalf("test input source fence: hook=%d deletion=%v comment=%v", hits, removal, err)
	}
}
