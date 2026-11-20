package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueBudgetObservationOffResumeIsAtomicAndReplayed(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	root := f.enable(t, f.issue(t, "Root", ""))
	beforeEntries := issueBudgetRows(t, f.fx, "issue_budget_entry", f.workspace)
	for epoch := int64(2); epoch <= 3; epoch++ {
		off, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, int64(root.PolicyRevision)), IssueBudgetModeOff)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.count(t, "issue_budget_observation"); got != int(epoch-2) {
			t.Fatalf("off wrote an observation: %d", got)
		}
		resume := f.manage(root.BudgetRootID, int64(off.PolicyRevision))
		root, err = f.svc.SetMode(ctx, resume, IssueBudgetModeObserve)
		if err != nil {
			t.Fatal(err)
		}
		if int64(root.PolicyEpoch) != epoch {
			t.Fatalf("epoch = %d, want %d", root.PolicyEpoch, epoch)
		}
		replay, err := f.svc.SetMode(ctx, resume, IssueBudgetModeObserve)
		if err != nil || !replay.Replayed || replay.OperationID != root.OperationID {
			t.Fatalf("replay = %+v, %v", replay, err)
		}
		var gaps int
		f.fx.QueryRow(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1
		 AND kind='coverage_gap' AND reason='observation_off' AND detail->>'policy_epoch'=$2
		 AND issue_id IS NULL AND task_id IS NULL AND claim_generation IS NULL AND attempt_id IS NULL`, root.BudgetRootID, strconv.FormatInt(int64(root.PolicyEpoch), 10)).Scan(&gaps)
		if gaps != 1 {
			t.Fatalf("epoch %d has %d resume gaps", epoch, gaps)
		}
	}
	if got := issueBudgetRows(t, f.fx, "issue_budget_entry", f.workspace); got != beforeEntries {
		t.Fatal("mode changes rewrote ledger")
	}
	gaps, err := f.q.CountIssueBudgetUncoveredGaps(ctx, db.CountIssueBudgetUncoveredGapsParams{WorkspaceID: f.ws, BudgetRootID: util.MustParseUUID(root.BudgetRootID)})
	if err != nil || len(gaps) != 1 || gaps[0].Reason != "observation_off" || gaps[0].GapCount != 2 {
		t.Fatalf("uncovered gaps = %+v, %v", gaps, err)
	}

	// A later policy write failure rolls the newly inserted gap back too.
	if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, int64(root.PolicyRevision)), IssueBudgetModeOff); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE issue_budget_root SET policy_revision=9223372036854775807 WHERE id=$1`, root.BudgetRootID)
	before := f.snapshot(t)
	if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, 9223372036854775807), IssueBudgetModeObserve); err == nil {
		t.Fatal("expected policy revision overflow")
	}
	f.requireUnchanged(t, before, "failed resume")
}

func TestIssueBudgetOffBindingKeepsMembersWithoutObservations(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	parent := f.issue(t, "Root", "")
	root := f.enable(t, parent)
	otherParent := f.issue(t, "Other root", "")
	otherMember := f.issue(t, "Other member", otherParent)
	f.enable(t, otherParent)
	if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, 1), IssueBudgetModeOff); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	child := f.createChild(t, "Child while off", parent)
	f.requireMember(t, util.UUIDToString(child.ID), issueBudgetMemberRow{Root: root.BudgetRootID, BoundParent: parent, Source: "create", Version: 1})
	// A member moving across roots keeps its old root and writes no conflict.
	f.inBinding(t, func(q *db.Queries) error {
		return BindIssueBudgetOnReparent(ctx, q, f.setParent(t, util.UUIDToString(child.ID), otherParent))
	})
	// An unbound cycle includes a foreign member and a running task: none of
	// parent_cycle, root_conflict or in_flight_at_bind may be written while off.
	subtree := f.issue(t, "Unbound subtree", "")
	leaf := f.issue(t, "Leaf", subtree)
	f.setParent(t, otherMember, subtree)
	f.setParent(t, parent, leaf)
	agent, runtime := f.agent(t)
	f.task(t, agent, runtime, leaf, "running", 1)
	sent := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, f.setParent(t, subtree, parent)) })
	want := []string{"GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "GetIssueBudgetMember", "ListIssueBudgetDescendants", "LockIssueBudgetBinding", "GetIssueBudgetRoot", "InsertIssueBudgetMembers"}
	if !reflect.DeepEqual(sent, want) {
		t.Fatalf("off binding sent %v, want %v", sent, want)
	}
	f.requireMember(t, subtree, issueBudgetMemberRow{Root: root.BudgetRootID, BoundParent: parent, Source: "reparent", Version: 1})
	f.requireMember(t, leaf, issueBudgetMemberRow{Root: root.BudgetRootID, BoundParent: subtree, Source: "reparent", Version: 1})
	for _, table := range []string{"issue_budget_root", "issue_budget_attempt", "issue_budget_entry", "issue_budget_operation", "issue_budget_observation"} {
		if after := issueBudgetRows(t, f.fx, table, f.workspace); after != before[table] {
			t.Fatalf("off issue writer changed %s", table)
		}
	}
}

