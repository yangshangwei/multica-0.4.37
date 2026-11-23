package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeApprovalHTTPFixture struct {
	workspace, issue, project, approver, member string
	proposal                                    protocol.CollaborationKnowledgeProposal
	approval                                    protocol.CollaborationKnowledgeApproveRequest
	ctx                                         context.Context
}

func knowledgeApprovalHTTPGuard(t *testing.T) {
	t.Helper()
	u, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil || u.Path == "" {
		t.Fatal("approval HTTP matrix requires explicit isolated DATABASE_URL")
	}
	if want := os.Getenv("MULTICA_KNOWLEDGE_MATRIX_DATABASE"); want != "" && strings.TrimPrefix(u.Path, "/") != want {
		t.Fatal("approval HTTP matrix selected another lane's database")
	}
}
func newKnowledgeApprovalHTTPFixture(t *testing.T, kind, role string) *knowledgeApprovalHTTPFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	f := &knowledgeApprovalHTTPFixture{ctx: ctx}
	f.workspace = dbfx.Workspace(t, "Approval handler matrix", "approval-http-"+uuid.NewString())
	dbfx.Member(t, f.workspace, testUserID, "owner")
	f.approver = dbfx.User(t, "Independent human confirmer", "approval-http-"+uuid.NewString()+"@example.test")
	f.member = dbfx.Member(t, f.workspace, f.approver, role)
	f.issue = collaborationBoundaryIssue(t, f.workspace)
	// Source and target are independent: deleting the target project must not
	// be mistaken for deleting a project that structurally contains this issue.
	f.project = dbfx.Project(t, "Independent knowledge target", testutil.Cols{"workspace_id": f.workspace, "lead_type": "member", "lead_id": f.approver})
	for _, table := range []string{"collaboration_memory_version", "collaboration_event"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", f.issue)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue)
	ownerCtx := auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
	svc := service.NewCollaborationService(testHandler.Queries, testPool, testHandler.IssueService)
	if _, err := svc.UpdateMemory(ownerCtx, service.CollaborationMemoryUpdateParams{IssueID: parseUUID(f.issue), RequestID: uuid.NewString(), Content: "PRIVATE_HTTP_MATRIX_SOURCE"}); err != nil {
		t.Fatal(err)
	}
	target := protocol.CollaborationKnowledgeTarget{Kind: kind, WorkspaceID: f.workspace}
	if kind == "project" {
		target.ProjectID = f.project
	}
	f.proposal = protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: target, Summary: "PRIVATE_HTTP_MATRIX_SUMMARY", Content: "PRIVATE_HTTP_MATRIX_BODY", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: f.issue, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_HTTP_MATRIX_SOURCE")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}}}
	if _, err := svc.ProposeKnowledge(ownerCtx, parseUUID(f.issue), f.proposal); err != nil {
		t.Fatal(err)
	}
	f.approval = protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(f.proposal.Content)}
	return f
}
func (f *knowledgeApprovalHTTPFixture) request(actor, method, path string, body any, params ...string) *http.Request {
	r := testutil.JSONRequest(method, path, body)
	r.Header.Set("X-User-ID", actor)
	r.Header.Set("X-Workspace-ID", f.workspace)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	return testutil.WithURLParams(r, params...)
}

type knowledgeApprovalHTTPGate struct {
	service.TxStarter
	entered chan uint32
	release chan struct{}
	once    *sync.Once
}
type knowledgeApprovalHTTPGateTx struct {
	pgx.Tx
	gate knowledgeApprovalHTTPGate
}

func (s knowledgeApprovalHTTPGate) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeApprovalHTTPGateTx{Tx: tx, gate: s}, nil
}
func (tx knowledgeApprovalHTTPGateTx) Commit(ctx context.Context) error {
	tx.gate.once.Do(func() {
		tx.gate.entered <- tx.Conn().PgConn().PID()
		select {
		case <-tx.gate.release:
		case <-ctx.Done():
		}
	})
	return tx.Tx.Commit(ctx)
}

