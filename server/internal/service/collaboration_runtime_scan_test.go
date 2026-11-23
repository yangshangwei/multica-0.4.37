package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// List commits scheduling metadata before it locks an issue. FIFO races must
// pause the actual source transaction, not that earlier cursor transaction.
type runtimeSourceCommitStarter struct{ recipientCommitStarter }

type runtimeSourceCommitTx struct {
	recipientCommitTx
	sourceLocked bool
}

func (s runtimeSourceCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &runtimeSourceCommitTx{recipientCommitTx: recipientCommitTx{Tx: tx, reached: s.reached, release: s.release}}, nil
}

func (tx *runtimeSourceCommitTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: LockCollaborationIssue ") {
		tx.sourceLocked = true
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx *runtimeSourceCommitTx) Commit(ctx context.Context) error {
	if tx.sourceLocked {
		return tx.recipientCommitTx.Commit(ctx)
	}
	return tx.Tx.Commit(ctx)
}

func newRuntimeScanLanes(t *testing.T, count int) (*managedCollaborationFixture, []pgtype.UUID) {
	t.Helper()
	f := newManagedCollaborationFixture(t)
	targets := []pgtype.UUID{f.targetID}
	for range count - 1 {
		session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		agent := f.fx.Agent(t, "Scan lane "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
		registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetAgent,
			ResourceID: util.MustParseUUID(agent), ExpectedTargetSetVersion: session.TargetSetVersion,
			Roster: []collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: registered.TargetID,
			AgentID: util.MustParseUUID(agent), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
			Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: 1}}); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, registered.TargetID)
	}
	sort.Slice(targets, func(i, j int) bool { return util.UUIDToString(targets[i]) < util.UUIDToString(targets[j]) })
	for i, target := range targets {
		f.submitManagedTo(t, target, fmt.Sprintf("lane %d first input", i))
	}
	f.resumeManaged(t)
	return f, targets
}

