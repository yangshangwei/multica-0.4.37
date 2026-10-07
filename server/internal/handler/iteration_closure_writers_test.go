package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func closureWriterFixture(t *testing.T) (*Handler, string, string, string) {
	t.Helper()
	h := lifecycleHTTPHandler(t)
	dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1 AND type='iteration'", testWorkspaceID)
	dbfx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", testWorkspaceID)
	period := lifecycleHTTPCreate(t, h, "Frozen writer integration")
	project := dbfx.Project(t, "Original project")
	issue := dbfx.Issue(t, "Original title", testutil.Cols{"project_id": project})
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: 1, TargetID: &period}}})
	row, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(period)})
	if err != nil {
		t.Fatal(err)
	}
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "start", IterationID: &period, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 2, Start: &iteration.StartDraft{TargetID: period, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
	return h, period, project, issue
}

func closureWriterRequest(t *testing.T, h *Handler, period, issue, operation, actor string) *http.Request {
	t.Helper()
	reason := "Writer concurrency verification"
	draft := iteration.Draft{Operation: operation, ExpectedSettingsRevision: 2, Reason: &reason, Moves: []iteration.Move{}}
	if operation != "disable" {
		row, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(period)})
		if err != nil {
			t.Fatal(err)
		}
		item, err := h.Queries.GetIssue(t.Context(), parseUUID(issue))
		if err != nil {
			t.Fatal(err)
		}
		draft.IterationID = &period
		draft.ExpectedIterationRevision = &row.Revision
		draft.ExpectedScopeRevision = &row.ScopeRevision
		if item.Status != "done" && item.Status != "cancelled" {
			draft.Moves = []iteration.Move{{IssueID: issue, ExpectedIssueRevision: item.Revision, ExpectedSourceID: &period}}
		}
	}
	var preview iteration.Preview
	req := lifecycleHTTPRequest("POST", "iteration-previews", "", draft)
	req.Header.Set("X-User-ID", actor)
	testutil.Call(t, h.PreviewIterationOperation, req).Want(200).JSON(&preview)
	if len(preview.InvalidItems) > 0 {
		t.Fatalf("invalid fixture preview: %+v", preview.InvalidItems)
	}
	req = lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})
	req.Header.Set("X-User-ID", actor)
	return req
}

func closureDigest(t *testing.T, period string) string {
	t.Helper()
	var digest string
	dbfx.QueryRow(t, "SELECT md5(body::text) FROM iteration_snapshot WHERE iteration_id=$1", period).Scan(&digest)
	return digest
}

func TestIterationClosureRealPostClosureWritersPreserveSnapshot(t *testing.T) {
	h, period, project, issue := closureWriterFixture(t)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": "done"}), "id", issue)).Want(200)
	testutil.Call(t, h.ApplyIterationOperation, closureWriterRequest(t, h, period, issue, "end", testUserID)).Want(200)
	before := closureDigest(t, period)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": "todo", "title": "Reopened renamed title"}), "id", issue)).Want(200)
	if after := closureDigest(t, period); after != before {
		t.Fatal("reopen/title drifted snapshot")
	}
	replacement := dbfx.Project(t, "Replacement project")
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"project_id": replacement}), "id", issue)).Want(200)
	if after := closureDigest(t, period); after != before {
		t.Fatal("project change drifted snapshot")
	}
	for _, id := range []string{project, replacement} {
		testutil.Call(t, h.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+id, map[string]any{"expected_revision": 1}), "id", id)).Want(204)
		if after := closureDigest(t, period); after != before {
			t.Fatal("project deletion drifted snapshot")
		}
	}
	testutil.Call(t, h.DeleteIssue, withURLParam(newRequest("DELETE", "/api/issues/"+issue, nil), "id", issue)).Want(204)
	if after := closureDigest(t, period); after != before {
		t.Fatal("issue deletion drifted snapshot")
	}
	var detail struct {
		Snapshot   *iteration.Snapshot  `json:"snapshot"`
		Statistics iteration.Statistics `json:"statistics"`
	}
	testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+period, period, nil)).Want(200).JSON(&detail)
	if detail.Statistics.Completed != 1 || detail.Statistics.OriginalCompleted != 1 {
		t.Fatalf("real closed read changed: %+v", detail.Statistics)
	}
}

