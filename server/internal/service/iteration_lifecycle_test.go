package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func lifecycleFixture(t *testing.T) (*testutil.Fixture, *IterationService) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires isolated DATABASE_URL")
	}
	pool := newTaskClaimRacePool(t)
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	fx.UserID = fx.User(t, "Lifecycle", suffix+"@test.invalid")
	fx.WorkspaceID = fx.Workspace(t, "Lifecycle", suffix)
	// Existing lifecycle scenarios use UTC dates independently of the workspace default.
	fx.Exec(t, "UPDATE workspace SET planning_timezone='UTC' WHERE id=$1", fx.WorkspaceID)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	fx.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": fx.WorkspaceID, "enabled": true}, "workspace_id=$1", fx.WorkspaceID)
	for _, table := range []string{"iteration_snapshot", "iteration_notification", "iteration", "iteration_participation", "iteration_event", "iteration_operation"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE workspace_id=$1", fx.WorkspaceID)
	}
	return fx, &IterationService{TxStarter: pool, AuthorizeIssues: func(context.Context, pgx.Tx, []db.Issue) error { return nil }}
}
func lifecycleAuth(context.Context, pgx.Tx) error { return nil }
func lifecycleIDs(fx *testutil.Fixture) (pgtype.UUID, pgtype.UUID) {
	return util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(fx.UserID)
}
func lifecycleCreate(t *testing.T, fx *testutil.Fixture, s *IterationService) string {
	t.Helper()
	ws, actor := lifecycleIDs(fx)
	out, err := s.Create(t.Context(), ws, actor, CreateIterationInput{RequestID: uuid.NewString(), Name: "Sprint", StartDate: time.Now().UTC().Format(time.DateOnly), EndDate: time.Now().UTC().AddDate(0, 0, 13).Format(time.DateOnly), ConfirmedTimezone: "UTC"}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	return out.IterationIDs[0]
}
func lifecycleDraft(t *testing.T, fx *testutil.Fixture, iid, operation string) iteration.Draft {
	t.Helper()
	row, err := db.New(fx.Pool).GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: util.MustParseUUID(fx.WorkspaceID), ID: util.MustParseUUID(iid)})
	if err != nil {
		t.Fatal(err)
	}
	return iteration.Draft{Operation: operation, IterationID: &iid, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 1, Moves: []iteration.Move{}}
}
func lifecycleApply(t *testing.T, fx *testutil.Fixture, s *IterationService, draft iteration.Draft) iteration.WriteResult {
	t.Helper()
	ws, actor := lifecycleIDs(fx)
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.InvalidItems) > 0 {
		t.Fatalf("invalid preview: %+v", preview.InvalidItems)
	}
	out, err := s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), PreviewHash: preview.PreviewHash, Draft: draft}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func lifecycleMove(t *testing.T, fx *testutil.Fixture, s *IterationService, issueID string, target *string) {
	t.Helper()
	row, err := db.New(fx.Pool).GetIssue(t.Context(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	var source *string
	if row.CurrentIterationID.Valid {
		v := util.UUIDToString(row.CurrentIterationID)
		source = &v
	}
	reason := "Move to chosen scope"
	lifecycleApply(t, fx, s, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{{IssueID: issueID, ExpectedIssueRevision: row.Revision, ExpectedSourceID: source, TargetID: target}}})
}
func TestIterationLifecycleCreateReplayAndSavedTimezone(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, actor := lifecycleIDs(fx)
	in := CreateIterationInput{RequestID: uuid.NewString(), Name: "  Sprint  ", StartDate: "2026-10-10", EndDate: "2026-10-23", ConfirmedTimezone: "UTC"}
	out, err := s.Create(t.Context(), ws, actor, in, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE workspace SET planning_timezone='Asia/Shanghai' WHERE id=$1", fx.WorkspaceID)
	again, err := s.Create(t.Context(), ws, actor, in, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.OperationID != out.OperationID {
		t.Fatalf("not exact replay: %+v", again)
	}
	var name, zone string
	fx.QueryRow(t, "SELECT name,timezone FROM iteration WHERE id=$1", out.IterationIDs[0]).Scan(&name, &zone)
	if name != "Sprint" || zone != "UTC" {
		t.Fatalf("saved metadata %q %q", name, zone)
	}
	in.Name = "Different"
	_, err = s.Create(t.Context(), ws, actor, in, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(err, &op) || op.Code != "idempotency_conflict" {
		t.Fatalf("payload conflict: %v", err)
	}
}
func TestIterationLifecycleStartOriginalAndReentry(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	issueID := fx.Issue(t, "Original", testutil.Cols{"status": "todo", "iteration_rollover_count": 2})
	lifecycleMove(t, fx, s, issueID, &iid)
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, draft)
	var original []byte
	fx.QueryRow(t, "SELECT original_facts FROM iteration_participation WHERE iteration_id=$1", iid).Scan(&original)
	var captured iteration.HistoricalIssue
	if err := json.Unmarshal(original, &captured); err != nil {
		t.Fatal(err)
	}
	if captured.Title != "Original" || captured.IssueID != issueID || captured.Identifier == "" || captured.RolloverCount != 2 {
		t.Fatalf("missing original capture: %s", original)
	}
	lifecycleMove(t, fx, s, issueID, nil)
	lifecycleMove(t, fx, s, issueID, &iid)
	var originalAfter []byte
	var rollover int
	fx.QueryRow(t, "SELECT original_facts FROM iteration_participation WHERE iteration_id=$1", iid).Scan(&originalAfter)
	fx.QueryRow(t, "SELECT iteration_rollover_count FROM issue WHERE id=$1", issueID).Scan(&rollover)
	if string(original) != string(originalAfter) || rollover != 2 {
		t.Fatal("reentry rewrote original or rollover")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='reenter'", iid) != 1 {
		t.Fatal("missing one reentry")
	}
}
func TestIterationLifecyclePreviewRejectsStaleBatch(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	a := fx.Issue(t, "A")
	b := fx.Issue(t, "B")
	ws, actor := lifecycleIDs(fx)
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: []iteration.Move{{IssueID: a, ExpectedIssueRevision: 1, TargetID: &iid}, {IssueID: b, ExpectedIssueRevision: 1, TargetID: &iid}}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if preview.TotalAffected != 2 || !preview.Complete {
		t.Fatalf("incomplete preview: %+v", preview)
	}
	fx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", b)
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(err, &op) || op.Code != "iteration_preview_stale" {
		t.Fatalf("expected stale: %v", err)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 0 || fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", iid) != 0 {
		t.Fatal("partial move committed")
	}
}

func TestIterationLifecycleEditGuardsAndDeleteReplay(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	in := EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: 1, Fields: map[string]json.RawMessage{"name": json.RawMessage(`"Renamed"`)}}
	if _, err := s.Edit(t.Context(), ws, actor, util.MustParseUUID(iid), in, lifecycleAuth); err != nil {
		t.Fatal(err)
	}
	var auditedRevision int64
	fx.QueryRow(t, "SELECT (after_facts->>'revision')::bigint FROM iteration_event WHERE iteration_id=$1 AND kind='edit'", iid).Scan(&auditedRevision)
	if auditedRevision != 2 {
		t.Fatalf("metadata audit has stale revision %d", auditedRevision)
	}
	draft := lifecycleDraft(t, fx, iid, "delete")
	p, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	request := ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: p.PreviewHash}
	out, err := s.Apply(t.Context(), ws, actor, request, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Apply(t.Context(), ws, actor, request, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Result.Deleted || !again.Replayed || again.OperationID != out.OperationID || fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1", iid) != 0 {
		t.Fatal("delete or durable replay failed")
	}
}

func TestIterationLifecycleUsedPlanCannotDeleteAndCancelReleasesAll(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	a := fx.Issue(t, "A")
	lifecycleMove(t, fx, s, a, &iid)
	lifecycleMove(t, fx, s, a, nil)
	ws, actor := lifecycleIDs(fx)
	p, err := s.Preview(t.Context(), ws, actor, lifecycleDraft(t, fx, iid, "delete"), lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.InvalidItems) != 1 || p.InvalidItems[0].Code != "iteration_delete_requires_unused_plan" {
		t.Fatalf("used plan deletion allowed: %+v", p.InvalidItems)
	}
	lifecycleMove(t, fx, s, a, &iid)
	b := fx.Issue(t, "B")
	lifecycleMove(t, fx, s, b, &iid)
	draft := lifecycleDraft(t, fx, iid, "cancel")
	reason := "Plans changed"
	draft.Reason = &reason
	lifecycleApply(t, fx, s, draft)
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 0 || fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND current_joined_at IS NOT NULL", iid) != 0 {
		t.Fatal("cancel left a partial scope")
	}
	if fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='cancelled' AND started_at IS NULL", iid) != 1 || fx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", iid) != 0 {
		t.Fatal("planned cancel invented active history")
	}
}

