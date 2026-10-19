package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationResultNotificationMigrationPreservesHistoryAndFencesRollback(t *testing.T) {
	fx := newCollaborationHoldMigrationFixture(t)
	fx.Exec(t, `CREATE TABLE collaboration_notification (LIKE public.collaboration_notification INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
	fx.Exec(t, `ALTER TABLE collaboration_notification DROP COLUMN run_id`)
	read := func(direction string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "729_collaboration_result_notification."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	legacy := fx.Insert(t, "collaboration_notification", testutil.Cols{"recipient_user_id": uuid.NewString(), "issue_id": uuid.NewString(), "event_seq": 1, "kind": "created"})
	fx.Exec(t, read("up"))
	fx.Exec(t, read("up"))
	if fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE id=$1 AND run_id IS NULL AND kind='created'`, legacy) != 1 {
		t.Fatal("additive upgrade changed legacy notification")
	}
	run := uuid.NewString()
	result := fx.Insert(t, "collaboration_notification", testutil.Cols{"recipient_user_id": uuid.NewString(), "issue_id": uuid.NewString(), "event_seq": 2, "kind": "ai_completed", "run_id": run})
	if _, err := fx.Pool.Exec(context.Background(), read("down")); err == nil || !strings.Contains(err.Error(), "retained collaboration result notifications") {
		t.Fatalf("rollback removed retained result links: %v", err)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE id=$1 AND run_id=$2`, result, run) != 1 {
		t.Fatal("refused rollback changed retained link")
	}
	fx.Exec(t, `DELETE FROM collaboration_notification WHERE id=$1`, result)
	holder, err := fx.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(context.Background(), `INSERT INTO collaboration_notification(recipient_user_id,issue_id,event_seq,kind,run_id) VALUES($1,$2,3,'ai_failed',$3)`, uuid.NewString(), uuid.NewString(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = fx.Pool.Exec(bounded, read("down"))
	cancel()
	var busy *pgconn.PgError
	if !errors.As(err, &busy) || busy.Code != "55P03" {
		t.Fatalf("rollback failed to fence an uncommitted first result: %v", err)
	}
	if err = holder.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, read("down"))
	if fx.Count(t, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='collaboration_notification' AND column_name='run_id'`) != 0 || fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE id=$1`, legacy) != 1 {
		t.Fatal("empty maintenance rollback lost legacy notification or retained new column")
	}
}