// Wait for the actual PostgreSQL lock queue, not a goroutine scheduling signal.
func closureWaitBlocked(t *testing.T, ctx context.Context, ownerPID int, atLeast int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var count int
		if err := testPool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid))", ownerPID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count >= atLeast {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d actual lock waiters; got %d", atLeast, count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIterationClosureRealIssueAndProjectWriterRaces(t *testing.T) {
	for _, operation := range []string{"end", "disable"} {
		for _, kind := range []string{"issue", "project", "status", "move"} {
			for _, writerFirst := range []bool{false, true} {
				order := "closure_first"
				if writerFirst {
					order = "writer_first"
				}
				t.Run(operation+"/"+kind+"/"+order, func(t *testing.T) {
					h, period, project, issue := closureWriterFixture(t)
					trace := &closureTransactionTrace{inner: h.TxStarter}
					h.TxStarter = trace
					t.Cleanup(func() {
						if t.Failed() {
							t.Log("transaction trace:\n" + trace.String())
						}
					})

					actor := dbfx.User(t, "Closure admin", uuid.NewString()+"@test.invalid")
					dbfx.Member(t, testWorkspaceID, actor, "admin")
					writer := h.DeleteIssue
					writerReq := withURLParam(newRequest("DELETE", "/api/issues/"+issue, nil), "id", issue)
					if kind == "project" {
						writer = h.DeleteProject
						writerReq = withURLParam(newRequest("DELETE", "/api/projects/"+project, map[string]any{"expected_revision": 1}), "id", project)
					}
					if kind == "status" {
						writer = h.UpdateIssue
						writerReq = withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": "done"}), "id", issue)
					}
					if kind == "move" {
						next := lifecycleHTTPCreate(t, h, "Competing destination")
						row, err := h.Queries.GetIssue(t.Context(), parseUUID(issue))
						if err != nil {
							t.Fatal(err)
						}
						reason := "Competing membership move"
						draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Reason: &reason, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: row.Revision, ExpectedSourceID: &period, TargetID: &next}}}
						var preview iteration.Preview
						testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", draft)).Want(200).JSON(&preview)
						writer = h.ApplyIterationOperation
						writerReq = lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})
					}
					closeReq := closureWriterRequest(t, h, period, issue, operation, actor)
					ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
					defer cancel()
					trace.acquired = make(chan uint32, 1)
					trace.release = make(chan struct{})
					var releaseOnce sync.Once
					release := func() { releaseOnce.Do(func() { close(trace.release) }) }
					defer release()
					closes, writes := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
					startClose := func() {
						go func() { closes <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(closeReq, ctx)) }()
					}
					startWrite := func() { go func() { writes <- testutil.Call(t, writer, closureRequestContext(writerReq, ctx)) }() }
					if writerFirst {
						startWrite()
					} else {
						startClose()
					}
					var firstPID uint32
					select {
					case firstPID = <-trace.acquired:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					// The first request owns I1 now, rather than merely waiting
					// ahead of another request behind an unrelated blocker.
					trace.mu.Lock()
					trace.began = make(chan uint32, 8)
					secondBegan := trace.began
					trace.mu.Unlock()
					if writerFirst {
						startClose()
					} else {
						startWrite()
					}
					var secondPID uint32
					select {
					case secondPID = <-secondBegan:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					waitForOperationReadBlock(t, ctx, secondPID, firstPID)
					release()
					closed, written := <-closes, <-writes
					writerStatus := 204
					if kind == "status" || kind == "move" {
						writerStatus = 200
					}
					if kind == "move" && !writerFirst {
						writerStatus = 409
						if operation == "disable" {
							writerStatus = 422
						}
					}
					written.Want(writerStatus)
					expected := 200
					if writerFirst {
						expected = 409
					}
					closed.Want(expected)
					snapshots := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period)
					if writerFirst && snapshots != 0 {
						t.Fatal("losing closure persisted snapshot")
					}
					if !writerFirst && snapshots != 1 {
						t.Fatal("winning closure lost snapshot")
					}
					if operation == "disable" {
						enabled := dbfx.Count(t, "SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled", testWorkspaceID)
						if (enabled == 1) != writerFirst {
							t.Fatal("disable settings escaped atomic result")
						}
					}
				})
			}
		}
	}
}

