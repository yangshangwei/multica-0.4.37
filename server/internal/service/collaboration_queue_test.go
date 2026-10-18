package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationQueueFixture struct {
	*collaborationFixture
	issue    CollaborationIssueAccess
	targetID pgtype.UUID
}

func newCollaborationQueueFixture(t *testing.T) *collaborationQueueFixture {
	t.Helper()
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_command WHERE session_id=$1`, issue.SessionID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue.Issue.ID)
	target, _ := seedCollaborationTarget(t, f.fx, issue.Issue.ID, issue.SessionID, util.MustParseUUID(f.creator), 1)
	return &collaborationQueueFixture{collaborationFixture: f, issue: issue, targetID: target.ID}
}

func (f *collaborationQueueFixture) request() CollaborationQueueRequest {
	return CollaborationQueueRequest{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}
}

func (f *collaborationQueueFixture) submit(t *testing.T, actor, content string) CollaborationQueueReceipt {
	t.Helper()
	receipt, err := f.svc.SubmitCommand(collaborationContext(actor), CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

// Runs are fixtures only: this slice has no admission, promotion or launch API.
func (f *collaborationQueueFixture) run(t *testing.T, command CollaborationQueueReceipt, attempts int64) db.CollaborationRun {
	t.Helper()
	return seedCollaborationGroupRun(t, f.fx, f.issue.SessionID, command, attempts)
}

func (f *collaborationQueueFixture) stopParams(run db.CollaborationRun, version int64) CollaborationStopQueueParams {
	return CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget,
		Targets: []CollaborationTargetControl{{TargetID: run.TargetID, ExpectedTargetEpoch: run.TargetEpoch.Int64,
			ExpectedConfigVersion: run.ConfigVersion.Int64, ExpectedControlVersion: version,
			ObservedGroupID: run.GroupID, ObservedGroupGeneration: f.groupGeneration(run.TargetID)}}}
}

func (f *collaborationQueueFixture) groupGeneration(target pgtype.UUID) int64 {
	var generation int64
	if err := f.pool.QueryRow(f.ctx, `SELECT group_generation FROM collaboration_target WHERE id=$1`, target).Scan(&generation); err != nil {
		panic(err)
	}
	return generation
}

func (f *collaborationQueueFixture) issueControl(t *testing.T) CollaborationStopQueueParams {
	t.Helper()
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := f.svc.Queries.ListCollaborationTargets(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	p := CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlIssue,
		ExpectedControlVersion: session.ControlVersion, ExpectedTargetSetVersion: session.TargetSetVersion}
	for _, target := range targets {
		p.Targets = append(p.Targets, targetControl(target))
	}
	return p
}

func targetControl(target db.CollaborationTarget) CollaborationTargetControl {
	return CollaborationTargetControl{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		ExpectedControlVersion: target.ControlVersion, ObservedGroupID: target.ActiveGroupID, ObservedGroupGeneration: target.GroupGeneration}
}

func TestCollaborationQueueConcurrentSubmissionKeepsImmutableFIFO(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,creation_enabled=false WHERE id=1`)
	const count = 10
	start := make(chan struct{})
	type result struct {
		receipt CollaborationQueueReceipt
		err     error
	}
	results := make(chan result, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			actor := f.creator
			if i%2 == 1 {
				actor = f.handler
			}
			r, err := f.svc.SubmitCommand(collaborationContext(actor), CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: fmt.Sprintf("command %d", i)})
			results <- result{r, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var seqs []int64
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		seqs = append(seqs, result.receipt.CommandSequence)
		if result.receipt.QueueState != "paused" || result.receipt.ControlVersion != 1 {
			t.Fatalf("submission changed execution control: %+v", result.receipt)
		}
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for i, seq := range seqs {
		if seq != int64(i+1) {
			t.Fatalf("FIFO sequence gap/duplicate: %v", seqs)
		}
	}
	p := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "immutable original"}
	first, err := f.svc.SubmitCommand(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.SubmitCommand(f.ctx, p)
	if err != nil || !reflect.DeepEqual(replay, first) {
		t.Fatalf("request replay changed its receipt: %+v %+v %v", first, replay, err)
	}
	p.Content = "changed payload"
	if _, err = f.svc.SubmitCommand(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed submission key: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != count+1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("submission duplicated input or dispatched work while maintenance/paused")
	}
}

