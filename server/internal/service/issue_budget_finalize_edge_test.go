package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The claims off the ordinary path of a finalization: a ledger that cannot be
// loaded (design 10.5), a quick-create task, a task run from another
// workspace, a reserve that does not fit in the ledger, a transaction the
// budget step cannot judge in, a service without a transaction, a commit that
// does not go through, and a ledger served from the process cache.

// F09 (B47, second half; design 10.5). A ledger that cannot be loaded does
// not hold the execution back and is not passed over in silence. For each of
// the four causes the claim appends nothing and leaves the root row as it is;
// it gets its attempt, without a reservation, a coverage gap that says the
// ledger was not readable, and a receipt that says so too. What does not need
// the ledger is judged as usual: root_closed, root_deleted, and on a root
// with a limit unbounded_unknown, which names the ledger itself. The limit is
// not judged: there is no amount to judge it on. Earlier open attempts of the
// task stay open, because whether one has a reservation to hold cannot be
// asked.
//
// Once the rows are repaired the next claim is back on the normal path. It
// closes both earlier attempts; the one written without a reservation gets
// the state change and no hold.
func TestIssueBudgetFinalizeOnALedgerThatCannotBeLoaded(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	damages := map[string]func(t *testing.T, r *issueBudgetLedgerRoot){
		// A second row in a form version this build does not read.
		"stored_form": func(t *testing.T, r *issueBudgetLedgerRoot) {
			e := issueBudgetReserveEntry(uuid.NewString(), 300)
			raw, _, err := budget.EncodeEntry(e)
			if err != nil {
				t.Fatal(err)
			}
			r.insertForm(t, 2, e.ID, e, raw, 2)
			r.set(t, 2, issueBudgetLedgerFigures{ReservedTicks: 400})
		},
		// A hold on a reservation no entry opened.
		"replay": func(t *testing.T, r *issueBudgetLedgerRoot) {
			r.insert(t, 2, issueBudgetHoldEntry(uuid.NewString(), budget.HoldTimeout))
			r.set(t, 2, issueBudgetLedgerFigures{ReservedTicks: 100})
		},
		// The root row is at revision 3 and there is no row at seq 2.
		"sequence": func(t *testing.T, r *issueBudgetLedgerRoot) {
			r.insert(t, 3, issueBudgetReserveEntry(uuid.NewString(), 300))
			r.set(t, 3, issueBudgetLedgerFigures{ReservedTicks: 400})
		},
		// The figures on the root row are not what its entries add up to.
		"projection": func(t *testing.T, r *issueBudgetLedgerRoot) {
			r.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 101})
		},
	}
	for _, tc := range []struct {
		name  string
		cause string
		limit budget.Ticks
		// gone: the root is closed and its issue deleted before the claim that
		// cannot read the ledger.
		gone bool
	}{
		{name: "an entry that does not decode", cause: "stored_form", limit: 50},
		{name: "a replay that fails", cause: "replay", limit: 50},
		{name: "a gap in the sequence", cause: "sequence", limit: 50},
		{name: "root figures that do not match the replay", cause: "projection", limit: 50},
		{name: "a replay that fails on a root that is closed and whose issue was deleted", cause: "replay", limit: 50, gone: true},
		{name: "an entry that does not decode on a root without a limit", cause: "stored_form"},
		{name: "root figures that do not match the replay on a root without a limit, closed, its issue deleted", cause: "projection", gone: true},
	} {
		policy, parent, child := f.budgetTree(t, tc.limit, 100)
		root := policy.BudgetRootID
		ledger := &issueBudgetLedgerRoot{f: f, id: util.MustParseUUID(root)}
		limited := tc.limit != 0

		first := f.claimed(t, agent, runtime, child, 1)
		a1 := f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)
		var observed []issueBudgetClaimObservation
		if limited {
			observed = append(observed, issueBudgetObservationOfClaim("would_refuse", "limit", child, first, a1, map[string]string{
				"actual_known_ticks": "0", "reserved_ticks": "100", "unknown_hold_ticks": "0",
				"limit_ticks": "50", "this_reservation": "sized", "this_reservation_ticks": "100",
			}))
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observed, tc.name+": the claim on the readable ledger")
		firstPolicy := policy
		var gone []string
		if tc.gone {
			closed, err := f.svc.Close(ctx, f.manage(root, 1))
			if err != nil {
				t.Fatalf("%s: close the root: %v", tc.name, err)
			}
			policy = closed
			f.deleteIssue(t, parent)
			gone = []string{"root_closed", "root_deleted"}
		}
		if lines := logs(); len(lines) != 0 {
			t.Fatalf("%s: the claim on the readable ledger logged %q", tc.name, lines)
		}

		damages[tc.cause](t, ledger)
		before := f.snapshot(t)
		second := f.claimedAgain(t, first)
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		a2 := f.mustFinalize(t, f.claims(log), second, TaskClaimBudgetProtocolV1)
		if a2 == "" || a2 == a1 {
			t.Fatalf("%s: the claim that cannot read the ledger got attempt %q", tc.name, a2)
		}
		after := f.snapshot(t)
		for _, table := range []string{"issue_budget_root", "issue_budget_entry", "issue_budget_member"} {
			if before[table] != after[table] {
				t.Fatalf("%s: the claim that cannot read the ledger changed %s\nbefore: %s\nafter:  %s", tc.name, table, before[table], after[table])
			}
		}
		rows := f.attempts(t, first)
		if len(rows) != 2 {
			t.Fatalf("%s: two claims left %d attempt rows, want 2", tc.name, len(rows))
		}
		// The earlier attempt is left as it is.
		f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: firstPolicy, Task: first, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, tc.name+": the earlier attempt")
		f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: policy, Task: second, Issue: child, State: "open", Protocol: issueBudgetInt32(1)}, tc.name+": the attempt without a reservation")

		observed = append(observed, issueBudgetObservationOfClaim("coverage_gap", "ledger_unreadable", child, second, a2, nil))
		refuse := append([]string{}, gone...)
		for _, reason := range gone {
			observed = append(observed, issueBudgetObservationOfClaim("would_refuse", reason, child, second, a2, nil))
		}
		tail := []string{"InsertIssueBudgetAttempt", "InsertIssueBudgetObservation"}
		for range gone[:len(gone)/2] {
			tail = append(tail, "InsertIssueBudgetObservation") // root_closed
		}
		tail = append(tail, "GetIssueBudgetMember")
		for range gone[:len(gone)/2] {
			tail = append(tail, "InsertIssueBudgetObservation") // root_deleted
		}
		if limited {
			observed = append(observed, issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, second, a2,
				map[string]string{"ledger": "unavailable", "observation.ledger_unreadable": "1"}))
			refuse = append(refuse, "unbounded_unknown")
			tail = append(tail, "CountIssueBudgetUncoveredGaps", "InsertIssueBudgetObservation")
		}
		tail = append(tail, "InsertIssueBudgetOperation")
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observed, tc.name+": the claim that cannot read the ledger")
		operations := f.claimOperations(t, root)
		if len(operations) != 2 {
			t.Fatalf("%s: two claims left %d operation rows, want 2", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
			Root: root, Task: second, Attempt: a2, Reservation: "none", LedgerUnavailable: true, WouldRefuse: refuse,
		}, tc.name+": the claim that cannot read the ledger")
		if got := f.tokens(t, second); got != 2 {
			t.Fatalf("%s: two finalized claims of the task left %d task tokens, want 2", tc.name, got)
		}

		// The statements: the lock and the load as ever, then no list of
		// earlier attempts, no close, no entry and no root update; the attempt
		// row, the gap, and what is judged without the ledger.
		step := issueBudgetStep(t, log.names(), tc.name)
		head := []string{"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation", "LockIssueBudgetRoot"}
		if len(step) < len(head)+len(tail) || !reflect.DeepEqual(step[:len(head)], head) || !reflect.DeepEqual(step[len(step)-len(tail):], tail) {
			t.Fatalf("%s: the budget step sent\n%s\nwant\n%s\nthen the load, then\n%s", tc.name, strings.Join(step, "\n"), strings.Join(head, "\n"), strings.Join(tail, "\n"))
		}
		for _, name := range step[len(head) : len(step)-len(tail)] {
			if name != "ListIssueBudgetEntryFormsAfter" {
				t.Fatalf("%s: between the root lock and the attempt row the step sent %s; only the load belongs there:\n%s", tc.name, name, strings.Join(step, "\n"))
			}
		}

		// One error line that names the claim and the cause the test built.
		var errorLines []string
		for _, line := range logs() {
			if strings.Contains(line, "level=ERROR") {
				errorLines = append(errorLines, line)
			}
		}
		requireIssueBudgetLogLine(t, errorLines, "ERROR", []string{
			"budget_root_id=" + root, "attempt_id=" + a2, "task_id=" + util.UUIDToString(second.ID), "claim_generation=2",
		}, tc.name)
		if !strings.Contains(errorLines[0], ": "+tc.cause+": ") {
			t.Fatalf("%s: premise: the ledger was to be unavailable for cause %q: %s", tc.name, tc.cause, errorLines[0])
		}

		// Repaired: the rows and the figures of the one good entry.
		f.fx.Exec(t, `DELETE FROM issue_budget_entry WHERE budget_root_id=$1 AND seq>1`, root)
		ledger.set(t, 1, issueBudgetLedgerFigures{ReservedTicks: 100})
		third := f.claimedAgain(t, second)
		a3 := f.mustFinalize(t, f.claims(f.pool), third, TaskClaimBudgetProtocolV1)
		rows = f.attempts(t, first)
		if len(rows) != 3 {
			t.Fatalf("%s: three claims left %d attempt rows, want 3", tc.name, len(rows))
		}
		f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: firstPolicy, Task: first, Issue: child, State: "superseded_unproven", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, tc.name+": the first attempt after the repair")
		f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: policy, Task: second, Issue: child, State: "superseded_unproven", Protocol: issueBudgetInt32(1)}, tc.name+": the attempt without a reservation after the repair")
		f.requireAttempt(t, rows[2], a3, issueBudgetWantAttempt{Root: policy, Task: third, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, tc.name+": the attempt after the repair")
		operations = f.claimOperations(t, root)
		if len(operations) != 3 {
			t.Fatalf("%s: three claims left %d operation rows, want 3", tc.name, len(operations))
		}
		entries := f.entryRows(t, root)
		if len(entries) != 3 {
			t.Fatalf("%s: after the repair the root has %d entry rows, want the first reservation, one hold and the new reservation", tc.name, len(entries))
		}
		// One hold, for the attempt that has a reservation in the ledger.
		f.requireClaimEntry(t, entries[1], 2, issueBudgetHoldEntry(a1, "superseded_unproven"),
			budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "superseded_unproven"}, operations[2].ID, tc.name+": the hold after the repair")
		f.requireClaimEntry(t, entries[2], 3, issueBudgetReserveEntry(a3, 100),
			budget.Receipt{Disposition: budget.DispositionAccepted}, operations[2].ID, tc.name+": the reservation after the repair")
		f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, tc.name+": after the repair")

		refuse = append([]string{}, gone...)
		for _, reason := range gone {
			observed = append(observed, issueBudgetObservationOfClaim("would_refuse", reason, child, third, a3, nil))
		}
		if limited {
			// The gap of the claim that could not read the ledger is never
			// covered: it stays one of the things the root does not know.
			observed = append(observed,
				issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, third, a3,
					map[string]string{"ledger.superseded_unproven": "1", "observation.ledger_unreadable": "1"}),
				issueBudgetObservationOfClaim("would_refuse", "limit", child, third, a3, map[string]string{
					"actual_known_ticks": "0", "reserved_ticks": "100", "unknown_hold_ticks": "100",
					"limit_ticks": "50", "this_reservation": "sized", "this_reservation_ticks": "100",
				}))
			refuse = append(refuse, "unbounded_unknown", "limit")
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observed, tc.name+": the claim after the repair")
		f.requireClaimOperation(t, operations[2], issueBudgetWantClaim{
			Root: root, Task: third, Attempt: a3, Reservation: "sized", Reserved: "100",
			Superseded: []string{a1, a2}, WouldRefuse: refuse,
		}, tc.name+": the claim after the repair")
		if lines := logs(); len(lines) != 0 {
			t.Fatalf("%s: the claim after the repair logged %q", tc.name, lines)
		}
	}
}

