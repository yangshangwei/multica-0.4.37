package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
)

func TestPlatformAdminRejectsNonHumanFullSessionsBeforeDatabase(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	cases := []struct {
		name    string
		session auth.PasswordSession
	}{
		{"personal access token", auth.PasswordSession{UserID: "00000000-0000-4000-8000-000000000001", Version: 1, Kind: "pat"}},
		{"task token", auth.PasswordSession{UserID: "00000000-0000-4000-8000-000000000001", Version: 1, Kind: "task_token"}},
		{"unknown credential", auth.PasswordSession{UserID: "00000000-0000-4000-8000-000000000001", Version: 1}},
		{"password setup", auth.PasswordSession{UserID: "00000000-0000-4000-8000-000000000001", Kind: "jwt", Setup: true}},
		{"password recovery", auth.PasswordSession{UserID: "00000000-0000-4000-8000-000000000001", Version: 1, Kind: "jwt", Change: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewPlatformAdminService(nil, nil)
			_, err := svc.Authorize(auth.WithPasswordSession(context.Background(), tc.session), false)
			var apiErr *PlatformAdminError
			if !errors.As(err, &apiErr) || apiErr.Code != "admin_session_required" {
				t.Fatalf("Authorize error = %v, want admin_session_required", err)
			}
		})
	}
}

func TestPlatformAdminUnavailableModeFailsClosed(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "classic")
	_, err := NewPlatformAdminService(nil, nil).Authorize(context.Background(), false)
	var apiErr *PlatformAdminError
	if !errors.As(err, &apiErr) || apiErr.Code != "admin_mode_disabled" {
		t.Fatalf("Authorize error = %v, want admin_mode_disabled", err)
	}
}

const platformAdminTestPassword = "service-password-42"

type platformAdminFixture struct {
	pool *pgxpool.Pool
	fx   *testutil.Fixture
	svc  *PlatformAdminService
	org  pgtype.UUID
	hash string
}

