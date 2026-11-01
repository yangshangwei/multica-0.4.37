package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func runKnowledgeBundleFixture() CollaborationRunKnowledgeBundle {
	issue := uuid.NewString()
	c := CollaborationKnowledgeContribution{CandidateID: uuid.NewString(), Version: "1", SourceIssueID: issue, Target: CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: uuid.NewString()}, Summary: "approved exact source", Content: strings.Repeat("<&>\n", 16<<10), Sources: []CollaborationKnowledgeSource{{Kind: "memory", IssueID: issue, Version: "1", SHA256: strings.Repeat("c", 64)}}, Evidence: []CollaborationKnowledgeEvidence{}, Applicability: CollaborationKnowledgeApplicability{Surfaces: []string{"collaboration"}}}
	c.ContentSHA256 = CollaborationKnowledgeHash(c.Content)
	manifest := TaskKnowledgeManifest{ProtocolVersion: 1, Entries: []TaskKnowledgeManifestEntry{{CandidateID: c.CandidateID, Version: c.Version, SourceIssueID: c.SourceIssueID, ContentSHA256: c.ContentSHA256, Target: c.Target}}}
	raw, _ := json.Marshal(manifest)
	return CollaborationRunKnowledgeBundle{CollaborationRunKnowledgePrepare: CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: CollaborationRunIdentity{RunID: uuid.NewString()}, RequestID: uuid.NewString(), HandoffID: uuid.NewString(), InvocationFingerprint: strings.Repeat("a", 64)}, Manifest: manifest, ManifestSHA256: CollaborationKnowledgeHash(string(raw)), Contributions: []CollaborationKnowledgeContribution{c}, Targets: []TaskKnowledgeTargetRevision{}}
}

func TestCollaborationKnowledgeRuntimeWireKeepsFullEscapedContribution(t *testing.T) {
	p := runKnowledgeBundleFixture()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) <= 64<<10 {
		t.Fatal("fixture did not exceed metadata limit")
	}
	if err = ValidateCollaborationRunKnowledgeBundle(p, len(raw)); err != nil {
		t.Fatal(err)
	}
	var decoded CollaborationRunKnowledgeBundle
	if err = json.Unmarshal(raw, &decoded); err != nil || decoded.Contributions[0].Content != p.Contributions[0].Content {
		t.Fatalf("full bounded contribution was lost: %v", err)
	}
	p.Manifest.Entries[0].Version = "2"
	if ValidateCollaborationRunKnowledgeBundle(p, 0) == nil {
		t.Fatal("changed manifest accepted")
	}
}

func TestCollaborationKnowledgeRuntimeWireIncludesMetadataInMaterialBudget(t *testing.T) {
	p := runKnowledgeBundleFixture()
	p.InvocationFingerprint = strings.Repeat("a", CollaborationKnowledgeMaxRuntimeBytes)
	if ValidateCollaborationRunKnowledgeBundle(p, 0) == nil {
		t.Fatal("unbounded wrapper metadata accepted")
	}
	p = runKnowledgeBundleFixture()
	if ValidateCollaborationRunKnowledgeBundle(p, CollaborationKnowledgeMaxRuntimeJSONBytes+1) == nil {
		t.Fatal("raw wire budget ignored")
	}
}
