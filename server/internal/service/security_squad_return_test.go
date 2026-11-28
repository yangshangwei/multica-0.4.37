package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type securityReturnFixture struct {
	*delegatedFailureFixture
	fx         *testutil.Fixture
	squad      string
	workerTask pgtype.UUID
	gate       string
}

func seedSecurityReturnFixture(t *testing.T) (*securityReturnFixture, *TaskService) {
	t.Helper()
	f, svc := seedDelegatedFailureFixture(t)
	fx := testutil.New(f.pool, f.workspaceID, f.userID)
	fx.Exec(t, `UPDATE agent SET template_key='security-remediation-lead',template_version=3 WHERE id=$1`, f.coordinator)
	fx.Exec(t, `UPDATE agent SET template_key='security-reviewer',template_version=2 WHERE id=$1`, f.worker)
	squad := fx.Insert(t, "squad", testutil.Cols{"workspace_id": f.workspaceID, "name": "security-return", "leader_id": f.coordinator, "creator_id": f.userID, "template_key": "security-remediation", "template_version": 3})
	fx.Insert(t, "squad_member", testutil.Cols{"squad_id": squad, "member_type": "agent", "member_id": f.worker, "role": "reviewer"})
	fx.Exec(t, `UPDATE issue SET assignee_type='squad',assignee_id=$1,creator_type='agent',creator_id=$2 WHERE id=$3`, squad, f.coordinator, f.issueID)
	fx.Exec(t, `UPDATE agent_task_queue SET is_leader_task=true,squad_id=$1 WHERE id=$2`, squad, f.sourceTask)
	gate := fx.Insert(t, "comment", testutil.Cols{"workspace_id": f.workspaceID, "issue_id": f.issueID, "author_type": "agent", "author_id": f.coordinator, "source_task_id": f.sourceTask, "content": "review the security fix"})
	workerTask := f.insertWorkerTask(t, "running", "comment", 1, 2)
	fx.Exec(t, `UPDATE agent_task_queue SET issue_id=$1,trigger_comment_id=$2,started_at=now() WHERE id=$3`, f.issueID, gate, workerTask)
	return &securityReturnFixture{f, fx, squad, workerTask, gate}, svc
}

func (f *securityReturnFixture) result(t *testing.T, typ string) string {
	t.Helper()
	return f.fx.Insert(t, "comment", testutil.Cols{"workspace_id": f.workspaceID, "issue_id": f.issueID, "author_type": "agent", "author_id": f.worker, "source_task_id": f.workerTask, "parent_id": f.gate, "content": "review evidence; [@unrelated](mention://agent/00000000-0000-4000-8000-000000000001)", "type": typ, "dispatch_eligible": false})
}