func TestIterationLifecycleTerminalChoicesAndActiveDateGuards(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	done := fx.Issue(t, "Done later")
	cancelled := fx.Issue(t, "Cancelled later")
	lifecycleMove(t, fx, s, done, &iid)
	lifecycleMove(t, fx, s, cancelled, &iid)
	fx.Exec(t, "UPDATE issue SET status='done',revision=revision+1 WHERE id=$1", done)
	fx.Exec(t, "UPDATE issue SET status='cancelled',revision=revision+1 WHERE id=$1", cancelled)
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	ws, actor := lifecycleIDs(fx)
	p, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.InvalidItems) != 2 {
		t.Fatalf("missing terminal choices: %+v", p.InvalidItems)
	}
	draft.Start.TerminalChoices = []iteration.TerminalChoice{{IssueID: done, Retain: true}, {IssueID: cancelled, Retain: false}}
	lifecycleApply(t, fx, s, draft)
	if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND in_original", iid) != 1 {
		t.Fatal("wrong original membership")
	}
	var completed bool
	fx.QueryRow(t, "SELECT (original_facts->>'was_completed_at_start')::boolean FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2", iid, done).Scan(&completed)
	if !completed {
		t.Fatal("retained completion not tagged")
	}
	var revision int64
	fx.QueryRow(t, "SELECT revision FROM iteration WHERE id=$1", iid).Scan(&revision)
	for _, field := range []string{"start_date", "end_date"} {
		raw, _ := json.Marshal(time.Now().UTC().AddDate(0, 0, 30).Format(time.DateOnly))
		_, err = s.Edit(t.Context(), ws, actor, util.MustParseUUID(iid), EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: revision, Fields: map[string]json.RawMessage{field: raw}}, lifecycleAuth)
		var op *iteration.OperationError
		if !errors.As(err, &op) || op.Status != 422 {
			t.Fatalf("unguarded %s: %v", field, err)
		}
	}
}

