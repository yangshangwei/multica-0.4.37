package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"strings"
	"testing"
	"time"
)

func TestIterationClosureEndFreezesAndRollsOver(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source := lifecycleCreate(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	issue := fx.Issue(t, "Frozen title", testutil.Cols{"status": "todo"})
	lifecycleMove(t, fx, s, issue, &source)
	start := lifecycleDraft(t, fx, source, "start")
	start.Start = &iteration.StartDraft{TargetID: source, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, start)
	draft := lifecycleDraft(t, fx, source, "end")
	reason := "Completed review"
	draft.Reason = &reason
	var rev int64
	fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", issue).Scan(&rev)
	draft.Moves = []iteration.Move{{IssueID: issue, ExpectedIssueRevision: rev, ExpectedSourceID: &source, TargetID: &target}}
	lifecycleApply(t, fx, s, draft)
	var status, destination string
	var count int
	fx.QueryRow(t, "SELECT status FROM iteration WHERE id=$1", source).Scan(&status)
	fx.QueryRow(t, "SELECT current_iteration_id,iteration_rollover_count FROM issue WHERE id=$1", issue).Scan(&destination, &count)
	if status != "completed" || destination != target || count != 1 {
		t.Fatalf("closure=%s target=%s count=%d", status, destination, count)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 1 {
		t.Fatal("missing immutable snapshot")
	}
}

func closureStarted(t *testing.T, fx *testutil.Fixture, s *IterationService) (string, string) {
	t.Helper()
	id := lifecycleCreate(t, fx, s)
	issue := fx.Issue(t, "Scope", testutil.Cols{"status": "todo", "assignee_type": "member", "assignee_id": fx.UserID})
	lifecycleMove(t, fx, s, issue, &id)
	d := lifecycleDraft(t, fx, id, "start")
	d.Start = &iteration.StartDraft{TargetID: id, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, d)
	return id, issue
}
func closureDraft(t *testing.T, fx *testutil.Fixture, source, issue, operation string, target *string) iteration.Draft {
	t.Helper()
	d := lifecycleDraft(t, fx, source, operation)
	reason := "Reviewed complete scope"
	d.Reason = &reason
	var rev int64
	fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", issue).Scan(&rev)
	d.Moves = []iteration.Move{{IssueID: issue, ExpectedIssueRevision: rev, ExpectedSourceID: &source, TargetID: target}}
	return d
}
func TestIterationClosureHandoffIncludesExistingAndIncomingOriginal(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	existing := fx.Issue(t, "Existing", testutil.Cols{"status": "todo"})
	lifecycleMove(t, fx, s, existing, &target)
	d := closureDraft(t, fx, source, issue, "handoff", &target)
	d.Start = &iteration.StartDraft{TargetID: target, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, d)
	if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND in_original", target) != 2 {
		t.Fatal("handoff baseline missing existing or incoming scope")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active'", target) != 1 {
		t.Fatal("handoff did not start target")
	}
}
func TestIterationClosureCancelAndDisable(t *testing.T) {
	for _, operation := range []string{"cancel", "disable"} {
		t.Run(operation, func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			source, issue := closureStarted(t, fx, s)
			d := closureDraft(t, fx, source, issue, operation, nil)
			if operation == "disable" {
				for range 102 {
					lifecycleCreate(t, fx, s)
				}
				d.IterationID = nil
				d.ExpectedIterationRevision = nil
				d.ExpectedScopeRevision = nil
				d.Moves = []iteration.Move{}
			}
			out := lifecycleApply(t, fx, s, d)
			if operation == "disable" && (out.Result.SettingsRevision != 2 || fx.Count(t, "SELECT count(*) FROM iteration WHERE workspace_id=$1 AND status IN ('planned','active')", fx.WorkspaceID) != 0) {
				t.Fatal("disable left plans outside first page")
			}
			expected := "cancelled"
			if operation == "disable" {
				expected = "completed"
			}
			var endType string
			fx.QueryRow(t, "SELECT body->>'end_type' FROM iteration_snapshot WHERE iteration_id=$1", source).Scan(&endType)
			if endType != expected || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id IS NOT NULL", issue) != 0 {
				t.Fatalf("close type=%s", endType)
			}
			if operation == "disable" {
				if fx.Count(t, "SELECT count(*) FROM iteration WHERE workspace_id=$1 AND end_reason NOT LIKE '功能禁用%'", fx.WorkspaceID) != 0 {
					t.Fatal("disable omitted explicit end reason")
				}
				if fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE operation_id=$1 AND kind='disable' AND iteration_id IS NULL", out.OperationID) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE operation_id=$1 AND kind IN ('end','cancel')", out.OperationID) != 0 {
					t.Fatal("disable must emit one workspace summary per recipient")
				}
			}
		})
	}
}
func TestIterationClosureRollbackReplayAndFrozenDigest(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	d := closureDraft(t, fx, source, issue, "end", &target)
	ws, actor := lifecycleIDs(fx)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	request := ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}
	s.TxStarter = &lifecycleFailStarter{TxStarter: fx.Pool}
	_, e = s.Apply(t.Context(), ws, actor, request, lifecycleAuth)
	if e == nil {
		t.Fatal("fault injection did not fail")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 0 || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active'", source) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND iteration_rollover_count=0 AND current_iteration_id=$2", issue, source) != 1 {
		t.Fatal("failure left partial closure")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE iteration_id=$1 AND kind='end'", source) != 0 {
		t.Fatal("rolled back closure retained outbox")
	}
	s.TxStarter = fx.Pool
	first, e := s.Apply(t.Context(), ws, actor, request, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Apply(t.Context(), ws, actor, request, lifecycleAuth)
	if e != nil || !again.Replayed || again.OperationID != first.OperationID {
		t.Fatalf("replay %+v %v", again, e)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE iteration_id=$1 AND kind='end'", source) != 1 {
		t.Fatal("replay duplicated or lost outbox")
	}
	var before, after string
	fx.QueryRow(t, "SELECT md5(body::text) FROM iteration_snapshot WHERE iteration_id=$1", source).Scan(&before)
	fx.Exec(t, "UPDATE issue SET title='Later title',status='done' WHERE id=$1", issue)
	fx.Exec(t, "DELETE FROM issue WHERE id=$1", issue)
	fx.QueryRow(t, "SELECT md5(body::text) FROM iteration_snapshot WHERE iteration_id=$1", source).Scan(&after)
	readTx, e := fx.Pool.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer readTx.Rollback(t.Context())
	history, e := iteration.LoadHistory(t.Context(), readTx, ws, util.MustParseUUID(source), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if before != after || history.Scope[0].Title != "Scope" || history.Scope[0].RolloverCount != 0 {
		t.Fatal("closed history drifted with live issue")
	}
}
func TestIterationClosureRejectsMissingAndTerminalMoves(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	d := closureDraft(t, fx, source, issue, "end", nil)
	ws, actor := lifecycleIDs(fx)
	d.Moves = []iteration.Move{}
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil || len(p.InvalidItems) == 0 {
		t.Fatalf("missing R accepted %+v %v", p, e)
	}
}

func TestIterationClosureTargetCancelRace(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	second := util.MustParseUUID(fx.User(t, "Second", uuid.NewString()+"@test.invalid"))
	fx.Member(t, fx.WorkspaceID, util.UUIDToString(second), "member")
	closeDraft := closureDraft(t, fx, source, issue, "end", &target)
	cancel := lifecycleDraft(t, fx, target, "cancel")
	reason := "Cancel target"
	cancel.Reason = &reason
	actors := []pgtype.UUID{actor, second}
	requests := []ApplyIterationInput{}
	for i, d := range []iteration.Draft{closeDraft, cancel} {
		p, e := s.Preview(t.Context(), ws, actors[i], d, lifecycleAuth)
		if e != nil {
			t.Fatal(e)
		}
		requests = append(requests, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash})
	}
	owner, e := fx.Pool.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer owner.Rollback(t.Context())
	if e = iteration.LockWorkspace(t.Context(), owner, ws); e != nil {
		t.Fatal(e)
	}
	entered := make(chan struct{}, 2)
	results := make(chan error, 2)
	for i, r := range requests {
		go func(in ApplyIterationInput, current pgtype.UUID) {
			_, e := s.Apply(t.Context(), ws, current, in, func(context.Context, pgx.Tx) error { entered <- struct{}{}; return nil })
			results <- e
		}(r, actors[i])
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("writer did not reach barrier")
		}
	}
	if e = owner.Commit(t.Context()); e != nil {
		t.Fatal(e)
	}
	successes, conflicts := 0, 0
	for range 2 {
		e := <-results
		if e == nil {
			successes++
		} else {
			var op *iteration.OperationError
			if errors.As(e, &op) && op.Status == 409 {
				conflicts++
			} else {
				t.Fatal(e)
			}
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("race successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestIterationClosureHandoffTerminalAndMidnight(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	terminal := fx.Issue(t, "Terminal plan", testutil.Cols{"status": "todo"})
	lifecycleMove(t, fx, s, terminal, &target)
	fx.Exec(t, "UPDATE issue SET status='done',revision=revision+1 WHERE id=$1", terminal)
	now := time.Now().UTC()
	now = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return now, nil }
	d := closureDraft(t, fx, source, issue, "handoff", &target)
	d.Start = &iteration.StartDraft{TargetID: target, Mode: "today", TerminalChoices: []iteration.TerminalChoice{{IssueID: terminal, Retain: false}}}
	ws, actor := lifecycleIDs(fx)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil || len(p.InvalidItems) > 0 {
		t.Fatalf("preview %v %+v", e, p.InvalidItems)
	}
	now = now.Add(2 * time.Second)
	_, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(e, &op) || op.Code != "iteration_preview_stale" {
		t.Fatalf("midnight accepted: %v", e)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 0 {
		t.Fatal("stale handoff froze source")
	}
	lifecycleApply(t, fx, s, d)
	if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND in_original", target) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id IS NULL", terminal) != 1 {
		t.Fatal("terminal choice not respected")
	}
}
func TestIterationClosureDisableAdminLimitAndRollback(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	reason := "Disable feature"
	d := iteration.Draft{Operation: "disable", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{}}
	member := util.MustParseUUID(fx.User(t, "Member", uuid.NewString()+"@test.invalid"))
	fx.Member(t, fx.WorkspaceID, util.UUIDToString(member), "member")
	_, e := s.Preview(t.Context(), ws, member, d, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(e, &op) || op.Status != 403 {
		t.Fatalf("member disable: %v", e)
	}
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	s.TxStarter = &lifecycleFailStarter{TxStarter: fx.Pool}
	_, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
	if e == nil {
		t.Fatal("expected injected failure")
	}
	s.TxStarter = fx.Pool
	if fx.Count(t, "SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled", fx.WorkspaceID) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2", issue, source) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 0 {
		t.Fatal("failed disable leaked mutation")
	}
	for range MaxIterationOperationIssues {
		fx.Issue(t, "Capacity", testutil.Cols{"current_iteration_id": source})
	}
	_, e = s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if !errors.As(e, &op) || op.Status != 413 {
		t.Fatalf("disable complete set cap: %v", e)
	}
	if fx.Count(t, "SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled", fx.WorkspaceID) != 1 {
		t.Fatal("oversized disable changed settings")
	}
}

func closureSetStatus(t *testing.T, fx *testutil.Fixture, issue, status string) {
	t.Helper()
	ctx := t.Context()
	ws, actor := lifecycleIDs(fx)
	id := util.MustParseUUID(issue)
	tx, e := fx.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = iteration.LockWorkspace(ctx, tx, ws); e != nil {
		t.Fatal(e)
	}
	row, e := iteration.LockIssueIteration(ctx, tx, ws, id)
	if e != nil {
		t.Fatal(e)
	}
	q := db.New(tx)
	before, e := q.GetIssue(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	record, e := iteration.PrepareIssueRecord(ctx, tx, before, row)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE issue SET status=$1,revision=revision+1 WHERE id=$2", status, id); e != nil {
		t.Fatal(e)
	}
	after, e := q.GetIssue(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	if e = iteration.RecordIssueChange(ctx, tx, record, after, iterationActor(actor), util.MustParseUUID(uuid.NewString())); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestIterationClosureCompletionAfterPreviewAndTerminalRelease(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	d := closureDraft(t, fx, source, issue, "end", &target)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	closureSetStatus(t, fx, issue, "done")
	_, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(e, &op) || op.Code != "iteration_preview_stale" {
		t.Fatalf("late completion accepted: %v", e)
	}
	d = lifecycleDraft(t, fx, source, "end")
	reason := "Release completed scope"
	d.Reason = &reason
	lifecycleApply(t, fx, s, d)
	var body []byte
	fx.QueryRow(t, "SELECT body FROM iteration_snapshot WHERE iteration_id=$1", source).Scan(&body)
	var snapshot iteration.Snapshot
	if e = json.Unmarshal(body, &snapshot); e != nil {
		t.Fatal(e)
	}
	if snapshot.Destinations[0].TargetIterationID != nil || snapshot.Destinations[0].RolloverCountAfter != 0 || snapshot.Statistics.Completed != 1 {
		t.Fatalf("bad terminal close: %+v", snapshot)
	}
}

func TestIterationClosureDisableReplayRechecksAdministrator(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, actor := lifecycleIDs(fx)
	reason := "Disable"
	d := iteration.Draft{Operation: "disable", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{}}
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	in := ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}
	if _, e = s.Apply(t.Context(), ws, actor, in, lifecycleAuth); e != nil {
		t.Fatal(e)
	}
	fx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", ws, actor)
	_, e = s.Apply(t.Context(), ws, actor, in, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(e, &op) || op.Status != 403 {
		t.Fatalf("demoted admin replay: %v", e)
	}
}

type closureSnapshotFailStarter struct {
	TxStarter
	writes int
}
type closureSnapshotFailTx struct {
	pgx.Tx
	owner *closureSnapshotFailStarter
}

func (s *closureSnapshotFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return &closureSnapshotFailTx{Tx: tx, owner: s}, nil
}
func (tx *closureSnapshotFailTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: SetIssueCurrentIteration") {
		tx.owner.writes++
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *closureSnapshotFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: InsertIterationSnapshot") {
		return pgconn.CommandTag{}, errors.New("injected final snapshot failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestIterationClosureFinalSnapshotFailureRollsBackHandoff(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	d := closureDraft(t, fx, source, issue, "handoff", &target)
	d.Start = &iteration.StartDraft{TargetID: target, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	fail := &closureSnapshotFailStarter{TxStarter: fx.Pool}
	s.TxStarter = fail
	_, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
	if e == nil || !strings.Contains(e.Error(), "injected final snapshot failure") || fail.writes != 1 {
		t.Fatalf("snapshot must persist after final ownership writes: writes=%d err=%v", fail.writes, e)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 0 || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active'", source) != 1 || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='planned'", target) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=0", issue, source) != 1 {
		t.Fatal("final snapshot failure left handoff mutations")
	}
}

func TestIterationClosurePreservesSampledClockAndFinalProcessingTime(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	var started time.Time
	fx.QueryRow(t, "SELECT started_at FROM iteration WHERE id=$1", source).Scan(&started)
	sampled := started.Add(-time.Minute)
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return sampled, nil }
	d := closureDraft(t, fx, source, issue, "end", nil)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) {
		calls++
		if calls == 1 {
			return sampled, nil
		}
		return started.Add(time.Minute), nil
	}
	if _, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth); e != nil {
		t.Fatal(e)
	}
	var actualSample, occurred, processed, logical time.Time
	fx.QueryRow(t, "SELECT sampled_at,occurred_at FROM iteration_event WHERE iteration_id=$1 AND kind='end'", source).Scan(&actualSample, &occurred)
	fx.QueryRow(t, "SELECT logical_ended_at,processed_at FROM iteration WHERE id=$1", source).Scan(&logical, &processed)
	if !actualSample.Equal(sampled) || occurred.Before(started) || logical.Before(started) || !processed.Equal(started.Add(time.Minute)) {
		t.Fatalf("sample=%v occurred=%v logical=%v processed=%v", actualSample, occurred, logical, processed)
	}
}

func TestIterationClosureBatchMultiTargetHandoff(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	other := fx.Issue(t, "Second scope", testutil.Cols{"status": "todo"})
	lifecycleMove(t, fx, s, other, &source)
	a := lifecycleCreate(t, fx, s)
	b := lifecycleCreate(t, fx, s)
	d := closureDraft(t, fx, source, issue, "handoff", &a)
	var rev int64
	fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", other).Scan(&rev)
	d.Moves = append(d.Moves, iteration.Move{IssueID: other, ExpectedIssueRevision: rev, ExpectedSourceID: &source, TargetID: &b})
	d.Start = &iteration.StartDraft{TargetID: a, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, d)
	if fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=1", issue, a) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=1", other, b) != 1 {
		t.Fatal("multi target batch lost destinations or counters")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active'", a) != 1 || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='planned'", b) != 1 {
		t.Fatal("handoff started wrong set")
	}
}
func TestIterationLifecycleBatchSwappedSources(t *testing.T) {
	fx, s := lifecycleFixture(t)
	a := lifecycleCreate(t, fx, s)
	b := lifecycleCreate(t, fx, s)
	x := fx.Issue(t, "X")
	y := fx.Issue(t, "Y")
	lifecycleMove(t, fx, s, x, &a)
	lifecycleMove(t, fx, s, y, &b)
	reason := "Swap scope"
	d := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{{IssueID: x, ExpectedIssueRevision: 2, ExpectedSourceID: &a, TargetID: &b}, {IssueID: y, ExpectedIssueRevision: 2, ExpectedSourceID: &b, TargetID: &a}}}
	lifecycleApply(t, fx, s, d)
	if fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2", x, b) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2", y, a) != 1 {
		var currentX, currentY string
		fx.QueryRow(t, "SELECT current_iteration_id FROM issue WHERE id=$1", x).Scan(&currentX)
		fx.QueryRow(t, "SELECT current_iteration_id FROM issue WHERE id=$1", y).Scan(&currentY)
		t.Fatalf("swapped batch lost membership: X=%s want %s Y=%s want %s", currentX, b, currentY, a)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND current_joined_at IS NOT NULL", a) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND current_joined_at IS NOT NULL", b) != 1 {
		t.Fatal("swapped batch retained source participation")
	}
}

