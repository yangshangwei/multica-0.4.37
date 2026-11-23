package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type iterationTaskReadCounter struct {
	TxStarter
	reads                int
	participationReads   int
	participationBatches int
}
type iterationTaskCountTx struct {
	pgx.Tx
	owner *iterationTaskReadCounter
}

func (c *iterationTaskReadCounter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := c.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationTaskCountTx{Tx: tx, owner: c}, nil
}
func (tx *iterationTaskCountTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "agent_task_queue") {
		tx.owner.reads++
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *iterationTaskCountTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: LockIssueIterationParticipation") {
		tx.owner.participationReads++
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *iterationTaskCountTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	for _, q := range b.QueuedQueries {
		if strings.Contains(q.SQL, "-- name: LockIssueIterationParticipation") {
			tx.owner.participationBatches++
			break
		}
	}
	return tx.Tx.SendBatch(ctx, b)
}

func TestIterationPreviewCountsRunningTasksWithOneRead(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, actor := lifecycleIDs(fx)
	iid := lifecycleCreate(t, fx, s)
	runtime := fx.Runtime(t, "Count runtime")
	agent := fx.Agent(t, "Count agent", runtime)
	ids := []string{fx.Issue(t, "First"), fx.Issue(t, "Second"), fx.Issue(t, "No running task")}
	for _, id := range ids {
		lifecycleMove(t, fx, s, id, &iid)
	}
	for index, status := range []string{"running", "running", "queued", "dispatched", "waiting_local_directory", "completed", "failed", "cancelled"} {
		fx.Task(t, fx.Agent(t, fmt.Sprintf("Status count agent %d", index), runtime), testutil.Cols{"issue_id": ids[0], "runtime_id": runtime, "status": status})
	}
	fx.Task(t, agent, testutil.Cols{"issue_id": ids[1], "runtime_id": runtime, "status": "running"})
	unrelated := fx.Issue(t, "Outside selected scope")
	fx.Task(t, agent, testutil.Cols{"issue_id": unrelated, "runtime_id": runtime, "status": "running"})
	counter := &iterationTaskReadCounter{TxStarter: s.TxStarter}
	s.TxStarter = counter
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, issue := range preview.Issues {
		counts[issue.IssueID] = issue.RunningExecutionCount
	}
	if len(counts) != 3 || counts[ids[0]] != 2 || counts[ids[1]] != 1 || counts[ids[2]] != 0 {
		t.Fatalf("running counts include nonrunning or unrelated tasks: %v", counts)
	}
	if counter.reads != 1 {
		t.Fatalf("task reads=%d for three issues; want one set read", counter.reads)
	}
}

