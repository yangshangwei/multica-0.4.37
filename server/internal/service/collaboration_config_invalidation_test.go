package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type configInvalidationFixture struct {
	*managedCollaborationFixture
	worker *CollaborationConfigReconciler
	owned  []pgtype.UUID
}

func newConfigInvalidationFixture(t *testing.T) *configInvalidationFixture {
	t.Helper()
	f := &configInvalidationFixture{}
	url := os.Getenv("DATABASE_URL")
	// Fixture deletion itself emits resource tombstones. Run this last, on a
	// fresh connection after the fixture pool closes, and only for owned IDs.
	t.Cleanup(func() {
		if len(f.owned) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Errorf("connect for configuration fixture cleanup: %v", err)
			return
		}
		defer conn.Close(ctx)
		if _, err = conn.Exec(ctx, `DELETE FROM collaboration_config_invalidation WHERE resource_id=ANY($1::uuid[])`, f.owned); err != nil {
			t.Errorf("cleanup owned configuration intents: %v", err)
		}
	})
	f.managedCollaborationFixture = newManagedCollaborationFixture(t)
	f.owned = []pgtype.UUID{f.grant.AgentID, f.runtime.ID}
	f.worker = NewCollaborationConfigReconciler(f.svc.Queries, f.pool)
	return f
}

func (f *configInvalidationFixture) updateAgent(t *testing.T) {
	t.Helper()
	if _, err := f.svc.Queries.UpdateAgent(f.ctx, db.UpdateAgentParams{ID: f.grant.AgentID, ExpectedWorkspaceID: util.MustParseUUID(f.workspace), Instructions: pgtype.Text{String: "changed execution instructions", Valid: true}}); err != nil {
		t.Fatal(err)
	}
}

func (f *configInvalidationFixture) drain(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	// Other service and handler tests also emit durable intents. Keep the
	// real scheduling order while waiting only for this fixture's resources.
	for {
		var pending int
		err := f.pool.QueryRow(ctx, `SELECT count(*) FROM collaboration_config_invalidation
			WHERE resource_id=ANY($1::uuid[]) AND reconciled_at IS NULL`, f.owned).Scan(&pending)
		if err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			return
		}
		processed, err := f.worker.Reconcile(ctx, 128)
		if err != nil {
			t.Fatal(err)
		}
		if processed == 0 {
			t.Fatalf("fixture still has %d pending configuration intents without consumer progress", pending)
		}
	}
}

