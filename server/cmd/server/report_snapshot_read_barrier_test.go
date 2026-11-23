package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type f2ReadStarter struct {
	inner interface {
		Begin(context.Context) (pgx.Tx, error)
	}
	before, commitFailure bool
	entered, release      chan struct{}
	once                  sync.Once
}

func (s *f2ReadStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &f2ReadTx{Tx: tx, starter: s}, nil
}

type f2ReadTx struct {
	pgx.Tx
	starter *f2ReadStarter
}

func (tx *f2ReadTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if !strings.Contains(sql, "-- name: GetTaskTokenByHash") || tx.starter.entered == nil {
		return tx.Tx.QueryRow(ctx, sql, args...)
	}
	var row pgx.Row
	tx.starter.once.Do(func() {
		// This is the first data query in taskCapabilityRead, after SET
		// TRANSACTION. The preceding Auth and guard use the ordinary pool.
		if !tx.starter.before {
			row = tx.Tx.QueryRow(ctx, sql, args...)
		}
		close(tx.starter.entered)
		select {
		case <-tx.starter.release:
		case <-ctx.Done():
		}
	})
	if row != nil {
		return row
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx *f2ReadTx) Commit(ctx context.Context) error {
	if tx.starter.commitFailure {
		return errors.New("test-owned F2 read commit failure")
	}
	return tx.Tx.Commit(ctx)
}

func f2BarrierRouter(t *testing.T, starter *f2ReadStarter) http.Handler {
	t.Helper()
	for _, key := range []string{"MULTICA_LARK_SECRET_KEY", "MULTICA_SLACK_SECRET_KEY", "MULTICA_DINGTALK_SECRET_KEY", "MULTICA_WECOM_SECRET_KEY", "MULTICA_TELEGRAM_SECRET_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("CHANNEL_WS_LEASE_BACKEND", "postgres")
	router, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	starter.inner = h.TxStarter
	h.TxStarter = starter
	t.Cleanup(func() {
		if !h.ChannelRouter.Drain(context.Background()) {
			t.Error("F2 router did not drain")
		}
	})
	return router
}

func TestF2ReportSnapshotFirstReadAuthorityBarrier(t *testing.T) {
	changes := []struct {
		name   string
		mutate func(*testing.T, targetedCapabilityFixture)
	}{
		{"token", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "DELETE FROM task_token WHERE token_hash=$1", auth.HashToken(f.token))
		}},
		{"claim", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1", f.task)
		}},
		{"runtime_owner", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "UPDATE agent_runtime SET owner_id=$1 WHERE id=$2", testUserID, f.runtime)
		}},
		{"originator_membership", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, f.originator)
		}},
		{"originator_invoke", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2", f.agent, f.originator)
		}},
		{"report_project", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "UPDATE issue SET project_id=NULL WHERE id=$1", f.issue)
		}},
		{"report_admission", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.Exec(t, "UPDATE issue SET admission_status='pending' WHERE id=$1", f.issue)
		}},
		{"report_kind", func(t *testing.T, f targetedCapabilityFixture) {
			f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": f.issue}, "issue_id=$1", f.issue)
		}},
		{"withdrawal", func(t *testing.T, f targetedCapabilityFixture) {
			var current string
			f.fx.QueryRow(t, "SELECT current_snapshot_id FROM report_snapshot_control WHERE report_issue_id=$1", f.issue).Scan(&current)
			f2RouterWithdraw(t, f, current, uuid.NewString())
		}},
	}
	for _, change := range changes {
		for _, before := range []bool{true, false} {
			phase := "after_first_read"
			if before {
				phase = "before_first_read"
			}
			t.Run(change.name+"/"+phase, func(t *testing.T) {
				f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
				captured := f2RouterCapture(t, f)
				starter := &f2ReadStarter{before: before, entered: make(chan struct{}), release: make(chan struct{})}
				router := f2BarrierRouter(t, starter)
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				var release sync.Once
				unblock := func() { release.Do(func() { close(starter.release) }) }
				defer unblock()
				path := "/api/issues/" + f.issue + "/report-snapshots/" + captured.Snapshot.SnapshotID
				req := testutil.JSONRequest(http.MethodGet, path, nil).WithContext(ctx)
				req.Header.Set("Authorization", "Bearer "+f.token)
				finished := make(chan *testutil.Response, 1)
				go func() { finished <- testutil.Call(t, router.ServeHTTP, req) }()
				select {
				case <-starter.entered:
				case <-ctx.Done():
					t.Fatal("request did not reach first RR data query")
				}
				change.mutate(t, f)
				unblock()
				var response *testutil.Response
				select {
				case response = <-finished:
				case <-ctx.Done():
					t.Fatal("RR read did not finish")
				}
				if before {
					want := http.StatusForbidden
					if change.name == "withdrawal" {
						want = http.StatusNotFound
					}
					response.Want(want)
					if strings.Contains(response.Body.String(), captured.Snapshot.ProjectionDigest) || strings.Contains(response.Body.String(), "\"snapshot\"") {
						t.Fatal("pre-cut revocation disclosed snapshot bytes")
					}
				} else {
					response.Want(http.StatusOK)
					if !strings.Contains(response.Body.String(), captured.Snapshot.ProjectionDigest) {
						t.Fatal("read escaped the established RR data snapshot")
					}
				}
				if response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("RR response was cacheable")
				}
				want := http.StatusForbidden
				if change.name == "token" {
					want = http.StatusUnauthorized
				} else if change.name == "withdrawal" {
					want = http.StatusNotFound
				}
				f2RouterRead(t, f.token, http.MethodGet, path, nil).Want(want)
			})
		}
	}
}

func TestF2ReportSnapshotReadCommitFailureWithholdsPublicBody(t *testing.T) {
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	captured := f2RouterCapture(t, f)
	router := f2BarrierRouter(t, &f2ReadStarter{commitFailure: true})
	req := testutil.JSONRequest(http.MethodGet, "/api/issues/"+f.issue+"/report-snapshots/current", nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	response := testutil.Call(t, router.ServeHTTP, req).Want(http.StatusServiceUnavailable)
	if strings.Contains(response.Body.String(), captured.Snapshot.ProjectionDigest) || strings.Contains(response.Body.String(), "\"snapshot\"") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("failed read commit released cacheable snapshot data")
	}
}
