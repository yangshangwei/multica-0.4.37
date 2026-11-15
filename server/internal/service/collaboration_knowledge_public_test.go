package service

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgePublicV2PreservesHumanReceiptsAndAudit(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	p.ProtocolVersion = 2
	receipt, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p)
	if err != nil || receipt.ProtocolVersion != 2 {
		t.Fatalf("v2 proposal receipt: %+v %v", receipt, err)
	}
	ctx := WithKnowledgeProtocolVersion(f.ctx, 2)
	view, err := f.svc.KnowledgeCandidate(ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || view.ProtocolVersion != 2 {
		t.Fatalf("v2 detail: %d %v", view.ProtocolVersion, err)
	}
	raw, err := json.Marshal(view.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"proposal_origin": `"human"`, "decision_origin": `"human"`, "automation": "null", "usage_status": `"inactive"`} {
		if string(fields[key]) != want {
			t.Fatalf("v2 %s = %s, want %s", key, fields[key], want)
		}
	}
	recovered, err := f.svc.GetKnowledgeOperation(f.ctx, util.MustParseUUID(p.CandidateID), p.RequestID)
	if err != nil || recovered != receipt {
		t.Fatalf("original v2 receipt changed: %+v %v", recovered, err)
	}
	legacy, err := f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || legacy.ProtocolVersion != 1 {
		t.Fatalf("legacy-expressible human detail: %d %v", legacy.ProtocolVersion, err)
	}
	raw, _ = json.Marshal(legacy.Candidate)
	fields = nil
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, added := fields["proposal_origin"]; added {
		t.Fatal("v1 projection gained an unsupported origin field")
	}
}

func TestKnowledgePublicCursorCannotChangeProtocolVersion(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	for range 2 {
		p.CandidateID, p.RequestID = uuid.NewString(), uuid.NewString()
		if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{Limit: 1})
	if err != nil || page.NextCursor == nil {
		t.Fatalf("v1 cursor: %+v %v", page, err)
	}
	if _, err = f.svc.ListKnowledgeCandidates(WithKnowledgeProtocolVersion(f.ctx, 2), f.issue.Issue.ID, KnowledgePageQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "invalid_knowledge_request") {
		t.Fatalf("v1 cursor admitted under v2: %v", err)
	}
}

func TestKnowledgePublicV1CannotHideAutomationBeyondPage(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	project := f.fx.Project(t, "Knowledge projection project", nil)
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	p.Target = protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.workspace, ProjectID: project}
	original, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	sources, _ := json.Marshal(p.Sources)
	applicability, _ := json.Marshal(p.Applicability)
	id := uuid.NewString()
	f.fx.Insert(t, "collaboration_knowledge_candidate", testutil.Cols{
		"id": id, "lineage_id": id, "version": 1,
		"source_workspace_id": f.workspace, "source_issue_id": f.issue.Issue.ID,
		"target_kind": "project", "target_workspace_id": f.workspace, "target_project_id": project,
		"content": "Automated projection fixture", "summary": "Automated projection fixture",
		"content_sha256": protocol.CollaborationKnowledgeHash("Automated projection fixture"),
		"sources":        sources, "evidence": `[]`, "applicability": applicability,
		"proposal_origin": "automation", "automation_job_id": uuid.NewString(),
		"automation_policy_revision": 0, "automation_rule_version": "project-knowledge-v1",
	})
	if _, err = f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{Limit: 1}); !collaborationErrorCode(err, "knowledge_protocol_unsupported") {
		t.Fatalf("v1 source page silently hid system candidate: %v", err)
	}
	if _, err = f.svc.ListTargetKnowledge(f.ctx, p.Target, KnowledgePageQuery{Limit: 1}); !collaborationErrorCode(err, "knowledge_protocol_unsupported") {
		t.Fatalf("v1 target page silently hid system candidate: %v", err)
	}
	ctx := WithKnowledgeProtocolVersion(f.ctx, 2)
	view, err := f.svc.KnowledgeCandidate(ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(view.Candidate)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ProposedByUserID *string `json:"proposed_by_user_id"`
		ProposalOrigin   string  `json:"proposal_origin"`
		UsageStatus      string  `json:"usage_status"`
	}
	if err = json.Unmarshal(raw, &audit); err != nil || audit.ProposedByUserID != nil || audit.ProposalOrigin != "automation" || audit.UsageStatus != "inactive" {
		t.Fatalf("system proposal impersonated a human or became effective: %+v %v", audit, err)
	}
	recovered, err := f.svc.GetKnowledgeOperation(ctx, util.MustParseUUID(p.CandidateID), p.RequestID)
	if err != nil || recovered != original {
		t.Fatalf("new source shapes changed original v1 receipt: %+v %v", recovered, err)
	}
}
