package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"testing"
	"time"
)

func TestIterationNotificationDeliveryAndRevocation(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	q := db.New(fx.Pool)
	if err := EnqueueIterationNotifications(t.Context(), q, ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "start", []string{fx.UserID, fx.UserID}); err != nil {
		t.Fatal(err)
	}
	published := 0
	for range 2 {
		if err := s.DeliverIterationNotifications(t.Context(), func(item db.InboxItem) {
			published++
			var status string
			if err := fx.Pool.QueryRow(t.Context(), "SELECT status FROM iteration_notification WHERE id=$1", item.ID).Scan(&status); err != nil || status != "delivered" {
				t.Fatalf("callback before commit: %s %v", status, err)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if published != 1 {
		t.Fatalf("post-commit callback count: %d", published)
	}
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stable inbox: %d %v", count, err)
	}
	var status string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT status FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&status); err != nil || status != "delivered" {
		t.Fatalf("atomic delivered: %s %v", status, err)
	}
	if err := EnqueueIterationNotifications(t.Context(), q, ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "end", []string{fx.UserID}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Pool.Exec(t.Context(), "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, actor); err != nil {
		t.Fatal(err)
	}
	if err := s.DeliverIterationNotifications(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("revoked recipient received: %d %v", count, err)
	}
}

func TestIterationOverdueLocalDayAndFallback(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	runtime := fx.Runtime(t, "Reminder runtime")
	agent := fx.Agent(t, "Reminder agent", runtime)
	issue := fx.Issue(t, "Running work", testutil.Cols{"status": "in_progress"})
	task := fx.Task(t, agent, testutil.Cols{"issue_id": issue, "status": "running", "runtime_id": runtime, "originator_user_id": fx.UserID, "accountable_user_id": fx.UserID})
	var before string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT row_to_json(t)::text FROM agent_task_queue t WHERE id=$1", task).Scan(&before); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET status='active',started_by=$2,timezone='Asia/Shanghai',start_date='2026-10-01',end_date='2026-10-05' WHERE id=$1", iid, actor); err != nil {
		t.Fatal(err)
	}
	for _, stamp := range []string{"2026-10-05T15:59:00Z", "2026-10-05T16:00:00Z", "2026-10-05T23:00:00Z", "2026-10-06T16:00:00Z"} {
		now, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM iteration_notification WHERE workspace_id=$1 AND recipient_user_id=$2 AND kind='overdue'", ws, actor).Scan(&count); err != nil || count != 2 {
		t.Fatalf("daily fallback reminders: %d %v", count, err)
	}
	var status string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT status FROM iteration WHERE id=$1", iid).Scan(&status); err != nil || status != "active" {
		t.Fatalf("must not end: %s %v", status, err)
	}
	var after string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT row_to_json(t)::text FROM agent_task_queue t WHERE id=$1", task).Scan(&after); err != nil || after != before {
		t.Fatalf("reminder changed execution: %v", err)
	}
	var issueStatus string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT status FROM issue WHERE id=$1", issue).Scan(&issueStatus); err != nil || issueStatus != "in_progress" {
		t.Fatalf("reminder changed issue: %s %v", issueStatus, err)
	}
}