func TestCollaborationConfigurationWritersStopOnlyDependentWork(t *testing.T) {
	for _, change := range []string{"agent_instructions", "runtime_visibility", "invoke_membership", "skill_binding"} {
		t.Run(change, func(t *testing.T) {
			f := newConfigInvalidationFixture(t)
			other, otherGrant := unrelatedResourceTarget(t, f.managedCollaborationFixture)
			f.owned = append(f.owned, otherGrant.AgentID, otherGrant.RuntimeID)
			run, claim, permit := startResourceCollaboration(t, f.managedCollaborationFixture)
			other = queueTarget(t, f.collaborationQueueFixture, other.ID)
			waiting := f.submitManaged(t, f.handler, "waiting input keeps its original identity")
			channel := resourceChannelState(t, f.managedCollaborationFixture)
			q := f.svc.Queries
			var err error
			switch change {
			case "agent_instructions":
				f.updateAgent(t)
			case "runtime_visibility":
				_, err = q.UpdateAgentRuntimeVisibility(f.ctx, db.UpdateAgentRuntimeVisibilityParams{ID: f.runtime.ID, Visibility: "public"})
			case "invoke_membership":
				err = q.CreateAgentInvocationTarget(f.ctx, db.CreateAgentInvocationTargetParams{AgentID: f.grant.AgentID, TargetType: "member", TargetID: util.MustParseUUID(f.handler), CreatedBy: util.MustParseUUID(f.creator)})
				f.fx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, f.grant.AgentID)
			case "skill_binding":
				skill := f.fx.Insert(t, "skill", testutil.Cols{"workspace_id": f.workspace, "name": "Configuration bound skill " + uuid.NewString(), "content": "test-owned skill", "created_by": f.creator})
				err = q.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: f.grant.AgentID, SkillID: util.MustParseUUID(skill)})
				f.fx.Cleanup(t, `DELETE FROM agent_skill WHERE agent_id=$1`, f.grant.AgentID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=ANY($1::uuid[]) AND reconciled_at IS NULL`, []pgtype.UUID{f.grant.AgentID, f.runtime.ID}) != 1 {
				t.Fatal("actual configuration writer did not atomically retain one invalidation intent")
			}
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
				InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "must not enter on old config"}); !collaborationErrorCode(err, "target_configuration_changed") {
				t.Fatalf("old authority survived writer commit before reconciliation: %v", err)
			}
			f.drain(t)
			if resourceChannelState(t, f.managedCollaborationFixture) != channel ||
				!reflect.DeepEqual(other, queueTarget(t, f.collaborationQueueFixture, other.ID)) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, otherGrant.ID) != 1 {
				t.Fatal("configuration loss paused its lane or changed an unrelated target")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, f.targetID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, f.grant.ID, f.grant.Epoch+1) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
				t.Fatal("configuration convergence lost hold, exact grant revocation or retained waiting input")
			}
			intent, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
			if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
				t.Fatalf("configuration stop changed original supervisor identity: %+v %v", intent, err)
			}
			if _, err = f.svc.AcceptRuntimeProof(t.Context(), run, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='exited_proven' AND settled_at IS NOT NULL`, run) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, f.targetID) != 1 {
				t.Fatal("historical proof did not settle just the old execution or released its hold")
			}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			f.worker = NewCollaborationConfigReconciler(q, f.pool)
			f.drain(t)
			if before != controlStateSnapshot(t, f.collaborationQueueFixture) || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='configuration_invalidated'`, f.issue.Issue.ID) != 1 {
				t.Fatal("restarted consumer repeated committed source effects")
			}
		})
	}
}

func TestCollaborationConfigurationOldIntentPreservesNewConsentAndRunningGroup(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	f.updateAgent(t)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(target), Roster: roster}); err != nil {
		t.Fatal(err)
	}
	fresh := consentManagedControlTarget(t, f.managedCollaborationFixture, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	run, claim, _ := startResourceCollaboration(t, f.managedCollaborationFixture)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	f.drain(t)
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=$2`, fresh.ID, fresh.Epoch) != 1 {
		t.Fatal("an old configuration intent revoked successfully reconfigured current authority")
	}
	intent, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
	if err != nil || intent.StopRequested {
		t.Fatalf("an old configuration intent stopped a fresh group: %+v %v", intent, err)
	}
}

