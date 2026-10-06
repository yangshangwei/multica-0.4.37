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

type triageGrantObservationStarter struct {
	inner txStarter
	after func()
	once  *sync.Once
}

func (s triageGrantObservationStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return triageGrantObservationTx{Tx: tx, owner: s}, nil
}

type triageGrantObservationTx struct {
	pgx.Tx
	owner triageGrantObservationStarter
}

func (tx triageGrantObservationTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if !strings.Contains(sql, "-- name: LockProjectUpdateEvidenceTargets ") && !strings.Contains(sql, "-- name: ListAgentInvocationTargets ") {
		return rows, err
	}
	after := func() { tx.owner.once.Do(tx.owner.after) }
	if err != nil {
		after()
		return rows, err
	}
	return &issueGrantReadRows{Rows: rows, after: after}, nil
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
			// UpdateAgent replaces targets after its agent-row update. At this point
			// a grant replacement can already be running without an agent row lock.
			if err = db.New(holder).DeleteAgentInvocationTargets(ctx, parseUUID(agent)); err != nil {
				t.Fatal(err)
			}
			observed := false
			h.TxStarter = triageGrantObservationStarter{inner: h.TxStarter, once: &sync.Once{}, after: func() {
				observed = true
				if e := holder.Commit(ctx); e != nil {
					t.Error(e)
				}
			}}
			requestID := uuid.NewString()
			request := withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", requestID)).WithContext(ctx), "id", item.Issue.ID)
			testutil.Call(t, h.ActOnTriageItem, request).Want(http.StatusForbidden)
			if !observed {
				t.Fatal("grant read barrier did not run")
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
	inserted := false
	h.TxStarter = issueGrantReadStarter{base: h.TxStarter, after: func() {
		if inserted {
			return
		}
		inserted = true
		dbfx.Exec(t, "INSERT INTO agent_invocation_target(agent_id,target_type,target_id) VALUES($1,'member',$2)", agent, testUserID)
	}}
	requestID := uuid.NewString()
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", requestID)), "id", item.Issue.ID)).Want(http.StatusForbidden)
	if !inserted {
		t.Fatal("authorization never captured locked grant rows")
	}
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL", item.Issue.ID) != 1 || dbfx.Count(t, "SELECT count(*) FROM triage_action WHERE request_id=$1", requestID) != 0 {
		t.Fatal("unlocked inserted grant authorized acceptance")
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