func newPlatformAdminFixture(t *testing.T) *platformAdminFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "password")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for platform administration integration tests")
	}
	base, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(base.Close)
	if err := base.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	schema := "admin_service_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(context.Background(), "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("remove isolated test schema: %v", err)
		}
	})
	// Every table written by role mutation or credential revocation is local.
	// Global last-admin counts must not include another package's fixtures.
	for _, table := range []string{"user", "user_password_credential", "platform_role_binding", "organization", "organization_workspace", "admin_operation", "admin_audit_event", "personal_access_token", "task_token", "daemon_token", "agent_runtime", "agent_task_queue"} {
		qualified := pgx.Identifier{schema, table}.Sanitize()
		source := pgx.Identifier{"public", table}.Sanitize()
		if _, err := base.Exec(context.Background(), "CREATE TABLE "+qualified+" (LIKE "+source+" INCLUDING ALL)"); err != nil {
			t.Fatalf("clone test table %s: %v", table, err)
		}
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	org, err := db.New(pool).GetInternalOrganization(context.Background())
	if errors.Is(err, pgx.ErrNoRows) {
		fx.Insert(t, "organization", testutil.Cols{"name": "Platform admin test organization"})
		org, err = db.New(pool).GetInternalOrganization(context.Background())
	}
	if err != nil {
		t.Fatalf("platform administration migrations required: %v", err)
	}
	hash, err := auth.HashPassword(context.Background(), platformAdminTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	return &platformAdminFixture{pool: pool, fx: fx, svc: NewPlatformAdminService(db.New(pool), pool), org: org.ID, hash: hash}
}
func (f *platformAdminFixture) user(t *testing.T, role string, overrides ...testutil.Cols) pgtype.UUID {
	t.Helper()
	unique := strings.ReplaceAll(uuid.NewString(), "-", "")
	id := f.fx.User(t, "Platform admin fixture", "admin-"+unique+"@example.invalid", overrides...)
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": id, "username": unique, "password_hash": f.hash}, "user_id = $1", id)
	if role != "" {
		f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": id, "role": role}, "user_id = $1", id)
	}
	f.fx.Cleanup(t, "DELETE FROM platform_role_binding WHERE user_id = $1", id)
	f.fx.Cleanup(t, "DELETE FROM admin_operation WHERE target_id = $1", id)
	f.fx.Cleanup(t, "DELETE FROM admin_audit_event WHERE target_id = $1", id)
	parsed, err := util.ParseUUID(id)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
func adminTestContext(id pgtype.UUID, version int64) context.Context {
	return auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: uuid.UUID(id.Bytes).String(), Version: version, Kind: "jwt"})
}
func (f *platformAdminFixture) roleChange(target pgtype.UUID, role, previous *string) PlatformRoleChangeParams {
	return PlatformRoleChangeParams{OrganizationID: f.org, TargetUserID: target, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Role: role, ExpectedRole: previous, ExpectedAuthVersion: 1, Reason: "Named administrative role change", Password: platformAdminTestPassword, RequestID: uuid.NewString()}
}
func adminRolePointer(value string) *string { return &value }
func assertPlatformAdminError(t *testing.T, err error, code string) {
	t.Helper()
	var adminErr *PlatformAdminError
	if !errors.As(err, &adminErr) || adminErr.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestPlatformAdminGrantRevokesOldCredentialsAndPersistsOneOperation(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	pat := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "old source PAT", "token_hash": uuid.NewString(), "token_prefix": "mul_old", "auth_version": 1})
	params := f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil)
	ctx := adminTestContext(actor, 1)
	result, err := f.svc.ChangeRole(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if !result.CredentialsRevoked || result.TargetUserID != target || result.Replayed {
		t.Fatalf("grant result = %+v", result)
	}
	if _, err := auth.CheckPasswordVersion(context.Background(), f.svc.Queries, uuid.UUID(target.Bytes).String(), 1); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("old credential remains usable: %v", err)
	}
	if _, err := f.svc.Authorize(ctx, true); err != nil {
		t.Fatalf("actor was revoked instead of target: %v", err)
	}
	var revoked bool
	f.fx.QueryRow(t, "SELECT revoked FROM personal_access_token WHERE id = $1", pat).Scan(&revoked)
	if !revoked {
		t.Fatal("target PAT not revoked")
	}
	replay, err := f.svc.ChangeRole(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed || replay.Operation.ID != result.Operation.ID || replay.CredentialsRevoked {
		t.Fatalf("replay = %+v", replay)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE operation_id = $1", result.Operation.ID); n != 1 {
		t.Fatalf("audit count = %d, want one", n)
	}
	found, err := f.svc.FindOperationByKey(ctx, f.org, params.IdempotencyKey)
	if err != nil || found.ID != result.Operation.ID {
		t.Fatalf("lost response lookup = %v, %v", found.ID, err)
	}
	if _, err := f.svc.FindOperationByKey(adminTestContext(target, 2), f.org, params.IdempotencyKey); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("another actor can recover operation: %v", err)
	}
	params.Role = adminRolePointer(PlatformRoleSuperAdmin)
	_, err = f.svc.ChangeRole(ctx, params)
	assertPlatformAdminError(t, err, "idempotency_conflict")
}

func TestPlatformAdminRolePayloadHashExcludesPasswordAndRequestID(t *testing.T) {
	params := PlatformRoleChangeParams{TargetUserID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Role: adminRolePointer(PlatformRoleObserver), ExpectedAuthVersion: 1, Reason: "Grant read access", Password: "one-password", RequestID: "request-one"}
	first, err := roleChangeHash(params)
	if err != nil {
		t.Fatal(err)
	}
	params.Password = "a-different-password"
	params.RequestID = "request-two"
	second, err := roleChangeHash(params)
	if err != nil || first != second {
		t.Fatalf("secret or request ID affected digest: %s vs %s (%v)", first, second, err)
	}
	params.Reason = "Grant reviewed access"
	third, err := roleChangeHash(params)
	if err != nil || first == third {
		t.Fatal("non-secret request change did not affect digest")
	}
}

func TestPlatformAdminLastSuperAdminAndExpectedState(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor := f.user(t, PlatformRoleSuperAdmin)
	params := f.roleChange(actor, adminRolePointer(PlatformRoleObserver), adminRolePointer(PlatformRoleSuperAdmin))
	_, err := f.svc.ChangeRole(adminTestContext(actor, 1), params)
	assertPlatformAdminError(t, err, "last_super_admin")
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id = $1", actor); n != 0 {
		t.Fatalf("failed mutation persisted %d operations", n)
	}
	params.ExpectedAuthVersion = 2
	_, err = f.svc.ChangeRole(adminTestContext(actor, 1), params)
	assertPlatformAdminError(t, err, "version_conflict")
}

type platformAdminFailAuditStarter struct{ pool *pgxpool.Pool }

