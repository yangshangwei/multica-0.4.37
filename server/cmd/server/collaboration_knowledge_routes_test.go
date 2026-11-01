package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeProductionRoutesPreserveAccountAndTargetScopes(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	email := "knowledge-router-" + uuid.NewString() + "@example.test"
	external := fx.User(t, "External knowledge proposer", email)
	token, err := generateTestJWT(external, email, "External knowledge proposer")
	if err != nil {
		t.Fatal(err)
	}
	issue := fx.Issue(t, "Knowledge route source", testutil.Cols{"assignee_type": "member", "assignee_id": external})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, user := range []string{testUserID, external} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": user}, "issue_id=$1 AND user_id=$2", issue, user)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	call := func(token, method, path string, body any, workspace string) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, path, body)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Workspace-ID", workspace)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
	}
	memory := testutil.JSONRequest("PUT", "/api/collaboration/issues/"+issue+"/memory", map[string]any{"expected_version": "0", "content": "PRIVATE_ROUTE_KNOWLEDGE_SOURCE"})
	memory.Header.Set("Authorization", "Bearer "+token)
	memory.Header.Set("Idempotency-Key", uuid.NewString())
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, memory).Want(200)
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: testWorkspaceID}, Summary: "PRIVATE_ROUTE_KNOWLEDGE_SUMMARY", Content: "PRIVATE_ROUTE_KNOWLEDGE_BODY", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: issue, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_ROUTE_KNOWLEDGE_SOURCE")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}}}
	collection := "/api/collaboration/issues/" + issue + "/knowledge-candidates"
	candidate := "/api/collaboration/knowledge-candidates/" + p.CandidateID
	created := call(token, "POST", collection, p, "invalid-ambient-workspace").Want(201)
	if created.Map()["code"] != "knowledge_proposed" || created.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("proposal route missing original/private contract")
	}
	if call(token, "GET", collection, nil, "invalid-ambient-workspace").Want(200).Map()["issue_id"] != issue {
		t.Fatal("account list inherited workspace context")
	}
	detail := call(token, "GET", candidate, nil, "invalid-ambient-workspace").Want(200).Map()
	if detail["history_truncated"] != false || detail["candidate"].(map[string]any)["content"] != p.Content {
		t.Fatal("candidate route did not use strict detail shape")
	}
	request := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	call(token, "POST", candidate+"/approve", request, "").Want(403)
	call(testToken, "POST", candidate+"/approve", request, "").Want(200)
	if call(testToken, "GET", "/api/workspaces/"+testWorkspaceID+"/knowledge", nil, testWorkspaceID).Want(200).Map()["protocol_version"] != float64(1) {
		t.Fatal("workspace target route absent")
	}
	project := fx.Project(t, "Knowledge project route")
	call(testToken, "GET", "/api/projects/"+project+"/knowledge", nil, testWorkspaceID).Want(200)
	call(token, "GET", candidate+"/operations/"+p.RequestID, nil, "").Want(200)
	call(token, "POST", candidate+"/withdraw", protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "source manager withdrawal"}, "").Want(200)
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	call(token, "POST", collection, second, "").Want(201)
	call(testToken, "POST", "/api/collaboration/knowledge-candidates/"+second.CandidateID+"/reject", protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "not reusable"}, "").Want(200)
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, external) != 0 {
		t.Fatal("account routes invented source membership")
	}
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	patID := fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": external, "name": "Knowledge original credential", "token_hash": auth.HashToken(pat), "token_prefix": "mul_test", "expires_at": time.Now().Add(time.Hour)})
	read := call(pat, "GET", candidate, nil, "").Want(200)
	if strings.Contains(read.Text(), auth.HashToken(pat)) {
		t.Fatal("PAT provenance entered response")
	}
	fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID)
	response := call(pat, "GET", candidate, nil, "")
	if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
		t.Fatalf("revoked credential reached knowledge route: %d", response.Code)
	}
}
