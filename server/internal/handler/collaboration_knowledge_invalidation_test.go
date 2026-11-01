package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeHookFailureStarter struct{ service.TxStarter }

func (s knowledgeHookFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeHookFailureTx{tx}, nil
}

type knowledgeHookFailureTx struct{ pgx.Tx }

func (tx knowledgeHookFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO collaboration_knowledge_invalidation") {
		return pgconn.CommandTag{}, errors.New("injected durable invalidation failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func knowledgeHookWorkspace(t *testing.T) string {
	t.Helper()
	ws := dbfx.Workspace(t, "Knowledge hook workspace", "knowledge-hook-"+uuid.NewString())
	dbfx.Member(t, ws, testUserID, "owner")
	return ws
}

func seedKnowledgeHookTarget(t *testing.T, workspace, project string) {
	t.Helper()
	kind, target := "workspace", workspace
	if project != "" {
		kind, target = "project", project
	}
	dbfx.InsertNoID(t, "collaboration_knowledge_target", testutil.Cols{"target_kind": kind, "target_id": target, "workspace_id": workspace, "knowledge_revision": 1}, "target_kind=$1 AND target_id=$2", kind, target)
}

func TestKnowledgeMemberRoleAndRemovalAppendInAuthoritativeTransaction(t *testing.T) {
	ws := knowledgeHookWorkspace(t)
	user := dbfx.User(t, "Departing source member", "knowledge-member-"+uuid.NewString()+"@example.test")
	member := dbfx.Member(t, ws, user, "member")
	request := func(role string) *http.Request {
		r := newRequest("PUT", "/api/workspaces/"+ws+"/members/"+member, map[string]string{"role": role})
		r.Header.Set("X-Workspace-ID", ws)
		return withURLParams(r, "id", ws, "memberId", member)
	}
	failing := *testHandler
	failing.TxStarter = knowledgeHookFailureStarter{testPool}
	testutil.Call(t, failing.UpdateMember, request("admin")).Want(500)
	if dbfx.Count(t, `SELECT count(*) FROM member WHERE id=$1 AND role='member'`, member) != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1`, ws) != 0 {
		t.Fatal("role transition committed without its durable scan")
	}
	testutil.Call(t, testHandler.UpdateMember, request("admin")).Want(200)
	testutil.Call(t, testHandler.UpdateMember, request("admin")).Want(200)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_kind='source_workspace' AND scope_id=$1`, ws) != 1 {
		t.Fatal("role change missing scan or unchanged role duplicated it")
	}
	remove := func() *http.Request {
		r := newRequest("DELETE", "/api/workspaces/"+ws+"/members/"+member, nil)
		r.Header.Set("X-Workspace-ID", ws)
		return withURLParams(r, "id", ws, "memberId", member)
	}
	testutil.Call(t, failing.DeleteMember, remove()).Want(500)
	if dbfx.Count(t, `SELECT count(*) FROM member WHERE id=$1`, member) != 1 {
		t.Fatal("member removed without durable scan")
	}
	testutil.Call(t, testHandler.DeleteMember, remove()).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM member WHERE id=$1`, member) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_kind='source_workspace' AND scope_id=$1`, ws) != 2 {
		t.Fatal("member removal did not share its scan transaction")
	}
}

