package handler

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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func triageAuthorizationFixture(t *testing.T, kind string) (*Handler, TriageItem, string, string, string) {
	t.Helper()
	h := lifecycleHTTPHandler(t)
	targetIteration := lifecycleHTTPCreate(t, h, "Authorized accepted work")
	triageEnableForTest(t)
	owner := dbfx.User(t, "Triage target owner", uuid.NewString()+"@example.invalid")
	agent := dbfx.Agent(t, "Triage protected target", testRuntimeID, testutil.Cols{"owner_id": owner, "permission_mode": "public_to"})
	dbfx.Exec(t, "INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES($1,'member',$2)", agent, testUserID)
	dbfx.Cleanup(t, "DELETE FROM agent_invocation_target WHERE agent_id=$1", agent)
	assignee := agent
	if kind == "squad" {
		assignee = dbfx.Squad(t, "Triage protected squad", agent)
	}
	input := triageInputForTest()
	input["candidate_assignee_type"] = kind
	input["candidate_assignee_id"] = assignee
	var item TriageItem
	testutil.Call(t, h.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(201).JSON(&item)
	return h, item, targetIteration, agent, assignee
}

func TestTriageIterationAuthorizationHeldThroughCommit(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			h, item, target, agent, assignee := triageAuthorizationFixture(t, kind)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			barrier := projectAssociationCommitStarter{base: testPool, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
			defer func() {
				select {
				case <-barrier.release:
				default:
					close(barrier.release)
				}
			}()
			h.TxStarter = barrier
			request := withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", uuid.NewString())).WithContext(ctx), "id", item.Issue.ID)
			done := make(chan *testutil.Response, 1)
			go func() { done <- testutil.Call(t, h.ActOnTriageItem, request) }()
			select {
			case <-barrier.reached:
			case response := <-done:
				t.Fatalf("acceptance stopped before commit: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			probes := []struct{ sql, id string }{{"SELECT id FROM agent WHERE id=$1 FOR NO KEY UPDATE NOWAIT", agent}, {"SELECT agent_id FROM agent_invocation_target WHERE agent_id=$1 FOR UPDATE NOWAIT", agent}}
			if kind == "squad" {
				probes = append(probes, struct{ sql, id string }{"SELECT id FROM squad WHERE id=$1 FOR NO KEY UPDATE NOWAIT", assignee})
			}
			for _, probe := range probes {
				tx, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(ctx, probe.sql, probe.id)
				_ = tx.Rollback(context.Background())
				var busy *pgconn.PgError
				if !errors.As(err, &busy) || busy.Code != "55P03" {
					t.Errorf("acceptance did not retain authorization lock (%s): %v", probe.sql, err)
				}
			}
			close(barrier.release)
			(<-done).Want(200)
			if dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", item.Issue.ID) != 0 {
				t.Fatal("ordinary acceptance started execution")
			}
		})
	}
}

func TestTriageIterationAuthorizationRejectsPrestartedGrantRemoval(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			h, item, target, agent, _ := triageAuthorizationFixture(t, kind)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			// Grant deletion also owns the agent configuration row. Acceptance
			// must retry its earlier agent lock before reading the revoked grant.
			if err = db.New(holder).DeleteAgentInvocationTargets(ctx, parseUUID(agent)); err != nil {
				t.Fatal(err)
			}
			observed := false
			attempts := 0
			var once sync.Once
			h.TxStarter = progressTxStarter{base: h.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
				attempts++
				return progressHookTx{Tx: tx, afterRow: func(sql string, readErr error) {
					if strings.Contains(sql, "-- name: LockLifecycleAgent ") {
						once.Do(func() {
							observed = true
							var busy *pgconn.PgError
							if !errors.As(readErr, &busy) || busy.Code != "55P03" {
								t.Errorf("revocation did not block the agent locking read: %v", readErr)
							}
							if e := holder.Commit(ctx); e != nil {
								t.Error(e)
							}
						})
					}
				}}
			}}
			requestID := uuid.NewString()
			request := withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", requestID)).WithContext(ctx), "id", item.Issue.ID)
			testutil.Call(t, h.ActOnTriageItem, request).Want(http.StatusForbidden)
			if !observed || attempts < 2 {
				t.Fatalf("acceptance did not retry the current agent authority: observed=%t attempts=%d", observed, attempts)
			}
			if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL", item.Issue.ID) != 1 || dbfx.Count(t, "SELECT count(*) FROM triage_action WHERE request_id=$1", requestID) != 0 || dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE issue_id=$1", item.Issue.ID) != 0 {
				t.Fatal("revoked grant left partial acceptance/history")
			}
		})
	}
}

func TestTriageIterationAuthorizationIgnoresGrantInsertedAfterLockedRead(t *testing.T) {
	h, item, target, agent, _ := triageAuthorizationFixture(t, "agent")
	dbfx.Exec(t, "DELETE FROM agent_invocation_target WHERE agent_id=$1", agent)
	attempted := false
	h.TxStarter = issueGrantReadStarter{base: h.TxStarter, after: func() {
		if attempted {
			return
		}
		attempted = true
		_, err := testPool.Exec(t.Context(), "INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES($1,'member',$2)", agent, testUserID)
		var busy *pgconn.PgError
		if !errors.As(err, &busy) || busy.Code != "55P03" {
			t.Errorf("late grant insert bypassed the held agent configuration lock: %v", err)
		}
	}}
	requestID := uuid.NewString()
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", requestID)), "id", item.Issue.ID)).Want(http.StatusForbidden)
	if !attempted {
		t.Fatal("authorization never captured locked grant rows")
	}
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL", item.Issue.ID) != 1 || dbfx.Count(t, "SELECT count(*) FROM triage_action WHERE request_id=$1", requestID) != 0 ||
		dbfx.Count(t, "SELECT count(*) FROM agent_invocation_target WHERE agent_id=$1", agent) != 0 || dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE issue_id=$1", item.Issue.ID) != 0 {
		t.Fatal("unlocked inserted grant authorized acceptance")
	}
	dbfx.Exec(t, "INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES($1,'member',$2)", agent, testUserID)
	freshID := uuid.NewString()
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", freshID)), "id", item.Issue.ID)).Want(http.StatusOK)
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND admission_status='accepted' AND current_iteration_id=$2", item.Issue.ID, target) != 1 ||
		dbfx.Count(t, "SELECT count(*) FROM triage_action WHERE request_id=$1", freshID) != 1 || dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", item.Issue.ID) != 0 {
		t.Fatal("freshly granted plain acceptance lost its membership, action or no-dispatch contract")
	}
}

func TestTriageIterationAuthorizationRejectsForeignSquadLeader(t *testing.T) {
	h, item, target, originalLeader, squad := triageAuthorizationFixture(t, "squad")
	foreignWorkspace := dbfx.Workspace(t, "Foreign triage leader workspace", uuid.NewString())
	foreignLeader := dbfx.Agent(t, "Foreign owned leader", "", testutil.Cols{"workspace_id": foreignWorkspace})
	dbfx.Exec(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", squad, foreignLeader)
	dbfx.Cleanup(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", squad, originalLeader)
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", uuid.NewString())), "id", item.Issue.ID)).Want(http.StatusBadRequest)
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL", item.Issue.ID) != 1 {
		t.Fatal("foreign leader authorized acceptance")
	}
}