// Distinct comments produce independent member runs with their own immutable
// identities. Ending the leader releases the actual namespace before the scan.
func newRuntimeScanMembers(t *testing.T, count int) (*managedCollaborationFixture, []pgtype.UUID, pgtype.UUID) {
	t.Helper()
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "scan member inputs")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	for i := range count {
		receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
			CommentID: util.MustParseUUID(uuid.NewString()), Content: fmt.Sprintf("[@member](mention://agent/%s) item %d", util.UUIDToString(member), i),
		})
		if err != nil || len(receipt.RunIDs) != 1 {
			t.Fatalf("member %d: %+v %v", i, receipt, err)
		}
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	rows, err := f.pool.Query(f.ctx, `SELECT id FROM collaboration_run WHERE parent_run_id=$1 AND step_kind='member' ORDER BY created_at,id`, root)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	runs, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
	if err != nil || len(runs) != count {
		t.Fatalf("member inventory: %v %v", runs, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1 AND released_at IS NULL`, f.runtime.ID) != 0 {
		t.Fatal("the fixture must not claim progress past a live namespace")
	}
	return f, runs, root
}

func TestCollaborationRuntimeWaitingScanAdvancesPersistentRefusals(t *testing.T) {
	f, targets := newRuntimeScanLanes(t, 18)
	healthy := targets[len(targets)-1]
	f.submitManagedTo(t, healthy, "healthy second input")
	fault := &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationChannel", faults: make(map[pgtype.UUID]error)}
	for _, id := range targets[:len(targets)-1] {
		fault.faults[queueTarget(t, f.collaborationQueueFixture, id).ChannelID] = collaborationError(http.StatusConflict, "runtime_upgrade_required")
	}
	var offered []string
	for range 4 {
		// A new service instance has no process-local cursor to remember.
		svc := NewCollaborationService(db.New(f.pool), fault, f.svc.Issues)
		list, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(list.RunIDs) > 0 {
			offered = list.RunIDs
			break
		}
	}
	if len(offered) != 1 {
		t.Fatalf("healthy target after 17 persistent refusals was never promoted: %v", offered)
	}
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, util.MustParseUUID(offered[0]), f.managedClaimRequest())
	if err != nil || claim.Identity.TargetID != util.UUIDToString(healthy) || len(claim.Inputs) != 2 || claim.Inputs[0].Body != "lane 17 first input" || claim.Inputs[1].Body != "healthy second input" {
		t.Fatalf("healthy claim lost target identity or FIFO: %+v %v", claim, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=ANY($1::uuid[]) AND state='waiting' AND group_id IS NULL`, targets[:17]) != 17 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=ANY($1::uuid[]) AND hold_reason IS NOT NULL`, targets[:17]) != 0 {
		t.Fatal("scanning rewrote refused input or manufactured a target hold")
	}
}

func TestCollaborationRuntimeCandidateScanAdvancesPersistentRefusals(t *testing.T) {
	f, runs, _ := newRuntimeScanMembers(t, 18)
	healthy := runs[len(runs)-1]
	fault := &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationRuntimeRun", faults: make(map[pgtype.UUID]error)}
	for _, id := range runs[:len(runs)-1] {
		fault.faults[id] = collaborationError(http.StatusConflict, "stale_authority")
	}
	claimed := false
	for range len(runs) {
		svc := NewCollaborationService(db.New(f.pool), fault, f.svc.Issues)
		list, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
		if err != nil || len(list.RunIDs) != 1 {
			t.Fatalf("bounded list: %+v %v", list, err)
		}
		// Production dispatch consumes only the first candidate from each List.
		// Advancing a whole page here can repeatedly hide its unattempted tail.
		if len(list.RunIDs) > 0 {
			value := list.RunIDs[0]
			id := util.MustParseUUID(value)
			_, err = svc.ClaimRuntimeRun(f.daemonCtx, id, f.managedClaimRequest())
			if id == healthy {
				if err != nil {
					t.Fatalf("healthy member could not really claim: %v", err)
				}
				claimed = true
			} else if !collaborationErrorCode(err, "stale_authority") {
				t.Fatalf("persistent refusal unexpectedly changed: %v", err)
			}
		}
		if claimed {
			break
		}
	}
	if !claimed {
		t.Fatal("the candidate behind 17 persistent claim refusals never received an opportunity")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1 AND released_at IS NULL`, f.runtime.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=ANY($1::uuid[]) AND progress_state='prepared' AND authorized_attempt_seq=0`, runs[:17]) != 17 {
		t.Fatal("candidate fairness changed rejected runs or bypassed actual capacity")
	}
}

func TestCollaborationRuntimeScanIsolatesObjectFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"ledger", invalidCollaborationLedger("injected object-local mismatch")},
		{"retry_exhausted", collaborationError(http.StatusServiceUnavailable, "collaboration_busy")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, targets := newRuntimeScanLanes(t, 2)
			before := f.listRuns(t)
			if len(before) != 1 {
				t.Fatalf("expected an existing reserved candidate: %v", before)
			}
			bad := queueTarget(t, f.collaborationQueueFixture, targets[1])
			f.svc.TxStarter = &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationChannel", faults: map[pgtype.UUID]error{bad.ChannelID: tc.err}}
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 1 || list.RunIDs[0] != util.UUIDToString(before[0]) {
				t.Fatalf("one object's failure hid an independently prepared candidate: %+v %v", list, err)
			}
			if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, before[0], f.managedClaimRequest()); err != nil {
				t.Fatalf("the independent reserved candidate could not claim: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1 AND state='waiting'`, bad.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason IS NOT NULL`, bad.ID) != 0 {
				t.Fatal("object failure rewrote waiting state or hold")
			}
		})
	}
}

func TestCollaborationRuntimeWakeFailureDoesNotHideCandidate(t *testing.T) {
	f, runs, _ := newRuntimeScanMembers(t, 2)
	parent := runs[0]
	f.runAndFinish(t, parent)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE wake_parent_run_id=$1 AND wake_pending`, parent) != 1 {
		t.Fatal("fixture must retain a real pending squad wake")
	}
	f.svc.TxStarter = &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationRuntimeRun", faults: map[pgtype.UUID]error{parent: invalidCollaborationLedger("injected wake object mismatch")}}
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 1 || list.RunIDs[0] != util.UUIDToString(runs[1]) {
		t.Fatalf("one wake failure hid the existing member: %+v %v", list, err)
	}
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runs[1], f.managedClaimRequest()); err != nil {
		t.Fatalf("the independent member could not claim: %v", err)
	}
}

