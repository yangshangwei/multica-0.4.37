package handler

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type preparationMatrixGate struct {
	base    service.TxStarter
	entered chan uint32
	release chan struct{}
	once    sync.Once
}

func TestKnowledgeOrdinaryCoalescedCommentsCheckEveryCurrentAuthor(t *testing.T) {
	for _, state := range []string{"eligible", "older_author_disabled"} {
		t.Run(state, func(t *testing.T) {
			f := newTaskKnowledgeHTTPFixture(t, "comment")
			other := preparationMatrixActor(t)
			dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to',visibility='workspace' WHERE id=$1`, f.agentID)
			dbfx.InsertNoID(t, "agent_invocation_target", testutil.Cols{"agent_id": f.agentID, "target_type": "workspace", "target_id": testWorkspaceID}, "agent_id=$1", f.agentID)
			comment := func(actor, text string) string {
				t.Helper()
				r := withURLParam(preparationMatrixRequest(http.MethodPost, "/api/issues/"+f.issueID+"/comments", actor, map[string]any{"content": text + " [@Agent](mention://agent/" + f.agentID + ")"}), "id", f.issueID)
				var out CommentResponse
				testutil.Call(t, testHandler.CreateComment, r).Want(201).JSON(&out)
				return out.ID
			}
			older := comment(other, "Older deliberate coalesced input")
			_ = comment(testUserID, "Latest deliberate coalesced input")
			req := withURLParam(testutil.JSONRequest(http.MethodPost, "/api/daemon/runtimes/"+f.runtimeID+"/tasks/claim", map[string]any{"knowledge_context": f.inv}), "runtimeId", f.runtimeID)
			req.Header.Set("Authorization", "Bearer "+f.token)
			req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityCoalescedCommentsV1)
			wrapped := middleware.DaemonAuth(testHandler.Queries, nil, nil, nil)(http.HandlerFunc(testHandler.ClaimTaskByRuntime))
			var claimed struct {
				Task AgentTaskResponse `json:"task"`
			}
			response := testutil.Call(t, wrapped.ServeHTTP, req).Want(200)
			response.JSON(&claimed)
			if !strings.Contains(response.Text(), "Older deliberate coalesced input") || !strings.Contains(response.Text(), "Latest deliberate coalesced input") {
				t.Fatal("claimed batch did not deliver both real coalesced inputs")
			}
			row, err := testHandler.Queries.GetAgentTask(t.Context(), parseUUID(f.taskID))
			if err != nil {
				t.Fatal(err)
			}
			if uuidToString(row.OriginatorUserID) == other {
				t.Fatal("fixture must revoke an additional author, not the trusted originator")
			}
			found := false
			for _, id := range row.CoalescedCommentIds {
				found = found || uuidToString(id) == older
			}
			if !found {
				t.Fatal("additional author was not part of retained coalesced input")
			}
			if state == "older_author_disabled" {
				dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, other)
			}
			d := claimed.Task.KnowledgeContext
			if d == nil {
				t.Fatal("coalesced claim did not finalize knowledge negotiation")
			}
			p := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)}
			prepared := f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", p, "taskId", f.taskID)
			if state == "older_author_disabled" {
				prepared.Want(403)
				if strings.Contains(prepared.Text(), f.proposal.Content) {
					t.Fatal("private body escaped after a coalesced author's authority loss")
				}
			} else {
				prepared.Want(200)
				var out protocol.TaskKnowledgePrepareResponse
				prepared.JSON(&out)
				if len(out.Contributions) != 1 || out.Contributions[0].CandidateID != f.proposal.CandidateID {
					t.Fatal("eligible coalesced author/audience did not receive current knowledge")
				}
			}
		})
	}
}

