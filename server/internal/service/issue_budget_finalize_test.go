package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The budget step of a claim's finalization (design 10.2), in three files.
// This one holds the helpers and the rows an ordinary claim leaves: the
// attempt, its reservation, the claims it supersedes, what a root would
// refuse it for (10.12), and the claims a root does not see.
// issue_budget_finalize_edge_test.go holds the claims off the ordinary path,
// the ledger that cannot be loaded (10.5) among them, and
// issue_budget_finalize_locks_test.go the cases of 10.9 and 10.10 that need a
// second connection.
//
// Rows of the F3 acceptance matrix covered by the three: B07, B08, B34, B35,
// B35a, B45, B46, B47, the finalize half of B53 and the finalize part of B55.
// The handler suite owns the two claim endpoints.
//
// The expectations are written from the design: the rows a claim leaves, the
// statements it sends and their order are literals here, and the identities
// and digests are spelled out and not taken from the code under test.

// claims is a task service that finalizes claims in transactions begun
// through tx.
func (f *issueBudgetFixture) claims(tx TxStarter) *TaskService {
	return NewTaskService(f.q, tx, nil, events.New())
}

func (f *issueBudgetFixture) taskRow(t *testing.T, id string) db.AgentTaskQueue {
	t.Helper()
	row, err := f.q.GetAgentTask(context.Background(), util.MustParseUUID(id))
	if err != nil {
		t.Fatalf("read task %s: %v", id, err)
	}
	return row
}

// claimed is a task as a committed claim leaves it: dispatched to its runtime
// at the generation given. issue is empty for a task without an issue.
func (f *issueBudgetFixture) claimed(t *testing.T, agent, runtime, issue string, generation int64, over ...testutil.Cols) db.AgentTaskQueue {
	t.Helper()
	cols := testutil.Cols{"runtime_id": runtime, "status": "dispatched", "claim_generation": generation, "dispatched_at": testutil.Raw("now()")}
	if issue != "" {
		cols["issue_id"] = issue
	}
	return f.taskRow(t, f.fx.Task(t, agent, append([]testutil.Cols{cols}, over...)...))
}

// claimedQuickCreate is a claimed quick-create task whose saved context names
// parent.
func (f *issueBudgetFixture) claimedQuickCreate(t *testing.T, agent, runtime, parent string, generation int64) db.AgentTaskQueue {
	t.Helper()
	return f.taskRow(t, f.quickCreateTask(t, agent, runtime, parent, "dispatched", generation, testutil.Cols{"dispatched_at": testutil.Raw("now()")}))
}

// claimedAgain is the next claim of a task that went back to the queue: what
// the claim statement does to the row.
func (f *issueBudgetFixture) claimedAgain(t *testing.T, task db.AgentTaskQueue) db.AgentTaskQueue {
	t.Helper()
	id := util.UUIDToString(task.ID)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched', dispatched_at=clock_timestamp(), claim_generation=claim_generation+1 WHERE id=$1`, id)
	return f.taskRow(t, id)
}

// token is the task token a claim handler finalizes with: the task's, its
// agent's, and the workspace of the runtime that claimed.
func (f *issueBudgetFixture) token(t *testing.T, task db.AgentTaskQueue, runtimeWorkspace string) db.CreateTaskTokenParams {
	t.Helper()
	hash := "budget-claim-" + uuid.NewString()
	f.fx.Cleanup(t, `DELETE FROM task_token WHERE token_hash=$1`, hash)
	return db.CreateTaskTokenParams{
		TokenHash: hash, TaskID: task.ID, AgentID: task.AgentID,
		WorkspaceID: util.MustParseUUID(runtimeWorkspace), UserID: f.actor,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}
}

func (f *issueBudgetFixture) tokens(t *testing.T, task db.AgentTaskQueue) int {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, util.UUIDToString(task.ID))
}

// finalize is one claim finalization of a task run from the fixture
// workspace, by a daemon that declared the protocol given.
func (f *issueBudgetFixture) finalize(t *testing.T, svc *TaskService, task db.AgentTaskQueue, declared TaskClaimBudgetProtocol) (TaskClaimFinalization, error) {
	t.Helper()
	return svc.FinalizeTaskClaim(context.Background(), task, f.token(t, task, f.workspace), nil, false, nil, declared)
}

// mustFinalize is finalize that has to commit, with the attempt id it handed
// back; the id is empty when the claim got no attempt.
func (f *issueBudgetFixture) mustFinalize(t *testing.T, svc *TaskService, task db.AgentTaskQueue, declared TaskClaimBudgetProtocol) string {
	t.Helper()
	finalized, err := f.finalize(t, svc, task, declared)
	if err != nil {
		t.Fatalf("finalize claim of task %s at generation %d: %v", util.UUIDToString(task.ID), task.ClaimGeneration, err)
	}
	if !finalized.BudgetAttemptID.Valid {
		return ""
	}
	return util.UUIDToString(finalized.BudgetAttemptID)
}

// budgetTree is a parent issue with a child, the parent enabled as a root with
// the limit and the reserve given; zero stands for none.
func (f *issueBudgetFixture) budgetTree(t *testing.T, limit, reserve budget.Ticks) (enabled IssueBudgetOperationResult, parent, child string) {
	t.Helper()
	parent = f.issue(t, "Budget parent", "")
	child = f.issue(t, "Budget child", parent)
	params := f.enableParams(parent)
	if limit != 0 {
		params.LimitTicks = &limit
	}
	if reserve != 0 {
		params.AttemptReserveTicks = &reserve
	}
	enabled, err := f.svc.Enable(context.Background(), params)
	if err != nil {
		t.Fatalf("enable budget root: %v", err)
	}
	return enabled, parent, child
}

func (f *issueBudgetFixture) rootRow(t *testing.T, root string) db.IssueBudgetRoot {
	t.Helper()
	row, err := f.q.GetIssueBudgetRoot(context.Background(), db.GetIssueBudgetRootParams{WorkspaceID: f.ws, ID: util.MustParseUUID(root)})
	if err != nil {
		t.Fatalf("read budget root: %v", err)
	}
	return row
}

// requireRootLedger compares the ledger columns of a root row with literals.
func (f *issueBudgetFixture) requireRootLedger(t *testing.T, root string, revision int64, want issueBudgetLedgerFigures, what string) {
	t.Helper()
	row := f.rootRow(t, root)
	if got := issueBudgetRootFigures(row); row.LedgerRevision != revision || got != want {
		t.Fatalf("%s: root at revision %d with %+v, want revision %d with %+v", what, row.LedgerRevision, got, revision, want)
	}
}

func (f *issueBudgetFixture) entryRows(t *testing.T, root string) []db.IssueBudgetEntry {
	t.Helper()
	rows, err := f.q.ListIssueBudgetEntries(context.Background(), db.ListIssueBudgetEntriesParams{WorkspaceID: f.ws, BudgetRootID: util.MustParseUUID(root)})
	if err != nil {
		t.Fatalf("read entry rows: %v", err)
	}
	return rows
}

// issueBudgetAttemptRow is an attempt row as a test compares it; a nil pointer
// is a NULL column.
type issueBudgetAttemptRow struct {
	ID                   string  `json:"id"`
	WorkspaceID          string  `json:"workspace_id"`
	BudgetRootID         string  `json:"budget_root_id"`
	TaskID               string  `json:"task_id"`
	ClaimGeneration      int64   `json:"claim_generation"`
	RuntimeID            string  `json:"runtime_id"`
	AgentID              string  `json:"agent_id"`
	IssueID              *string `json:"issue_id"`
	ContextParentIssueID *string `json:"context_parent_issue_id"`
	State                string  `json:"state"`
	ReservedTicks        *int64  `json:"reserved_ticks"`
	ProtocolVersion      *int32  `json:"protocol_version"`
	PolicyRevision       *int64  `json:"policy_revision"`
	PolicyEpoch          *int64  `json:"policy_epoch"`
}

