package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeServiceProposal(t *testing.T, f *collaborationQueueFixture) protocol.CollaborationKnowledgeProposal {
	t.Helper()
	f.memoryUpdate(t, f.creator, 0, "PRIVATE_KNOWLEDGE_SOURCE")
	return protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: f.workspace}, Summary: "PRIVATE_KNOWLEDGE_SUMMARY", Content: "PRIVATE_KNOWLEDGE_BODY", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(f.issue.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_KNOWLEDGE_SOURCE")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat", "quick_create", "autopilot", "collaboration"}}}
}

func TestKnowledgeExternalHandlerCanProposeToSourceWorkspaceWithoutMembership(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	r, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p)
	if err != nil || r.Code != "knowledge_proposed" || r.CandidateID != p.CandidateID || r.State != "pending" {
		t.Fatalf("external proposal: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
		t.Fatal("proposal created target membership")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("pending proposal changed target knowledge")
	}
	replay, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(replay)
	if err != nil || string(a) != string(b) {
		t.Fatalf("proposal replay: %+v %v", replay, err)
	}
	p.Content += " changed"
	if _, err = f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed original proposal: %v", err)
	}
}

func TestKnowledgeHumanApprovalAndWithdrawalUseExactReviewedVersion(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(p.CandidateID)
	approve := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	if _, err := f.svc.ApproveKnowledge(collaborationContext(f.handler), id, approve); !collaborationErrorCode(err, "knowledge_permission_denied") {
		t.Fatalf("external proposal author gained confirmation: %v", err)
	}
	r, err := f.svc.ApproveKnowledge(f.ctx, id, approve)
	if err != nil || r.Code != "knowledge_approved" || r.StateRevision != "2" || r.TargetKnowledgeRevision == nil || *r.TargetKnowledgeRevision != "1" {
		t.Fatalf("approve: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint'`, f.workspace) != 1 {
		t.Fatal("approval did not retain one target hint")
	}
	replay, err := f.svc.ApproveKnowledge(f.ctx, id, approve)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(replay)
	if err != nil || string(a) != string(b) {
		t.Fatalf("approval replay: %+v %v", replay, err)
	}
	withdraw := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "Replace with reviewed correction"}
	stale, err := f.svc.WithdrawKnowledge(f.ctx, id, withdraw)
	if !collaborationErrorCode(err, "knowledge_revision_conflict") || stale.Code != "knowledge_revision_conflict" {
		t.Fatalf("stale review: %+v %v", stale, err)
	}
	withdraw.RequestID = uuid.NewString()
	withdraw.ExpectedStateRevision = "2"
	r, err = f.svc.WithdrawKnowledge(collaborationContext(f.handler), id, withdraw)
	if err != nil || r.State != "withdrawn" || r.StateRevision != "3" || *r.TargetKnowledgeRevision != "2" {
		t.Fatalf("source manager withdrawal: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND approved_at IS NOT NULL AND published_target_revision=1`, id) != 1 {
		t.Fatal("withdrawal erased approval facts")
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "PRIVATE_KNOWLEDGE") || strings.Contains(string(encoded), f.workspace) {
		t.Fatal("receipt disclosed private source or content")
	}
}

func TestKnowledgeSourceAvailabilitySuppressesPurgedAndDeletedSources(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	available, err := f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, p.Sources)
	if err != nil || !available {
		t.Fatalf("valid source: %v %v", available, err)
	}
	f.fx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=now(),purged_by_user_id=$2,purge_reason='remove retained source' WHERE issue_id=$1 AND version=1`, f.issue.Issue.ID, f.creator)
	available, err = f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, p.Sources)
	if err != nil || available {
		t.Fatalf("purged source restored from hash: %v %v", available, err)
	}
	comment := f.fx.Insert(t, "comment", testutil.Cols{"workspace_id": f.workspace, "issue_id": f.issue.Issue.ID, "author_type": "member", "author_id": f.creator, "content": "PRIVATE_COMMENT", "type": "comment"})
	ref := []protocol.CollaborationKnowledgeSource{{Kind: "comment", IssueID: util.UUIDToString(f.issue.Issue.ID), CommentID: comment, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_COMMENT")}}
	available, err = f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, ref)
	if err != nil || !available {
		t.Fatalf("current comment source: %v %v", available, err)
	}
	f.fx.Exec(t, `UPDATE comment SET content='',deleted_at=now(),revision=revision+1 WHERE id=$1`, comment)
	available, err = f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, ref)
	if err != nil || available {
		t.Fatalf("deleted comment source: %v %v", available, err)
	}
}

func TestKnowledgeHumanChangesPreserveBaseContextAndExecutionState(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	project := f.fx.Project(t, "Knowledge base project", testutil.Cols{"description": "human-maintained project base"})
	f.fx.Exec(t, `UPDATE workspace SET context='human-maintained workspace base' WHERE id=$1`, f.workspace)
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	p.Target.Kind = "project"
	p.Target.ProjectID = project
	snapshot := func() string {
		t.Helper()
		var value string
		f.fx.QueryRow(t, `SELECT jsonb_build_object(
        'workspace',(SELECT to_jsonb(w) FROM workspace w WHERE id=$1),
        'project',(SELECT to_jsonb(p) FROM project p WHERE id=$2),
        'issue',(SELECT to_jsonb(i) FROM issue i WHERE id=$3),
        'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE issue_id=$3),
        'tasks',(SELECT jsonb_agg(to_jsonb(q)) FROM agent_task_queue q WHERE issue_id=$3),
        'targets',(SELECT jsonb_agg(to_jsonb(t)) FROM collaboration_target t WHERE session_id=$4))::text`, f.workspace, project, f.issue.Issue.ID, f.issue.SessionID).Scan(&value)
		return value
	}
	before := snapshot()
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw managed material"}); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(); after != before {
		t.Fatal("knowledge publication changed base strings, parent revisions, task inputs or execution state")
	}
}