func (f *securityReturnFixture) complete(t *testing.T, svc *TaskService) {
	t.Helper()
	if _, err := svc.CompleteTask(context.Background(), f.workerTask, []byte(`{"output":""}`), "", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityReviewerCompletionReturnsToOriginalLeaderOnce(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	var pluginComments int
	SubscribePluginEvents(svc.Bus, recordingSink(func(kind string) {
		if kind == "comment.created" {
			pluginComments++
		}
	}))
	result := f.result(t, "comment")
	f.complete(t, svc)
	f.complete(t, svc)
	if _, err := svc.RecoverPendingDelegatedFailures(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id NOT IN ($2,$3)`, f.issueID, f.sourceTask, f.workerTask); n != 1 {
		t.Fatalf("return task count=%d, want 1", n)
	}
	var agent, squad, status, origin string
	f.fx.QueryRow(t, `SELECT agent_id::text,squad_id::text,status,originator_user_id::text FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask).Scan(&agent, &squad, &status, &origin)
	if agent != f.coordinator || squad != f.squad || status != "queued" || origin != f.userID {
		t.Fatalf("wrong return identity %s/%s/%s/%s", agent, squad, status, origin)
	}
	var eligible bool
	f.fx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE id=$1`, result).Scan(&eligible)
	if eligible {
		t.Fatal("reviewer result became dispatch eligible")
	}
	f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.issueID).Scan(&status)
	if status != "in_progress" {
		t.Fatalf("return accepted issue: %s", status)
	}
	if pluginComments != 0 {
		t.Fatalf("platform return fanned out %d comment plugin events", pluginComments)
	}
}

func TestSecurityReviewerCompletionWithoutResultReturnsMissingEvidence(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.result(t, "progress_update")
	f.complete(t, svc)
	var content string
	f.fx.QueryRow(t, `SELECT content FROM comment WHERE author_type='system' AND type='progress_update' AND source_task_id=$1`, f.workerTask).Scan(&content)
	if !strings.Contains(content, "no review result") {
		t.Fatalf("missing evidence not distinguished: %s", content)
	}
}

func TestSecurityReviewerReturnRejectsStaleOrUnrelatedAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		args      func(*securityReturnFixture) []any
	}{
		{"ordinary squad", `UPDATE squad SET template_key='bug-fix' WHERE id=$1`, func(f *securityReturnFixture) []any { return []any{f.squad} }},
		{"reassigned issue", `UPDATE issue SET assignee_type='agent',assignee_id=$1 WHERE id=$2`, func(f *securityReturnFixture) []any { return []any{f.worker, f.issueID} }},
		{"removed reviewer", `DELETE FROM squad_member WHERE squad_id=$1`, func(f *securityReturnFixture) []any { return []any{f.squad} }},
		{"rotated leader", `UPDATE squad SET leader_id=$1 WHERE id=$2`, func(f *securityReturnFixture) []any { return []any{f.worker, f.squad} }},
		{"unrelated comment", `UPDATE comment SET source_task_id=NULL WHERE id=$1`, func(f *securityReturnFixture) []any { return []any{f.gate} }},
		{"revoked member", `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, func(f *securityReturnFixture) []any { return []any{f.workspaceID, f.userID} }},
		{"missing originator", `UPDATE agent_task_queue SET originator_user_id=NULL WHERE id=$1`, func(f *securityReturnFixture) []any { return []any{f.workerTask} }},
		{"runtime changed", `UPDATE agent SET runtime_id=NULL WHERE id=$1`, func(f *securityReturnFixture) []any { return []any{f.worker} }},
		{"invoke revoked", `UPDATE agent SET owner_id=NULL,permission_mode='private' WHERE id=$1`, func(f *securityReturnFixture) []any { return []any{f.coordinator} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, svc := seedSecurityReturnFixture(t)
			f.result(t, "comment")
			f.fx.Exec(t, tc.sql, tc.args(f)...)
			f.complete(t, svc)
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND id<>$3`, f.issueID, f.coordinator, f.sourceTask); n != 0 {
				t.Fatalf("unauthorized return created %d tasks", n)
			}
		})
	}
}

func TestSecurityReviewerFinalFailureAndLegacyInstancesReturnWithoutAcceptance(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.fx.Exec(t, `UPDATE squad SET template_version=2 WHERE id=$1`, f.squad)
	f.fx.Exec(t, `UPDATE agent SET template_version=2 WHERE id=$1`, f.coordinator)
	if _, err := svc.FailTask(context.Background(), f.workerTask, "review failed", "", "", "", "agent_error.process_failure", false, "", ""); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='delegated_failure' AND trigger_evidence_ref_id=$1`, util.UUIDToString(f.workerTask)); n != 1 {
		t.Fatalf("failure return count=%d", n)
	}
	var status string
	f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.issueID).Scan(&status)
	if status != "in_progress" {
		t.Fatalf("failure accepted issue: %s", status)
	}
}

func TestSecurityReviewerReturnRevocationBeforeStartIsRefused(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.result(t, "comment")
	f.complete(t, svc)
	var returned string
	f.fx.QueryRow(t, `SELECT id::text FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask).Scan(&returned)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, returned)
	f.fx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, f.squad, f.worker)
	if _, err := svc.StartTask(context.Background(), util.MustParseUUID(returned)); err == nil {
		t.Fatal("queued return survived reviewer removal at execution start")
	}
}

func TestSecurityReviewerReturnDoesNotMergeAcrossCoordinatorIdentity(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.result(t, "comment")
	blocker := f.fx.Task(t, f.coordinator, testutil.Cols{"runtime_id": f.runtimeID, "issue_id": f.issueID, "status": "queued", "originator_user_id": f.userID, "accountable_user_id": f.userID})
	if _, err := svc.CompleteTask(context.Background(), f.workerTask, []byte(`{"output":""}`), "", "", "", false, "", ""); err == nil {
		t.Fatal("unrelated ordinary coordinator task was treated as a squad return")
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND trigger_comment_id IS NULL AND cardinality(coalesced_comment_ids)=0`, blocker); n != 1 {
		t.Fatal("signal changed unrelated pending task")
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, blocker)
	result, err := svc.RecoverPendingDelegatedFailures(context.Background(), 50)
	if err != nil || result.Replayed != 1 {
		t.Fatalf("outbox return after blocker: %+v %v", result, err)
	}
}

func TestSecurityReviewerReturnAttemptsRemainBounded(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.result(t, "comment")
	f.complete(t, svc)
	for range delegatedFailureRecoveryMaxTaskAttempts {
		f.fx.Exec(t, `UPDATE agent_task_queue SET status='cancelled',completed_at=now() WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask)
		if _, err := svc.RecoverPendingDelegatedFailures(context.Background(), 50); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask); n != delegatedFailureRecoveryMaxTaskAttempts {
		t.Fatalf("return attempts=%d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM comment WHERE author_type='system' AND type='system' AND source_task_id=$1`, f.workerTask); n != 1 {
		t.Fatalf("exhaustion notices=%d", n)
	}
}

func TestSecurityReviewerRevokedReturnCannotStarveLaterReturn(t *testing.T) {
	older, svc := seedSecurityReturnFixture(t)
	older.result(t, "comment")
	older.complete(t, svc)
	newer, otherSvc := seedSecurityReturnFixture(t)
	newer.result(t, "comment")
	newer.complete(t, otherSvc)
	for _, f := range []*securityReturnFixture{older, newer} {
		f.fx.Exec(t, `UPDATE agent_task_queue SET status='cancelled',completed_at=now() WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask)
	}
	older.fx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, older.squad, older.worker)
	first, err := svc.RecoverPendingDelegatedFailures(context.Background(), 1)
	if err != nil || first.Scanned != 1 || first.Replayed != 0 {
		t.Fatalf("first sweep: %+v %v", first, err)
	}
	var eligible bool
	older.fx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE author_type='system' AND type='progress_update' AND source_task_id=$1`, older.workerTask).Scan(&eligible)
	if eligible {
		t.Fatal("revoked return was not settled")
	}
	second, err := svc.RecoverPendingDelegatedFailures(context.Background(), 1)
	if err != nil || second.Scanned != 1 || second.Replayed != 1 {
		t.Fatalf("later return starved: %+v %v", second, err)
	}
	older.fx.Insert(t, "squad_member", testutil.Cols{"squad_id": older.squad, "member_type": "agent", "member_id": older.worker, "role": "reviewer"})
	older.complete(t, svc)
	if n := older.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, older.workerTask); n != 1 {
		t.Fatalf("restored permission revived withdrawn return: %d", n)
	}
}

func TestSecurityReturnGuardPreservesDeletedOrdinaryInputs(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	taskID := f.fx.Task(t, f.coordinator, testutil.Cols{"runtime_id": f.runtimeID, "issue_id": f.issueID, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "trigger_comment_id": f.gate, "coalesced_comment_ids": testutil.Raw("ARRAY[gen_random_uuid()]"), "originator_user_id": f.userID, "accountable_user_id": f.userID})
	if err := finalizeReturnTestTask(t, svc, f, taskID); err != nil {
		t.Fatalf("ordinary claim lost surviving trigger: %v", err)
	}
}

func finalizeReturnTestTask(t *testing.T, svc *TaskService, f *securityReturnFixture, taskID string) error {
	t.Helper()
	task, err := svc.Queries.GetAgentTask(context.Background(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, taskID)
	_, err = svc.FinalizeTaskClaim(context.Background(), task, db.CreateTaskTokenParams{TokenHash: fmt.Sprintf("security-return-%d", time.Now().UnixNano()), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: util.MustParseUUID(f.workspaceID), UserID: util.MustParseUUID(f.userID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, nil, TaskClaimBudgetProtocolNone)
	return err
}

func TestSecurityReviewerReturnClaimRechecksCoalescedAuthority(t *testing.T) {
	for _, coalesced := range []bool{false, true} {
		t.Run(fmt.Sprint(coalesced), func(t *testing.T) {
			f, svc := seedSecurityReturnFixture(t)
			f.result(t, "comment")
			f.complete(t, svc)
			var returned string
			f.fx.QueryRow(t, `SELECT id::text FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask).Scan(&returned)
			if coalesced {
				f.fx.Exec(t, `UPDATE agent_task_queue SET coalesced_comment_ids=ARRAY[trigger_comment_id],trigger_comment_id=$1,trigger_evidence_kind='comment' WHERE id=$2`, f.gate, returned)
			}
			f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, returned)
			f.fx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, f.squad, f.worker)
			if err := finalizeReturnTestTask(t, svc, f, returned); err == nil {
				t.Fatal("claim minted credentials after reviewer removal")
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, returned); n != 0 {
				t.Fatalf("claim minted %d tokens", n)
			}
		})
	}
}

func TestSecurityReviewerConcurrentCompletionsCoalesce(t *testing.T) {
	f, svc := seedSecurityReturnFixture(t)
	f.result(t, "comment")
	other := f.fx.Agent(t, "second security reviewer", f.runtimeID, testutil.Cols{"template_key": "code-reviewer", "template_version": 2, "runtime_id": f.runtimeID, "owner_id": f.userID})
	f.fx.Insert(t, "squad_member", testutil.Cols{"squad_id": f.squad, "member_type": "agent", "member_id": other, "role": "code reviewer"})
	second := f.fx.Task(t, other, testutil.Cols{"runtime_id": f.runtimeID, "issue_id": f.issueID, "status": "running", "started_at": testutil.Raw("now()"), "trigger_comment_id": f.gate, "delegated_from_task_id": f.sourceTask, "originator_user_id": f.userID, "accountable_user_id": f.userID, "originator_source": "delegation"})
	f.fx.Insert(t, "comment", testutil.Cols{"workspace_id": f.workspaceID, "issue_id": f.issueID, "author_type": "agent", "author_id": other, "source_task_id": second, "parent_id": f.gate, "content": "independent review result", "dispatch_eligible": false})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []pgtype.UUID{f.workerTask, util.MustParseUUID(second)} {
		wg.Add(1)
		go func(id pgtype.UUID) {
			defer wg.Done()
			_, err := svc.CompleteTask(context.Background(), id, []byte(`{"output":""}`), "", "", "", false, "", "")
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status='queued'`, f.issueID, f.coordinator); n != 1 {
		t.Fatalf("concurrent returns queued %d leaders", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM comment c JOIN agent_task_queue t ON t.issue_id=c.issue_id AND (c.id=t.trigger_comment_id OR c.id=ANY(t.coalesced_comment_ids)) WHERE t.status='queued' AND t.agent_id=$1 AND c.author_type='system' AND c.type='progress_update'`, f.coordinator); n != 2 {
		t.Fatalf("coalesced return contains %d signals", n)
	}
}

func TestSecurityReviewerReturnRequiresItsPlatformSignal(t *testing.T) {
	for _, mode := range []string{"missing row", "cleared trigger"} {
		t.Run(mode, func(t *testing.T) {
			f, svc := seedSecurityReturnFixture(t)
			f.result(t, "comment")
			f.complete(t, svc)
			var returned, signal string
			f.fx.QueryRow(t, `SELECT id::text,trigger_comment_id::text FROM agent_task_queue WHERE trigger_evidence_kind='delegated_result' AND trigger_evidence_ref_id=$1`, f.workerTask).Scan(&returned, &signal)
			f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1`, returned)
			if mode == "missing row" {
				f.fx.Exec(t, `DELETE FROM comment WHERE id=$1`, signal)
			} else {
				f.fx.Exec(t, `UPDATE agent_task_queue SET trigger_comment_id=NULL WHERE id=$1`, returned)
			}
			if _, err := svc.StartTask(context.Background(), util.MustParseUUID(returned)); err == nil {
				t.Fatal("return started without its platform signal")
			}
		})
	}
}
