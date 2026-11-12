package protocol

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const TaskKnowledgeVersion = 1

// TaskKnowledgeFence binds an ordinary mutation to the finalized claim even
// when it reports no native ID or the next claim no longer negotiates knowledge.
type TaskKnowledgeFence struct {
	ClaimGeneration string `json:"claim_generation"`
	DispatchedAt    string `json:"dispatched_at"`
}

func (v *TaskKnowledgeFence) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgeFence
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "claim_generation", "dispatched_at"); err != nil {
		return err
	}
	*v = TaskKnowledgeFence(w)
	return nil
}

// Invocation is supplied by the authenticated runtime, not by task text. Its
// claim response contains only the opaque identity/digest, never these args.
type TaskKnowledgeInvocation struct {
	ProtocolVersion int      `json:"protocol_version"`
	Provider        string   `json:"provider"`
	Mode            string   `json:"mode"`
	Builtin         bool     `json:"builtin"`
	ConfigurationID string   `json:"configuration_id"`
	ExtraArgs       []string `json:"extra_args"`
}

type TaskKnowledgeSessionBinding struct {
	Provider        string `json:"provider"`
	RuntimeID       string `json:"runtime_id"`
	AgentID         string `json:"agent_id"`
	ConfigurationID string `json:"configuration_id"`
	ScopeKind       string `json:"scope_kind"`
	ScopeID         string `json:"scope_id"`
	Generation      string `json:"generation"`
	NativeSessionID string `json:"native_session_id,omitempty"`
}

type TaskKnowledgeClaimDescriptor struct {
	ProtocolVersion     int                         `json:"protocol_version"`
	TaskID              string                      `json:"task_id"`
	RuntimeID           string                      `json:"runtime_id"`
	ClaimGeneration     string                      `json:"claim_generation"`
	DispatchedAt        string                      `json:"dispatched_at"`
	Provider            string                      `json:"provider"`
	Mode                string                      `json:"mode"`
	ConfigurationID     string                      `json:"configuration_id"`
	PreparationRequired bool                        `json:"preparation_required"`
	Session             TaskKnowledgeSessionBinding `json:"session"`
}

// Keep the field order stable: the exact JSON bytes define the manifest hash.
type TaskKnowledgeManifestEntry struct {
	CandidateID   string                       `json:"candidate_id"`
	Version       string                       `json:"version"`
	ContentSHA256 string                       `json:"content_sha256"`
	SourceIssueID string                       `json:"source_issue_id"`
	Target        CollaborationKnowledgeTarget `json:"target"`
}
type TaskKnowledgeManifest struct {
	ProtocolVersion int                          `json:"protocol_version"`
	Entries         []TaskKnowledgeManifestEntry `json:"entries"`
}
type TaskKnowledgeTargetRevision struct {
	Target   CollaborationKnowledgeTarget `json:"target"`
	Revision string                       `json:"knowledge_revision"`
}

type TaskKnowledgePrepareRequest struct {
	InvocationFingerprint string                      `json:"invocation_fingerprint"`
	ProtocolVersion       int                         `json:"protocol_version"`
	RequestID             string                      `json:"request_id"`
	ClaimGeneration       string                      `json:"claim_generation"`
	DispatchedAt          string                      `json:"dispatched_at"`
	Session               TaskKnowledgeSessionBinding `json:"session"`
}
type TaskKnowledgePrepareResponse struct {
	InvocationFingerprint string                               `json:"invocation_fingerprint"`
	ProtocolVersion       int                                  `json:"protocol_version"`
	PrepareID             string                               `json:"prepare_id"`
	Manifest              TaskKnowledgeManifest                `json:"manifest"`
	ManifestSHA256        string                               `json:"manifest_sha256"`
	Contributions         []CollaborationKnowledgeContribution `json:"contributions"`
	Targets               []TaskKnowledgeTargetRevision        `json:"targets"`
	Session               TaskKnowledgeSessionBinding          `json:"session"`
}
type TaskKnowledgeAuthorizeRequest struct {
	InvocationFingerprint string                      `json:"invocation_fingerprint"`
	ProtocolVersion       int                         `json:"protocol_version"`
	RequestID             string                      `json:"request_id"`
	PrepareID             string                      `json:"prepare_id"`
	ClaimGeneration       string                      `json:"claim_generation"`
	DispatchedAt          string                      `json:"dispatched_at"`
	ManifestSHA256        string                      `json:"manifest_sha256"`
	PromptSHA256          string                      `json:"prompt_sha256"`
	AttemptSequence       int64                       `json:"attempt_seq"`
	AttemptNonce          string                      `json:"attempt_nonce"`
	Session               TaskKnowledgeSessionBinding `json:"session"`
}
type TaskKnowledgeAuthorization struct {
	InvocationFingerprint string                      `json:"invocation_fingerprint"`
	ProtocolVersion       int                         `json:"protocol_version"`
	PrepareID             string                      `json:"prepare_id"`
	AttemptSequence       int64                       `json:"attempt_seq"`
	AttemptNonce          string                      `json:"attempt_nonce"`
	ManifestSHA256        string                      `json:"manifest_sha256"`
	PromptSHA256          string                      `json:"prompt_sha256"`
	Session               TaskKnowledgeSessionBinding `json:"session"`
}
type TaskKnowledgeObservation struct {
	ProtocolVersion   int    `json:"protocol_version"`
	RequestID         string `json:"request_id"`
	PrepareID         string `json:"prepare_id"`
	ClaimGeneration   string `json:"claim_generation"`
	DispatchedAt      string `json:"dispatched_at"`
	AttemptSequence   int64  `json:"attempt_seq"`
	AttemptNonce      string `json:"attempt_nonce"`
	Kind              string `json:"kind"`
	NativeSessionID   string `json:"native_session_id,omitempty"`
	Source            string `json:"source,omitempty"`
	ProcessInstanceID string `json:"process_instance_id,omitempty"`
	ProcessID         int    `json:"process_id,omitempty"`
	TerminationKind   string `json:"termination_kind,omitempty"`
	WaitObserved      bool   `json:"wait_observed,omitempty"`
}

