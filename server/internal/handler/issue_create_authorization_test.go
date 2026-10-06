package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// The HTTP admission checks run before the I1 fence. A creator must retain
// the same agent authority when the owning transaction finally proceeds.
func TestIssueCreateAgentAuthorizationAfterFence(t *testing.T) {
	for _, transport := range []string{"task_token", "legacy"} {
		for _, change := range []string{"demotion", "archive", "task_deleted", "task_reassigned", "completed", "failed", "cancelled"} {
			t.Run(transport+"/"+change, func(t *testing.T) {
				agentID, taskID := autonomyTestAgent(t, "Create authority", "contributor")
				otherID := dbfx.Agent(t, "Other creator", testRuntimeID)
				title := "Creation authority must remain current"
				dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, testUserID, taskID)
				dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
				dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1 AND title=$2)`, testWorkspaceID, title)
				syncCreationIterationCounter(t)
				beforeCounter := dbfx.Count(t, `SELECT issue_counter FROM workspace WHERE id=$1`, testWorkspaceID)
				h, wakeups, eventCount := lifecycleIsolatedHandler()
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				holder, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer holder.Rollback(context.Background())
				if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
					t.Fatal(err)
				}
				request := asAgent(newRequest(http.MethodPost, "/api/issues", map[string]any{"title": title, "assignee_type": "agent", "assignee_id": otherID}), agentID, taskID).WithContext(ctx)
				if transport == "task_token" {
					request.Header.Set("X-Actor-Source", "task_token")
				}
				result := make(chan *testutil.Response, 1)
				go func() { result <- testutil.Call(t, h.CreateIssue, request) }()
				if !waitForWaiterBlockedBy(t, int(holder.Conn().PgConn().PID()), 3*time.Second) {
					t.Fatal("creator did not wait on the I1 fence")
				}
				// These writes also prove the creator takes neither reference lock
				// before I1, which would invert execution/deletion lock order.
				switch change {
				case "demotion":
					_, err = holder.Exec(ctx, `UPDATE agent SET autonomy_level='observer' WHERE id=$1`, agentID)
				case "archive":
					_, err = holder.Exec(ctx, `UPDATE agent SET archived_at=now() WHERE id=$1`, agentID)
				case "task_deleted":
					_, err = holder.Exec(ctx, `DELETE FROM agent_task_queue WHERE id=$1`, taskID)
				case "task_reassigned":
					_, err = holder.Exec(ctx, `UPDATE agent_task_queue SET agent_id=$1 WHERE id=$2`, otherID, taskID)
				default:
					_, err = holder.Exec(ctx, `UPDATE agent_task_queue SET status=$1 WHERE id=$2`, change, taskID)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = holder.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				response := <-result
				if response.Code != http.StatusForbidden {
					t.Errorf("lost authority created work: HTTP %d %s", response.Code, response.Body.String())
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title); n != 0 {
					t.Errorf("created %d issues after authority changed", n)
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id IS NOT NULL`, otherID); n != 0 {
					t.Errorf("created %d dispatch tasks after authority changed", n)
				}
				if n := dbfx.Count(t, `SELECT issue_counter FROM workspace WHERE id=$1`, testWorkspaceID); n != beforeCounter {
					t.Errorf("failed create changed counter: %d -> %d", beforeCounter, n)
				}
				if eventCount.Load() != 0 || wakeups.calls.Load() != 0 {
					t.Errorf("failed create emitted events=%d wakeups=%d", eventCount.Load(), wakeups.calls.Load())
				}
			})
		}
	}
}