func TestIssueBudgetOffBindingInterleavesWithResume(t *testing.T) {
	for _, resumeBeforeRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume_after_off_read", true: "resume_while_binding_waits"}[resumeBeforeRead], func(t *testing.T) {
			f := newIssueBudgetFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			parent := f.issue(t, "Root", "")
			root := f.enable(t, parent)
			if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, 1), IssueBudgetModeOff); err != nil {
				t.Fatal(err)
			}
			moved := f.issue(t, "Moved issue", "")
			agent, runtime := f.agent(t)
			f.task(t, agent, runtime, moved, "running", 1)
			row := f.setParent(t, moved, parent)
			resume := func() {
				if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, 2), IssueBudgetModeObserve); err != nil {
					t.Fatal(err)
				}
			}
			if !resumeBeforeRead {
				f.inBindingHeld(t, func(q *db.Queries) error { return BindIssueBudgetOnReparent(ctx, q, row) }, func(uint32) { resume() })
			} else {
				holder, err := f.pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer holder.Rollback(context.Background())
				if ok, err := db.New(holder).TryLockIssueBudgetBindingShared(ctx, f.ws); err != nil || !ok {
					t.Fatalf("shared binding lock = %v, %v", ok, err)
				}
				log := &issueBudgetStatementLog{}
				done := make(chan error, 1)
				finished := make(chan struct{})
				go func() {
					defer close(finished)
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						done <- err
						return
					}
					defer tx.Rollback(context.Background())
					if err = iteration.LockWorkspace(ctx, tx, f.ws); err == nil {
						err = BindIssueBudgetOnReparent(ctx, db.New(issueBudgetLoggedTx{Tx: tx, log: log}), row)
					}
					if err == nil {
						err = tx.Commit(ctx)
					}
					done <- err
				}()
				f.awaitWriter(t, finished)
				f.blockedBy(t, holder.Conn().PgConn().PID())
				if names := log.names(); names[len(names)-1] != "LockIssueBudgetBinding" {
					t.Fatalf("binder did not wait at binding lock: %v", names)
				}
				resume()
				if err := holder.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(strings.Join(log.names(), ","), "LockIssueBudgetBinding,GetIssueBudgetRoot,InsertIssueBudgetMembers") {
					t.Fatalf("mode read precedes lock: %v", log.names())
				}
			}
			gaps, err := f.q.CountIssueBudgetUncoveredGaps(ctx, db.CountIssueBudgetUncoveredGapsParams{WorkspaceID: f.ws, BudgetRootID: util.MustParseUUID(root.BudgetRootID)})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int64{}
			for _, gap := range gaps {
				counts[gap.Reason] = gap.GapCount
			}
			wantFlight := int64(0)
			if resumeBeforeRead {
				wantFlight = 1
			}
			if counts["observation_off"] != 1 || counts["in_flight_at_bind"] != wantFlight {
				t.Fatalf("gaps = %v", counts)
			}
			f.requireMember(t, moved, issueBudgetMemberRow{Root: root.BudgetRootID, BoundParent: parent, Source: "reparent", Version: 1})
		})
	}
}