func TestIterationClosureRealMemberRevocationRaces(t *testing.T) {
	for _, operation := range []string{"end", "disable"} {
		for _, revokeFirst := range []bool{false, true} {
			order := "closure_first"
			if revokeFirst {
				order = "revoke_first"
			}
			t.Run(operation+"/"+order, func(t *testing.T) {
				h, period, _, issue := closureWriterFixture(t)
				actor := dbfx.User(t, "Revoked closer", uuid.NewString()+"@test.invalid")
				member := dbfx.Member(t, testWorkspaceID, actor, "admin")
				closeReq := closureWriterRequest(t, h, period, issue, operation, actor)
				revokeReq := testutil.WithURLParams(newRequest("DELETE", "/api/workspaces/"+testWorkspaceID+"/members/"+member, nil), "id", testWorkspaceID, "memberId", member)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				owner, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer owner.Rollback(context.Background())
				if err = db.New(owner).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(actor)}); err != nil {
					t.Fatal(err)
				}
				var pid int
				if err = owner.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
					t.Fatal(err)
				}
				closes, revokes := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
				startClose := func() {
					go func() { closes <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(closeReq, ctx)) }()
				}
				startRevoke := func() {
					go func() { revokes <- testutil.Call(t, h.DeleteMember, closureRequestContext(revokeReq, ctx)) }()
				}
				if revokeFirst {
					startRevoke()
				} else {
					startClose()
				}
				closureWaitBlocked(t, ctx, pid, 1)
				if revokeFirst {
					startClose()
				} else {
					startRevoke()
				}
				closureWaitBlocked(t, ctx, pid, 2)
				if err = owner.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				closed, revoked := <-closes, <-revokes
				revoked.Want(204)
				expected := 200
				if revokeFirst {
					expected = 403
				}
				closed.Want(expected)
				if n := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period); (n == 0) != revokeFirst {
					t.Fatal("revoked closure crossed authorization fence")
				}
				req := lifecycleHTTPRequest("GET", "iterations/"+period, period, nil)
				req.Header.Set("X-User-ID", actor)
				testutil.Call(t, h.GetIteration, req).Want(403)
			})
		}
	}
}

