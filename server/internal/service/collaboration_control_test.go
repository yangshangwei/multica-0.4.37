package service

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These service regressions use persisted synthetic execution evidence. Actual
// provider containment and proof ingestion are verified by the runtime suites.
func TestCollaborationStopReturnsNeverLaunchedBatchInputs(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	first := f.submit(t, f.creator, "first author input")
	second := f.submit(t, f.handler, "second author input")
	before, err := f.svc.Queries.ListCollaborationBatchInputs(f.ctx, first.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	run := f.run(t, first, 0)
	seedControlReservation(t, f, run)
	waiting := f.submit(t, f.handler, "a later batch")
	p := f.stopParams(run, 1)
	r, err := f.svc.StopQueue(f.ctx, p)
	if err != nil || !r.GroupSettled {
		t.Fatalf("never-permitted batch did not settle: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting' AND group_id IS NULL`, []pgtype.UUID{first.CommandID, second.CommandID}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE id=$1 AND state='stopped' AND settled_at IS NOT NULL`, first.BatchID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='paused' AND active_group_id IS NULL`, first.ChannelID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, run.ID) != 1 {
		t.Fatal("stopping the unlaunched group did not return unread input, finish the batch and release only its channel position")
	}
	commands, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	var after []db.CollaborationCommand
	for _, command := range commands {
		if command.ID == first.CommandID || command.ID == second.CommandID {
			command.Reason, command.BatchID = before[0].Reason, first.BatchID
			after = append(after, command)
		}
	}
	if !reflect.DeepEqual(before, after) || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
		t.Fatal("batch settlement rewrote immutable author/input identity or consumed the waiting batch")
	}
	replayed, err := f.svc.StopQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, replayed) {
		t.Fatalf("whole-batch stop receipt changed on replay: %+v %v", replayed, err)
	}
}

func seedControlReservation(t *testing.T, f *collaborationQueueFixture, run db.CollaborationRun) {
	t.Helper()
	f.fx.InsertNoID(t, "collaboration_runtime_reservation", testutil.Cols{
		"run_id": run.ID, "execution_workspace_id": run.ExecutionWorkspaceID, "agent_id": run.AgentID,
		"runtime_id": run.RuntimeID, "daemon_id": "control-test-" + uuid.NewString(),
		"installation_id": run.InstallationID, "binding_id": run.BindingID,
	}, "run_id=$1", run.ID)
}