func (s platformAdminFailAuditStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return platformAdminFailAuditTx{Tx: tx}, nil
}

type platformAdminFailAuditTx struct{ pgx.Tx }

func (tx platformAdminFailAuditTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO admin_audit_event") {
		return pgconn.CommandTag{}, errors.New("audit persistence failed")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestPlatformAdminAuditFailureRollsBackRoleVersionAndCredentials(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	pat := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "keep on rollback", "token_hash": uuid.NewString(), "token_prefix": "mul_keep", "auth_version": 1})
	f.svc.TxStarter = platformAdminFailAuditStarter{f.pool}
	_, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil))
	if err == nil || !strings.Contains(err.Error(), "audit persistence failed") {
		t.Fatalf("mutation error = %v", err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM platform_role_binding WHERE user_id = $1", target); n != 0 {
		t.Fatal("role survived audit failure")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id = $1", target); n != 0 {
		t.Fatal("operation survived audit failure")
	}
	if _, err := auth.CheckPasswordVersion(context.Background(), f.svc.Queries, uuid.UUID(target.Bytes).String(), 1); err != nil {
		t.Fatalf("version advanced on rollback: %v", err)
	}
	var revoked bool
	f.fx.QueryRow(t, "SELECT revoked FROM personal_access_token WHERE id = $1", pat).Scan(&revoked)
	if revoked {
		t.Fatal("PAT revocation survived rollback")
	}
}

func TestPlatformAdminConcurrentMutualDemotionLeavesOneEffectiveAdmin(t *testing.T) {
	f := newPlatformAdminFixture(t)
	first, second := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin)
	errs := make(chan error, 2)
	start := make(chan struct{})
	for _, pair := range [][2]pgtype.UUID{{first, second}, {second, first}} {
		go func(actor, target pgtype.UUID) {
			<-start
			_, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), adminRolePointer(PlatformRoleSuperAdmin)))
			errs <- err
		}(pair[0], pair[1])
	}
	close(start)
	success := 0
	for range 2 {
		if err := <-errs; err == nil {
			success++
		} else {
			assertPlatformAdminError(t, err, "admin_forbidden")
		}
	}
	if success != 1 {
		t.Fatalf("successful concurrent demotions = %d, want 1", success)
	}
	count, err := CountEffectiveSuperAdmins(context.Background(), f.svc.Queries)
	if err != nil || count != 1 {
		t.Fatalf("effective admins = %d (%v)", count, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE target_id = $1 OR target_id = $2", first, second); n != 1 {
		t.Fatalf("successful mutation audits = %d", n)
	}
}

func TestPlatformAdminBootstrapIsSerializedAndRequiresNewLogin(t *testing.T) {
	f := newPlatformAdminFixture(t)
	first, second := f.user(t, ""), f.user(t, "")
	errs := make(chan error, 2)
	results := make(chan PlatformRoleChangeResult, 2)
	start := make(chan struct{})
	for _, id := range []pgtype.UUID{first, second} {
		go func(id pgtype.UUID) {
			<-start
			result, err := f.svc.Bootstrap(context.Background(), PlatformAdminBootstrapParams{OrganizationID: f.org, TargetUserID: id, Reason: "Named deployment initialization", RequestID: uuid.NewString()})
			results <- result
			errs <- err
		}(id)
	}
	close(start)
	success := 0
	for range 2 {
		if err := <-errs; err == nil {
			success++
		} else {
			assertPlatformAdminError(t, err, "admin_already_initialized")
		}
	}
	if success != 1 {
		t.Fatalf("bootstrap successes = %d", success)
	}
	for range 2 {
		result := <-results
		if !result.Operation.ID.Valid {
			continue
		}
		if result.Operation.ActorKind != "deployment_operator" || result.Operation.ActorID.Valid {
			t.Fatal("bootstrap impersonated a user")
		}
		if _, err := f.svc.Authorize(adminTestContext(result.TargetUserID, 1), true); !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("pre-bootstrap session authorized: %v", err)
		}
		if _, err := f.svc.Authorize(adminTestContext(result.TargetUserID, 2), true); err != nil {
			t.Fatalf("fresh bootstrap login denied: %v", err)
		}
	}
}

type platformAdminPausedStarter struct {
	pool    *pgxpool.Pool
	reached chan struct{}
	resume  chan struct{}
}

