package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func initializationParams(t *testing.T, f *platformAdminFixture) PlatformAdminInitializationParams {
	t.Helper()
	deployment := uuid.New()
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "true")
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment.String())
	return PlatformAdminInitializationParams{OrganizationID: f.org, DeploymentID: pgtype.UUID{Bytes: deployment, Valid: true}, InitializationID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, PasswordHash: f.hash, Reason: "Offline administrator initialization", RequestID: uuid.NewString()}
}
func createInitialization(t *testing.T, f *platformAdminFixture, p PlatformAdminInitializationParams) db.PlatformAdminInitialization {
	t.Helper()
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	marker, err := CreatePlatformAdminInitializationInTx(context.Background(), tx, p)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return marker
}
func inspectInitialization(t *testing.T, f *platformAdminFixture, p PlatformAdminInitializationParams) PlatformAdminInitializationState {
	t.Helper()
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	state, err := InspectPlatformAdminInitializationInTx(context.Background(), tx, p.OrganizationID, p.DeploymentID)
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func TestPlatformAdminInitializationCreatesAtomicRestrictedIdentity(t *testing.T) {
	f := newPlatformAdminFixture(t)
	p := initializationParams(t, f)
	t.Setenv("ALLOW_SIGNUP", "false")
	marker := createInitialization(t, f, p)
	if marker.InitializationID != p.InitializationID || marker.State != "created" || marker.InitialCredentialVersion.Int64 != 1 {
		t.Fatal("incorrect immutable provenance")
	}
	credential, err := f.svc.Queries.GetPasswordCredential(context.Background(), marker.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != InitialPlatformAdminUsername || !credential.MustChangePassword || credential.PasswordHash != f.hash {
		t.Fatal("initial credential contract violated")
	}
	var counts []int
	f.fx.QueryRow(t, `SELECT ARRAY[(SELECT count(*)::int FROM "user"),(SELECT count(*)::int FROM user_password_credential),(SELECT count(*)::int FROM platform_role_binding),(SELECT count(*)::int FROM platform_admin_initialization),(SELECT count(*)::int FROM admin_operation),(SELECT count(*)::int FROM admin_audit_event)]`).Scan(&counts)
	for _, n := range counts {
		if n != 1 {
			t.Fatalf("atomic row counts: %v", counts)
		}
	}
	var audit string
	f.fx.QueryRow(t, `SELECT row_to_json(a)::text FROM admin_audit_event a`).Scan(&audit)
	if strings.Contains(audit, f.hash) || strings.Contains(audit, platformAdminTestPassword) || !strings.Contains(audit, "deployment_operator") {
		t.Fatal("audit does not preserve secret boundary")
	}
	count, err := CountEffectiveSuperAdmins(context.Background(), f.svc.Queries)
	if err != nil || count != 0 {
		t.Fatal("restricted principal became an effective administrator")
	}
	state := inspectInitialization(t, f, p)
	if state.Status != "already_initialized" || !state.Credential.UserID.Valid {
		t.Fatal("pending credential not identified")
	}
}
func TestPlatformAdminInitializationPreservesEveryAdministrationEvidence(t *testing.T) {
	for _, scenario := range []string{"observer", "ready", "pending", "disabled", "orphan_role", "bootstrap_history", "before_audit", "after_audit"} {
		t.Run(scenario, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			p := initializationParams(t, f)
			switch scenario {
			case "observer":
				f.user(t, PlatformRoleObserver)
			case "ready":
				f.user(t, PlatformRoleSuperAdmin)
			case "pending":
				id := f.user(t, PlatformRoleSuperAdmin)
				f.fx.Exec(t, "UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1", id)
			case "disabled":
				f.user(t, PlatformRoleSuperAdmin, testutil.Cols{"disabled_at": "2026-01-01T00:00:00Z"})
			case "orphan_role":
				f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": uuid.NewString(), "role": PlatformRoleSuperAdmin}, "true")
			case "bootstrap_history":
				f.fx.Insert(t, "admin_operation", testutil.Cols{"organization_id": f.org, "actor_kind": "deployment_operator", "target_kind": "user", "target_id": uuid.NewString(), "kind": "user.role.bootstrap", "idempotency_key": uuid.NewString(), "payload_hash": "historical", "reason": "Prior applied grant", "state": "applied"})
			default:
				col := "before_state"
				if scenario == "after_audit" {
					col = "after_state"
				}
				f.fx.Insert(t, "admin_audit_event", testutil.Cols{"organization_id": f.org, "actor_kind": "deployment_operator", "target_kind": "user", "target_id": uuid.NewString(), "action": "user.role", "phase": "applied", "request_id": uuid.NewString(), "reason": "Prior applied grant", "result_code": "role_changed", col: `{"role":"super_admin"}`})
			}
			state := inspectInitialization(t, f, p)
			if state.Status != "preserved_existing_admin" {
				t.Fatalf("status=%s", state.Status)
			}
			tx, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if err = LockPlatformAdminMutation(context.Background(), tx); err != nil {
				t.Fatal(err)
			}
			marker, err := PreservePlatformAdminInitializationInTx(context.Background(), tx, f.org)
			if err != nil {
				t.Fatal(err)
			}
			if marker.State != "preserved_existing_admin" || marker.DeploymentID.Valid || marker.UserID.Valid {
				t.Fatal("invalid preserved marker")
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			f.fx.Exec(t, "DELETE FROM platform_role_binding")
			f.fx.Exec(t, "DELETE FROM admin_operation")
			f.fx.Exec(t, "DELETE FROM admin_audit_event")
			if got := inspectInitialization(t, f, p); got.Status != "preserved_existing_admin" {
				t.Fatal("history removal reopened initialization")
			}
		})
	}
}
func TestPlatformAdminInitializationNeverAdoptsUsername(t *testing.T) {
	f := newPlatformAdminFixture(t)
	p := initializationParams(t, f)
	id := f.user(t, "")
	f.fx.Exec(t, "UPDATE user_password_credential SET username=$1 WHERE user_id=$2", InitialPlatformAdminUsername, id)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	_, err = CreatePlatformAdminInitializationInTx(context.Background(), tx, p)
	assertPlatformAdminError(t, err, "username_conflict")
}
func TestPlatformAdminInitializationRollsBackEveryRequiredWrite(t *testing.T) {
	for _, table := range []string{"user", "user_password_credential", "platform_role_binding", "platform_admin_initialization", "admin_operation", "admin_audit_event"} {
		t.Run(table, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			p := initializationParams(t, f)
			f.fx.Exec(t, "ALTER TABLE "+pgx.Identifier{table}.Sanitize()+" ADD CONSTRAINT reject_initialization CHECK (false) NOT VALID")
			tx, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			_, err = CreatePlatformAdminInitializationInTx(context.Background(), tx, p)
			if err == nil {
				t.Fatal("injected write failure was ignored")
			}
			_ = tx.Rollback(context.Background())
			for _, name := range []string{"user", "user_password_credential", "platform_role_binding", "platform_admin_initialization", "admin_operation", "admin_audit_event"} {
				var n int
				f.fx.QueryRow(t, "SELECT count(*) FROM "+pgx.Identifier{name}.Sanitize()).Scan(&n)
				if n != 0 {
					t.Fatalf("partial provisioning in %s", name)
				}
			}
		})
	}
}
func TestPlatformAdminInitializationRetainsTerminalMarkerAfterMaintenance(t *testing.T) {
	for _, action := range []string{"change", "recover", "disable", "revoke", "delete"} {
		t.Run(action, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			p := initializationParams(t, f)
			marker := createInitialization(t, f, p)
			switch action {
			case "change":
				f.fx.Exec(t, "UPDATE user_password_credential SET must_change_password=false,session_version=2")
			case "recover":
				f.fx.Exec(t, "UPDATE user_password_credential SET session_version=2")
			case "disable":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now()`)
			case "revoke":
				f.fx.Exec(t, "DELETE FROM platform_role_binding")
			case "delete":
				f.fx.Exec(t, `DELETE FROM "user"`)
			}
			state := inspectInitialization(t, f, p)
			if state.Status != "already_initialized" || state.Credential.UserID.Valid || state.Marker.InitializationID != marker.InitializationID {
				t.Fatal("maintenance reopened initialization or advertised initial credentials")
			}
			tx, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			retained, err := PreservePlatformAdminInitializationInTx(context.Background(), tx, f.org)
			if err != nil || retained.InitializationID != marker.InitializationID || retained.State != "created" {
				t.Fatal("manual preservation overwrote provenance")
			}
		})
	}
}
func TestPlatformAdminInitializationRejectsIdentityDrift(t *testing.T) {
	f := newPlatformAdminFixture(t)
	p := initializationParams(t, f)
	createInitialization(t, f, p)
	for _, field := range []string{"deployment", "organization"} {
		t.Run(field, func(t *testing.T) {
			changed := p
			if field == "deployment" {
				changed.DeploymentID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
				t.Setenv("MULTICA_DEPLOYMENT_ID", uuid.UUID(changed.DeploymentID.Bytes).String())
			} else {
				changed.OrganizationID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
			}
			tx, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			_, err = InspectPlatformAdminInitializationInTx(context.Background(), tx, changed.OrganizationID, changed.DeploymentID)
			assertPlatformAdminError(t, err, "initialization_identity_conflict")
		})
	}
}

func TestPlatformAdminInitializationManualBootstrapNeedsNoNewConfiguration(t *testing.T) {
	f := newPlatformAdminFixture(t)
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "false")
	t.Setenv("MULTICA_DEPLOYMENT_ID", "")
	target := f.user(t, "")
	_, err := f.svc.Bootstrap(context.Background(), PlatformAdminBootstrapParams{OrganizationID: f.org, TargetUserID: target, Reason: "Explicit operator bootstrap", RequestID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	marker, err := f.svc.Queries.GetPlatformAdminInitialization(context.Background())
	if err != nil || marker.State != "preserved_existing_admin" || marker.DeploymentID.Valid {
		t.Fatal("manual bootstrap failed to retain configuration-independent history")
	}
}
func TestPlatformAdminInitializationRoleGrantPreservesHistory(t *testing.T) {
	f := newPlatformAdminFixture(t)
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "false")
	t.Setenv("MULTICA_DEPLOYMENT_ID", "")
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	_, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := f.svc.Queries.GetPlatformAdminInitialization(context.Background())
	if err != nil || marker.State != "preserved_existing_admin" || marker.DeploymentID.Valid {
		t.Fatal("manual role grant failed to retain history")
	}
}