// claimedWithContext is a claimed task without an issue whose saved context is
// the quick-create context given, as it stands.
func (f *issueBudgetFixture) claimedWithContext(t *testing.T, agent, runtime string, qc QuickCreateContext, generation int64) db.AgentTaskQueue {
	t.Helper()
	encoded, err := json.Marshal(qc)
	if err != nil {
		t.Fatal(err)
	}
	return f.taskRow(t, f.fx.Task(t, agent, testutil.Cols{
		"runtime_id": runtime, "status": "dispatched", "claim_generation": generation,
		"dispatched_at": testutil.Raw("now()"), "context": string(encoded),
	}))
}

// F11 (B55, finalize part; 3.1). A quick-create task has no issue. Its claim
// is attributed through the parent issue its saved context names, in the
// workspace the context names: the attempt row carries the context parent and
// no issue, on the root the parent is a member of.
//
// The parent can be deleted while the task waits; its member row stays,
// marked. The claim is still attributed. A deleted root issue is root_deleted,
// as for any claim on that root. A deleted member below the root is not a
// reason to refuse: it is recorded as a difference in attribution and is not
// in the receipt's would_refuse list.
func TestIssueBudgetFinalizeAttributesAQuickCreateTaskThroughItsContextParent(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	const (
		rootIssue = "the root issue"
		below     = "a member below the root"
	)
	type observed struct{ kind, reason string }
	for _, tc := range []struct {
		name string
		// through is the issue the context names.
		through string
		// deleted are the issues deleted before the claim.
		deleted      []string
		observations []observed
		refuse       []string
		// judged are the statements between the attempt row and the count of
		// the gaps.
		judged []string
	}{
		{
			name: "the context names a member below the root", through: below,
			judged: []string{"GetIssueBudgetMember"},
		},
		{
			// The claim's member row is the root issue's: it is not read a
			// second time.
			name: "the context names the root issue", through: rootIssue,
		},
		{
			name: "the context names the root issue, which was deleted", through: rootIssue, deleted: []string{rootIssue},
			observations: []observed{{"would_refuse", "root_deleted"}}, refuse: []string{"root_deleted"},
			judged: []string{"InsertIssueBudgetObservation"},
		},
		{
			name: "the context names a member below the root, which was deleted", through: below, deleted: []string{below},
			observations: []observed{{"attribution_difference", "member_deleted"}},
			judged:       []string{"GetIssueBudgetMember", "InsertIssueBudgetObservation"},
		},
		{
			name: "the context names a deleted member below a root issue that was deleted too", through: below, deleted: []string{below, rootIssue},
			observations: []observed{{"would_refuse", "root_deleted"}, {"attribution_difference", "member_deleted"}}, refuse: []string{"root_deleted"},
			judged: []string{"GetIssueBudgetMember", "InsertIssueBudgetObservation", "InsertIssueBudgetObservation"},
		},
		{
			// The root issue is gone and the context parent is not: nothing
			// is said about the parent.
			name: "the context names a member below a root issue that was deleted", through: below, deleted: []string{rootIssue},
			observations: []observed{{"would_refuse", "root_deleted"}}, refuse: []string{"root_deleted"},
			judged: []string{"GetIssueBudgetMember", "InsertIssueBudgetObservation"},
		},
	} {
		enabled, parent, child := f.budgetTree(t, 1000, 100)
		root := enabled.BudgetRootID
		issues := map[string]string{rootIssue: parent, below: child}
		through := issues[tc.through]
		task := f.claimedQuickCreate(t, agent, runtime, through, 1)
		if task.IssueID.Valid {
			t.Fatalf("%s: premise: the quick-create task has issue %s", tc.name, util.UUIDToString(task.IssueID))
		}
		for _, name := range tc.deleted {
			f.deleteIssue(t, issues[name])
		}

		log := &issueBudgetStatementLog{TxStarter: f.pool}
		attempt := f.mustFinalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
		f.requireAttempt(t, f.onlyAttempt(t, task, attempt, tc.name), attempt, issueBudgetWantAttempt{
			Root: enabled, Task: task, ContextParent: through, State: "open",
			Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
		}, tc.name)
		var want []issueBudgetClaimObservation
		for _, o := range tc.observations {
			// An observation of the claim names the issue it was attributed
			// through, and has an empty detail here.
			want = append(want, issueBudgetObservationOfClaim(o.kind, o.reason, through, task, attempt, nil))
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), want, tc.name)
		operations := f.claimOperations(t, root)
		if len(operations) != 1 {
			t.Fatalf("%s: the claim left %d operation rows, want 1", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
			Root: root, Task: task, Attempt: attempt, Reservation: "sized", Reserved: "100", WouldRefuse: tc.refuse,
		}, tc.name)
		entries := f.entryRows(t, root)
		if len(entries) != 1 {
			t.Fatalf("%s: the claim left %d entry rows, want 1", tc.name, len(entries))
		}
		f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(attempt, 100),
			budget.Receipt{Disposition: budget.DispositionAccepted}, operations[0].ID, tc.name)
		f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, tc.name)

		// The try lock of a quick-create task is keyed by the workspace of its
		// context: there is no issue row to read it from.
		step := []string{
			"TryLockIssueBudgetBindingShared",
			"GetIssueBudgetMember",
			"GetIssueBudgetTransactionIsolation",
			"LockIssueBudgetRoot",
			"ListIssueBudgetOpenAttemptsOfTask",
			"InsertIssueBudgetEntry",
			"UpdateIssueBudgetRootLedger",
			"InsertIssueBudgetAttempt",
		}
		step = append(step, tc.judged...)
		step = append(step, "CountIssueBudgetUncoveredGaps", "InsertIssueBudgetOperation")
		requireIssueBudgetStep(t, log.names(), step, tc.name)
	}
}

