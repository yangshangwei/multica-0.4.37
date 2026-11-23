package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeHTTPVersionNegotiationAndOriginalReceipts(t *testing.T) {
	issue, proposal := knowledgeHTTPProposal(t)
	proposal.ProtocolVersion = 2
	post := func(header string) *http.Request {
		r := withURLParam(knowledgeHTTPRequest("POST", "/knowledge-candidates", testUserID, proposal), "id", issue)
		if header != "" {
			r.Header.Set(HeaderKnowledgeVersion, header)
		}
		return r
	}
	// Omitting the header is v1, even when the body asks for v2.
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, post("")).Want(400)
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, post("1")).Want(400)
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, post("3")).Want(409)
	duplicate := post("2")
	duplicate.Header.Add(HeaderKnowledgeVersion, "1")
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, duplicate).Want(400)
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, proposal.CandidateID) != 0 {
		t.Fatal("rejected protocol negotiation wrote a candidate")
	}
	var receipt protocol.CollaborationKnowledgeReceipt
	testutil.Call(t, testHandler.ProposeCollaborationKnowledge, post("2")).Want(201).JSON(&receipt)
	if receipt.ProtocolVersion != 2 || receipt.RequestID != proposal.RequestID {
		t.Fatalf("v2 mutation lost its request version: %+v", receipt)
	}
	read := testutil.WithURLParams(knowledgeHTTPRequest("GET", "/knowledge-candidates/"+proposal.CandidateID, testUserID, nil), "candidateId", proposal.CandidateID)
	read.Header.Set(HeaderKnowledgeVersion, "2")
	response := testutil.Call(t, testHandler.GetCollaborationKnowledgeCandidate, read).Want(200)
	if response.Header().Get(HeaderKnowledgeVersion) != "2" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("v2 response lost negotiation or private cache headers")
	}
	var detail struct {
		ProtocolVersion int `json:"protocol_version"`
		Candidate       struct {
			ProposalOrigin string `json:"proposal_origin"`
			UsageStatus    string `json:"usage_status"`
		} `json:"candidate"`
	}
	response.JSON(&detail)
	if detail.ProtocolVersion != 2 || detail.Candidate.ProposalOrigin != "human" || detail.Candidate.UsageStatus != "inactive" {
		t.Fatalf("v2 human projection mismatch: %+v", detail)
	}
	recoverRequest := func(version string) *http.Request {
		r := testutil.WithURLParams(knowledgeHTTPRequest("GET", "/operations/"+proposal.RequestID, testUserID, nil), "candidateId", proposal.CandidateID, "requestId", proposal.RequestID)
		r.Header.Set(HeaderKnowledgeVersion, version)
		return r
	}
	testutil.Call(t, testHandler.GetCollaborationKnowledgeOperation, recoverRequest("1")).Want(409)
	var recovered protocol.CollaborationKnowledgeReceipt
	testutil.Call(t, testHandler.GetCollaborationKnowledgeOperation, recoverRequest("2")).Want(200).JSON(&recovered)
	if recovered.ProtocolVersion != 2 || recovered.RequestID != receipt.RequestID || recovered.CandidateID != receipt.CandidateID {
		t.Fatal("original v2 receipt was relabeled during recovery")
	}
}

func TestKnowledgePolicyHTTPRequiresExplicitToggleAndRecoversAfterRoleLoss(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	workspace := dbfx.Workspace(t, "Knowledge policy HTTP", "knowledge-policy-"+uuid.NewString())
	dbfx.Member(t, workspace, testUserID, "owner")
	fixture := testutil.New(testPool, workspace, testUserID)
	project := fixture.Project(t, "Policy contract", nil)
	fixture.Cleanup(t, `DELETE FROM project_knowledge_policy WHERE project_id=$1`, project)
	request := func(method string, body any) *http.Request {
		r := testutil.WithURLParams(knowledgeHTTPRequest(method, "/api/projects/"+project+"/knowledge-policy", testUserID, body), "id", project)
		r.Header.Set("X-Workspace-ID", workspace)
		r.Header.Set(HeaderKnowledgeVersion, "2")
		return r
	}
	var policy service.ProjectKnowledgePolicyView
	testutil.Call(t, testHandler.GetProjectKnowledgePolicy, request("GET", nil)).Want(200).JSON(&policy)
	if !policy.IsEnabled || policy.Origin != "system" || policy.Revision != "0" || policy.ChangedByUserID != nil {
		t.Fatalf("default policy invented an owner or override: %+v", policy)
	}
	id := uuid.NewString()
	for name, body := range map[string]map[string]any{
		"missing toggle": {"request_id": id, "expected_revision": "0", "max_candidates": 2},
		"null toggle":    {"request_id": id, "expected_revision": "0", "max_candidates": 2, "is_enabled": nil},
		"missing limit":  {"request_id": id, "expected_revision": "0", "is_enabled": false},
		"system actor":   {"request_id": id, "expected_revision": "0", "is_enabled": false, "max_candidates": 2, "origin": "system"},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Call(t, testHandler.UpdateProjectKnowledgePolicy, request("PUT", body)).Want(400)
		})
	}
	if dbfx.Count(t, `SELECT count(*) FROM project_knowledge_policy WHERE project_id=$1`, project) != 0 {
		t.Fatal("missing/forged configuration fields disabled the default policy")
	}
	body := service.ProjectKnowledgePolicyRequest{RequestID: id, ExpectedRevision: "0", IsEnabled: false, MaxCandidates: 2}
	var receipt service.ProjectKnowledgePolicyReceipt
	testutil.Call(t, testHandler.UpdateProjectKnowledgePolicy, request("PUT", body)).Want(200).JSON(&receipt)
	if receipt.Revision != "1" || receipt.RequestID != id {
		t.Fatalf("policy receipt: %+v", receipt)
	}
	stale := body
	stale.RequestID = uuid.NewString()
	var refused service.ProjectKnowledgePolicyReceipt
	testutil.Call(t, testHandler.UpdateProjectKnowledgePolicy, request("PUT", stale)).Want(409).JSON(&refused)
	if refused.ProtocolVersion != 2 || refused.RequestID != stale.RequestID || refused.ProjectID != project || refused.Code != "knowledge_policy_revision_conflict" || refused.Revision != "1" {
		t.Fatalf("sealed stale policy refusal lost its original receipt: %+v", refused)
	}
	dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, workspace, testUserID)
	testutil.Call(t, testHandler.GetProjectKnowledgePolicy, request("GET", nil)).Want(403)
	recovery := testutil.WithURLParams(knowledgeHTTPRequest("GET", "/knowledge-policy/operations/"+id, testUserID, nil), "id", project, "requestId", id)
	recovery.Header.Set(HeaderKnowledgeVersion, "2")
	var recovered service.ProjectKnowledgePolicyReceipt
	testutil.Call(t, testHandler.GetProjectKnowledgePolicyOperation, recovery).Want(200).JSON(&recovered)
	if recovered != receipt {
		t.Fatal("loss of current project role changed the original policy receipt")
	}
}