// Category editing has no production API: the status presentation endpoint
// changes name/color/description/position only. This maintenance-writer fixture
// proves the lower-level catalog lock invariant without claiming API coverage.
func TestIterationClosureCatalogReclassificationFence(t *testing.T) {
	for _, operation := range []string{"end", "disable"} {
		for _, catalogFirst := range []bool{false, true} {
			order := "closure_first"
			if catalogFirst {
				order = "catalog_first"
			}
			t.Run(operation+"/"+order, func(t *testing.T) {
				h, period, _, issue := closureWriterFixture(t)
				status := createTestCustomStatus(t, "closure_"+uuid.NewString()[:8], "todo")
				testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issue, map[string]any{"status": status.Key}), "id", issue)).Want(200)
				closeReq := closureWriterRequest(t, h, period, issue, operation, testUserID)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				owner, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer owner.Rollback(context.Background())
				if err = db.New(owner).LockIssueStatusCatalog(ctx, parseUUID(testWorkspaceID)); err != nil {
					t.Fatal(err)
				}
				var pid int
				if err = owner.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
					t.Fatal(err)
				}
				closes, writes := make(chan *testutil.Response, 1), make(chan error, 1)
				startClose := func() {
					go func() { closes <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(closeReq, ctx)) }()
				}
				startCatalog := func() {
					go func() {
						tx, e := testPool.Begin(ctx)
						if e != nil {
							writes <- e
							return
						}
						defer tx.Rollback(context.Background())
						if e = db.New(tx).LockIssueStatusCatalog(ctx, parseUUID(testWorkspaceID)); e == nil {
							_, e = tx.Exec(ctx, "UPDATE issue_status SET category='done' WHERE workspace_id=$1 AND id=$2", parseUUID(testWorkspaceID), status.ID)
						}
						if e == nil {
							e = tx.Commit(ctx)
						}
						writes <- e
					}()
				}
				if catalogFirst {
					startCatalog()
				} else {
					startClose()
				}
				closureWaitBlocked(t, ctx, pid, 1)
				if catalogFirst {
					startClose()
				} else {
					startCatalog()
				}
				closureWaitBlocked(t, ctx, pid, 2)
				if err = owner.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				closed := <-closes
				if err = <-writes; err != nil {
					t.Fatal(err)
				}
				expected := 200
				if catalogFirst {
					expected = 409
				}
				closed.Want(expected)
				if catalogFirst {
					if n := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period); n != 0 {
						t.Fatal("stale category closure wrote snapshot")
					}
				} else {
					var remaining int
					dbfx.QueryRow(t, "SELECT (body->'statistics'->>'remaining')::int FROM iteration_snapshot WHERE iteration_id=$1", period).Scan(&remaining)
					if remaining != 1 {
						t.Fatalf("later catalog edit changed frozen category count: %d", remaining)
					}
				}
			})
		}
	}
}

func closureRequestContext(req *http.Request, ctx context.Context) *http.Request {
	return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, chi.RouteContext(req.Context())))
}

type closureHTTPCommitLossStarter struct{ inner txStarter }

func (s closureHTTPCommitLossStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &closureHTTPCommitLossTx{Tx: tx}, nil
}

type closureHTTPCommitLossTx struct{ pgx.Tx }

func (tx *closureHTTPCommitLossTx) Commit(ctx context.Context) error {
	if err := tx.Tx.Commit(ctx); err != nil {
		return err
	}
	return errors.New("injected response loss after successful closure commit")
}

func TestIterationClosureHTTPRecoversLostCommitResponse(t *testing.T) {
	h, period, _, issue := closureWriterFixture(t)
	req := closureWriterRequest(t, h, period, issue, "end", testUserID)
	var input service.ApplyIterationInput
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		t.Fatal(err)
	}
	broken := *h
	broken.TxStarter = closureHTTPCommitLossStarter{inner: h.TxStarter}
	testutil.Call(t, broken.ApplyIterationOperation, lifecycleHTTPRequest("POST", "iteration-operations", "", input)).Want(503)
	if n := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period); n != 1 {
		t.Fatal("lost response did not actually commit")
	}
	var recovered iteration.WriteResult
	testutil.Call(t, h.GetIterationOperation, iterationOperationRequest(input.RequestID)).Want(200).JSON(&recovered)
	var replay iteration.WriteResult
	testutil.Call(t, h.ApplyIterationOperation, lifecycleHTTPRequest("POST", "iteration-operations", "", input)).Want(200).JSON(&replay)
	if !replay.Replayed || replay.OperationID != recovered.OperationID || replay.RequestID != input.RequestID {
		t.Fatalf("GET/retry changed operation identity: %+v %+v", recovered, replay)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM iteration_snapshot WHERE iteration_id=$1", period); n != 1 {
		t.Fatal("recovery duplicated snapshot")
	}
}