func TestKnowledgeHeldOwnerApprovalAllowsActualUnrelatedWriters(t *testing.T) {
	for _, operation := range []string{"issue_create", "project_association", "triage_accept", "iteration_create"} {
		t.Run(operation, func(t *testing.T) {
			source, p := knowledgeHTTPProposal(t)
			project := dbfx.Project(t, "Knowledge approval target")
			p.Target = protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: testWorkspaceID, ProjectID: project}
			actor := preparationMatrixActor(t)
			issue := dbfx.Issue(t, "Unrelated ordinary issue", testutil.Cols{"creator_id": actor})
			dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT max(number) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
			human := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
			if _, err := testHandler.knowledgeService().ProposeKnowledge(human, parseUUID(source), p); err != nil {
				t.Fatal(err)
			}
			var triage TriageItem
			ordinary := testHandler
			if operation == "triage_accept" {
				triageEnableForTest(t)
				input := triageInputForTest()
				input["candidate_project_id"] = project
				testutil.Call(t, testHandler.CreateTriageItem, preparationMatrixRequest(http.MethodPost, "/triage/items", actor, input)).Want(201).JSON(&triage)
			}
			if operation == "iteration_create" {
				ordinary = lifecycleHTTPHandler(t)
			}
			gate := newPreparationMatrixGate(t)
			approval := service.NewCollaborationService(testHandler.Queries, gate, testHandler.IssueService)
			ctx, cancel := context.WithTimeout(human, 8*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := approval.ApproveKnowledge(ctx, parseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)})
				done <- err
			}()
			select {
			case <-gate.entered:
			case err := <-done:
				t.Fatalf("approval failed before commit: %v", err)
			case <-ctx.Done():
				t.Fatal("approval missed gate")
			}
			// The distinct actor and issue make this a reachable independence
			// schedule, rather than a test blocked on the approval author's row.
			var call http.HandlerFunc
			var request *http.Request
			want := http.StatusOK
			switch operation {
			case "issue_create":
				request = preparationMatrixRequest(http.MethodPost, "/issues", actor, map[string]any{"title": "Created while approval is held", "status": "todo", "project_id": project})
				call = ordinary.CreateIssue
				want = 201
			case "project_association":
				request = withURLParam(preparationMatrixRequest(http.MethodPut, "/issues/"+issue, actor, map[string]any{"project_id": project}), "id", issue)
				call = ordinary.UpdateIssue
			case "triage_accept":
				request = withURLParam(preparationMatrixRequest(http.MethodPost, "/triage/items/actions", actor, map[string]any{"request_id": uuid.NewString(), "expected_revision": triage.Issue.Revision, "action": "accept", "fields": map[string]any{"project_id": project}}), "id", triage.Issue.ID)
				call = ordinary.ActOnTriageItem
			case "iteration_create":
				now := time.Now().UTC()
				request = withURLParam(preparationMatrixRequest(http.MethodPost, "/workspaces/"+testWorkspaceID+"/iterations", actor, service.CreateIterationInput{RequestID: uuid.NewString(), Name: "Independent period", StartDate: now.Format(time.DateOnly), EndDate: now.AddDate(0, 0, 13).Format(time.DateOnly), ConfirmedTimezone: "UTC"}), "id", testWorkspaceID)
				call = ordinary.CreateIteration
				want = 201
			}
			writerCtx, stopWriter := context.WithTimeout(request.Context(), 2*time.Second)
			defer stopWriter()
			response := testutil.Call(t, call, request.WithContext(writerCtx))
			close(gate.release)
			if err := <-done; err != nil {
				t.Fatalf("approval failed: %v", err)
			}
			response.Want(want)
			switch operation {
			case "issue_create":
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Created while approval is held' AND project_id=$2`, testWorkspaceID, project) != 1 {
					t.Fatal("actual issue create did not persist")
				}
			case "project_association":
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id=$2`, issue, project) != 1 {
					t.Fatal("actual association did not persist")
				}
			case "triage_accept":
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND admission_status='accepted' AND project_id=$2`, triage.Issue.ID, project) != 1 {
					t.Fatal("actual triage acceptance did not persist")
				}
			case "iteration_create":
				var result iteration.WriteResult
				response.JSON(&result)
				if len(result.IterationIDs) != 1 {
					t.Fatal("actual iteration writer did not persist a period")
				}
			}
		})
	}
}

type preparationMatrixTx struct {
	pgx.Tx
	gate *preparationMatrixGate
}