type closureBatchCASFailStarter struct{ TxStarter }
type closureBatchCASFailTx struct{ pgx.Tx }

func (s closureBatchCASFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return closureBatchCASFailTx{tx}, nil
}
func (tx closureBatchCASFailTx) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	count := 0
	for _, q := range batch.QueuedQueries {
		if strings.Contains(q.SQL, "-- name: SetIssueCurrentIterationBatch") {
			count++
			if count == 2 {
				q.SQL = strings.Replace(q.SQL, "RETURNING", "AND FALSE RETURNING", 1)
			}
		}
	}
	return tx.Tx.SendBatch(ctx, batch)
}
func TestIterationClosureBatchCASFailureRollsBackAllPhases(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	other := fx.Issue(t, "Second")
	lifecycleMove(t, fx, s, other, &source)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	d := closureDraft(t, fx, source, issue, "end", &target)
	var rev int64
	fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", other).Scan(&rev)
	d.Moves = append(d.Moves, iteration.Move{IssueID: other, ExpectedIssueRevision: rev, ExpectedSourceID: &source, TargetID: &target})
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	s.TxStarter = closureBatchCASFailStarter{fx.Pool}
	_, e = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(e, &op) || op.Code != "iteration_preview_stale" {
		t.Fatalf("CAS fault result: %v", e)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1 AND iteration_rollover_count=0", source) != 2 || fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", target) != 0 || fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 0 || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active'", source) != 1 {
		t.Fatal("batch CAS rollback leaked earlier phases")
	}
}

