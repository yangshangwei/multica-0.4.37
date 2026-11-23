package handler

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type f2CollectionStarter struct {
	inner         txStarter
	entered       chan struct{}
	release       chan struct{}
	once          sync.Once
	beginCount    atomic.Int32
	pageCount     atomic.Int32
	serialization bool
	cancel        context.CancelFunc
}

func (s *f2CollectionStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.beginCount.Add(1)
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &f2CollectionTx{Tx: tx, starter: s}, nil
}

type f2CollectionTx struct {
	pgx.Tx
	starter *f2CollectionStarter
}

func (tx *f2CollectionTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.starter.serialization && strings.Contains(sql, "-- name: ListReportSnapshotSourceIssues") {
		fail := false
		tx.starter.once.Do(func() { fail = true })
		if fail {
			if tx.starter.cancel != nil {
				tx.starter.cancel()
			}
			return nil, &pgconn.PgError{Code: "40001", Message: "task-owned serialization conflict"}
		}
	}
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err != nil || !strings.Contains(sql, "-- name: ListReportSnapshotSourceActivities") {
		return rows, err
	}
	tx.starter.pageCount.Add(1)
	if tx.starter.entered != nil {
		tx.starter.once.Do(func() {
			close(tx.starter.entered)
			select {
			case <-tx.starter.release:
			case <-ctx.Done():
			}
		})
	}
	return rows, nil
}

func TestF2ReportSnapshotRetainedPagesKeepOneCutAndConfirmEOF(t *testing.T) {
	project := dbfx.Project(t, "F2 page cut")
	report := f2ReportIssue(t, project)
	source := dbfx.Issue(t, "F2 page source", testutil.Cols{"project_id": project})
	dbfx.Exec(t, `INSERT INTO activity_log(id,workspace_id,issue_id,action,details,created_at) SELECT gen_random_uuid(),$1,$2,'status_changed','{"from":"todo","to":"done"}'::jsonb,'2026-01-01'::timestamptz FROM generate_series(1,2000)`, testWorkspaceID, source)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE issue_id=$1", source)
	body := f2CaptureBody(t, report)
	starter := &f2CollectionStarter{inner: testHandler.TxStarter, entered: make(chan struct{}), release: make(chan struct{})}
	h := *testHandler
	h.TxStarter = starter
	req := f2HumanRequest("POST", report, "", uuid.NewString(), body)
	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	var released sync.Once
	unblock := func() { released.Do(func() { close(starter.release) }) }
	defer unblock()
	finished := make(chan *testutil.Response, 1)
	go func() { finished <- f2Call(t, &h, req.WithContext(ctx)) }()
	select {
	case <-starter.entered:
	case <-ctx.Done():
		t.Fatal("capture did not reach first source page")
	}
	// This transaction commits after C, despite its tuple sorting before all
	// existing activity. Later pages of C must not see it or claim a watermark.
	dbfx.Insert(t, "activity_log", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": source, "action": "status_changed", "details": map[string]string{"from": "todo", "to": "done"}, "created_at": time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)})
	unblock()
	var response *testutil.Response
	select {
	case response = <-finished:
	case <-ctx.Done():
		t.Fatal("capture did not finish after page release")
	}
	var first service.ReportSnapshotResponse
	response.Want(201).JSON(&first)
	if starter.pageCount.Load() != 3 {
		t.Fatalf("exact two pages require a third EOF read, got %d reads", starter.pageCount.Load())
	}
	stored := f2StoredInternal(t, first.Snapshot.SnapshotID)
	proof := f2Source(t, stored, "activities")
	if len(stored.Facts.Activities) != 2000 || !proof.ReadComplete || proof.ProducerComplete || proof.Watermark != nil {
		t.Fatal("source pagination mixed cuts or invented historical completeness")
	}
	body["correction_of"] = first.Snapshot.SnapshotID
	second := f2Capture(t, testHandler, report, uuid.NewString(), body, 201)
	if len(f2StoredInternal(t, second.Snapshot.SnapshotID).Facts.Activities) != 2001 {
		t.Fatal("fresh correction cut did not observe the late retained row")
	}
	if f2StoredInternal(t, first.Snapshot.SnapshotID).FactsDigest != stored.FactsDigest {
		t.Fatal("late row changed the original frozen source facts")
	}
}

func TestF2ReportSnapshotCancelStopsCollectionWithoutRestart(t *testing.T) {
	project := dbfx.Project(t, "F2 canceled cut")
	report := f2ReportIssue(t, project)
	starter := &f2CollectionStarter{inner: testHandler.TxStarter, entered: make(chan struct{}), release: make(chan struct{})}
	h := *testHandler
	h.TxStarter = starter
	req := f2HumanRequest("POST", report, "", uuid.NewString(), f2CaptureBody(t, report))
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	finished := make(chan *testutil.Response, 1)
	go func() { finished <- f2Call(t, &h, req.WithContext(ctx)) }()
	select {
	case <-starter.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("capture did not reach cancel barrier")
	}
	cancel()
	select {
	case response := <-finished:
		response.Want(503)
		if strings.Contains(response.Body.String(), "\"snapshot\"") {
			t.Fatal("canceled capture released a snapshot")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("canceled capture did not stop")
	}
	if starter.beginCount.Load() != 1 || dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 0 {
		t.Fatal("cancellation restarted or published a capture")
	}
}

func TestF2ReportSnapshotOnlyLiveSerializationConflictRestarts(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "live_request"
		if canceled {
			name = "canceled_request"
		}
		t.Run(name, func(t *testing.T) {
			report := f2ReportIssue(t, "")
			starter := &f2CollectionStarter{inner: testHandler.TxStarter, serialization: true}
			h := *testHandler
			h.TxStarter = starter
			req := f2HumanRequest("POST", report, "", uuid.NewString(), f2CaptureBody(t, report))
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			if canceled {
				starter.cancel = cancel
			}
			response := f2Call(t, &h, req.WithContext(ctx))
			wantRows := 1
			wantBegins := int32(2)
			if canceled {
				response.Want(503)
				wantRows, wantBegins = 0, 1
			} else {
				response.Want(201)
			}
			if starter.beginCount.Load() != wantBegins || dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != wantRows {
				t.Fatal("capture retried outside the live original request or published partial state")
			}
		})
	}
}