func TestIssueCreateAssignmentAuthorizationAfterFence(t *testing.T) {
	for _, actor := range []string{"member", "agent"} {
		for _, change := range []string{"grant_removed", "target_archived", "squad_archived", "leader_replaced", "member_removed"} {
			t.Run(actor+"/"+change, func(t *testing.T) {
				creatorID, taskID := autonomyTestAgent(t, "Assignment creator", "contributor")
				dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, testUserID, taskID)
				otherUser := dbfx.User(t, "Other owner", uuidToString(dbid.NewV7())+"@example.test")
				target := dbfx.Agent(t, "Assignment target", testRuntimeID, testutil.Cols{"owner_id": otherUser, "permission_mode": "public_to"})
				dbfx.Exec(t, `INSERT INTO agent_invocation_target (agent_id,target_type,target_id) VALUES ($1,'member',$2)`, target, testUserID)
				dbfx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target)
				kind, assigned := "agent", target
				squad := ""
				member := ""
				if change == "squad_archived" || change == "leader_replaced" {
					squad = dbfx.Squad(t, "Create assignment squad", target)
					kind, assigned = "squad", squad
				} else if change == "member_removed" {
					member = dbfx.Member(t, testWorkspaceID, otherUser, "member")
					kind, assigned = "member", otherUser
				}
				replacement := dbfx.Agent(t, "Unauthorized replacement "+uuidToString(dbid.NewV7()), testRuntimeID, testutil.Cols{"owner_id": otherUser})
				title := "Assignment must use current permission"
				dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
				dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1 AND title=$2)`, testWorkspaceID, title)
				syncCreationIterationCounter(t)
				beforeCounter := dbfx.Count(t, `SELECT issue_counter FROM workspace WHERE id=$1`, testWorkspaceID)
				h, wakeups, eventCount := lifecycleIsolatedHandler()
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				holder, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer holder.Rollback(context.Background())
				if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
					t.Fatal(err)
				}
				request := newRequest(http.MethodPost, "/api/issues", map[string]any{"title": title, "assignee_type": kind, "assignee_id": assigned}).WithContext(ctx)
				if actor == "agent" {
					asAgent(request, creatorID, taskID)
					request.Header.Set("X-Actor-Source", "task_token")
				}
				result := make(chan *testutil.Response, 1)
				go func() { result <- testutil.Call(t, h.CreateIssue, request) }()
				if !waitForWaiterBlockedBy(t, int(holder.Conn().PgConn().PID()), 3*time.Second) {
					t.Fatal("creator did not wait on I1")
				}
				want := http.StatusBadRequest
				switch change {
				case "grant_removed":
					_, err = holder.Exec(ctx, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target)
					want = http.StatusForbidden
				case "target_archived":
					_, err = holder.Exec(ctx, `UPDATE agent SET archived_at=now() WHERE id=$1`, target)
				case "squad_archived":
					_, err = holder.Exec(ctx, `UPDATE squad SET archived_at=now() WHERE id=$1`, squad)
				case "leader_replaced":
					_, err = holder.Exec(ctx, `UPDATE squad SET leader_id=$1 WHERE id=$2`, replacement, squad)
					want = http.StatusForbidden
				case "member_removed":
					_, err = holder.Exec(ctx, `DELETE FROM member WHERE id=$1`, member)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = holder.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				response := <-result
				if response.Code != want {
					t.Errorf("stale assignment HTTP %d, want %d: %s", response.Code, want, response.Body.String())
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title); n != 0 {
					t.Errorf("created %d issues with revoked assignment", n)
				}
				if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id IN ($1,$2) AND issue_id IS NOT NULL`, target, replacement); n != 0 {
					t.Errorf("created %d tasks with revoked assignment", n)
				}
				if n := dbfx.Count(t, `SELECT issue_counter FROM workspace WHERE id=$1`, testWorkspaceID); n != beforeCounter {
					t.Errorf("failed create changed counter: %d -> %d", beforeCounter, n)
				}
				if eventCount.Load() != 0 || wakeups.calls.Load() != 0 {
					t.Errorf("failed create emitted events=%d wakeups=%d", eventCount.Load(), wakeups.calls.Load())
				}
			})
		}
	}
}

func TestIssueCreateAgentAuthorizationHeldThroughCommit(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Create authority held", "contributor")
	title := "Authority stays locked through creation"
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
	syncCreationIterationCounter(t)
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
	h := *testHandler
	svc := *h.IssueService
	svc.TxStarter, h.IssueService = barrier, &svc
	request := asAgent(newRequest(http.MethodPost, "/api/issues", map[string]any{"title": title, "status": "backlog"}), agentID, taskID).WithContext(ctx)
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.CreateIssue, request) }()
	select {
	case <-barrier.reached:
	case response := <-result:
		t.Fatalf("create did not reach commit: HTTP %d %s", response.Code, response.Body.String())
	case <-ctx.Done():
		t.Fatal("create did not reach commit")
	}
	for _, row := range []struct{ table, id string }{{"agent", agentID}, {"agent_task_queue", taskID}} {
		probe, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = probe.Exec(ctx, "SELECT id FROM "+row.table+" WHERE id=$1 FOR NO KEY UPDATE NOWAIT", row.id)
		_ = probe.Rollback(context.Background())
		var busy *pgconn.PgError
		if !errors.As(err, &busy) || busy.Code != "55P03" {
			t.Errorf("%s authorization can change before commit: %v", row.table, err)
		}
	}
	close(barrier.release)
	(<-result).Want(http.StatusCreated)
}

