package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationFixture struct {
	pool                                  *pgxpool.Pool
	fx                                    *testutil.Fixture
	svc                                   *CollaborationService
	creator, handler, outsider, workspace string
	ctx                                   context.Context
}

func newCollaborationFixture(t *testing.T) *collaborationFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	creator := fx.User(t, "Collaboration creator", "creator-"+suffix+"@example.test")
	handler := fx.User(t, "External handler", "handler-"+suffix+"@example.test")
	outsider := fx.User(t, "Unrelated account", "outsider-"+suffix+"@example.test")
	workspace := fx.Workspace(t, "Source workspace", "collab-"+suffix)
	fx.Member(t, workspace, creator, "owner")
	other := fx.Workspace(t, "Other workspace", "other-"+suffix)
	fx.Member(t, other, handler, "owner")
	fx.WorkspaceID, fx.UserID = workspace, creator
	q := db.New(pool)
	f := &collaborationFixture{pool: pool, fx: fx, creator: creator, handler: handler, outsider: outsider, workspace: workspace,
		svc: NewCollaborationService(q, pool, NewIssueService(q, pool, nil, nil, nil)), ctx: collaborationContext(creator)}
	fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	fx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	// Services own the rows created below. Test cleanup stays scoped to these
	// fixture accounts/workspace and runs before their parent fixture cleanup.
	fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{creator, handler, outsider})
	for _, table := range []string{"issue", "issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification"} {
		if table == "issue" {
			fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, workspace)
			continue
		}
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, workspace)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, workspace)
	return f
}

func collaborationContext(user string) context.Context {
	return auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT})
}

func (f *collaborationFixture) params(t *testing.T) CollaborationCreateParams {
	t.Helper()
	ws, err := util.ParseUUID(f.workspace)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := util.ParseUUID(f.handler)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationCreateParams{SourceWorkspaceID: ws, HandlerID: handler, RequestID: uuid.NewString(), Title: "Cross-workspace collaboration", Description: "Private source description"}
}