func TestIterationRunningTaskCountIsWorkspaceScoped(t *testing.T) {
	fx, _ := lifecycleFixture(t)
	ws, _ := lifecycleIDs(fx)
	runtime := fx.Runtime(t, "Scoped runtime")
	agent := fx.Agent(t, "Scoped agent", runtime)
	issue := fx.Issue(t, "Owned issue")
	fx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running"})
	other := fx.Workspace(t, "Foreign workspace", "counts-foreign-"+issue)
	q := db.New(fx.Pool)
	for _, input := range []db.CountRunningIterationTasksParams{
		{WorkspaceID: util.MustParseUUID(other), IssueIds: []pgtype.UUID{util.MustParseUUID(issue)}},
		{WorkspaceID: ws, IssueIds: []pgtype.UUID{}},
	} {
		counts, err := q.CountRunningIterationTasks(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		if len(counts) != 0 {
			t.Fatalf("foreign workspace or empty selection leaked task counts: %v", counts)
		}
	}
}

func TestIterationDisablePreparesParticipationInOneBatch(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, actor := lifecycleIDs(fx)
	iid := lifecycleCreate(t, fx, s)
	for _, title := range []string{"One", "Two", "Three"} {
		lifecycleMove(t, fx, s, fx.Issue(t, title), &iid)
	}
	counter := &iterationTaskReadCounter{TxStarter: s.TxStarter}
	s.TxStarter = counter
	reason := "Disable all plans"
	draft := iteration.Draft{Operation: "disable", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if counter.participationReads != 0 || counter.participationBatches != 1 {
		t.Fatalf("participation reads=%d batches=%d, want0/1", counter.participationReads, counter.participationBatches)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND current_iteration_id IS NOT NULL", fx.WorkspaceID) != 0 {
		t.Fatal("batched disable left membership behind")
	}
}

func TestIterationBatchPreparationPreservesAbsentAndInactiveParticipation(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	targetID := lifecycleCreate(t, fx, s)
	q := db.New(fx.Pool)
	ws, _ := lifecycleIDs(fx)
	source, err := q.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: ws, ID: util.MustParseUUID(iid)})
	if err != nil {
		t.Fatal(err)
	}
	target, err := q.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: ws, ID: util.MustParseUUID(targetID)})
	if err != nil {
		t.Fatal(err)
	}
	a, b := fx.Issue(t, "A"), fx.Issue(t, "B")
	lifecycleMove(t, fx, s, a, &iid)
	lifecycleMove(t, fx, s, b, &iid)
	inputs := []iteration.MembershipInput{}
	for _, id := range []string{a, b} {
		issue, e := q.GetIssue(t.Context(), util.MustParseUUID(id))
		if e != nil {
			t.Fatal(e)
		}
		inputs = append(inputs, iteration.MembershipInput{Before: issue, Source: source, Target: target})
	}
	fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": ws, "iteration_id": target.ID, "issue_id": inputs[0].Before.ID, "first_joined_at": time.Now().UTC(), "last_left_at": time.Now().UTC()}, "iteration_id=$1 AND issue_id=$2", target.ID, inputs[0].Before.ID)
	for _, tc := range []struct {
		name    string
		mutate  func(pgx.Tx)
		wantErr bool
	}{
		{"missing_target_is_valid", nil, false},
		{"already_current_target", func(tx pgx.Tx) {
			if _, e := tx.Exec(t.Context(), "UPDATE iteration_participation SET current_joined_at=clock_timestamp(),last_left_at=NULL WHERE iteration_id=$1 AND issue_id=$2", target.ID, inputs[0].Before.ID); e != nil {
				t.Fatal(e)
			}
		}, true},
		{"missing_source", func(tx pgx.Tx) {
			if _, e := tx.Exec(t.Context(), "DELETE FROM iteration_participation WHERE issue_id=$1", b); e != nil {
				t.Fatal(e)
			}
		}, true},
		{"inactive_source", func(tx pgx.Tx) {
			if _, e := tx.Exec(t.Context(), "UPDATE iteration_participation SET current_joined_at=NULL,last_left_at=clock_timestamp() WHERE issue_id=$1", b); e != nil {
				t.Fatal(e)
			}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := fx.Pool.Begin(t.Context())
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(t.Context())
			if tc.mutate != nil {
				tc.mutate(tx)
			}
			changes, e := iteration.PrepareMembershipChanges(t.Context(), tx, inputs)
			if (e != nil) != tc.wantErr {
				t.Fatalf("error=%v, expected error=%t", e, tc.wantErr)
			}
			var retained int
			if queryErr := tx.QueryRow(t.Context(), "SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND current_iteration_id=$2", []string{a, b}, iid).Scan(&retained); queryErr != nil {
				t.Fatal(queryErr)
			}
			if retained != 2 {
				t.Fatal("preparation wrote membership before every input was validated")
			}
			if e == nil && len(changes) != 2 {
				t.Fatal("lost aligned membership results")
			}
		})
	}
	if changes, err := iteration.PrepareMembershipChanges(t.Context(), nil, nil); err != nil || len(changes) != 0 {
		t.Fatal("empty preparation must issue no SQL")
	}
	if _, err := iteration.PrepareMembershipChanges(t.Context(), nil, []iteration.MembershipInput{inputs[0], inputs[0]}); err == nil {
		t.Fatal("duplicate issue preparation was accepted")
	}
}

func TestIterationParticipationBatchRetainsCanonicalLockSQL(t *testing.T) {
	read := func(path, name string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		start := strings.Index(text, "-- name: "+name+" ")
		if start < 0 {
			t.Fatal("canonical query missing")
		}
		text = text[start:]
		text = text[strings.Index(text, "\n")+1:]
		return strings.Join(strings.Fields(strings.SplitN(text, ";", 2)[0]), " ")
	}
	single := read("../../pkg/db/queries/iteration.sql", "LockIssueIterationParticipation")
	batch := read("../../pkg/db/queries/iteration_membership_batch.sql", "LockIssueIterationParticipationBatch")
	if single != batch {
		t.Fatalf("batch lock changed canonical query: %s vs %s", single, batch)
	}
}
