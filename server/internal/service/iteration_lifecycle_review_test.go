package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type lifecycleReviewStarter struct {
	inner    TxStarter
	began    chan uint32
	conflict chan struct{}
	attempts atomic.Int32
}

func (s *lifecycleReviewStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.attempts.Add(1)
	if s.began != nil {
		s.began <- tx.Conn().PgConn().PID()
	}
	return &lifecycleReviewTx{Tx: tx, owner: s}, nil
}

type lifecycleReviewTx struct {
	pgx.Tx
	owner *lifecycleReviewStarter
}

func (tx *lifecycleReviewTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: LockIterationReferenceMember") {
		return lifecycleReviewRow{Row: row, conflict: tx.owner.conflict}
	}
	return row
}

type lifecycleReviewRow struct {
	pgx.Row
	conflict chan struct{}
}

func (row lifecycleReviewRow) Scan(dest ...any) error {
	err := row.Row.Scan(dest...)
	var pgErr *pgconn.PgError
	if row.conflict != nil && errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		select {
		case row.conflict <- struct{}{}:
		default:
		}
	}
	return err
}

func TestIterationLifecycleReviewCoordinatorLockRetries(t *testing.T) {
	fx, s := lifecycleFixture(t)
	id := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	coordinator := fx.User(t, "Coordinator", uuid.NewString()+"@test.invalid")
	fx.Member(t, fx.WorkspaceID, coordinator, "member")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	holder, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(ctx, `UPDATE "user" SET name='Renamed coordinator' WHERE id=$1`, coordinator); err != nil {
		t.Fatal(err)
	}
	starter := &lifecycleReviewStarter{inner: s.TxStarter, conflict: make(chan struct{}, 1)}
	s.TxStarter = starter
	raw, err := json.Marshal(coordinator)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.Edit(ctx, ws, actor, util.MustParseUUID(id), EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: 1, Fields: map[string]json.RawMessage{"coordinator_user_id": raw}}, lifecycleAuth)
		done <- err
	}()
	select {
	case <-starter.conflict:
	case err := <-done:
		t.Fatalf("edit never reached NOWAIT conflict: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatalf("coordinator conflict was not retried: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if starter.attempts.Load() != 2 {
		t.Fatalf("expected one fresh retry, got %d", starter.attempts.Load())
	}
	if fx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='edit'", id) != 1 {
		t.Fatal("retry duplicated edit event")
	}
}

