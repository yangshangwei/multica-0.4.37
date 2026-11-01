package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeHTTPRequest(method, url, actor string, body any) *http.Request {
	r := newRequest(method, url, body)
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func knowledgeWireEvidence(t *testing.T, name string, value any) {
	t.Helper()
	dir := os.Getenv("MULTICA_KNOWLEDGE_WIRE_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func knowledgeHTTPProposal(t *testing.T) (string, protocol.CollaborationKnowledgeProposal) {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	issue := collaborationMemoryIssue(t)
	ctx := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	_, err := testHandler.knowledgeService().UpdateMemory(ctx, service.CollaborationMemoryUpdateParams{IssueID: parseUUID(issue), RequestID: uuid.NewString(), ExpectedVersion: 0, Content: "PRIVATE_HTTP_SOURCE"})
	if err != nil {
		t.Fatal(err)
	}
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: testWorkspaceID}, Summary: "PRIVATE_HTTP_SUMMARY", Content: "PRIVATE_HTTP_BODY", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: issue, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_HTTP_SOURCE")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}}}
	return issue, p
}

func TestKnowledgeHTTPHumanRoundTripAndPurgedProjection(t *testing.T) {
	issue, p := knowledgeHTTPProposal(t)
	r := withURLParam(knowledgeHTTPRequest("POST", "/api/collaboration/issues/"+issue+"/knowledge-candidates", testUserID, p), "id", issue)
	r.Header.Set("X-Workspace-ID", "wrong-ambient-workspace")
	var receipt protocol.CollaborationKnowledgeReceipt
	response := testutil.Call(t, testHandler.ProposeCollaborationKnowledge, r).Want(201)
	response.JSON(&receipt)
	knowledgeWireEvidence(t, "receipt", response.Map())
	if response.Header().Get("Cache-Control") != "no-store" || receipt.CandidateID != p.CandidateID {
		t.Fatal("creation response lost private identity/cache boundary")
	}
	read := func() map[string]any {
		t.Helper()
		r := withURLParam(knowledgeHTTPRequest("GET", "/api/collaboration/knowledge-candidates/"+p.CandidateID, testUserID, nil), "candidateId", p.CandidateID)
		return testutil.Call(t, testHandler.GetCollaborationKnowledgeCandidate, r).Want(200).Map()
	}
	view := read()
	knowledgeWireEvidence(t, "available-detail", view)
	sourceRequest := withURLParam(knowledgeHTTPRequest("GET", "/knowledge-candidates", testUserID, nil), "id", issue)
	knowledgeWireEvidence(t, "source-list", testutil.Call(t, testHandler.ListCollaborationKnowledgeCandidates, sourceRequest).Want(200).Map())
	candidate := view["candidate"].(map[string]any)
	if candidate["content"] != p.Content || candidate["availability"] != "available" || view["history_truncated"] != false {
		t.Fatalf("detail contract: %v", view)
	}
	approve := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	r = withURLParam(knowledgeHTTPRequest("POST", "/approve", testUserID, approve), "candidateId", p.CandidateID)
	testutil.Call(t, testHandler.ApproveCollaborationKnowledge, r).Want(200).JSON(&receipt)
	if receipt.Code != "knowledge_approved" {
		t.Fatalf("approve receipt: %+v", receipt)
	}
	availableTargetRequest := withURLParam(knowledgeHTTPRequest("GET", "/knowledge", testUserID, nil), "id", testWorkspaceID)
	availableTargetRequest.Header.Set("X-Workspace-ID", testWorkspaceID)
	knowledgeWireEvidence(t, "available-target", testutil.Call(t, testHandler.GetWorkspaceKnowledge, availableTargetRequest).Want(200).Map())
	dbfx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=now(),purged_by_user_id=$2,purge_reason='remove source' WHERE issue_id=$1`, issue, testUserID)
	view = read()
	knowledgeWireEvidence(t, "unavailable-detail", view)
	raw, _ := json.Marshal(view)
	if strings.Contains(string(raw), "PRIVATE_HTTP") || view["candidate"].(map[string]any)["availability"] != "source_unavailable" {
		t.Fatalf("purged body escaped human detail: %s", raw)
	}
	r = withURLParam(knowledgeHTTPRequest("GET", "/api/workspaces/"+testWorkspaceID+"/knowledge", testUserID, nil), "id", testWorkspaceID)
	r.Header.Set("X-Workspace-ID", testWorkspaceID)
	target := testutil.Call(t, testHandler.GetWorkspaceKnowledge, r).Want(200)
	knowledgeWireEvidence(t, "unavailable-target", target.Map())
	if strings.Contains(target.Body.String(), "PRIVATE_HTTP") || target.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("target projection exposed purged text")
	}
}

func TestKnowledgeHTTPStrictErrorsAndOriginalRecovery(t *testing.T) {
	issue, p := knowledgeHTTPProposal(t)
	raw, _ := json.Marshal(p)
	for name, body := range map[string][]byte{"unknown": bytes.Replace(raw, []byte(`"protocol_version":1`), []byte(`"protocol_version":1,"unexpected":"private"`), 1), "duplicate": bytes.Replace(raw, []byte(`"protocol_version":1`), []byte(`"protocol_version":1,"protocol_version":1`), 1), "alias": bytes.Replace(raw, []byte(`"candidate_id"`), []byte(`"Candidate_ID"`), 1)} {
		t.Run(name, func(t *testing.T) {
			r := withURLParam(knowledgeHTTPRequest("POST", "/knowledge", testUserID, json.RawMessage(body)), "id", issue)
			testutil.Call(t, testHandler.ProposeCollaborationKnowledge, r).Want(400)
		})
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, issue, p.RequestID) != 0 {
		t.Fatal("invalid HTTP input persisted a proposal")
	}
	r := withURLParam(knowledgeHTTPRequest("POST", "/knowledge", testUserID, p), "id", issue)
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, r).Want(201)
	r = withURLParam(knowledgeHTTPRequest("POST", "/approve", testUserID, map[string]any{"protocol_version": 1, "request_id": uuid.NewString()}), "candidateId", p.CandidateID)
	testutil.Call(t, testHandler.ApproveCollaborationKnowledge, r).Want(428)
	r = testutil.WithURLParams(knowledgeHTTPRequest("GET", "/operations/"+p.RequestID, testUserID, nil), "candidateId", p.CandidateID, "requestId", p.RequestID)
	var receipt protocol.CollaborationKnowledgeReceipt
	testutil.Call(t, testHandler.GetCollaborationKnowledgeOperation, r).Want(200).JSON(&receipt)
	if receipt.Code != "knowledge_proposed" || receipt.RequestID != p.RequestID {
		t.Fatal("original-key recovery mismatch")
	}
	r = testutil.WithURLParams(knowledgeHTTPRequest("GET", "/operations/"+p.RequestID, testUserID, nil), "candidateId", p.CandidateID, "requestId", p.RequestID)
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, AgentActor: true}))
	testutil.Call(t, testHandler.GetCollaborationKnowledgeOperation, r).Want(403)
}