func TestIterationLifecycleNormalizeDoesNotMutateCaller(t *testing.T) {
	a := "11111111-1111-4111-8111-111111111111"
	b := "22222222-2222-4222-8222-222222222222"
	reason := "Swap"
	d := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{{IssueID: "44444444-4444-4444-8444-444444444444", ExpectedIssueRevision: 1, ExpectedSourceID: &a, TargetID: &b}, {IssueID: "33333333-3333-4333-8333-333333333333", ExpectedIssueRevision: 1, ExpectedSourceID: &b, TargetID: &a}}}
	before, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	normalized, e := normalizeLifecycleDraft(d)
	if e != nil {
		t.Fatal(e)
	}
	after, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	if string(before) != string(after) || normalized.Moves[0].IssueID != d.Moves[1].IssueID {
		t.Fatal("canonical sort mutated caller or did not sort normalized copy")
	}
}
func TestIterationClosureDateNoticeSkipsRevokedRecipients(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	member := fx.User(t, "Departed", uuid.NewString()+"@test.invalid")
	fx.Member(t, fx.WorkspaceID, member, "member")
	fx.Exec(t, "UPDATE iteration SET coordinator_user_id=$1 WHERE id=$2", member, iid)
	issue := fx.Issue(t, "Assigned", testutil.Cols{"assignee_type": "member", "assignee_id": member})
	lifecycleMove(t, fx, s, issue, &iid)
	fx.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, member)
	row, e := db.New(fx.Pool).GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: ws, ID: util.MustParseUUID(iid)})
	if e != nil {
		t.Fatal(e)
	}
	date, _ := json.Marshal(row.EndDate.Time.AddDate(0, 0, 1).Format(time.DateOnly))
	out, e := s.Edit(t.Context(), ws, actor, row.ID, EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: row.Revision, Fields: map[string]json.RawMessage{"end_date": date}}, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	fx.Member(t, fx.WorkspaceID, member, "member")
	if fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE operation_id=$1 AND recipient_user_id=$2", out.OperationID, member) != 0 {
		t.Fatal("date edit recreated revoked recipient notification")
	}
}

