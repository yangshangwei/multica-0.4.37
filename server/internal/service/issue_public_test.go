package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueContentDeletedWorkspacePreservesConditionalConflict(t *testing.T) {
	pool := newTaskClaimRacePool(t)
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	fx.UserID = fx.User(t, "Content actor", suffix+"@test.invalid")
	fx.WorkspaceID = fx.Workspace(t, "Deleted content workspace", suffix)
	id := util.MustParseUUID(fx.Issue(t, "Content before workspace deletion"))
	q := db.New(pool)
	stale, err := q.GetIssue(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `DELETE FROM issue WHERE id=$1`, id)
	fx.Exec(t, `DELETE FROM workspace WHERE id=$1`, fx.WorkspaceID)
	svc := &IssueService{Queries: q, TxStarter: pool}
	title := "Must not restore deleted content"
	revision := int64(1)
	authorize := func(context.Context, pgx.Tx) (json.RawMessage, error) {
		t.Error("authorization should not run without workspace")
		return nil, nil
	}
	if _, err = svc.UpdateContent(t.Context(), stale, IssueContentPatch{Title: &title, ExpectedRevision: &revision}, authorize); !errors.Is(err, ErrIssueRevisionConflict) {
		t.Fatalf("conditional deleted workspace error=%v", err)
	}
	if _, err = svc.UpdateContent(t.Context(), stale, IssueContentPatch{Title: &title}, authorize); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unconditional deleted workspace error=%v", err)
	}
}
