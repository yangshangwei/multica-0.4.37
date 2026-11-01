package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const CollaborationHandoffVersion = 1

// Material includes JSON metadata/framing and decoded UTF-8 source text. Wire
// bytes have a separate ceiling because admitted text can expand six-fold.
const CollaborationHandoffMaxBytes = 256 << 10
const CollaborationHandoffMaxJSONBytes = 2 << 20
const CollaborationHandoffPageLimit = 32

// Provenance describes server facts. Source text never grants execution rights.
type CollaborationHandoffEnvelope struct {
	ProtocolVersion    int                              `json:"protocol_version"`
	IssueID            string                           `json:"issue_id"`
	RunID              string                           `json:"run_id"`
	SnapshotID         string                           `json:"snapshot_id,omitempty"`
	Mode               string                           `json:"mode"`
	EventSequence      string                           `json:"event_seq"`
	Authority          CollaborationHandoffAuthority    `json:"authority"`
	Assignment         CollaborationHandoffAssignment   `json:"assignment"`
	Handoff            *CollaborationHandoffTransfer    `json:"handoff"`
	InformationRequest *CollaborationHandoffTransfer    `json:"information_request"`
	Recording          *CollaborationRecordingRetention `json:"recording,omitempty"`
	Memory             CollaborationHandoffMemory       `json:"memory"`
	Inputs             CollaborationHandoffInputs       `json:"inputs"`
	Results            CollaborationHandoffResults      `json:"results"`
	WakeupCauses       CollaborationHandoffCauses       `json:"wakeup_causes"`
	Sources            []CollaborationHandoffSource     `json:"sources"`
	Missing            []string                         `json:"missing"`
}

type CollaborationHandoffAuthority struct {
	Status    string `json:"status"`
	HandlerID string `json:"handler_id,omitempty"`
	CreatorID string `json:"creator_id,omitempty"`
}

type CollaborationHandoffAssignment struct {
	Role         string `json:"role"`
	StepKey      string `json:"step_key"`
	ParentRunID  string `json:"parent_run_id,omitempty"`
	DelegationID string `json:"delegation_id,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
}

type CollaborationHandoffTransfer struct {
	ID            string `json:"id"`
	AuthorID      string `json:"author_id"`
	FromID        string `json:"from_id"`
	ToID          string `json:"to_id"`
	Purpose       string `json:"purpose"`
	RequesterID   string `json:"requester_id,omitempty"`
	EventSequence string `json:"event_seq"`
	SourceID      string `json:"source_id"`
}

type CollaborationHandoffMemory struct {
	Version      string `json:"version"`
	SHA256       string `json:"sha256,omitempty"`
	Availability string `json:"availability"`
}

type CollaborationHandoffPage struct {
	Total      int    `json:"total"`
	Returned   int    `json:"returned"`
	Truncated  bool   `json:"truncated"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type CollaborationHandoffInput struct {
	CommandID        string   `json:"command_id"`
	AuthorID         string   `json:"author_id"`
	State            string   `json:"state"`
	ReaderRunIDs     []string `json:"reader_run_ids"`
	ReadersTruncated bool     `json:"readers_truncated"`
	ReadersCursor    string   `json:"readers_cursor,omitempty"`
}
type CollaborationHandoffInputs struct {
	CollaborationHandoffPage
	Items []CollaborationHandoffInput `json:"items"`
}

type CollaborationHandoffResult struct {
	RunID               string `json:"run_id"`
	ParentRunID         string `json:"parent_run_id,omitempty"`
	Role                string `json:"role"`
	StepKey             string `json:"step_key"`
	LeaderTurn          string `json:"leader_turn,omitempty"`
	Progress            string `json:"progress"`
	Outcome             string `json:"outcome,omitempty"`
	ProofState          string `json:"proof_state"`
	RecordingIncomplete bool   `json:"recording_incomplete"`
	ResultState         string `json:"result_state"`
	FinalAnswerState    string `json:"final_answer_state"`
	SourceID            string `json:"source_id,omitempty"`
	SourceKind          string `json:"source_kind,omitempty"`
}
type CollaborationHandoffResults struct {
	CollaborationHandoffPage
	Items []CollaborationHandoffResult `json:"items"`
}

type CollaborationHandoffCause struct {
	ID          string `json:"id"`
	SourceKind  string `json:"source_kind"`
	SourceID    string `json:"source_id"`
	SourceRunID string `json:"source_run_id"`
	Sequence    string `json:"sequence"`
}
type CollaborationHandoffCauses struct {
	CollaborationHandoffPage
	Items        []CollaborationHandoffCause `json:"items"`
	Completeness string                      `json:"completeness"`
}

// Sources are the sole text-bearing portion of an envelope. Snapshot storage
// strips Content, then rehydrates from authorized source/version/hash checks.
type CollaborationHandoffSource struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	RunID        string `json:"run_id,omitempty"`
	Version      string `json:"version,omitempty"`
	SHA256       string `json:"sha256"`
	Content      string `json:"content"`
	Truncated    bool   `json:"truncated"`
	Availability string `json:"availability"`
	Trust        string `json:"trust"`
}