func TestIterationNotificationDeliveryCrashRetry(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, _ := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	// This database is exclusive. Fail after INSERT inbox but before the outbox
	// status update to reproduce a worker crash at the atomicity boundary.
	_, err := fx.Pool.Exec(t.Context(), `CREATE FUNCTION fail_iteration_delivery() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='delivered' THEN RAISE EXCEPTION 'injected delivery failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_iteration_delivery BEFORE UPDATE ON iteration_notification FOR EACH ROW EXECUTE FUNCTION fail_iteration_delivery()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = fx.Pool.Exec(t.Context(), "DROP TRIGGER IF EXISTS fail_iteration_delivery ON iteration_notification; DROP FUNCTION IF EXISTS fail_iteration_delivery()")
	})
	q := db.New(fx.Pool)
	if err = EnqueueIterationNotifications(t.Context(), q, ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "end", []string{fx.UserID}); err != nil {
		t.Fatal(err)
	}
	if err = s.DeliverIterationNotifications(t.Context(), func(db.InboxItem) { t.Error("rollback callback fired") }); err != nil {
		t.Fatal(err)
	}
	var count, attempts int
	if err = fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled back inbox: %d %v", count, err)
	}
	if err = fx.Pool.QueryRow(t.Context(), "SELECT attempts FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatalf("persisted retry: %d %v", attempts, err)
	}
	if _, err = fx.Pool.Exec(t.Context(), "DROP TRIGGER fail_iteration_delivery ON iteration_notification; DROP FUNCTION fail_iteration_delivery()"); err != nil {
		t.Fatal(err)
	}
	if _, err = fx.Pool.Exec(t.Context(), "UPDATE iteration_notification SET next_attempt_at=clock_timestamp() WHERE workspace_id=$1", ws); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- s.DeliverIterationNotifications(t.Context(), nil) }()
	}
	for range 2 {
		if err = <-results; err != nil {
			t.Fatal(err)
		}
	}
	if err = fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent restart exactly once: %d %v", count, err)
	}
}

func TestIterationOverdueSuppressedAfterDateExtension(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET status='active',started_by=$2,start_date='2020-01-01',end_date='2020-01-02' WHERE id=$1", iid, actor); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueIterationOverdueReminders(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET end_date='2099-01-01' WHERE id=$1", iid); err != nil {
		t.Fatal(err)
	}
	if err := s.DeliverIterationNotifications(t.Context(), func(db.InboxItem) { t.Fatal("suppressed callback fired") }); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := fx.Pool.QueryRow(t.Context(), "SELECT status FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&status); err != nil || status != "suppressed" {
		t.Fatalf("obsolete reminder: %s %v", status, err)
	}
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("obsolete inbox: %d %v", count, err)
	}
}

func TestIterationNotificationRevocationFence(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	q := db.New(fx.Pool)
	if err := EnqueueIterationNotifications(t.Context(), q, ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "start", []string{fx.UserID}); err != nil {
		t.Fatal(err)
	}
	revoke, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer revoke.Rollback(t.Context())
	rq := db.New(revoke)
	if err = rq.LockSubscriberWrites(t.Context(), db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- s.DeliverIterationNotifications(t.Context(), nil) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		err = fx.Pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%LockSubscriberWrites%')").Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("delivery did not wait on recipient fence")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err = revoke.Exec(t.Context(), "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, actor); err != nil {
		t.Fatal(err)
	}
	if err = revoke.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	var count int
	if err = fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("post-fence reauthorization: %d %v", count, err)
	}
}

func TestIterationOverdueRevokedCoordinatorDoesNotFallback(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	coordinator := fx.User(t, "Coordinator", uuid.NewString()+"@test.invalid")
	fx.Member(t, fx.WorkspaceID, coordinator, "member")
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET status='active',started_by=$2,coordinator_user_id=$3,start_date='2020-01-01',end_date='2020-01-02' WHERE id=$1", iid, actor, coordinator); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.Pool.Exec(t.Context(), "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, coordinator); err != nil {
		t.Fatal(err)
	}
	if err := s.EnqueueIterationOverdueReminders(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 0 {
		t.Fatalf("revoked coordinator must not fallback: %d %v", count, err)
	}
}

func TestIterationOverdueRejoinRetainsSuppression(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET status='active',started_by=$2,start_date='2020-01-01',end_date='2020-01-02' WHERE id=$1", iid, actor); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	q := db.New(tx)
	if err = q.LockSubscriberWrites(t.Context(), db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: actor}); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteIterationNotificationsForMember(t.Context(), db.DeleteIterationNotificationsForMemberParams{WorkspaceID: ws, UserID: actor}); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(t.Context(), "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", ws, actor); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	if err = s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	if err = s.DeliverIterationNotifications(t.Context(), func(db.InboxItem) { t.Fatal("rejoin reconstructed protected inbox") }); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM iteration_notification WHERE workspace_id=$1 AND status='suppressed'", ws).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retained dedup ledger: %d %v", count, err)
	}
}

func TestIterationOverdueDSTAndDisabled(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, actor := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE iteration SET status='active',started_by=$2,timezone='America/New_York',start_date='2026-10-01',end_date='2026-10-31' WHERE id=$1", iid, actor); err != nil {
		t.Fatal(err)
	}
	for _, stamp := range []string{"2026-11-01T05:30:00Z", "2026-11-01T06:30:00Z", "2026-11-02T05:00:00Z"} {
		now, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 2 {
		t.Fatalf("DST fold local-date dedup: %d %v", count, err)
	}
	if _, err := fx.Pool.Exec(t.Context(), "UPDATE workspace_iteration_settings SET enabled=false WHERE workspace_id=$1", ws); err != nil {
		t.Fatal(err)
	}
	now, _ := time.Parse(time.RFC3339, "2026-11-03T05:00:00Z")
	if err := s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM iteration_notification WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 2 {
		t.Fatalf("disabled scan created reminder: %d %v", count, err)
	}
}

func TestIterationNotificationRetryDeadLetter(t *testing.T) {
	fx, s := lifecycleFixture(t)
	iid := lifecycleCreate(t, fx, s)
	ws, _ := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	q := db.New(fx.Pool)
	if err := EnqueueIterationNotifications(t.Context(), q, ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "end", []string{fx.UserID}); err != nil {
		t.Fatal(err)
	}
	rows, err := q.ListDueIterationNotifications(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var id = rows[0].ID
	for attempt := 1; attempt <= 12; attempt++ {
		if err = q.RetryIterationNotification(t.Context(), db.RetryIterationNotificationParams{WorkspaceID: ws, ID: id}); err != nil {
			t.Fatal(err)
		}
		var attempts int
		var status string
		var delay float64
		if err = fx.Pool.QueryRow(t.Context(), "SELECT attempts,status,extract(epoch FROM next_attempt_at-clock_timestamp()) FROM iteration_notification WHERE id=$1", id).Scan(&attempts, &status, &delay); err != nil {
			t.Fatal(err)
		}
		expected := min(900, 5*(1<<min(attempt-1, 8)))
		if attempts != attempt || delay < float64(expected)-2 || delay > float64(expected) {
			t.Fatalf("retry %d: attempts=%d delay=%f want=%d", attempt, attempts, delay, expected)
		}
		if attempt == 12 && status != "dead_letter" {
			t.Fatalf("retry limit status: %s", status)
		}
	}
	counts, err := s.IterationNotificationCounts(t.Context())
	if err != nil || counts.Pending != 0 || counts.DeadLetter != 1 {
		t.Fatalf("queue health: %+v %v", counts, err)
	}
	if err = s.DeliverIterationNotifications(t.Context(), func(db.InboxItem) { t.Fatal("dead letter delivered automatically") }); err != nil {
		t.Fatal(err)
	}
}

func TestIterationOverdueScanBoundedAndDrains(t *testing.T) {
	fx, s := lifecycleFixture(t)
	for range 101 {
		ws := fx.Workspace(t, "Reminder batch", "reminder-"+uuid.NewString())
		fx.Member(t, ws, fx.UserID, "owner")
		fx.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": ws, "enabled": true}, "workspace_id=$1", ws)
		fx.Insert(t, "iteration", testutil.Cols{"workspace_id": ws, "name": "Overdue", "timezone": "UTC", "start_date": "2020-01-01", "end_date": "2020-01-02", "status": "active", "created_by": fx.UserID, "started_by": fx.UserID})
		fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", ws)
	}
	now := time.Now()
	q := db.New(fx.Pool)
	rows, err := q.ListActiveIterationsForReminder(t.Context(), pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil || len(rows) != 100 {
		t.Fatalf("bounded discovery: %d %v", len(rows), err)
	}
	if err = s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	rows, err = q.ListActiveIterationsForReminder(t.Context(), pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil || len(rows) != 1 {
		t.Fatalf("undelivered page remains discoverable: %d %v", len(rows), err)
	}
	if err = s.EnqueueIterationOverdueReminders(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	rows, err = q.ListActiveIterationsForReminder(t.Context(), pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil || len(rows) != 0 {
		t.Fatalf("daily scan fully drained: %d %v", len(rows), err)
	}
}

func TestIterationNotificationWorkerResumesPersistedDelivery(t *testing.T) {
	for _, name := range []string{"enabled", "disabled"} {
		t.Run(name, func(t *testing.T) {
			fx, setup := lifecycleFixture(t)
			iid := lifecycleCreate(t, fx, setup)
			ws, _ := lifecycleIDs(fx)
			fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
			fx.Exec(t, "UPDATE workspace_iteration_settings SET enabled=$2 WHERE workspace_id=$1", fx.WorkspaceID, name == "enabled")
			if err := EnqueueIterationNotifications(t.Context(), db.New(fx.Pool), ws, util.MustParseUUID(iid), util.MustParseUUID(uuid.NewString()), "dates_changed", []string{fx.UserID}); err != nil {
				t.Fatal(err)
			}
			// Startup must resume retained outbox work without rollout configuration,
			// including historical notifications after a workspace was disabled.
			s := &IterationService{TxStarter: fx.Pool}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			published := 0
			s.RunIterationNotifications(ctx, func(item db.InboxItem) {
				published++
				if n := fx.Count(t, "SELECT count(*) FROM iteration_notification WHERE id=$1 AND status='delivered'", item.ID); n != 1 {
					t.Fatal("worker published before delivery committed")
				}
				cancel()
			})
			if published != 1 {
				t.Fatalf("worker did not resume persisted delivery: %d", published)
			}
			if err := s.DeliverIterationNotifications(t.Context(), func(db.InboxItem) { published++ }); err != nil {
				t.Fatal(err)
			}
			if published != 1 || fx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws) != 1 {
				t.Fatalf("worker did not deliver exactly once: %d", published)
			}
		})
	}
}

func TestIterationNotificationDisableWorkspaceSummary(t *testing.T) {
	fx, s := lifecycleFixture(t)
	ws, _ := lifecycleIDs(fx)
	fx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1", fx.WorkspaceID)
	fx.Cleanup(t, "DELETE FROM iteration_notification WHERE workspace_id=$1", fx.WorkspaceID)
	op := util.MustParseUUID(uuid.NewString())
	q := db.New(fx.Pool)
	for range 2 {
		if err := EnqueueIterationNotifications(t.Context(), q, ws, pgtype.UUID{}, op, "disable", []string{fx.UserID, fx.UserID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeliverIterationNotifications(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	var title string
	var details []byte
	var count int
	if err := fx.Pool.QueryRow(t.Context(), "SELECT title,details FROM inbox_item WHERE workspace_id=$1", ws).Scan(&title, &details); err != nil {
		t.Fatal(err)
	}
	if title != "Iterations disabled" || string(details) != `{"kind": "disable"}` {
		t.Fatalf("workspace summary leaked unrelated period: %s %s", title, details)
	}
	if err := fx.Pool.QueryRow(t.Context(), "SELECT count(*) FROM inbox_item WHERE workspace_id=$1", ws).Scan(&count); err != nil || count != 1 {
		t.Fatalf("disable summary dedup: %d %v", count, err)
	}
}