func TestIterationLifecycleNoopAndNewActiveAdmission(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	a := fx.Issue(t, "A", testutil.Cols{"status": "todo"})
	lifecycleMove(t, fx, s, a, &iid)
	before := fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iid)
	lifecycleMove(t, fx, s, a, &iid)
	if fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iid) != before {
		t.Fatal("same-target noop manufactured events")
	}
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, draft)
	for _, state := range []string{"pending", "rejected", "duplicate"} {
		issue := fx.Issue(t, state, testutil.Cols{"admission_status": state})
		ws, actor := lifecycleIDs(fx)
		preview, err := s.Preview(t.Context(), ws, actor, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: 1, TargetID: &iid}}}, lifecycleAuth)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.InvalidItems) != 1 || preview.InvalidItems[0].Code != "triage_review_required" {
			t.Fatalf("admission %s accepted: %+v", state, preview.InvalidItems)
		}
	}
}

func TestIterationLifecycleClockMidnightAndEmptyBaseline(t *testing.T) {
	for _, tc := range []struct {
		name, zone string
		hour       int
		configured bool
	}{
		{"explicit UTC", "UTC", 23, true},
		{"default Shanghai", "Asia/Shanghai", 15, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			if !tc.configured {
				fx.Exec(t, "UPDATE workspace SET planning_timezone=NULL WHERE id=$1", fx.WorkspaceID)
			}
			now := time.Date(2026, 10, 8, tc.hour, 59, 59, 0, time.UTC)
			s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return now, nil }
			ws, actor := lifecycleIDs(fx)
			input := CreateIterationInput{RequestID: uuid.NewString(), Name: "Midnight", StartDate: "2026-10-10", EndDate: "2026-10-23", ConfirmedTimezone: tc.zone}
			created, err := s.Create(t.Context(), ws, actor, input, lifecycleAuth)
			if err != nil {
				t.Fatal(err)
			}
			iid := created.IterationIDs[0]
			// A later workspace setting must not rewrite the saved iteration timezone.
			fx.Exec(t, "UPDATE workspace SET planning_timezone='America/New_York' WHERE id=$1", fx.WorkspaceID)
			draft := lifecycleDraft(t, fx, iid, "start")
			draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "today", TerminalChoices: []iteration.TerminalChoice{}}
			preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
			if err != nil {
				t.Fatal(err)
			}
			if preview.StartPreview.Timezone != tc.zone || preview.StartPreview.EffectiveStartDate != "2026-10-08" || preview.StartPreview.EffectiveEndDate != "2026-10-21" {
				t.Fatalf("wrong calendar duration or saved timezone: %+v", preview.StartPreview)
			}
			now = now.Add(2 * time.Second)
			_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), PreviewHash: preview.PreviewHash, Draft: draft}, lifecycleAuth)
			var op *iteration.OperationError
			if !errors.As(err, &op) || op.Code != "iteration_preview_stale" {
				t.Fatalf("midnight accepted: %v", err)
			}
			lifecycleApply(t, fx, s, draft)
			if fx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status='active' AND start_date='2026-10-09' AND end_date='2026-10-22' AND timezone=$2", iid, tc.zone) != 1 {
				t.Fatal("today shift not committed with saved timezone")
			}
		})
	}
}

