package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeSupportDependenciesRequireBoundedExactRefs(t *testing.T) {
	ref := `{"kind":"memory","issue_id":"11111111-1111-4111-8111-111111111111","version":"1","sha256":"` + strings.Repeat("a", 64) + `"}`
	if refs, valid := knowledgeSupportDependencies([]byte("[" + ref + "]")); !valid || len(refs) != 1 {
		t.Fatal("one exact required support was rejected")
	}
	for name, raw := range map[string]string{
		"null": "null", "empty": "[]", "object": "{}", "null source": "[null]",
		"duplicate field": "[" + strings.Replace(ref, `"version":"1"`, `"version":"1","version":"2"`, 1) + "]",
		"unknown field":   "[" + strings.Replace(ref, `"kind":"memory"`, `"kind":"memory","private":"not a ref"`, 1) + "]",
		"too many":        "[" + strings.TrimSuffix(strings.Repeat(ref+",", 9), ",") + "]",
	} {
		t.Run(name, func(t *testing.T) {
			if _, valid := knowledgeSupportDependencies([]byte(raw)); valid {
				t.Fatal("incomplete or unbounded support proof accepted")
			}
		})
	}
}

func TestKnowledgeAutomaticApprovalRequiresProofAndV2(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	project := f.fx.Project(t, "Automatic proof target")
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	p := knowledgeServiceProposal(t, f)
	sources, err := json.Marshal(p.Sources)
	if err != nil {
		t.Fatal(err)
	}
	id, job := uuid.NewString(), uuid.NewString()
	// Missing head/job is deliberate corruption: no FK is relied upon to make
	// the runtime proof complete, and no immutable trigger is disabled.
	f.fx.Insert(t, "collaboration_knowledge_candidate", testutil.Cols{"id": id, "lineage_id": id, "version": 1,
		"source_workspace_id": f.workspace, "source_issue_id": f.issue.Issue.ID, "target_kind": "project", "target_workspace_id": f.workspace, "target_project_id": project,
		"content": p.Content, "summary": p.Summary, "content_sha256": protocol.CollaborationKnowledgeHash(p.Content), "sources": sources, "evidence": `[]`, "applicability": `{"surfaces":["issue"]}`,
		"proposal_origin": "automation", "automation_job_id": job, "automation_policy_revision": 0, "automation_rule_version": "2026-11-14-v1",
		"state": "approved", "state_revision": 2, "approval_origin": "automation", "approval_job_id": job, "approved_at": time.Now(), "published_target_revision": 1,
		"decision_origin": "automation", "last_decision_job_id": job, "last_decided_at": time.Now()})
	f.fx.InsertNoID(t, "collaboration_knowledge_target", testutil.Cols{"target_kind": "project", "target_id": project, "workspace_id": f.workspace, "knowledge_revision": 1}, "target_kind='project' AND target_id=$1", project)
	facts := knowledgeConsumerFixture(t, f, f.workspace)
	facts.ProjectID = util.MustParseUUID(project)
	t.Run("v1 must not silently omit or consume automation", func(t *testing.T) {
		if _, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts); !collaborationErrorCode(err, "knowledge_protocol_unsupported") {
			t.Fatalf("v1 automatic target did not explicitly refuse: %v", err)
		}
	})
	t.Run("v2 needs a complete proof", func(t *testing.T) {
		facts.ProtocolVersion = 2
		resolved, err := f.svc.ResolveApprovedKnowledge(f.ctx, f.svc.Queries, facts)
		if err != nil || len(resolved.Contributions) != 0 || resolved.Manifest.ProtocolVersion != 2 {
			t.Fatalf("missing proof became active knowledge: items=%d version=%d err=%v", len(resolved.Contributions), resolved.Manifest.ProtocolVersion, err)
		}
	})
}