type CollaborationHandoffSnapshotRequest struct {
	RequestID     string `json:"request_id"`
	MemoryVersion string `json:"memory_version"`
}
type CollaborationHandoffBinding struct {
	SnapshotID   string `json:"snapshot_id"`
	PromptSHA256 string `json:"prompt_sha256"`
	AttemptSeq   string `json:"attempt_seq"`
	AttemptNonce string `json:"attempt_nonce"`
}
type CollaborationHandoffBindingReceipt struct {
	ProtocolVersion int    `json:"protocol_version"`
	IssueID         string `json:"issue_id"`
	RunID           string `json:"run_id"`
	SnapshotID      string `json:"snapshot_id"`
	PromptSHA256    string `json:"prompt_sha256"`
	AttemptSeq      string `json:"attempt_seq"`
	AttemptNonce    string `json:"attempt_nonce"`
	Accepted        bool   `json:"accepted"`
}
type CollaborationHandoffSourcePage struct {
	ProtocolVersion int                        `json:"protocol_version"`
	IssueID         string                     `json:"issue_id"`
	RunID           string                     `json:"run_id"`
	Source          CollaborationHandoffSource `json:"source"`
	Offset          int                        `json:"offset"`
	TotalBytes      int                        `json:"total_bytes"`
	NextOffset      *int                       `json:"next_offset"`
}

// DecodeCollaborationHandoffSourcePage strictly decodes one runtime source
// chunk. Callers still compare the identity with the reference they requested.
func DecodeCollaborationHandoffSourcePage(raw []byte) (CollaborationHandoffSourcePage, error) {
	var out CollaborationHandoffSourcePage
	invalid := errors.New("invalid collaboration handoff source page")
	if decodeCollaborationHostObject(raw, &out, "protocol_version", "issue_id", "run_id", "source", "offset", "total_bytes") != nil ||
		!collaborationRecallHasFields(raw, "next_offset") {
		return out, invalid
	}
	s := out.Source
	end := out.Offset + len(s.Content)
	trust := "data_only"
	if s.Kind == "delegation" {
		trust = "admitted_assignment"
	}
	if out.ProtocolVersion != CollaborationHandoffVersion || !preparationUUID(out.IssueID) || !preparationUUID(out.RunID) || !preparationUUID(s.ID) ||
		s.RunID != "" && !preparationUUID(s.RunID) || s.Version != "" && !HandoffDecimal(s.Version, false) || !HandoffDigest(s.SHA256) || s.Availability != "available" || s.Trust != trust ||
		!utf8.ValidString(s.Content) || out.Offset < 0 || end > out.TotalBytes || s.Truncated != (out.NextOffset != nil) ||
		out.NextOffset != nil && (len(s.Content) == 0 || *out.NextOffset != end) || out.NextOffset == nil && end != out.TotalBytes {
		return out, invalid
	}
	switch s.Kind {
	case "handoff", "delegation", "provider_result", "squad_activity":
	default:
		return out, invalid
	}
	return out, nil
}

func CollaborationHandoffHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func HandoffDecimal(v string, zero bool) bool {
	n, e := strconv.ParseInt(v, 10, 64)
	return e == nil && n >= 0 && (zero || n > 0) && strconv.FormatInt(n, 10) == v
}
func HandoffDigest(v string) bool {
	b, e := hex.DecodeString(v)
	return e == nil && len(b) == 32 && strings.ToLower(v) == v
}
func (v *CollaborationHandoffEnvelope) UnmarshalJSON(raw []byte) error {
	if len(raw) > CollaborationHandoffMaxJSONBytes {
		return errors.New("collaboration handoff exceeds wire budget")
	}
	type wire CollaborationHandoffEnvelope
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "protocol_version", "issue_id", "run_id", "mode", "event_seq", "authority", "assignment", "memory", "inputs", "results", "wakeup_causes", "sources", "missing"); err != nil {
		return err
	}
	if !collaborationRecallHasFields(raw, "handoff", "information_request") {
		return errors.New("invalid collaboration handoff")
	}
	if err := ValidateCollaborationHandoff(CollaborationHandoffEnvelope(w)); err != nil {
		return err
	}
	*v = CollaborationHandoffEnvelope(w)
	return nil
}