func TestIterationLifecycleAtomicCurrentAuthorizationAndReplay(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	a := fx.Issue(t, "A")
	b := fx.Issue(t, "B")
	ws, actor := lifecycleIDs(fx)
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: []iteration.Move{{IssueID: a, ExpectedIssueRevision: 1, TargetID: &iid}, {IssueID: b, ExpectedIssueRevision: 1, TargetID: &iid}}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	s.AuthorizeIssues = func(_ context.Context, _ pgx.Tx, issues []db.Issue) error {
		if len(issues) != 2 {
			t.Fatal("authorization did not receive complete set")
		}
		return iterationFailure(403, "forbidden", "Grant revoked")
	}
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(err, &op) || op.Status != 403 {
		t.Fatalf("authority bypassed: %v", err)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 0 {
		t.Fatal("unauthorized partial batch")
	}
}

type lifecycleFailStarter struct {
	TxStarter
	events atomic.Int32
}
type lifecycleFailTx struct {
	pgx.Tx
	owner *lifecycleFailStarter
}

func (s *lifecycleFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &lifecycleFailTx{Tx: tx, owner: s}, nil
}
func (tx *lifecycleFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: AppendIterationLifecycleEvent") && tx.owner.events.Add(1) == 2 {
		return pgconn.CommandTag{}, errors.New("injected second event failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestIterationLifecycleSecondMemberFailureRollsBackWholeBatch(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	a := fx.Issue(t, "A")
	b := fx.Issue(t, "B")
	ws, actor := lifecycleIDs(fx)
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: []iteration.Move{{IssueID: a, ExpectedIssueRevision: 1, TargetID: &iid}, {IssueID: b, ExpectedIssueRevision: 1, TargetID: &iid}}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	s.TxStarter = &lifecycleFailStarter{TxStarter: fx.Pool}
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	if err == nil || !strings.Contains(err.Error(), "injected second event failure") {
		t.Fatalf("did not exercise recorder failure: %v", err)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 0 || fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", iid) != 0 || fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND issue_id IS NOT NULL", iid) != 0 {
		t.Fatal("failed batch left mutation or facts")
	}
}

func TestIterationLifecyclePreviewUsesRepeatableRead(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	called := false
	_, err := s.Preview(t.Context(), ws, actor, lifecycleDraft(t, fx, iid, "delete"), func(ctx context.Context, tx pgx.Tx) error {
		called = true
		var isolation string
		if err := tx.QueryRow(ctx, "SHOW transaction_isolation").Scan(&isolation); err != nil {
			return err
		}
		if isolation != "repeatable read" {
			t.Fatalf("preview isolation=%q", isolation)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("preview skipped current authority")
	}
}

func TestIterationLifecycleCompetingStartsOnlyOneActive(t *testing.T) {
	fx, s := lifecycleFixture(t)
	a := lifecycleCreate(t, fx, s)
	b := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	secondActor := util.MustParseUUID(fx.User(t, "Second lifecycle member", uuid.NewString()+"@test.invalid"))
	fx.Member(t, fx.WorkspaceID, util.UUIDToString(secondActor), "member")
	actors := []pgtype.UUID{actor, secondActor}
	requests := []ApplyIterationInput{}
	for index, iid := range []string{a, b} {
		draft := lifecycleDraft(t, fx, iid, "start")
		draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
		p, err := s.Preview(t.Context(), ws, actors[index], draft, lifecycleAuth)
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: p.PreviewHash})
	}
	owner, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Rollback(t.Context()) }()
	if err = iteration.LockWorkspace(t.Context(), owner, ws); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 2)
	results := make(chan error, 2)
	for index, request := range requests {
		go func(in ApplyIterationInput, currentActor pgtype.UUID) {
			_, err := s.Apply(t.Context(), ws, currentActor, in, func(context.Context, pgx.Tx) error { entered <- struct{}{}; return nil })
			results <- err
		}(request, actors[index])
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("start did not reach fence")
		}
	}
	if err = owner.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	success := 0
	conflicts := 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else {
			var op *iteration.OperationError
			if errors.As(err, &op) && op.Status == 409 {
				conflicts++
			} else {
				t.Fatal(err)
			}
		}
	}
	if success != 1 || conflicts != 1 || fx.Count(t, "SELECT count(*) FROM iteration WHERE workspace_id=$1 AND status='active'", fx.WorkspaceID) != 1 {
		t.Fatalf("start winners=%d conflicts=%d", success, conflicts)
	}
}