func TestIterationClosurePlannedStartVersusDisable(t *testing.T) {
	for _, startFirst := range []bool{false, true} {
		order := "disable_first"
		if startFirst {
			order = "start_first"
		}
		t.Run(order, func(t *testing.T) {
			h := lifecycleHTTPHandler(t)
			dbfx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", testWorkspaceID)
			period := lifecycleHTTPCreate(t, h, "Concurrent planned start")
			issue := dbfx.Issue(t, "Planned start commitment")
			lifecycleHTTPApply(t, h, iteration.Draft{Operation: "move", ExpectedSettingsRevision: 2, Moves: []iteration.Move{{IssueID: issue, ExpectedIssueRevision: 1, TargetID: &period}}})
			row, err := h.Queries.GetIteration(t.Context(), db.GetIterationParams{WorkspaceID: parseUUID(testWorkspaceID), ID: parseUUID(period)})
			if err != nil {
				t.Fatal(err)
			}
			draft := iteration.Draft{Operation: "start", IterationID: &period, ExpectedIterationRevision: &row.Revision, ExpectedScopeRevision: &row.ScopeRevision, ExpectedSettingsRevision: 2, Moves: []iteration.Move{}, Start: &iteration.StartDraft{TargetID: period, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}}
			var preview iteration.Preview
			testutil.Call(t, h.PreviewIterationOperation, lifecycleHTTPRequest("POST", "iteration-previews", "", draft)).Want(200).JSON(&preview)
			if len(preview.InvalidItems) != 0 {
				t.Fatalf("start preview invalid: %+v", preview.InvalidItems)
			}
			startReq := lifecycleHTTPRequest("POST", "iteration-operations", "", service.ApplyIterationInput{RequestID: uuid.NewString(), Draft: preview.Draft, PreviewHash: preview.PreviewHash})
			actor := dbfx.User(t, "Disable admin", uuid.NewString()+"@test.invalid")
			dbfx.Member(t, testWorkspaceID, actor, "admin")
			disableReq := closureWriterRequest(t, h, period, issue, "disable", actor)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			owner, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Rollback(context.Background())
			if err = iteration.LockWorkspace(ctx, owner, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			var pid int
			if err = owner.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				t.Fatal(err)
			}
			starts, disables := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
			launchStart := func() {
				go func() { starts <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(startReq, ctx)) }()
			}
			launchDisable := func() {
				go func() {
					disables <- testutil.Call(t, h.ApplyIterationOperation, closureRequestContext(disableReq, ctx))
				}()
			}
			if startFirst {
				launchStart()
			} else {
				launchDisable()
			}
			closureWaitBlocked(t, ctx, pid, 1)
			if startFirst {
				launchDisable()
			} else {
				launchStart()
			}
			closureWaitBlocked(t, ctx, pid, 2)
			if err = owner.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			started, disabled := <-starts, <-disables
			state := "cancelled"
			if startFirst {
				started.Want(200)
				disabled.Want(409)
				state = "active"
			} else {
				disabled.Want(200)
				started.Want(422)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM iteration WHERE id=$1 AND status=$2", period, state); n != 1 {
				t.Fatalf("unexpected final lifecycle state, want %s", state)
			}
			enabled := dbfx.Count(t, "SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled", testWorkspaceID)
			if (enabled == 1) != startFirst {
				t.Fatal("start/disable partially committed settings")
			}
			if !startFirst && dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='baseline'", period) != 0 {
				t.Fatal("losing start captured baseline")
			}
		})
	}
}

// closureTransactionTrace records real attempts and failed statements without
// changing locking or retry decisions. Failed queue-order assertions must show
// whether the first attempt rolled back before another request committed.
type closureTransactionTrace struct {
	inner    txStarter
	mu       sync.Mutex
	next     int
	events   []string
	acquired chan uint32
	release  chan struct{}
	began    chan uint32
	paused   bool
}

func (s *closureTransactionTrace) add(id int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, fmt.Sprintf("%s attempt=%d %s", time.Now().Format("15:04:05.000000"), id, message))
}
func (s *closureTransactionTrace) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.events, "\n")
}
func (s *closureTransactionTrace) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.next++
	id := s.next
	began := s.began
	s.mu.Unlock()
	if began != nil {
		began <- tx.Conn().PgConn().PID()
	}
	s.add(id, fmt.Sprintf("begin pid=%d", tx.Conn().PgConn().PID()))
	return &closureTracedTx{Tx: tx, trace: s, id: id, ctx: ctx}, nil
}

