package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type resourceConcurrentRequest struct {
	resourceID string
	body       map[string]any
}

// Hold the parent while both requests reach a real database lock wait. Before
// the fix they have already read stale resources; afterwards they wait before
// reading. Distinct users prevent the subscriber fence masking the race.
func runConcurrentResourceRequests(t *testing.T, projectID string, requests [2]resourceConcurrentRequest) [2]*testutil.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	users := [2]string{testUserID, dbfx.User(t, "Resource writer", "resource-"+uuid.NewString()+"@example.invalid")}
	dbfx.Member(t, testWorkspaceID, users[1], "member")
	holder, err := pgx.ConnectConfig(ctx, testPool.Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close(context.Background())
	tx, err := holder.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT id FROM project WHERE id=$1 FOR NO KEY UPDATE", projectID); err != nil {
		t.Fatal(err)
	}
	var conns [2]*pgx.Conn
	for i := range conns {
		conns[i], err = pgx.ConnectConfig(ctx, testPool.Config().ConnConfig.Copy())
		if err != nil {
			t.Fatal(err)
		}
		defer conns[i].Close(context.Background())
	}
	var wg sync.WaitGroup
	var responses [2]*testutil.Response
	// Cancel and release the fixture fence before joining, including on failure.
	defer func() { cancel(); _ = tx.Rollback(context.Background()); wg.Wait() }()
	for i, input := range requests {
		h := *testHandler
		h.Queries, h.TxStarter = db.New(conns[i]), conns[i]
		method, path := http.MethodPost, "/api/projects/"+projectID+"/resources"
		action := h.CreateProjectResource
		if input.resourceID != "" {
			method, path, action = http.MethodPut, path+"/"+input.resourceID, h.UpdateProjectResource
		}
		req := testutil.WithURLParams(testutil.JSONRequest(method, path, input.body).WithContext(ctx), "id", projectID, "resourceId", input.resourceID)
		req = testutil.WithHeaders(req, "X-User-ID", users[i], "X-Workspace-ID", testWorkspaceID)
		wg.Go(func() { responses[i] = testutil.Call(t, action, req) })
	}
	pids := []int32{int32(conns[0].PgConn().PID()), int32(conns[1].PgConn().PID())}
	for {
		var waiting int
		err = testPool.QueryRow(ctx, `WITH RECURSIVE blockers(root, pid) AS (
   SELECT pid, unnest(pg_blocking_pids(pid)) FROM pg_stat_activity
   WHERE pid=ANY($1::int[]) AND wait_event_type='Lock'
   UNION
   SELECT b.root, unnest(pg_blocking_pids(b.pid)) FROM blockers b
  ) SELECT count(DISTINCT root) FROM blockers WHERE pid=$2`, pids, int32(holder.PgConn().PID())).Scan(&waiting)
		if err != nil {
			t.Fatalf("observe both resource writers blocked: %v", err)
		}
		if waiting == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("both resource writers did not reach the project lock")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	return responses
}

func resourceConcurrencyProject(t *testing.T) string {
	t.Helper()
	projectID := dbfx.Project(t, "Concurrent resource writes")
	dbfx.Cleanup(t, "DELETE FROM project_resource WHERE project_id=$1", projectID)
	return projectID
}
func resourceConcurrencyRef(daemon, path string) map[string]any {
	return map[string]any{"daemon_id": daemon, "local_path": path}
}
func resourceConcurrencyRow(t *testing.T, projectID, daemon, path string) string {
	t.Helper()
	ref, err := json.Marshal(resourceConcurrencyRef(daemon, path))
	if err != nil {
		t.Fatal(err)
	}
	return dbfx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": projectID, "resource_type": "local_directory", "resource_ref": ref, "label": "Original", "position": 0})
}