func TestIssueCreateAgentAuthorizationRechecksRetry(t *testing.T) {
	for _, change := range []string{"demotion", "task_reassigned"} {
		t.Run(change, func(t *testing.T) {
			agentID, taskID := autonomyTestAgent(t, "Retry creator", "contributor")
			otherID := dbfx.Agent(t, "Retry replacement", testRuntimeID)
			title := "Retry must keep original actor"
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
			syncCreationIterationCounter(t)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			query := "-- name: LockLifecycleAgent "
			if change == "demotion" {
				_, err = holder.Exec(ctx, `UPDATE agent SET autonomy_level='observer' WHERE id=$1`, agentID)
			} else {
				query = "-- name: LockLifecycleOriginTask "
				_, err = holder.Exec(ctx, `UPDATE agent_task_queue SET agent_id=$1 WHERE id=$2`, otherID, taskID)
			}
			if err != nil {
				t.Fatal(err)
			}
			h := *testHandler
			svc := *h.IssueService
			attempts := 0
			svc.TxStarter = lifecycleHookStarter{base: testPool, after: func(sql string, queryErr error) {
				if !strings.Contains(sql, query) {
					return
				}
				attempts++
				if attempts == 1 {
					var busy *pgconn.PgError
					if !errors.As(queryErr, &busy) || busy.Code != "55P03" {
						t.Errorf("expected a real NOWAIT refusal, got %v", queryErr)
					}
					if err := holder.Commit(ctx); err != nil {
						t.Error(err)
					}
				}
			}}
			h.IssueService = &svc
			request := asAgent(newRequest(http.MethodPost, "/api/issues", map[string]any{"title": title}), agentID, taskID).WithContext(ctx)
			request.Header.Set("X-Actor-Source", "task_token")
			response := testutil.Call(t, h.CreateIssue, request)
			if response.Code != http.StatusForbidden {
				t.Errorf("retry with changed authority: HTTP %d %s", response.Code, response.Body.String())
			}
			if attempts != 2 {
				t.Errorf("reference reads=%d, want first refusal and one reauthorized attempt", attempts)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title); n != 0 {
				t.Errorf("retry created %d issues under changed actor", n)
			}
		})
	}
}

func TestIssueCreateTerminalTaskRetainsOrdinaryAuthority(t *testing.T) {
	for _, targetKind := range []string{"unassigned", "workspace_public"} {
		t.Run(targetKind, func(t *testing.T) {
			agentID, taskID := autonomyTestAgent(t, "Terminal ordinary creator", "contributor")
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, testUserID, taskID)
			title := "Terminal task ordinary create"
			body := map[string]any{"title": title, "status": "backlog"}
			if targetKind == "workspace_public" {
				target := createHandlerTestAgent(t, "Terminal public target", nil)
				body["assignee_type"], body["assignee_id"] = "agent", target
			}
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
			syncCreationIterationCounter(t)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			request := asAgent(newRequest(http.MethodPost, "/api/issues", body), agentID, taskID).WithContext(ctx)
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, testHandler.CreateIssue, request) }()
			if !waitForWaiterBlockedBy(t, int(holder.Conn().PgConn().PID()), 3*time.Second) {
				t.Fatal("creator did not wait on I1")
			}
			if _, err = holder.Exec(ctx, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, taskID); err != nil {
				t.Fatal(err)
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var created IssueResponse
			(<-result).Want(http.StatusCreated).JSON(&created)
			var kind, creator string
			var noOrigin bool
			dbfx.QueryRow(t, `SELECT creator_type,creator_id,origin_type IS NULL AND origin_id IS NULL FROM issue WHERE id=$1`, created.ID).Scan(&kind, &creator, &noOrigin)
			if kind != "agent" || creator != agentID || !noOrigin {
				t.Errorf("terminal task identity/provenance: kind=%s creator=%s no_origin=%v", kind, creator, noOrigin)
			}
		})
	}
}