func TestCollaborationSquadConfigurationHoldsOnlyItsAlias(t *testing.T) {
	for _, change := range []string{"leader", "add_member", "remove_member", "member_role"} {
		t.Run(change, func(t *testing.T) {
			f := newConfigInvalidationFixture(t)
			squad, grant := registerManagedControlTarget(t, f.managedCollaborationFixture, true)
			f.owned = append(f.owned, squad.ResourceID)
			if change == "remove_member" || change == "member_role" {
				// A leader member row is included before refreshing consent, so
				// only the later real writer is obsolete in this test.
				f.fx.SquadMember(t, util.UUIDToString(squad.ResourceID), "agent", util.UUIDToString(f.grant.AgentID), testutil.Cols{"role": "leader"})
				roster, err := decodeCollaborationRoster[collaboration.TargetResource](squad.Roster)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(queueTarget(t, f.collaborationQueueFixture, squad.ID)), Roster: roster}); err != nil {
					t.Fatal(err)
				}
				grant = consentManagedControlTarget(t, f.managedCollaborationFixture, queueTarget(t, f.collaborationQueueFixture, squad.ID), f.grant.AgentID)
				f.drain(t)
			}
			_, claim, _ := startResourceCollaboration(t, f.managedCollaborationFixture)
			beforeChannel := resourceChannelState(t, f.managedCollaborationFixture)
			beforeAgent := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			q := f.svc.Queries
			var err error
			switch change {
			case "leader", "add_member":
				other := util.MustParseUUID(f.fx.Agent(t, "new squad agent "+uuid.NewString(), util.UUIDToString(f.runtime.ID)))
				f.owned = append(f.owned, other)
				if change == "leader" {
					_, err = q.UpdateSquad(f.ctx, db.UpdateSquadParams{ID: squad.ResourceID, ExpectedWorkspaceID: util.MustParseUUID(f.workspace), LeaderID: other})
				} else {
					_, err = q.AddSquadMember(f.ctx, db.AddSquadMemberParams{SquadID: squad.ResourceID, MemberType: "agent", MemberID: other, Role: "developer"})
					f.fx.Cleanup(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, squad.ResourceID, other)
				}
			case "remove_member":
				_, err = q.RemoveSquadMember(f.ctx, db.RemoveSquadMemberParams{SquadID: squad.ResourceID, MemberType: "agent", MemberID: f.grant.AgentID})
			case "member_role":
				_, err = q.UpdateSquadMemberRole(f.ctx, db.UpdateSquadMemberRoleParams{SquadID: squad.ResourceID, MemberType: "agent", MemberID: f.grant.AgentID, Role: "changed execution role"})
			}
			if err != nil {
				t.Fatal(err)
			}
			f.drain(t)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, squad.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL`, grant.ID) != 1 ||
				beforeChannel != resourceChannelState(t, f.managedCollaborationFixture) || !reflect.DeepEqual(beforeAgent, queueTarget(t, f.collaborationQueueFixture, f.targetID)) {
				t.Fatal("squad drift did not hold just its target or disturbed its active direct-agent alias")
			}
			stop, err := f.svc.RuntimeStopIntent(t.Context(), util.MustParseUUID(claim.Identity.RunID), claim.TerminationToken)
			if err != nil || stop.StopRequested {
				t.Fatalf("an unused squad dependency stopped the direct-agent group: %+v %v", stop, err)
			}
		})
	}
}

func TestCollaborationConfigurationWriterCommitsWhileSourceBusyAndConsumerRetries(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	holder, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	f.updateAgent(t)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	counting := &queueFaultStarter{TxStarter: f.pool}
	f.worker.TxStarter = counting
	if processed, err := f.worker.Reconcile(t.Context(), 1); err == nil || processed != 0 || counting.begins.Load() != 3 {
		t.Fatalf("busy source must retain intent after three complete rolled-back attempts: %d %d %v", processed, counting.begins.Load(), err)
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) || f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1 AND last_issue_id IS NULL AND last_attempted_at IS NOT NULL AND reconciled_at IS NULL`, f.grant.AgentID) != 1 {
		t.Fatal("source conflict partially committed hold or advanced the durable cursor")
	}
	if err = holder.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.worker = NewCollaborationConfigReconciler(f.svc.Queries, f.pool)
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, f.targetID) != 1 {
		t.Fatal("restarted consumer lost the busy source's durable invalidation")
	}
}

func TestCollaborationConfigurationAtomicFailureAndUnknownCommit(t *testing.T) {
	for _, failure := range []string{"event", "cursor", "commit", "unknown_commit"} {
		t.Run(failure, func(t *testing.T) {
			f := newConfigInvalidationFixture(t)
			startResourceCollaboration(t, f.managedCollaborationFixture)
			f.updateAgent(t)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			fault := &accountReconciliationFaultStarter{TxStarter: f.pool, failure: failure}
			f.worker.TxStarter = fault
			if processed, err := f.worker.Reconcile(t.Context(), 1); err == nil || processed != 0 || IsRuntimeResourceBusy(err) || fault.begins.Load() != 1 {
				t.Fatalf("non-retryable failure was replayed: %d %d %v", processed, fault.begins.Load(), err)
			}
			if failure != "unknown_commit" && (before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1 AND last_issue_id IS NOT NULL`, f.grant.AgentID) != 0) {
				t.Fatal("failed source transaction committed only part of its hold/stop/cursor")
			}
			f.worker = NewCollaborationConfigReconciler(f.svc.Queries, f.pool)
			f.drain(t)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='configuration_invalidated'`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, f.grant.ID, f.grant.Epoch+1) != 1 {
				t.Fatal("recovering the original intent duplicated revocation or source history")
			}
		})
	}
}