type closureTracedTx struct {
	pgx.Tx
	trace *closureTransactionTrace
	id    int
	ctx   context.Context
}

func (tx *closureTracedTx) statement(sql string, err error) {
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		var pgerr *pgconn.PgError
		code := ""
		if errors.As(err, &pgerr) {
			code = pgerr.Code
		}
		tx.trace.add(tx.id, fmt.Sprintf("sqlstate=%s error=%v statement=%s", code, err, strings.Join(strings.Fields(sql), " ")))
	} else if sql == iteration.WorkspaceFenceSQL {
		tx.trace.add(tx.id, "I1 fence acquired")
		tx.trace.mu.Lock()
		pause := tx.trace.acquired != nil && !tx.trace.paused
		if pause {
			tx.trace.paused = true
		}
		tx.trace.mu.Unlock()
		if pause {
			tx.trace.acquired <- tx.Conn().PgConn().PID()
			select {
			case <-tx.trace.release:
			case <-tx.ctx.Done():
			}
		}
	}
}
func (tx *closureTracedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	tx.statement(sql, err)
	return tag, err
}
func (tx *closureTracedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return closureTracedRow{Row: tx.Tx.QueryRow(ctx, sql, args...), tx: tx, sql: sql}
}
func (tx *closureTracedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	tx.statement(sql, err)
	if err != nil {
		return rows, err
	}
	return closureTracedRows{Rows: rows, tx: tx, sql: sql}, nil
}
func (tx *closureTracedTx) Commit(ctx context.Context) error {
	err := tx.Tx.Commit(ctx)
	tx.trace.add(tx.id, fmt.Sprintf("commit=%v", err))
	return err
}
func (tx *closureTracedTx) Rollback(ctx context.Context) error {
	err := tx.Tx.Rollback(ctx)
	if !errors.Is(err, pgx.ErrTxClosed) {
		tx.trace.add(tx.id, fmt.Sprintf("rollback=%v", err))
	}
	return err
}
func (tx *closureTracedTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return &closureTracedBatch{BatchResults: tx.Tx.SendBatch(ctx, b), tx: tx, queries: b.QueuedQueries}
}

type closureTracedRow struct {
	pgx.Row
	tx  *closureTracedTx
	sql string
}

func (r closureTracedRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.tx.statement(r.sql, err)
	return err
}

type closureTracedRows struct {
	pgx.Rows
	tx  *closureTracedTx
	sql string
}

func (r closureTracedRows) Err() error { err := r.Rows.Err(); r.tx.statement(r.sql, err); return err }

type closureTracedBatch struct {
	pgx.BatchResults
	tx      *closureTracedTx
	queries []*pgx.QueuedQuery
	index   int
}

func (b *closureTracedBatch) nextSQL() string { sql := b.queries[b.index].SQL; b.index++; return sql }
func (b *closureTracedBatch) Exec() (pgconn.CommandTag, error) {
	sql := b.nextSQL()
	tag, err := b.BatchResults.Exec()
	b.tx.statement(sql, err)
	return tag, err
}
func (b *closureTracedBatch) QueryRow() pgx.Row {
	return closureTracedRow{Row: b.BatchResults.QueryRow(), tx: b.tx, sql: b.nextSQL()}
}
func (b *closureTracedBatch) Query() (pgx.Rows, error) {
	sql := b.nextSQL()
	rows, err := b.BatchResults.Query()
	b.tx.statement(sql, err)
	if err != nil {
		return rows, err
	}
	return closureTracedRows{Rows: rows, tx: b.tx, sql: sql}, nil
}
func (b *closureTracedBatch) Close() error {
	err := b.BatchResults.Close()
	if err != nil {
		b.tx.statement("batch close", err)
	}
	return err
}
