package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type passwordChangeBeforeBegin struct {
	base   txStarter
	before func()
}

func (s passwordChangeBeforeBegin) Begin(ctx context.Context) (pgx.Tx, error) {
	s.before()
	return s.base.Begin(ctx)
}

func TestPasswordSameValueChangeRechecksLockedCredential(t *testing.T) {
	for _, tc := range []struct {
		name       string
		restricted bool
		mutation   string
		status     int
		code       string
	}{
		{"restriction added", false, "must_change_password=true", 400, "password_unchanged"},
		{"version revoked", true, "session_version=session_version+1", 401, "session_revoked"},
		{"hash replaced", true, "password_hash=$2", 401, "session_revoked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			passwordTestSetup(t)
			registered := passwordRegister(t, fmt.Sprintf("locked%d", time.Now().UnixNano()))
			dbfx.Exec(t, `UPDATE user_password_credential SET must_change_password=$2 WHERE user_id=$1`, registered.User.ID, tc.restricted)
			replacementHash, err := auth.HashPassword(t.Context(), "replacement temporary password")
			if err != nil {
				t.Fatal(err)
			}
			h := *testHandler
			var snapshot db.UserPasswordCredential
			began := false
			h.TxStarter = passwordChangeBeforeBegin{base: testPool, before: func() {
				began = true
				// The mutation lands after current-password verification and before
				// the transaction's user lock, leaving its earlier snapshot stale.
				args := []any{registered.User.ID}
				if tc.name == "hash replaced" {
					args = append(args, replacementHash)
				}
				dbfx.Exec(t, `UPDATE user_password_credential SET `+tc.mutation+` WHERE user_id=$1`, args...)
				snapshot, err = h.Queries.GetPasswordCredential(t.Context(), parseUUID(registered.User.ID))
				if err != nil {
					t.Fatal(err)
				}
			}}
			response := passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "correct horse battery staple"}, registered.Token, h.PasswordChange).Want(tc.status)
			if !began || response.Map()["code"] != tc.code {
				t.Fatalf("same-password rejection did not use the locked snapshot: began=%t code=%v", began, response.Map()["code"])
			}
			after, err := h.Queries.GetPasswordCredential(t.Context(), parseUUID(registered.User.ID))
			if err != nil {
				t.Fatal(err)
			}
			if after.PasswordHash != snapshot.PasswordHash || after.SessionVersion != snapshot.SessionVersion || after.MustChangePassword != snapshot.MustChangePassword {
				t.Fatal("rejected same-value change overwrote the current credential")
			}
		})
	}
}