func TestCollaborationRuntimeWakeScanAdvancesPersistentFailures(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.resumeManaged(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	var targets, parents, groups []pgtype.UUID
	for range 18 {
		leader := f.fx.Agent(t, "Wake leader "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
		member := f.fx.Agent(t, "Wake member "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
		squad := f.fx.Squad(t, "Wake scan squad", leader)
		f.fx.SquadMember(t, squad, "agent", member)
		session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad,
			ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: session.TargetSetVersion, Roster: []collaboration.TargetResource{
				{Role: "leader", AgentID: leader, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
				{Role: "member", AgentID: member, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
			}})
		if err != nil {
			t.Fatal(err)
		}
		target := queueTarget(t, f.collaborationQueueFixture, registered.TargetID)
		consentManagedControlTarget(t, f, target, util.MustParseUUID(leader))
		consentManagedControlTarget(t, f, target, util.MustParseUUID(member))
		lane := *f
		base := *f.collaborationQueueFixture
		base.targetID = target.ID
		lane.collaborationQueueFixture = &base
		lane.submitManaged(t, f.creator, "wake scan instruction")
		if _, err = f.svc.ResumeQueue(f.ctx, managedTargetResume(t, &lane, target.ID)); err != nil {
			t.Fatal(err)
		}
		root, _, claim := lane.claimManaged(t)
		token, nonce := permitSquadRun(t, &lane, root, claim)
		var memberRuns []pgtype.UUID
		for range 2 {
			receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{
				CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + member + ") wake scan work",
			})
			if err != nil || len(receipt.RunIDs) != 1 {
				t.Fatalf("wake fixture dispatch: %+v %v", receipt, err)
			}
			memberRuns = append(memberRuns, receipt.RunIDs[0])
		}
		finishSquadRun(t, &lane, root, claim, token, nonce, "success")
		lane.runAndFinish(t, memberRuns[0])
		target = queueTarget(t, f.collaborationQueueFixture, target.ID)
		targets = append(targets, target.ID)
		parents = append(parents, memberRuns[0])
		groups = append(groups, target.ActiveGroupID)
		// Isolate fixture construction from earlier queued member work. All runs
		// that reserved the shared namespace have real settled server proofs.
		// Restore the unchanged grants/holds together before exercising the scan.
		f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='grant_required' WHERE id=$1`, target.ID)
	}
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL WHERE id=ANY($1::uuid[])`, targets)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1 AND released_at IS NULL`, f.runtime.ID) != 0 {
		t.Fatal("wake fixture has an occupied namespace")
	}
	fault := &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationRuntimeRun", faults: make(map[pgtype.UUID]error)}
	for _, parent := range parents[:17] {
		fault.faults[parent] = invalidCollaborationLedger("injected persistent wake failure")
	}
	for range 2 {
		svc := NewCollaborationService(db.New(f.pool), fault, f.svc.Issues)
		if _, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err != nil {
			t.Fatal(err)
		}
	}
	if len(fault.hits) != 17 || f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=ANY($1::uuid[]) AND wake_pending`, groups[:17]) != 17 {
		t.Fatal("wake scan dropped or did not reach a persistent failing object")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND NOT wake_pending`, groups[17]) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND step_kind='continuation' AND progress_state='prepared'`, groups[17]) != 1 {
		t.Fatal("the healthy wake after 17 persistent failures did not create its one continuation")
	}
}

func TestCollaborationRuntimePendingDerivedCheckIncludesOtherTiers(t *testing.T) {
	f, runs, _ := newRuntimeScanMembers(t, 18)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runs[0], f.managedClaimRequest()); err != nil {
		t.Fatal(err)
	}
	if offered := f.listRuns(t); len(offered) != 1 || offered[0] != runs[0] {
		t.Fatalf("expected the reserved tier only: %v", offered)
	}
	pending, err := f.svc.Queries.HasCollaborationRuntimePendingDerivedRun(f.ctx, db.HasCollaborationRuntimePendingDerivedRunParams{RuntimeID: f.runtime.ID, BindingID: f.binding.ID})
	if err != nil || !pending {
		t.Fatalf("a reserved page hid the pending derived runs in another tier: %v %v", pending, err)
	}
}