// F11, the contexts that name no member. A quick-create task without a
// context parent costs the budget step nothing; one whose parent is in no
// tree costs it the try lock and the member row; one whose ids do not parse
// names no member row, which is logged once as a warning (3.3). None writes a
// budget row, and each is finalized.
func TestIssueBudgetFinalizeLeavesAQuickCreateTaskThatNamesNoMemberAlone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	_, _, member := f.budgetTree(t, 1000, 100)
	outside := f.issue(t, "Outside every root", "")
	agent, runtime := f.agent(t)
	before := f.snapshot(t)
	logs := issueBudgetClaimLog(t)
	context := func(workspace, parent string) QuickCreateContext {
		return QuickCreateContext{Type: QuickCreateContextType, Prompt: "file a sub-issue", RequesterID: f.user, WorkspaceID: workspace, ParentIssueID: parent}
	}

	for _, tc := range []struct {
		name    string
		context QuickCreateContext
		step    []string
		// warning lists the fields of the one warning; nil is no log line.
		warning []string
	}{
		{name: "no context parent", context: context(f.workspace, "")},
		{
			name: "a context parent outside every root", context: context(f.workspace, outside),
			step: []string{"TryLockIssueBudgetBindingShared", "GetIssueBudgetMember"},
		},
		{
			// The parent would be a member; the workspace that would find its
			// row is not an id.
			name: "a workspace id that does not parse", context: context("not-a-workspace", member),
			warning: []string{"parent_issue_id_valid=true", "workspace_id_valid=false"},
		},
		{
			name: "a parent id that does not parse", context: context(f.workspace, "not-an-issue"),
			warning: []string{"parent_issue_id_valid=false", "workspace_id_valid=true"},
		},
	} {
		task := f.claimedWithContext(t, agent, runtime, tc.context, 1)
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolNone)
		if err != nil {
			t.Fatalf("%s: finalize: %v", tc.name, err)
		}
		if finalized.BudgetAttemptID.Valid {
			t.Fatalf("%s: the finalization handed back attempt %s", tc.name, util.UUIDToString(finalized.BudgetAttemptID))
		}
		f.requireUnchanged(t, before, tc.name)
		if got := f.tokens(t, task); got != 1 {
			t.Fatalf("%s: the claim left %d task tokens, want 1", tc.name, got)
		}
		requireIssueBudgetStep(t, log.names(), tc.step, tc.name)
		lines := logs()
		if tc.warning == nil {
			if len(lines) != 0 {
				t.Fatalf("%s: the claim logged %q", tc.name, lines)
			}
			continue
		}
		requireIssueBudgetLogLine(t, lines, "WARN", append([]string{"task_id=" + util.UUIDToString(task.ID)}, tc.warning...), tc.name)
	}
}

// issueBudgetHugeReserve is a declared reserve of more than half of what the
// ledger can count: a second one beside it takes the root's commitment out of
// range.
const issueBudgetHugeReserve = 5_000_000_000_000_000_000

// requireIssueBudgetOverflowLog fails unless lines is the one error a claim
// logs when its reserve does not fit. The reserve it names is the one of the
// root, which in every test that overflows is issueBudgetHugeReserve.
func requireIssueBudgetOverflowLog(t *testing.T, lines []string, root, attempt string, task db.AgentTaskQueue, what string) {
	t.Helper()
	requireIssueBudgetLogLine(t, lines, "ERROR", []string{
		"budget_root_id=" + root, "attempt_id=" + attempt, "task_id=" + util.UUIDToString(task.ID),
		fmt.Sprintf("claim_generation=%d", task.ClaimGeneration),
		fmt.Sprintf("attempt_reserve_ticks=%d", int64(issueBudgetHugeReserve)),
	}, what)
}

// F20 (3.2). A reservation is never released in this slice, so what a root
// has reserved only grows, and a root may declare any reserve. A claim whose
// sized reserve would take known spend, reservations and holds out of range
// must not fail: a root that only observes does not hold an execution back.
// It reserves without an amount, the attempt has no reserved amount, the
// receipt says "unsized", and one coverage gap records the reserve that did
// not fit. The gap is written in front of what is judged, so the root's
// unknown set counts it, and the limit is judged with a reservation of 0.
func TestIssueBudgetFinalizeReservesWithoutAnAmountWhenItsReserveDoesNotFit(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, issueBudgetHugeReserve)
	root := enabled.BudgetRootID
	logs := issueBudgetClaimLog(t)
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	unsized := budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "reservation_unsized"}
	overflow := map[string]string{"attempt_reserve_ticks": "5000000000000000000"}

	// The first claim fits.
	agent, runtime := f.agent(t)
	first := f.claimed(t, agent, runtime, child, 1)
	a1 := f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, first, a1, "the claim that fits"), a1, issueBudgetWantAttempt{
		Root: enabled, Task: first, Issue: child, State: "open", Reserved: issueBudgetInt64(issueBudgetHugeReserve), Protocol: issueBudgetInt32(1),
	}, "the claim that fits")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: issueBudgetHugeReserve}, "after the claim that fits")
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("the claim that fits logged %q", lines)
	}

	// The second does not: 5e18 + 5e18 is more than an int64 holds.
	agent, runtime = f.agent(t)
	second := f.claimed(t, agent, runtime, child, 7)
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	finalized, err := f.finalize(t, f.claims(log), second, TaskClaimBudgetProtocolV1)
	if err != nil {
		t.Fatalf("finalize a claim whose reserve does not fit: %v (the ledger's overflow: %t)", err, errors.Is(err, budget.ErrOverflow))
	}
	a2 := util.UUIDToString(finalized.BudgetAttemptID)
	f.requireAttempt(t, f.onlyAttempt(t, second, a2, "the claim whose reserve does not fit"), a2, issueBudgetWantAttempt{
		Root: enabled, Task: second, Issue: child, State: "open", Protocol: issueBudgetInt32(1),
	}, "the claim whose reserve does not fit")
	requireIssueBudgetOverflowLog(t, logs(), root, a2, second, "the claim whose reserve does not fit")
	// The sized batch was refused before a statement was sent: there is one
	// entry insert, the one of the reservation without an amount. The gap
	// follows the attempt row; the judgements follow the gap.
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfTask",
		"InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
		"InsertIssueBudgetAttempt",
		"InsertIssueBudgetObservation",
		"GetIssueBudgetMember",
		"CountIssueBudgetUncoveredGaps",
		"InsertIssueBudgetObservation",
		"InsertIssueBudgetObservation",
		"InsertIssueBudgetOperation",
	}, "the claim whose reserve does not fit")

	// A third, from a daemon that declares no protocol: its two gaps in the
	// order legacy_protocol, reservation_overflow.
	agent, runtime = f.agent(t)
	third := f.claimed(t, agent, runtime, child, 1)
	a3 := f.mustFinalize(t, f.claims(f.pool), third, TaskClaimBudgetProtocolNone)
	f.requireAttempt(t, f.onlyAttempt(t, third, a3, "the legacy claim whose reserve does not fit"), a3, issueBudgetWantAttempt{
		Root: enabled, Task: third, Issue: child, State: "open",
	}, "the legacy claim whose reserve does not fit")
	requireIssueBudgetOverflowLog(t, logs(), root, a3, third, "the legacy claim whose reserve does not fit")

	limit := func(thisReservation, ticks string) map[string]string {
		return map[string]string{
			"actual_known_ticks": "0", "reserved_ticks": "5000000000000000000", "unknown_hold_ticks": "0",
			"limit_ticks": "1000", "this_reservation": thisReservation, "this_reservation_ticks": ticks,
		}
	}
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), []issueBudgetClaimObservation{
		issueBudgetObservationOfClaim("would_refuse", "limit", child, first, a1, limit("sized", "5000000000000000000")),

		issueBudgetObservationOfClaim("coverage_gap", "reservation_overflow", child, second, a2, overflow),
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, second, a2,
			map[string]string{"ledger.reservation_unsized": "1", "observation.reservation_overflow": "1"}),
		issueBudgetObservationOfClaim("would_refuse", "limit", child, second, a2, limit("unsized", "0")),

		issueBudgetObservationOfClaim("coverage_gap", "legacy_protocol", child, third, a3, nil),
		issueBudgetObservationOfClaim("coverage_gap", "reservation_overflow", child, third, a3, overflow),
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, third, a3,
			map[string]string{"ledger.reservation_unsized": "2", "observation.legacy_protocol": "1", "observation.reservation_overflow": "2"}),
		issueBudgetObservationOfClaim("would_refuse", "limit", child, third, a3, limit("unsized", "0")),
	}, "three claims on a root whose reserve fits once")
	operations := f.claimOperations(t, root)
	if len(operations) != 3 {
		t.Fatalf("three claims left %d operation rows, want 3", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: root, Task: first, Attempt: a1, Reservation: "sized", Reserved: "5000000000000000000", WouldRefuse: []string{"limit"},
	}, "the claim that fits")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
		Root: root, Task: second, Attempt: a2, Reservation: "unsized", WouldRefuse: []string{"unbounded_unknown", "limit"},
	}, "the claim whose reserve does not fit")
	f.requireClaimOperation(t, operations[2], issueBudgetWantClaim{
		Root: root, Task: third, Attempt: a3, Reservation: "unsized", WouldRefuse: []string{"unbounded_unknown", "limit"},
	}, "the legacy claim whose reserve does not fit")
	entries := f.entryRows(t, root)
	if len(entries) != 3 {
		t.Fatalf("three claims left %d entry rows, want 3", len(entries))
	}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(a1, issueBudgetHugeReserve), accepted, operations[0].ID, "the reservation that fits")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetUnsizedReserveEntry(a2), unsized, operations[1].ID, "the reservation that did not fit")
	f.requireClaimEntry(t, entries[2], 3, issueBudgetUnsizedReserveEntry(a3), unsized, operations[2].ID, "the legacy reservation that did not fit")
	f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{ReservedTicks: issueBudgetHugeReserve, UnknownCount: 2}, "after three claims")
	// What the claims published is the ledger a full replay of the rows gives.
	revision, ledger, ok := issueBudgetLedgers.peek(util.MustParseUUID(root))
	if !ok || revision != 3 {
		t.Fatalf("the cache holds (%t, revision %d) for the root, want revision 3", ok, revision)
	}
	requireIssueBudgetLedger(t, ledger, 3, issueBudgetLedgerFigures{ReservedTicks: issueBudgetHugeReserve, UnknownCount: 2}, "the published ledger")
}

