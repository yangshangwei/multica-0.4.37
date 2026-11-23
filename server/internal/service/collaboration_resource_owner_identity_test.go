package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type resourceIdentityQueryError struct {
	db.DBTX
	err     error
	inspect func(context.Context)
}

func (q resourceIdentityQueryError) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: GetPasswordCredential ") {
		if q.inspect != nil {
			q.inspect(ctx)
		}
		if q.err != nil {
			return queueFailureRow{err: q.err}
		}
	}
	return q.DBTX.QueryRow(ctx, sql, args...)
}

// This regression borrows only tracked fixtures. A resource owner's validated
// binding is an authority fact; it must never replace the request's caller.
func TestCollaborationResourceIdentityValidatesStoredOwnerSeparatelyFromDaemonCaller(t *testing.T) {
	for _, scenario := range []string{"valid", "owner_unknown", "owner_password_missing", "owner_password_stale", "binding_missing", "binding_epoch_stale", "binding_revoked", "owner_membership_lost", "owner_disabled", "installation_revoked", "database_failure", "cancelled", "cancelled_during_owner_check"} {
		t.Run(scenario, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			caller := f.handler
			f.fx.Member(t, f.workspace, caller, "member")
			daemon := uuid.NewString()
			runtime := f.fx.Runtime(t, "Independent caller runtime", testutil.Cols{"owner_id": caller, "daemon_id": daemon, "provider": "codex", "status": "online"})
			binding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": f.installation.ID, "workspace_id": f.workspace,
				"daemon_id": daemon, "principal_user_id": caller, "auth_version": 1, "binding_epoch": 1})
			foreignSession := auth.PasswordSession{UserID: caller, Version: 1, Kind: "daemon_token", WorkspaceID: f.workspace,
				DaemonID: daemon, BindingID: binding, BindingEpoch: 1}
			foreign := auth.WithPasswordSession(f.ctx, foreignSession)
			if _, _, err := f.svc.authorizeCollaborationDaemon(foreign, f.svc.Queries, util.MustParseUUID(runtime)); err != nil {
				t.Fatalf("different valid caller prerequisite: %v", err)
			}
			resource := collaboration.TargetResource{Role: "agent", AgentID: util.UUIDToString(f.grant.AgentID), RuntimeID: util.UUIDToString(f.runtime.ID),
				OwnerUserID: f.creator, ExecutionWorkspaceID: f.workspace}
			identity := collaborationGrantIdentity(f.grant)
			wanted, injected := true, error(nil)
			switch scenario {
			case "owner_unknown":
				wanted = false
				resource.OwnerUserID = uuid.NewString()
			case "owner_password_missing":
				wanted = false
				f.fx.Exec(t, `DELETE FROM user_password_credential WHERE user_id=$1`, f.creator)
			case "owner_password_stale":
				wanted = false
				f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.creator)
			case "binding_missing":
				wanted = false
				identity.BindingID = util.MustParseUUID(uuid.NewString())
			case "binding_epoch_stale":
				wanted = false
				f.fx.Exec(t, `UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$1`, f.binding.ID)
			case "binding_revoked":
				wanted = false
				f.fx.Exec(t, `UPDATE installation_daemon_binding SET state='revoked',revoked_at=now() WHERE id=$1`, f.binding.ID)
			case "owner_membership_lost":
				wanted = false
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
			case "owner_disabled":
				wanted = false
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			case "installation_revoked":
				wanted = false
				f.fx.Exec(t, `UPDATE managed_installation SET lifecycle='retired' WHERE id=$1`, f.installation.ID)
			case "database_failure":
				wanted, injected = false, errors.New("resource owner password lookup unavailable")
			case "cancelled", "cancelled_during_owner_check":
				wanted, injected = false, context.Canceled
			}
			for _, contextCase := range []struct {
				name string
				ctx  context.Context
			}{{"same_owner_daemon", f.daemonCtx}, {"different_valid_daemon", foreign}, {"human", f.ctx}} {
				t.Run(contextCase.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(contextCase.ctx, 5*time.Second)
					defer cancel()
					if scenario == "cancelled" {
						cancel()
					}
					before, hasSession := auth.PasswordSessionFromContext(ctx)
					deadline, hasDeadline := ctx.Deadline()
					observed := false
					passwordErr := error(nil)
					if scenario == "database_failure" {
						passwordErr = injected
					}
					queries := db.New(resourceIdentityQueryError{DBTX: f.pool, err: passwordErr, inspect: func(ownerCtx context.Context) {
						observed = true
						owner, ok := auth.PasswordSessionFromContext(ownerCtx)
						ownerDeadline, ownerHasDeadline := ownerCtx.Deadline()
						if !ok || owner.UserID != resource.OwnerUserID || owner.Version != f.binding.AuthVersion || owner.Kind != "daemon_token" ||
							owner.BindingID != util.UUIDToString(f.binding.ID) || owner.BindingEpoch != f.binding.BindingEpoch ||
							owner.WorkspaceID != resource.ExecutionWorkspaceID || owner.DaemonID != f.binding.DaemonID ||
							ownerHasDeadline != hasDeadline || ownerDeadline != deadline {
							t.Fatal("password check did not use the verified stored owner with the original request deadline")
						}
						if scenario == "cancelled_during_owner_check" {
							cancel()
						}
					}})
					current, err := collaborationIdentityCurrent(ctx, queries, resource, identity)
					if injected != nil {
						if !errors.Is(err, injected) || current {
							t.Fatalf("resource identity swallowed cancellation/dependency failure: current=%v err=%v", current, err)
						}
					} else if err != nil || current != wanted {
						t.Fatalf("resource owner identity was judged using %s caller: current=%v want=%v err=%v", contextCase.name, current, wanted, err)
					}
					after, stillSession := auth.PasswordSessionFromContext(ctx)
					afterDeadline, stillDeadline := ctx.Deadline()
					if hasSession != stillSession || !reflect.DeepEqual(before, after) || hasDeadline != stillDeadline || deadline != afterDeadline {
						t.Fatal("resource identity validation replaced the request's caller/deadline")
					}
					if (scenario == "valid" || scenario == "database_failure" || scenario == "cancelled_during_owner_check") && !observed {
						t.Fatal("resource owner password check was bypassed")
					}
				})
			}
			if scenario != "valid" {
				return
			}
			input := f.submitManaged(t, f.creator, "Caller runtime authority stays separate from resource validity")
			f.resumeManaged(t)
			run, request, _ := f.claimManaged(t)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			if _, err := f.svc.ClaimRuntimeRun(foreign, run, request); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("different valid daemon caller acquired resource owner's claim: %v", err)
			}
			if !input.CommandID.Valid {
				t.Fatal("valid independent caller control lacked a retained input")
			}
			staleCaller := foreignSession
			staleCaller.BindingEpoch++
			staleContext := auth.WithPasswordSession(f.ctx, staleCaller)
			if _, err := f.svc.ListRuntimeRuns(staleContext, util.MustParseUUID(runtime)); err == nil {
				t.Fatal("stale external caller binding passed ordinary List authority")
			}
			if _, err := f.svc.ListRuntimeRuns(f.ctx, f.runtime.ID); err == nil {
				t.Fatal("human context was authenticated as a managed daemon by resource check")
			}
		})
	}
}
