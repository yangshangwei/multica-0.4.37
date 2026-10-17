package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationAccountChange(f *platformAdminFixture, target pgtype.UUID, action string, version int64) PlatformAccountChangeParams {
	return PlatformAccountChangeParams{OrganizationID: f.org, TargetUserID: target, Action: action,
		ExpectedAuthVersion: version, Reason: "Review account security", Password: platformAdminTestPassword,
		IdempotencyKey: randomAccountKey(), RequestID: uuid.NewString()}
}

func collaborationAccountFence(t *testing.T, f *platformAdminFixture, user pgtype.UUID) {
	t.Helper()
	f.fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
}

// Clone every table used by the queue service so neither writes nor membership
// reads can escape this fixture's private schema through its public fallback.
func collaborationAccountIssue(t *testing.T, f *platformAdminFixture, user pgtype.UUID) (pgtype.UUID, *CollaborationService) {
	t.Helper()
	for _, table := range []string{"workspace", "member", "issue", "issue_status", "issue_collaboration", "collaboration_session", "issue_participant", "collaboration_operation", "collaboration_event", "collaboration_command", "collaboration_target", "collaboration_execution_group", "collaboration_delegation", "collaboration_grant", "collaboration_run", "collaboration_termination", "collaboration_runtime_enrollment", "agent", "agent_invocation_target", "squad", "squad_member", "installation_daemon_binding", "managed_installation"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	f.fx.WorkspaceID = f.fx.Workspace(t, "Private collaboration", "account-"+uuid.NewString())
	f.fx.UserID = util.UUIDToString(user)
	id := f.fx.Issue(t, "Retained command", testutil.Cols{"assignee_type": "member", "assignee_id": user, "admission_status": "not_required"})
	f.fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": id}, "issue_id=$1", id)
	f.fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": id})
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": id, "user_id": user}, "issue_id=$1 AND user_id=$2", id, user)
	return util.MustParseUUID(id), NewCollaborationService(f.svc.Queries, f.pool, nil)
}

func collaborationPasswordContext(user pgtype.UUID, version int64) context.Context {
	return auth.WithAuthenticatedPrincipal(adminTestContext(user, version), auth.AuthenticatedPrincipal{
		UserID: util.UUIDToString(user), Kind: auth.CredentialJWT,
	})
}

func TestCollaborationAccountInvalidationDisableRestoreRejectsOldIntent(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	collaborationAccountFence(t, f, actor)
	collaborationAccountFence(t, f, target)
	issue, queue := collaborationAccountIssue(t, f, target)
	session, err := queue.Queries.GetCollaborationSession(t.Context(), issue)
	if err != nil {
		t.Fatal(err)
	}
	executionTarget, _ := seedCollaborationTarget(t, f.fx, issue, session.ID, target, 1)
	p := CollaborationSubmitCommandParams{CollaborationQueueRequest: CollaborationQueueRequest{
		IssueID: issue, RequestID: uuid.NewString(), ExpectedAccountEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedLifecycleEpoch: 1,
	}, TargetID: executionTarget.ID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "Original immutable command"}
	command, err := queue.SubmitCommand(collaborationPasswordContext(target, 1), p)
	if err != nil {
		t.Fatal(err)
	}
	// This separate intent was prepared before disable but reaches the server
	// only after restoration with a newly authenticated password session.
	p.RequestID, p.Content = uuid.NewString(), "Delayed old intent"
	disable := collaborationAccountChange(f, target, "disable", 1)
	first, err := f.svc.ChangeAccount(adminTestContext(actor, 1), disable)
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, target) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 AND credential_version=2 AND revocation_key=$2 AND reason='credentials_revoked' AND reconciled_at IS NULL`, target, "admin:"+util.UUIDToString(first.Operation.ID)) != 1 {
		t.Fatal("disable did not atomically advance the account and persist pending intent")
	}
	replayed, err := f.svc.ChangeAccount(adminTestContext(actor, 1), disable)
	if err != nil || !replayed.Replayed || replayed.Operation.ID != first.Operation.ID {
		t.Fatalf("disable replay: %+v, %v", replayed, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, target) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, target) != 1 {
		t.Fatal("replayed disable repeated invalidation")
	}
	if _, err = f.svc.ChangeAccount(adminTestContext(actor, 1), collaborationAccountChange(f, target, "restore", 2)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND reconciled_at IS NULL`, target) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=3`, target) != 1 {
		t.Fatal("restore lost earlier invalidation or failed to advance authority")
	}
	if _, err = auth.CheckPasswordVersion(t.Context(), f.svc.Queries, util.UUIDToString(target), 3); err != nil {
		t.Fatalf("restored account cannot authenticate: %v", err)
	}
	receipt, err := queue.SubmitCommand(collaborationPasswordContext(target, 3), p)
	if !collaborationErrorCode(err, "stale_authority") || receipt.Code != "stale_authority" {
		t.Fatalf("old intent revived after restore: %+v, %v", receipt, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND author_auth_epoch=1 AND state='waiting' AND content='Original immutable command'`, command.CommandID) != 1 {
		t.Fatal("security mutation rewrote the old command instead of invalidating its authority")
	}
	// Reconciliation/promotion is not implemented by this slice. The stored
	// command remains auditable and provably carries an obsolete account epoch.
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command c JOIN collaboration_account_fence f ON f.user_id=c.author_user_id WHERE c.id=$1 AND c.author_auth_epoch<f.epoch`, command.CommandID) != 1 {
		t.Fatal("pre-disable command authority became current again")
	}
	// Restoring an owner does not revive old consent; seed the independently
	// authorized replacement configuration before a fresh input.
	rebindCollaborationTestTarget(t, f.fx, issue, session.ID, executionTarget.ID, target, 3)
	p.ExpectedAccountEpoch, p.RequestID, p.ExpectedConfigVersion, p.InputEventID = 3, uuid.NewString(), 2, util.MustParseUUID(uuid.NewString())
	if receipt, err = queue.SubmitCommand(collaborationPasswordContext(target, 3), p); err != nil || receipt.Code != "command_queued" {
		t.Fatalf("fresh intent refused after restore: %+v, %v", receipt, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=1`, actor) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, actor) != 0 {
		t.Fatal("target security operation invalidated the acting administrator")
	}
}