func TestCollaborationRuntimeScanPropagatesGlobalFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"daemon_identity", collaborationError(http.StatusForbidden, "managed_runtime_required")},
		{"database", errors.New("injected database connection unavailable")},
		{"context", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, targets := newRuntimeScanLanes(t, 2)
			f.listRuns(t)
			bad := queueTarget(t, f.collaborationQueueFixture, targets[1])
			f.svc.TxStarter = &runtimeScanFaultStarter{TxStarter: f.pool, query: "GetCollaborationChannel", faults: map[pgtype.UUID]error{bad.ChannelID: tc.err}}
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if !errors.Is(err, tc.err) || len(list.RunIDs) != 0 {
				t.Fatalf("global failure was hidden as an object refusal: %+v %v", list, err)
			}
		})
	}
}

func TestCollaborationRuntimeCandidateScanPriorityPrecedesCursor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index int
	}{
		{"reservation_before_cursor", 0},
		{"reservation_after_cursor", 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, runs, _ := newRuntimeScanMembers(t, 18)
			for range 16 {
				if first := f.listRuns(t); len(first) != 1 {
					t.Fatalf("expected one candidate per actual dispatch: %v", first)
				}
			}
			reserved := runs[tc.index]
			if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, reserved, f.managedClaimRequest()); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				svc := NewCollaborationService(db.New(f.pool), f.pool, f.svc.Issues)
				list, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
				if err != nil || len(list.RunIDs) != 1 || list.RunIDs[0] != util.UUIDToString(reserved) {
					t.Fatalf("a new reservation lost priority across cursor wrap/restart: %+v %v", list, err)
				}
			}
			other := runs[1]
			if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, other, f.managedClaimRequest()); !collaborationErrorCode(err, "runtime_busy") {
				t.Fatalf("lower priority member bypassed the namespace reservation: %v", err)
			}
		})
	}
}

