package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeResolverCanonicalContributionsMatchPreparedReplay(t *testing.T) {
	f := newRuntimeKnowledgeFixture(t, false, true)
	project := f.fx.Project(t, "Canonical runtime contributions")
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	for _, target := range []struct{ id, kind string }{
		{"f" + uuid.NewString()[1:], "project"},
		{"0" + uuid.NewString()[1:], "workspace"},
	} {
		p := f.proposal
		p.CandidateID, p.RequestID = target.id, uuid.NewString()
		if target.kind == "project" {
			p.Target.Kind, p.Target.ProjectID = "project", project
		}
		if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Contributions) != 2 || !reflect.DeepEqual(first, replay) {
		t.Fatal("first preparation and original-key replay changed contribution order")
	}
	for i, entry := range first.Manifest.Entries {
		if first.Contributions[i].CandidateID != entry.CandidateID {
			t.Fatal("wire contributions do not follow canonical manifest order")
		}
	}
}

func TestKnowledgeSourceDependencyFailureIsNotEmptyKnowledge(t *testing.T) {
	s := &CollaborationService{}
	if available, err := s.KnowledgeSourceAvailability(context.Background(), nil, util.MustParseUUID(uuid.NewString()), nil); available || !collaborationErrorCode(err, "knowledge_unavailable") {
		t.Fatalf("missing source dependency became legitimate empty data: %v %v", available, err)
	}
	f := KnowledgeConsumerFacts{Supported: true, OriginatorUserID: util.MustParseUUID(uuid.NewString()), RuntimeOwnerUserID: util.MustParseUUID(uuid.NewString()), RuntimeID: util.MustParseUUID(uuid.NewString())}
	if _, err := s.ResolveApprovedKnowledge(context.Background(), nil, f); !collaborationErrorCode(err, "knowledge_unavailable") {
		t.Fatalf("missing resolver dependency became a no-contribution success: %v", err)
	}
}

func knowledgeConsumerFixture(t *testing.T, f *collaborationQueueFixture, workspace string) KnowledgeConsumerFacts {
	t.Helper()
	runtime := f.fx.Runtime(t, "Knowledge runtime", testutil.Cols{"workspace_id": workspace, "owner_id": f.creator, "provider": "claude"})
	agent := f.fx.Agent(t, "Knowledge agent", runtime, testutil.Cols{"workspace_id": workspace, "owner_id": f.creator})
	task := f.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "originator_user_id": f.creator, "accountable_user_id": f.creator})
	return KnowledgeConsumerFacts{Supported: true, Surface: "issue", WorkspaceID: util.MustParseUUID(workspace), OriginatorUserID: util.MustParseUUID(f.creator), RuntimeOwnerUserID: util.MustParseUUID(f.creator), AgentID: util.MustParseUUID(agent), RuntimeID: util.MustParseUUID(runtime), TaskID: util.MustParseUUID(task), Audience: "workspace"}
}