// attempts are the attempt rows of a task in any workspace, oldest claim
// first.
func (f *issueBudgetFixture) attempts(t *testing.T, task db.AgentTaskQueue) []issueBudgetAttemptRow {
	t.Helper()
	var raw string
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.claim_generation), '[]'::jsonb)::text FROM issue_budget_attempt a WHERE a.task_id=$1`,
		util.UUIDToString(task.ID)).Scan(&raw)
	var rows []issueBudgetAttemptRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode attempt rows %s: %v", raw, err)
	}
	return rows
}

// issueBudgetWantAttempt is the attempt row a claim of task must leave on a
// root, apart from its id.
type issueBudgetWantAttempt struct {
	Root          IssueBudgetOperationResult
	Task          db.AgentTaskQueue
	Issue         string // the task's own issue
	ContextParent string // the context parent of a quick-create task
	State         string
	Reserved      *int64
	Protocol      *int32
	// Revision and Epoch are the policy the root had when the claim was
	// judged, where that is not the policy Root was enabled with.
	Revision, Epoch int64
}

func issueBudgetInt64(v int64) *int64 { return &v }
func issueBudgetInt32(v int32) *int32 { return &v }

// requireAttempt fails unless got is the row want describes, with the id
// given, in the workspace of the fixture: the workspace of the issue, whoever
// ran the task.
func (f *issueBudgetFixture) requireAttempt(t *testing.T, got issueBudgetAttemptRow, id string, want issueBudgetWantAttempt, what string) {
	t.Helper()
	revision, epoch := int64(want.Root.PolicyRevision), int64(want.Root.PolicyEpoch)
	if want.Revision != 0 {
		revision, epoch = want.Revision, want.Epoch
	}
	row := issueBudgetAttemptRow{
		ID: id, WorkspaceID: f.workspace, BudgetRootID: want.Root.BudgetRootID,
		TaskID: util.UUIDToString(want.Task.ID), ClaimGeneration: want.Task.ClaimGeneration,
		RuntimeID: util.UUIDToString(want.Task.RuntimeID), AgentID: util.UUIDToString(want.Task.AgentID),
		State: want.State, ReservedTicks: want.Reserved, ProtocolVersion: want.Protocol,
		PolicyRevision: &revision, PolicyEpoch: &epoch,
	}
	if want.Issue != "" {
		row.IssueID = &want.Issue
	}
	if want.ContextParent != "" {
		row.ContextParentIssueID = &want.ContextParent
	}
	if !reflect.DeepEqual(got, row) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(row)
		t.Fatalf("%s: attempt row\n got: %s\nwant: %s", what, gotJSON, wantJSON)
	}
}

// issueBudgetClaimObservation is an observation as a claim test compares it.
// Generation is -1 when the row names none.
type issueBudgetClaimObservation struct {
	Kind, Reason, Issue, Task, Attempt string
	Generation                         int64
	Detail                             map[string]string
}

// claimObservations are the observations of a root as the service lists them,
// oldest first, each with its attempt and its detail.
func (f *issueBudgetFixture) claimObservations(t *testing.T, root string) []issueBudgetClaimObservation {
	t.Helper()
	rows, err := f.svc.ListObservations(context.Background(), f.ws, util.MustParseUUID(root))
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	out := make([]issueBudgetClaimObservation, 0, len(rows))
	for _, row := range rows {
		item := issueBudgetClaimObservation{Kind: row.Row.Kind, Reason: row.Row.Reason, Generation: -1}
		if row.Row.IssueID.Valid {
			item.Issue = util.UUIDToString(row.Row.IssueID)
		}
		if row.Row.TaskID.Valid {
			item.Task = util.UUIDToString(row.Row.TaskID)
		}
		if row.Row.AttemptID.Valid {
			item.Attempt = util.UUIDToString(row.Row.AttemptID)
		}
		if row.Row.ClaimGeneration.Valid {
			item.Generation = row.Row.ClaimGeneration.Int64
		}
		if err := json.Unmarshal(row.Row.Detail, &item.Detail); err != nil {
			t.Fatalf("decode observation detail %s: %v", row.Row.Detail, err)
		}
		out = append(out, item)
	}
	return out
}

// claimObservation is an observation a claim of task writes about attempt on
// the issue it was attributed through. A nil detail is the empty one.
func issueBudgetObservationOfClaim(kind, reason, issue string, task db.AgentTaskQueue, attempt string, detail map[string]string) issueBudgetClaimObservation {
	if detail == nil {
		detail = map[string]string{}
	}
	return issueBudgetClaimObservation{
		Kind: kind, Reason: reason, Issue: issue, Task: util.UUIDToString(task.ID), Attempt: attempt,
		Generation: task.ClaimGeneration, Detail: detail,
	}
}

func requireIssueBudgetClaimObservations(t *testing.T, got, want []issueBudgetClaimObservation, what string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", " ")
		wantJSON, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("%s: observations\n got: %s\nwant: %s", what, gotJSON, wantJSON)
	}
}

// issueBudgetOperationRow is an operation row as a test compares it.
type issueBudgetOperationRow struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	BudgetRootID  string          `json:"budget_root_id"`
	ScopeKind     string          `json:"scope_kind"`
	ScopeID       string          `json:"scope_id"`
	OperationKey  string          `json:"operation_key"`
	OperationKind string          `json:"operation_kind"`
	IntentDigest  string          `json:"intent_digest"`
	ActorType     string          `json:"actor_type"`
	ActorID       string          `json:"actor_id"`
	Result        json.RawMessage `json:"result"`
}

// claimOperations are the operation rows of a root that its management did
// not write, oldest first: every row whose scope is not the root or its issue.
func (f *issueBudgetFixture) claimOperations(t *testing.T, root string) []issueBudgetOperationRow {
	t.Helper()
	var raw string
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.created_at, o.id), '[]'::jsonb)::text
		FROM issue_budget_operation o WHERE o.budget_root_id=$1 AND o.scope_kind NOT IN ('issue', 'root')`, root).Scan(&raw)
	var rows []issueBudgetOperationRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode operation rows %s: %v", raw, err)
	}
	return rows
}

// issueBudgetWantClaim is the receipt a claim of Task must leave for Attempt.
type issueBudgetWantClaim struct {
	Root string
	Task db.AgentTaskQueue
	// Attempt is the id of the attempt row the claim wrote.
	Attempt string
	// Reservation is "sized", "unsized" or "none"; Reserved is the amount of a
	// sized one, in decimal.
	Reservation, Reserved string
	LedgerUnavailable     bool
	Superseded            []string
	WouldRefuse           []string
}

// requireClaimOperation compares an operation row with the receipt design
// 10.2 gives a claim. The key, the digest input and the result are spelled out
// here; only the id of the row is read back, and it has to be the one the
// result names.
func (f *issueBudgetFixture) requireClaimOperation(t *testing.T, got issueBudgetOperationRow, want issueBudgetWantClaim, what string) {
	t.Helper()
	task, generation := util.UUIDToString(want.Task.ID), fmt.Sprintf("%d", want.Task.ClaimGeneration)
	list := func(items []string) string {
		encoded, err := json.Marshal(append([]string{}, items...))
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	reserved := "null"
	if want.Reserved != "" {
		reserved = `"` + want.Reserved + `"`
	}
	result := `{"operation_id":"` + got.ID + `","operation_kind":"attempt_open","budget_root_id":"` + want.Root +
		`","attempt_id":"` + want.Attempt + `","task_id":"` + task + `","claim_generation":"` + generation +
		`","reservation":"` + want.Reservation + `","reserved_ticks":` + reserved +
		`,"ledger_unavailable":` + fmt.Sprintf("%t", want.LedgerUnavailable) +
		`,"superseded_attempt_ids":` + list(want.Superseded) + `,"would_refuse":` + list(want.WouldRefuse) + `}`
	intent := "budget-operation-intent-v1\n" + `{"kind":"attempt_open","workspace_id":"` + f.workspace +
		`","scope_kind":"attempt","scope_id":"` + want.Attempt + `","task_id":"` + task + `","claim_generation":"` + generation + `"}`
	sum := sha256.Sum256([]byte(intent))
	wantRow := issueBudgetOperationRow{
		ID: got.ID, WorkspaceID: f.workspace, BudgetRootID: want.Root,
		ScopeKind: "attempt", ScopeID: want.Attempt,
		OperationKey: "claim:" + task + ":" + generation, OperationKind: "attempt_open",
		IntentDigest: hex.EncodeToString(sum[:]),
		ActorType:    "runtime", ActorID: util.UUIDToString(want.Task.RuntimeID),
	}
	var gotResult, wantResult any
	if err := json.Unmarshal(got.Result, &gotResult); err != nil {
		t.Fatalf("%s: decode stored result %s: %v", what, got.Result, err)
	}
	if err := json.Unmarshal([]byte(result), &wantResult); err != nil {
		t.Fatalf("%s: the expected result is not JSON: %s: %v", what, result, err)
	}
	gotRow := got
	gotRow.Result = nil
	if !reflect.DeepEqual(gotRow, wantRow) || !reflect.DeepEqual(gotResult, wantResult) {
		gotJSON, _ := json.Marshal(got)
		wantRow.Result = json.RawMessage(result)
		wantJSON, _ := json.Marshal(wantRow)
		t.Fatalf("%s: operation row\n got: %s\nwant: %s", what, gotJSON, wantJSON)
	}
}

// requireClaimEntry compares one entry row a claim appended with the entry
// design 10.2 gives it: its place in the sequence, its identity, the attempt
// it belongs to, the stored form, the receipt, the source and the operation
// it was written with.
func (f *issueBudgetFixture) requireClaimEntry(t *testing.T, got db.IssueBudgetEntry, seq int64, want budget.Entry, receipt budget.Receipt, operation, what string) {
	t.Helper()
	if got.Seq != seq || got.EntryID != want.ID || got.Kind != string(want.Kind) ||
		util.UUIDToString(got.AttemptID) != want.ReservationID || got.Source != "finalize" ||
		util.UUIDToString(got.OperationID) != operation || util.UUIDToString(got.WorkspaceID) != f.workspace {
		t.Fatalf("%s: entry row seq %d %q kind %q attempt %s source %q operation %s workspace %s, want seq %d %q kind %q attempt %s source \"finalize\" operation %s workspace %s",
			what, got.Seq, got.EntryID, got.Kind, util.UUIDToString(got.AttemptID), got.Source, util.UUIDToString(got.OperationID), util.UUIDToString(got.WorkspaceID),
			seq, want.ID, want.Kind, want.ReservationID, operation, f.workspace)
	}
	if got.InvocationID.Valid || got.ScopeID.Valid || got.ReceivedAt.Valid {
		t.Fatalf("%s: entry row %q carries an invocation, a scope or an arrival time; an entry the server wrote has none", what, got.EntryID)
	}
	stored, err := budget.DecodeEntry(int(got.SchemaVersion), got.EntryJson)
	if err != nil {
		t.Fatalf("%s: stored form of %q: %v", what, got.EntryID, err)
	}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("%s: stored entry\n got: %+v\nwant: %+v", what, stored, want)
	}
	if got.EntryDigest != issueBudgetStoredDigest(got.EntryJson) {
		t.Fatalf("%s: entry row %q has digest %s, which is not the digest of its stored form", what, got.EntryID, got.EntryDigest)
	}
	var committed budget.Receipt
	if err := json.Unmarshal(got.Receipt, &committed); err != nil {
		t.Fatalf("%s: receipt of %q: %v", what, got.EntryID, err)
	}
	receipt.EntryID, receipt.Kind = want.ID, want.Kind
	if committed != receipt {
		t.Fatalf("%s: receipt of %q\n got: %+v\nwant: %+v", what, got.EntryID, committed, receipt)
	}
}

