package main

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestInboxSnoozeRollbackGuardsEveryIndexStep(t *testing.T) {
	for _, version := range []string{
		"886_inbox_snooze", "887_inbox_snooze_identity", "888_inbox_snooze_due", "889_inbox_critical_source_identity",
	} {
		if preRollbackHooks[version] == nil {
			t.Errorf("%s can roll back before checking retained inbox snoozes", version)
		}
	}
}

func TestInboxSnoozeRollbackRefusesPendingPlans(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires an explicitly allocated migrated rollback test database")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal("rollback test database unavailable")
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	user := fx.User(t, "Rollback fixture", "rollback-inbox-"+uuid.NewString()+"@example.test")
	ws := fx.Workspace(t, "Rollback fixture", "rollback-inbox-"+uuid.NewString())
	fx.UserID, fx.WorkspaceID = user, ws
	fx.Member(t, ws, user, "owner")
	issue := fx.Issue(t, "Retained reminder")
	id := fx.Insert(t, "inbox_snooze", testutil.Cols{
		"workspace_id": ws, "recipient_type": "member", "recipient_id": user,
		"group_kind": "issue", "group_id": issue, "snoozed_until": testutil.Raw("now()+interval '1 hour'"),
	})
	versions := []string{"886_inbox_snooze", "887_inbox_snooze_identity", "888_inbox_snooze_due", "889_inbox_critical_source_identity"}
	for _, version := range versions {
		if err := preRollbackHooks[version](context.Background(), pool); err == nil || err.Error() != "restore retained inbox snoozes before rollback" {
			t.Errorf("%s did not return the retained future reminder refusal: %v", version, err)
		}
	}
	fx.Exec(t, "UPDATE inbox_snooze SET snoozed_until=now()-interval '1 hour' WHERE id=$1", id)
	for _, version := range versions {
		if err := preRollbackHooks[version](context.Background(), pool); err == nil || err.Error() != "restore retained inbox snoozes before rollback" {
			t.Errorf("%s did not refuse the unacknowledged overdue reminder: %v", version, err)
		}
	}
	fx.Exec(t, "UPDATE inbox_snooze SET snoozed_until=NULL WHERE id=$1", id)
	for _, version := range versions {
		if err := preRollbackHooks[version](context.Background(), pool); err != nil {
			t.Errorf("%s refused rollback after reminders were restored: %v", version, err)
		}
	}
}
