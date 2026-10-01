package service

import (
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestPlatformAccountDisableRestoreKeepsHistoryAndNeverRevivesCredentials(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	pat := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "original", "token_hash": uuid.NewString(), "token_prefix": "mul_old", "auth_version": 1})
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: "disable", ExpectedAuthVersion: 1, Reason: "Account access review", Password: platformAdminTestPassword, IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
	result, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	if err != nil || !result.CredentialsRevoked || result.TargetUserID != target {
		t.Fatalf("disable = %+v, %v", result, err)
	}
	if _, err := auth.CheckPasswordVersion(t.Context(), f.svc.Queries, util.UUIDToString(target), 1); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("disabled credential valid: %v", err)
	}
	if _, err := f.svc.Authorize(adminTestContext(actor, 1), true); err != nil {
		t.Fatalf("actor revoked: %v", err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM "user" WHERE id=$1`, target); n != 1 {
		t.Fatal("disabled account was deleted")
	}
	replay, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	if err != nil || !replay.Replayed || replay.Operation.ID != result.Operation.ID {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	p.Action = "restore"
	p.ExpectedAuthVersion = 2
	p.IdempotencyKey = randomAccountKey()
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
		t.Fatal(err)
	}
	for _, version := range []int64{1, 2} {
		if _, err := auth.CheckPasswordVersion(t.Context(), f.svc.Queries, util.UUIDToString(target), version); !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("old version %d revived", version)
		}
	}
	if _, err := auth.CheckPasswordVersion(t.Context(), f.svc.Queries, util.UUIDToString(target), 3); err != nil {
		t.Fatal(err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM personal_access_token WHERE id=$1 AND revoked", pat); n != 1 {
		t.Fatal("restore revived old PAT")
	}
}

func TestPlatformAccountRecoveryReplayNeverRehashesSecret(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: "recover-password", ExpectedAuthVersion: 1, Reason: "Lost credentials", Password: platformAdminTestPassword, TemporaryPassword: "temporary-password-42", IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
	first, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.svc.Queries.GetPasswordCredential(context.Background(), target)
	if err != nil || !c.MustChangePassword || c.SessionVersion != 2 {
		t.Fatalf("recovery credential = %+v, %v", c, err)
	}
	if ok, err := auth.VerifyPassword(t.Context(), c.PasswordHash, p.TemporaryPassword); err != nil || !ok {
		t.Fatal("temporary password not stored correctly", err)
	}
	p.TemporaryPassword = "different-temporary-password"
	second, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	if err != nil || !second.Replayed || first.Operation.ID != second.Operation.ID {
		t.Fatalf("recovery replay = %+v, %v", second, err)
	}
	after, err := f.svc.Queries.GetPasswordCredential(t.Context(), target)
	if err != nil || after.PasswordHash != c.PasswordHash {
		t.Fatal("replay changed password")
	}
}

func TestPlatformAccountDenialsAndAuditFailureAreAtomic(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: actor, Action: "disable", ExpectedAuthVersion: 1, Reason: "Invalid self action", Password: platformAdminTestPassword, IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
	_, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	assertPlatformAdminError(t, err, "self_action_forbidden")
	p.TargetUserID = target
	p.Password = "wrong-password"
	_, err = f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	assertPlatformAdminError(t, err, "password_verification_failed")
	p.Password = platformAdminTestPassword
	f.svc.TxStarter = platformAdminFailAuditStarter{f.pool}
	_, err = f.svc.ChangeAccount(adminTestContext(actor, 1), p)
	if err == nil {
		t.Fatal("audit failure accepted")
	}
	user, err := f.svc.Queries.GetUser(t.Context(), target)
	if err != nil || user.DisabledAt.Valid {
		t.Fatal("audit failure persisted disable", err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation"); n != 0 {
		t.Fatal("failed account operation persisted")
	}
}

func randomAccountKey() pgtype.UUID { return pgtype.UUID{Bytes: uuid.New(), Valid: true} }

func TestPlatformAccountLegacyDisableRestoreRevokesMigrationSession(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor := f.user(t, PlatformRoleSuperAdmin)
	legacyString := f.fx.User(t, "Legacy account", uuid.NewString()+"@example.invalid")
	legacy, err := util.ParseUUID(legacyString)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	claims := jwt.MapClaims{"sub": legacyString, "iat": float64(time.Now().Add(-time.Hour).Unix()), "exp": float64(time.Now().Add(time.Hour).Unix())}
	if session, err := auth.CheckPasswordJWT(t.Context(), f.svc.Queries, claims); err != nil || !session.Setup {
		t.Fatalf("legacy control = %+v, %v", session, err)
	}
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: legacy, Action: "disable", ExpectedAuthVersion: 0, Reason: "Disable legacy account", Password: platformAdminTestPassword, IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
		t.Fatal(err)
	}
	p.Action = "restore"
	p.IdempotencyKey = randomAccountKey()
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CheckPasswordJWT(t.Context(), f.svc.Queries, claims); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("legacy session revived after restore: %v", err)
	}
	p.Action = "recover-password"
	p.Username = "legacy_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	p.TemporaryPassword = "temporary-password-42"
	p.IdempotencyKey = randomAccountKey()
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
		t.Fatal(err)
	}
	credential, err := f.svc.Queries.GetPasswordCredential(t.Context(), legacy)
	if err != nil || !credential.MustChangePassword || credential.SessionVersion != 1 {
		t.Fatalf("legacy recovery = %+v, %v", credential, err)
	}
}

func TestPlatformAccountConcurrentMutualDisablePreservesOneAdmin(t *testing.T) {
	f := newPlatformAdminFixture(t)
	a, b := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin)
	results := make(chan error, 2)
	for _, pair := range [][2]pgtype.UUID{{a, b}, {b, a}} {
		go func(actor, target pgtype.UUID) {
			_, err := f.svc.ChangeAccount(adminTestContext(actor, 1), PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: "disable", ExpectedAuthVersion: 1, Reason: "Concurrent access review", Password: platformAdminTestPassword, IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()})
			results <- err
		}(pair[0], pair[1])
	}
	success := 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successful disables=%d", success)
	}
	count, err := CountEffectiveSuperAdmins(t.Context(), f.svc.Queries)
	if err != nil || count != 1 {
		t.Fatalf("effective administrators=%d, %v", count, err)
	}
}

func TestPlatformAccountRestorePreservesForcedPasswordChange(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	p := PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: "recover-password", ExpectedAuthVersion: 1, Reason: "Recover then review access", Password: platformAdminTestPassword, TemporaryPassword: "temporary-password-42", IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
	for _, step := range []struct {
		action  string
		version int64
	}{{"recover-password", 1}, {"disable", 2}, {"restore", 3}} {
		p.Action = step.action
		p.ExpectedAuthVersion = step.version
		p.IdempotencyKey = randomAccountKey()
		if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
			t.Fatalf("%s: %v", step.action, err)
		}
	}
	credential, err := f.svc.Queries.GetPasswordCredential(t.Context(), target)
	if err != nil || !credential.MustChangePassword || credential.SessionVersion != 4 {
		t.Fatalf("restore cleared recovery state: %+v, %v", credential, err)
	}
}

func TestPlatformAccountRevokedActorCannotFinishQueuedChange(t *testing.T) {
	f := newPlatformAdminFixture(t)
	admin, revoked, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	reached, resume := make(chan struct{}), make(chan struct{})
	delayed := NewPlatformAdminService(f.svc.Queries, platformAdminPausedStarter{f.pool, reached, resume})
	done := make(chan error, 1)
	go func() {
		_, err := delayed.ChangeAccount(adminTestContext(revoked, 1), PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: "disable", ExpectedAuthVersion: 1, Reason: "Queued access change", Password: platformAdminTestPassword, IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()})
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("queued operation did not finish password verification")
	}
	_, err := f.svc.ChangeRole(adminTestContext(admin, 1), f.roleChange(revoked, nil, adminRolePointer(PlatformRoleSuperAdmin)))
	close(resume)
	if err != nil {
		t.Fatal(err)
	}
	assertPlatformAdminError(t, <-done, "admin_forbidden")
	user, err := f.svc.Queries.GetUser(t.Context(), target)
	if err != nil || user.DisabledAt.Valid {
		t.Fatalf("revoked actor changed account: %v", err)
	}
}