func TestCollaborationCreateIsAtomicIdempotentAndScoped(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	first, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.Create(f.ctx, p)
	if err != nil || replay.Issue.ID != first.Issue.ID {
		t.Fatalf("create replay duplicated issue: %+v %v", replay, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 {
		t.Fatal("duplicate source issue")
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
		t.Fatal("creation fabricated workspace membership")
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND revoked_at IS NULL`, first.Issue.ID) != 2 {
		t.Fatal("missing participants")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='paused'`, first.Issue.ID) != 1 {
		t.Fatal("new session was not atomically paused")
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, first.Issue.ID) != 0 {
		t.Fatal("creation started ordinary AI")
	}
	p.Title = "changed request"
	if _, err = f.svc.Create(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed payload replay: %v", err)
	}
	if _, err = f.svc.ResolveIssueAccess(collaborationContext(f.outsider), nil, first.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("outsider read: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
	if _, err = f.svc.ResolveIssueAccess(machine, nil, first.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("machine inherited creator content: %v", err)
	}
	access, err := f.svc.ResolveIssueAccess(collaborationContext(f.handler), nil, first.Issue.ID)
	if err != nil || !access.Capabilities.Edit || access.Capabilities.EditSourceStructure || access.Capabilities.Complete {
		t.Fatalf("external handler access: %+v %v", access, err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	access, err = f.svc.ResolveIssueAccess(f.ctx, nil, first.Issue.ID)
	if err != nil || !access.Capabilities.Complete || !access.Capabilities.Cancel || access.Capabilities.EditSourceStructure {
		t.Fatalf("departed creator access: %+v %v", access, err)
	}
}

func TestCollaborationCreateRollbackAndConcurrentReplay(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := f.svc.CreateInTx(f.ctx, tx, p, IssueCountPolicy{})
	if err != nil {
		_ = tx.Rollback(f.ctx)
		t.Fatal(err)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, created.Issue.ID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, p.SourceWorkspaceID) != 0 {
		t.Fatal("borrowed rollback left partial creation")
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan CollaborationIssueAccess, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := f.svc.Create(f.ctx, p)
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id pgtype.UUID
	for result := range results {
		if id.Valid && id != result.Issue.ID {
			t.Fatal("same request acquired two identities")
		}
		id = result.Issue.ID
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 {
		t.Fatal("concurrent replay duplicated issue")
	}
}

func TestCollaborationCreationGateAndSourceAuthority(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	if _, err := f.svc.Create(collaborationContext(f.handler), p); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("non-source member created issue: %v", err)
	}
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	if _, err := f.svc.Create(f.ctx, p); !collaborationErrorCode(err, "collaboration_unavailable") {
		t.Fatalf("closed creation gate ignored: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("denied creation wrote issue")
	}
}

func TestCollaborationCreationReceiptSurvivesSourceDepartureAndHandlerDisable(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	first, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	replay, err := f.svc.Create(f.ctx, p)
	if err != nil || replay.Issue.ID != first.Issue.ID {
		t.Fatalf("completed creation lost its recoverable identity: %+v %v", replay, err)
	}
	receipt, err := f.svc.GetCreation(f.ctx, p.SourceWorkspaceID, p.RequestID)
	if err != nil || receipt.Issue.ID != first.Issue.ID {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	if _, err := f.svc.GetCreation(f.ctx, p.SourceWorkspaceID, p.RequestID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("disabled creator read a receipt's private content: %v", err)
	}
}

func TestCollaborationCreateRejectsUntrustedPrincipalBeforeDependencies(t *testing.T) {
	svc := NewCollaborationService(nil, nil, nil)
	for _, ctx := range []context.Context{
		context.Background(),
		auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: "owner", Kind: auth.CredentialTask}),
		auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: "invalid-actor", Kind: auth.CredentialJWT}),
	} {
		if _, err := svc.Create(ctx, CollaborationCreateParams{}); !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("untrusted creation: %v", err)
		}
	}
}

func collaborationErrorCode(err error, code string) bool {
	var target *CollaborationError
	return errors.As(err, &target) && target.Code == code
}

func TestCollaborationAccessRevocationFallsBackOnlyToCurrentSourceMembership(t *testing.T) {
	f := newCollaborationFixture(t)
	created, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Member(t, f.workspace, f.handler, "member")
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, created.Issue.ID, f.handler)
	access, err := f.svc.ResolveIssueAccess(collaborationContext(f.handler), nil, created.Issue.ID)
	if err != nil || !access.Capabilities.Read || !access.Capabilities.Comment || access.Capabilities.ControlAI || access.Capabilities.Edit || access.Capabilities.Transfer {
		t.Fatalf("revoked participant did not retain only source-member rights: %+v %v", access.Capabilities, err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler)
	if _, err := f.svc.ResolveIssueAccess(collaborationContext(f.handler), nil, created.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("historical handler retained access after losing both bases: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2`, created.Issue.ID, f.handler) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, created.Issue.ID) != 1 {
		t.Fatal("authorization recheck changed retained responsibility/history")
	}
}

type collaborationFailureStarter struct {
	TxStarter
	queryName string
}

func (s collaborationFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return collaborationFailureTx{Tx: tx, queryName: s.queryName}, nil
}

type collaborationFailureTx struct {
	pgx.Tx
	queryName string
}

func (tx collaborationFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: "+tx.queryName+" ") {
		return errRow{err: errors.New("injected collaboration persistence failure")}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestCollaborationCreationFailureRollsBackEveryOwnedRecord(t *testing.T) {
	for _, queryName := range []string{"CreateIssueCollaboration", "CreateCollaborationParticipant", "CreateCollaborationSession", "CreateCollaborationEvent", "CreateCollaborationNotification", "CompleteCollaborationCreation"} {
		t.Run(queryName, func(t *testing.T) {
			f := newCollaborationFixture(t)
			p := f.params(t)
			f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: queryName}
			if _, err := f.svc.Create(f.ctx, p); err == nil || !strings.Contains(err.Error(), "injected collaboration persistence failure") {
				t.Fatalf("required write failure was not returned: %v", err)
			}
			for _, query := range []string{
				`SELECT count(*) FROM issue WHERE workspace_id=$1`,
				`SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`,
			} {
				if f.fx.Count(t, query, p.SourceWorkspaceID) != 0 {
					t.Fatalf("failed creation left partial state at %s", queryName)
				}
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE actor_user_id=$1`, f.creator) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE recipient_user_id=$1`, f.handler) != 0 {
				t.Fatal("failed creation retained participant, account fence, or notification")
			}
		})
	}
}

func TestCollaborationCreationUnknownCommitRecoversOriginalIdentity(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	f.svc.TxStarter = platformAdminUncertainCommitStarter{pool: f.pool}
	if _, err := f.svc.Create(f.ctx, p); err == nil || !strings.Contains(err.Error(), "commit collaboration creation") {
		t.Fatalf("lost commit response was reported as success: %v", err)
	}
	receipt, err := f.svc.GetCreation(f.ctx, p.SourceWorkspaceID, p.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.TxStarter = f.pool
	replay, err := f.svc.Create(f.ctx, p)
	if err != nil || replay.Issue.ID != receipt.Issue.ID {
		t.Fatalf("unknown commit replay did not recover original identity: %+v %v", replay, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, receipt.Issue.ID) != 1 {
		t.Fatal("unknown commit duplicated issue or assignment notification")
	}
	if _, err := f.svc.GetCreation(collaborationContext(f.handler), p.SourceWorkspaceID, p.RequestID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("another participant read creator-scoped creation receipt: %v", err)
	}
}
