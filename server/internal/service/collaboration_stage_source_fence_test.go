package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageSourceFenceStarter struct {
	TxStarter
	query  string
	before func()
}
type stageSourceFenceTx struct {
	pgx.Tx
	query  string
	before func()
	fired  bool
}

func (s stageSourceFenceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageSourceFenceTx{Tx: tx, query: s.query, before: s.before}, nil
}
func (t *stageSourceFenceTx) fire(sql string) {
	if !t.fired && strings.Contains(sql, "-- name: "+t.query+" ") {
		t.fired = true
		t.before()
	}
}
func (t *stageSourceFenceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.fire(sql)
	return t.Tx.Exec(ctx, sql, args...)
}
func (t *stageSourceFenceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.fire(sql)
	return t.Tx.QueryRow(ctx, sql, args...)
}

func stageTryDeleteMember(t *testing.T, f *collaborationFixture, user string) error {
	t.Helper()
	member, err := f.svc.Queries.GetMemberByUserAndWorkspace(f.ctx, db.GetMemberByUserAndWorkspaceParams{UserID: util.MustParseUUID(user), WorkspaceID: util.MustParseUUID(f.workspace)})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), "SET LOCAL lock_timeout='50ms'"); err != nil {
		t.Fatal(err)
	}
	err = db.New(tx).DeleteMember(context.Background(), member.ID)
	if err != nil {
		return err
	}
	return tx.Commit(context.Background())
}

func TestStageScopeMemberFenceAtActualDecisionWrite(t *testing.T) {
	f := newStageReviewFixture(t)
	f.fx.Member(t, f.workspace, f.handler, "member")
	roles := f.roleSet()
	roles.ProductApprover = util.MustParseUUID(f.handler)
	if _, err := f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, Roles: roles}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.StageView(f.handlerCtx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	digest := stageReadableDigest(t, stageChildView(t, view, f.child))
	p := CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.handlerCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 2, ExpectedScopeRevision: 1, Children: []CollaborationStageScopeChild{{IssueID: util.MustParseUUID(f.child), ContentDigest: digest}}}
	hits := 0
	var removal error
	f.svc.TxStarter = stageSourceFenceStarter{TxStarter: f.pool, query: "CreateCollaborationStageDecision", before: func() { hits++; removal = stageTryDeleteMember(t, f.collaborationFixture, f.handler) }}
	_, err = f.svc.ConfirmStageScope(f.handlerCtx, p)
	f.svc.TxStarter = f.pool
	var locked *pgconn.PgError
	if hits != 1 || !errors.As(removal, &locked) || locked.Code != "55P03" || err != nil {
		t.Fatalf("scope committed after source deletion: hook=%d deletion=%v confirmation=%v", hits, removal, err)
	}
	if err = stageTryDeleteMember(t, f.collaborationFixture, f.handler); err != nil {
		t.Fatal(err)
	}
	p.CollaborationWorkflowRequest = f.req(t, f.handlerCtx)
	p.ExpectedScopeRevision = 2
	if _, err = f.svc.ConfirmStageScope(f.handlerCtx, p); !collaborationErrorCode(err, "stage_scope_child_unavailable") {
		t.Fatalf("revocation-first cached selection: %v", err)
	}
	p.CollaborationWorkflowRequest = f.req(t, f.handlerCtx)
	p.Children = []CollaborationStageScopeChild{}
	if _, err = f.svc.ConfirmStageScope(f.handlerCtx, p); err != nil {
		t.Fatalf("empty external scope unexpectedly requires source membership: %v", err)
	}
}