func TestCollaborationQueueCancelUsesCurrentRolesAndPreservesOrder(t *testing.T) {
	for _, role := range []string{"author", "creator", "handler", "other_participant", "source_member", "former_handler", "revoked"} {
		t.Run(role, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			author := f.outsider
			f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": author}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, author)
			f.fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": author}, "user_id=$1", author)
			first := f.submit(t, author, "keep original content")
			f.submit(t, f.creator, "second")
			actor := f.handler
			allowed := role == "author" || role == "creator" || role == "handler"
			switch role {
			case "author":
				actor = author
			case "creator":
				actor = f.creator
			case "other_participant", "former_handler":
				f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
			case "source_member":
				f.fx.Member(t, f.workspace, f.handler, "member")
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
			case "revoked":
				actor = author
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, author)
			}
			p := CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: first.CommandID, ExpectedCollaborationRevision: 1}
			r, err := f.svc.CancelWaitingCommand(collaborationContext(actor), p)
			if allowed && err != nil || !allowed && err == nil {
				t.Fatalf("role %s cancellation: %+v %v", role, r, err)
			}
			if allowed {
				replayed, err := f.svc.CancelWaitingCommand(collaborationContext(actor), p)
				if err != nil || !reflect.DeepEqual(replayed, r) {
					t.Fatalf("cancel replay: %+v %v", replayed, err)
				}
				p.RequestID = uuid.NewString()
				if _, err = f.svc.CancelWaitingCommand(collaborationContext(actor), p); !collaborationErrorCode(err, "command_not_waiting") {
					t.Fatalf("second cancel: %v", err)
				}
			}
			var content, authorID, state, queue string
			var seq, control int64
			f.fx.QueryRow(t, `SELECT content,author_user_id::text,state,seq FROM collaboration_command WHERE id=$1`, first.CommandID).Scan(&content, &authorID, &state, &seq)
			f.fx.QueryRow(t, `SELECT queue_state,control_version FROM collaboration_session WHERE id=$1`, f.issue.SessionID).Scan(&queue, &control)
			if content != "keep original content" || authorID != author || seq != 1 || queue != "paused" || control != 1 {
				t.Fatalf("cancel mutated immutable input or controls: %s %s %d %s %d", content, authorID, seq, queue, control)
			}
			if (state == "cancelled") != allowed {
				t.Fatalf("wrong command state: %s", state)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND seq=2 AND state='waiting'`, f.issue.SessionID) != 1 {
				t.Fatal("cancel reordered or removed the next command")
			}
		})
	}
}

func TestCollaborationQueueStopPreservesBarrierAfterAnyPermit(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		attempts         int64
		outcome, started bool
		settled          bool
		progress         string
	}{
		{"never_permitted", 0, false, false, true, ""},
		{"permitted_without_started", 1, false, false, false, ""},
		{"business_success_without_proof", 1, true, false, false, ""},
		{"inconsistent_started_without_permit", 0, false, true, false, ""},
		{"inconsistent_success_without_permit", 0, true, false, false, ""},
		{"inconsistent_running_without_permit", 0, false, false, false, "running"},
		{"unrecognized_progress_without_permit", 0, false, false, false, "future_state"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			command := f.submit(t, f.creator, "first")
			run := f.run(t, command, scenario.attempts)
			f.submit(t, f.handler, "second")
			if scenario.outcome {
				f.fx.Exec(t, `UPDATE collaboration_run SET business_outcome='success' WHERE id=$1`, run.ID)
			}
			if scenario.started {
				f.fx.Exec(t, `UPDATE collaboration_run SET started_at=now() WHERE id=$1`, run.ID)
			}
			if scenario.progress != "" {
				f.fx.Exec(t, `UPDATE collaboration_run SET progress_state=$2 WHERE id=$1`, run.ID, scenario.progress)
			}
			p := f.stopParams(run, 1)
			receipt, err := f.svc.StopQueue(f.ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.GroupSettled != scenario.settled || receipt.QueueState != "paused" {
				t.Fatalf("stop claimed wrong end state: %+v", receipt)
			}
			current, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: run.ID})
			if err != nil {
				t.Fatal(err)
			}
			if current.SettledAt.Valid != scenario.settled || current.Fence != run.Fence || current.LaunchNonce != run.LaunchNonce || current.ClaimGeneration <= run.ClaimGeneration || current.ForcedDisposition.String != "stopped" {
				t.Fatalf("stop lost barrier or changed historical proof identity: %+v", current)
			}
			if !scenario.settled && current.ProofState != "unknown" {
				t.Fatal("stop fabricated daemon proof")
			}
			if (f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND active_group_id=$2`, f.targetID, run.GroupID) == 1) != !scenario.settled {
				t.Fatal("stop cleared a run that still needs termination proof")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND seq=2 AND state='waiting'`, f.issue.SessionID) != 1 {
				t.Fatal("stop discarded waiting input")
			}
			replay, err := f.svc.StopQueue(f.ctx, p)
			if err != nil || !reflect.DeepEqual(replay, receipt) {
				t.Fatalf("stop replay: %+v %v", replay, err)
			}
		})
	}
}

func TestCollaborationQueueResumeRejectionNeverTurnsIntoDeferredIntent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	command := f.submit(t, f.creator, "first")
	run := f.run(t, command, 0)
	epoch := enableControlGate(t, f.collaborationFixture)
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.stopParams(run, 1), ExpectedExecutionEpoch: epoch}
	denied, err := f.svc.ResumeQueue(f.ctx, p)
	if !collaborationErrorCode(err, "stop_unconfirmed") {
		t.Fatalf("unsettled resume: %v", err)
	}
	if _, err = f.svc.StopQueue(f.ctx, f.stopParams(run, 1)); err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.ResumeQueue(f.ctx, p)
	if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(replay, denied) {
		t.Fatalf("failed resume key revived after legal settlement: %+v %v", replay, err)
	}
	p.CollaborationStopQueueParams = f.stopParams(run, 2)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true,epoch=epoch+1 WHERE id=1`)
	if _, err = f.svc.ResumeQueue(f.ctx, p); !collaborationErrorCode(err, "stale_admission") {
		t.Fatalf("old admission epoch revived after enable: %v", err)
	}
	p.RequestID = uuid.NewString()
	p.ExpectedExecutionEpoch = epoch + 1
	if _, err = f.svc.ResumeQueue(f.ctx, p); !collaborationErrorCode(err, "runtime_upgrade_required") {
		t.Fatalf("incomplete admission allowed resume: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND active_group_id IS NULL`, f.targetID) != 1 {
		t.Fatal("resume refusal wrote active state")
	}
}