type knowledgeApprovalHTTPState struct{ code, query string }
type knowledgeApprovalHTTPObserver struct {
	service.TxStarter
	states chan knowledgeApprovalHTTPState
}
type knowledgeApprovalHTTPObservedTx struct {
	pgx.Tx
	states chan knowledgeApprovalHTTPState
}
type knowledgeApprovalHTTPObservedRow struct {
	pgx.Row
	states chan knowledgeApprovalHTTPState
	query  string
}

func (s knowledgeApprovalHTTPObserver) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeApprovalHTTPObservedTx{Tx: tx, states: s.states}, nil
}
func knowledgeApprovalHTTPRecord(states chan knowledgeApprovalHTTPState, query string, err error) {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		select {
		case states <- knowledgeApprovalHTTPState{pgerr.Code, strings.SplitN(query, "\n", 2)[0]}:
		default:
		}
	}
}
func (tx knowledgeApprovalHTTPObservedTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return knowledgeApprovalHTTPObservedRow{Row: tx.Tx.QueryRow(ctx, query, args...), states: tx.states, query: query}
}
func (r knowledgeApprovalHTTPObservedRow) Scan(values ...any) error {
	err := r.Row.Scan(values...)
	knowledgeApprovalHTTPRecord(r.states, r.query, err)
	return err
}
func (tx knowledgeApprovalHTTPObservedTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, query, args...)
	knowledgeApprovalHTTPRecord(tx.states, query, err)
	return tag, err
}

type knowledgeApprovalHTTPCall func(service.TxStarter) *testutil.Response