// issueBudgetStep is what the budget step of a finalization sent: every
// statement behind CreateTaskToken, which is the last write a finalization
// without a comment receipt makes in front of the step. No budget statement
// may come before it.
func issueBudgetStep(t *testing.T, names []string, what string) []string {
	t.Helper()
	at := -1
	for i, name := range names {
		switch {
		case name == "CreateTaskToken":
			if at >= 0 {
				t.Fatalf("%s: the finalization wrote two task tokens; statements:\n%s", what, strings.Join(names, "\n"))
			}
			at = i
		case at < 0 && strings.Contains(name, "IssueBudget"):
			t.Fatalf("%s: %s was sent in front of the task token; statements:\n%s", what, name, strings.Join(names, "\n"))
		}
	}
	if at < 0 {
		t.Fatalf("%s: the finalization never wrote its task token; statements:\n%s", what, strings.Join(names, "\n"))
	}
	return names[at+1:]
}

func requireIssueBudgetStep(t *testing.T, names []string, want []string, what string) {
	t.Helper()
	got := issueBudgetStep(t, names, what)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: the budget step sent\n%s\nwant\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// F21. The intent digest of a claim, from two vectors computed outside Go
// (shasum over the input spelled out below, which has no trailing newline).
// The input is the domain prefix and the six keys of design 10.2 in their
// order; a generation is a decimal inside a JSON string.
func TestIssueBudgetFinalizeIntentDigestMatchesGoldenVectors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		generation budget.Count
		input      string
		digest     string
	}{
		{
			name: "generation 3", generation: 3,
			input:  "budget-operation-intent-v1\n" + `{"kind":"attempt_open","workspace_id":"11111111-2222-4333-8444-555555555555","scope_kind":"attempt","scope_id":"66666666-7777-4888-8999-aaaaaaaaaaaa","task_id":"bbbbbbbb-cccc-4ddd-8eee-ffffffffffff","claim_generation":"3"}`,
			digest: "5f2e4c37c71a6576e32770626caee975abeddc7b8b4e04f4c411cabfd6bf8e4f",
		},
		{
			name: "generation 0", generation: 0,
			input:  "budget-operation-intent-v1\n" + `{"kind":"attempt_open","workspace_id":"11111111-2222-4333-8444-555555555555","scope_kind":"attempt","scope_id":"66666666-7777-4888-8999-aaaaaaaaaaaa","task_id":"bbbbbbbb-cccc-4ddd-8eee-ffffffffffff","claim_generation":"0"}`,
			digest: "bb9ff61c65fc956466d7be9815ce8df8fd550c8aefcb2571979742f9c1e9ecf0",
		},
	} {
		// The two literals of a vector must agree with each other before
		// either says anything about the code.
		if sum := sha256.Sum256([]byte(tc.input)); hex.EncodeToString(sum[:]) != tc.digest {
			t.Fatalf("%s: the vector is inconsistent: its input hashes to %x", tc.name, sum)
		}
		intent := issueBudgetClaimIntent{
			Kind:            "attempt_open",
			WorkspaceID:     "11111111-2222-4333-8444-555555555555",
			ScopeKind:       "attempt",
			ScopeID:         "66666666-7777-4888-8999-aaaaaaaaaaaa",
			TaskID:          "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff",
			ClaimGeneration: tc.generation,
		}
		encoded, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if hashed := issueBudgetIntentDomain + string(encoded); hashed != tc.input {
			t.Fatalf("%s: the digest input changed\n got %s\nwant %s", tc.name, hashed, tc.input)
		}
		got, err := intent.digest()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.digest {
			t.Fatalf("%s: digest %s, want %s", tc.name, got, tc.digest)
		}
	}
}

// F01 (B34). One claim of a task of a member issue, by a daemon that declared
// the protocol, on a root with a limit and a reserve that nothing exceeds.
// Every row it leaves and every statement of its budget step, from design
// 10.2.
func TestIssueBudgetFinalizeWritesTheAttemptItsReservationAndItsReceipt(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	rootID := util.MustParseUUID(root)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	// Nothing of the claim is published before its commit is confirmed: with
	// the transaction still open the cache holds what the claim loaded, the
	// committed ledger without an entry.
	log.beforeCommit = func(uint32) {
		if revision, _, ok := issueBudgetLedgers.peek(rootID); !ok || revision != 0 {
			t.Errorf("before the finalize transaction committed the cache holds (%t, revision %d) for the root, want the committed revision 0", ok, revision)
		}
	}
	finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if len(finalized.DeliveredCommentIDs) != 0 {
		t.Fatalf("a claim without a comment receipt handed back %v", finalized.DeliveredCommentIDs)
	}

	rows := f.attempts(t, task)
	if len(rows) != 1 {
		t.Fatalf("the claim left %d attempt rows, want 1", len(rows))
	}
	attempt := rows[0].ID
	if !finalized.BudgetAttemptID.Valid || util.UUIDToString(finalized.BudgetAttemptID) != attempt {
		t.Fatalf("finalize handed back attempt %s (valid %t), the row is %s", util.UUIDToString(finalized.BudgetAttemptID), finalized.BudgetAttemptID.Valid, attempt)
	}
	if _, err := uuid.Parse(attempt); err != nil {
		t.Fatalf("attempt id %q: %v", attempt, err)
	}
	f.requireAttempt(t, rows[0], attempt, issueBudgetWantAttempt{
		Root: enabled, Task: task, Issue: child, State: "open",
		Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim")

	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("the claim left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: root, Task: task, Attempt: attempt, Reservation: "sized", Reserved: "100",
	}, "the claim")
	if operations[0].ID == attempt || operations[0].ID == enabled.OperationID {
		t.Fatalf("the receipt has id %s, which is not an id of its own", operations[0].ID)
	}

	entries := f.entryRows(t, root)
	if len(entries) != 1 {
		t.Fatalf("the claim left %d entry rows, want 1", len(entries))
	}
	f.requireClaimEntry(t, entries[0], 1, budget.Entry{
		ID: "reserve/" + attempt, Kind: budget.KindReserve, ReservationID: attempt, ReservedTicks: 100,
	}, budget.Receipt{Disposition: budget.DispositionAccepted}, operations[0].ID, "the reservation")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the claim")

	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, "a claim nothing would refuse")
	if got := f.tokens(t, task); got != 1 {
		t.Fatalf("the claim left %d task tokens, want 1", got)
	}

	// Steps 2 to 9 of design 10.2 as statements, behind every other write of
	// the finalization. The root is at revision 0, so there is nothing to
	// load; the claim's issue is not the root issue, so the root issue's
	// member row is read; the root has a limit, so the gaps are counted.
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
	}, "the first claim on a root with a limit")
	if log.transactions() != 1 {
		t.Fatalf("the finalization opened %d transactions, want 1", log.transactions())
	}

	// Once committed, the ledger of the claim is what the next writer of the
	// root starts from.
	revision, ledger, ok := issueBudgetLedgers.peek(rootID)
	if !ok || revision != 1 {
		t.Fatalf("after the commit the cache holds (%t, revision %d) for the root, want revision 1", ok, revision)
	}
	requireIssueBudgetLedger(t, ledger, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "the published ledger")
}