// F20, at a re-claim. The batch of a claim that supersedes an earlier one is
// the hold of the earlier reservation and then its own reserve. The hold
// moves 5e18 from reserved to held, and the new reserve beside it does not
// fit. The claim keeps the hold and reserves without an amount: the earlier
// attempt is closed and held exactly once.
func TestIssueBudgetFinalizeKeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	for _, tc := range []struct {
		name  string
		limit budget.Ticks
	}{
		{name: "a root with a limit", limit: 1000},
		{name: "a root without a limit"},
	} {
		enabled, _, child := f.budgetTree(t, tc.limit, issueBudgetHugeReserve)
		root := enabled.BudgetRootID
		first := f.claimed(t, agent, runtime, child, 1)
		a1 := f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)
		if lines := logs(); len(lines) != 0 {
			t.Fatalf("%s: the claim that fits logged %q", tc.name, lines)
		}

		second := f.claimedAgain(t, first)
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		finalized, err := f.finalize(t, f.claims(log), second, TaskClaimBudgetProtocolV1)
		if err != nil {
			t.Fatalf("%s: finalize a re-claim whose reserve does not fit: %v (the ledger's overflow: %t)", tc.name, err, errors.Is(err, budget.ErrOverflow))
		}
		a2 := util.UUIDToString(finalized.BudgetAttemptID)
		requireIssueBudgetOverflowLog(t, logs(), root, a2, second, tc.name)

		rows := f.attempts(t, first)
		if len(rows) != 2 {
			t.Fatalf("%s: two claims left %d attempt rows, want 2", tc.name, len(rows))
		}
		f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: enabled, Task: first, Issue: child, State: "superseded_unproven", Reserved: issueBudgetInt64(issueBudgetHugeReserve), Protocol: issueBudgetInt32(1)}, tc.name+": the superseded attempt")
		f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: enabled, Task: second, Issue: child, State: "open", Protocol: issueBudgetInt32(1)}, tc.name+": the attempt whose reserve does not fit")

		var first1, judged []issueBudgetClaimObservation
		var refuse1, refuse2 []string
		step := []string{
			"TryLockIssueBudgetBindingSharedForIssue",
			"GetIssueBudgetMember",
			"GetIssueBudgetTransactionIsolation",
			"LockIssueBudgetRoot",
			"ListIssueBudgetOpenAttemptsOfTask",
			// Closed once, and each entry inserted once: the refused batch
			// sent nothing.
			"CloseIssueBudgetAttempt",
			"InsertIssueBudgetEntry",
			"InsertIssueBudgetEntry",
			"UpdateIssueBudgetRootLedger",
			"InsertIssueBudgetAttempt",
			"InsertIssueBudgetObservation",
			"GetIssueBudgetMember",
		}
		if tc.limit != 0 {
			first1 = append(first1, issueBudgetObservationOfClaim("would_refuse", "limit", child, first, a1, map[string]string{
				"actual_known_ticks": "0", "reserved_ticks": "5000000000000000000", "unknown_hold_ticks": "0",
				"limit_ticks": "1000", "this_reservation": "sized", "this_reservation_ticks": "5000000000000000000",
			}))
			refuse1 = []string{"limit"}
			judged = append(judged,
				issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, second, a2, map[string]string{
					"ledger.superseded_unproven": "1", "ledger.reservation_unsized": "1", "observation.reservation_overflow": "1",
				}),
				issueBudgetObservationOfClaim("would_refuse", "limit", child, second, a2, map[string]string{
					"actual_known_ticks": "0", "reserved_ticks": "0", "unknown_hold_ticks": "5000000000000000000",
					"limit_ticks": "1000", "this_reservation": "unsized", "this_reservation_ticks": "0",
				}))
			refuse2 = []string{"unbounded_unknown", "limit"}
			step = append(step, "CountIssueBudgetUncoveredGaps", "InsertIssueBudgetObservation", "InsertIssueBudgetObservation")
		}
		step = append(step, "InsertIssueBudgetOperation")
		want := append(first1, issueBudgetObservationOfClaim("coverage_gap", "reservation_overflow", child, second, a2,
			map[string]string{"attempt_reserve_ticks": "5000000000000000000"}))
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), append(want, judged...), tc.name)
		requireIssueBudgetStep(t, log.names(), step, tc.name)

		operations := f.claimOperations(t, root)
		if len(operations) != 2 {
			t.Fatalf("%s: two claims left %d operation rows, want 2", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
			Root: root, Task: first, Attempt: a1, Reservation: "sized", Reserved: "5000000000000000000", WouldRefuse: refuse1,
		}, tc.name+": the claim that fits")
		f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
			Root: root, Task: second, Attempt: a2, Reservation: "unsized", Superseded: []string{a1}, WouldRefuse: refuse2,
		}, tc.name+": the re-claim")
		entries := f.entryRows(t, root)
		if len(entries) != 3 {
			t.Fatalf("%s: two claims of one task left %d entry rows, want 3", tc.name, len(entries))
		}
		f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(a1, issueBudgetHugeReserve),
			budget.Receipt{Disposition: budget.DispositionAccepted}, operations[0].ID, tc.name+": the reservation that fits")
		f.requireClaimEntry(t, entries[1], 2, issueBudgetHoldEntry(a1, "superseded_unproven"),
			budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "superseded_unproven"}, operations[1].ID, tc.name+": the hold")
		f.requireClaimEntry(t, entries[2], 3, issueBudgetUnsizedReserveEntry(a2),
			budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "reservation_unsized"}, operations[1].ID, tc.name+": the reservation that did not fit")
		f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{UnknownHoldTicks: issueBudgetHugeReserve, UnknownCount: 2}, tc.name)
	}
}