func TestKnowledgeJobHTTPPreservesSealedRefusal(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		name := "stale revision"
		if disabled {
			name = "disabled policy"
		}
		t.Run(name, func(t *testing.T) {
			issue, _ := knowledgeHTTPProposal(t)
			fixture := testutil.New(testPool, testWorkspaceID, testUserID)
			project := fixture.Project(t, "Retry receipt", nil)
			fixture.Cleanup(t, `DELETE FROM project_knowledge_policy WHERE project_id=$1`, project)
			fixture.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, issue, project)
			ctx := context.Background()
			state, err := testHandler.Queries.GetKnowledgeSourceState(ctx, parseUUID(issue))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = testHandler.Queries.CreateProjectKnowledgePolicy(ctx, db.CreateProjectKnowledgePolicyParams{ProjectID: parseUUID(project), WorkspaceID: parseUUID(testWorkspaceID), IsEnabled: !disabled, MaxCandidates: 3, BuiltinRuleVersion: service.ProjectKnowledgeRuleVersion, ChangedByUserID: parseUUID(testUserID)}); err != nil {
				t.Fatal(err)
			}
			job, err := testHandler.Queries.CreateKnowledgeAutomationJob(ctx, db.CreateKnowledgeAutomationJobParams{ID: parseUUID(uuid.NewString()), SourceWorkspaceID: parseUUID(testWorkspaceID), SourceIssueID: parseUUID(issue), ProjectID: parseUUID(project), SubmissionID: parseUUID(uuid.NewString()), DeliveryEventSeq: 1, AcceptedEventSeq: 2, ValidatedCloseEpoch: state.LifecycleEpoch, SourceReaderUserID: parseUUID(testUserID), PolicyRevision: 1, BuiltinRuleVersion: service.ProjectKnowledgeRuleVersion, MaxCandidates: 3})
			if err != nil {
				t.Fatal(err)
			}
			fixture.Exec(t, `UPDATE collaboration_knowledge_job SET phase='review_pending' WHERE id=$1`, job.ID)
			request := service.KnowledgeAutomationRetryRequest{RequestID: uuid.NewString(), ExpectedRevision: "2"}
			want := "knowledge_job_revision_conflict"
			if disabled {
				request.ExpectedRevision, want = "1", "knowledge_automation_disabled"
			}
			r := testutil.WithURLParams(knowledgeHTTPRequest("POST", "/knowledge-jobs/retry", testUserID, request), "id", issue, "jobId", uuidToString(job.ID))
			r.Header.Set(HeaderKnowledgeVersion, "2")
			var refusal service.KnowledgeAutomationRetryReceipt
			testutil.Call(t, testHandler.RetryCollaborationKnowledgeJob, r).Want(409).JSON(&refusal)
			if refusal.ProtocolVersion != 2 || refusal.Code != want || refusal.RequestID != request.RequestID || refusal.IssueID != issue || refusal.JobID != uuidToString(job.ID) || refusal.Revision != "1" {
				t.Fatalf("sealed retry refusal was presented as an unknown outcome: %+v", refusal)
			}
			recovery := testutil.WithURLParams(knowledgeHTTPRequest("GET", "/knowledge-jobs/operations", testUserID, nil), "id", issue, "jobId", uuidToString(job.ID), "requestId", request.RequestID)
			recovery.Header.Set(HeaderKnowledgeVersion, "2")
			var recovered service.KnowledgeAutomationRetryReceipt
			testutil.Call(t, testHandler.GetCollaborationKnowledgeJobOperation, recovery).Want(200).JSON(&recovered)
			if recovered != refusal || fixture.Count(t, `SELECT count(*) FROM collaboration_knowledge_job WHERE id=$1 AND phase='review_pending' AND execution_revision=1`, job.ID) != 1 {
				t.Fatal("refused retry changed job state or recovered a different result")
			}
		})
	}
}