func TestKnowledgePATRevocationRetainsOriginalCauseAndRollsBackOnAppendFailure(t *testing.T) {
	_, id := insertTestPAT(t, time.Now().Add(time.Hour))
	request := func() *http.Request { return withURLParam(newRequest("DELETE", "/api/tokens/"+id, nil), "id", id) }
	cause := uuid.NewSHA1(uuid.MustParse(id), []byte("collaboration-knowledge:pat-revoked"))
	failing := *testHandler
	failing.TxStarter = knowledgeHookFailureStarter{testPool}
	testutil.Call(t, failing.RevokePersonalAccessToken, request()).Want(500)
	if dbfx.Count(t, `SELECT count(*) FROM personal_access_token WHERE id=$1 AND NOT revoked`, id) != 1 {
		t.Fatal("credential revoked without durable scan")
	}
	testutil.Call(t, testHandler.RevokePersonalAccessToken, request()).Want(204)
	testutil.Call(t, testHandler.RevokePersonalAccessToken, request()).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM personal_access_token WHERE id=$1 AND revoked`, id) != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1 AND scope_kind='all_sources'`, cause.String()) != 1 {
		t.Fatal("PAT retry lost or changed its original scan cause")
	}
	missing := uuid.NewString()
	testutil.Call(t, testHandler.RevokePersonalAccessToken, withURLParam(newRequest("DELETE", "/api/tokens/"+missing, nil), "id", missing)).Want(204)
	missingCause := uuid.NewSHA1(uuid.MustParse(missing), []byte("collaboration-knowledge:pat-revoked"))
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE cause_id=$1`, missingCause.String()) != 0 {
		t.Fatal("missing token minted an invalidation")
	}
}

func TestKnowledgeProjectDeletionRetainsFinalHintAndPendingRouting(t *testing.T) {
	project := dbfx.Project(t, "Knowledge deletion target")
	seedKnowledgeHookTarget(t, testWorkspaceID, project)
	request := func() *http.Request {
		return withURLParam(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)
	}
	failing := *testHandler
	failing.TxStarter = knowledgeHookFailureStarter{testPool}
	testutil.Call(t, failing.DeleteProject, request()).Want(503)
	if dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, project) != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_id=$1 AND knowledge_revision=1`, project) != 1 {
		t.Fatal("project deletion committed before durable final hint")
	}
	testutil.Call(t, testHandler.DeleteProject, request()).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, project) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_id=$1`, project) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_id=$1 AND target_workspace_id=$2 AND knowledge_revision=2 AND state='pending'`, project, testWorkspaceID) != 1 {
		t.Fatal("project deletion dropped final hint or retained counter")
	}
}

func TestKnowledgeWorkspaceDeletionRetainsEveryTargetHint(t *testing.T) {
	ws := knowledgeHookWorkspace(t)
	project := dbfx.Project(t, "Workspace deletion project", testutil.Cols{"workspace_id": ws})
	seedKnowledgeHookTarget(t, ws, "")
	seedKnowledgeHookTarget(t, ws, project)
	request := func() *http.Request {
		r := newRequest("DELETE", "/api/workspaces/"+ws, nil)
		r.Header.Set("X-Workspace-ID", ws)
		return withURLParam(r, "id", ws)
	}
	failing := *testHandler
	failing.TxStarter = knowledgeHookFailureStarter{testPool}
	testutil.Call(t, failing.DeleteWorkspace, request()).Want(500)
	if dbfx.Count(t, `SELECT count(*) FROM workspace WHERE id=$1`, ws) != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1 AND knowledge_revision=1`, ws) != 2 {
		t.Fatal("workspace deletion escaped hint rollback")
	}
	testutil.Call(t, testHandler.DeleteWorkspace, request()).Want(204)
	if dbfx.Count(t, `SELECT count(*) FROM workspace WHERE id=$1`, ws) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1`, ws) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint' AND knowledge_revision=2 AND state='pending'`, ws) != 2 {
		t.Fatal("workspace deletion erased pending routing or left active targets")
	}
}

