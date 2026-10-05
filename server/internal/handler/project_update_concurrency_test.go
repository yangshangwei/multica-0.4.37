package handler

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type progressTxStarter struct {
	base txStarter
	wrap func(pgx.Tx) pgx.Tx
}

func (s progressTxStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, e := s.base.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return s.wrap(tx), nil
}

type progressHookTx struct {
	pgx.Tx
	afterQuery   func(string)
	beforeQuery  func(string)
	beforeExec   func(string)
	queryFailure func(string) error
}
type progressErrorRow struct{ err error }

func (r progressErrorRow) Scan(...any) error { return r.err }
func (tx progressHookTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.beforeQuery != nil {
		tx.beforeQuery(sql)
	}
	if tx.queryFailure != nil {
		if e := tx.queryFailure(sql); e != nil {
			return progressErrorRow{e}
		}
	}
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if tx.afterQuery != nil {
		tx.afterQuery(sql)
	}
	return row
}
func (tx progressHookTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.queryFailure != nil {
		if e := tx.queryFailure(sql); e != nil {
			return nil, e
		}
	}
	rows, err := tx.Tx.Query(ctx, sql, args...)
	if tx.afterQuery != nil {
		tx.afterQuery(sql)
	}
	return rows, err
}
func (tx progressHookTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.beforeExec != nil {
		tx.beforeExec(sql)
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func progressNotification(t *testing.T) (string, string, db.ProjectUpdateNotification) {
	t.Helper()
	p := progressProject(t)
	recipient := dbfx.User(t, "Pending recipient", "progress-notice-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, recipient, "member")
	dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2", testWorkspaceID, recipient)
	progressCall(t, "create", p, "", progressInput(progressPreview(t, p, progressDraft("[@Recipient](mention://member/"+recipient+")"))), 201)
	rows, e := testHandler.Queries.ListDueProjectUpdateNotifications(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if uuidToString(r.ProjectID) == p {
			return p, recipient, r
		}
	}
	t.Fatal("notification absent")
	return "", "", db.ProjectUpdateNotification{}
}
func TestProjectUpdateNotificationConcurrentWorkersAndCorrection(t *testing.T) {
	p, recipient, row := progressNotification(t)
	ctx := context.Background()
	start := make(chan struct{})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; done <- testHandler.deliverProjectUpdateNotification(ctx, row) }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent notification workers deadlocked")
		}
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 1 {
		t.Fatalf("inbox=%d", n)
	}
	newRecipient := dbfx.User(t, "New mention", "progress-new-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, newRecipient, "member")
	dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2", testWorkspaceID, newRecipient)
	d := progressDraft("[@Old](mention://member/" + recipient + ") [@New](mention://member/" + newRecipient + ")")
	d["operation"] = "correct"
	d["update_id"] = uuidToString(row.UpdateID)
	d["expected_revision"] = 1
	d["correction_reason"] = "add current recipient"
	in := progressInput(progressPreview(t, p, d))
	progressCall(t, "correct", p, uuidToString(row.UpdateID), in, 200)
	progressCall(t, "correct", p, uuidToString(row.UpdateID), in, 200)
	if e := testHandler.deliverProjectUpdateNotifications(ctx); e != nil {
		t.Fatal(e)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update_notification WHERE project_id=$1", p); n != 2 {
		t.Fatalf("correction outbox=%d", n)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE recipient_id IN ($1,$2) AND type='project_update'", recipient, newRecipient); n != 2 {
		t.Fatalf("correction inbox=%d", n)
	}
}
func TestProjectUpdateNotificationFailureRecoveryAndDeadLetter(t *testing.T) {
	_, _, row := progressNotification(t)
	h := *testHandler
	h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
		return progressHookTx{Tx: tx, queryFailure: func(sql string) error {
			if strings.Contains(sql, "-- name: CreateProjectUpdateInbox") {
				return &pgconn.PgError{Code: "08006", Message: "injected database error"}
			}
			return nil
		}}
	}}
	if e := h.deliverProjectUpdateNotifications(context.Background()); e != nil {
		t.Fatal(e)
	}
	var status string
	var attempts int
	var next time.Time
	dbfx.QueryRow(t, "SELECT status,attempts,next_attempt_at FROM project_update_notification WHERE id=$1", row.ID).Scan(&status, &attempts, &next)
	if status != "pending" || attempts != 1 || !next.After(time.Now().Add(3*time.Second)) {
		t.Fatalf("retry=%s/%d/%s", status, attempts, next)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 0 {
		t.Fatalf("failed transaction retained inbox=%d", n)
	}
	dbfx.Exec(t, "UPDATE project_update_notification SET next_attempt_at=NULL WHERE id=$1", row.ID)
	if e := testHandler.deliverProjectUpdateNotifications(context.Background()); e != nil {
		t.Fatal(e)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 1 {
		t.Fatalf("recovery inbox=%d", n)
	}
	// Recovery from a historical committed inbox with a pending outbox must
	// retain the same identity and must not publish another inbox event.
	dbfx.Exec(t, "UPDATE project_update_notification SET status='pending',next_attempt_at=NULL WHERE id=$1", row.ID)
	if e := testHandler.deliverProjectUpdateNotifications(context.Background()); e != nil {
		t.Fatal(e)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 1 {
		t.Fatalf("replayed delivery inbox=%d", n)
	}
	dbfx.Exec(t, "UPDATE project_update_notification SET status='pending',attempts=11,next_attempt_at=NULL WHERE id=$1", row.ID)
	if e := h.deliverProjectUpdateNotifications(context.Background()); e != nil {
		t.Fatal(e)
	}
	dbfx.QueryRow(t, "SELECT status,attempts FROM project_update_notification WHERE id=$1", row.ID).Scan(&status, &attempts)
	if status != "dead_letter" || attempts != 12 {
		t.Fatalf("dead letter=%s/%d", status, attempts)
	}
	if e := testHandler.deliverProjectUpdateNotifications(context.Background()); e != nil {
		t.Fatal(e)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 1 {
		t.Fatalf("dead letter recreated inbox=%d", n)
	}
	for _, tc := range []struct {
		attempt int32
		want    time.Duration
	}{{1, 5 * time.Second}, {2, 10 * time.Second}, {9, 15 * time.Minute}, {12, 15 * time.Minute}} {
		if got := projectNotificationBackoff(tc.attempt, 0); got != tc.want {
			t.Fatalf("backoff(%d)=%s want=%s", tc.attempt, got, tc.want)
		}
	}
}
func TestProjectUpdateNotificationDeletionBothLockOrders(t *testing.T) {
	for _, deliveryFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "delivery-first", false: "delete-first"}[deliveryFirst], func(t *testing.T) {
			p, _, row := progressNotification(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			h := *testHandler
			h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
				return progressHookTx{Tx: tx, afterQuery: func(sql string) {
					target := "-- name: LockProjectForAssociation"
					if !deliveryFirst {
						target = "-- name: LockProjectForExecutionSquad"
					}
					if strings.Contains(sql, target) {
						once.Do(func() {
							close(entered)
							select {
							case <-release:
							case <-ctx.Done():
							}
						})
					}
				}}
			}}
			deliveryDone, deleteDone := make(chan error, 1), make(chan int, 1)
			deliver := func(handler *Handler) { deliveryDone <- handler.deliverProjectUpdateNotification(ctx, row) }
			deleteProject := func(handler *Handler) {
				req := testutil.WithURLParams(newRequest("DELETE", "/api/projects/"+p, nil).WithContext(ctx), "id", p)
				deleteDone <- testutil.Call(t, handler.DeleteProject, req).Code
			}
			if deliveryFirst {
				go deliver(&h)
			} else {
				go deleteProject(&h)
			}
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("first operation did not acquire the project lock")
			}
			blockedPID := make(chan uint32, 1)
			var secondOnce sync.Once
			second := *testHandler
			second.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
				return progressHookTx{Tx: tx, beforeQuery: func(sql string) {
					target := "-- name: LockProjectForAssociation"
					if deliveryFirst {
						target = "-- name: LockProjectForExecutionSquad"
					}
					if strings.Contains(sql, target) {
						secondOnce.Do(func() { blockedPID <- tx.Conn().PgConn().PID() })
					}
				}}
			}}
			if deliveryFirst {
				go deleteProject(&second)
			} else {
				go deliver(&second)
			}
			select {
			case pid := <-blockedPID:
				progressAwaitDatabaseLock(t, ctx, pid)
			case <-ctx.Done():
				t.Fatal("second operation did not request its project lock")
			}
			close(release)
			select {
			case e := <-deliveryDone:
				if e != nil {
					t.Fatal(e)
				}
			case <-ctx.Done():
				t.Fatal("delivery/delete deadlock")
			}
			select {
			case code := <-deleteDone:
				if code != 204 {
					t.Fatalf("delete code=%d", code)
				}
			case <-ctx.Done():
				t.Fatal("delete/delivery deadlock")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 0 {
				t.Fatalf("deleted project left inbox=%d", n)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM project_update_notification WHERE id=$1", row.ID); n != 0 {
				t.Fatalf("deleted project left outbox=%d", n)
			}
		})
	}
}
func TestProjectUpdateNotificationRevocationBothLockOrders(t *testing.T) {
	for _, deliveryFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "delivery-first", false: "revoke-first"}[deliveryFirst], func(t *testing.T) {
			_, recipient, row := progressNotification(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if deliveryFirst {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				h := *testHandler
				h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
					return progressHookTx{Tx: tx, afterQuery: func(sql string) {
						if strings.Contains(sql, "-- name: LockProjectUpdateNotification") {
							once.Do(func() { close(entered); <-release })
						}
					}}
				}}
				done := make(chan error, 1)
				go func() { done <- h.deliverProjectUpdateNotification(ctx, row) }()
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal("delivery barrier")
				}
				revoked := make(chan error, 1)
				blockedPID := make(chan uint32, 1)
				go func() { revoked <- progressRevoke(ctx, recipient, blockedPID) }()
				select {
				case pid := <-blockedPID:
					progressAwaitDatabaseLock(t, ctx, pid)
				case <-ctx.Done():
					t.Fatal("revoke did not request recipient fence")
				}
				close(release)
				if e := <-done; e != nil {
					t.Fatal(e)
				}
				if e := <-revoked; e != nil {
					t.Fatal(e)
				}
			} else {
				tx, e := testPool.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				defer tx.Rollback(ctx)
				q := testHandler.Queries.WithTx(tx)
				if e = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: row.WorkspaceID, UserID: row.RecipientUserID}); e != nil {
					t.Fatal(e)
				}
				if _, e = tx.Exec(ctx, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", row.WorkspaceID, row.RecipientUserID); e != nil {
					t.Fatal(e)
				}
				entered := make(chan uint32, 1)
				var once sync.Once
				h := *testHandler
				h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
					return progressHookTx{Tx: tx, beforeExec: func(sql string) {
						if strings.Contains(sql, "-- name: LockSubscriberWrites") {
							once.Do(func() { entered <- tx.Conn().PgConn().PID() })
						}
					}}
				}}
				done := make(chan error, 1)
				go func() { done <- h.deliverProjectUpdateNotification(ctx, row) }()
				select {
				case pid := <-entered:
					progressAwaitDatabaseLock(t, ctx, pid)
				case <-ctx.Done():
					t.Fatal("recipient fence barrier")
				}
				if e = tx.Commit(ctx); e != nil {
					t.Fatal(e)
				}
				if e = <-done; e != nil {
					t.Fatal(e)
				}
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", row.ID); n != 0 {
				t.Fatalf("revocation recreated inbox=%d", n)
			}
			if !deliveryFirst {
				var status string
				dbfx.QueryRow(t, "SELECT status FROM project_update_notification WHERE id=$1", row.ID).Scan(&status)
				if status != "cancelled" {
					t.Fatalf("revoked notification status=%s", status)
				}
			}
		})
	}
}
func progressRevoke(ctx context.Context, recipient string, observed ...chan uint32) error {
	tx, e := testPool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	q := testHandler.Queries.WithTx(tx)
	if _, e = q.LockWorkspaceForChatSessionCreate(ctx, parseUUID(testWorkspaceID)); e != nil {
		return e
	}
	if len(observed) > 0 {
		observed[0] <- tx.Conn().PgConn().PID()
	}
	if e = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(recipient)}); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2", testWorkspaceID, recipient); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, recipient); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func progressAwaitDatabaseLock(t *testing.T, ctx context.Context, pid uint32) {
	t.Helper()
	for ctx.Err() == nil {
		var waiting bool
		if e := testPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')", pid).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Millisecond):
		}
	}
	t.Fatal("second transaction never entered a database lock wait")
}