func (s platformAdminPausedStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	close(s.reached)
	select {
	case <-s.resume:
		return s.pool.Begin(ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func TestPlatformAdminRevocationAfterPasswordVerificationFencesQueuedWrite(t *testing.T) {
	f := newPlatformAdminFixture(t)
	admin, revoked, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	reached, resume := make(chan struct{}), make(chan struct{})
	delayed := NewPlatformAdminService(f.svc.Queries, platformAdminPausedStarter{f.pool, reached, resume})
	done := make(chan error, 1)
	go func() {
		_, err := delayed.ChangeRole(adminTestContext(revoked, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil))
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("password verification never reached transaction boundary")
	}
	_, err := f.svc.ChangeRole(adminTestContext(admin, 1), f.roleChange(revoked, nil, adminRolePointer(PlatformRoleSuperAdmin)))
	close(resume)
	if err != nil {
		t.Fatal(err)
	}
	assertPlatformAdminError(t, <-done, "admin_forbidden")
	if n := f.fx.Count(t, "SELECT count(*) FROM platform_role_binding WHERE user_id = $1", target); n != 0 {
		t.Fatal("queued write crossed committed role revocation")
	}
}

// A role grant may commit between authentication reads. Advancing the old
// credential snapshot must not combine with a newly granted role to authorize
// a JWT that predates the grant.
type platformAdminGrantBetweenReads struct {
	*pgxpool.Pool
	afterCredential func()
}

func (q platformAdminGrantBetweenReads) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := q.Pool.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: GetPasswordCredential :one") {
		return platformAdminAfterScan{row: row, after: q.afterCredential}
	}
	return row
}

type platformAdminAfterScan struct {
	row   pgx.Row
	after func()
}

func (r platformAdminAfterScan) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if err == nil {
		r.after()
	}
	return err
}
func TestPlatformAdminAuthorizationCannotCombineOldVersionWithNewRole(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, "")
	afterCredential := func() {
		f.fx.Exec(t, "UPDATE user_password_credential SET session_version = 2 WHERE user_id = $1", target)
		f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": target, "role": PlatformRoleSuperAdmin}, "user_id = $1", target)
	}
	svc := NewPlatformAdminService(db.New(platformAdminGrantBetweenReads{f.pool, afterCredential}), f.pool)
	_, err := svc.Authorize(adminTestContext(target, 1), false)
	if err == nil {
		t.Fatal("a pre-grant session gained administrative access from mixed authorization snapshots")
	}
}

func TestPlatformAdminCommonOperationCreationRechecksActorAndIsIdempotent(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	ctx := adminTestContext(actor, 1)
	digest := strings.Repeat("a", 64)
	params := db.CreateAdminOperationParams{OrganizationID: f.org, TargetID: target, TargetKind: "user", Kind: "user.role", IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, PayloadHash: digest, Reason: "Record reviewed operation", State: "applied", ResultCode: "role_changed"}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	first, replayed, err := f.svc.CreateOperationInTx(ctx, tx, params)
	if err != nil || replayed {
		t.Fatalf("create = %v, %v", replayed, err)
	}
	second, replayed, err := f.svc.CreateOperationInTx(ctx, tx, params)
	if err != nil || !replayed || first.ID != second.ID {
		t.Fatalf("replay = %v, %v", replayed, err)
	}
	if first.ActorID != actor || first.ActorKind != "user" || first.ActorAuthVersion != 1 {
		t.Fatal("operation attribution did not come from current authenticated actor")
	}
	if _, _, err := f.svc.CreateOperationInTx(adminTestContext(target, 1), tx, params); err == nil {
		t.Fatal("common operation helper accepted an unauthorized actor")
	}
	params.Kind = "user.disable"
	_, _, err = f.svc.CreateOperationInTx(ctx, tx, params)
	assertPlatformAdminError(t, err, "idempotency_conflict")
	params.IdempotencyKey = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	params.Kind = "unregistered.command"
	_, _, err = f.svc.CreateOperationInTx(ctx, tx, params)
	assertPlatformAdminError(t, err, "invalid_operation")
}

