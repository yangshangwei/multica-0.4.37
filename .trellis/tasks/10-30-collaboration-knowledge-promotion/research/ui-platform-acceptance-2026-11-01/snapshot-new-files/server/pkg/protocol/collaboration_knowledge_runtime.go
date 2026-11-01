package protocol

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const CollaborationRunKnowledgeVersion = 1

// This descriptor is source-free. Namespace is exclusive to this run, even
// when its authorized contribution set is empty.
type CollaborationRunKnowledgeClaim struct {
	ProtocolVersion     int                      `json:"protocol_version"`
	Identity            CollaborationRunIdentity `json:"identity"`
	ConfigurationDigest string                   `json:"configuration_digest"`
	Namespace           string                   `json:"namespace"`
}

type CollaborationRunKnowledgePrepare struct {
	ProtocolVersion       int                      `json:"protocol_version"`
	Identity              CollaborationRunIdentity `json:"identity"`
	RequestID             string                   `json:"request_id"`
	HandoffID             string                   `json:"handoff_id"`
	InvocationFingerprint string                   `json:"invocation_fingerprint"`
}

type CollaborationRunKnowledgeBundle struct {
	CollaborationRunKnowledgePrepare
	Manifest       TaskKnowledgeManifest                `json:"manifest"`
	ManifestSHA256 string                               `json:"manifest_sha256"`
	Contributions  []CollaborationKnowledgeContribution `json:"contributions"`
	Targets        []TaskKnowledgeTargetRevision        `json:"targets"`
}

type CollaborationRunKnowledgeAuthorization struct {
	CollaborationRunKnowledgePrepare
	ManifestSHA256 string `json:"manifest_sha256"`
	PromptSHA256   string `json:"prompt_sha256"`
	AttemptSeq     string `json:"attempt_seq"`
	AttemptNonce   string `json:"attempt_nonce"`
}

// Recovery exposes only prior operation identities, never a manifest or body.
type CollaborationRunKnowledgeStatus struct {
	ProtocolVersion int                      `json:"protocol_version"`
	Identity        CollaborationRunIdentity `json:"identity"`
	Prepared        bool                     `json:"prepared"`
	RequestID       string                   `json:"request_id,omitempty"`
}

func ValidateCollaborationRunKnowledgePrepare(p CollaborationRunKnowledgePrepare) error {
	if p.ProtocolVersion != 1 || !HandoffDigest(p.InvocationFingerprint) {
		return errors.New("invalid collaboration knowledge preparation")
	}
	for _, id := range []string{p.RequestID, p.HandoffID, p.Identity.RunID} {
		u, err := uuid.Parse(id)
		if err != nil || u == uuid.Nil || u.String() != id {
			return errors.New("invalid collaboration knowledge identity")
		}
	}
	return nil
}

func ValidateCollaborationRunKnowledgeBundle(p CollaborationRunKnowledgeBundle, wireBytes int) error {
	if ValidateCollaborationRunKnowledgePrepare(p.CollaborationRunKnowledgePrepare) != nil ||
		p.Manifest.ProtocolVersion != 1 || p.Manifest.Entries == nil || p.Contributions == nil || p.Targets == nil ||
		len(p.Manifest.Entries) != len(p.Contributions) || len(p.Targets) > 2 {
		return errors.New("invalid collaboration knowledge bundle")
	}
	raw, err := json.Marshal(p.Manifest)
	if err != nil || CollaborationKnowledgeHash(string(raw)) != p.ManifestSHA256 {
		return errors.New("invalid collaboration knowledge manifest")
	}
	for i, entry := range p.Manifest.Entries {
		if i > 0 && p.Manifest.Entries[i-1].CandidateID >= entry.CandidateID {
			return errors.New("invalid collaboration knowledge manifest order")
		}
		found := false
		for _, c := range p.Contributions {
			if c.CandidateID == entry.CandidateID && c.Version == entry.Version && c.ContentSHA256 == entry.ContentSHA256 && c.SourceIssueID == entry.SourceIssueID && c.Target == entry.Target {
				found = true
			}
		}
		if !found {
			return errors.New("invalid collaboration knowledge contribution")
		}
	}
	if err := ValidateCollaborationKnowledgeRuntimeBudget(p.Contributions, wireBytes); err != nil {
		return err
	}
	material, encoded, err := CollaborationKnowledgeSizes(p)
	if err != nil || material > CollaborationKnowledgeMaxRuntimeBytes || encoded+1 > CollaborationKnowledgeMaxRuntimeJSONBytes {
		return errors.New("collaboration knowledge wire budget exceeded")
	}
	return nil
}
