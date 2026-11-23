package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/iteration"
)

const DeliveryCompletionContract = "delivery-completion-enforcement-v1"

// DeliveryCompletionError is a content-free domain refusal, not an observation.
type DeliveryCompletionError struct {
	Status int
	Code   string
}

func (e DeliveryCompletionError) Error() string { return e.Code }
func completionInvalid() error {
	return DeliveryCompletionError{400, "invalid_delivery_completion_request"}
}
func completionDenied() error   { return DeliveryCompletionError{403, "delivery_completion_denied"} }
func completionConflict() error { return DeliveryCompletionError{409, "delivery_completion_conflict"} }
func completionUnsupported() error {
	return DeliveryCompletionError{409, "delivery_completion_consumer_unsupported"}
}

// DeliveryCompletionBinding refers to one immutable authorization and its original operation.
type DeliveryCompletionBinding struct {
	AuthorizationID       string `json:"authorization_id"`
	OperationID           string `json:"operation_id"`
	ExpectedIssueRevision int64  `json:"expected_issue_revision"`
}

func (b DeliveryCompletionBinding) Validate() error {
	if !completionUUID(b.AuthorizationID) || !completionUUID(b.OperationID) || b.ExpectedIssueRevision < 0 {
		return completionInvalid()
	}
	return nil
}
func ParseDeliveryCompletionBinding(raw []byte) (DeliveryCompletionBinding, error) {
	var b DeliveryCompletionBinding
	if _, err := completionObject(raw, []string{"authorization_id", "operation_id", "expected_issue_revision"}, nil); err != nil {
		return b, err
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return b, completionInvalid()
	}
	return b, b.Validate()
}

type DeliveryCompletionTarget struct {
	Kind                          string   `json:"kind"`
	IssueID                       string   `json:"issue_id"`
	ExpectedIssueRevision         int64    `json:"expected_issue_revision"`
	ExpectedStatus                string   `json:"expected_status"`
	TargetStatus                  string   `json:"target_status"`
	TargetPosition                *float64 `json:"target_position"`
	ExpectedCollaborationRevision string   `json:"expected_collaboration_revision,omitempty"`
	ExpectedLifecycleEpoch        string   `json:"expected_lifecycle_epoch,omitempty"`
	ExpectedParticipantEpoch      string   `json:"expected_participant_epoch,omitempty"`
	ExpectedAccountEpoch          string   `json:"expected_account_epoch,omitempty"`
	StatusOptionsVersion          string   `json:"status_options_version,omitempty"`
	SubmissionID                  *string  `json:"submission_id,omitempty"`
	AcceptanceNote                string   `json:"acceptance_note,omitempty"`
	Result                        string   `json:"result,omitempty"`
	Basis                         string   `json:"basis,omitempty"`
	DraftID                       *string  `json:"draft_id,omitempty"`
	AttachmentIDs                 []string `json:"attachment_ids,omitempty"`
	SourceCommentID               *string  `json:"source_comment_id,omitempty"`
	AdministrationReason          string   `json:"administration_reason,omitempty"`
}

var completionTargetCommon = []string{"kind", "issue_id", "expected_issue_revision", "expected_status", "target_status"}