func TestCollaborationProvenSettlementRejectsOrphanDelegation(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	run := f.run(t, f.submit(t, f.creator, "retained proven input"), 0)
	seedControlReservation(t, f, run)
	// Persist synthetic server-proven nonlaunch evidence without using Stop,
	// so the shared completion path must validate the descendant ledger itself.
	f.fx.Exec(t, `UPDATE collaboration_run SET forced_disposition='stopped',proof_state='never_launched_proven',sealed_attempt_seq=0,progress_state='settled',settled_at=now() WHERE id=$1`, run.ID)
	f.fx.Exec(t, `UPDATE collaboration_execution_group SET forced_disposition='stopped',expansion_sealed_at=now(),state='ending' WHERE id=$1`, run.GroupID)
	f.fx.Insert(t, "collaboration_delegation", testutil.Cols{"group_id": run.GroupID, "parent_run_id": uuid.NewString(),
		"request_key": "orphan", "payload_hash": "fixture", "phase_key": "phase", "members": "[]", "continuation_key": "continuation", "state": "cancelled"})
	before := controlStateSnapshot(t, f)
	_, err := f.svc.mutateCollaborationQueue(f.ctx, f.request(), "test_group_settlement", struct{}{}, func(tx *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if _, err := tx.q.LockCollaborationSessionCommands(f.ctx, f.issue.SessionID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		group, err := tx.q.LockCollaborationRuntimeGroup(f.ctx, run.GroupID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		target, _ := tx.target(group.TargetID)
		_, err = tx.settleProvenGroup(f.ctx, target, group)
		return tx.receipt("test_group_settlement", 200), err
	})
	if err == nil || controlStateSnapshot(t, f) != before {
		t.Fatalf("shared proof settlement accepted an orphan descendant or partially released its barrier: %v", err)
	}
}

func enableControlGate(t *testing.T, f *collaborationFixture) int64 {
	t.Helper()
	gate, err := f.svc.Queries.LockCollaborationGateShared(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true WHERE id=1`)
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET enabled=$1,epoch=$2 WHERE id=1`, gate.Enabled, gate.Epoch)
	return gate.Epoch
}

func controlStateSnapshot(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'session', (SELECT to_jsonb(s) FROM collaboration_session s WHERE s.id=$1),
		'channels', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$1),
		'targets', (SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id=$1),
		'commands', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1),
		'groups', (SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$1),
		'runs', (SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$1),
		'grants', (SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$2),
		'delegations', (SELECT jsonb_agg(to_jsonb(d) ORDER BY d.id) FROM collaboration_delegation d
			JOIN collaboration_execution_group g ON g.id=d.group_id WHERE g.session_id=$1),
		'terminations', (SELECT jsonb_agg(to_jsonb(p) ORDER BY p.run_id) FROM collaboration_termination p WHERE p.issue_id=$2),
		'capabilities', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.run_id,c.attempt_seq) FROM collaboration_business_capability c
			JOIN collaboration_run r ON r.id=c.run_id WHERE r.session_id=$1),
		'attempts', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a
			JOIN collaboration_run r ON r.id=a.run_id WHERE r.session_id=$1),
		'reservations', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.run_id) FROM collaboration_runtime_reservation v
			JOIN collaboration_run r ON r.id=v.run_id WHERE r.session_id=$1),
		'events', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM collaboration_event e WHERE e.issue_id=$2)
	)::text`, f.issue.SessionID, f.issue.Issue.ID).Scan(&snapshot)
	return snapshot
}

func TestCollaborationResumeAllGlobalGuardsPreserveState(t *testing.T) {
	for _, scenario := range []struct{ name, code string }{
		{"closed", "issue_closed"}, {"terminal", "command_unavailable"}, {"admission", "command_unavailable"},
		{"scope", "stale_scope"}, {"session_control", "stale_control_version"}, {"channel_control", "stale_control_version"},
		{"group_generation", "stale_execution"}, {"target_epoch", "stale_target"}, {"config_version", "stale_target"},
		{"gate_epoch", "stale_admission"}, {"gate_disabled", "collaboration_execution_unavailable"},
		{"lifecycle", "stale_authority"}, {"participant", "stale_authority"}, {"account", "stale_authority"},
		{"requester", "not_found"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.submit(t, f.creator, "waiting input survives global refusal")
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: enableControlGate(t, f.collaborationFixture)}
			ctx := f.ctx
			switch scenario.name {
			case "closed":
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
			case "terminal":
				f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
			case "admission":
				f.fx.Exec(t, `UPDATE issue SET admission_status='pending' WHERE id=$1`, f.issue.Issue.ID)
			case "scope":
				p.ExpectedTargetSetVersion++
			case "session_control":
				p.ExpectedControlVersion++
			case "channel_control":
				p.Targets[0].ExpectedControlVersion++
			case "group_generation":
				p.Targets[0].ObservedGroupGeneration++
			case "target_epoch":
				p.Targets[0].ExpectedTargetEpoch++
			case "config_version":
				p.Targets[0].ExpectedConfigVersion++
			case "gate_epoch":
				p.ExpectedExecutionEpoch++
			case "gate_disabled":
				f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false WHERE id=1`)
			case "lifecycle":
				p.ExpectedLifecycleEpoch++
			case "participant":
				p.ExpectedParticipantEpoch++
			case "account":
				p.ExpectedAccountEpoch++
			case "requester":
				ctx = collaborationContext(f.outsider)
			}
			before := controlStateSnapshot(t, f)
			r, err := f.svc.ResumeQueue(ctx, p)
			if !collaborationErrorCode(err, scenario.code) {
				t.Fatalf("global guard should return %s: %+v %v", scenario.code, r, err)
			}
			if after := controlStateSnapshot(t, f); after != before {
				t.Fatal("global refusal changed control, input, proof, reservation or event state")
			}
			replayed, err := f.svc.ResumeQueue(ctx, p)
			if !collaborationErrorCode(err, scenario.code) || !reflect.DeepEqual(replayed, r) || controlStateSnapshot(t, f) != before {
				t.Fatalf("global refusal was not replayed unchanged: %+v %v", replayed, err)
			}
		})
	}
}

