package handler

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func iterationNoticeFixture(t *testing.T, fx *testutil.Fixture, workspace, user string) string {
	t.Helper()
	id := fx.Insert(t, "iteration_notification", testutil.Cols{"workspace_id": workspace, "operation_id": uuid.NewString(), "recipient_user_id": user, "kind": "ended"})
	fx.Insert(t, "inbox_item", testutil.Cols{"id": id, "workspace_id": workspace, "recipient_type": "member", "recipient_id": user, "type": "iteration_ended", "title": "Protected iteration summary", "severity": "info"})
	return id
}

func TestIterationMemberRevocationClearsOnlyProtectedRecipientData(t *testing.T) {
	user := dbfx.User(t, "Iteration recipient", uuid.NewString()+"@example.invalid")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	target := iterationNoticeFixture(t, dbfx, testWorkspaceID, user)
	retained := iterationNoticeFixture(t, dbfx, testWorkspaceID, testUserID)
	foreignWS := dbfx.Workspace(t, "Other notice workspace", "notice-"+uuid.NewString())
	foreign := testutil.New(testPool, foreignWS, user)
	foreign.Member(t, foreignWS, user, "member")
	elsewhere := iterationNoticeFixture(t, foreign, foreignWS, user)
	stored := storedIterationOperationFixture(t, user)
	if _, err := testHandler.revokeAndRemoveMember(context.Background(), parseUUID(testWorkspaceID), parseUUID(user), parseUUID(member), parseUUID(testUserID)); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"iteration_notification", "inbox_item"} {
		if table == "inbox_item" {
			if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE id=$1", target); n != 0 {
				t.Fatal("revoked member retained protected inbox")
			}
		} else if n := dbfx.Count(t, "SELECT count(*) FROM iteration_notification WHERE id=$1 AND status='suppressed'", target); n != 1 {
			t.Fatal("revocation lost suppression ledger")
		}
		if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE id=ANY($1::uuid[])", []string{retained, elsewhere}); n != 2 {
			t.Fatalf("revocation removed another recipient or workspace from %s", table)
		}
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM iteration_operation WHERE id=$1", stored.OperationID); n != 1 {
		t.Fatal("revocation erased durable audit operation")
	}
	request := iterationOperationRequest(stored.RequestID)
	request.Header.Set("X-User-ID", user)
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(403)
}

type iterationRevokeFailureStarter struct{ inner txStarter }

func (s iterationRevokeFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationRevokeFailureTx{tx}, nil
}

type iterationRevokeFailureTx struct{ pgx.Tx }

func (tx *iterationRevokeFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "DeleteMember :exec") {
		return pgconn.CommandTag{}, errors.New("injected member delete failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestIterationMemberRevocationRollsBackNotificationCleanup(t *testing.T) {
	user := dbfx.User(t, "Rollback recipient", uuid.NewString()+"@example.invalid")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	notice := iterationNoticeFixture(t, dbfx, testWorkspaceID, user)
	h := *testHandler
	h.TxStarter = iterationRevokeFailureStarter{h.TxStarter}
	_, err := h.revokeAndRemoveMember(context.Background(), parseUUID(testWorkspaceID), parseUUID(user), parseUUID(member), parseUUID(testUserID))
	if err == nil || !strings.Contains(err.Error(), "injected member delete failure") {
		t.Fatalf("expected final member deletion failure, got %v", err)
	}
	for _, table := range []string{"iteration_notification", "inbox_item"} {
		if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE id=$1", notice); n != 1 {
			t.Fatalf("%s escaped revoke rollback", table)
		}
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM member WHERE id=$1", member); n != 1 {
		t.Fatal("failed revoke deleted member")
	}
}