func completionTargetFields(kind string) ([]string, []string) {
	fields := append([]string{}, completionTargetCommon...)
	var nullable []string
	switch kind {
	case "ordinary":
		fields = append(fields, "target_position")
		nullable = []string{"target_position"}
	case "collaboration_submission":
		fields = append(fields, "expected_collaboration_revision", "expected_lifecycle_epoch", "expected_participant_epoch", "expected_account_epoch", "status_options_version", "submission_id", "acceptance_note")
	case "collaboration_self":
		fields = append(fields, "expected_collaboration_revision", "expected_lifecycle_epoch", "expected_participant_epoch", "expected_account_epoch", "status_options_version", "result", "basis", "draft_id", "attachment_ids", "source_comment_id")
		nullable = []string{"draft_id", "source_comment_id"}
	case "collaboration_admin":
		fields = append(fields, "expected_collaboration_revision", "expected_lifecycle_epoch", "status_options_version", "submission_id", "administration_reason")
		nullable = []string{"submission_id"}
	default:
		return nil, nil
	}
	return fields, nullable
}
func (v *DeliveryCompletionTarget) UnmarshalJSON(raw []byte) error {
	var kind struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return completionInvalid()
	}
	fields, nullable := completionTargetFields(kind.Kind)
	if len(fields) == 0 {
		return completionInvalid()
	}
	if _, err := completionObject(raw, fields, nullable); err != nil {
		return err
	}
	type plain DeliveryCompletionTarget
	var out plain
	if json.Unmarshal(raw, &out) != nil {
		return completionInvalid()
	}
	*v = DeliveryCompletionTarget(out)
	return v.Validate()
}
func (v DeliveryCompletionTarget) MarshalJSON() ([]byte, error) {
	// Map projection follows the closed variant, including required explicit nulls.
	type plain DeliveryCompletionTarget
	raw, err := json.Marshal(plain(v))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	allowed, _ := completionTargetFields(v.Kind)
	out := make(map[string]json.RawMessage, len(allowed))
	for _, key := range allowed {
		if val, ok := fields[key]; ok {
			out[key] = val
		} else {
			switch key {
			case "acceptance_note", "result", "basis", "administration_reason":
				out[key] = json.RawMessage(`""`)
			case "attachment_ids":
				out[key] = json.RawMessage(`[]`)
			default:
				out[key] = json.RawMessage(`null`)
			}
		}
	}
	return json.Marshal(out)
}
func (v DeliveryCompletionTarget) Validate() error {
	if !completionUUID(v.IssueID) || v.ExpectedIssueRevision < 0 || !completionStatus(v.ExpectedStatus) || !completionStatus(v.TargetStatus) {
		return completionInvalid()
	}
	if v.Kind == "ordinary" {
		if v.TargetPosition != nil && (math.IsNaN(*v.TargetPosition) || math.IsInf(*v.TargetPosition, 0)) {
			return completionInvalid()
		}
		return nil
	}
	if !completionEpoch(v.ExpectedCollaborationRevision) || !completionEpoch(v.ExpectedLifecycleEpoch) || v.StatusOptionsVersion == "" || len(v.StatusOptionsVersion) > 256 {
		return completionInvalid()
	}
	if v.Kind != "collaboration_admin" && (!completionEpoch(v.ExpectedParticipantEpoch) || !completionEpoch(v.ExpectedAccountEpoch)) {
		return completionInvalid()
	}
	switch v.Kind {
	case "collaboration_submission":
		if v.SubmissionID == nil || !completionUUID(*v.SubmissionID) || !completionProse(v.AcceptanceNote, false) {
			return completionInvalid()
		}
	case "collaboration_self":
		if !completionProse(v.Result, true) || !completionProse(v.Basis, true) || !completionOptionalUUID(v.DraftID) || !completionOptionalUUID(v.SourceCommentID) || len(v.AttachmentIDs) > 64 {
			return completionInvalid()
		}
		seen := map[string]bool{}
		for _, id := range v.AttachmentIDs {
			if !completionUUID(id) || seen[id] {
				return completionInvalid()
			}
			seen[id] = true
		}
	case "collaboration_admin":
		if !completionOptionalUUID(v.SubmissionID) || !completionReason(v.AdministrationReason) {
			return completionInvalid()
		}
	default:
		return completionInvalid()
	}
	return nil
}

type DeliveryCompletionConsumer struct {
	Kind                  string `json:"kind"`
	UserID                string `json:"user_id,omitempty"`
	AgentID               string `json:"agent_id,omitempty"`
	TaskID                string `json:"task_id,omitempty"`
	TaskGeneration        int64  `json:"task_generation,omitempty"`
	InstallationID        string `json:"installation_id,omitempty"`
	ConnectionID          string `json:"connection_id,omitempty"`
	ExpectedTrustRevision int64  `json:"expected_trust_revision,omitempty"`
	PullRequestID         string `json:"pull_request_id,omitempty"`
	OrganizationID        string `json:"organization_id,omitempty"`
}