func TestCollaborationResumeAllRetainsRemovedChannelReservation(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	run := f.run(t, f.submit(t, f.creator, "removed target remains possibly live"), 1)
	seedControlReservation(t, f, run)
	if _, err := f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM collaboration_target WHERE id=$1`, run.TargetID)
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: enableControlGate(t, f.collaborationFixture)}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.Code != "partially_resumed" || r.QueueState != "active" || r.ControlVersion != p.ExpectedControlVersion+1 {
		t.Fatalf("removed target prevented release of the global pause: %+v %v", r, err)
	}
	if len(r.Channels) != 1 || r.Channels[0].ChannelID != run.ChannelID || r.Channels[0].Code != "stop_unconfirmed" || len(r.TargetHolds) != 0 {
		t.Fatalf("removed target hid the retained channel's control result: %+v", r)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL AND proof_state='unknown'`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND active_group_id=$2 AND queue_state='paused'`, run.ChannelID, run.GroupID) != 1 {
		t.Fatal("global resume released the removed target's unproven physical reservation")
	}
	before := controlStateSnapshot(t, f)
	replayed, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, replayed) || controlStateSnapshot(t, f) != before {
		t.Fatalf("removed-channel partial receipt changed on replay: %+v %v", replayed, err)
	}
}

type controlCommitStarter struct {
	TxStarter
	ready   chan int
	release <-chan struct{}
}

type controlCommitTx struct {
	pgx.Tx
	starter *controlCommitStarter
}

func (s *controlCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &controlCommitTx{Tx: tx, starter: s}, nil
}

func (tx *controlCommitTx) Commit(ctx context.Context) error {
	var pid int
	if err := tx.Tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		return err
	}
	select {
	case tx.starter.ready <- pid:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-tx.starter.release:
		return tx.Tx.Commit(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCollaborationResumeAndFreezeSerializeThroughCommit(t *testing.T) {
	for _, first := range []string{"resume", "freeze"} {
		t.Run(first+"_first", func(t *testing.T) {
			base := newCollaborationFixture(t)
			issue, err := base.svc.Create(base.ctx, base.params(t))
			if err != nil {
				t.Fatal(err)
			}
			f := &collaborationQueueFixture{collaborationFixture: base, issue: issue}
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: enableControlGate(t, base)}
			ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
			defer cancel()
			before := controlStateSnapshot(t, f)
			freeze, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer freeze.Rollback(context.WithoutCancel(ctx))
			var freezePID int
			if err = freeze.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&freezePID); err != nil {
				t.Fatal(err)
			}
			resumeDone := make(chan error, 1)
			resumeResult := make(chan CollaborationQueueReceipt, 1)
			resume := func() {
				r, err := f.svc.ResumeQueue(ctx, p)
				resumeResult <- r
				resumeDone <- err
			}
			if first == "freeze" {
				if _, err = freeze.Exec(ctx, `UPDATE collaboration_execution_gate SET enabled=false,epoch=epoch+1 WHERE id=1`); err != nil {
					t.Fatal(err)
				}
				go resume()
				waitWorkflowBlocked(t, f, freezePID, resumeDone)
				if err = freeze.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err = <-resumeDone; !collaborationErrorCode(err, "stale_admission") {
					t.Fatalf("old resume crossed the committed freeze: %v", err)
				}
				if controlStateSnapshot(t, f) != before {
					t.Fatal("resume wrote source state after the gate epoch changed")
				}
				return
			}
			release := make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			starter := &controlCommitStarter{TxStarter: f.pool, ready: make(chan int, 1), release: release}
			f.svc.TxStarter = starter
			go resume()
			var resumePID int
			select {
			case resumePID = <-starter.ready:
			case err = <-resumeDone:
				t.Fatalf("resume ended before its commit fence: %v", err)
			case <-ctx.Done():
				t.Fatal("resume did not reach the commit fence")
			}
			freezeDone := make(chan error, 1)
			go func() {
				_, err := freeze.Exec(ctx, `UPDATE collaboration_execution_gate SET enabled=false,epoch=epoch+1 WHERE id=1`)
				if err == nil {
					err = freeze.Commit(ctx)
				}
				freezeDone <- err
			}()
			waitWorkflowBlocked(t, f, resumePID, freezeDone)
			unblock()
			if err = <-resumeDone; err != nil {
				t.Fatal(err)
			}
			if err = <-freezeDone; err != nil {
				t.Fatal(err)
			}
			if r := <-resumeResult; r.Code != "resumed" || r.QueueState != "active" {
				t.Fatalf("completed resume lost its result before the subsequent freeze: %+v", r)
			}
			gate, err := f.svc.Queries.LockCollaborationGateShared(ctx)
			if err != nil || gate.Enabled || gate.Epoch != p.ExpectedExecutionEpoch+1 {
				t.Fatalf("freeze did not follow the original resume commit: %+v %v", gate, err)
			}
		})
	}
}

func TestCollaborationStopAliasIncludesEveryGroupDescendant(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, memberGrant := seedQueueSquad(t, f, true)
	direct := f.submit(t, f.creator, "direct alias input")
	squadInput := submitTarget(t, f, squad.ID, f.handler, "squad alias input", pgtype.UUID{})
	root := f.run(t, direct, 1)
	member := seedQueueMember(t, f, root, memberGrant, 1)
	// The read contract exposes one channel position through each typed alias.
	f.fx.Exec(t, `UPDATE collaboration_target SET active_group_id=$2,group_generation=$3,queue_state='active' WHERE channel_id=$1`, direct.ChannelID, root.GroupID, f.groupGeneration(squad.ID))
	other, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.handler), 1)
	otherRun := f.run(t, submitTarget(t, f, other.ID, f.handler, "unrelated channel", pgtype.UUID{}), 1)
	otherBefore := queueTarget(t, f, other.ID)
	p := CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget,
		Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f, f.targetID))}}
	r, err := f.svc.StopQueue(f.ctx, p)
	if err != nil {
		t.Fatalf("direct alias could not stop the shared squad channel: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=ANY($1::uuid[]) AND forced_disposition='stopped' AND settled_at IS NULL`, []pgtype.UUID{root.ID, member.ID}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=ANY($1::uuid[]) AND queue_state='paused' AND control_version=2 AND active_group_id=$2`, []pgtype.UUID{f.targetID, squad.ID}, root.GroupID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='paused' AND control_version=2 AND active_group_id=$2`, direct.ChannelID, root.GroupID) != 1 {
		t.Fatal("alias stop missed a descendant, advanced the channel twice, or cleared unproven work")
	}
	if !reflect.DeepEqual(otherBefore, queueTarget(t, f, other.ID)) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL`, otherRun.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='in_flight'`, []pgtype.UUID{direct.CommandID, squadInput.CommandID}) != 2 {
		t.Fatal("alias stop reached unrelated execution or prematurely settled the batch")
	}
}

func TestCollaborationResumeAllEmptyScopeReleasesGlobalPause(t *testing.T) {
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	gate, err := f.svc.Queries.LockCollaborationGateShared(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true WHERE id=1`)
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET enabled=$1 WHERE id=1`, gate.Enabled)
	qf := &collaborationQueueFixture{collaborationFixture: f, issue: issue}
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: qf.issueControl(t), ExpectedExecutionEpoch: gate.Epoch}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.Code != "resumed" || r.QueueState != "active" || r.ControlVersion != 2 || len(r.Targets) != 0 {
		t.Fatalf("empty issue scope should release only the global pause: %+v %v", r, err)
	}
	replayed, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, replayed) {
		t.Fatalf("successful resume receipt changed on replay: %+v %v", replayed, err)
	}
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, issue.Issue.ID)
	if err != nil || session.QueueState != "active" || session.ControlVersion != 2 {
		t.Fatalf("empty resume repeated its control transition: %+v %v", session, err)
	}
}
