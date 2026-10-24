package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminCollaborationOperationKinds(t *testing.T) {
	f := newPlatformAdminFixture(t)
	actor := f.user(t, PlatformRoleSuperAdmin)
	ctx := adminTestContext(actor, 1)
	for _, action := range []struct{ kind, code string }{
		{"collaboration.complete", "completed"},
		{"collaboration.cancel", "cancelled"},
		{"collaboration.reopen", "reopened"},
	} {
		t.Run(action.kind, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if err = LockPlatformAdminMutation(ctx, tx, actor); err != nil {
				t.Fatal(err)
			}
			op, replayed, err := f.svc.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{
				OrganizationID: f.org, TargetKind: "collaboration_issue", TargetID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Kind: action.kind, IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, PayloadHash: strings.Repeat("a", 64),
				Reason: "Resolve a disabled creator's collaboration", State: "applied", ResultCode: action.code,
			})
			if err != nil || replayed || op.ActorID != actor || op.Kind != action.kind || op.ResultCode != action.code {
				t.Fatalf("dedicated lifecycle operation = %+v, replayed=%t, err=%v", op, replayed, err)
			}
		})
	}
}

type adminCollaborationFixture struct {
	*collaborationQueueFixture
	actions    *AdminCollaborationService
	org, admin pgtype.UUID
	adminCtx   context.Context
}

func newAdminCollaborationFixture(t *testing.T, prepare ...func(*collaborationQueueFixture)) *adminCollaborationFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationLifecycleFixture(t)
	for _, seed := range prepare {
		seed(f)
	}
	q := f.svc.Queries
	admin := util.MustParseUUID(f.fx.User(t, "Independent administrator", "admin-"+uuid.NewString()+"@example.invalid"))
	hash, err := auth.HashPassword(f.ctx, platformAdminTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []pgtype.UUID{util.MustParseUUID(f.creator), util.MustParseUUID(f.handler), util.MustParseUUID(f.outsider), admin} {
		f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": id, "username": strings.ReplaceAll(uuid.NewString(), "-", ""), "password_hash": hash}, "user_id=$1", id)
		f.fx.Cleanup(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, id)
		f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, id)
	}
	f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": admin, "role": PlatformRoleSuperAdmin}, "user_id=$1", admin)
	org, err := q.GetInternalOrganization(f.ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		f.fx.Insert(t, "organization", testutil.Cols{"name": "Collaboration administration fixture"})
		org, err = q.GetInternalOrganization(f.ctx)
	}
	if err != nil {
		t.Fatal(err)
	}
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": f.workspace, "organization_id": org.ID}, "workspace_id=$1", f.workspace)
	for _, table := range []string{"admin_operation", "admin_audit_event"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE target_id=$1", f.issue.Issue.ID)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	t.Setenv("MULTICA_AUTH_MODE", "password")
	f.ctx = adminCollaborationContext(util.MustParseUUID(f.creator), 1)
	return &adminCollaborationFixture{collaborationQueueFixture: f, actions: NewAdminCollaborationService(q, f.pool, true),
		org: org.ID, admin: admin, adminCtx: adminCollaborationContext(admin, 1)}
}

func adminCollaborationContext(user pgtype.UUID, version int64) context.Context {
	return auth.WithPasswordSession(collaborationContext(util.UUIDToString(user)), auth.PasswordSession{UserID: util.UUIDToString(user), Version: version, Kind: "jwt"})
}

func (f *adminCollaborationFixture) params(t *testing.T, action string) AdminCollaborationLifecycleParams {
	t.Helper()
	issue, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := f.svc.Queries.ListIssueStatusEntries(f.ctx, db.ListIssueStatusEntriesParams{WorkspaceID: issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	options, err := collaborationStatusOptions(issue, action, catalog, db.IssueHandoff{})
	if err != nil {
		t.Fatal(err)
	}
	var revision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, issue.ID).Scan(&revision)
	return AdminCollaborationLifecycleParams{OrganizationID: f.org, IssueID: issue.ID, IdempotencyKey: util.MustParseUUID(uuid.NewString()),
		ExpectedCollaborationRevision: revision, ExpectedStatus: issue.Status, Action: action, TargetStatus: options.DefaultStatus,
		StatusOptionsVersion: options.StatusOptionsVersion, Reason: "Resolve the disabled creator's pending request", RequestID: uuid.NewString()}
}

func TestAdminCollaborationLifecyclePreservesCreatorAndRealActor(t *testing.T) {
	for _, action := range []string{"complete", "cancel"} {
		t.Run(action, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			p := f.params(t, action)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
			if err != nil {
				t.Fatalf("disabled creator's %s: %v", action, err)
			}
			if result.Result.CollaborationRevision != 2 || result.Result.CreatorUserID != util.MustParseUUID(f.creator) ||
				result.Result.HandlerUserID != util.MustParseUUID(f.handler) || result.Result.QueueState != "closed" ||
				result.Operation.ActorID != f.admin || result.Operation.ActorAuthVersion != 1 {
				t.Fatalf("lifecycle result changed identity or missed closure: %+v", result)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, p.IssueID, f.admin) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND actor_user_id=$2 AND reason=$3 AND after_state->>'creator_user_id'=$4`, result.Operation.ID, f.admin, p.Reason, f.creator) != 1 {
				t.Fatal("administration enrolled the actor or lost their original-creator audit")
			}
		})
	}
}

func TestAdminCollaborationReopenAssignsExistingParticipantAtomically(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	closeParams := lifecycleParams(t, f.collaborationQueueFixture, "complete", "done", 1, 1)
	if _, err := f.svc.ChangeLifecycle(f.ctx, closeParams); err != nil {
		t.Fatal(err)
	}
	// Retained information history cannot become a stale return-to obligation
	// when administration assigns the reopened responsibility to someone else.
	information := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator,
		"from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "in_progress"})
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, information)
	successor := util.MustParseUUID(f.outsider)
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": successor}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, successor)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
	p := f.params(t, "reopen")
	p.SuccessorUserID = &successor
	result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
	if err != nil {
		t.Fatalf("double-disabled reopen: %v", err)
	}
	if result.Result.HandlerUserID != successor || result.Result.PreviousHandlerUserID != util.MustParseUUID(f.handler) ||
		result.Result.CreatorUserID != util.MustParseUUID(f.creator) || result.Result.QueueState != "paused" || result.Result.CollaborationRevision != 3 {
		t.Fatalf("reopen/handler result = %+v", result.Result)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen' AND actor_user_id=$2 AND from_user_id=$3 AND to_user_id=$4`, p.IssueID, f.admin, f.handler, successor) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='superseded'`, information) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND active_information_request_id IS NULL`, p.IssueID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND creator_id=$2 AND assignee_id=$3 AND status='todo'`, p.IssueID, f.creator, successor) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='paused'`, p.IssueID) != 1 {
		t.Fatal("reopen did not atomically persist responsibility and its paused session")
	}
	current, err := f.svc.ListWorkbench(adminCollaborationContext(successor, 1), CollaborationWorkbenchQuery{View: "current", Limit: 10})
	if err != nil || len(current.Items) != 1 || current.Items[0].ID != p.IssueID {
		t.Fatalf("successor's durable workbench = %+v, %v", current, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1`, p.IssueID) != 3 {
		t.Fatal("reopen changed the participant roster")
	}
}