func TestCollaborationQueueStaleStopCannotReachNextRun(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	first := f.run(t, f.submit(t, f.creator, "first"), 0)
	old := f.stopParams(first, 1)
	if _, err := f.svc.StopQueue(f.ctx, old); err != nil {
		t.Fatal(err)
	}
	next := f.run(t, f.submit(t, f.handler, "second"), 1)
	old.RequestID = uuid.NewString()
	old.Targets[0].ExpectedControlVersion = 2
	if _, err := f.svc.StopQueue(f.ctx, old); !collaborationErrorCode(err, "stale_execution") {
		t.Fatalf("old stop reached next run: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL`, next.ID) != 1 {
		t.Fatal("next run was changed by old fence")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=2`, f.targetID) != 1 {
		t.Fatal("stale stop changed current controls")
	}
}

func TestCollaborationQueueRejectsStaleIdentityAndMachine(t *testing.T) {
	for _, scenario := range []string{"machine", "participant_epoch", "account_epoch", "lifecycle_epoch", "closed", "unknown_status", "source_member_only"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			ctx := f.ctx
			switch scenario {
			case "machine":
				ctx = auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
			case "participant_epoch":
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
			case "account_epoch":
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.creator)
			case "lifecycle_epoch":
				f.fx.Exec(t, `UPDATE issue_collaboration SET lifecycle_epoch=lifecycle_epoch+1 WHERE issue_id=$1`, f.issue.Issue.ID)
			case "closed":
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
			case "unknown_status":
				f.fx.Exec(t, `UPDATE issue SET status='unrecognized_state' WHERE id=$1`, f.issue.Issue.ID)
			case "source_member_only":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
			}
			if _, err := f.svc.SubmitCommand(ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "must not enqueue"}); err == nil {
				t.Fatal("invalid authority accepted")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatal("denial persisted command")
			}
		})
	}
}

func TestCollaborationQueueCancelRechecksStateAfterSessionLock(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	command := f.submit(t, f.creator, "first")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	q := f.svc.Queries.WithTx(tx)
	if _, err = q.LockCollaborationSession(f.ctx, f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	var blocker int
	if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: command.CommandID, ExpectedCollaborationRevision: 1})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		f.fx.QueryRow(t, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked)
		if blocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancellation did not reach the session lock")
		}
		select {
		case err := <-done:
			t.Fatalf("cancel bypassed lock: %v", err)
		default:
		}
	}
	// This is a competing writer fixture, not a claim that promote exists.
	if _, err = tx.Exec(f.ctx, `UPDATE collaboration_command SET state='in_flight' WHERE id=$1`, command.CommandID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !collaborationErrorCode(err, "command_not_waiting") {
		t.Fatalf("cancel used pre-lock state: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='paused' AND control_version=1`, f.issue.SessionID) != 1 {
		t.Fatal("waiting cancellation became Stop")
	}
}

type queueFaultStarter struct {
	TxStarter
	begins                   atomic.Int32
	eventFailure, lostCommit bool
}
type queueFaultTx struct {
	pgx.Tx
	starter *queueFaultStarter
}
type queueFailureRow struct{ err error }

func (r queueFailureRow) Scan(...any) error { return r.err }
func (s *queueFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.begins.Add(1)
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &queueFaultTx{Tx: tx, starter: s}, nil
}
func (tx *queueFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.starter.eventFailure && strings.Contains(sql, "INSERT INTO collaboration_event") {
		return queueFailureRow{errors.New("injected event failure")}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *queueFaultTx) Commit(ctx context.Context) error {
	err := tx.Tx.Commit(ctx)
	if err == nil && tx.starter.lostCommit {
		return &pgconn.PgError{Code: "40001", Message: "injected unknown commit reply"}
	}
	return err
}

func TestCollaborationQueueEventRollbackAndUnknownCommitRecovery(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	fault := &queueFaultStarter{TxStarter: f.pool, eventFailure: true}
	f.svc.TxStarter = fault
	p := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "one immutable command"}
	if _, err := f.svc.SubmitCommand(f.ctx, p); err == nil {
		t.Fatal("event failure reported success")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("failed event left command or receipt committed")
	}
	fault.eventFailure = false
	fault.lostCommit = true
	before := fault.begins.Load()
	if _, err := f.svc.SubmitCommand(f.ctx, p); err == nil {
		t.Fatal("unknown commit reported success")
	}
	if fault.begins.Load() != before+1 {
		t.Fatal("unknown commit automatically retried")
	}
	f.svc.TxStarter = f.pool
	recovered, err := f.svc.SubmitCommand(f.ctx, p)
	if err != nil || recovered.CommandSequence != 1 {
		t.Fatalf("same-key recovery failed: %+v %v", recovered, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 1 {
		t.Fatal("unknown commit duplicated command")
	}
}

func TestCollaborationQueueResumeTreatsEitherBarrierAsUnconfirmed(t *testing.T) {
	for _, missing := range []string{"pointer", "run"} {
		t.Run(missing, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			run := f.run(t, f.submit(t, f.creator, "first"), 1)
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.stopParams(run, 1), ExpectedExecutionEpoch: enableControlGate(t, f.collaborationFixture)}
			if missing == "pointer" {
				f.fx.Exec(t, `UPDATE collaboration_target SET active_group_id=NULL WHERE id=$1`, f.targetID)
			} else {
				f.fx.Exec(t, `DELETE FROM collaboration_run WHERE id=$1`, run.ID)
			}
			denied, err := f.svc.ResumeQueue(f.ctx, p)
			if !collaborationErrorCode(err, "stop_unconfirmed") {
				t.Fatalf("inconsistent %s barrier must remain a stop refusal: %v", missing, err)
			}
			// Repair the fixture, not a production cleanup path: the original
			// refused intent must still replay its durable refusal afterwards.
			f.fx.Exec(t, `DELETE FROM collaboration_run WHERE id=$1`, run.ID)
			f.fx.Exec(t, `DELETE FROM collaboration_execution_group WHERE id=$1`, run.GroupID)
			f.fx.Exec(t, `UPDATE collaboration_target SET active_group_id=NULL WHERE id=$1`, f.targetID)
			replayed, err := f.svc.ResumeQueue(f.ctx, p)
			if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(replayed, denied) {
				t.Fatalf("barrier repair revived the old resume: %+v %v", replayed, err)
			}
		})
	}
}

func TestCollaborationQueueConcurrentSameKeyAndLargeSequence(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	const sequence int64 = 9007199254740993
	const controlVersion int64 = 9007199254740995
	f.fx.Exec(t, `UPDATE collaboration_target SET next_command_seq=$2,control_version=$3 WHERE id=$1`, f.targetID, sequence, controlVersion)
	p := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "same input"}
	start := make(chan struct{})
	type result struct {
		receipt CollaborationQueueReceipt
		err     error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() { <-start; receipt, err := f.svc.SubmitCommand(f.ctx, p); results <- result{receipt, err} }()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || !reflect.DeepEqual(first.receipt, second.receipt) || first.receipt.CommandSequence != sequence || first.receipt.ControlVersion != controlVersion {
		t.Fatalf("concurrent key or bigint identity changed: %+v %+v", first, second)
	}
	encoded, err := json.Marshal(first.receipt)
	if err != nil || !strings.Contains(string(encoded), `"command_sequence":"9007199254740993"`) || !strings.Contains(string(encoded), `"control_version":"9007199254740995"`) {
		t.Fatalf("sequence lost precision in durable receipt: %s %v", encoded, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 1 {
		t.Fatal("same request produced two commands")
	}
}

func TestCollaborationQueueReceiptOmitsAbsentCommandSequence(t *testing.T) {
	receipt := CollaborationQueueReceipt{Status: 409, Code: "stop_unconfirmed", QueueState: "paused", ControlVersion: 1}
	encoded, err := json.Marshal(receipt)
	if err != nil || strings.Contains(string(encoded), `"command_sequence"`) {
		t.Fatalf("missing command must not advertise a zero sequence: %s %v", encoded, err)
	}
	var decoded CollaborationQueueReceipt
	if err := json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(decoded, receipt) {
		t.Fatalf("receipt round trip changed the refusal: %+v %v", decoded, err)
	}
}