func TestIterationLifecycleReviewPreviewReauthorizesAfterRevocation(t *testing.T) {
	fx, s := lifecycleFixture(t)
	id := lifecycleCreate(t, fx, s)
	ws, _ := lifecycleIDs(fx)
	user := fx.User(t, "Preview reader", uuid.NewString()+"@test.invalid")
	member := fx.Member(t, fx.WorkspaceID, user, "member")
	actor := util.MustParseUUID(user)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	holder, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if err = db.New(holder).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		t.Fatal(err)
	}
	if _, err = holder.Exec(ctx, "DELETE FROM member WHERE id=$1", member); err != nil {
		t.Fatal(err)
	}
	starter := &lifecycleReviewStarter{inner: s.TxStarter, began: make(chan uint32, 3)}
	s.TxStarter = starter
	draft := lifecycleDraft(t, fx, id, "delete")
	done := make(chan error, 1)
	go func() { _, err := s.Preview(ctx, ws, actor, draft, lifecycleAuth); done <- err }()
	var pid uint32
	select {
	case pid = <-starter.began:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for {
		var blocked bool
		if err = fx.Pool.QueryRow(ctx, "SELECT $1::int=ANY(pg_blocking_pids($2::int))", holder.Conn().PgConn().PID(), pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err = <-done:
			t.Fatalf("preview skipped revoke fence: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var api *iteration.OperationError
	if !errors.As(err, &api) || api.Status != 403 {
		t.Fatalf("preview did not reauthorize after RR conflict: %v", err)
	}
	if starter.attempts.Load() != 2 {
		t.Fatalf("expected fresh authorization attempt, got %d", starter.attempts.Load())
	}
}

func TestIterationLifecycleReviewAvailabilityInvalidatesPreview(t *testing.T) {
	for _, kind := range []string{"agent", "squad", "squad_leader", "foreign_squad_leader"} {
		t.Run(kind, func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			id := lifecycleCreate(t, fx, s)
			ws, actor := lifecycleIDs(fx)
			agent := fx.Agent(t, "Persistent agent name", "")
			assignee := agent
			assigneeType := "agent"
			if kind != "agent" {
				assignee = fx.Squad(t, "Persistent squad name", agent)
				assigneeType = "squad"
			}
			issue := fx.Issue(t, "Referenced issue", testutil.Cols{"assignee_type": assigneeType, "assignee_id": assignee})
			draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: 1, TargetID: &id}}}
			preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
			if err != nil {
				t.Fatal(err)
			}
			if !preview.Issues[0].Assignee.Available {
				t.Fatal("initial assignee should be available")
			}
			name := *preview.Issues[0].Assignee.Name
			if kind == "foreign_squad_leader" {
				foreignWorkspace := fx.Workspace(t, "Foreign leader workspace", uuid.NewString())
				foreignLeader := fx.Agent(t, "Foreign leader", "", testutil.Cols{"workspace_id": foreignWorkspace})
				fx.Exec(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", assignee, foreignLeader)
				fx.Cleanup(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", assignee, agent)
			} else if kind == "squad" {
				fx.Exec(t, "UPDATE squad SET archived_at=clock_timestamp() WHERE id=$1", assignee)
			} else {
				if _, err = db.New(fx.Pool).ArchiveAgent(t.Context(), db.ArchiveAgentParams{ID: util.MustParseUUID(agent), ArchivedBy: actor}); err != nil {
					t.Fatal(err)
				}
			}
			_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
			var api *iteration.OperationError
			if !errors.As(err, &api) || api.Code != "iteration_preview_stale" {
				t.Fatalf("archived %s did not stale preview: %v", kind, err)
			}
			fresh, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
			if err != nil {
				t.Fatal(err)
			}
			if fresh.Issues[0].Assignee.Available || fresh.Issues[0].Assignee.Name == nil || *fresh.Issues[0].Assignee.Name != name {
				t.Fatalf("availability conflated historical display: %+v", fresh.Issues[0].Assignee)
			}
			if fx.Count(t, "SELECT count(*) FROM issue WHERE current_iteration_id=$1", id) != 0 {
				t.Fatal("stale reference preview mutated membership")
			}
		})
	}
}

func TestIterationLifecycleReviewPreviewConflictBudget(t *testing.T) {
	fx, s := lifecycleFixture(t)
	id := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	calls := 0
	preview, err := s.Preview(t.Context(), ws, actor, lifecycleDraft(t, fx, id, "delete"), func(context.Context, pgx.Tx) error {
		calls++
		return &pgconn.PgError{Code: "40001", Message: "injected repeated serialization conflict"}
	})
	var api *iteration.OperationError
	if calls != 3 || !errors.As(err, &api) || api.Status != 503 || !api.Retryable || preview.PreviewHash != "" {
		t.Fatalf("unbounded or partial preview: attempts=%d preview=%+v error=%v", calls, preview, err)
	}
}

func TestIterationLifecycleReviewStartBindsFrozenIdentifier(t *testing.T) {
	for _, change := range []string{"prefix", "legacy_name"} {
		t.Run(change, func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			id := lifecycleCreate(t, fx, s)
			ws, actor := lifecycleIDs(fx)
			issue := fx.Issue(t, "Start identity")
			lifecycleMove(t, fx, s, issue, &id)
			draft := lifecycleDraft(t, fx, id, "start")
			draft.Start = &iteration.StartDraft{TargetID: id, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
			preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Issues[0].Identifier != "LIF-1" {
				t.Fatalf("preview omitted legacy identifier: %+v", preview.Issues[0])
			}
			if change == "prefix" {
				fx.Exec(t, "UPDATE workspace SET issue_prefix='NEW' WHERE id=$1", fx.WorkspaceID)
			} else {
				fx.Exec(t, "UPDATE workspace SET name='Renamed workspace' WHERE id=$1", fx.WorkspaceID)
			}
			_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
			var api *iteration.OperationError
			if !errors.As(err, &api) || api.Code != "iteration_preview_stale" {
				t.Fatalf("changed %s silently altered frozen identifier: %v", change, err)
			}
			if fx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND in_original", id) != 0 {
				t.Fatal("stale identifier created an original baseline")
			}
		})
	}
}