func TestCollaborationConfigurationBackgroundConsumerRunsWithGateDisabled(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	f.updateAgent(t)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,epoch=epoch+1 WHERE id=1`)
	committed := make(chan struct{})
	var once sync.Once
	f.worker.TxStarter = &accountReconciliationFaultStarter{TxStarter: f.pool, afterCommit: func() { once.Do(func() { close(committed) }) }}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); f.worker.Run(ctx) }()
	select {
	case <-committed:
	case <-time.After(5 * time.Second):
		t.Fatal("disabled execution gate blocked the background configuration consumer")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("configuration consumer did not stop with the server sweep context")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, f.targetID) != 1 {
		t.Fatal("background consumer committed without converging the stale target")
	}
}

func TestCollaborationConfigurationDeletedResourceWithoutGrantRetainsProof(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	run, claim, permit := startResourceCollaboration(t, f.managedCollaborationFixture)
	f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, f.grant.ID)
	// This is the underlying hard-delete writer, without the explicit teardown
	// helper. Keep its legacy parent relationship valid; the deletion outbox
	// must still find the retained roster and run after the resource is gone.
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := f.svc.Queries.WithTx(tx)
	if _, err = q.UnbindUserAgentsFromRuntime(f.ctx, f.runtime.ID); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteAgentRuntime(f.ctx, f.runtime.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.drain(t)
	intent, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("resource deletion hid retained execution without its grant: %+v %v", intent, err)
	}
	if _, err = f.svc.AcceptRuntimeProof(t.Context(), run, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationConfigurationAttemptFairnessSurvivesRestart(t *testing.T) {
	blocked := newConfigInvalidationFixture(t)
	healthy := newConfigInvalidationFixture(t)
	blocked.updateAgent(t)
	healthy.updateAgent(t)
	holder, err := blocked.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(t.Context(), `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, blocked.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	if processed, err := blocked.worker.Reconcile(t.Context(), 1); processed != 0 || err == nil {
		t.Fatalf("first blocked source did not retain pending work: %d %v", processed, err)
	}
	restarted := NewCollaborationConfigReconciler(blocked.svc.Queries, blocked.pool)
	if processed, err := restarted.Reconcile(t.Context(), 1); processed != 1 || err != nil {
		t.Fatalf("persisted attempt order did not let another resource progress: %d %v", processed, err)
	}
	if healthy.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, healthy.targetID) != 1 ||
		blocked.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1 AND last_issue_id IS NULL AND reconciled_at IS NULL`, blocked.grant.AgentID) != 1 {
		t.Fatal("fair scheduling advanced a blocked source or skipped a healthy one")
	}
	if err = holder.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	blocked.worker = restarted
	blocked.drain(t)
}

func TestCollaborationConfigurationConcurrentConsumersCommitOneEffect(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	f.updateAgent(t)
	before := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			worker := NewCollaborationConfigReconciler(f.svc.Queries, f.pool)
			_, err := worker.Reconcile(t.Context(), 8)
			results <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	f.drain(t)
	after := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if after.HoldReason.String != "config_invalid" || after.HoldVersion != before.HoldVersion+1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='configuration_invalidated'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1 AND reconciled_at IS NOT NULL`, f.grant.AgentID) != 1 {
		t.Fatal("concurrent consumers replayed the source effect or lost the original intent")
	}
}

func TestCollaborationConfigurationChildWriterRollbackIsAtomic(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := f.svc.Queries.WithTx(tx)
	if err = q.CreateAgentInvocationTarget(t.Context(), db.CreateAgentInvocationTargetParams{AgentID: f.grant.AgentID, TargetType: "member", TargetID: util.MustParseUUID(f.handler)}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_invocation_target WHERE agent_id=$1`, f.grant.AgentID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.grant.AgentID) != 0 {
		t.Fatal("child rollback retained an epoch/intent or invocation row")
	}
	var agent db.Agent
	agent, err = f.svc.Queries.GetAgent(t.Context(), f.grant.AgentID)
	if err != nil || agent.CollaborationConfigEpoch != 1 {
		t.Fatalf("rolled-back invoke change advanced authority: %d %v", agent.CollaborationConfigEpoch, err)
	}
	// A reverse child writer gets NOWAIT rather than holding a child while
	// blocking on the parent held by a regular source admission transaction.
	blocker, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(t.Context(), `SELECT id FROM agent WHERE id=$1 FOR SHARE`, f.grant.AgentID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_, err = f.pool.Exec(ctx, `INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES ($1,'member',$2)`, f.grant.AgentID, f.handler)
	var conflict *pgconn.PgError
	if !errors.As(err, &conflict) || conflict.Code != "55P03" {
		t.Fatalf("reverse child writer must refuse its parent immediately: %v", err)
	}
}