func ValidTaskKnowledgeIdentity(value string) bool {
	return value != "" && len(value) <= 4096 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
func ValidTaskKnowledgeDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
func ValidateTaskKnowledgeSession(b TaskKnowledgeSessionBinding) error {
	for _, id := range []string{b.RuntimeID, b.AgentID, b.ScopeID, b.Generation} {
		if _, err := uuid.Parse(id); err != nil {
			return errors.New("invalid knowledge session identity")
		}
	}
	if !ValidTaskKnowledgeIdentity(b.Provider) || !ValidTaskKnowledgeIdentity(b.ConfigurationID) || len(b.ConfigurationID) > 256 {
		return errors.New("invalid knowledge session configuration")
	}
	switch b.ScopeKind {
	case "issue", "chat", "quick_create", "autopilot":
	default:
		return errors.New("invalid knowledge session scope")
	}
	if b.NativeSessionID != "" && !ValidTaskKnowledgeIdentity(b.NativeSessionID) {
		return errors.New("invalid native session identity")
	}
	return nil
}
func ValidateTaskKnowledgeRequest(version int, requestID, generation, dispatchedAt string) error {
	if version != TaskKnowledgeVersion {
		return errors.New("unsupported knowledge version")
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return errors.New("invalid knowledge request identity")
	}
	number, err := strconv.ParseInt(generation, 10, 64)
	if err != nil || number < 1 || strconv.FormatInt(number, 10) != generation {
		return errors.New("invalid claim generation")
	}
	if _, err := time.Parse(time.RFC3339Nano, dispatchedAt); err != nil {
		return errors.New("invalid dispatch identity")
	}
	return nil
}
func TaskKnowledgeRequestHash(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) > CollaborationKnowledgeMaxDeliveryBytes {
		return "", errors.New("invalid knowledge request")
	}
	return CollaborationKnowledgeHash(string(raw)), nil
}

func (v *TaskKnowledgeInvocation) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgeInvocation
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "provider", "mode", "builtin", "configuration_id"); err != nil {
		return err
	}
	if !knowledgeText(w.Provider, 128) || !knowledgeText(w.Mode, 64) || !knowledgeText(w.ConfigurationID, 256) || len(w.ExtraArgs) > 64 {
		return ErrInvalidCollaborationKnowledge
	}
	for _, arg := range w.ExtraArgs {
		if !knowledgeText(arg, 4096) {
			return ErrInvalidCollaborationKnowledge
		}
	}
	*v = TaskKnowledgeInvocation(w)
	return nil
}

func (v *TaskKnowledgeSessionBinding) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgeSessionBinding
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "provider", "runtime_id", "agent_id", "configuration_id", "scope_kind", "scope_id", "generation"); err != nil {
		return err
	}
	if ValidateTaskKnowledgeSession(TaskKnowledgeSessionBinding(w)) != nil {
		return ErrInvalidCollaborationKnowledge
	}
	*v = TaskKnowledgeSessionBinding(w)
	return nil
}

func (v *TaskKnowledgePrepareRequest) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgePrepareRequest
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id", "claim_generation", "dispatched_at", "session", "invocation_fingerprint"); err != nil {
		return err
	}
	*v = TaskKnowledgePrepareRequest(w)
	return nil
}

func (v *TaskKnowledgeAuthorizeRequest) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgeAuthorizeRequest
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id", "prepare_id", "claim_generation", "dispatched_at", "manifest_sha256", "prompt_sha256", "attempt_seq", "attempt_nonce", "session", "invocation_fingerprint"); err != nil {
		return err
	}
	*v = TaskKnowledgeAuthorizeRequest(w)
	return nil
}

func (v *TaskKnowledgeObservation) UnmarshalJSON(raw []byte) error {
	type wire TaskKnowledgeObservation
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id", "prepare_id", "claim_generation", "dispatched_at", "attempt_seq", "attempt_nonce", "kind"); err != nil {
		return err
	}
	switch w.Kind {
	case "started":
		if !knowledgeAbsent(raw, "native_session_id", "source", "termination_kind", "wait_observed") {
			return ErrInvalidCollaborationKnowledge
		}
	case "native_session":
		if !knowledgeAbsent(raw, "process_id", "process_instance_id", "termination_kind", "wait_observed") {
			return ErrInvalidCollaborationKnowledge
		}
	case "terminated":
		if !knowledgeAbsent(raw, "native_session_id", "source") {
			return ErrInvalidCollaborationKnowledge
		}
	default:
		return ErrInvalidCollaborationKnowledge
	}
	*v = TaskKnowledgeObservation(w)
	return nil
}
