package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func runDeploymentRecovery(t *testing.T, f *platformAdminFixture, p DeploymentPasswordRecoveryParams, auditFailure bool) (DeploymentPasswordRecoveryResult, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var writer pgx.Tx = tx
	if auditFailure {
		writer = platformAdminFailAuditTx{Tx: tx}
	}
	result, err := RecoverPasswordForDeploymentInTx(ctx, writer, p)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func TestPlatformPasswordRecoveryLastAdminRequiresBreakGlass(t *testing.T) {
	f := newPlatformAdminFixture(t)
	user := f.user(t, PlatformRoleSuperAdmin)
	p := DeploymentPasswordRecoveryParams{TargetUserID: user, PasswordHash: "temporary-secret-hash", Reason: "Lost last administrator password", RequestID: uuid.NewString()}
	_, err := runDeploymentRecovery(t, f, p, false)
	assertPlatformAdminError(t, err, "last_super_admin")
	credential, err := f.svc.Queries.GetPasswordCredential(t.Context(), user)
	if err != nil || credential.SessionVersion != 1 || credential.MustChangePassword || credential.PasswordHash != f.hash {
		t.Fatalf("refused recovery changed credentials: %+v, %v", credential, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id=$1", user); n != 0 {
		t.Fatal("refused recovery persisted an operation")
	}
	p.BreakGlass = true
	result, err := runDeploymentRecovery(t, f, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.EffectiveSuperAdmins != 0 || result.Operation.ActorKind != "deployment_operator" || result.Operation.ActorID.Valid {
		t.Fatalf("invalid break-glass result: %+v", result)
	}
	credential, err = f.svc.Queries.GetPasswordCredential(t.Context(), user)
	if err != nil || !credential.MustChangePassword || credential.SessionVersion != 2 {
		t.Fatalf("break-glass did not restrict temporary credentials: %+v, %v", credential, err)
	}
	role, err := f.svc.Queries.GetPlatformRole(t.Context(), user)
	if err != nil || role != PlatformRoleSuperAdmin {
		t.Fatal("recovery changed platform role", err)
	}
	var afterRaw, auditText []byte
	f.fx.QueryRow(t, "SELECT after_state, to_jsonb(a)::text FROM admin_audit_event a WHERE operation_id=$1", result.Operation.ID).Scan(&afterRaw, &auditText)
	var after struct {
		BreakGlass           bool  `json:"break_glass"`
		EffectiveSuperAdmins int64 `json:"effective_super_admins"`
	}
	if err := json.Unmarshal(afterRaw, &after); err != nil || !after.BreakGlass || after.EffectiveSuperAdmins != 0 {
		t.Fatalf("audit omitted no-administrator window: %s, %v", afterRaw, err)
	}
	if strings.Contains(string(auditText), p.PasswordHash) {
		t.Fatal("password hash leaked into recovery audit")
	}
}

func TestPlatformPasswordRecoveryPreservesOtherAdmin(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, PlatformRoleSuperAdmin)
	f.user(t, PlatformRoleSuperAdmin)
	p := DeploymentPasswordRecoveryParams{TargetUserID: target, PasswordHash: "temporary-hash", Reason: "Recover named administrator", RequestID: uuid.NewString()}
	result, err := runDeploymentRecovery(t, f, p, false)
	if err != nil || result.EffectiveSuperAdmins != 1 {
		t.Fatalf("ordinary administrator recovery = %+v, %v", result, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id=$1", result.Operation.ID); n != 1 {
		t.Fatal("ordinary recovery was not audited")
	}
}

func TestPlatformPasswordRecoveryAuditFailureRollsBack(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, "")
	pat := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "recovery rollback", "token_hash": uuid.NewString(), "token_prefix": "mul_", "auth_version": int64(1)})
	p := DeploymentPasswordRecoveryParams{TargetUserID: target, PasswordHash: "temporary-hash", Reason: "Recover ordinary account", RequestID: uuid.NewString()}
	_, err := runDeploymentRecovery(t, f, p, true)
	if err == nil || !strings.Contains(err.Error(), "audit persistence failed") {
		t.Fatalf("expected audit failure, got %v", err)
	}
	credential, err := f.svc.Queries.GetPasswordCredential(t.Context(), target)
	if err != nil || credential.SessionVersion != 1 || credential.PasswordHash != f.hash || credential.MustChangePassword {
		t.Fatal("audit failure left changed credentials", err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM personal_access_token WHERE id=$1 AND NOT revoked", pat); n != 1 {
		t.Fatal("audit failure revoked the original PAT")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id=$1", target); n != 0 {
		t.Fatal("audit failure persisted an operation")
	}
}

func TestPlatformPasswordRecoveryRejectsBreakGlassForOrdinaryAccount(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, "")
	_, err := runDeploymentRecovery(t, f, DeploymentPasswordRecoveryParams{TargetUserID: target, PasswordHash: "temporary-hash", Reason: "Invalid exception", RequestID: uuid.NewString(), BreakGlass: true}, false)
	assertPlatformAdminError(t, err, "break_glass_not_applicable")
	if _, err := f.svc.Queries.GetPlatformRole(t.Context(), target); err != pgx.ErrNoRows {
		t.Fatal("break-glass granted a platform role", err)
	}
}