func TestCollaborationRuntimeCandidateScanSharesCursorAcrossPollers(t *testing.T) {
	f, runs, _ := newRuntimeScanMembers(t, 18)
	start := make(chan struct{})
	type result struct {
		list protocol.CollaborationRunListResponse
		err  error
	}
	results := make(chan result, len(runs))
	for range len(runs) {
		go func() {
			<-start
			svc := NewCollaborationService(db.New(f.pool), f.pool, f.svc.Issues)
			list, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			results <- result{list, err}
		}()
	}
	close(start)
	seen := make(map[string]bool)
	for range len(runs) {
		got := <-results
		if got.err != nil || len(got.list.RunIDs) != 1 {
			t.Fatalf("concurrent scan failed: %+v %v", got.list, got.err)
		}
		for _, id := range got.list.RunIDs {
			if seen[id] {
				t.Fatalf("concurrent pollers replayed the same prefix before scanning the tail: %s", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != len(runs) {
		t.Fatalf("concurrent cursor lost candidate opportunities: got %d, want %d", len(seen), len(runs))
	}
}

func TestCollaborationRuntimeScanUnknownCommitPreservesOriginalRun(t *testing.T) {
	f, targets := newRuntimeScanLanes(t, 1)
	// Even an error carrying a normally local refusal must retain the distinct
	// unknown-commit boundary, rather than being treated as a rolled-back item.
	lost := collaborationError(http.StatusConflict, "stale_authority")
	fault := &runtimeScanFaultStarter{TxStarter: f.pool, commitQuery: "CreateCollaborationRuntimeRun", commitError: lost}
	svc := NewCollaborationService(db.New(f.pool), fault, f.svc.Issues)
	list, err := svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if !errors.Is(err, lost) || len(list.RunIDs) != 0 || fault.lostCommits != 1 {
		t.Fatalf("unknown promotion commit became a local refusal or was replayed: %+v %v commits=%d", list, err, fault.lostCommits)
	}
	var original pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM collaboration_run WHERE target_id=$1`, targets[0]).Scan(&original)
	for range 2 {
		recovered := f.listRuns(t)
		if len(recovered) != 1 || recovered[0] != original || f.groupCount(t, targets[0]) != 1 {
			t.Fatalf("poll recovery replaced an uncertain original run: %v", recovered)
		}
	}
}

func TestCollaborationRuntimeCandidateScanRechecksPriorityAfterCursorLock(t *testing.T) {
	f, runs, _ := newRuntimeScanMembers(t, 18)
	f.listRuns(t)
	blocker, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	q := f.svc.Queries.WithTx(blocker)
	if _, err = q.LockAgentRuntime(f.ctx, f.runtime.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = q.LockCollaborationRuntimeScanCursor(f.ctx, db.LockCollaborationRuntimeScanCursorParams{
		RuntimeID: f.runtime.ID, BindingID: f.binding.ID, ScanKind: collaborationScanUnreserved,
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.daemonCtx, 10*time.Second)
	defer cancel()
	type response struct {
		candidates []db.ListCollaborationRuntimeCandidatesRow
		err        error
	}
	done := make(chan response, 1)
	go func() {
		candidates, err := f.svc.scanRuntimeCandidates(ctx, f.runtime.ID, f.binding.ID)
		done <- response{candidates, err}
	}()
	// Wait for the actual lock boundary, not a guessed scheduling delay.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '-- name: LockAgentRuntime%')`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("poller did not reach the expected runtime fence before its cursor lock")
		case <-ticker.C:
		}
	}
	// Commit the same reservation primitive used by Claim while following
	// runtime -> cursor order. The blocked scanner has already observed the old
	// priority and must recheck it after acquiring these locks.
	run, err := q.GetCollaborationRuntimeRun(f.ctx, runs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = q.CreateCollaborationRuntimeReservation(f.ctx, db.CreateCollaborationRuntimeReservationParams{RunID: run.ID,
		ExecutionWorkspaceID: run.ExecutionWorkspaceID, AgentID: run.AgentID, RuntimeID: run.RuntimeID, DaemonID: f.runtime.DaemonID.String,
		InstallationID: run.InstallationID, BindingID: run.BindingID}); err != nil {
		t.Fatal(err)
	}
	if err = blocker.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil || len(got.candidates) != 0 {
		t.Fatalf("stale lower tier was returned after a reservation appeared: %+v %v", got.candidates, got.err)
	}
	cursor, err := f.svc.Queries.LockCollaborationRuntimeScanCursor(f.ctx, db.LockCollaborationRuntimeScanCursorParams{
		RuntimeID: f.runtime.ID, BindingID: f.binding.ID, ScanKind: collaborationScanUnreserved,
	})
	if err != nil || cursor.AfterID != runs[0] {
		t.Fatalf("an empty priority-race page erased the tier's previous progress: %+v %v", cursor, err)
	}
	if list := f.listRuns(t); len(list) != 1 || list[0] != runs[0] {
		t.Fatalf("the next selection did not discover the new reservation: %v", list)
	}
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runs[0], f.managedClaimRequest()); err != nil {
		t.Fatalf("the reserved candidate could not claim: %v", err)
	}
}

func TestCollaborationRuntimeScanCursorCleanup(t *testing.T) {
	for _, scope := range []string{"runtime", "workspace"} {
		t.Run(scope, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.listRuns(t)
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, f.runtime.ID) == 0 {
				t.Fatal("list must have persisted the scan positions")
			}
			otherWorkspace := f.fx.Workspace(t, "Unrelated scan workspace", "other-scan-"+uuid.NewString())
			other := util.MustParseUUID(f.fx.Runtime(t, "Other scan runtime", testutil.Cols{"workspace_id": otherWorkspace}))
			f.fx.InsertNoID(t, "collaboration_runtime_scan_cursor", testutil.Cols{"runtime_id": other, "binding_id": f.binding.ID, "scan_kind": "waiting"}, "runtime_id=$1", other)
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			q := f.svc.Queries.WithTx(tx)
			if _, err = TeardownRuntime(f.ctx, q, f.runtime.ID, RuntimeTeardownOptions{CancelNonTerminalTasks: true}); err != nil {
				t.Fatal(err)
			}
			if scope == "runtime" {
				err = q.DeleteAgentRuntime(f.ctx, f.runtime.ID)
			} else {
				err = q.DeleteWorkspaceRuntimesAndProjects(f.ctx, util.MustParseUUID(f.workspace))
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, f.runtime.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, other) != 1 {
				t.Fatal("runtime deletion left orphan scan positions or removed another runtime's progress")
			}
		})
	}
}