// F20 (3.2), the other failures of the append. A claim reserves without an
// amount for one failure only: a reserve that does not fit. An entry the
// ledger cannot take for any other reason fails the finalization as every
// failed write of the step does, and is not sent a second time without its
// amount.
//
// The failure here is a sequence number that is taken: an entry row committed
// behind the revision of the root row, which a load at revision 0 does not
// read.
func TestIssueBudgetFinalizeDoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	rootID := util.MustParseUUID(root)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	(&issueBudgetLedgerRoot{f: f, id: rootID}).insert(t, 1, issueBudgetUnsizedReserveEntry(uuid.NewString()))
	f.requireRootLedger(t, root, 0, issueBudgetLedgerFigures{}, "premise: the root row behind the entry row the test committed")
	logs := issueBudgetClaimLog(t)
	before := f.snapshot(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
	var refused *pgconn.PgError
	if err == nil || !errors.As(err, &refused) || refused.Code != "23505" || refused.TableName != "issue_budget_entry" {
		t.Fatalf("finalize with the sequence number taken = %+v, %v; want the unique violation of the entry table", finalized, err)
	}
	if errors.Is(err, budget.ErrOverflow) {
		t.Fatalf("a sequence number that is taken was reported as an overflow: %v", err)
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("a failed finalization handed back attempt %s", util.UUIDToString(finalized.BudgetAttemptID))
	}
	// One entry was sent, the sized reserve, and nothing behind it.
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfTask",
		"InsertIssueBudgetEntry",
	}, "the finalization whose entry is refused")
	// The line of a reserve that does not fit is not written.
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("the finalization whose entry is refused logged %q", lines)
	}
	f.requireUnchanged(t, before, "the finalization whose entry is refused")
	if got := f.tokens(t, task); got != 0 {
		t.Fatalf("the failed finalization left %d task tokens, want 0", got)
	}
	if rows := f.attempts(t, task); len(rows) != 0 {
		t.Fatalf("the failed finalization left attempt rows: %+v", rows)
	}
}