func TestIterationLifecycleCompleteThousandMemberPreviewAndLimit(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	moves := make([]iteration.Move, 0, 1000)
	for range 1000 {
		id := fx.Issue(t, "Capacity task", testutil.Cols{"status": "todo"})
		moves = append(moves, iteration.Move{IssueID: id, ExpectedIssueRevision: 1, TargetID: &iid})
	}
	draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: moves}
	begin := time.Now()
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("1000-member move preview %s", time.Since(begin))
	if !preview.Complete || preview.TotalAffected != 1000 || len(preview.Issues) != 1000 || len(preview.InvalidItems) != 0 {
		t.Fatalf("truncated complete set: total=%d issues=%d invalid=%d", preview.TotalAffected, len(preview.Issues), len(preview.InvalidItems))
	}
	begin = time.Now()
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("1000-member atomic move %s", time.Since(begin))
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 1000 {
		t.Fatal("capacity move truncated")
	}
	start := lifecycleDraft(t, fx, iid, "start")
	start.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	begin = time.Now()
	lifecycleApply(t, fx, s, start)
	t.Logf("1000-member preview and start %s", time.Since(begin))
	if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND in_original", iid) != 1000 {
		t.Fatal("capacity baseline truncated")
	}
	oversize := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: make([]iteration.Move, MaxIterationOperationIssues+1)}
	for i := range oversize.Moves {
		oversize.Moves[i] = iteration.Move{IssueID: uuid.NewString(), ExpectedIssueRevision: 1, TargetID: &iid}
	}
	_, err = s.Preview(t.Context(), ws, actor, oversize, lifecycleAuth)
	var op *iteration.OperationError
	if !errors.As(err, &op) || op.Status != 413 {
		t.Fatalf("oversize not rejected: %v", err)
	}
	if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", iid) != 1000 {
		t.Fatal("oversize changed scope")
	}
}