// onlyAttempt is the one attempt row a task must have, which has to be the
// attempt its finalization handed back.
func (f *issueBudgetFixture) onlyAttempt(t *testing.T, task db.AgentTaskQueue, handedBack, what string) issueBudgetAttemptRow {
	t.Helper()
	rows := f.attempts(t, task)
	if len(rows) != 1 {
		t.Fatalf("%s: the task has %d attempt rows, want 1", what, len(rows))
	}
	if handedBack == "" || rows[0].ID != handedBack {
		t.Fatalf("%s: the finalization handed back attempt %q, the row is %s", what, handedBack, rows[0].ID)
	}
	return rows[0]
}

// F02. A root that declares no reserve: the reservation has no amount. That
// is one unknown item of the ledger, which a root with a limit would refuse
// the claim for and a root without one has nothing to say about.
func TestIssueBudgetFinalizeReservesWithoutAnAmountOnARootThatDeclaresNoReserve(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	for _, tc := range []struct {
		name   string
		limit  budget.Ticks
		refuse []string
		step   []string
	}{
		{
			name: "a root with a limit", limit: 1000, refuse: []string{"unbounded_unknown"},
			step: []string{
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
				"InsertIssueBudgetObservation",
				"InsertIssueBudgetOperation",
			},
		},
		{
			// Without a limit there is no amount to refuse on, and the gaps
			// are not counted.
			name: "a root without a limit",
			step: []string{
				"TryLockIssueBudgetBindingSharedForIssue",
				"GetIssueBudgetMember",
				"GetIssueBudgetTransactionIsolation",
				"LockIssueBudgetRoot",
				"ListIssueBudgetOpenAttemptsOfTask",
				"InsertIssueBudgetEntry",
				"UpdateIssueBudgetRootLedger",
				"InsertIssueBudgetAttempt",
				"GetIssueBudgetMember",
				"InsertIssueBudgetOperation",
			},
		},
	} {
		enabled, _, child := f.budgetTree(t, tc.limit, 0)
		root := enabled.BudgetRootID
		task := f.claimed(t, agent, runtime, child, 1)
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		attempt := f.mustFinalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)

		f.requireAttempt(t, f.onlyAttempt(t, task, attempt, tc.name), attempt, issueBudgetWantAttempt{
			Root: enabled, Task: task, Issue: child, State: "open", Protocol: issueBudgetInt32(1),
		}, tc.name)
		operations := f.claimOperations(t, root)
		if len(operations) != 1 {
			t.Fatalf("%s: the claim left %d operation rows, want 1", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
			Root: root, Task: task, Attempt: attempt, Reservation: "unsized", WouldRefuse: tc.refuse,
		}, tc.name)
		entries := f.entryRows(t, root)
		if len(entries) != 1 {
			t.Fatalf("%s: the claim left %d entry rows, want 1", tc.name, len(entries))
		}
		f.requireClaimEntry(t, entries[0], 1, budget.Entry{
			ID: "reserve/" + attempt, Kind: budget.KindReserve, ReservationID: attempt, Unsized: true,
		}, budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "reservation_unsized"}, operations[0].ID, tc.name)
		f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{UnknownCount: 1}, tc.name)

		var want []issueBudgetClaimObservation
		if tc.limit != 0 {
			want = append(want, issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, task, attempt,
				map[string]string{"ledger.reservation_unsized": "1"}))
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), want, tc.name)
		requireIssueBudgetStep(t, log.names(), tc.step, tc.name)
	}
}