func completionConsumerFields(kind string) []string {
	switch kind {
	case "human":
		return []string{"kind", "user_id"}
	case "task":
		return []string{"kind", "agent_id", "task_id", "task_generation"}
	case "github":
		return []string{"kind", "installation_id", "expected_trust_revision", "pull_request_id"}
	case "vcs":
		return []string{"kind", "connection_id", "expected_trust_revision", "pull_request_id"}
	case "admin":
		return []string{"kind", "user_id", "organization_id"}
	}
	return nil
}
func (v *DeliveryCompletionConsumer) UnmarshalJSON(raw []byte) error {
	var kind struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return completionInvalid()
	}
	fields := completionConsumerFields(kind.Kind)
	if len(fields) == 0 {
		return completionInvalid()
	}
	if _, err := completionObject(raw, fields, nil); err != nil {
		return err
	}
	type plain DeliveryCompletionConsumer
	var out plain
	if json.Unmarshal(raw, &out) != nil {
		return completionInvalid()
	}
	*v = DeliveryCompletionConsumer(out)
	return v.Validate()
}
func (v DeliveryCompletionConsumer) Validate() error {
	switch v.Kind {
	case "human":
		if !completionUUID(v.UserID) {
			return completionInvalid()
		}
	case "task":
		if !completionUUID(v.AgentID) || !completionUUID(v.TaskID) || v.TaskGeneration < 0 {
			return completionInvalid()
		}
	case "github":
		if !completionUUID(v.InstallationID) || !completionUUID(v.PullRequestID) || v.ExpectedTrustRevision < 1 {
			return completionInvalid()
		}
	case "vcs":
		if !completionUUID(v.ConnectionID) || !completionUUID(v.PullRequestID) || v.ExpectedTrustRevision < 1 {
			return completionInvalid()
		}
	case "admin":
		if !completionUUID(v.UserID) || !completionUUID(v.OrganizationID) {
			return completionInvalid()
		}
	default:
		return completionInvalid()
	}
	return nil
}
func (v DeliveryCompletionConsumer) MarshalJSON() ([]byte, error) {
	type plain DeliveryCompletionConsumer
	raw, err := json.Marshal(plain(v))
	if err != nil {
		return nil, err
	}
	var all map[string]json.RawMessage
	if err = json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	for _, key := range completionConsumerFields(v.Kind) {
		if val, ok := all[key]; ok {
			out[key] = val
		} else if key == "task_generation" {
			out[key] = json.RawMessage(`0`)
		} else {
			return nil, completionInvalid()
		}
	}
	return json.Marshal(out)
}

type DeliveryCompletionProject struct {
	ID                     string `json:"id"`
	ExpectedRevision       int64  `json:"expected_revision"`
	ExpectedPolicyRevision int64  `json:"expected_policy_revision"`
}
type DeliveryCompletionEvidence struct {
	IssueID                string `json:"issue_id"`
	ExpectedIssueRevision  int64  `json:"expected_issue_revision"`
	ApprovalID             string `json:"approval_id"`
	ExpectedSourceRevision string `json:"expected_source_revision"`
	ExpectedSpecDigest     string `json:"expected_spec_digest"`
	ExpectedArtifactDigest string `json:"expected_artifact_digest"`
}
type DeliveryCompletionAuthorizationRequest struct {
	ContractVersion   string                     `json:"contract_version"`
	SourceWorkspaceID string                     `json:"source_workspace_id"`
	OperationID       string                     `json:"operation_id"`
	Target            DeliveryCompletionTarget   `json:"target"`
	Project           DeliveryCompletionProject  `json:"project"`
	Evidence          DeliveryCompletionEvidence `json:"evidence"`
	Consumer          DeliveryCompletionConsumer `json:"consumer"`
	ExpiresAt         time.Time                  `json:"expires_at"`
	Reason            string                     `json:"reason"`
}