func TestProjectUpdateMembershipLockRejectsRevokedRRSnapshot(t *testing.T) {
	p := progressProject(t)
	user := dbfx.User(t, "Publishing member", "progress-actor-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, user, "member")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, e := testPool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	q := testHandler.Queries.WithTx(tx)
	if e = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(user)}); e != nil {
		t.Fatal(e)
	}
	entered := make(chan struct{})
	var once sync.Once
	h := *testHandler
	h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
		return progressHookTx{Tx: tx, beforeExec: func(sql string) {
			if strings.Contains(sql, "-- name: LockSubscriberWrites") {
				once.Do(func() { close(entered) })
			}
		}}
	}}
	done := make(chan int, 1)
	go func() {
		req := testutil.WithURLParams(newRequest("POST", "/api/projects/"+p+"/updates/preview", progressDraft("Old membership")).WithContext(ctx), "id", p)
		req.Header.Set("X-User-ID", user)
		done <- testutil.Call(t, h.PreviewProjectUpdate, req).Code
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("actor did not reach its fence after taking RR snapshot")
	}
	if _, e = tx.Exec(ctx, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, user); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case status := <-done:
		if status != 403 {
			t.Fatalf("stale RR membership status=%d", status)
		}
	case <-ctx.Done():
		t.Fatal("revoked preview did not exit")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 0 {
		t.Fatalf("revoked member published %d", n)
	}
}