func TestKnowledgeResolverCurrentSourcesConsumersAndExactManifest(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	resolution, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(resolution.Contributions) != 0 {
		t.Fatalf("pending injected: %+v %v", resolution, err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	resolution, err = f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(resolution.Contributions) != 1 || resolution.Contributions[0].Content != p.Content {
		t.Fatalf("approved input: %+v %v", resolution, err)
	}
	original := resolution.Manifest
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.Content = "second approved fact"
	if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), knowledgeApproveRequest(second)); err != nil {
		t.Fatal(err)
	}
	if ok, reason, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, original); err != nil || !ok {
		t.Fatalf("unrelated new contribution revoked exact history: %v %s %v", ok, reason, err)
	}
	if _, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "old fact withdrawn"}); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, f.svc.Queries, facts, original); err != nil || ok {
		t.Fatalf("withdrawn manifest was replaced with current set: %v %v", ok, err)
	}
	current, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(current.Contributions) != 1 || current.Contributions[0].CandidateID != second.CandidateID {
		t.Fatalf("current contribution set: %+v %v", current, err)
	}
	f.fx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=now(),purged_by_user_id=$2,purge_reason='remove source' WHERE issue_id=$1`, f.issue.Issue.ID, f.creator)
	current, err = f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	raw, _ := json.Marshal(current)
	if err != nil || len(current.Contributions) != 0 || strings.Contains(string(raw), second.CandidateID) || strings.Contains(string(raw), "PRIVATE_") {
		t.Fatalf("purged runtime source leaked: %s %v", raw, err)
	}
}

func TestKnowledgeResolverNoOriginatorOrOwnerAuthorityFallback(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	base := knowledgeConsumerFixture(t, f, f.workspace)
	for name, change := range map[string]func(*KnowledgeConsumerFacts){
		"unsupported":           func(v *KnowledgeConsumerFacts) { v.Supported = false },
		"missing originator":    func(v *KnowledgeConsumerFacts) { v.OriginatorUserID.Valid = false },
		"missing owner":         func(v *KnowledgeConsumerFacts) { v.RuntimeOwnerUserID.Valid = false },
		"unproven task":         func(v *KnowledgeConsumerFacts) { v.TaskID.Valid = false },
		"unavailable audience":  func(v *KnowledgeConsumerFacts) { v.Audience = "unavailable" },
		"wrong runtime owner":   func(v *KnowledgeConsumerFacts) { v.RuntimeOwnerUserID = util.MustParseUUID(f.handler) },
		"unreadable originator": func(v *KnowledgeConsumerFacts) { v.OriginatorUserID = util.MustParseUUID(f.outsider) },
	} {
		t.Run(name, func(t *testing.T) {
			facts := base
			change(&facts)
			result, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
			raw, _ := json.Marshal(result)
			if err != nil || len(result.Contributions) != 0 || len(result.Manifest.Entries) != 0 || strings.Contains(string(raw), p.CandidateID) {
				t.Fatalf("fallback disclosed source: %s %v", raw, err)
			}
		})
	}
}

func TestKnowledgeResolverCrossWorkspaceAudienceAndPrivateChat(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	target := f.fx.Workspace(t, "Private chat target", "knowledge-chat-"+uuid.NewString())
	f.fx.Member(t, target, f.creator, "owner")
	p.Target.WorkspaceID = target
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, target)
	result, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(result.Contributions) != 0 || result.Reason != "audience_unavailable" {
		t.Fatalf("ordinary target audience broadened source: %+v %v", result, err)
	}
	chat := f.fx.ChatSession(t, util.UUIDToString(facts.AgentID), testutil.Cols{"workspace_id": target, "creator_id": f.creator})
	facts.Surface = "chat"
	facts.Audience = "private_chat"
	facts.ChatSessionID = util.MustParseUUID(chat)
	result, err = f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(result.Contributions) != 1 || result.Contributions[0].Content != p.Content {
		t.Fatalf("authorized private chat: %+v %v", result, err)
	}
	facts.OriginatorUserID = util.MustParseUUID(f.handler)
	result, err = f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil || len(result.Contributions) != 0 || result.Reason != "audience_unavailable" {
		t.Fatalf("chat creator was replaced by sender/installer: %+v %v", result, err)
	}
}

func TestKnowledgeReadLockHintsContainOnlyBoundedMetadata(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	hints, err := f.svc.CollectKnowledgeReadLockHints(f.ctx, f.svc.Queries, facts, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(hints)
	if strings.Contains(string(raw), "PRIVATE_KNOWLEDGE") || len(hints.CandidateIDs) != 1 || len(hints.Sources) != 1 || len(hints.Targets) != 1 {
		t.Fatalf("discovery leaked body or omitted locks: %s", raw)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	q := f.svc.Queries.WithTx(tx)
	if _, err = q.LockKnowledgeWorkspace(f.ctx, facts.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.LockKnowledgeReadTargets(f.ctx, q, hints); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.LockKnowledgeReadSources(f.ctx, q, hints); err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.ResolveApprovedKnowledge(f.ctx, q, facts)
	if err != nil || len(result.Contributions) != 1 {
		t.Fatalf("locked resolution: %+v %v", result, err)
	}
}

func TestKnowledgeHumanTargetBudgetRefusalRetainsOriginalOutcome(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	p.Content = strings.Repeat("x", 64<<10)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	request := knowledgeApproveRequest(second)
	refused, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), request)
	if !collaborationErrorCode(err, "knowledge_target_budget_exceeded") || refused.State != "pending" {
		t.Fatalf("complete target material budget: %+v %v", refused, err)
	}
	if _, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "release target capacity"}); err != nil {
		t.Fatal(err)
	}
	replay, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), request)
	a, _ := json.Marshal(refused)
	b, _ := json.Marshal(replay)
	if !collaborationErrorCode(err, "knowledge_target_budget_exceeded") || string(a) != string(b) {
		t.Fatalf("old refusal recomputed after capacity changed: %s %v", b, err)
	}
	request.RequestID = uuid.NewString()
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), request); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeFreshLockHintsOmitMissingSourceButExactManifestRefuses(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	// This test deliberately deletes the source before ordinary fixture
	// cleanup can discover it through the issue table. Retire only these
	// fixture-owned legacy rows explicitly; retained candidate routing stays.
	for _, table := range []string{"issue_collaboration", "collaboration_session", "issue_participant", "collaboration_event"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", f.issue.Issue.ID)
	}
	bad := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(bad.CandidateID), knowledgeApproveRequest(bad)); err != nil {
		t.Fatal(err)
	}
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	original, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: other.Issue.ID, RequestID: uuid.NewString(), Content: "HEALTHY_SOURCE"}); err != nil {
		t.Fatal(err)
	}
	healthy := bad
	healthy.CandidateID = uuid.NewString()
	healthy.RequestID = uuid.NewString()
	healthy.Content = "HEALTHY_BODY"
	healthy.Sources = []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(other.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("HEALTHY_SOURCE")}}
	if _, err = f.svc.ProposeKnowledge(f.ctx, other.Issue.ID, healthy); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(healthy.CandidateID), knowledgeApproveRequest(healthy)); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM issue WHERE id=$1`, f.issue.Issue.ID)
	hints, err := f.svc.CollectKnowledgeReadLockHints(f.ctx, f.svc.Queries, facts, nil)
	if err != nil || len(hints.CandidateIDs) != 1 || hints.CandidateIDs[0] != util.MustParseUUID(healthy.CandidateID) {
		t.Fatalf("fresh missing-source discovery: %+v %v", hints, err)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	q := f.svc.Queries.WithTx(tx)
	if _, err = q.LockKnowledgeWorkspace(f.ctx, facts.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.LockKnowledgeReadTargets(f.ctx, q, hints); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.LockKnowledgeReadSources(f.ctx, q, hints); err != nil {
		t.Fatal(err)
	}
	resolved, err := f.svc.ResolveApprovedKnowledge(f.ctx, q, facts)
	if err != nil || len(resolved.Contributions) != 1 || resolved.Contributions[0].Content != "HEALTHY_BODY" {
		t.Fatalf("fresh resolution failed to omit unavailable source: %+v %v", resolved, err)
	}
	if err = ValidateKnowledgeResolutionLocks(resolved, hints); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := f.svc.RevalidateKnowledgeManifest(f.ctx, q, facts, original.Manifest); err != nil || ok {
		t.Fatalf("missing exact history became healthy replacement: %v %v", ok, err)
	}
	if _, err = f.svc.CollectKnowledgeReadLockHints(f.ctx, q, facts, &original.Manifest); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("missing exact history was silently omitted: %v", err)
	}
	if err = ValidateKnowledgeResolutionLocks(original, hints); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("unlocked restored source admitted: %v", err)
	}
}