func TestPlatformPasswordRecoveryDigestExcludesPasswordHash(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, "")
	p := DeploymentPasswordRecoveryParams{TargetUserID: target, PasswordHash: "first-secret-hash", Reason: "Repeat deployment recovery", RequestID: uuid.NewString()}
	first, err := runDeploymentRecovery(t, f, p, false)
	if err != nil {
		t.Fatal(err)
	}
	p.PasswordHash = "second-secret-hash"
	p.RequestID = uuid.NewString()
	second, err := runDeploymentRecovery(t, f, p, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Operation.PayloadHash != second.Operation.PayloadHash {
		t.Fatal("secret changed the non-secret recovery digest")
	}
	if first.Operation.ID == second.Operation.ID || second.Operation.IdempotencyKey == first.Operation.IdempotencyKey {
		t.Fatal("distinct CLI invocations reused one operation")
	}
}

func TestPlatformPasswordRecoveryConcurrentAdminsPreserveOneEffectiveAdmin(t *testing.T) {
	f := newPlatformAdminFixture(t)
	first := f.user(t, PlatformRoleSuperAdmin)
	second := f.user(t, PlatformRoleSuperAdmin)
	results := make(chan error, 2)
	for _, user := range []pgtype.UUID{first, second} {
		go func() {
			ctx := context.Background()
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				results <- err
				return
			}
			defer tx.Rollback(ctx)
			_, err = RecoverPasswordForDeploymentInTx(ctx, tx, DeploymentPasswordRecoveryParams{TargetUserID: user, PasswordHash: "temporary-hash", Reason: "Concurrent recovery", RequestID: uuid.NewString()})
			if err == nil {
				err = tx.Commit(ctx)
			}
			results <- err
		}()
	}
	var succeeded, rejected int
	for range 2 {
		err := <-results
		var apiErr *PlatformAdminError
		switch {
		case err == nil:
			succeeded++
		case errors.As(err, &apiErr) && apiErr.Code == "last_super_admin":
			rejected++
		default:
			t.Fatalf("unexpected recovery result: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent recoveries: %d succeeded, %d rejected", succeeded, rejected)
	}
	count, err := CountEffectiveSuperAdmins(t.Context(), f.svc.Queries)
	if err != nil || count != 1 {
		t.Fatalf("effective administrators = %d, %v; want 1", count, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE target_id IN ($1,$2)", first, second); n != 1 {
		t.Fatalf("concurrent recoveries produced %d audits, want 1", n)
	}
}