type closureLostCommitStarter struct{ TxStarter }
type closureLostCommitTx struct{ pgx.Tx }

func (s closureLostCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return closureLostCommitTx{tx}, nil
}
func (tx closureLostCommitTx) Commit(ctx context.Context) error {
	if e := tx.Tx.Commit(ctx); e != nil {
		return e
	}
	return errors.New("injected lost commit response")
}
func TestIterationClosureLostCommitResponseRecoversOriginalOperation(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	d := closureDraft(t, fx, source, issue, "end", &target)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	requestID := uuid.NewString()
	in := ApplyIterationInput{RequestID: requestID, Draft: d, PreviewHash: p.PreviewHash}
	s.TxStarter = closureLostCommitStarter{fx.Pool}
	_, e = s.Apply(t.Context(), ws, actor, in, lifecycleAuth)
	if e == nil || !strings.Contains(e.Error(), "lost commit response") {
		t.Fatalf("missing uncertain result: %v", e)
	}
	s.TxStarter = fx.Pool
	tx, e := fx.Pool.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := iteration.ReadOperation(t.Context(), tx, ws, actor, util.MustParseUUID(requestID))
	_ = tx.Rollback(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	replay, e := s.Apply(t.Context(), ws, actor, in, lifecycleAuth)
	if e != nil || !replay.Replayed || replay.OperationID != recovered.OperationID {
		t.Fatalf("recovery %+v %+v %v", recovered, replay, e)
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", source) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='end'", source) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE iteration_id=$1 AND kind='end'", source) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND iteration_rollover_count=1 AND current_iteration_id=$2", issue, target) != 1 {
		t.Fatal("lost response recovery repeated committed side effects")
	}
}