// F12. The budget of a task is the budget of its issue, whoever runs it. A
// task of a member issue whose agent and runtime belong to another workspace
// is attributed in the workspace of the issue: the try lock is that
// workspace's, the member row is found there, and every row the claim leaves
// is written there. A quick-create task run from another workspace is
// attributed in the workspace its context names. Nothing is written in the
// workspace that runs the task.
func TestIssueBudgetFinalizeAttributesATaskRunFromAnotherWorkspaceToTheWorkspaceOfItsIssue(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, parent, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID

	executing := f.fx.Workspace(t, "Executing workspace", "budget-exec-"+strings.ReplaceAll(uuid.NewString(), "-", ""))
	f.fx.Member(t, executing, f.user, "owner")
	issueBudgetCleanWorkspace(t, f.fx, executing)
	foreignRuntime := f.fx.Runtime(t, "budget-foreign-runtime-"+uuid.NewString(), testutil.Cols{"workspace_id": executing})
	foreignAgent := f.fx.Agent(t, "budget-foreign-agent-"+uuid.NewString(), foreignRuntime, testutil.Cols{"workspace_id": executing, "max_concurrent_tasks": 20})
	if executing == f.workspace {
		t.Fatal("premise: the executing workspace is the workspace of the issue")
	}

	// The handler finalizes with a token of the workspace of the runtime that
	// claimed.
	finalize := func(svc *TaskService, task db.AgentTaskQueue) string {
		t.Helper()
		finalized, err := svc.FinalizeTaskClaim(ctx, task, f.token(t, task, executing), nil, false, nil, TaskClaimBudgetProtocolV1)
		if err != nil {
			t.Fatalf("finalize the claim of a task run from another workspace: %v", err)
		}
		if !finalized.BudgetAttemptID.Valid {
			t.Fatal("the claim of a member issue's task run from another workspace got no attempt")
		}
		return util.UUIDToString(finalized.BudgetAttemptID)
	}

	task := f.claimed(t, foreignAgent, foreignRuntime, child, 2)
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	attempt := finalize(f.claims(log), task)
	// requireAttempt pins the workspace of the row to the fixture's: the
	// workspace of the issue.
	f.requireAttempt(t, f.onlyAttempt(t, task, attempt, "the task of an issue"), attempt, issueBudgetWantAttempt{
		Root: enabled, Task: task, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the task of an issue run from another workspace")
	if got := util.UUIDToString(task.RuntimeID); got != foreignRuntime {
		t.Fatalf("premise: the task is run by runtime %s, want the foreign runtime %s", got, foreignRuntime)
	}
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfTask",
		"InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
		"InsertIssueBudgetAttempt",
		"GetIssueBudgetMember",
		"CountIssueBudgetUncoveredGaps",
		"InsertIssueBudgetOperation",
	}, "the task of an issue run from another workspace")

	// The same for a quick-create task: its context names the workspace of
	// the parent.
	quick := f.claimedQuickCreate(t, foreignAgent, foreignRuntime, parent, 1)
	quickAttempt := finalize(f.claims(f.pool), quick)
	f.requireAttempt(t, f.onlyAttempt(t, quick, quickAttempt, "the quick-create task"), quickAttempt, issueBudgetWantAttempt{
		Root: enabled, Task: quick, ContextParent: parent, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the quick-create task run from another workspace")

	// The receipts name the workspace of the issue in their intent, and the
	// runtime that claimed as their actor; the entries are that workspace's.
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: task, Attempt: attempt, Reservation: "sized", Reserved: "100"}, "the task of an issue run from another workspace")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: quick, Attempt: quickAttempt, Reservation: "sized", Reserved: "100"}, "the quick-create task run from another workspace")
	if operations[0].ActorID != foreignRuntime || operations[1].ActorID != foreignRuntime {
		t.Fatalf("the receipts name actors %s and %s, want the runtime that claimed, %s", operations[0].ActorID, operations[1].ActorID, foreignRuntime)
	}
	entries := f.entryRows(t, root)
	if len(entries) != 2 {
		t.Fatalf("two claims left %d entry rows, want 2", len(entries))
	}
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(attempt, 100), accepted, operations[0].ID, "the reservation of the task of an issue")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetReserveEntry(quickAttempt, 100), accepted, operations[1].ID, "the reservation of the quick-create task")
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, "after two claims run from another workspace")
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, "two claims run from another workspace")

	for _, table := range issueBudgetTables {
		if rows := issueBudgetRows(t, f.fx, table, executing); rows != "[]" {
			t.Fatalf("the claims wrote %s rows in the workspace that ran the tasks: %s", table, rows)
		}
	}
	// The task tokens are the executing workspace's, as the handler mints
	// them.
	for _, claimed := range []db.AgentTaskQueue{task, quick} {
		if got := f.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1 AND workspace_id=$2`, util.UUIDToString(claimed.ID), executing); got != 1 {
			t.Fatalf("task %s has %d tokens of the executing workspace, want 1", util.UUIDToString(claimed.ID), got)
		}
	}
}

// issueBudgetIsolated begins every transaction at one isolation level.
type issueBudgetIsolated struct {
	pool  *pgxpool.Pool
	level pgx.TxIsoLevel
}

func (s issueBudgetIsolated) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: s.level})
}

// F13. The budget step judges on what it reads behind the root lock, which is
// the committed state only under READ COMMITTED: a stricter level reads the
// snapshot it took before it waited for the lock. Reached from such a
// transaction the step fails, in front of the root lock, for every claim that
// has a member row. The mode of the root does not matter: it is on the row
// the lock is for. A claim without a member row is never asked for its level.
func TestIssueBudgetFinalizeNeedsReadCommittedWhereThereIsARoot(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, parent, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	switchedOff, _, offChild := f.budgetTree(t, 0, 100)
	if _, err := f.svc.SetMode(ctx, f.manage(switchedOff.BudgetRootID, 1), "off"); err != nil {
		t.Fatalf("switch the second root off: %v", err)
	}
	agent, runtime := f.agent(t)
	onRoot := f.claimed(t, agent, runtime, child, 1)
	quick := f.claimedQuickCreate(t, agent, runtime, parent, 1)
	onOff := f.claimed(t, agent, runtime, offChild, 1)
	before := f.snapshot(t)

	for _, level := range []pgx.TxIsoLevel{pgx.RepeatableRead, pgx.Serializable} {
		for _, tc := range []struct {
			name string
			task db.AgentTaskQueue
			try  string
		}{
			{"a task of a member issue", onRoot, "TryLockIssueBudgetBindingSharedForIssue"},
			{"a quick-create task under a member issue", quick, "TryLockIssueBudgetBindingShared"},
			{"a task of a member issue of a root switched off", onOff, "TryLockIssueBudgetBindingSharedForIssue"},
		} {
			what := string(level) + ": " + tc.name
			log := &issueBudgetStatementLog{TxStarter: issueBudgetIsolated{pool: f.pool, level: level}}
			finalized, err := f.finalize(t, f.claims(log), tc.task, TaskClaimBudgetProtocolV1)
			if !errors.Is(err, ErrIssueBudgetIsolation) {
				t.Fatalf("%s: finalize = %+v, %v; want ErrIssueBudgetIsolation", what, finalized, err)
			}
			if finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the failed finalization handed back attempt %s", what, util.UUIDToString(finalized.BudgetAttemptID))
			}
			// The level is asked once the member row is found, and the root
			// row is not locked.
			requireIssueBudgetStep(t, log.names(), []string{tc.try, "GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation"}, what)
			if got := f.tokens(t, tc.task); got != 0 {
				t.Fatalf("%s: the failed finalization left %d task tokens, want 0", what, got)
			}
		}
		f.requireUnchanged(t, before, string(level)+": three failed finalizations")

		outside := f.issue(t, "Outside every root", "")
		for _, tc := range []struct {
			name  string
			issue string
			step  []string
		}{
			{"a task of an issue outside every root", outside, []string{"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember"}},
			{"a task without an issue", "", nil},
		} {
			what := string(level) + ": " + tc.name
			task := f.claimed(t, agent, runtime, tc.issue, 1)
			log := &issueBudgetStatementLog{TxStarter: issueBudgetIsolated{pool: f.pool, level: level}}
			finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
			if err != nil {
				t.Fatalf("%s: finalize: %v", what, err)
			}
			if finalized.BudgetAttemptID.Valid {
				t.Fatalf("%s: the finalization handed back attempt %s", what, util.UUIDToString(finalized.BudgetAttemptID))
			}
			requireIssueBudgetStep(t, log.names(), tc.step, what)
			if got := f.tokens(t, task); got != 1 {
				t.Fatalf("%s: the claim left %d task tokens, want 1", what, got)
			}
		}
		f.requireUnchanged(t, before, string(level)+": two claims outside every root")
	}

	// The same starter at READ COMMITTED: the claim that failed twice is a
	// first attempt.
	log := &issueBudgetStatementLog{TxStarter: issueBudgetIsolated{pool: f.pool, level: pgx.ReadCommitted}}
	attempt := f.mustFinalize(t, f.claims(log), onRoot, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, onRoot, attempt, "the claim at READ COMMITTED"), attempt, issueBudgetWantAttempt{
		Root: enabled, Task: onRoot, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim at READ COMMITTED")
	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("one claim left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: onRoot, Attempt: attempt, Reservation: "sized", Reserved: "100"}, "the claim at READ COMMITTED")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the claim at READ COMMITTED")
}

// finalizeWithoutTransaction is one finalization by a task service that has
// no transaction starter, with the statements it sent on the pool.
func (f *issueBudgetFixture) finalizeWithoutTransaction(t *testing.T, task db.AgentTaskQueue, before func(name string)) (TaskClaimFinalization, []string, error) {
	t.Helper()
	sent := &issueBudgetLedgerStatements{DBTX: f.pool, before: before}
	finalized, err := f.finalize(t, NewTaskService(db.New(sent), nil, nil, events.New()), task, TaskClaimBudgetProtocolV1)
	return finalized, sent.names(), err
}

// F14. A task service without a transaction starter sends the statements of
// a finalization one by one on the pool. The budget step has no transaction
// to lock a root in, so it takes no lock and writes nothing: it reads whether
// the claim has a member row and fails the claim that has one. It does so
// whatever the mode of the root, which is on a row it cannot lock. A claim
// without a member row goes on as before.
func TestIssueBudgetFinalizeWithoutATransactionFailsTheClaimOfAMemberIssue(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	_, parent, child := f.budgetTree(t, 1000, 100)
	switchedOff, offParent, offChild := f.budgetTree(t, 0, 100)
	if _, err := f.svc.SetMode(ctx, f.manage(switchedOff.BudgetRootID, 1), "off"); err != nil {
		t.Fatalf("switch the second root off: %v", err)
	}
	outside := f.issue(t, "Outside every root", "")
	agent, runtime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	before := f.snapshot(t)
	ofIssue := []string{"GetIssue", "GetIssueBudgetMember"}
	ofContext := []string{"GetIssueBudgetMember"}

	for _, tc := range []struct {
		name string
		task db.AgentTaskQueue
		step []string
	}{
		{"a task of a member issue", f.claimed(t, agent, runtime, child, 1), ofIssue},
		{"a task of the root issue", f.claimed(t, agent, runtime, parent, 1), ofIssue},
		{"a task of a member issue of a root switched off", f.claimed(t, agent, runtime, offChild, 1), ofIssue},
		{"a quick-create task under a member issue", f.claimedQuickCreate(t, agent, runtime, child, 1), ofContext},
		{"a quick-create task under the issue of a root switched off", f.claimedQuickCreate(t, agent, runtime, offParent, 1), ofContext},
	} {
		finalized, sent, err := f.finalizeWithoutTransaction(t, tc.task, nil)
		if !errors.Is(err, ErrIssueBudgetClaimNeedsTransaction) {
			t.Fatalf("%s: finalize without a transaction = %+v, %v; want ErrIssueBudgetClaimNeedsTransaction", tc.name, finalized, err)
		}
		if finalized.BudgetAttemptID.Valid {
			t.Fatalf("%s: the failed finalization handed back attempt %s", tc.name, util.UUIDToString(finalized.BudgetAttemptID))
		}
		// Reads only: no lock is asked for and no row is written.
		requireIssueBudgetStep(t, sent, tc.step, tc.name)
		if rows := f.attempts(t, tc.task); len(rows) != 0 {
			t.Fatalf("%s: the failed claim has attempt rows: %+v", tc.name, rows)
		}
	}
	f.requireUnchanged(t, before, "five claims of member issues without a transaction")

	for _, tc := range []struct {
		name string
		task db.AgentTaskQueue
		step []string
	}{
		{"a task of an issue outside every root", f.claimed(t, agent, runtime, outside, 1), ofIssue},
		{"a task without an issue", f.claimed(t, agent, runtime, "", 1), nil},
		{"a quick-create task under an issue outside every root", f.claimedQuickCreate(t, agent, runtime, outside, 1), ofContext},
		{"a quick-create task without a context parent", f.claimedWithContext(t, agent, runtime, QuickCreateContext{
			Type: QuickCreateContextType, Prompt: "file an issue", RequesterID: f.user, WorkspaceID: f.workspace,
		}, 1), nil},
	} {
		finalized, sent, err := f.finalizeWithoutTransaction(t, tc.task, nil)
		if err != nil {
			t.Fatalf("%s: finalize without a transaction: %v", tc.name, err)
		}
		if finalized.BudgetAttemptID.Valid {
			t.Fatalf("%s: the finalization handed back attempt %s", tc.name, util.UUIDToString(finalized.BudgetAttemptID))
		}
		requireIssueBudgetStep(t, sent, tc.step, tc.name)
		if got := f.tokens(t, tc.task); got != 1 {
			t.Fatalf("%s: the claim left %d task tokens, want 1", tc.name, got)
		}
	}
	f.requireUnchanged(t, before, "four claims outside every root without a transaction")
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("nine claims without a transaction logged %q", lines)
	}
}

// 3.3, the issue row that is not found. Without a transaction nothing holds
// the issue of a claimed task: it can be deleted between two statements of
// the finalization. The budget step then finds no issue row to read the
// workspace from. It logs one warning and lets the finalization go on without
// an attempt; it does not fail the claim, although the issue was a member.
func TestIssueBudgetFinalizeWithoutATransactionGoesOnWhenTheIssueRowIsGone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	_, _, child := f.budgetTree(t, 1000, 100)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	logs := issueBudgetClaimLog(t)
	before := f.snapshot(t)

	tokenWritten, deleted := false, false
	finalized, sent, err := f.finalizeWithoutTransaction(t, task, func(name string) {
		switch {
		case name == "CreateTaskToken":
			tokenWritten = true
		case name == "GetIssue" && tokenWritten && !deleted:
			// The first statement of the budget step is about to be sent.
			deleted = true
			f.deleteIssue(t, child)
		}
	})
	if err != nil {
		t.Fatalf("finalize a claim whose issue was deleted under it: %v", err)
	}
	if !deleted {
		t.Fatalf("the budget step never read the issue row; statements: %v", sent)
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("the finalization handed back attempt %s", util.UUIDToString(finalized.BudgetAttemptID))
	}
	// The member row is not looked for: there is no workspace to look in.
	requireIssueBudgetStep(t, sent, []string{"GetIssue"}, "the claim whose issue was deleted under it")
	requireIssueBudgetLogLine(t, logs(), "WARN", []string{"task_id=" + util.UUIDToString(task.ID), "issue_id=" + child}, "the claim whose issue was deleted under it")
	after := f.snapshot(t)
	for _, table := range []string{"issue_budget_attempt", "issue_budget_entry", "issue_budget_operation", "issue_budget_observation", "issue_budget_root"} {
		if before[table] != after[table] {
			t.Fatalf("the claim whose issue was deleted under it changed %s\nbefore: %s\nafter:  %s", table, before[table], after[table])
		}
	}
}

// 3.3, the issue row that is not found, in the path with a transaction. The
// budget step asks for the shared try lock through the issue row; no row means
// no workspace to look in, so it logs one warning and hands back an empty
// claim without an error. Failing instead would send the task back to the
// queue for ever.
//
// The case is built by a direct call because a finalization cannot produce
// it: lock_issue_execution ends a claim whose issue is gone before the budget
// step, and agent_task_queue_issue_id_fkey removes a task with its issue.
func TestIssueBudgetFinalizeStepGoesOnWhenTheIssueRowIsGone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	_, _, child := f.budgetTree(t, 1000, 100)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	// Only the value handed to the step names an issue that no row has; the
	// task row keeps the issue it was written with.
	gone := uuid.NewString()
	task.IssueID = util.MustParseUUID(gone)
	logs := issueBudgetClaimLog(t)
	before := f.snapshot(t)

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	claim, err := issueBudgetLedgers.finalizeTaskBudget(ctx, db.New(tx), true, task, TaskClaimBudgetProtocolV1)
	if err != nil {
		t.Fatalf("the budget step of a claim whose issue row is gone: %v", err)
	}
	if claim.AttemptID.Valid {
		t.Fatalf("the budget step handed back attempt %s", util.UUIDToString(claim.AttemptID))
	}
	// Commit, so that a row the step had written would be there to find.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	claim.Committed()
	requireIssueBudgetLogLine(t, logs(), "WARN", []string{"task_id=" + util.UUIDToString(task.ID), "issue_id=" + gone}, "the claim whose issue row is gone")
	f.requireUnchanged(t, before, "the claim whose issue row is gone")
	if rows := f.attempts(t, task); len(rows) != 0 {
		t.Fatalf("the claim whose issue row is gone has attempt rows: %+v", rows)
	}
}

// issueBudgetLostCommit is a TxStarter whose transactions are rolled back
// where they were to commit and say so: what a finalization is left with when
// its commit does not go through.
type issueBudgetLostCommit struct{ TxStarter }

var errIssueBudgetLostCommit = errors.New("test: the commit did not go through")

func (s issueBudgetLostCommit) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return issueBudgetLostCommitTx{Tx: tx}, nil
}

type issueBudgetLostCommitTx struct{ pgx.Tx }

func (tx issueBudgetLostCommitTx) Commit(ctx context.Context) error {
	if err := tx.Tx.Rollback(ctx); err != nil {
		return err
	}
	return errIssueBudgetLostCommit
}

// issueBudgetClaimStatements is what the budget step sends for the first
// claim of a task of an issue below the root issue, on an observing root with
// a limit whose ledger needs no load (design 10.2, steps 2 to 9).
var issueBudgetClaimStatements = []string{
	"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember", "GetIssueBudgetTransactionIsolation",
	"LockIssueBudgetRoot", "ListIssueBudgetOpenAttemptsOfTask", "InsertIssueBudgetEntry", "UpdateIssueBudgetRootLedger",
	"InsertIssueBudgetAttempt", "GetIssueBudgetMember", "CountIssueBudgetUncoveredGaps", "InsertIssueBudgetOperation",
}

// F18 (10.9, the cache), the commit that does not go through. The ledger a
// claim appended to is published to the process cache once the commit of its
// transaction is confirmed, and not before: a finalization that ran its
// budget step to the end and then lost its transaction leaves the cache with
// what is committed. The next claim, served from that cache, writes the rows
// and the figures of a first claim.
func TestIssueBudgetFinalizeThatDoesNotCommitPublishesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	rootID := util.MustParseUUID(root)
	agent, runtime := f.agent(t)
	otherAgent, otherRuntime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	issueBudgetLedgers.requireNotCached(t, rootID, "premise: a root no claim has read")

	// lose finalizes a claim in a transaction that is rolled back where it
	// was to commit.
	lose := func(task db.AgentTaskQueue, what string) {
		t.Helper()
		before := f.snapshot(t)
		log := &issueBudgetStatementLog{TxStarter: issueBudgetLostCommit{TxStarter: f.pool}}
		finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
		if !errors.Is(err, errIssueBudgetLostCommit) {
			t.Fatalf("%s: finalize = %+v, %v; want the error of the commit", what, finalized, err)
		}
		if finalized.BudgetAttemptID.Valid {
			t.Fatalf("%s: the finalization that did not commit handed back attempt %s", what, util.UUIDToString(finalized.BudgetAttemptID))
		}
		// The budget step ran to its last statement: the transaction was
		// lost behind it.
		requireIssueBudgetStep(t, log.names(), issueBudgetClaimStatements, what)
		f.requireUnchanged(t, before, what)
		if got := f.tokens(t, task); got != 0 {
			t.Fatalf("%s: the finalization that did not commit left %d task tokens, want 0", what, got)
		}
		if lines := logs(); len(lines) != 0 {
			t.Fatalf("%s: logged %q", what, lines)
		}
	}

	// On a root no claim has read. What the step loaded under the root lock
	// is committed and may stay in the cache; the reservation may not.
	first := f.claimed(t, agent, runtime, child, 1)
	lose(first, "a lost commit on a root no claim has read")
	if revision, ledger, ok := issueBudgetLedgers.peek(rootID); ok && (revision != 0 || ledger.Len() != 0) {
		t.Fatalf("after a lost commit the cache holds revision %d with %d entries; nothing of the root is committed", revision, ledger.Len())
	}

	again := f.claimedAgain(t, first)
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	a1 := f.mustFinalize(t, f.claims(log), again, TaskClaimBudgetProtocolV1)
	requireIssueBudgetStep(t, log.names(), issueBudgetClaimStatements, "the claim after the first lost commit")
	f.requireAttempt(t, f.onlyAttempt(t, again, a1, "the claim after the first lost commit"), a1, issueBudgetWantAttempt{
		Root: enabled, Task: again, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim after the first lost commit")
	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("one committed claim left %d operation rows, want 1", len(operations))
	}
	// It supersedes nothing: the claim that did not commit left no attempt.
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: again, Attempt: a1, Reservation: "sized", Reserved: "100"}, "the claim after the first lost commit")
	entries := f.entryRows(t, root)
	if len(entries) != 1 {
		t.Fatalf("one committed claim left %d entry rows, want 1", len(entries))
	}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(a1, 100), accepted, operations[0].ID, "the reservation after the first lost commit")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the first committed claim")

	// On a root the cache holds: the state it holds is the very one it held.
	revision, cached, ok := issueBudgetLedgers.peek(rootID)
	if !ok || revision != 1 {
		t.Fatalf("after a committed claim the cache holds (%v, revision %d), want revision 1", ok, revision)
	}
	requireIssueBudgetLedger(t, cached, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the cache after the first committed claim")
	second := f.claimed(t, otherAgent, otherRuntime, child, 1)
	lose(second, "a lost commit on a root the cache holds")
	issueBudgetLedgers.requireCached(t, rootID, 1, cached, "after a lost commit on a root the cache holds")

	secondAgain := f.claimedAgain(t, second)
	log = &issueBudgetStatementLog{TxStarter: f.pool}
	a2 := f.mustFinalize(t, f.claims(log), secondAgain, TaskClaimBudgetProtocolV1)
	// No load and no state dropped: the cache was at the committed revision.
	requireIssueBudgetStep(t, log.names(), issueBudgetClaimStatements, "the claim after the second lost commit")
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("the claim after the second lost commit logged %q", lines)
	}
	f.requireAttempt(t, f.onlyAttempt(t, secondAgain, a2, "the claim after the second lost commit"), a2, issueBudgetWantAttempt{
		Root: enabled, Task: secondAgain, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim after the second lost commit")
	operations = f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two committed claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: secondAgain, Attempt: a2, Reservation: "sized", Reserved: "100"}, "the claim after the second lost commit")
	entries = f.entryRows(t, root)
	if len(entries) != 2 {
		t.Fatalf("two committed claims left %d entry rows, want 2", len(entries))
	}
	f.requireClaimEntry(t, entries[1], 2, issueBudgetReserveEntry(a2, 100), accepted, operations[1].ID, "the reservation after the second lost commit")
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, "after the second committed claim")
}

// replayedLedger is the ledger a replay of every committed entry row of a
// root gives, from an empty ledger and without the cache.
func (f *issueBudgetFixture) replayedLedger(t *testing.T, root string) *budget.Ledger {
	t.Helper()
	rows := f.entryRows(t, root)
	entries := make([]budget.Entry, 0, len(rows))
	for i, row := range rows {
		if row.Seq != int64(i+1) {
			t.Fatalf("entry row %d of the root has seq %d", i+1, row.Seq)
		}
		entry, err := budget.DecodeEntry(int(row.SchemaVersion), row.EntryJson)
		if err != nil {
			t.Fatalf("stored form of entry %d: %v", row.Seq, err)
		}
		entries = append(entries, entry)
	}
	ledger, _, err := budget.NewLedger().With(entries...)
	if err != nil {
		t.Fatalf("replay %d entry rows: %v", len(rows), err)
	}
	return ledger
}

// F18 (10.9, the cache), the claim served from the cache. A claim that finds
// the root's revision in the process cache reads no entry row. What it
// judges on and what it writes to the root row must be what a replay of every
// entry row gives: after each claim the figures on the root row, the ledger
// in the cache and a full replay are compared with the same literals. A
// claim on a root the cache has forgotten reads the entry rows again and
// comes to the same.
func TestIssueBudgetFinalizeServedFromTheCacheWritesWhatAFullReplayGives(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 250, 100)
	root := enabled.BudgetRootID
	rootID := util.MustParseUUID(root)
	agent, runtime := f.agent(t)
	otherAgent, otherRuntime := f.agent(t)
	logs := issueBudgetClaimLog(t)
	const load = "ListIssueBudgetEntryFormsAfter"
	unknown := func(task db.AgentTaskQueue, attempt, holds string) issueBudgetClaimObservation {
		return issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, task, attempt, map[string]string{"ledger.superseded_unproven": holds})
	}
	overLimit := func(task db.AgentTaskQueue, attempt, reserved, hold string) issueBudgetClaimObservation {
		return issueBudgetObservationOfClaim("would_refuse", "limit", child, task, attempt, map[string]string{
			"actual_known_ticks": "0", "reserved_ticks": reserved, "unknown_hold_ticks": hold,
			"limit_ticks": "250", "this_reservation": "sized", "this_reservation_ticks": "100",
		})
	}
	var observations []issueBudgetClaimObservation

	// claim finalizes one claim and compares what it left with the revision
	// and the figures given. cached says whether the step had to be served
	// from the cache.
	claim := func(task db.AgentTaskQueue, cached bool, revision int64, figures issueBudgetLedgerFigures, what string) string {
		t.Helper()
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		attempt := f.mustFinalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
		if attempt == "" {
			t.Fatalf("%s: the claim got no attempt", what)
		}
		step := issueBudgetStep(t, log.names(), what)
		at := -1
		for i, name := range step {
			if name != load {
				continue
			}
			if at >= 0 {
				t.Fatalf("%s: the step read the entry rows twice:\n%s", what, strings.Join(step, "\n"))
			}
			at = i
		}
		switch {
		case cached && at >= 0:
			t.Fatalf("%s: the step read the entry rows; the cache held the revision of the root:\n%s", what, strings.Join(step, "\n"))
		case !cached && (at < 1 || step[at-1] != "LockIssueBudgetRoot"):
			t.Fatalf("%s: the step did not read the entry rows behind the root lock:\n%s", what, strings.Join(step, "\n"))
		}
		if lines := logs(); len(lines) != 0 {
			t.Fatalf("%s: logged %q", what, lines)
		}

		f.requireRootLedger(t, root, revision, figures, what)
		replayed := f.replayedLedger(t, root)
		requireIssueBudgetLedger(t, replayed, int(revision), figures, what+": a full replay")
		gotRevision, published, ok := issueBudgetLedgers.peek(rootID)
		if !ok || gotRevision != revision {
			t.Fatalf("%s: the cache holds (%v, revision %d), want revision %d", what, ok, gotRevision, revision)
		}
		requireIssueBudgetLedger(t, published, int(revision), figures, what+": the cache")
		if !reflect.DeepEqual(published.Projection(), replayed.Projection()) {
			t.Fatalf("%s: the cache projects\n%+v\na full replay\n%+v", what, published.Projection(), replayed.Projection())
		}
		return attempt
	}

	// A first claim: the root is at revision 0 and has no entry row to read.
	first := f.claimed(t, agent, runtime, child, 1)
	a1 := claim(first, true, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the first claim")
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, "the first claim")

	// The task again: the first reservation becomes a hold, 100 + 100.
	second := f.claimedAgain(t, first)
	a2 := claim(second, true, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, "the second claim")
	observations = append(observations, unknown(second, a2, "1"))
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, "the second claim")

	// Another task: 200 reserved beside the hold of 100 is over 250, which
	// the claim can know only from the two entries it did not read.
	third := f.claimed(t, otherAgent, otherRuntime, child, 1)
	a3 := claim(third, true, 4, issueBudgetLedgerFigures{ReservedTicks: 200, UnknownHoldTicks: 100, UnknownCount: 1}, "the third claim")
	observations = append(observations, unknown(third, a3, "1"), overLimit(third, a3, "200", "100"))
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, "the third claim")

	// The cache forgets the root: the next claim reads the four entry rows
	// and goes on from the same figures.
	if !issueBudgetLedgers.forget(rootID, nil) {
		t.Fatal("the cache did not hold the root it was to forget")
	}
	fourth := f.claimedAgain(t, third)
	a4 := claim(fourth, false, 6, issueBudgetLedgerFigures{ReservedTicks: 200, UnknownHoldTicks: 200, UnknownCount: 2}, "the claim after the cache forgot the root")
	observations = append(observations, unknown(fourth, a4, "2"), overLimit(fourth, a4, "200", "200"))
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, "the claim after the cache forgot the root")

	operations := f.claimOperations(t, root)
	if len(operations) != 4 {
		t.Fatalf("four claims left %d operation rows, want 4", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: first, Attempt: a1, Reservation: "sized", Reserved: "100"}, "the first claim")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
		Root: root, Task: second, Attempt: a2, Reservation: "sized", Reserved: "100", Superseded: []string{a1}, WouldRefuse: []string{"unbounded_unknown"},
	}, "the second claim")
	f.requireClaimOperation(t, operations[2], issueBudgetWantClaim{
		Root: root, Task: third, Attempt: a3, Reservation: "sized", Reserved: "100", WouldRefuse: []string{"unbounded_unknown", "limit"},
	}, "the third claim")
	f.requireClaimOperation(t, operations[3], issueBudgetWantClaim{
		Root: root, Task: fourth, Attempt: a4, Reservation: "sized", Reserved: "100", Superseded: []string{a3}, WouldRefuse: []string{"unbounded_unknown", "limit"},
	}, "the claim after the cache forgot the root")
}