func TestKnowledgeMemberRemovalFansOutHistoricalTargetsOutsideSource(t *testing.T) {
	source := knowledgeHookWorkspace(t)
	// Retained publication history outlives fixture cleanup, so its target must
	// not share the suite workspace after the target counter is removed.
	target := knowledgeHookWorkspace(t)
	user := dbfx.User(t, "Historical source reader", "knowledge-history-"+uuid.NewString()+"@example.test")
	member := dbfx.Member(t, source, user, "member")
	seedKnowledgeHookTarget(t, target, "")
	q := testHandler.Queries
	row, err := q.CreateKnowledgeCandidate(context.Background(), db.CreateKnowledgeCandidateParams{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, SourceWorkspaceID: parseUUID(source), SourceIssueID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, TargetKind: "workspace", TargetWorkspaceID: parseUUID(target), Content: "PRIVATE_SOURCE_HISTORY", Summary: "PRIVATE_SUMMARY", ContentSha256: protocol.CollaborationKnowledgeHash("PRIVATE_SOURCE_HISTORY"), Sources: []byte(`[{"kind":"memory"}]`), Evidence: []byte(`[]`), Applicability: []byte(`{"surfaces":["issue"]}`), ProposedByUserID: parseUUID(user)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.ApproveKnowledgeCandidate(context.Background(), db.ApproveKnowledgeCandidateParams{ID: row.ID, ActorID: parseUUID(testUserID), TargetRevision: pgtype.Int8{Int64: 1, Valid: true}, ExpectedStateRevision: 1, ExpectedVersion: 1, ExpectedContentSha256: row.ContentSha256}); err != nil {
		t.Fatal(err)
	}
	r := newRequest("DELETE", "/api/workspaces/"+source+"/members/"+member, nil)
	r.Header.Set("X-Workspace-ID", source)
	testutil.Call(t, testHandler.DeleteMember, withURLParams(r, "id", source, "memberId", member)).Want(204)
	// Run the real global worker; unrelated pending jobs from earlier hook
	// tests are legitimate and may conservatively increment this target too.
	w := &service.CollaborationKnowledgeInvalidation{Queries: q, TxStarter: testPool}
	for range 50 {
		n, err := w.ScanPending(context.Background(), 100)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation h JOIN collaboration_knowledge_invalidation s ON s.cause_id=h.cause_id AND s.kind='scan' WHERE s.scope_id=$1 AND h.target_workspace_id=$2 AND h.kind='target_hint'`, source, target) != 1 {
		t.Fatal("historical outside target disappeared when source member was removed")
	}
}

func TestKnowledgeMembershipGrantsAppendOnlyWithCommittedMembership(t *testing.T) {
	for _, via := range []string{"invitation", "share_link"} {
		t.Run(via, func(t *testing.T) {
			ws := knowledgeHookWorkspace(t)
			user := dbfx.User(t, "Knowledge joining member", "knowledge-join-"+uuid.NewString()+"@example.test")
			var invoke func(*Handler) *testutil.Response
			if via == "invitation" {
				id := dbfx.Insert(t, "workspace_invitation", testutil.Cols{"workspace_id": ws, "inviter_id": testUserID, "invitee_email": "knowledge-invite@example.test", "invitee_user_id": user, "role": "member", "status": "pending", "expires_at": time.Now().Add(time.Hour)})
				invoke = func(h *Handler) *testutil.Response {
					r := newRequest("POST", "/api/invitations/"+id+"/accept", nil)
					r.Header.Set("X-User-ID", user)
					return testutil.Call(t, h.AcceptInvitation, withURLParam(r, "id", id))
				}
			} else {
				code := uuid.NewString()
				dbfx.Insert(t, "workspace_share_link", testutil.Cols{"workspace_id": ws, "created_by": testUserID, "code": code, "role": "member"})
				invoke = func(h *Handler) *testutil.Response {
					r := newRequest("POST", "/api/share-links/join", map[string]string{"code": code})
					r.Header.Set("X-User-ID", user)
					return testutil.Call(t, h.JoinByShareLink, r)
				}
			}
			failing := *testHandler
			failing.TxStarter = knowledgeHookFailureStarter{testPool}
			invoke(&failing).Want(500)
			if dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, ws, user) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1`, ws) != 0 {
				t.Fatal("membership grant escaped scan rollback")
			}
			invoke(testHandler).Want(200)
			if dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, ws, user) != 1 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1 AND scope_kind='source_workspace'`, ws) != 1 {
				t.Fatal("committed membership grant omitted source scan")
			}
		})
	}
}

func TestKnowledgeDeviceJoinInvalidatesExistingWorkspaceOnly(t *testing.T) {
	slug := "knowledge-device-" + uuid.NewString()
	enableDeviceAuth(t, slug, "member")
	ws := dbfx.Workspace(t, "Existing device workspace", slug)
	device := strings.ReplaceAll(uuid.NewString(), "-", "")
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE email=$1`, deviceUserEmail(device))
	failing := *testHandler
	failing.TxStarter = knowledgeHookFailureStarter{testPool}
	if _, err := failing.provisionDeviceIdentity(context.Background(), device, "new device"); err == nil {
		t.Fatal("device membership committed without scan")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1`, ws) != 0 {
		t.Fatal("failed device join left scan")
	}
	if _, err := testHandler.provisionDeviceIdentity(context.Background(), device, "new device"); err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.provisionDeviceIdentity(context.Background(), device, "new device"); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE scope_id=$1 AND scope_kind='source_workspace'`, ws) != 1 {
		t.Fatal("device grant missing scan or replay generated another")
	}
	freshSlug := "knowledge-device-new-" + uuid.NewString()
	enableDeviceAuth(t, freshSlug, "member")
	freshDevice := strings.ReplaceAll(uuid.NewString(), "-", "")
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE email=$1`, deviceUserEmail(freshDevice))
	if _, err := testHandler.provisionDeviceIdentity(context.Background(), freshDevice, "fresh workspace device"); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation k JOIN workspace w ON w.id=k.scope_id WHERE w.slug=$1`, freshSlug) != 0 {
		t.Fatal("new empty workspace generated a historical source scan")
	}
}
