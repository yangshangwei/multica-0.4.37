package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CollaborationKnowledgeVersion                 = 1
	CollaborationKnowledgeMaxContentBytes         = 64 << 10
	CollaborationKnowledgeMaxSummaryBytes         = 2 << 10
	CollaborationKnowledgeMaxReferences           = 32
	CollaborationKnowledgeMaxMetadataBytes        = 16 << 10
	CollaborationKnowledgeMaxRequestBytes         = 6*(CollaborationKnowledgeMaxContentBytes+CollaborationKnowledgeMaxSummaryBytes+CollaborationKnowledgeMaxMetadataBytes) + 16<<10
	CollaborationKnowledgeMaxReasonBytes          = 1000
	CollaborationKnowledgeMaxRecordIDBytes        = 256
	CollaborationKnowledgeMaxReferenceBytes       = 2048
	CollaborationKnowledgeMaxTargetItems          = 16
	CollaborationKnowledgeMaxTargetBytes          = 128 << 10
	CollaborationKnowledgeMaxRuntimeItems         = 32
	CollaborationKnowledgeMaxRuntimeBytes         = 256 << 10
	CollaborationKnowledgeMaxRuntimeJSONBytes     = 2 << 20
	CollaborationKnowledgeMaxLaunchAuthorizations = 32
	CollaborationKnowledgeMaxDeliveryBytes        = 64 << 10
	CollaborationKnowledgeDefaultPageSize         = 20
	CollaborationKnowledgeMaxPageSize             = 50
)

var (
	ErrInvalidCollaborationKnowledge          = errors.New("invalid_knowledge_request")
	ErrCollaborationKnowledgeRevisionRequired = errors.New("knowledge_revision_required")
	ErrCollaborationKnowledgeTooLarge         = errors.New("knowledge_candidate_too_large")
	ErrCollaborationKnowledgeTargetBudget     = errors.New("knowledge_target_budget_exceeded")
)

type CollaborationKnowledgeTarget struct {
	Kind        string `json:"kind"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id,omitempty"`
}