// F03 (B07). A server retry gives the work a new task id. Its claim is an
// attempt of its own on the same root: it supersedes nothing, and both
// reservations are in the reserved total.
func TestIssueBudgetFinalizeCountsEveryTaskOfAnIssue(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 0, 100)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	svc := f.claims(f.pool)

	first := f.claimed(t, agent, runtime, child, 1)
	firstAttempt := f.mustFinalize(t, svc, first, TaskClaimBudgetProtocolV1)
	// The first task ends in a failure; the retry the server queues for it is
	// a row of its own.
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='failed', completed_at=now() WHERE id=$1`, util.UUIDToString(first.ID))
	retry := f.claimed(t, agent, runtime, child, 1)
	retryAttempt := f.mustFinalize(t, svc, retry, TaskClaimBudgetProtocolV1)
	if first.ID == retry.ID || firstAttempt == retryAttempt {
		t.Fatalf("the retry is task %s with attempt %s, the same as the first", util.UUIDToString(retry.ID), retryAttempt)
	}

	for _, claim := range []struct {
		task    db.AgentTaskQueue
		attempt string
	}{{first, firstAttempt}, {retry, retryAttempt}} {
		f.requireAttempt(t, f.onlyAttempt(t, claim.task, claim.attempt, "a task of the issue"), claim.attempt, issueBudgetWantAttempt{
			Root: enabled, Task: claim.task, Issue: child, State: "open",
			Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
		}, "a task of the issue")
	}
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: first, Attempt: firstAttempt, Reservation: "sized", Reserved: "100"}, "the first task")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: retry, Attempt: retryAttempt, Reservation: "sized", Reserved: "100"}, "the retry")
	entries := f.entryRows(t, root)
	if len(entries) != 2 {
		t.Fatalf("two claims left %d entry rows, want 2", len(entries))
	}
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(firstAttempt, 100), accepted, operations[0].ID, "the first task")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetReserveEntry(retryAttempt, 100), accepted, operations[1].ID, "the retry")
	f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, "after both claims")
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, "two claims on a root without a limit")
}

// F04 (B08, B35). The same task claimed again: the claim before it was handed
// to a daemon or it was not, and nothing here can tell. Its attempt is closed
// as superseded_unproven and what it reserved becomes a hold; nothing is
// released. Each claim supersedes the open attempts before it and no other.
func TestIssueBudgetFinalizeSupersedesTheEarlierClaimOfATask(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 0, 100)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	accepted := budget.Receipt{Disposition: budget.DispositionAccepted}
	held := budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "superseded_unproven"}

	first := f.claimed(t, agent, runtime, child, 1)
	a1 := f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)

	second := f.claimedAgain(t, first)
	if second.ID != first.ID || second.ClaimGeneration != 2 {
		t.Fatalf("the second claim is task %s at generation %d", util.UUIDToString(second.ID), second.ClaimGeneration)
	}
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	a2 := f.mustFinalize(t, f.claims(log), second, TaskClaimBudgetProtocolV1)
	// The earlier attempt is listed and closed before the batch is appended:
	// its hold in front of the new reservation, then the root row. The cache
	// holds the ledger the first claim published, so nothing is loaded.
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfTask",
		"CloseIssueBudgetAttempt",
		"InsertIssueBudgetEntry",
		"InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
		"InsertIssueBudgetAttempt",
		"GetIssueBudgetMember",
		"InsertIssueBudgetOperation",
	}, "the second claim of the task")

	rows := f.attempts(t, first)
	if len(rows) != 2 || rows[0].ID != a1 || rows[1].ID != a2 || a1 == a2 {
		t.Fatalf("after the second claim the task has attempts %+v, want %s then %s", rows, a1, a2)
	}
	f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: enabled, Task: first, Issue: child, State: "superseded_unproven", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, "the superseded attempt")
	f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: enabled, Task: second, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, "the second attempt")
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{Root: root, Task: first, Attempt: a1, Reservation: "sized", Reserved: "100"}, "the first claim")
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: second, Attempt: a2, Reservation: "sized", Reserved: "100", Superseded: []string{a1}}, "the second claim")
	entries := f.entryRows(t, root)
	if len(entries) != 3 {
		t.Fatalf("two claims of one task left %d entry rows, want 3", len(entries))
	}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(a1, 100), accepted, operations[0].ID, "the first reservation")
	// The hold a server writes has an empty detail (design 10.2).
	f.requireClaimEntry(t, entries[1], 2, budget.Entry{
		ID: "hold/" + a1 + "/superseded_unproven", Kind: budget.KindHold, ReservationID: a1, HoldReason: "superseded_unproven",
	}, held, operations[1].ID, "the hold of the superseded attempt")
	f.requireClaimEntry(t, entries[2], 3, issueBudgetReserveEntry(a2, 100), accepted, operations[1].ID, "the second reservation")
	// What the first claim reserved moved from R to H: 100 + 100, none of it
	// released.
	f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 100, UnknownCount: 1}, "after the second claim")

	third := f.claimedAgain(t, second)
	a3 := f.mustFinalize(t, f.claims(f.pool), third, TaskClaimBudgetProtocolV1)
	rows = f.attempts(t, first)
	if len(rows) != 3 || rows[0].ID != a1 || rows[1].ID != a2 || rows[2].ID != a3 {
		t.Fatalf("after the third claim the task has attempts %+v, want %s, %s, %s", rows, a1, a2, a3)
	}
	f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: enabled, Task: first, Issue: child, State: "superseded_unproven", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, "the first attempt after the third claim")
	f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: enabled, Task: second, Issue: child, State: "superseded_unproven", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, "the second attempt after the third claim")
	f.requireAttempt(t, rows[2], a3, issueBudgetWantAttempt{Root: enabled, Task: third, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1)}, "the third attempt")
	operations = f.claimOperations(t, root)
	if len(operations) != 3 {
		t.Fatalf("three claims left %d operation rows, want 3", len(operations))
	}
	// The third claim supersedes the second only: the first is not open.
	f.requireClaimOperation(t, operations[2], issueBudgetWantClaim{Root: root, Task: third, Attempt: a3, Reservation: "sized", Reserved: "100", Superseded: []string{a2}}, "the third claim")
	entries = f.entryRows(t, root)
	if len(entries) != 5 {
		t.Fatalf("three claims of one task left %d entry rows, want 5", len(entries))
	}
	f.requireClaimEntry(t, entries[3], 4, issueBudgetHoldEntry(a2, "superseded_unproven"), held, operations[2].ID, "the hold of the second attempt")
	f.requireClaimEntry(t, entries[4], 5, issueBudgetReserveEntry(a3, 100), accepted, operations[2].ID, "the third reservation")
	f.requireRootLedger(t, root, 5, issueBudgetLedgerFigures{ReservedTicks: 100, UnknownHoldTicks: 200, UnknownCount: 2}, "after the third claim")
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, "three claims on a root without a limit")
}

// F04, the unsized half. An earlier reservation without an amount is held
// like any other: it has nothing to move, and it stays one of the things the
// root does not know beside the hold and the new reservation.
func TestIssueBudgetFinalizeSupersedesAnEarlierClaimThatReservedNoAmount(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 0, 0)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	svc := f.claims(f.pool)

	first := f.claimed(t, agent, runtime, child, 1)
	a1 := f.mustFinalize(t, svc, first, TaskClaimBudgetProtocolV1)
	second := f.claimedAgain(t, first)
	a2 := f.mustFinalize(t, svc, second, TaskClaimBudgetProtocolV1)

	rows := f.attempts(t, first)
	if len(rows) != 2 {
		t.Fatalf("two claims left %d attempt rows, want 2", len(rows))
	}
	f.requireAttempt(t, rows[0], a1, issueBudgetWantAttempt{Root: enabled, Task: first, Issue: child, State: "superseded_unproven", Protocol: issueBudgetInt32(1)}, "the superseded attempt")
	f.requireAttempt(t, rows[1], a2, issueBudgetWantAttempt{Root: enabled, Task: second, Issue: child, State: "open", Protocol: issueBudgetInt32(1)}, "the second attempt")
	operations := f.claimOperations(t, root)
	if len(operations) != 2 {
		t.Fatalf("two claims left %d operation rows, want 2", len(operations))
	}
	f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{Root: root, Task: second, Attempt: a2, Reservation: "unsized", Superseded: []string{a1}}, "the second claim")
	entries := f.entryRows(t, root)
	if len(entries) != 3 {
		t.Fatalf("two claims of one task left %d entry rows, want 3", len(entries))
	}
	unsized := budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "reservation_unsized"}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetUnsizedReserveEntry(a1), unsized, operations[0].ID, "the first reservation")
	f.requireClaimEntry(t, entries[1], 2, issueBudgetHoldEntry(a1, "superseded_unproven"),
		budget.Receipt{Disposition: budget.DispositionAccepted, Reason: "superseded_unproven"}, operations[1].ID, "the hold of the superseded attempt")
	f.requireClaimEntry(t, entries[2], 3, issueBudgetUnsizedReserveEntry(a2), unsized, operations[1].ID, "the second reservation")
	f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{UnknownCount: 3}, "after the second claim")
}

// issueBudgetClaimLog captures what the budget step logs at warn level and
// above until the test ends. The reader it returns hands back the lines
// written since it was last called.
func issueBudgetClaimLog(t *testing.T) func() []string {
	t.Helper()
	read := issueBudgetLedgerWarnings(issueBudgetLedgers)
	t.Cleanup(func() { issueBudgetLedgers.log = nil })
	return read
}

// requireIssueBudgetLogLine fails unless lines is exactly one line of the
// level given that carries every field.
func requireIssueBudgetLogLine(t *testing.T, lines []string, level string, fields []string, what string) {
	t.Helper()
	if len(lines) != 1 {
		t.Fatalf("%s: %d log lines, want 1: %q", what, len(lines), lines)
	}
	for _, field := range append([]string{"level=" + level}, fields...) {
		if !strings.Contains(lines[0]+" ", " "+field+" ") {
			t.Fatalf("%s: the log line does not carry %q: %s", what, field, lines[0])
		}
	}
}

// deleteIssue is what the delete transaction of an issue does to a budget
// tree: the member row is marked, the children are detached, the issue row
// goes.
func (f *issueBudgetFixture) deleteIssue(t *testing.T, issue string) {
	t.Helper()
	f.inBinding(t, func(q *db.Queries) error {
		return TombstoneIssueBudgetMember(context.Background(), q, f.ws, util.MustParseUUID(issue))
	})
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE parent_issue_id=$1`, issue)
	f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, issue)
}

// F08 (B47, first half). A tree without a root: the claim is finalized as it
// was before there were budgets. The budget step asks two things for a task
// of an issue, the shared try lock and the member row, and nothing for a task
// that has no issue and is not a quick-create task.
func TestIssueBudgetFinalizeLeavesATreeWithoutARootAlone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	parent := f.issue(t, "Plain parent", "")
	child := f.issue(t, "Plain child", parent)
	agent, runtime := f.agent(t)
	before := f.snapshot(t)
	for _, table := range issueBudgetTables {
		if before[table] != "[]" {
			t.Fatalf("premise: %s holds rows before any root was enabled: %s", table, before[table])
		}
	}
	logs := issueBudgetClaimLog(t)

	for _, tc := range []struct {
		name     string
		issue    string
		declared TaskClaimBudgetProtocol
		step     []string
	}{
		{
			name: "a task of an issue", issue: child, declared: TaskClaimBudgetProtocolV1,
			step: []string{"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember"},
		},
		{
			// A daemon that declares no protocol is a gap of a root, and of
			// nothing else.
			name: "a task of an issue, run by a daemon that declares no protocol", issue: parent, declared: TaskClaimBudgetProtocolNone,
			step: []string{"TryLockIssueBudgetBindingSharedForIssue", "GetIssueBudgetMember"},
		},
		{name: "a task without an issue", declared: TaskClaimBudgetProtocolV1},
	} {
		task := f.claimed(t, agent, runtime, tc.issue, 1)
		log := &issueBudgetStatementLog{TxStarter: f.pool}
		finalized, err := f.finalize(t, f.claims(log), task, tc.declared)
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
		if log.transactions() != 1 {
			t.Fatalf("%s: the finalization opened %d transactions, want 1", tc.name, log.transactions())
		}
	}
	if lines := logs(); len(lines) != 0 {
		t.Fatalf("claims outside every root logged %q", lines)
	}
}