func TestIterationLifecycleFirstActiveJoinAfterPlannedLeaveAndStartedReset(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	issueID := fx.Issue(t, "Started before joining", testutil.Cols{"status": "in_progress"})
	lifecycleMove(t, fx, s, issueID, &iid)
	lifecycleMove(t, fx, s, issueID, nil)
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, draft)
	lifecycleMove(t, fx, s, issueID, &iid)
	lifecycleMove(t, fx, s, issueID, nil)
	fx.Exec(t, "UPDATE issue SET status='todo',revision=revision+1 WHERE id=$1", issueID)
	lifecycleMove(t, fx, s, issueID, &iid)
	tx, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	history, err := iteration.LoadHistory(t.Context(), tx, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(iid), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stats := history.Statistics
	if stats.Original != 0 || stats.AddedUnique != 1 || stats.ReentryEvents != 1 || stats.RemovedEvents != 1 || stats.Current != 1 || stats.Started != 0 {
		t.Fatalf("incorrect active participation counters: %+v", stats)
	}
}

func TestIterationLifecycleHistoryRenameKeepsRevokedCoordinator(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	old := fx.User(t, "Former coordinator", uuid.NewString()+"@test.invalid")
	fx.Member(t, fx.WorkspaceID, old, "member")
	fx.Exec(t, "UPDATE iteration SET coordinator_user_id=$1 WHERE id=$2", old, iid)
	draft := lifecycleDraft(t, fx, iid, "cancel")
	reason := "Changed plans"
	draft.Reason = &reason
	lifecycleApply(t, fx, s, draft)
	fx.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", fx.WorkspaceID, old)
	var revision int64
	fx.QueryRow(t, "SELECT revision FROM iteration WHERE id=$1", iid).Scan(&revision)
	_, err := s.Edit(t.Context(), ws, actor, util.MustParseUUID(iid), EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: revision, Fields: map[string]json.RawMessage{"name": json.RawMessage(`"Corrected history title"`)}}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	var coordinator string
	fx.QueryRow(t, "SELECT coordinator_user_id FROM iteration WHERE id=$1", iid).Scan(&coordinator)
	if coordinator != old {
		t.Fatal("historical correction changed coordinator")
	}
}

func TestIterationLifecycleBackwardsClockClampsStartAndEvents(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	fx.Exec(t, "UPDATE iteration_event SET occurred_at=$1 WHERE iteration_id=$2", future, iid)
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return now, nil }
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	var clamped, raw bool
	fx.QueryRow(t, "SELECT i.started_at=e.occurred_at,e.sampled_at<e.occurred_at FROM iteration i JOIN iteration_event e ON e.iteration_id=i.id AND e.kind='start' WHERE i.id=$1", iid).Scan(&clamped, &raw)
	if !clamped || !raw {
		t.Fatal("backwards clock reordered start or lost raw sample")
	}
}