func TestIssueCreateAssignmentReferencesHeldThroughCommit(t *testing.T) {
	for _, kind := range []string{"agent", "squad", "member"} {
		t.Run(kind, func(t *testing.T) {
			creator, task := autonomyTestAgent(t, "Reference creator", "contributor")
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, testUserID, task)
			owner := dbfx.User(t, "Reference target owner", uuidToString(dbid.NewV7())+"@example.test")
			member := dbfx.Member(t, testWorkspaceID, owner, "member")
			target := dbfx.Agent(t, "Reference target", testRuntimeID, testutil.Cols{"owner_id": owner, "permission_mode": "public_to"})
			dbfx.Exec(t, `INSERT INTO agent_invocation_target (agent_id,target_type,target_id) VALUES ($1,'member',$2)`, target, testUserID)
			dbfx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target)
			assigned := target
			probes := []struct{ sql, id string }{
				{`SELECT id FROM agent WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, target},
				{`SELECT agent_id FROM agent_invocation_target WHERE agent_id=$1 FOR UPDATE NOWAIT`, target},
			}
			if kind == "squad" {
				assigned = dbfx.Squad(t, "Reference squad", target)
				probes = append(probes, struct{ sql, id string }{`SELECT id FROM squad WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, assigned})
			} else if kind == "member" {
				assigned = owner
				probes = []struct{ sql, id string }{{`SELECT id FROM member WHERE id=$1 FOR UPDATE NOWAIT`, member}}
			}
			title := "Create assignment reference commit"
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
			syncCreationIterationCounter(t)
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
			h := *testHandler
			svc := *h.IssueService
			svc.TxStarter, h.IssueService = barrier, &svc
			request := asAgent(newRequest(http.MethodPost, "/api/issues", map[string]any{"title": title, "status": "backlog", "assignee_type": kind, "assignee_id": assigned}), creator, task).WithContext(ctx)
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, h.CreateIssue, request) }()
			select {
			case <-barrier.reached:
			case response := <-result:
				t.Fatalf("create stopped before commit: HTTP %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("create did not reach commit")
			}
			for _, query := range probes {
				probe, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				_, err = probe.Exec(ctx, query.sql, query.id)
				_ = probe.Rollback(context.Background())
				var busy *pgconn.PgError
				if !errors.As(err, &busy) || busy.Code != "55P03" {
					t.Errorf("reference can change before commit (%s): %v", query.sql, err)
				}
			}
			close(barrier.release)
			(<-result).Want(http.StatusCreated)
		})
	}
}

// A different human lends invocation rights to an agent task. The request's
// runtime-owner member lock does not stop that human's grant being revoked.
func TestIssueAssignmentGrantHeldThroughCommit(t *testing.T) {
	for _, writer := range []string{"update", "lifecycle"} {
		for _, kind := range []string{"agent", "squad"} {
			t.Run(writer+"/"+kind, func(t *testing.T) {
				source := lifecycleAtomicSource(t, "Assignment grant source")
				originator := dbfx.User(t, "Delegating member", uuidToString(dbid.NewV7())+"@example.test")
				dbfx.Member(t, testWorkspaceID, originator, "member")
				creator, task := autonomyTestAgent(t, "Grant-holder creator", "contributor")
				dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, originator, task)
				target := dbfx.Agent(t, "Grant-holder target", testRuntimeID, testutil.Cols{"permission_mode": "public_to"})
				dbfx.Exec(t, `INSERT INTO agent_invocation_target (agent_id,target_type,target_id) VALUES ($1,'member',$2)`, target, originator)
				dbfx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target)
				assigned := target
				if kind == "squad" {
					assigned = dbfx.Squad(t, "Grant-holder squad", target)
				}
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
				h := *testHandler
				h.TxStarter = barrier
				body := map[string]any{"assignee_type": kind, "assignee_id": assigned}
				method, handler, want := http.MethodPut, h.UpdateIssue, http.StatusOK
				if writer == "lifecycle" {
					body = lifecycleAtomicAssignedBody(kind, assigned)
					method, handler, want = http.MethodPost, h.CreateLifecycleHandoff, http.StatusCreated
				}
				request := withURLParam(asAgent(newRequest(method, "/api/issues/"+source, body), creator, task).WithContext(ctx), "id", source)
				result := make(chan *testutil.Response, 1)
				go func() { result <- testutil.Call(t, handler, request) }()
				select {
				case <-barrier.reached:
				case response := <-result:
					t.Fatalf("writer stopped before commit: HTTP %d %s", response.Code, response.Body.String())
				case <-ctx.Done():
					t.Fatal("writer did not reach commit")
				}
				probe, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				_, err = probe.Exec(ctx, `SELECT agent_id FROM agent_invocation_target WHERE agent_id=$1 FOR UPDATE NOWAIT`, target)
				_ = probe.Rollback(context.Background())
				var busy *pgconn.PgError
				if !errors.As(err, &busy) || busy.Code != "55P03" {
					t.Errorf("invocation grant can disappear before %s commits: %v", writer, err)
				}
				close(barrier.release)
				(<-result).Want(want)
			})
		}
	}
}