func TestCollaborationAccountInvalidationLegacyAccount(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor := f.user(t, PlatformRoleSuperAdmin)
	target := util.MustParseUUID(f.fx.User(t, "Legacy collaborator", uuid.NewString()+"@example.invalid"))
	collaborationAccountFence(t, f, target)
	for _, action := range []string{"disable", "restore"} {
		p := collaborationAccountChange(f, target, action, 0)
		result, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p)
		if err != nil {
			t.Fatal(err)
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND revocation_key=$2 AND credential_version IS NULL`, target, "admin:"+util.UUIDToString(result.Operation.ID)) != 1 {
			t.Fatalf("%s skipped the unconfigured legacy account", action)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM user_password_credential WHERE user_id=$1`, target) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=3`, target) != 1 {
		t.Fatal("legacy invalidation requires or fabricates a password credential")
	}
	p := collaborationAccountChange(f, target, "recover-password", 0)
	p.Username, p.TemporaryPassword = "legacy_"+uuid.NewString()[:8], "temporary-password-42"
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=4 AND credential_version=1`, target) != 1 {
		t.Fatal("first password recovery did not invalidate prior legacy authority")
	}
}

func TestCollaborationAccountInvalidationFailuresRollBackSecurityChange(t *testing.T) {
	for _, failure := range []string{"outbox", "audit", "epoch_exhausted"} {
		t.Run(failure, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
			collaborationAccountFence(t, f, target)
			pat := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "preserved on rollback", "token_hash": uuid.NewString(), "token_prefix": "mul_old", "auth_version": 1})
			var initialEpoch int64 = 1
			switch failure {
			case "outbox":
				f.fx.Exec(t, `ALTER TABLE collaboration_account_invalidation ADD CONSTRAINT reject_invalidation CHECK (false)`)
			case "audit":
				f.svc.TxStarter = platformAdminFailAuditStarter{f.pool}
			case "epoch_exhausted":
				initialEpoch = math.MaxInt64
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=$2 WHERE user_id=$1`, target, initialEpoch)
			}
			if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), collaborationAccountChange(f, target, "disable", 1)); err == nil {
				t.Fatal("security change committed despite required persistence failure")
			}
			credential, err := f.svc.Queries.GetPasswordCredential(t.Context(), target)
			if err != nil || credential.SessionVersion != 1 || credential.PasswordHash != f.hash {
				t.Fatalf("failure changed credentials: %+v, %v", credential, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM "user" WHERE id=$1 AND disabled_at IS NULL`, target) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM personal_access_token WHERE id=$1 AND NOT revoked`, pat) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=$2`, target, initialEpoch) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation`) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM admin_operation`) != 0 {
				t.Fatal("failed security transaction left partial account, token, fence or intent state")
			}
		})
	}
}

func TestCollaborationAccountInvalidationDoesNotCreateOrdinaryAccountFence(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	if _, err := f.svc.ChangeAccount(adminTestContext(actor, 1), collaborationAccountChange(f, target, "disable", 1)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence`) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation`) != 0 {
		t.Fatal("ordinary account revocation created collaboration state")
	}
}