// F15. A root that is switched off is not observed: the claim finds the root,
// reads its mode on the row it locked and writes nothing. It gets no attempt
// id. Observed again, the next claim of the task is a first attempt of the
// new policy epoch.
func TestIssueBudgetFinalizeWritesNothingForARootSwitchedOff(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	enabled, _, child := f.budgetTree(t, 50, 100)
	root := enabled.BudgetRootID
	if _, err := f.svc.SetMode(ctx, f.manage(root, 1), "off"); err != nil {
		t.Fatalf("switch the root off: %v", err)
	}
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	before := f.snapshot(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	// A root that is off has nothing to say about the daemon's protocol or
	// about its limit either.
	finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolNone)
	if err != nil {
		t.Fatalf("finalize on a root that is off: %v", err)
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("the finalization handed back attempt %s for a root that is off", util.UUIDToString(finalized.BudgetAttemptID))
	}
	f.requireUnchanged(t, before, "the claim on a root that is off")
	if got := f.tokens(t, task); got != 1 {
		t.Fatalf("the claim left %d task tokens, want 1", got)
	}
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
	}, "the claim on a root that is off")
	if revision, _, ok := issueBudgetLedgers.peek(util.MustParseUUID(root)); ok {
		t.Fatalf("the claim on a root that is off left the root in the cache at revision %d; it loads no ledger", revision)
	}

	observed, err := f.svc.SetMode(ctx, f.manage(root, 2), "observe")
	if err != nil {
		t.Fatalf("switch the root back to observe: %v", err)
	}
	again := f.claimedAgain(t, task)
	attempt := f.mustFinalize(t, f.claims(f.pool), again, TaskClaimBudgetProtocolV1)
	// Enabled at revision 1 in epoch 1, off at revision 2, observed again at
	// revision 3 in a new epoch.
	f.requireAttempt(t, f.onlyAttempt(t, again, attempt, "the claim after the root is observed again"), attempt, issueBudgetWantAttempt{
		Root: observed, Task: again, Issue: child, State: "open",
		Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1), Revision: 3, Epoch: 2,
	}, "the claim after the root is observed again")
	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("one observed claim left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: root, Task: again, Attempt: attempt, Reservation: "sized", Reserved: "100", WouldRefuse: []string{"unbounded_unknown", "limit"},
	}, "the claim after the root is observed again")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the observed claim")
}

// F22 (3.3). A member row whose root row does not exist is damage no code
// path writes. There is nothing to attribute to: the claim gets no attempt,
// the damage is logged once as an error, and the execution goes on.
func TestIssueBudgetFinalizeGoesOnWhenTheRootRowOfAMemberIsGone(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	f.fx.Exec(t, `DELETE FROM issue_budget_root WHERE id=$1`, root)
	if _, ok := f.member(t, child); !ok {
		t.Fatal("premise: the member row of the child went with the root row")
	}
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 3)
	before := f.snapshot(t)
	logs := issueBudgetClaimLog(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
	if err != nil {
		t.Fatalf("finalize a claim whose root row is gone: %v", err)
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("the finalization handed back attempt %s without a root", util.UUIDToString(finalized.BudgetAttemptID))
	}
	f.requireUnchanged(t, before, "the claim whose root row is gone")
	if got := f.tokens(t, task); got != 1 {
		t.Fatalf("the claim left %d task tokens, want 1", got)
	}
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
	}, "the claim whose root row is gone")
	requireIssueBudgetLogLine(t, logs(), "ERROR", []string{
		"budget_root_id=" + root, "issue_id=" + child, "task_id=" + util.UUIDToString(task.ID), "claim_generation=3",
	}, "the claim whose root row is gone")
}

// F10 (B53, finalize half). A daemon that declares no usage protocol reports
// nothing the root could meter: its attempt has no protocol version, and the
// claim leaves a coverage gap that names the attempt, the task and the claim
// generation. On a root with a limit every uncovered gap is part of what the
// root does not know, for that claim and for each one after it. A daemon that
// declares the protocol leaves no gap.
func TestIssueBudgetFinalizeRecordsADaemonWithoutTheProtocolAsAGap(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 1000, 100)
	root := enabled.BudgetRootID
	agent, runtime := f.agent(t)
	secondAgent, secondRuntime := f.agent(t)
	thirdAgent, thirdRuntime := f.agent(t)

	legacy := f.claimed(t, agent, runtime, child, 4)
	log := &issueBudgetStatementLog{TxStarter: f.pool}
	a1 := f.mustFinalize(t, f.claims(log), legacy, TaskClaimBudgetProtocolNone)
	f.requireAttempt(t, f.onlyAttempt(t, legacy, a1, "the first legacy claim"), a1, issueBudgetWantAttempt{
		Root: enabled, Task: legacy, Issue: child, State: "open", Reserved: issueBudgetInt64(100),
	}, "the first legacy claim")
	// The gap is written behind the attempt row and in front of everything
	// that is judged, so the judgement counts it.
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
		"InsertIssueBudgetOperation",
	}, "the first legacy claim")

	secondLegacy := f.claimed(t, secondAgent, secondRuntime, child, 1)
	a2 := f.mustFinalize(t, f.claims(f.pool), secondLegacy, TaskClaimBudgetProtocolNone)
	declared := f.claimed(t, thirdAgent, thirdRuntime, child, 1)
	a3 := f.mustFinalize(t, f.claims(f.pool), declared, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, secondLegacy, a2, "the second legacy claim"), a2, issueBudgetWantAttempt{
		Root: enabled, Task: secondLegacy, Issue: child, State: "open", Reserved: issueBudgetInt64(100),
	}, "the second legacy claim")
	f.requireAttempt(t, f.onlyAttempt(t, declared, a3, "the claim of a daemon that declares the protocol"), a3, issueBudgetWantAttempt{
		Root: enabled, Task: declared, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim of a daemon that declares the protocol")

	requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), []issueBudgetClaimObservation{
		issueBudgetObservationOfClaim("coverage_gap", "legacy_protocol", child, legacy, a1, nil),
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, legacy, a1, map[string]string{"observation.legacy_protocol": "1"}),
		issueBudgetObservationOfClaim("coverage_gap", "legacy_protocol", child, secondLegacy, a2, nil),
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, secondLegacy, a2, map[string]string{"observation.legacy_protocol": "2"}),
		// The third claim declares the protocol and leaves no gap of its own;
		// the two before it are still not covered.
		issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, declared, a3, map[string]string{"observation.legacy_protocol": "2"}),
	}, "two legacy claims and a declared one on a root with a limit")
	operations := f.claimOperations(t, root)
	if len(operations) != 3 {
		t.Fatalf("three claims left %d operation rows, want 3", len(operations))
	}
	for i, claim := range []struct {
		task    db.AgentTaskQueue
		attempt string
	}{{legacy, a1}, {secondLegacy, a2}, {declared, a3}} {
		f.requireClaimOperation(t, operations[i], issueBudgetWantClaim{
			Root: root, Task: claim.task, Attempt: claim.attempt, Reservation: "sized", Reserved: "100", WouldRefuse: []string{"unbounded_unknown"},
		}, fmt.Sprintf("claim %d on the root with a limit", i+1))
	}
	f.requireRootLedger(t, root, 3, issueBudgetLedgerFigures{ReservedTicks: 300}, "after three claims")

	// A root without a limit records the gap and judges nothing on it.
	unlimited, _, unlimitedChild := f.budgetTree(t, 0, 100)
	plain := f.claimed(t, agent, runtime, unlimitedChild, 1)
	log = &issueBudgetStatementLog{TxStarter: f.pool}
	a4 := f.mustFinalize(t, f.claims(log), plain, TaskClaimBudgetProtocolNone)
	f.requireAttempt(t, f.onlyAttempt(t, plain, a4, "the legacy claim on a root without a limit"), a4, issueBudgetWantAttempt{
		Root: unlimited, Task: plain, Issue: unlimitedChild, State: "open", Reserved: issueBudgetInt64(100),
	}, "the legacy claim on a root without a limit")
	requireIssueBudgetClaimObservations(t, f.claimObservations(t, unlimited.BudgetRootID), []issueBudgetClaimObservation{
		issueBudgetObservationOfClaim("coverage_gap", "legacy_protocol", unlimitedChild, plain, a4, nil),
	}, "a legacy claim on a root without a limit")
	operations = f.claimOperations(t, unlimited.BudgetRootID)
	if len(operations) != 1 {
		t.Fatalf("one claim left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: unlimited.BudgetRootID, Task: plain, Attempt: a4, Reservation: "sized", Reserved: "100",
	}, "the legacy claim on a root without a limit")
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
		"InsertIssueBudgetOperation",
	}, "the legacy claim on a root without a limit")
}