func TestIterationLifecycleCreateHoldsSettingsLock(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, actor := lifecycleIDs(fx)
	owner, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Rollback(t.Context()) }()
	if _, err = db.New(owner).LockIterationSettings(t.Context(), ws); err != nil {
		t.Fatal(err)
	}
	pid := make(chan uint32, 1)
	done := make(chan error, 1)
	go func() {
		_, err := s.Create(t.Context(), ws, actor, CreateIterationInput{RequestID: uuid.NewString(), Name: "Blocked create", StartDate: "2026-10-06", EndDate: "2026-10-19", ConfirmedTimezone: "UTC"}, func(_ context.Context, tx pgx.Tx) error { pid <- tx.Conn().PgConn().PID(); return nil })
		done <- err
	}()
	writerPID := <-pid
	deadline := time.After(5 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("create bypassed settings lock: %v", err)
		case <-deadline:
			t.Fatal("create did not wait on settings lock")
		default:
		}
		var blocked bool
		fx.QueryRow(t, "SELECT $1::integer=ANY(pg_blocking_pids($2::integer))", owner.Conn().PgConn().PID(), writerPID).Scan(&blocked)
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err = owner.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestIterationLifecycleBackwardsClockClampsPlannedCancellation(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	now := time.Now().UTC()
	fx.Exec(t, "UPDATE iteration_event SET occurred_at=$1 WHERE iteration_id=$2", now.Add(time.Hour), iid)
	s.Now = func(context.Context, pgx.Tx) (time.Time, error) { return now, nil }
	draft := lifecycleDraft(t, fx, iid, "cancel")
	reason := "Cancelled after clock correction"
	draft.Reason = &reason
	lifecycleApply(t, fx, s, draft)
	var ordered, raw bool
	fx.QueryRow(t, "SELECT i.processed_at>=i.logical_ended_at AND i.logical_ended_at=e.occurred_at,e.sampled_at<e.occurred_at FROM iteration i JOIN iteration_event e ON e.iteration_id=i.id AND e.kind='cancel_planned' WHERE i.id=$1", iid).Scan(&ordered, &raw)
	if !ordered || !raw {
		t.Fatal("planned cancellation timestamps lost logical order or raw clock")
	}
}

func TestIterationLifecycleManagementPreservesRunningExecutionAndProject(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	runtime := fx.Runtime(t, "Existing runtime")
	agent := fx.Agent(t, "Existing agent", runtime)
	project := fx.Project(t, "Existing project")
	issueID := fx.Issue(t, "Running elsewhere", testutil.Cols{"status": "blocked", "assignee_type": "agent", "assignee_id": agent, "project_id": project})
	taskID := fx.Task(t, agent, testutil.Cols{"issue_id": issueID, "runtime_id": runtime, "status": "running", "session_id": "existing-session", "started_at": testutil.Raw("clock_timestamp()")})
	lifecycleMove(t, fx, s, issueID, &iid)
	ws, actor := lifecycleIDs(fx)
	draft := lifecycleDraft(t, fx, iid, "start")
	draft.Start = &iteration.StartDraft{TargetID: iid, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Issues) != 1 || preview.Issues[0].RunningExecutionCount != 1 {
		t.Fatal("preview omitted existing execution")
	}
	fx.Exec(t, "UPDATE agent_task_queue SET context=$2::jsonb WHERE id=$1", taskID, `{"progress":50}`)
	before, err := db.New(fx.Pool).GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleMove(t, fx, s, issueID, nil)
	after, err := db.New(fx.Pool).GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(before)
	next, _ := json.Marshal(after)
	if string(old) != string(next) {
		t.Fatal("iteration management changed the running execution")
	}
	var status, projectAfter string
	fx.QueryRow(t, "SELECT status,project_id FROM issue WHERE id=$1", issueID).Scan(&status, &projectAfter)
	if status != "blocked" || projectAfter != project || fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issueID) != 1 {
		t.Fatal("iteration management changed issue semantics or started another task")
	}
}

// Faults follow the generated bulk transport as well as the borrowed single
// writer, so changing transport cannot silently disable rollback coverage.
func (tx *lifecycleFailTx) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	failAt := -1
	for i, q := range batch.QueuedQueries {
		if strings.Contains(q.SQL, "-- name: AppendIterationLifecycleEvent") && tx.owner.events.Add(1) == 2 {
			failAt = i
		}
	}
	return &lifecycleFailBatch{BatchResults: tx.Tx.SendBatch(ctx, batch), failAt: failAt}
}

type lifecycleFailBatch struct {
	pgx.BatchResults
	failAt, index int
}

func (b *lifecycleFailBatch) Exec() (pgconn.CommandTag, error) {
	tag, e := b.BatchResults.Exec()
	i := b.index
	b.index++
	if i == b.failAt {
		return tag, errors.New("injected second event failure")
	}
	return tag, e
}