func TestCollaborationAccountInvalidationDoesNotWaitForSourceIssue(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	collaborationAccountFence(t, f, target)
	issue, _ := collaborationAccountIssue(t, f, target)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err = db.New(tx).LockCollaborationIssue(t.Context(), issue); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(adminTestContext(actor, 1), 5*time.Second)
	defer cancel()
	if _, err = f.svc.ChangeAccount(ctx, collaborationAccountChange(f, target, "disable", 1)); err != nil {
		t.Fatalf("account invalidation waited for a source issue: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2`, target) != 1 {
		t.Fatal("source-independent invalidation was not persisted")
	}
}

func TestCollaborationAccountInvalidationAdministrativeCredentialWriters(t *testing.T) {
	for _, writer := range []string{"role_elevation", "bootstrap", "deployment_recovery"} {
		t.Run(writer, func(t *testing.T) {
			f := newPlatformAdminFixture(t)
			target := f.user(t, "")
			collaborationAccountFence(t, f, target)
			var operation db.AdminOperation
			switch writer {
			case "role_elevation":
				actor := f.user(t, PlatformRoleSuperAdmin)
				result, err := f.svc.ChangeRole(adminTestContext(actor, 1), f.roleChange(target, adminRolePointer(PlatformRoleObserver), nil))
				if err != nil {
					t.Fatal(err)
				}
				operation = result.Operation
				// Role removal has never revoked all account credentials.
				p := f.roleChange(target, nil, adminRolePointer(PlatformRoleObserver))
				p.ExpectedAuthVersion = 2
				if _, err = f.svc.ChangeRole(adminTestContext(actor, 1), p); err != nil {
					t.Fatal(err)
				}
			case "bootstrap":
				result, err := f.svc.Bootstrap(t.Context(), PlatformAdminBootstrapParams{
					OrganizationID: f.org, TargetUserID: target, Reason: "Initialize reviewed administrator", RequestID: uuid.NewString(),
				})
				if err != nil {
					t.Fatal(err)
				}
				operation = result.Operation
			case "deployment_recovery":
				result, err := runDeploymentRecovery(t, f, DeploymentPasswordRecoveryParams{
					TargetUserID: target, PasswordHash: f.hash, Reason: "Recover collaborator credentials", RequestID: uuid.NewString(),
				}, false)
				if err != nil {
					t.Fatal(err)
				}
				operation = result.Operation
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, target) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2 AND credential_version=2 AND revocation_key=$2`, target, "admin:"+util.UUIDToString(operation.ID)) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, target) != 1 {
				t.Fatal("administrative credential writer did not persist exactly one invalidation")
			}
		})
	}
}

func TestCollaborationAccountInvalidationBorrowedTransactionReplay(t *testing.T) {
	f := newPlatformAdminFixture(t)
	target := f.user(t, "")
	collaborationAccountFence(t, f, target)
	for attempt := range 2 {
		tx, err := f.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(t.Context())
		q := db.New(tx)
		if _, err = q.LockPasswordUser(t.Context(), target); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			if _, err = q.ChangePasswordCredential(t.Context(), db.ChangePasswordCredentialParams{UserID: target, PasswordHash: f.hash, SessionVersion: 1}); err != nil {
				t.Fatal(err)
			}
		}
		for range 2 {
			if _, err = auth.RevokePasswordCredentials(t.Context(), tx, target, "password:2"); err != nil {
				t.Fatal(err)
			}
		}
		if err = tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, target) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1`, target) != 1 {
		t.Fatal("repeating the borrowed primitive duplicated invalidation")
	}
}

func TestCollaborationAccountInvalidationWaitsForFirstParticipationUserFence(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor, target := f.user(t, PlatformRoleSuperAdmin), f.user(t, "")
	ctx, cancel := context.WithTimeout(adminTestContext(actor, 1), 10*time.Second)
	defer cancel()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	if _, err = q.LockCollaborationUsers(ctx, []pgtype.UUID{target}); err != nil {
		t.Fatal(err)
	}
	if _, err = q.EnsureCollaborationAccountFence(ctx, target); err != nil {
		t.Fatal(err)
	}
	var holder int32
	if err = tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holder); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.ChangeAccount(ctx, collaborationAccountChange(f, target, "disable", 1))
		done <- err
	}()
	for {
		var blocked bool
		if err = f.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, holder).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("account change bypassed the first-participation user lock: %v", err)
		case <-ctx.Done():
			t.Fatal("account change never reached the shared user lock")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1 AND epoch=2`, target) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_account_invalidation WHERE user_id=$1 AND account_epoch=2`, target) != 1 {
		t.Fatal("account revocation lost the concurrently established collaboration fence")
	}
}