func (g *preparationMatrixGate) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := g.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &preparationMatrixTx{Tx: tx, gate: g}, nil
}
func (tx *preparationMatrixTx) Commit(ctx context.Context) error {
	tx.gate.once.Do(func() {
		tx.gate.entered <- tx.Conn().PgConn().PID()
		select {
		case <-tx.gate.release:
		case <-ctx.Done():
		}
	})
	return tx.Tx.Commit(ctx)
}
func newPreparationMatrixGate(t *testing.T) *preparationMatrixGate {
	t.Helper()
	g := &preparationMatrixGate{base: testPool, entered: make(chan uint32, 1), release: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-g.release:
		default:
			close(g.release)
		}
	})
	return g
}
func preparationMatrixActor(t *testing.T) string {
	t.Helper()
	id := dbfx.User(t, "Independent ordinary writer", "prep-matrix-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, id, "admin")
	return id
}
func preparationMatrixRequest(method, path, actor string, body any) *http.Request {
	r := newRequest(method, path, body)
	r.Header.Set("X-User-ID", actor)
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func TestKnowledgePreparationWhileRealOrdinaryWriterIsUncommitted(t *testing.T) {
	for _, writer := range []string{"issue_number_allocation", "project_description"} {
		t.Run(writer, func(t *testing.T) {
			f := newTaskKnowledgeHTTPFixture(t, "issue")
			actor := preparationMatrixActor(t)
			project := dbfx.Project(t, "Preparation independence")
			dbfx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issueID, project)
			dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT max(number) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
			d := f.claim(t, "single", &f.inv).KnowledgeContext
			gate := newPreparationMatrixGate(t)
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			done := make(chan error, 1)
			allocated := make(chan int32, 1)
			if writer == "issue_number_allocation" {
				svc := service.NewIssueService(testHandler.Queries, gate, testHandler.Bus, analytics.NoopClient{}, nil)
				go func() {
					_, err := svc.Create(ctx, service.IssueCreateParams{WorkspaceID: parseUUID(testWorkspaceID), Title: "Uncommitted numbered issue", Status: "todo", Priority: "none", CreatorType: "member", CreatorID: parseUUID(actor)}, service.IssueCreateOpts{AfterCreateInTx: func(_ context.Context, _ pgx.Tx, result *service.IssueCreateResult) error {
						allocated <- result.Issue.Number
						return nil
					}})
					done <- err
				}()
			} else {
				h := *testHandler
				h.TxStarter = gate
				var revision int64
				dbfx.QueryRow(t, `SELECT description_revision FROM project WHERE id=$1`, project).Scan(&revision)
				r := withURLParam(preparationMatrixRequest(http.MethodPut, "/projects/"+project, actor, map[string]any{"description": "ordinary uncommitted description", "expected_description_revision": revision}), "id", project)
				writerCtx, cancelWriter := context.WithTimeout(r.Context(), 8*time.Second)
				defer cancelWriter()
				r = r.WithContext(writerCtx)
				go func() {
					response := testutil.Call(t, h.UpdateProject, r)
					if response.Code != 200 {
						done <- &service.CollaborationError{Status: response.Code, Code: response.Body.String()}
						return
					}
					done <- nil
				}()
			}
			select {
			case <-gate.entered:
			case err := <-done:
				t.Fatalf("writer failed before commit barrier: %v", err)
			case <-ctx.Done():
				t.Fatal("writer missed commit barrier")
			}
			if writer == "issue_number_allocation" {
				select {
				case number := <-allocated:
					if number < 1 {
						t.Fatal("real create did not allocate a number")
					}
				default:
					t.Fatal("commit reached without number allocation")
				}
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Uncommitted numbered issue'`, testWorkspaceID) != 0 {
					t.Fatal("held issue create was already visible")
				}
			} else {
				var description pgtype.Text
				dbfx.QueryRow(t, `SELECT description FROM project WHERE id=$1`, project).Scan(&description)
				if description.String == "ordinary uncommitted description" {
					t.Fatal("held description writer already committed")
				}
			}
			req := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			response := f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", req, "taskId", f.taskID)
			close(gate.release)
			if err := <-done; err != nil {
				t.Fatalf("ordinary writer failed: %v", err)
			}
			response.Want(http.StatusOK)
			var prepared protocol.TaskKnowledgePrepareResponse
			response.JSON(&prepared)
			if len(prepared.Contributions) != 1 || prepared.Contributions[0].CandidateID != f.proposal.CandidateID {
				t.Fatal("independent ordinary write prevented eligible preparation")
			}
		})
	}
}

func TestKnowledgePreparationReplayStillFencesActualProjectDeletion(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	project := dbfx.Project(t, "Actual deletion remains fenced")
	dbfx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issueID, project)
	human := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	if _, err := testHandler.knowledgeService().WithdrawKnowledge(human, parseUUID(f.proposal.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "use project-only fixture"}); err != nil {
		t.Fatal(err)
	}
	p := f.proposal
	p.CandidateID = uuid.NewString()
	p.RequestID = uuid.NewString()
	p.Target = protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: testWorkspaceID, ProjectID: project}
	p.Content = "PROJECT_DELETE_PRIVATE_CONTRIBUTION"
	if _, err := testHandler.knowledgeService().ProposeKnowledge(human, parseUUID(p.Sources[0].IssueID), p); err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.knowledgeService().ApproveKnowledge(human, parseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
		t.Fatal(err)
	}
	d := f.claim(t, "single", &f.inv).KnowledgeContext
	req := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)}
	var initial protocol.TaskKnowledgePrepareResponse
	f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", req, "taskId", f.taskID).Want(200).JSON(&initial)
	if len(initial.Contributions) != 1 || initial.Contributions[0].Content != p.Content {
		t.Fatal("project contribution was not initially disclosed")
	}
	gate := newPreparationMatrixGate(t)
	h := *testHandler
	h.TxStarter = gate
	actor := preparationMatrixActor(t)
	request := withURLParam(preparationMatrixRequest(http.MethodDelete, "/projects/"+project, actor, nil), "id", project)
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	request = request.WithContext(ctx)
	done := make(chan *testutil.Response, 1)
	go func() { done <- testutil.Call(t, h.DeleteProject, request) }()
	select {
	case <-gate.entered:
	case result := <-done:
		t.Fatalf("delete missed barrier: %d %s", result.Code, result.Text())
	case <-ctx.Done():
		t.Fatal("delete missed commit")
	}
	blocked := f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", req, "taskId", f.taskID)
	close(gate.release)
	(<-done).Want(204)
	blocked.Want(503)
	if strings.Contains(blocked.Text(), p.Content) {
		t.Fatal("held deletion exposed original private body")
	}
	after := f.call(t, testHandler.PrepareTaskKnowledge, "/prepare", req, "taskId", f.taskID).Want(409)
	if strings.Contains(after.Text(), p.Content) || !strings.Contains(after.Text(), "knowledge_context_changed") {
		t.Fatal("deleted project restored original manifest disclosure")
	}
}