type closureTaskBarrierStarter struct {
	TxStarter
	entered chan struct{}
	release chan struct{}
}
type closureTaskBarrierTx struct {
	pgx.Tx
	owner *closureTaskBarrierStarter
}

func (s *closureTaskBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return &closureTaskBarrierTx{Tx: tx, owner: s}, nil
}
func (tx *closureTaskBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: CompleteAgentTask") {
		tx.owner.entered <- struct{}{}
		select {
		case <-tx.owner.release:
		case <-ctx.Done():
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func TestIterationClosureDaemonCompletionBarrierDoesNotRestartExecution(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	target := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	runtime := fx.Runtime(t, "Completion runtime")
	agent := fx.Agent(t, "Completion agent", runtime)
	taskID := fx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "started_at": time.Now().UTC()})
	d := closureDraft(t, fx, source, issue, "end", &target)
	p, e := s.Preview(t.Context(), ws, actor, d, lifecycleAuth)
	if e != nil {
		t.Fatal(e)
	}
	if p.Issues[0].RunningExecutionCount != 1 {
		t.Fatal("fixture not running")
	}
	enteredClose := make(chan struct{}, 1)
	releaseClose := make(chan struct{})
	calls := 0
	s.Now = func(ctx context.Context, tx pgx.Tx) (time.Time, error) {
		calls++
		if calls == 1 {
			enteredClose <- struct{}{}
			select {
			case <-releaseClose:
			case <-ctx.Done():
				return time.Time{}, ctx.Err()
			}
		}
		return time.Now().UTC(), nil
	}
	barrier := &closureTaskBarrierStarter{TxStarter: fx.Pool, entered: make(chan struct{}, 1), release: make(chan struct{})}
	taskSvc := &TaskService{Queries: db.New(fx.Pool), TxStarter: barrier, Bus: events.New()}
	completion := make(chan error, 1)
	closed := make(chan error, 1)
	go func() {
		_, e := taskSvc.CompleteTask(t.Context(), util.MustParseUUID(taskID), []byte(`{}`), "", "", "", false, "", "")
		completion <- e
	}()
	go func() {
		_, e := s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: d, PreviewHash: p.PreviewHash}, lifecycleAuth)
		closed <- e
	}()
	for _, entered := range []chan struct{}{barrier.entered, enteredClose} {
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
			t.Fatal("both real transactions did not reach barrier")
		}
	}
	close(barrier.release)
	select {
	case e := <-completion:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("completion blocked on management")
	}
	close(releaseClose)
	select {
	case e := <-closed:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("closure failed to resume")
	}
	if fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issue) != 1 || fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='completed'", taskID) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND status='todo' AND iteration_rollover_count=1", issue, target) != 1 {
		t.Fatal("management interfered with execution or inferred issue completion")
	}
}