func TestProjectUpdateUnavailableStatisticsNeverPublish(t *testing.T) {
	p := progressProject(t)
	d := progressDraft("Unavailable facts")
	d["include_statistics"] = true
	h := *testHandler
	h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
		return progressHookTx{Tx: tx, queryFailure: func(sql string) error {
			if strings.Contains(sql, "-- name: ListProjectHealthIssues") {
				return &pgconn.PgError{Code: "08006", Message: "injected statistics failure"}
			}
			return nil
		}}
	}}
	req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", d), "id", p)
	out := testutil.Call(t, h.PreviewProjectUpdate, req).Want(503).Map()
	if out["code"] != "project_health_unavailable" {
		t.Fatalf("statistics error code=%v", out)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 0 {
		t.Fatalf("unavailable facts published %d", n)
	}
}

func TestProjectUpdateEvidenceGrantRevocationRestartsRRAuthorization(t *testing.T) {
	p := progressProject(t)
	user := dbfx.User(t, "Grant member", "progress-grant-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, user, "member")
	runtime := dbfx.Runtime(t, "Grant runtime")
	agent := dbfx.Agent(t, "Granted evidence", runtime, testutil.Cols{"permission_mode": "public_to"})
	task := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime})
	dbfx.InsertNoID(t, "agent_invocation_target", testutil.Cols{"agent_id": agent, "target_type": "member", "target_id": user}, "agent_id=$1 AND target_id=$2", agent, user)
	d := progressDraft("Grant evidence")
	d["evidence"] = []any{map[string]any{"kind": "execution", "id": task, "url": nil}}
	request := func() *http.Request {
		req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", d), "id", p)
		req.Header.Set("X-User-ID", user)
		return req
	}
	testutil.Call(t, testHandler.PreviewProjectUpdate, request()).Want(200)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, e := testPool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2", agent, user); e != nil {
		t.Fatal(e)
	}
	reached := make(chan struct{})
	var once sync.Once
	h := *testHandler
	h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
		return progressHookTx{Tx: tx, afterQuery: func(sql string) {
			if strings.Contains(sql, "-- name: LockProjectUpdateEvidenceTargets") {
				once.Do(func() { close(reached) })
			}
		}}
	}}
	done := make(chan int, 1)
	go func() {
		req := testutil.WithURLParams(request().WithContext(ctx), "id", p)
		done <- testutil.Call(t, h.PreviewProjectUpdate, req).Code
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("evidence grant locking read was not reached")
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case status := <-done:
		if status != 403 {
			t.Fatalf("revoked grant status=%d", status)
		}
	case <-ctx.Done():
		t.Fatal("grant retry stalled")
	}
}