// F07 (B46). What an enforcing root would have refused a claim for: each of
// the four reasons alone, and all four in one claim, one observation each, in
// the order root_closed, root_deleted, unbounded_unknown, limit, which is the
// order of the receipt's would_refuse list. A root without a limit has no
// amount to refuse on and can record only the first two. The execution is
// never held back: every finalization here commits.
func TestIssueBudgetFinalizeRecordsEveryReasonARootWouldRefuseFor(t *testing.T) {
	f := newIssueBudgetFixture(t)
	ctx := context.Background()
	agent, runtime := f.agent(t)
	type refusal struct {
		reason string
		detail map[string]string
	}
	for _, tc := range []struct {
		name            string
		limit, reserve  budget.Ticks
		closed, deleted bool
		// reclaim: the claim that is judged is the second claim of its task;
		// the root is closed and its issue deleted between the two.
		reclaim  bool
		declared TaskClaimBudgetProtocol
		// gap is true when the claim leaves a legacy_protocol gap in front of
		// what is judged.
		gap    bool
		refuse []refusal
		step   []string
	}{
		{
			name: "a closed root", limit: 1000, reserve: 100, closed: true, declared: TaskClaimBudgetProtocolV1,
			refuse: []refusal{{reason: "root_closed"}},
		},
		{
			name: "a root whose issue was deleted", limit: 1000, reserve: 100, deleted: true, declared: TaskClaimBudgetProtocolV1,
			refuse: []refusal{{reason: "root_deleted"}},
		},
		{
			name: "a root with a limit that does not know an amount", limit: 1000, declared: TaskClaimBudgetProtocolV1,
			refuse: []refusal{{reason: "unbounded_unknown", detail: map[string]string{"ledger.reservation_unsized": "1"}}},
		},
		{
			name: "a claim that takes the root over its limit", limit: 50, reserve: 100, declared: TaskClaimBudgetProtocolV1,
			refuse: []refusal{{reason: "limit", detail: map[string]string{
				"actual_known_ticks": "0", "reserved_ticks": "100", "unknown_hold_ticks": "0",
				"limit_ticks": "50", "this_reservation": "sized", "this_reservation_ticks": "100",
			}}},
		},
		{
			name: "all four", limit: 150, reserve: 100, closed: true, deleted: true, reclaim: true, declared: TaskClaimBudgetProtocolV1,
			refuse: []refusal{
				{reason: "root_closed"},
				{reason: "root_deleted"},
				{reason: "unbounded_unknown", detail: map[string]string{"ledger.superseded_unproven": "1"}},
				{reason: "limit", detail: map[string]string{
					"actual_known_ticks": "0", "reserved_ticks": "100", "unknown_hold_ticks": "100",
					"limit_ticks": "150", "this_reservation": "sized", "this_reservation_ticks": "100",
				}},
			},
			// Each judgement is written where it is made: root_closed, the
			// root issue's member row and root_deleted, the gaps counted, then
			// unbounded_unknown and limit, and the receipt behind them all.
			step: []string{
				"TryLockIssueBudgetBindingSharedForIssue",
				"GetIssueBudgetMember",
				"GetIssueBudgetTransactionIsolation",
				"LockIssueBudgetRoot",
				"ListIssueBudgetOpenAttemptsOfTask",
				"CloseIssueBudgetAttempt",
				"InsertIssueBudgetEntry",
				"InsertIssueBudgetEntry",
				"UpdateIssueBudgetRootLedger",
				"InsertIssueBudgetAttempt",
				"InsertIssueBudgetObservation",
				"GetIssueBudgetMember",
				"InsertIssueBudgetObservation",
				"CountIssueBudgetUncoveredGaps",
				"InsertIssueBudgetObservation",
				"InsertIssueBudgetObservation",
				"InsertIssueBudgetOperation",
			},
		},
		{
			// No limit: the reservation without an amount and the legacy
			// daemon are both things the root does not know, and neither is
			// judged.
			name: "a root without a limit, closed, its issue deleted", closed: true, deleted: true, declared: TaskClaimBudgetProtocolNone, gap: true,
			refuse: []refusal{{reason: "root_closed"}, {reason: "root_deleted"}},
			step: []string{
				"TryLockIssueBudgetBindingSharedForIssue",
				"GetIssueBudgetMember",
				"GetIssueBudgetTransactionIsolation",
				"LockIssueBudgetRoot",
				"ListIssueBudgetOpenAttemptsOfTask",
				"InsertIssueBudgetEntry",
				"UpdateIssueBudgetRootLedger",
				"InsertIssueBudgetAttempt",
				"InsertIssueBudgetObservation",
				"InsertIssueBudgetObservation",
				"GetIssueBudgetMember",
				"InsertIssueBudgetObservation",
				"InsertIssueBudgetOperation",
			},
		},
	} {
		policy, parent, child := f.budgetTree(t, tc.limit, tc.reserve)
		root := policy.BudgetRootID
		task := f.claimed(t, agent, runtime, child, 1)
		var superseded []string
		if tc.reclaim {
			first := f.mustFinalize(t, f.claims(f.pool), task, TaskClaimBudgetProtocolV1)
			requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, tc.name+": the first claim")
			superseded = []string{first}
			task = f.claimedAgain(t, task)
		}
		if tc.closed {
			closed, err := f.svc.Close(ctx, f.manage(root, 1))
			if err != nil {
				t.Fatalf("%s: close the root: %v", tc.name, err)
			}
			policy = closed
		}
		if tc.deleted {
			f.deleteIssue(t, parent)
		}

		log := &issueBudgetStatementLog{TxStarter: f.pool}
		attempt := f.mustFinalize(t, f.claims(log), task, tc.declared)
		if attempt == "" {
			t.Fatalf("%s: the claim got no attempt", tc.name)
		}
		rows := f.attempts(t, task)
		wantAttempt := issueBudgetWantAttempt{Root: policy, Task: task, Issue: child, State: "open"}
		reservation, reserved := "unsized", ""
		if tc.reserve != 0 {
			wantAttempt.Reserved = issueBudgetInt64(int64(tc.reserve))
			reservation, reserved = "sized", fmt.Sprintf("%d", tc.reserve)
		}
		if tc.declared == TaskClaimBudgetProtocolV1 {
			wantAttempt.Protocol = issueBudgetInt32(1)
		}
		f.requireAttempt(t, rows[len(rows)-1], attempt, wantAttempt, tc.name)

		var observations []issueBudgetClaimObservation
		if tc.gap {
			observations = append(observations, issueBudgetObservationOfClaim("coverage_gap", "legacy_protocol", child, task, attempt, nil))
		}
		var reasons []string
		for _, r := range tc.refuse {
			observations = append(observations, issueBudgetObservationOfClaim("would_refuse", r.reason, child, task, attempt, r.detail))
			reasons = append(reasons, r.reason)
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), observations, tc.name)
		operations := f.claimOperations(t, root)
		f.requireClaimOperation(t, operations[len(operations)-1], issueBudgetWantClaim{
			Root: root, Task: task, Attempt: attempt, Reservation: reservation, Reserved: reserved,
			Superseded: superseded, WouldRefuse: reasons,
		}, tc.name)
		if tc.step != nil {
			requireIssueBudgetStep(t, log.names(), tc.step, tc.name)
		}
	}
}