func TestIterationClosureManagementPreservesRunningTask(t *testing.T) {
	for _, operation := range []string{"end", "cancel", "handoff", "disable"} {
		t.Run(operation, func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			source := lifecycleCreate(t, fx, s)
			target := lifecycleCreate(t, fx, s)
			runtime := fx.Runtime(t, "Live runtime")
			agent := fx.Agent(t, "Live agent", runtime)
			project := fx.Project(t, "Unchanged project")
			issue := fx.Issue(t, "Running scope", testutil.Cols{"status": "blocked", "assignee_type": "agent", "assignee_id": agent, "project_id": project})
			task := fx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "session_id": "live-session", "work_dir": "/tmp/iteration-preserved-workdir", "started_at": testutil.Raw("clock_timestamp()")})
			lifecycleMove(t, fx, s, issue, &source)
			start := lifecycleDraft(t, fx, source, "start")
			start.Start = &iteration.StartDraft{TargetID: source, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
			lifecycleApply(t, fx, s, start)
			q := db.New(fx.Pool)
			before, e := q.GetAgentTask(t.Context(), util.MustParseUUID(task))
			if e != nil {
				t.Fatal(e)
			}
			d := closureDraft(t, fx, source, issue, operation, &target)
			if operation == "cancel" {
				d.Moves[0].TargetID = nil
			}
			if operation == "handoff" {
				d.Start = &iteration.StartDraft{TargetID: target, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
			}
			if operation == "disable" {
				d.IterationID = nil
				d.ExpectedIterationRevision = nil
				d.ExpectedScopeRevision = nil
				d.Moves = []iteration.Move{}
			}
			lifecycleApply(t, fx, s, d)
			after, e := q.GetAgentTask(t.Context(), util.MustParseUUID(task))
			if e != nil {
				t.Fatal(e)
			}
			old, _ := json.Marshal(before)
			next, _ := json.Marshal(after)
			if string(old) != string(next) || fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issue) != 1 || fx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND status='blocked' AND project_id=$2 AND assignee_type='agent' AND assignee_id=$3", issue, project, agent) != 1 {
				t.Fatal("management altered execution identity/status, restarted a task, or changed project/status/assignee")
			}
		})
	}
}
func TestIterationClosureNanosecondInjectedClockRemainsReadable(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source, issue := closureStarted(t, fx, s)
	ws, _ := lifecycleIDs(fx)
	now := time.Now().UTC().Add(time.Minute)
	now = time.Unix(now.Unix(), 123456789).UTC()
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return now, nil }
	lifecycleApply(t, fx, s, closureDraft(t, fx, source, issue, "end", nil))
	tx, e := fx.Pool.Begin(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(t.Context())
	history, e := iteration.LoadHistory(t.Context(), tx, ws, util.MustParseUUID(source), now)
	if e != nil {
		t.Fatal(e)
	}
	if history.Snapshot == nil || history.Snapshot.ProcessedAt.Nanosecond()%1000 != 0 {
		t.Fatal("snapshot precision differs from PostgreSQL rows")
	}
}