// All required sources are immutable references to the same collaboration
// issue. The service must resolve their current availability before disclosure.
type CollaborationKnowledgeSource struct {
	Kind      string `json:"kind"`
	IssueID   string `json:"issue_id"`
	Version   string `json:"version,omitempty"`
	SHA256    string `json:"sha256"`
	CommentID string `json:"comment_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	RecordID  string `json:"record_id,omitempty"`
}

// Verification is the proposer's attributed claim, never server verification.
// Commit/test references are inert text; no URL or path is dereferenced.
type CollaborationKnowledgeEvidence struct {
	Kind         string                        `json:"kind"`
	RunID        string                        `json:"run_id,omitempty"`
	RecordID     string                        `json:"record_id,omitempty"`
	Source       *CollaborationKnowledgeSource `json:"source,omitempty"`
	Reference    string                        `json:"reference,omitempty"`
	Verification string                        `json:"verification"`
}

type CollaborationKnowledgeApplicability struct {
	Surfaces []string `json:"surfaces"`
}

type CollaborationKnowledgeProposal struct {
	ProtocolVersion        int                                 `json:"protocol_version"`
	CandidateID            string                              `json:"candidate_id"`
	RequestID              string                              `json:"request_id"`
	Target                 CollaborationKnowledgeTarget        `json:"target"`
	Summary                string                              `json:"summary"`
	Content                string                              `json:"content"`
	Sources                []CollaborationKnowledgeSource      `json:"sources"`
	Evidence               []CollaborationKnowledgeEvidence    `json:"evidence"`
	Applicability          CollaborationKnowledgeApplicability `json:"applicability"`
	ExpiresAt              *time.Time                          `json:"expires_at"`
	SupersedesCandidateID  *string                             `json:"supersedes_candidate_id"`
	DerivedFromCandidateID *string                             `json:"derived_from_candidate_id"`
}

type CollaborationKnowledgeApproveRequest struct {
	ProtocolVersion       int    `json:"protocol_version"`
	RequestID             string `json:"request_id"`
	ExpectedStateRevision string `json:"expected_state_revision"`
	ExpectedVersion       string `json:"expected_version"`
	ExpectedContentSHA256 string `json:"expected_content_sha256"`
}

type CollaborationKnowledgeDecisionRequest struct {
	ProtocolVersion       int    `json:"protocol_version"`
	RequestID             string `json:"request_id"`
	ExpectedStateRevision string `json:"expected_state_revision"`
	Reason                string `json:"reason"`
}

// Receipts never contain source IDs, text, decision reasons or references.
type CollaborationKnowledgeReceipt struct {
	ProtocolVersion         int     `json:"protocol_version"`
	RequestID               string  `json:"request_id"`
	Action                  string  `json:"action"`
	Code                    string  `json:"code"`
	CandidateID             string  `json:"candidate_id"`
	Version                 string  `json:"version"`
	StateRevision           string  `json:"state_revision"`
	State                   string  `json:"state"`
	TargetKnowledgeRevision *string `json:"target_knowledge_revision,omitempty"`
}

// This is the complete public invalidation payload. Internal outbox routing,
// cause and candidate IDs must never be serialized into it.
type CollaborationKnowledgeTargetHint struct {
	TargetKind        string `json:"target_kind"`
	TargetID          string `json:"target_id"`
	KnowledgeRevision string `json:"knowledge_revision"`
}

// Contributions are assembled only after source/target/consumer authorization.
// Budget checks charge all projection fields, not just content.
type CollaborationKnowledgeContribution struct {
	CandidateID   string                              `json:"candidate_id"`
	Version       string                              `json:"version"`
	ContentSHA256 string                              `json:"content_sha256"`
	Target        CollaborationKnowledgeTarget        `json:"target"`
	SourceIssueID string                              `json:"source_issue_id"`
	Summary       string                              `json:"summary"`
	Content       string                              `json:"content"`
	Sources       []CollaborationKnowledgeSource      `json:"sources"`
	Evidence      []CollaborationKnowledgeEvidence    `json:"evidence"`
	Applicability CollaborationKnowledgeApplicability `json:"applicability"`
	ExpiresAt     *time.Time                          `json:"expires_at"`
}

func CollaborationKnowledgeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func knowledgeText(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func decodeKnowledgeObject(raw []byte, value any, required ...string) error {
	if !utf8.Valid(raw) || decodeCollaborationHostObject(raw, value, required...) != nil {
		return ErrInvalidCollaborationKnowledge
	}
	return nil
}

func knowledgeAbsent(raw []byte, fields ...string) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, field := range fields {
		if _, ok := values[field]; ok {
			return false
		}
	}
	return true
}

func (v *CollaborationKnowledgeTarget) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeTarget
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "kind", "workspace_id"); err != nil {
		return err
	}
	if w.Kind == "workspace" && !knowledgeAbsent(raw, "project_id") {
		return ErrInvalidCollaborationKnowledge
	}
	if !validKnowledgeTarget(CollaborationKnowledgeTarget(w)) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeTarget(w)
	return nil
}

func validKnowledgeTarget(v CollaborationKnowledgeTarget) bool {
	return preparationUUID(v.WorkspaceID) && (v.Kind == "workspace" && v.ProjectID == "" || v.Kind == "project" && preparationUUID(v.ProjectID))
}

func (v *CollaborationKnowledgeSource) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeSource
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "kind", "issue_id", "sha256"); err != nil {
		return err
	}
	switch w.Kind {
	case "memory":
		if !knowledgeAbsent(raw, "comment_id", "run_id", "record_id") {
			return ErrInvalidCollaborationKnowledge
		}
	case "comment":
		if !knowledgeAbsent(raw, "run_id", "record_id") {
			return ErrInvalidCollaborationKnowledge
		}
	case "run_record":
		if !knowledgeAbsent(raw, "comment_id", "version") {
			return ErrInvalidCollaborationKnowledge
		}
	}
	if !validKnowledgeSource(CollaborationKnowledgeSource(w)) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeSource(w)
	return nil
}

func validKnowledgeSource(v CollaborationKnowledgeSource) bool {
	if !preparationUUID(v.IssueID) || !preparationHex(v.SHA256, 64) {
		return false
	}
	switch v.Kind {
	case "memory":
		return preparationVersion(v.Version) && v.CommentID == "" && v.RunID == "" && v.RecordID == ""
	case "comment":
		return preparationVersion(v.Version) && preparationUUID(v.CommentID) && v.RunID == "" && v.RecordID == ""
	case "run_record":
		return v.Version == "" && v.CommentID == "" && preparationUUID(v.RunID) && knowledgeText(v.RecordID, CollaborationKnowledgeMaxRecordIDBytes)
	default:
		return false
	}
}

func (v *CollaborationKnowledgeEvidence) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeEvidence
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "kind", "verification"); err != nil {
		return err
	}
	if w.Kind == "run_record" && !knowledgeAbsent(raw, "source", "reference") || w.Kind != "run_record" && !knowledgeAbsent(raw, "run_id", "record_id") {
		return ErrInvalidCollaborationKnowledge
	}
	if !validKnowledgeEvidence(CollaborationKnowledgeEvidence(w)) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeEvidence(w)
	return nil
}

func validKnowledgeEvidence(v CollaborationKnowledgeEvidence) bool {
	if v.Verification != "unverified" && v.Verification != "observed" && v.Verification != "verified" {
		return false
	}
	switch v.Kind {
	case "run_record":
		return preparationUUID(v.RunID) && knowledgeText(v.RecordID, CollaborationKnowledgeMaxRecordIDBytes) && v.Source == nil && v.Reference == ""
	case "commit", "test":
		return v.RunID == "" && v.RecordID == "" && v.Source != nil && validKnowledgeSource(*v.Source) && knowledgeText(v.Reference, CollaborationKnowledgeMaxReferenceBytes)
	default:
		return false
	}
}

func (v *CollaborationKnowledgeApplicability) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeApplicability
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "surfaces"); err != nil {
		return err
	}
	if !validKnowledgeApplicability(CollaborationKnowledgeApplicability(w)) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeApplicability(w)
	return nil
}

func validKnowledgeApplicability(v CollaborationKnowledgeApplicability) bool {
	if len(v.Surfaces) == 0 || len(v.Surfaces) > 5 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range v.Surfaces {
		switch s {
		case "issue", "chat", "quick_create", "autopilot", "collaboration":
		default:
			return false
		}
		if seen[s] {
			return false
		}
		seen[s] = true
	}
	return true
}

func (v *CollaborationKnowledgeProposal) UnmarshalJSON(raw []byte) error {
	if len(raw) > CollaborationKnowledgeMaxRequestBytes {
		return ErrCollaborationKnowledgeTooLarge
	}
	type wire CollaborationKnowledgeProposal
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "candidate_id", "request_id", "target", "summary", "content", "sources", "evidence", "applicability"); err != nil {
		return err
	}
	if !collaborationRecallHasFields(raw, "expires_at", "supersedes_candidate_id", "derived_from_candidate_id") {
		return ErrInvalidCollaborationKnowledge
	}
	if err := ValidateCollaborationKnowledgeProposal(CollaborationKnowledgeProposal(w), ""); err != nil {
		return err
	}
	*v = CollaborationKnowledgeProposal(w)
	return nil
}

// sourceIssueID comes from the authenticated route, never a workspace header.
// The service rechecks expiry against its post-lock transaction time.
func ValidateCollaborationKnowledgeProposal(v CollaborationKnowledgeProposal, sourceIssueID string) error {
	if v.ProtocolVersion != CollaborationKnowledgeVersion || !preparationUUID(v.CandidateID) || !preparationUUID(v.RequestID) || !validKnowledgeTarget(v.Target) || !validKnowledgeApplicability(v.Applicability) {
		return ErrInvalidCollaborationKnowledge
	}
	if len(v.Content) > CollaborationKnowledgeMaxContentBytes || len(v.Summary) > CollaborationKnowledgeMaxSummaryBytes || len(v.Sources)+len(v.Evidence) > CollaborationKnowledgeMaxReferences {
		return ErrCollaborationKnowledgeTooLarge
	}
	if !knowledgeText(v.Content, CollaborationKnowledgeMaxContentBytes) || !knowledgeText(v.Summary, CollaborationKnowledgeMaxSummaryBytes) || len(v.Sources) == 0 || v.Evidence == nil {
		return ErrInvalidCollaborationKnowledge
	}
	for _, id := range []*string{v.SupersedesCandidateID, v.DerivedFromCandidateID} {
		if id != nil && (!preparationUUID(*id) || *id == v.CandidateID) {
			return ErrInvalidCollaborationKnowledge
		}
	}
	if v.SupersedesCandidateID != nil && v.DerivedFromCandidateID != nil {
		return ErrInvalidCollaborationKnowledge
	}
	if v.ExpiresAt != nil {
		_, offset := v.ExpiresAt.Zone()
		if offset != 0 || v.ExpiresAt.IsZero() {
			return ErrInvalidCollaborationKnowledge
		}
	}
	if sourceIssueID == "" {
		sourceIssueID = v.Sources[0].IssueID
	}
	for _, s := range v.Sources {
		if !validKnowledgeSource(s) || s.IssueID != sourceIssueID {
			return ErrInvalidCollaborationKnowledge
		}
	}
	for _, e := range v.Evidence {
		if !validKnowledgeEvidence(e) || e.Source != nil && e.Source.IssueID != sourceIssueID {
			return ErrInvalidCollaborationKnowledge
		}
		// Every evidence anchor must also be a required source so clearing it
		// suppresses any text derived from that evidence.
		found := false
		for _, s := range v.Sources {
			if e.Source != nil && s == *e.Source || e.Kind == "run_record" && s.Kind == "run_record" && s.RunID == e.RunID && s.RecordID == e.RecordID {
				found = true
				break
			}
		}
		if !found {
			return ErrInvalidCollaborationKnowledge
		}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ErrInvalidCollaborationKnowledge
	}
	var metadata map[string]any
	if json.Unmarshal(raw, &metadata) != nil {
		return ErrInvalidCollaborationKnowledge
	}
	delete(metadata, "content")
	delete(metadata, "summary")
	material, _, err := CollaborationKnowledgeSizes(metadata)
	if err != nil {
		return err
	}
	if material > CollaborationKnowledgeMaxMetadataBytes {
		return ErrCollaborationKnowledgeTooLarge
	}
	return nil
}

// New proposals validate the time after original-key recovery and authority
// locks. Decoding an exact old request must not depend on when it is retried.
func ValidateCollaborationKnowledgeProposalAt(v CollaborationKnowledgeProposal, sourceIssueID string, now time.Time) error {
	if err := ValidateCollaborationKnowledgeProposal(v, sourceIssueID); err != nil {
		return err
	}
	if v.ExpiresAt != nil && !v.ExpiresAt.After(now) {
		return ErrInvalidCollaborationKnowledge
	}
	return nil
}

func (v *CollaborationKnowledgeContribution) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeContribution
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "candidate_id", "version", "content_sha256", "target", "source_issue_id", "summary", "content", "sources", "evidence", "applicability"); err != nil {
		return err
	}
	if !collaborationRecallHasFields(raw, "expires_at") || !preparationVersion(w.Version) || !preparationUUID(w.SourceIssueID) || !preparationHex(w.ContentSHA256, 64) || w.ContentSHA256 != CollaborationKnowledgeHash(w.Content) {
		return ErrInvalidCollaborationKnowledge
	}
	p := CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: w.CandidateID, RequestID: w.CandidateID, Target: w.Target, Summary: w.Summary, Content: w.Content, Sources: w.Sources, Evidence: w.Evidence, Applicability: w.Applicability, ExpiresAt: w.ExpiresAt}
	if err := ValidateCollaborationKnowledgeProposal(p, w.SourceIssueID); err != nil {
		return err
	}
	*v = CollaborationKnowledgeContribution(w)
	return nil
}

func (v *CollaborationKnowledgeApproveRequest) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeApproveRequest
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id"); err != nil {
		return err
	}
	if !knowledgePresentValues(raw, "expected_state_revision", "expected_version", "expected_content_sha256") {
		return ErrCollaborationKnowledgeRevisionRequired
	}
	if w.ProtocolVersion != 1 || !preparationUUID(w.RequestID) || !preparationVersion(w.ExpectedStateRevision) || !preparationVersion(w.ExpectedVersion) || !preparationHex(w.ExpectedContentSHA256, 64) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeApproveRequest(w)
	return nil
}

func (v *CollaborationKnowledgeDecisionRequest) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeDecisionRequest
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id", "reason"); err != nil {
		return err
	}
	if !knowledgePresentValues(raw, "expected_state_revision") {
		return ErrCollaborationKnowledgeRevisionRequired
	}
	if w.ProtocolVersion != 1 || !preparationUUID(w.RequestID) || !preparationVersion(w.ExpectedStateRevision) || !knowledgeText(w.Reason, CollaborationKnowledgeMaxReasonBytes) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeDecisionRequest(w)
	return nil
}

func knowledgePresentValues(raw []byte, fields ...string) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, field := range fields {
		value, ok := values[field]
		if !ok || strings.TrimSpace(string(value)) == "null" {
			return false
		}
	}
	return true
}

func (v *CollaborationKnowledgeReceipt) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeReceipt
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "protocol_version", "request_id", "action", "code", "candidate_id", "version", "state_revision", "state"); err != nil {
		return err
	}
	if w.ProtocolVersion != 1 || !preparationUUID(w.RequestID) || !preparationUUID(w.CandidateID) || !preparationVersion(w.Version) || !preparationVersion(w.StateRevision) || !knowledgeText(w.Code, 128) {
		return ErrInvalidCollaborationKnowledge
	}
	switch w.Action {
	case "propose", "approve", "reject", "withdraw":
	default:
		return ErrInvalidCollaborationKnowledge
	}
	switch w.State {
	case "pending", "approved", "rejected", "withdrawn", "superseded":
	default:
		return ErrInvalidCollaborationKnowledge
	}
	if w.TargetKnowledgeRevision != nil && !knowledgeRevision(*w.TargetKnowledgeRevision) {
		return ErrInvalidCollaborationKnowledge
	}
	if w.TargetKnowledgeRevision == nil && !knowledgeAbsent(raw, "target_knowledge_revision") {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeReceipt(w)
	return nil
}

func knowledgeRevision(v string) bool { return v == "0" || preparationVersion(v) }

func (v *CollaborationKnowledgeTargetHint) UnmarshalJSON(raw []byte) error {
	type wire CollaborationKnowledgeTargetHint
	var w wire
	if err := decodeKnowledgeObject(raw, &w, "target_kind", "target_id", "knowledge_revision"); err != nil {
		return err
	}
	if w.TargetKind != "workspace" && w.TargetKind != "project" || !preparationUUID(w.TargetID) || !knowledgeRevision(w.KnowledgeRevision) {
		return ErrInvalidCollaborationKnowledge
	}
	*v = CollaborationKnowledgeTargetHint(w)
	return nil
}

// Sizes include all framing and metadata, charging strings as decoded UTF-8.
// rawWireBytes is checked separately by runtime readers before JSON decoding.
func CollaborationKnowledgeSizes(value any) (material, wire int, err error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, 0, err
	}
	// encoding/json repairs invalid Go strings. Reject them before counting
	// that repaired serialization, including nested typed reference fields.
	if !knowledgeValidStrings(reflect.ValueOf(value)) {
		return 0, 0, ErrInvalidCollaborationKnowledge
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return 0, 0, err
	}
	wire = len(raw) + 1
	material = wire
	var visit func(any) error
	visit = func(v any) error {
		switch x := v.(type) {
		case string:
			if !utf8.ValidString(x) || strings.ContainsRune(x, 0) {
				return ErrInvalidCollaborationKnowledge
			}
			encoded, _ := json.Marshal(x)
			material -= len(encoded) - 2 - len(x)
		case []any:
			for _, child := range x {
				if err := visit(child); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, child := range x {
				if err := visit(key); err != nil {
					return err
				}
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err = visit(decoded)
	return
}

func knowledgeValidStrings(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String()) && !strings.ContainsRune(v.String(), 0)
	case reflect.Pointer, reflect.Interface:
		return v.IsNil() || knowledgeValidStrings(v.Elem())
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !knowledgeValidStrings(v.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		entries := v.MapRange()
		for entries.Next() {
			if !knowledgeValidStrings(entries.Key()) || !knowledgeValidStrings(entries.Value()) {
				return false
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.PkgPath == "" && f.Tag.Get("json") != "-" && !knowledgeValidStrings(v.Field(i)) {
				return false
			}
		}
	}
	return true
}

func ValidateCollaborationKnowledgeTargetBudget(items []CollaborationKnowledgeContribution) error {
	material, _, err := CollaborationKnowledgeSizes(items)
	if err != nil {
		return err
	}
	if len(items) > CollaborationKnowledgeMaxTargetItems || material > CollaborationKnowledgeMaxTargetBytes {
		return ErrCollaborationKnowledgeTargetBudget
	}
	return nil
}

func ValidateCollaborationKnowledgeRuntimeBudget(items []CollaborationKnowledgeContribution, rawWireBytes int) error {
	material, wire, err := CollaborationKnowledgeSizes(items)
	if err != nil {
		return err
	}
	if rawWireBytes < 0 || len(items) > CollaborationKnowledgeMaxRuntimeItems || material > CollaborationKnowledgeMaxRuntimeBytes || wire > CollaborationKnowledgeMaxRuntimeJSONBytes || rawWireBytes > CollaborationKnowledgeMaxRuntimeJSONBytes {
		return ErrCollaborationKnowledgeTooLarge
	}
	return nil
}
