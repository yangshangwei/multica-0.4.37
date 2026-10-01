package service

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func managedRuntimeFixture(t *testing.T) (*installationFixture, auth.PasswordSession, pgtype.UUID) {
	t.Helper()
	f := newInstallationFixture(t)
	inst, err := f.management.Enroll(f.ctx, f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "test", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	ws := f.fx.Workspace(t, "Managed runtime", "managed-runtime-"+uuid.NewString())
	f.fx.Member(t, ws, util.UUIDToString(f.user), "owner")
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": f.org, "workspace_id": ws}, "workspace_id=$1", ws)
	daemonID := uuid.NewString()
	proof := f.challenge(t, InstallationChallengeParams{Purpose: "bind", InstallationID: inst.InstallationID, WorkspaceID: ws, DaemonID: daemonID})
	pat := auth.WithPasswordSession(t.Context(), auth.PasswordSession{UserID: util.UUIDToString(f.user), Version: 1, Kind: "pat"})
	bound, err := f.management.Bind(pat, proof)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := auth.CheckPasswordToken(t.Context(), f.svc.Queries, bound.DaemonToken, true)
	if err != nil {
		t.Fatal(err)
	}
	id := f.fx.Runtime(t, "Bound runtime", testutil.Cols{"workspace_id": ws, "owner_id": f.user, "daemon_id": daemonID, "runtime_mode": "local"})
	runtimeID, err := util.ParseUUID(id)
	if err != nil {
		t.Fatal(err)
	}
	return f, identity.Session, runtimeID
}

func TestManagedRuntimeRejectsPATForgedNamespaceAndOtherOwner(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	runtime, err := f.svc.Queries.GetAgentRuntime(t.Context(), runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithPasswordSession(t.Context(), source)
	if _, err := ValidateManagedRuntime(ctx, f.svc.Queries, runtime); err != nil {
		t.Fatal(err)
	}
	pat := source
	pat.Kind = "pat"
	pat.BindingID = ""
	pat.BindingEpoch = 0
	if _, err := ValidateManagedRuntime(auth.WithPasswordSession(t.Context(), pat), f.svc.Queries, runtime); !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("PAT result=%v", err)
	}
	wrong := runtime
	wrong.OwnerID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err := ValidateManagedRuntime(ctx, f.svc.Queries, wrong); !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("foreign owner result=%v", err)
	}
	wrong = runtime
	wrong.WorkspaceID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if _, err := ValidateManagedRuntime(ctx, f.svc.Queries, wrong); !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("foreign workspace result=%v", err)
	}
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", source.BindingID)
	if _, err := ValidateManagedRuntime(ctx, f.svc.Queries, runtime); err == nil {
		t.Fatal("revoked binding remained authorized")
	}
}
func TestManagedRuntimeAllowsUnboundLegacyNamespace(t *testing.T) {
	f := newInstallationFixture(t)
	ws := f.fx.Workspace(t, "Legacy runtime", "legacy-"+uuid.NewString())
	id := f.fx.Runtime(t, "Legacy runtime", testutil.Cols{"workspace_id": ws, "owner_id": f.user, "daemon_id": uuid.NewString()})
	runtimeID, err := util.ParseUUID(id)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := f.svc.Queries.GetAgentRuntime(t.Context(), runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	if binding, err := ValidateManagedRuntime(context.Background(), f.svc.Queries, runtime); err != nil || binding.ID.Valid {
		t.Fatalf("legacy namespace changed: %+v %v", binding, err)
	}
}

func TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	runtime, err := f.svc.Queries.GetAgentRuntime(t.Context(), runtimeID)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", source.BindingID)
	pat := source
	pat.Kind = "pat"
	pat.BindingID = ""
	pat.BindingEpoch = 0
	unboundDaemon := pat
	unboundDaemon.Kind = "daemon_token"
	for name, ctx := range map[string]context.Context{
		"owner PAT":                 auth.WithPasswordSession(t.Context(), pat),
		"unbound daemon credential": auth.WithPasswordSession(t.Context(), unboundDaemon),
		"legacy context":            t.Context(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateManagedRuntime(ctx, f.svc.Queries, runtime); !errors.Is(err, ErrManagedRuntimeSource) {
				t.Fatalf("revoked managed namespace downgraded to legacy authorization: %v", err)
			}
		})
	}
	ctx := auth.WithPasswordSession(t.Context(), pat)
	t.Run("registration", func(t *testing.T) {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = LockManagedDaemonRegistration(ctx, db.New(tx), runtime.WorkspaceID, source.DaemonID, f.user); !errors.Is(err, ErrManagedRuntimeSource) {
			t.Fatalf("revoked namespace accepted legacy registration: %v", err)
		}
	})
	t.Run("claim", func(t *testing.T) {
		for _, table := range []string{"agent", "issue"} {
			f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
		}
		fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
		agentID := fx.Agent(t, "Revoked runtime agent", util.UUIDToString(runtimeID))
		issueID := fx.Issue(t, "Revoked namespace must not claim")
		taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
		taskUUID := mustManagedUUID(t, taskID)
		before, err := f.svc.Queries.GetAgentTask(ctx, taskUUID)
		if err != nil {
			t.Fatal(err)
		}
		svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
		if _, err := svc.ClaimTaskForRuntime(ctx, runtimeID); !errors.Is(err, ErrManagedRuntimeSource) {
			t.Errorf("revoked namespace accepted legacy claim: %v", err)
		}
		after, err := f.svc.Queries.GetAgentTask(ctx, taskUUID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Status != before.Status || after.DispatchedAt.Valid {
			t.Fatalf("rejected claim changed task delivery state: status=%s dispatched=%v", after.Status, after.DispatchedAt.Valid)
		}
	})
}

func TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	for _, table := range []string{"agent", "issue"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	agentID := fx.Agent(t, "Managed agent", util.UUIDToString(runtimeID))
	issueID := fx.Issue(t, "Managed claim")
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
	svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	pat := source
	pat.Kind = "pat"
	pat.BindingID = ""
	pat.BindingEpoch = 0
	if _, err := svc.ClaimTaskForRuntime(auth.WithPasswordSession(t.Context(), pat), runtimeID); !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("PAT claim=%v", err)
	}
	ctx := auth.WithPasswordSession(t.Context(), source)
	claimed, err := svc.ClaimTaskForRuntime(ctx, runtimeID)
	if err != nil || claimed == nil {
		t.Fatalf("managed claim=%+v, %v", claimed, err)
	}
	if util.UUIDToString(claimed.ID) != taskID || util.UUIDToString(claimed.ExecutionBindingID) != source.BindingID || claimed.ExecutionBindingEpoch.Int64 != source.BindingEpoch || !claimed.ExecutionInstallationID.Valid {
		t.Fatalf("claim lost binding snapshot: %+v", claimed)
	}
	// A pre-upgrade dispatched execution has no known installation identity.
	// Reclaiming delivery must not fabricate an association for that history.
	fx.Exec(t, "UPDATE agent_task_queue SET execution_binding_id=NULL,execution_binding_epoch=NULL,execution_installation_id=NULL,dispatched_at=now()-interval '5 minutes',prepare_lease_expires_at=now()-interval '1 minute' WHERE id=$1", taskID)
	svc.ReclaimCheck = nil
	reclaimed, err := svc.ClaimTaskForRuntime(ctx, runtimeID)
	if err != nil || reclaimed == nil {
		t.Fatalf("historical reclaim=%+v, %v", reclaimed, err)
	}
	if reclaimed.ExecutionBindingID.Valid || reclaimed.ExecutionInstallationID.Valid {
		t.Fatal("historical task was retroactively associated")
	}
	tokenHash := auth.HashToken(uuid.NewString())
	derivedHash := auth.HashToken(uuid.NewString())
	_, err = svc.FinalizeTaskClaim(ctx, *reclaimed, db.CreateTaskTokenParams{TokenHash: tokenHash, TaskID: reclaimed.ID, AgentID: reclaimed.AgentID, WorkspaceID: mustManagedUUID(t, source.WorkspaceID), UserID: f.user, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, db.CreateDaemonTokenParams{TokenHash: derivedHash, WorkspaceID: mustManagedUUID(t, source.WorkspaceID), DaemonID: source.DaemonID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}, InstallationBindingID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, InstallationBindingEpoch: pgtype.Int8{Int64: 999, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	token, err := f.svc.Queries.GetTaskTokenByHash(t.Context(), tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if util.UUIDToString(token.InstallationBindingID) != source.BindingID || token.InstallationBindingEpoch.Int64 != source.BindingEpoch {
		t.Fatal("new task token lost current minting provenance")
	}
	derived, err := f.svc.Queries.GetDaemonTokenByHash(t.Context(), derivedHash)
	if err != nil {
		t.Fatal(err)
	}
	if util.UUIDToString(derived.InstallationBindingID) != source.BindingID || derived.InstallationBindingEpoch.Int64 != source.BindingEpoch {
		t.Fatal("derived MCP token accepted caller binding instead of source")
	}
	badHash := auth.HashToken(uuid.NewString())
	_, err = svc.FinalizeTaskClaim(ctx, *reclaimed, db.CreateTaskTokenParams{TokenHash: badHash, TaskID: reclaimed.ID, AgentID: reclaimed.AgentID, WorkspaceID: mustManagedUUID(t, source.WorkspaceID), UserID: f.user, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, db.CreateDaemonTokenParams{TokenHash: auth.HashToken(uuid.NewString()), WorkspaceID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, DaemonID: source.DaemonID, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	if !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("cross-workspace derived token minted: %v", err)
	}
	if _, err = f.svc.Queries.GetTaskTokenByHash(t.Context(), badHash); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("derived-token denial did not roll back task token")
	}
	historical, err := f.svc.Queries.GetAgentTask(t.Context(), reclaimed.ID)
	if err != nil || historical.ExecutionBindingID.Valid {
		t.Fatal("credential provenance backfilled execution history", err)
	}
}

func TestManagedRegistrationCannotCrossFirstBindWithLegacyPAT(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	other := f.platformAdminFixture.user(t, "")
	f.fx.Member(t, source.WorkspaceID, util.UUIDToString(other), "member")
	prior, err := f.svc.Queries.GetInstallationBinding(t.Context(), mustManagedUUID(t, source.BindingID))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", source.BindingID)
	bindTx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer bindTx.Rollback(t.Context())
	q := db.New(bindTx)
	if _, err = q.LockPasswordUser(t.Context(), f.user); err != nil {
		t.Fatal(err)
	}
	if err = q.LockInstallationNamespace(t.Context(), installationNamespace(prior.WorkspaceID, source.DaemonID)); err != nil {
		t.Fatal(err)
	}
	if _, err = q.CreateInstallationBinding(t.Context(), db.CreateInstallationBindingParams{InstallationID: prior.InstallationID, WorkspaceID: prior.WorkspaceID, DaemonID: source.DaemonID, PrincipalUserID: f.user, AuthVersion: 1, BindingEpoch: 2}); err != nil {
		t.Fatal(err)
	}
	started, done := make(chan struct{}), make(chan error, 1)
	go func() {
		ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(other), Kind: "pat", Version: 1})
		tx, e := f.pool.Begin(ctx)
		if e != nil {
			done <- e
			return
		}
		defer tx.Rollback(ctx)
		close(started)
		owner, e := LockManagedDaemonRegistration(ctx, db.New(tx), prior.WorkspaceID, source.DaemonID, other)
		if e == nil {
			_, e = tx.Exec(ctx, "UPDATE agent_runtime SET owner_id=$2 WHERE id=$1", runtimeID, owner)
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		done <- e
	}()
	<-started
	if err = bindTx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("registration/bind lock did not settle")
	}
	if !errors.Is(err, ErrManagedRuntimeSource) {
		t.Fatalf("legacy registration crossed first bind: %v", err)
	}
	runtime, err := f.svc.Queries.GetAgentRuntime(t.Context(), runtimeID)
	if err != nil || runtime.OwnerID != f.user {
		t.Fatal("runtime owner changed after first binding", err)
	}
}
func mustManagedUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	id, err := util.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