func TestIterationClosureRolloverEventsMatchCommittedCounters(t *testing.T) {
	for _, operation := range []string{"end", "handoff"} {
		for _, size := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/%d", operation, size), func(t *testing.T) {
				fx, s := lifecycleFixture(t)
				source, first := closureStarted(t, fx, s)
				target := lifecycleCreate(t, fx, s)
				ids := []string{first}
				if size == 2 {
					second := fx.Issue(t, "Second rollover", testutil.Cols{"iteration_rollover_count": 3})
					lifecycleMove(t, fx, s, second, &source)
					ids = append(ids, second)
				}
				d := closureDraft(t, fx, source, first, operation, &target)
				if size == 2 {
					var rev int64
					fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", ids[1]).Scan(&rev)
					d.Moves = append(d.Moves, iteration.Move{IssueID: ids[1], ExpectedIssueRevision: rev, ExpectedSourceID: &source, TargetID: &target})
				}
				if operation == "handoff" {
					d.Start = &iteration.StartDraft{TargetID: target, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
				}
				out := lifecycleApply(t, fx, s, d)
				for index, id := range ids {
					old := 0
					if index == 1 {
						old = 3
					}
					var sourceCount, targetCount, liveCount int
					fx.QueryRow(t, "SELECT (before_facts->>'rollover_count')::int FROM iteration_event WHERE operation_id=$1 AND iteration_id=$2 AND issue_id=$3 AND kind='leave'", out.OperationID, source, id).Scan(&sourceCount)
					fx.QueryRow(t, "SELECT (after_facts->>'rollover_count')::int FROM iteration_event WHERE operation_id=$1 AND iteration_id=$2 AND issue_id=$3 AND kind='planned_activity'", out.OperationID, target, id).Scan(&targetCount)
					fx.QueryRow(t, "SELECT iteration_rollover_count FROM issue WHERE id=$1", id).Scan(&liveCount)
					if sourceCount != old || targetCount != old+1 || liveCount != old+1 {
						t.Errorf("source=%d target=%d live=%d want old=%d new=%d", sourceCount, targetCount, liveCount, old, old+1)
					}
				}
			})
		}
	}
}