func TestProjectResourceConcurrentDaemonConflict(t *testing.T) {
	for _, variant := range []string{"create", "update", "create_update"} {
		t.Run(variant, func(t *testing.T) {
			p, target := resourceConcurrencyProject(t), uuid.NewString()
			var inputs [2]resourceConcurrentRequest
			var ids [2]string
			for i := range inputs {
				inputs[i].body = map[string]any{"resource_type": "local_directory", "resource_ref": resourceConcurrencyRef(target, fmt.Sprintf("/target/%d", i))}
				if variant == "update" || (variant == "create_update" && i == 1) {
					ids[i] = resourceConcurrencyRow(t, p, uuid.NewString(), fmt.Sprintf("/original/%d", i))
					inputs[i].resourceID = ids[i]
				}
			}
			responses := runConcurrentResourceRequests(t, p, inputs)
			successes, conflicts := 0, 0
			for i, r := range responses {
				want := http.StatusCreated
				if ids[i] != "" {
					want = http.StatusOK
				}
				switch r.Code {
				case want:
					successes++
				case http.StatusConflict:
					conflicts++
					if ids[i] != "" {
						var path string
						dbfx.QueryRow(t, "SELECT resource_ref->>'local_path' FROM project_resource WHERE id=$1", ids[i]).Scan(&path)
						if path != fmt.Sprintf("/original/%d", i) {
							t.Errorf("loser changed path: %s", path)
						}
					}
				default:
					t.Errorf("unexpected status %d: %s", r.Code, r.Text())
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Errorf("successes=%d conflicts=%d; want one each", successes, conflicts)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM project_resource WHERE project_id=$1 AND resource_ref->>'daemon_id'=$2", p, target); n != 1 {
				t.Errorf("target daemon has %d rows, want 1", n)
			}
		})
	}
}

func TestProjectResourceConcurrentPartialEdits(t *testing.T) {
	for _, field := range []string{"label", "position"} {
		t.Run(field, func(t *testing.T) {
			p, daemon := resourceConcurrencyProject(t), uuid.NewString()
			id := resourceConcurrencyRow(t, p, daemon, "/original")
			dbfx.Runtime(t, "Worktree capable resource writer", testutil.Cols{"daemon_id": daemon, "metadata": `{"capabilities":["local-worktree-v1"]}`})
			executionRef := resourceConcurrencyRef(daemon, "/new-path")
			executionRef["execution_mode"] = "worktree"
			edit := map[string]any{"label": "Renamed"}
			if field == "position" {
				edit = map[string]any{"position": 7}
			}
			responses := runConcurrentResourceRequests(t, p, [2]resourceConcurrentRequest{
				{id, map[string]any{"resource_ref": executionRef}}, {id, edit},
			})
			for _, r := range responses {
				r.Want(http.StatusOK)
			}
			var ref []byte
			var label string
			var position int
			dbfx.QueryRow(t, "SELECT resource_ref,label,position FROM project_resource WHERE id=$1", id).Scan(&ref, &label, &position)
			var stored localDirectoryRef
			if err := json.Unmarshal(ref, &stored); err != nil {
				t.Fatal(err)
			}
			if stored.LocalPath != "/new-path" || stored.ExecutionMode != "worktree" {
				t.Errorf("execution ref lost: %s", ref)
			}
			if field == "label" && (label != "Renamed" || stored.Label != "Renamed") {
				t.Errorf("rename lost: column=%q ref=%s", label, ref)
			}
			if field == "position" && position != 7 {
				t.Errorf("position lost: %d", position)
			}
		})
	}
}

func TestProjectResourceConcurrentDifferentDaemons(t *testing.T) {
	p := resourceConcurrencyProject(t)
	var inputs [2]resourceConcurrentRequest
	for i := range inputs {
		inputs[i].body = map[string]any{"resource_type": "local_directory", "resource_ref": resourceConcurrencyRef(uuid.NewString(), "/repo")}
	}
	responses := runConcurrentResourceRequests(t, p, inputs)
	for _, r := range responses {
		r.Want(http.StatusCreated)
	}
	if n := dbfx.Count(t, "SELECT count(DISTINCT position) FROM project_resource WHERE project_id=$1", p); n != 2 {
		t.Errorf("append positions collide: %d distinct positions", n)
	}
}

func TestProjectResourceTransactionCommitBoundary(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, outcome := range []string{"commit", "write_failure", "commit_failure"} {
			t.Run(operation+"_"+outcome, func(t *testing.T) {
				p, daemon := resourceConcurrencyProject(t), uuid.NewString()
				h := *testHandler
				h.Bus = events.New()
				published := make(chan events.Event, 2)
				eventType := protocol.EventProjectResourceCreated
				body := map[string]any{"resource_type": "local_directory", "resource_ref": resourceConcurrencyRef(daemon, "/repo")}
				path, method, resourceID := "/api/projects/"+p+"/resources", http.MethodPost, ""
				if operation == "update" {
					resourceID = resourceConcurrencyRow(t, p, daemon, "/original")
					path, method = path+"/"+resourceID, http.MethodPut
					eventType = protocol.EventProjectResourceUpdated
				}
				h.Bus.Subscribe(eventType, func(e events.Event) { published <- e })
				barrier := projectAssociationCommitStarter{base: testHandler.TxStarter, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
				switch outcome {
				case "commit":
					h.TxStarter = barrier
				case "commit_failure":
					h.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
				case "write_failure":
					h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
						return progressHookTx{Tx: tx, queryFailure: func(sql string) error {
							if strings.Contains(sql, "-- name: CreateProjectResource :one") || strings.Contains(sql, "-- name: UpdateProjectResource :one") {
								return errors.New("forced resource write failure")
							}
							return nil
						}}
					}}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				req := testutil.WithURLParams(testutil.JSONRequest(method, path, body).WithContext(ctx), "id", p, "resourceId", resourceID)
				testutil.WithHeaders(req, "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
				action := h.CreateProjectResource
				if operation == "update" {
					action = h.UpdateProjectResource
				}
				done := make(chan struct{})
				var response *testutil.Response
				go func() { defer close(done); response = testutil.Call(t, action, req) }()
				defer func() {
					cancel()
					select {
					case <-barrier.release:
					default:
						close(barrier.release)
					}
					<-done
				}()
				if outcome == "commit" {
					select {
					case <-barrier.reached:
					case <-done:
						t.Fatalf("request stopped before commit: %d %s", response.Code, response.Text())
					case <-ctx.Done():
						t.Fatal("commit barrier timed out")
					}
					tx, err := testPool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					_, err = tx.Exec(ctx, "SELECT id FROM project WHERE id=$1 FOR NO KEY UPDATE NOWAIT", p)
					_ = tx.Rollback(context.Background())
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
						t.Errorf("project lock released before commit: %v", err)
					}
					if len(published) != 0 {
						t.Error("resource event published before commit")
					}
					close(barrier.release)
				}
				<-done
				if outcome == "commit" {
					status := http.StatusCreated
					if operation == "update" {
						status = http.StatusOK
					}
					response.Want(status)
					if len(published) != 1 {
						t.Errorf("published %d events, want one", len(published))
					}
				} else {
					response.Want(http.StatusInternalServerError)
					if len(published) != 0 {
						t.Error("failed transaction published a resource event")
					}
					if operation == "create" {
						if n := dbfx.Count(t, "SELECT count(*) FROM project_resource WHERE project_id=$1", p); n != 0 {
							t.Error("failed create persisted a resource")
						}
					} else {
						var path string
						dbfx.QueryRow(t, "SELECT resource_ref->>'local_path' FROM project_resource WHERE id=$1", resourceID).Scan(&path)
						if path != "/original" {
							t.Error("failed update changed the resource")
						}
					}
				}
			})
		}
	}
}