func knowledgeApprovalHTTPAwait(t *testing.T, ch <-chan *testutil.Response) *testutil.Response {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(8 * time.Second):
		t.Fatal("actual HTTP writer did not finish after its commit barrier")
		return nil
	}
}
func knowledgeApprovalHTTPBlock(t *testing.T, holder uint32) {
	t.Helper()
	until := time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		var blocked bool
		if err := testPool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)))`, int32(holder)).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			t.Log("observed actual handler writer's PostgreSQL wait chain before first commit")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("actual handler follower never reached its expected PostgreSQL wait chain")
}
func knowledgeApprovalHTTPPair(t *testing.T, first, second knowledgeApprovalHTTPCall, nowait bool) (*testutil.Response, *testutil.Response) {
	t.Helper()
	gate := knowledgeApprovalHTTPGate{TxStarter: testPool, entered: make(chan uint32, 1), release: make(chan struct{}), once: &sync.Once{}}
	defer func() {
		select {
		case <-gate.release:
		default:
			close(gate.release)
		}
	}()
	firstDone, secondDone := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
	go func() { firstDone <- first(gate) }()
	var holder uint32
	select {
	case holder = <-gate.entered:
	case r := <-firstDone:
		t.Fatalf("first actual handler refused before commit gate: %d %s", r.Code, r.Text())
	case <-time.After(8 * time.Second):
		t.Fatal("first handler did not reach commit gate")
	}
	states := make(chan knowledgeApprovalHTTPState, 16)
	go func() { secondDone <- second(knowledgeApprovalHTTPObserver{TxStarter: testPool, states: states}) }()
	var follower *testutil.Response
	if nowait {
		follower = knowledgeApprovalHTTPAwait(t, secondDone)
		follower.Want(503)
		if follower.Map()["code"] != "knowledge_busy" {
			t.Fatalf("wrong bounded approval refusal: %s", follower.Text())
		}
		seen := false
		for len(states) > 0 {
			state := <-states
			if state.code == "55P03" {
				seen = true
				t.Logf("actual handler NOWAIT boundary: %s SQLSTATE %s", state.query, state.code)
			}
		}
		if !seen {
			t.Fatal("NOWAIT refusal had no PostgreSQL observation")
		}
	} else {
		knowledgeApprovalHTTPBlock(t, holder)
	}
	close(gate.release)
	leader := knowledgeApprovalHTTPAwait(t, firstDone)
	if !nowait {
		follower = knowledgeApprovalHTTPAwait(t, secondDone)
	}
	return leader, follower
}

func TestKnowledgeApprovalHTTPMatrixAuthorityAndProjectDeletionBothOrders(t *testing.T) {
	knowledgeApprovalHTTPGuard(t)
	for _, change := range []string{"role_downgrade", "member_removal", "lead_change", "project_delete"} {
		for _, approvalFirst := range []bool{true, false} {
			t.Run(change+"/"+map[bool]string{true: "approval_first", false: "authority_first"}[approvalFirst], func(t *testing.T) {
				kind, role := "workspace", "admin"
				if change == "lead_change" {
					kind, role = "project", "member"
				}
				if change == "project_delete" {
					kind = "project"
				}
				f := newKnowledgeApprovalHTTPFixture(t, kind, role)
				approve := func(starter service.TxStarter) *testutil.Response {
					h := *testHandler
					h.TxStarter = starter
					return testutil.Call(t, h.ApproveCollaborationKnowledge, f.request(f.approver, "POST", "/knowledge/approve", f.approval, "candidateId", f.proposal.CandidateID))
				}
				modify := func(starter service.TxStarter) *testutil.Response {
					h := *testHandler
					h.TxStarter = starter
					switch change {
					case "role_downgrade":
						return testutil.Call(t, h.UpdateMember, f.request(testUserID, "PUT", "/members/"+f.member, map[string]any{"role": "member"}, "id", f.workspace, "memberId", f.member))
					case "member_removal":
						return testutil.Call(t, h.DeleteMember, f.request(testUserID, "DELETE", "/members/"+f.member, nil, "id", f.workspace, "memberId", f.member))
					case "lead_change":
						return testutil.Call(t, h.UpdateProject, f.request(testUserID, "PUT", "/projects/"+f.project, map[string]any{"lead_type": "member", "lead_id": testUserID, "expected_revision": 1}, "id", f.project))
					case "project_delete":
						return testutil.Call(t, h.DeleteProject, f.request(testUserID, "DELETE", "/projects/"+f.project, nil, "id", f.project))
					default:
						t.Fatal("unknown matrix writer")
						return nil
					}
				}
				first, second := approve, modify
				if !approvalFirst {
					first, second = modify, approve
				}
				nowait := !approvalFirst && change != "member_removal"
				leader, follower := knowledgeApprovalHTTPPair(t, first, second, nowait)
				modifyStatus := 200
				if change == "member_removal" || change == "project_delete" {
					modifyStatus = 204
				}
				approvalResponse := leader
				if approvalFirst {
					leader.Want(200)
					follower.Want(modifyStatus)
				} else {
					leader.Want(modifyStatus)
					if nowait {
						follower = approve(testPool)
					}
					approvalResponse = follower
					want := 403
					if change == "member_removal" {
						want = 404
					}
					follower.Want(want)
				}
				var state string
				var approved bool
				dbfx.QueryRow(t, `SELECT state,approved_at IS NOT NULL FROM collaboration_knowledge_candidate WHERE id=$1`, f.proposal.CandidateID).Scan(&state, &approved)
				if approvalFirst {
					if state != "approved" || !approved {
						t.Fatal("later authority/target change retroactively cascaded independent approval")
					}
					var receipt protocol.CollaborationKnowledgeReceipt
					approvalResponse.JSON(&receipt)
					if receipt.Code != "knowledge_approved" || receipt.StateRevision != "2" {
						t.Fatal("winning approval lost exact receipt")
					}
				} else if state != "pending" || approved {
					t.Fatal("authority-first approval published inaccessible candidate")
				}
				targetID := f.workspace
				if kind == "project" {
					targetID = f.project
				}
				if change == "project_delete" {
					if dbfx.Count(t, `SELECT count(*) FROM project WHERE id=$1`, f.project) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_id=$1`, f.project) != 0 {
						t.Fatal("deletion left an active parent/counter")
					}
					want := 0
					if approvalFirst {
						want = 2
					}
					if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_id=$1`, targetID) != want {
						t.Fatal("deletion lost final routing or exposed a pending-only target")
					}
				} else {
					want := 0
					if approvalFirst {
						want = 1
					}
					if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_id=$1`, targetID) != want {
						t.Fatal("authority race duplicated/fabricated direct approval hint")
					}
				}
			})
		}
	}
}