func (v *CollaborationHandoffAuthority) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffAuthority
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "status"); err != nil {
		return err
	}
	*v = CollaborationHandoffAuthority(w)
	return nil
}

func (v *CollaborationHandoffAssignment) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffAssignment
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "role", "step_key"); err != nil {
		return err
	}
	*v = CollaborationHandoffAssignment(w)
	return nil
}

func (v *CollaborationHandoffTransfer) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffTransfer
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "id", "author_id", "from_id", "to_id", "purpose", "event_seq", "source_id"); err != nil {
		return err
	}
	*v = CollaborationHandoffTransfer(w)
	return nil
}

func (v *CollaborationHandoffMemory) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffMemory
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "version", "availability"); err != nil {
		return err
	}
	*v = CollaborationHandoffMemory(w)
	return nil
}

// Decode concrete pages, not the embedded metadata: an embedded UnmarshalJSON
// method would consume the entire page and hide its items/completeness fields.
func (v *CollaborationHandoffInputs) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffInputs
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "total", "returned", "truncated", "items"); err != nil {
		return err
	}
	*v = CollaborationHandoffInputs(w)
	return nil
}

func (v *CollaborationHandoffInput) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffInput
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "command_id", "author_id", "state", "reader_run_ids", "readers_truncated"); err != nil {
		return err
	}
	*v = CollaborationHandoffInput(w)
	return nil
}

func (v *CollaborationHandoffResults) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffResults
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "total", "returned", "truncated", "items"); err != nil {
		return err
	}
	*v = CollaborationHandoffResults(w)
	return nil
}

func (v *CollaborationHandoffResult) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffResult
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "run_id", "role", "step_key", "progress", "proof_state", "recording_incomplete", "result_state", "final_answer_state"); err != nil {
		return err
	}
	*v = CollaborationHandoffResult(w)
	return nil
}

func (v *CollaborationHandoffCauses) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffCauses
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "total", "returned", "truncated", "items", "completeness"); err != nil {
		return err
	}
	*v = CollaborationHandoffCauses(w)
	return nil
}

func (v *CollaborationHandoffCause) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffCause
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "id", "source_kind", "source_id", "source_run_id", "sequence"); err != nil {
		return err
	}
	*v = CollaborationHandoffCause(w)
	return nil
}

func (v *CollaborationHandoffSource) UnmarshalJSON(raw []byte) error {
	type wire CollaborationHandoffSource
	var w wire
	if err := decodeCollaborationHostObject(raw, &w, "kind", "id", "sha256", "content", "truncated", "availability", "trust"); err != nil {
		return err
	}
	*v = CollaborationHandoffSource(w)
	return nil
}

