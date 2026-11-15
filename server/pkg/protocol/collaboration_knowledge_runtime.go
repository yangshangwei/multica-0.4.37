package protocol

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const (
	CollaborationRunKnowledgeVersion       = 1
	CollaborationRunKnowledgeLatestVersion = 2
)

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
	if !IsKnowledgeProtocolVersion(p.ProtocolVersion) || !HandoffDigest(p.InvocationFingerprint) {
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
		ValidateTaskKnowledgeManifestContributions(p.ProtocolVersion, p.Manifest, p.Contributions) != nil || p.Targets == nil || len(p.Targets) > 2 {
		return errors.New("invalid collaboration knowledge bundle")
	}
	raw, err := json.Marshal(p.Manifest)
	if err != nil || CollaborationKnowledgeHash(string(raw)) != p.ManifestSHA256 {
		return errors.New("invalid collaboration knowledge manifest")
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

func (v *CollaborationRunKnowledgeClaim) UnmarshalJSON(raw []byte) error {
	type wire CollaborationRunKnowledgeClaim
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "identity", "configuration_digest", "namespace"); err != nil {
		return err
	}
	if !IsKnowledgeProtocolVersion(w.ProtocolVersion) || !HandoffDigest(w.ConfigurationDigest) || !preparationUUID(w.Identity.RunID) || w.Namespace != w.Identity.RunID {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationRunKnowledgeClaim(w)
	return nil
}

// Decode concrete envelopes. A decoder on the embedded preparation would
// otherwise consume the complete bundle or authorization as that smaller type.
func (v *CollaborationRunKnowledgeBundle) UnmarshalJSON(raw []byte) error {
	if len(raw) > CollaborationKnowledgeMaxRuntimeJSONBytes {
		return ErrCollaborationKnowledgeTooLarge
	}
	type wire CollaborationRunKnowledgeBundle
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "identity", "request_id", "handoff_id", "invocation_fingerprint", "manifest", "manifest_sha256", "contributions", "targets"); err != nil {
		return err
	}
	if err := ValidateCollaborationRunKnowledgeBundle(CollaborationRunKnowledgeBundle(w), len(raw)); err != nil {
		return err
	}
	*v = CollaborationRunKnowledgeBundle(w)
	return nil
}

func (v *CollaborationRunKnowledgeAuthorization) UnmarshalJSON(raw []byte) error {
	type wire CollaborationRunKnowledgeAuthorization
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "identity", "request_id", "handoff_id", "invocation_fingerprint", "manifest_sha256", "prompt_sha256", "attempt_seq", "attempt_nonce"); err != nil {
		return err
	}
	if ValidateCollaborationRunKnowledgePrepare(w.CollaborationRunKnowledgePrepare) != nil || !HandoffDigest(w.ManifestSHA256) || !HandoffDigest(w.PromptSHA256) || !preparationVersion(w.AttemptSeq) || !preparationUUID(w.AttemptNonce) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationRunKnowledgeAuthorization(w)
	return nil
}

func (v *CollaborationRunKnowledgeStatus) UnmarshalJSON(raw []byte) error {
	type wire CollaborationRunKnowledgeStatus
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "identity", "prepared"); err != nil {
		return err
	}
	if !IsKnowledgeProtocolVersion(w.ProtocolVersion) || !preparationUUID(w.Identity.RunID) || w.Prepared && !preparationUUID(w.RequestID) || !w.Prepared && !knowledgeAbsent(raw, "request_id") {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationRunKnowledgeStatus(w)
	return nil
}