func ParseDeliveryCompletionAuthorization(raw []byte) (DeliveryCompletionAuthorizationRequest, error) {
	var v DeliveryCompletionAuthorizationRequest
	if iteration.ValidateObjectJSON(raw) == nil {
		var explicit map[string]json.RawMessage
		if json.Unmarshal(raw, &explicit) == nil {
			if _, exists := explicit["skip"]; exists {
				return v, DeliveryCompletionError{409, "delivery_skip_not_supported"}
			}
		}
	}
	fields, err := completionObject(raw, []string{"contract_version", "source_workspace_id", "operation_id", "target", "project", "evidence", "consumer", "expires_at", "reason"}, nil)
	if err != nil {
		return v, err
	}
	if _, err = completionObject(fields["project"], []string{"id", "expected_revision", "expected_policy_revision"}, nil); err != nil {
		return v, err
	}
	if _, err = completionObject(fields["evidence"], []string{"issue_id", "expected_issue_revision", "approval_id", "expected_source_revision", "expected_spec_digest", "expected_artifact_digest"}, nil); err != nil {
		return v, err
	}
	var expiry string
	if json.Unmarshal(fields["expires_at"], &expiry) != nil || !strings.HasSuffix(expiry, "Z") {
		return v, completionInvalid()
	}
	if json.Unmarshal(raw, &v) != nil {
		return v, completionInvalid()
	}
	v.Reason = strings.TrimSpace(v.Reason)
	v.ExpiresAt = v.ExpiresAt.UTC().Truncate(time.Microsecond)
	return v, v.Validate()
}
func (v DeliveryCompletionAuthorizationRequest) Validate() error {
	if v.ContractVersion != DeliveryCompletionContract || !completionUUID(v.SourceWorkspaceID) || !completionUUID(v.OperationID) || v.Target.Validate() != nil || v.Consumer.Validate() != nil || !completionReason(v.Reason) || v.ExpiresAt.IsZero() || v.ExpiresAt.Location() != time.UTC || v.ExpiresAt.Nanosecond()%1000 != 0 {
		return completionInvalid()
	}
	if !completionUUID(v.Project.ID) || v.Project.ExpectedRevision < 0 || v.Project.ExpectedPolicyRevision < 1 {
		return completionInvalid()
	}
	e := v.Evidence
	if !completionUUID(e.IssueID) || e.ExpectedIssueRevision < 0 || !completionUUID(e.ApprovalID) || !DeliveryHex(e.ExpectedSourceRevision, 40, 64) || !DeliveryHex(e.ExpectedSpecDigest, 64) || !DeliveryHex(e.ExpectedArtifactDigest, 64) {
		return completionInvalid()
	}
	if v.Target.Kind == "ordinary" {
		if e.IssueID != v.Target.IssueID || e.ExpectedIssueRevision != v.Target.ExpectedIssueRevision || v.Consumer.Kind == "admin" {
			return completionInvalid()
		}
	} else {
		if e.IssueID == v.Target.IssueID {
			return completionInvalid()
		}
		if v.Target.Kind == "collaboration_admin" {
			if v.Consumer.Kind != "admin" {
				return completionInvalid()
			}
		} else if v.Consumer.Kind != "human" {
			return completionInvalid()
		}
	}
	return nil
}
func (v DeliveryCompletionAuthorizationRequest) Digest() string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return completionDigest("request", raw)
}
func completionDigest(kind string, raw []byte) string {
	sum := sha256.Sum256(append([]byte("multica-delivery-completion-"+kind+"-v1\x00"), raw...))
	return hex.EncodeToString(sum[:])
}
func completionObject(raw []byte, keys, nullable []string) (map[string]json.RawMessage, error) {
	var v map[string]json.RawMessage
	if err := iteration.ValidateObjectJSON(raw); err != nil {
		return nil, completionInvalid()
	}
	if json.Unmarshal(raw, &v) != nil || v == nil || len(v) != len(keys) {
		return nil, completionInvalid()
	}
	for _, key := range keys {
		val, ok := v[key]
		if !ok {
			return nil, completionInvalid()
		}
		if bytes.Equal(bytes.TrimSpace(val), []byte("null")) {
			allow := false
			for _, n := range nullable {
				if n == key {
					allow = true
				}
			}
			if !allow {
				return nil, completionInvalid()
			}
		}
	}
	return v, nil
}
func completionUUID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u != uuid.Nil && u.String() == s
}
func completionOptionalUUID(s *string) bool { return s == nil || completionUUID(*s) }
func completionEpoch(s string) bool {
	n, e := strconv.ParseInt(s, 10, 64)
	return e == nil && n > 0 && strconv.FormatInt(n, 10) == s
}
func completionStatus(s string) bool {
	if issuestatus.IsBuiltIn(s) {
		return true
	}
	normalized, err := issuestatus.ValidateKey(s)
	return s != "" && len(s) <= 128 && err == nil && normalized == s
}
func completionProse(s string, required bool) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && len(s) <= 65536 && (!required || strings.TrimSpace(s) != "")
}
func completionReason(s string) bool {
	return completionProse(s, true) && utf8.RuneCountInString(s) <= 1000 && strings.TrimSpace(s) == s
}