func TestPlatformAdminDisabledAndRecoveryAdminsDoNotSatisfyLastAdminProtection(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, disabled, recovery := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleSuperAdmin)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at = now() WHERE id = $1`, disabled)
	f.fx.Exec(t, "UPDATE user_password_credential SET must_change_password = true WHERE user_id = $1", recovery)
	_, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(actor, nil, adminRolePointer(PlatformRoleSuperAdmin)))
	assertPlatformAdminError(t, err, "last_super_admin")
	for _, id := range []pgtype.UUID{disabled, recovery} {
		if _, err := f.svc.Authorize(adminTestContext(id, 1), false); !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("inactive admin authorization = %v", err)
		}
	}
}

func TestPlatformAdminRejectedPasswordAndObserverCannotMutate(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, observer, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleObserver), f.user(t, "")
	params := f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil)
	params.Password = "incorrect-password"
	_, err := f.svc.ChangeRole(adminTestContext(actor, 1), params)
	assertPlatformAdminError(t, err, "password_verification_failed")
	params.Password = platformAdminTestPassword
	_, err = f.svc.ChangeRole(adminTestContext(observer, 1), params)
	assertPlatformAdminError(t, err, "admin_forbidden")
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation WHERE target_id = $1", target); n != 0 {
		t.Fatal("denied mutation persisted operation")
	}
}

func TestPlatformAdminPromotionAdvancesVersionAndOverflowRollsBack(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, PlatformRoleObserver)
	params := f.roleChange(target, adminRolePointer(PlatformRoleSuperAdmin), adminRolePointer(PlatformRoleObserver))
	result, err := f.svc.ChangeRole(adminTestContext(actor, 1), params)
	if err != nil || !result.CredentialsRevoked {
		t.Fatalf("promotion = %v, %v", result.CredentialsRevoked, err)
	}
	credential, err := f.svc.Queries.GetPasswordCredential(context.Background(), target)
	if err != nil || credential.SessionVersion != 2 {
		t.Fatalf("promoted version = %d, %v", credential.SessionVersion, err)
	}
	exhausted := f.user(t, "")
	f.fx.Exec(t, "UPDATE user_password_credential SET session_version = 9223372036854775807 WHERE user_id = $1", exhausted)
	params = f.roleChange(exhausted, adminRolePointer(PlatformRoleObserver), nil)
	params.ExpectedAuthVersion = 9223372036854775807
	_, err = f.svc.ChangeRole(adminTestContext(actor, 1), params)
	assertPlatformAdminError(t, err, "auth_version_exhausted")
	if n := f.fx.Count(t, "SELECT count(*) FROM platform_role_binding WHERE user_id = $1", exhausted); n != 0 {
		t.Fatal("overflow persisted role grant")
	}
}

type platformAdminUncertainCommitStarter struct{ pool *pgxpool.Pool }

func (s platformAdminUncertainCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return platformAdminUncertainCommitTx{tx}, nil
}

type platformAdminUncertainCommitTx struct{ pgx.Tx }

func (tx platformAdminUncertainCommitTx) Commit(ctx context.Context) error {
	if err := tx.Tx.Commit(ctx); err != nil {
		return err
	}
	return errors.New("connection lost while receiving commit result")
}
func TestPlatformAdminUncertainCommitCanBeRecoveredWithoutApplyingTwice(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	params := f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil)
	uncertain := NewPlatformAdminService(f.svc.Queries, platformAdminUncertainCommitStarter{f.pool})
	result, err := uncertain.ChangeRole(adminTestContext(actor, 1), params)
	if err == nil || result.CredentialsRevoked {
		t.Fatal("uncertain commit reported certain external revocation outcome")
	}
	found, err := f.svc.FindOperationByKey(adminTestContext(actor, 1), f.org, params.IdempotencyKey)
	if err != nil {
		t.Fatalf("committed operation was not recoverable: %v", err)
	}
	replay, err := f.svc.ChangeRole(adminTestContext(actor, 1), params)
	if err != nil || !replay.Replayed || replay.Operation.ID != found.ID {
		t.Fatalf("reconcile/replay = %v, %v", replay.Replayed, err)
	}
	credential, err := f.svc.Queries.GetPasswordCredential(context.Background(), target)
	if err != nil || credential.SessionVersion != 2 {
		t.Fatalf("role applied twice: version %d, %v", credential.SessionVersion, err)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE target_id = $1", target); n != 1 {
		t.Fatalf("audit count = %d, want 1", n)
	}
}

func TestPlatformAdminSelfDemotionWithAnotherAdminCommits(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor := f.user(t, PlatformRoleSuperAdmin)
	f.user(t, PlatformRoleSuperAdmin)
	result, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(actor, adminRolePointer(PlatformRoleObserver), adminRolePointer(PlatformRoleSuperAdmin)))
	if err != nil {
		t.Fatalf("legitimate self-demotion failed: %v", err)
	}
	if result.CredentialsRevoked {
		t.Fatal("demotion unnecessarily revoked all credentials")
	}
	if _, err := f.svc.Authorize(adminTestContext(actor, 1), true); err == nil {
		t.Fatal("demoted actor retained write privilege")
	}
}

// These fixtures identify entries in auth's emergency denylist; production
// logic must always call auth.IsTemporarilyDisabledUser instead of copying it.
func platformBlockedUserCases() []struct {
	name string
	user testutil.Cols
} {
	return []struct {
		name string
		user testutil.Cols
	}{
		{"user_id", testutil.Cols{"id": "514492f7-b30f-4147-bd33-c0e8ce5d6d4f"}},
		{"email", testutil.Cols{"email": "pdzzer68@embassybase.com"}},
	}
}

func TestPlatformAdminTemporaryDenylistRejectsAuthorizationAndGrant(t *testing.T) {
	for _, tc := range platformBlockedUserCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			blocked, actor, target := f.user(t, PlatformRoleSuperAdmin, tc.user), f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
			_, err := f.svc.Authorize(adminTestContext(blocked, 1), false)
			assertPlatformAdminError(t, err, "account_disabled")
			_, err = f.svc.ChangeRole(adminTestContext(blocked, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil))
			assertPlatformAdminError(t, err, "account_disabled")
			f.fx.Exec(t, "DELETE FROM platform_role_binding WHERE user_id = $1", blocked)
			_, err = f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(blocked, adminRolePointer(PlatformRoleObserver), nil))
			assertPlatformAdminError(t, err, "account_not_ready")
			if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation"); n != 0 {
				t.Fatal("denylisted account produced an administrative mutation")
			}
		})
	}
}

func TestPlatformAdminTemporaryDenylistCannotBeBootstrapped(t *testing.T) {
	for _, tc := range platformBlockedUserCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			blocked := f.user(t, "", tc.user)
			_, err := f.svc.Bootstrap(t.Context(), PlatformAdminBootstrapParams{OrganizationID: f.org, TargetUserID: blocked, Reason: "Initialize named administrator", RequestID: uuid.NewString()})
			assertPlatformAdminError(t, err, "account_not_ready")
		})
	}
}

func TestPlatformAdminTemporaryDenylistDoesNotCountAsEffective(t *testing.T) {
	for _, tc := range platformBlockedUserCases() {
		t.Run(tc.name+"_last_admin", func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			f.user(t, PlatformRoleSuperAdmin, tc.user)
			actor := f.user(t, PlatformRoleSuperAdmin)
			_, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(actor, nil, adminRolePointer(PlatformRoleSuperAdmin)))
			assertPlatformAdminError(t, err, "last_super_admin")
		})
		t.Run(tc.name+"_bootstrap", func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			f.user(t, PlatformRoleSuperAdmin, tc.user)
			target := f.user(t, "")
			_, err := f.svc.Bootstrap(t.Context(), PlatformAdminBootstrapParams{OrganizationID: f.org, TargetUserID: target, Reason: "Initialize usable administrator", RequestID: uuid.NewString()})
			if err != nil {
				t.Fatalf("blocked role prevented usable administrator bootstrap: %v", err)
			}
		})
		t.Run(tc.name+"_recovery", func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			f.user(t, PlatformRoleSuperAdmin, tc.user)
			target := f.user(t, PlatformRoleSuperAdmin)
			_, err := runDeploymentRecovery(t, f, DeploymentPasswordRecoveryParams{TargetUserID: target, PasswordHash: "temporary-hash", Reason: "Recover usable administrator", RequestID: uuid.NewString()}, false)
			assertPlatformAdminError(t, err, "last_super_admin")
		})
	}
}

func TestPlatformAdminMissingOrInactiveOrganizationIsUnavailable(t *testing.T) {
	for _, state := range []string{"missing", "inactive"} {
		t.Run(state, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			actor := f.user(t, PlatformRoleSuperAdmin)
			if state == "missing" {
				f.fx.Exec(t, "DELETE FROM organization")
			} else {
				f.fx.Exec(t, "UPDATE organization SET state = 'inactive'")
			}
			_, err := f.svc.Authorize(adminTestContext(actor, 1), false)
			assertPlatformAdminError(t, err, "admin_unavailable")
			var adminErr *PlatformAdminError
			if !errors.As(err, &adminErr) || adminErr.Status != 503 {
				t.Fatalf("organization unavailability status = %v, want 503", err)
			}
		})
	}
}