func ValidateCollaborationHandoff(v CollaborationHandoffEnvelope) error {
	invalid := errors.New("invalid collaboration handoff")
	optionalID := func(s string) bool { return s == "" || preparationUUID(s) }
	if v.ProtocolVersion != 1 || !preparationUUID(v.IssueID) || !preparationUUID(v.RunID) || !optionalID(v.SnapshotID) || !HandoffDecimal(v.EventSequence, true) || (v.Mode != "live" && v.Mode != "snapshot") || v.Mode == "snapshot" && v.SnapshotID == "" || v.Authority.Status == "" || !optionalID(v.Authority.HandlerID) || !optionalID(v.Authority.CreatorID) || v.Sources == nil || v.Missing == nil {
		return invalid
	}
	if v.Recording != nil && ValidateCollaborationRecordingRetention(*v.Recording) != nil {
		return invalid
	}
	switch v.Assignment.Role {
	case "agent", "leader", "member", "continuation":
	default:
		return invalid
	}
	if !optionalID(v.Assignment.ParentRunID) || !optionalID(v.Assignment.DelegationID) || !optionalID(v.Assignment.SourceID) {
		return invalid
	}
	for _, t := range []*CollaborationHandoffTransfer{v.Handoff, v.InformationRequest} {
		if t != nil && (!preparationUUID(t.ID) || !preparationUUID(t.AuthorID) || !preparationUUID(t.FromID) || !preparationUUID(t.ToID) || !optionalID(t.RequesterID) || !HandoffDecimal(t.EventSequence, true) || t.SourceID != t.ID) {
			return invalid
		}
	}
	if !HandoffDecimal(v.Memory.Version, true) {
		return invalid
	}
	switch v.Memory.Availability {
	case "available":
		if !HandoffDigest(v.Memory.SHA256) {
			return invalid
		}
	case "absent", "unsupported", "purged":
	default:
		return invalid
	}
	page := func(p CollaborationHandoffPage, n int) bool {
		return n == p.Returned && p.Total >= n && p.Returned <= 32 && p.Truncated == (p.NextCursor != "")
	}
	if v.Inputs.Items == nil || v.Results.Items == nil || v.WakeupCauses.Items == nil || !page(v.Inputs.CollaborationHandoffPage, len(v.Inputs.Items)) || !page(v.Results.CollaborationHandoffPage, len(v.Results.Items)) || !page(v.WakeupCauses.CollaborationHandoffPage, len(v.WakeupCauses.Items)) {
		return invalid
	}
	if v.WakeupCauses.Completeness != "recorded" && v.WakeupCauses.Completeness != "unknown" {
		return invalid
	}
	seen := map[string]bool{}
	for _, i := range v.Inputs.Items {
		if !preparationUUID(i.CommandID) || !preparationUUID(i.AuthorID) || seen[i.CommandID] || i.ReaderRunIDs == nil || len(i.ReaderRunIDs) > 32 || i.ReadersTruncated && i.ReadersCursor == "" {
			return invalid
		}
		seen[i.CommandID] = true
		rseen := map[string]bool{}
		for _, id := range i.ReaderRunIDs {
			if !preparationUUID(id) || rseen[id] {
				return invalid
			}
			rseen[id] = true
		}
	}
	seen = map[string]bool{}
	for _, r := range v.Results.Items {
		if !preparationUUID(r.RunID) || seen[r.RunID] || !optionalID(r.ParentRunID) || !optionalID(r.SourceID) || (r.FinalAnswerState != "unknown" && r.FinalAnswerState != "absent") || (r.ResultState != "present" && r.ResultState != "absent") {
			return invalid
		}
		seen[r.RunID] = true
	}
	seen = map[string]bool{}
	for _, c := range v.WakeupCauses.Items {
		if !preparationUUID(c.ID) || !preparationUUID(c.SourceID) || !preparationUUID(c.SourceRunID) || !HandoffDecimal(c.Sequence, false) || seen[c.ID] {
			return invalid
		}
		seen[c.ID] = true
		switch c.SourceKind {
		case "comment", "input", "proof":
		default:
			return invalid
		}
	}
	seen = map[string]bool{}
	for _, s := range v.Sources {
		if !preparationUUID(s.ID) || !optionalID(s.RunID) || !HandoffDigest(s.SHA256) || seen[s.Kind+":"+s.ID] || !utf8.ValidString(s.Content) || strings.ContainsRune(s.Content, 0) {
			return invalid
		}
		seen[s.Kind+":"+s.ID] = true
		switch s.Kind {
		case "handoff", "delegation", "provider_result", "squad_activity":
		default:
			return invalid
		}
		if s.Trust != "data_only" && !(s.Kind == "delegation" && s.Trust == "admitted_assignment") {
			return invalid
		}
		if s.Availability != "available" && s.Availability != "missing" && s.Availability != "purged" {
			return invalid
		}
	}
	material, wire, err := CollaborationHandoffSizes(v)
	if err != nil || material > CollaborationHandoffMaxBytes || wire > CollaborationHandoffMaxJSONBytes {
		return invalid
	}
	return nil
}

// CollaborationHandoffSizes uses the same JSON serialization as HTTP and prompt
// construction. Only source content is charged by decoded bytes; metadata,
// references and JSON framing are charged by their actual serialized bytes.
func CollaborationHandoffSizes(v CollaborationHandoffEnvelope) (material, wire int, err error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0, 0, err
	}
	wire = len(raw) + 1 // HTTP writeJSON appends one newline.
	material = wire
	for _, source := range v.Sources {
		encoded, err := json.Marshal(source.Content)
		if err != nil {
			return 0, 0, err
		}
		material -= len(encoded) - 2 - len(source.Content)
	}
	return material, wire, nil
}