// F05 (B35a), a write that fails. The budget step is the last thing of the
// finalize transaction, so a write of it that does not go through takes the
// whole finalization with it: the entry and the root figures written just
// before, and the task token written before the step. Nothing of the claim is
// published either. The next claim of the task is a first attempt.
func TestIssueBudgetFinalizeThatFailsInTheBudgetStepLeavesNothing(t *testing.T) {
	f := newIssueBudgetFixture(t)
	enabled, _, child := f.budgetTree(t, 0, 100)
	root := enabled.BudgetRootID
	rootID := util.MustParseUUID(root)
	agent, runtime := f.agent(t)
	task := f.claimed(t, agent, runtime, child, 1)
	// The row the claim is about to write is taken: the insert of the attempt
	// row fails behind the entry row and the root figures.
	planted := f.fx.Insert(t, "issue_budget_attempt", testutil.Cols{
		"id": uuid.NewString(), "workspace_id": f.workspace, "budget_root_id": root,
		"task_id": util.UUIDToString(task.ID), "claim_generation": 1,
		"runtime_id": runtime, "agent_id": agent, "issue_id": child,
	})
	before := f.snapshot(t)

	log := &issueBudgetStatementLog{TxStarter: f.pool}
	finalized, err := f.finalize(t, f.claims(log), task, TaskClaimBudgetProtocolV1)
	var refused *pgconn.PgError
	if err == nil || !errors.As(err, &refused) || refused.Code != "23505" || refused.TableName != "issue_budget_attempt" {
		t.Fatalf("finalize with the attempt row taken = %+v, %v; want the unique violation of the attempt table", finalized, err)
	}
	if errors.Is(err, ErrIssueBudgetBindingInProgress) {
		t.Fatalf("a failed write was reported as a binding in progress: %v", err)
	}
	if finalized.BudgetAttemptID.Valid {
		t.Fatalf("a failed finalization handed back attempt %s", util.UUIDToString(finalized.BudgetAttemptID))
	}
	// The premise of the case: the step got as far as the write that failed,
	// with the entry and the root figures already sent.
	requireIssueBudgetStep(t, log.names(), []string{
		"TryLockIssueBudgetBindingSharedForIssue",
		"GetIssueBudgetMember",
		"GetIssueBudgetTransactionIsolation",
		"LockIssueBudgetRoot",
		"ListIssueBudgetOpenAttemptsOfTask",
		"InsertIssueBudgetEntry",
		"UpdateIssueBudgetRootLedger",
		"InsertIssueBudgetAttempt",
	}, "the finalization that fails")
	f.requireUnchanged(t, before, "the finalization that failed")
	if got := f.tokens(t, task); got != 0 {
		t.Fatalf("the failed finalization left %d task tokens, want 0", got)
	}
	// The cache holds what the claim loaded, the committed ledger without an
	// entry, and nothing of what it appended.
	revision, ledger, ok := issueBudgetLedgers.peek(rootID)
	if !ok || revision != 0 {
		t.Fatalf("after the failed finalization the cache holds (%t, revision %d) for the root, want the committed revision 0", ok, revision)
	}
	requireIssueBudgetLedger(t, ledger, 0, issueBudgetLedgerFigures{}, "the cached ledger after the failed finalization")

	// The test takes its row away again; the task goes back to the queue and
	// is claimed at the next generation.
	f.fx.Exec(t, `DELETE FROM issue_budget_attempt WHERE id=$1`, planted)
	again := f.claimedAgain(t, task)
	attempt := f.mustFinalize(t, f.claims(f.pool), again, TaskClaimBudgetProtocolV1)
	f.requireAttempt(t, f.onlyAttempt(t, again, attempt, "the claim after the failed one"), attempt, issueBudgetWantAttempt{
		Root: enabled, Task: again, Issue: child, State: "open", Reserved: issueBudgetInt64(100), Protocol: issueBudgetInt32(1),
	}, "the claim after the failed one")
	operations := f.claimOperations(t, root)
	if len(operations) != 1 {
		t.Fatalf("the claim after the failed one left %d operation rows, want 1", len(operations))
	}
	f.requireClaimOperation(t, operations[0], issueBudgetWantClaim{
		Root: root, Task: again, Attempt: attempt, Reservation: "sized", Reserved: "100",
	}, "the claim after the failed one")
	entries := f.entryRows(t, root)
	if len(entries) != 1 {
		t.Fatalf("the claim after the failed one left %d entry rows, want 1", len(entries))
	}
	f.requireClaimEntry(t, entries[0], 1, issueBudgetReserveEntry(attempt, 100),
		budget.Receipt{Disposition: budget.DispositionAccepted}, operations[0].ID, "the reservation after the failed claim")
	f.requireRootLedger(t, root, 1, issueBudgetLedgerFigures{ReservedTicks: 100}, "after the claim that followed the failed one")
	if got := f.tokens(t, again); got != 1 {
		t.Fatalf("the claim after the failed one left %d task tokens, want 1", got)
	}
}

// F06 (B45), one claim after another. The limit is judged on what the root
// has committed to with this claim's reservation in it: known spend, what is
// reserved and what is held. Exactly at the limit is inside it; one tick more
// is over. The detail names each of the three amounts, the limit and this
// claim's reservation.
func TestIssueBudgetFinalizeJudgesTheLimitWithItsOwnReservation(t *testing.T) {
	f := newIssueBudgetFixture(t)
	agent, runtime := f.agent(t)
	otherAgent, otherRuntime := f.agent(t)

	// Two claims of 100 each and nothing else: 200 committed, all of it
	// reserved.
	for _, tc := range []struct {
		name   string
		limit  budget.Ticks
		refuse []string
	}{
		{name: "two reservations that reach the limit", limit: 200},
		{name: "two reservations, one tick over the limit", limit: 199, refuse: []string{"limit"}},
	} {
		enabled, _, child := f.budgetTree(t, tc.limit, 100)
		root := enabled.BudgetRootID
		first := f.claimed(t, agent, runtime, child, 1)
		f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), nil, tc.name+": the first claim")
		second := f.claimed(t, otherAgent, otherRuntime, child, 1)
		attempt := f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)

		var want []issueBudgetClaimObservation
		if tc.refuse != nil {
			want = append(want, issueBudgetObservationOfClaim("would_refuse", "limit", child, second, attempt, map[string]string{
				"actual_known_ticks": "0", "reserved_ticks": "200", "unknown_hold_ticks": "0",
				"limit_ticks": "199", "this_reservation": "sized", "this_reservation_ticks": "100",
			}))
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), want, tc.name)
		operations := f.claimOperations(t, root)
		if len(operations) != 2 {
			t.Fatalf("%s: two claims left %d operation rows, want 2", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
			Root: root, Task: second, Attempt: attempt, Reservation: "sized", Reserved: "100", WouldRefuse: tc.refuse,
		}, tc.name)
		f.requireRootLedger(t, root, 2, issueBudgetLedgerFigures{ReservedTicks: 200}, tc.name)
	}

	// All three amounts at once. A first claim reserves 100 and its daemon
	// reports 30 of spend, which leaves 70 reserved. The task is claimed
	// again: the 70 become a hold and the new claim reserves 100. The root
	// has committed to 30 + 100 + 70 = 200. The hold is something the root
	// does not know, which is a reason of its own beside the limit.
	for _, tc := range []struct {
		name   string
		limit  budget.Ticks
		refuse []string
	}{
		{name: "spend, a reservation and a hold that reach the limit", limit: 200, refuse: []string{"unbounded_unknown"}},
		{name: "spend, a reservation and a hold, one tick over the limit", limit: 199, refuse: []string{"unbounded_unknown", "limit"}},
	} {
		enabled, _, child := f.budgetTree(t, tc.limit, 100)
		root := enabled.BudgetRootID
		first := f.claimed(t, agent, runtime, child, 1)
		a1 := f.mustFinalize(t, f.claims(f.pool), first, TaskClaimBudgetProtocolV1)
		usage := issueBudgetUsageEntry(a1, uuid.NewString(), "model-one", budget.Cost{Basis: budget.CostReported, Ticks: 30, Currency: budget.CurrencyUSD})
		(&issueBudgetLedgerRoot{f: f, id: util.MustParseUUID(root)}).write(t, issueBudgetLedgerFigures{ActualKnownTicks: 30, ReservedTicks: 70}, usage)

		second := f.claimedAgain(t, first)
		a2 := f.mustFinalize(t, f.claims(f.pool), second, TaskClaimBudgetProtocolV1)
		f.requireRootLedger(t, root, 4, issueBudgetLedgerFigures{ActualKnownTicks: 30, ReservedTicks: 100, UnknownHoldTicks: 70, UnknownCount: 1}, tc.name)

		want := []issueBudgetClaimObservation{
			issueBudgetObservationOfClaim("would_refuse", "unbounded_unknown", child, second, a2, map[string]string{"ledger.superseded_unproven": "1"}),
		}
		if len(tc.refuse) == 2 {
			want = append(want, issueBudgetObservationOfClaim("would_refuse", "limit", child, second, a2, map[string]string{
				"actual_known_ticks": "30", "reserved_ticks": "100", "unknown_hold_ticks": "70",
				"limit_ticks": "199", "this_reservation": "sized", "this_reservation_ticks": "100",
			}))
		}
		requireIssueBudgetClaimObservations(t, f.claimObservations(t, root), want, tc.name)
		operations := f.claimOperations(t, root)
		if len(operations) != 2 {
			t.Fatalf("%s: two claims left %d operation rows, want 2", tc.name, len(operations))
		}
		f.requireClaimOperation(t, operations[1], issueBudgetWantClaim{
			Root: root, Task: second, Attempt: a2, Reservation: "sized", Reserved: "100",
			Superseded: []string{a1}, WouldRefuse: tc.refuse,
		}, tc.name)
	}
}