func TestKnowledgeApprovalHTTPMatrixWrongCandidateRecoveryIsContentFree(t *testing.T) {
	knowledgeApprovalHTTPGuard(t)
	f := newKnowledgeApprovalHTTPFixture(t, "workspace", "admin")
	other := f.proposal
	other.CandidateID = uuid.NewString()
	other.RequestID = uuid.NewString()
	ownerCtx := auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
	if _, err := service.NewCollaborationService(testHandler.Queries, testPool, testHandler.IssueService).ProposeKnowledge(ownerCtx, parseUUID(f.issue), other); err != nil {
		t.Fatal(err)
	}
	r := testutil.Call(t, testHandler.GetCollaborationKnowledgeOperation, f.request(testUserID, "GET", "/operations/"+f.proposal.RequestID, nil, "candidateId", other.CandidateID, "requestId", f.proposal.RequestID)).Want(404)
	if r.Map()["code"] != "operation_not_found" || strings.Contains(r.Text(), "PRIVATE_") || strings.Contains(r.Text(), f.issue) || strings.Contains(r.Text(), `"candidate_id"`) {
		t.Fatalf("wrong-candidate recovery disclosed receipt/source fields: %s", r.Text())
	}
}

func TestKnowledgeApprovalHTTPMatrixLegacyDeletedSourceKeepsDeletionGuard(t *testing.T) {
	knowledgeApprovalHTTPGuard(t)
	f := newKnowledgeApprovalHTTPFixture(t, "workspace", "admin")
	ownerCtx := auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT})
	svc := service.NewCollaborationService(testHandler.Queries, testPool, testHandler.IssueService)
	if _, err := svc.ApproveKnowledge(ownerCtx, parseUUID(f.proposal.CandidateID), f.approval); err != nil {
		t.Fatal(err)
	}
	second := f.proposal
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.SupersedesCandidateID = &f.proposal.CandidateID
	if _, err := svc.ProposeKnowledge(ownerCtx, parseUUID(f.issue), second); err != nil {
		t.Fatal(err)
	}
	guard := testutil.Call(t, testHandler.DeleteIssue, f.request(testUserID, "DELETE", "/issues/"+f.issue, nil, "id", f.issue)).Want(409)
	if !strings.Contains(guard.Text(), "unsupported_collaboration_action") {
		t.Fatalf("ordinary delete guard changed: %s", guard.Text())
	}
	before := testutil.Call(t, testHandler.GetCollaborationKnowledgeCandidate, f.request(testUserID, "GET", "/knowledge", nil, "candidateId", f.proposal.CandidateID)).Want(200)
	if !strings.Contains(before.Text(), f.proposal.Content) {
		t.Fatal("legacy fixture was not initially source-readable")
	}
	// Explicit unavailable legacy state, not a claimed public deletion flow.
	// Fixed-ID fixture cleanup above remains effective after this row is gone.
	dbfx.Exec(t, `DELETE FROM issue WHERE id=$1`, f.issue)
	list := testutil.Call(t, testHandler.ListCollaborationKnowledgeCandidates, f.request(testUserID, "GET", "/knowledge-candidates", nil, "id", f.issue)).Want(404)
	detail := testutil.Call(t, testHandler.GetCollaborationKnowledgeCandidate, f.request(testUserID, "GET", "/knowledge", nil, "candidateId", f.proposal.CandidateID)).Want(200)
	history := testutil.Call(t, testHandler.GetCollaborationKnowledgeCandidate, f.request(testUserID, "GET", "/knowledge", nil, "candidateId", second.CandidateID)).Want(200)
	target := testutil.Call(t, testHandler.GetWorkspaceKnowledge, f.request(testUserID, "GET", "/knowledge", nil, "id", f.workspace)).Want(200)
	for name, r := range map[string]*testutil.Response{"list": list, "detail": detail, "history": history, "target": target} {
		if strings.Contains(r.Text(), "PRIVATE_") || strings.Contains(r.Text(), `"source_issue_id"`) || strings.Contains(r.Text(), `"content_sha256"`) || strings.Contains(r.Text(), `"summary"`) || strings.Contains(r.Text(), `"sources"`) {
			t.Fatalf("%s leaked deleted-source projection: %s", name, r.Text())
		}
	}
	if detail.Map()["candidate"].(map[string]any)["availability"] != "restricted" || len(history.Map()["history"].([]any)) != 0 {
		t.Fatal("deleted source was presented as readable history")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='approved' AND approved_at IS NOT NULL`, f.proposal.CandidateID) != 1 {
		t.Fatal("legacy source absence erased independent approval")
	}
}