func (f *issueBudgetFixture) createWithTaskIdentity(t *testing.T, parent string, task pgtype.UUID) (db.Issue, []string) {
	t.Helper()
	ctx := context.Background()
	f.syncIssueCounter(t)
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	log := &issueBudgetStatementLog{}
	p := IssueCreateParams{WorkspaceID: f.ws, Title: uuid.NewString(), Status: "todo", Priority: "none", CreatorType: "member", CreatorID: f.actor, VerifiedTaskID: task}
	if parent != "" {
		p.ParentIssueID = util.MustParseUUID(parent)
	}
	created, err := f.issues.CreateInTx(ctx, issueBudgetLoggedTx{Tx: tx, log: log}, p, IssueCountPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return created.Issue, log.names()
}

func TestIssueBudgetQuickCreateRecordsEachDifferentRoot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	root, parent, child := f.budgetTree(t, 1000, 100)
	otherParent := f.issue(t, "Other root", "")
	other := f.enable(t, otherParent)
	rootless := f.issue(t, "Rootless", "")
	agent, runtime := f.agent(t)
	task := f.claimedQuickCreate(t, agent, runtime, parent, 1)
	f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
	task = f.claimedAgain(t, task)
	latest := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
	// Latest does not mean latest open: later lifecycle/usage must not make
	// attribution silently select an earlier attempt.
	f.fx.Exec(t, `UPDATE issue_budget_attempt SET state='settled' WHERE id=$1`, latest)
	before := f.snapshot(t)
	if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1 AND kind='attribution_difference'`, root.BudgetRootID); n != 0 {
		t.Fatalf("zero creations wrote %d differences", n)
	}
	for i, target := range []string{otherParent, "", rootless} {
		created, names := f.createWithTaskIdentity(t, target, task.ID)
		if strings.Contains(strings.Join(names, ","), "LockIssueBudgetRoot") {
			t.Fatal("create locked a root")
		}
		var gotTask, gotAttempt, gotRoot, gotReason string
		var generation int64
		var detail []byte
		f.fx.QueryRow(t, `SELECT task_id::text, attempt_id::text, budget_root_id::text, claim_generation, reason, detail FROM issue_budget_observation
		 WHERE issue_id=$1 AND kind='attribution_difference'`, created.ID).Scan(&gotTask, &gotAttempt, &gotRoot, &generation, &gotReason, &detail)
		if gotTask != util.UUIDToString(task.ID) || gotAttempt != latest || gotRoot != root.BudgetRootID || generation != 2 || gotReason != "quick_create_root_mismatch" {
			t.Fatalf("wrong attribution: %s %s %s %d %s", gotTask, gotAttempt, gotRoot, generation, gotReason)
		}
		var fields map[string]string
		if err := json.Unmarshal(detail, &fields); err != nil {
			t.Fatal(err)
		}
		wantRoot := ""
		if target == otherParent {
			wantRoot = other.BudgetRootID
		}
		if fields["created_issue_budget_root_id"] != wantRoot || fields["context_parent_issue_id"] != parent {
			t.Fatalf("detail = %v", fields)
		}
		if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1 AND kind='attribution_difference'`, root.BudgetRootID); n != i+1 {
			t.Fatalf("after %d creations got %d differences", i+1, n)
		}
	}
	f.createWithTaskIdentity(t, child, task.ID) // same root: no difference
	for _, table := range []string{"issue_budget_root", "issue_budget_attempt", "issue_budget_entry", "issue_budget_operation"} {
		if got := issueBudgetRows(t, f.fx, table, f.workspace); got != before[table] {
			t.Fatalf("issue creation changed %s", table)
		}
	}
	ordinary := f.claimed(t, agent, runtime, child, 1)
	f.mustFinalize(t, f.claims(f.pool), ordinary, TaskClaimBudgetProtocolV1)
	f.createWithTaskIdentity(t, otherParent, ordinary.ID)
	_, humanNames := f.createWithTaskIdentity(t, "", pgtype.UUID{})
	for _, name := range humanNames {
		if name == "GetLatestIssueBudgetAttemptForTask" {
			t.Fatal("human create queried attempts")
		}
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE budget_root_id=$1 AND kind='attribution_difference'`, root.BudgetRootID); n != 3 {
		t.Fatalf("same-root/ordinary/human create changed differences: %d", n)
	}
	// Before creating the ordinary claim, creation only appended members and
	// observations: original quick-create attempts retain their original root.
	var changed int
	f.fx.QueryRow(t, `SELECT count(*) FROM issue_budget_attempt WHERE task_id=$1 AND budget_root_id<>$2`, task.ID, root.BudgetRootID).Scan(&changed)
	if changed != 0 {
		t.Fatal("creation reassigned spend")
	}
	if _, err := f.svc.SetMode(ctx, f.manage(root.BudgetRootID, 1), IssueBudgetModeOff); err != nil {
		t.Fatal(err)
	}
	off := f.snapshot(t)
	f.createWithTaskIdentity(t, otherParent, task.ID)
	for _, table := range []string{"issue_budget_root", "issue_budget_attempt", "issue_budget_entry", "issue_budget_operation", "issue_budget_observation"} {
		if got := issueBudgetRows(t, f.fx, table, f.workspace); got != off[table] {
			t.Fatalf("off quick-create changed %s", table)
		}
	}
}

// Keep this guard independent of handler headers and request origins: even a
// populated provenance field cannot supply the verified execution parameter.
func TestIssueBudgetQuickCreateIgnoresSelfReportedOrigin(t *testing.T) {
	f := newIssueBudgetFixture(t)
	_, parent, _ := f.budgetTree(t, 1000, 100)
	agent, runtime := f.agent(t)
	task := f.claimedQuickCreate(t, agent, runtime, parent, 1)
	f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
	issue := f.issue(t, "A caller's claimed origin", "", testutil.Cols{"origin_type": "quick_create", "origin_id": util.UUIDToString(task.ID)})
	row, err := f.q.GetIssueInWorkspace(context.Background(), db.GetIssueInWorkspaceParams{WorkspaceID: f.ws, ID: util.MustParseUUID(issue)})
	if err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	names := f.inBinding(t, func(q *db.Queries) error { return BindIssueBudgetOnCreate(context.Background(), q, row, pgtype.UUID{}) })
	if len(names) != 0 {
		t.Fatalf("self-reported origin sent %v", names)
	}
	f.requireUnchanged(t, before, "self-reported origin without verified task")
}