type issueGrantReadStarter struct {
	base  txStarter
	after func()
}

func (s issueGrantReadStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return issueGrantReadTx{Tx: tx, after: s.after}, nil
}

type issueGrantReadTx struct {
	pgx.Tx
	after func()
}

func (tx issueGrantReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if err != nil || !strings.Contains(sql, "-- name: LockProjectUpdateEvidenceTargets ") {
		return rows, err
	}
	return &issueGrantReadRows{Rows: rows, after: tx.after}, nil
}

type issueGrantReadRows struct {
	pgx.Rows
	after func()
	once  sync.Once
}

func (rows *issueGrantReadRows) Close() {
	rows.Rows.Close()
	rows.once.Do(rows.after)
}

// UpdateAgent writes its grant replacement after its agent update commits. A
// newly inserted grant must not be used unless this transaction locked it.
func TestIssueAssignmentIgnoresGrantInsertedAfterLockRead(t *testing.T) {
	for _, writer := range []string{"create", "update", "lifecycle"} {
		t.Run(writer, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "Late grant source")
			originator := dbfx.User(t, "Late grant member", uuidToString(dbid.NewV7())+"@example.test")
			dbfx.Member(t, testWorkspaceID, originator, "member")
			creator, task := autonomyTestAgent(t, "Late grant creator", "contributor")
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, originator, task)
			target := dbfx.Agent(t, "Late grant target", testRuntimeID, testutil.Cols{"permission_mode": "public_to"})
			dbfx.Exec(t, `INSERT INTO agent_invocation_target (agent_id,target_type,target_id) VALUES ($1,'member',$2)`, target, originator)
			dbfx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target)
			title := "Late grant must not authorize"
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, target)
			before := lifecycleAtomicSnapshot(t, source)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			if _, err = holder.Exec(ctx, `DELETE FROM agent_invocation_target WHERE agent_id=$1`, target); err != nil {
				t.Fatal(err)
			}
			h := *testHandler
			inserted := false
			starter := issueGrantReadStarter{base: testPool, after: func() {
				if inserted {
					return
				}
				inserted = true
				if _, e := testPool.Exec(ctx, `INSERT INTO agent_invocation_target (agent_id,target_type,target_id) VALUES ($1,'member',$2)`, target, originator); e != nil {
					t.Error(e)
				}
			}}
			h.TxStarter = starter
			svc := *h.IssueService
			svc.TxStarter, h.IssueService = starter, &svc
			body := map[string]any{"title": title, "assignee_type": "agent", "assignee_id": target}
			method, handler := http.MethodPost, h.CreateIssue
			if writer == "update" {
				method, handler = http.MethodPut, h.UpdateIssue
			} else if writer == "lifecycle" {
				body, handler = lifecycleAtomicAssignedBody("agent", target), h.CreateLifecycleHandoff
			}
			request := withURLParam(asAgent(newRequest(method, "/api/issues/"+source, body), creator, task).WithContext(ctx), "id", source)
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, handler, request) }()
			if !waitForWaiterBlockedBy(t, int(holder.Conn().PgConn().PID()), 3*time.Second) {
				t.Fatal("writer did not wait on I1")
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			response := <-result
			if response.Code != http.StatusForbidden {
				t.Errorf("unlocked inserted grant authorized %s: HTTP %d %s", writer, response.Code, response.Body.String())
			}
			if !inserted {
				t.Fatal("grant read hook did not run")
			}
			if after := lifecycleAtomicSnapshot(t, source); after != before {
				t.Fatal("denied late-grant operation changed issue, counter, audit or dispatch state")
			}
		})
	}
}
