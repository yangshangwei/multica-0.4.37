package protocol

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	CollaborationTestReportVersion        = 1
	CollaborationTestReportJSONLimit      = 2 << 20
	CollaborationTestReportCanonicalLimit = 256 << 10
	CollaborationTestReportLogBodyLimit   = 64 << 10
)

var ErrInvalidCollaborationTestReport = errors.New("invalid collaboration test report")

// The create-only reference is bound to actual authorized command IDs by the
// server. A runtime cannot supply its own source binding.
type CollaborationTestSubjectRef struct {
	CandidateID      string `json:"candidate_id"`
	CandidateVersion string `json:"candidate_version"`
	CandidateDigest  string `json:"candidate_digest"`
}

type CollaborationTestSubject struct {
	CandidateID      string  `json:"candidate_id"`
	CandidateVersion string  `json:"candidate_version"`
	CandidateDigest  string  `json:"candidate_digest"`
	BindingDigest    string  `json:"binding_digest"`
	SourceRevision   string  `json:"source_revision"`
	ArtifactDigest   *string `json:"artifact_digest"`
}

type CollaborationTestSubjectResponse struct {
	Version int                      `json:"version"`
	IssueID string                   `json:"issue_id"`
	RunID   string                   `json:"run_id"`
	Subject CollaborationTestSubject `json:"subject"`
}

type CollaborationTestCase struct {
	ID       string `json:"id"`
	Outcome  string `json:"outcome"`
	LogIndex int    `json:"log_index"`
}

type CollaborationTestLog struct {
	Kind         string `json:"kind"`
	EventID      string `json:"event_id,omitempty"`
	MessageIndex int    `json:"message_index,omitempty"`
	BodySHA256   string `json:"body_sha256,omitempty"`
	Body         string `json:"body,omitempty"`
}

type CollaborationTestReport struct {
	Subject CollaborationTestSubject `json:"subject"`
	SuiteID string                   `json:"suite_id"`
	Cases   []CollaborationTestCase  `json:"cases"`
	Logs    []CollaborationTestLog   `json:"logs"`
}

type CollaborationTestReportRequest struct {
	Version   int                      `json:"version"`
	Identity  CollaborationRunIdentity `json:"identity"`
	RequestID string                   `json:"request_id"`
	Report    CollaborationTestReport  `json:"report"`
}

type CollaborationTestReportReceipt struct {
	Version      int    `json:"version"`
	IssueID      string `json:"issue_id"`
	RunID        string `json:"run_id"`
	RequestID    string `json:"request_id"`
	ReportID     string `json:"report_id"`
	ReportDigest string `json:"report_digest"`
	Accepted     bool   `json:"accepted"`
}

func decodeTestReportObject(raw []byte, out any, fields ...string) error {
	if !utf8.Valid(raw) || decodeCollaborationHostObject(raw, out, fields...) != nil {
		return ErrInvalidCollaborationTestReport
	}
	return nil
}

func ValidateCollaborationTestSubjectRef(v CollaborationTestSubjectRef) error {
	if !preparationUUID(v.CandidateID) || !preparationVersion(v.CandidateVersion) || !preparationHex(v.CandidateDigest, 64) {
		return ErrInvalidCollaborationTestReport
	}
	return nil
}

func (v *CollaborationTestSubjectRef) UnmarshalJSON(raw []byte) error {
	type wire CollaborationTestSubjectRef
	var w wire
	if decodeTestReportObject(raw, &w, "candidate_id", "candidate_version", "candidate_digest") != nil || ValidateCollaborationTestSubjectRef(CollaborationTestSubjectRef(w)) != nil {
		return ErrInvalidCollaborationTestReport
	}
	*v = CollaborationTestSubjectRef(w)
	return nil
}

func ValidateCollaborationTestSubject(v CollaborationTestSubject) error {
	if ValidateCollaborationTestSubjectRef(CollaborationTestSubjectRef{v.CandidateID, v.CandidateVersion, v.CandidateDigest}) != nil || !preparationHex(v.BindingDigest, 64) ||
		(!preparationHex(v.SourceRevision, 40) && !preparationHex(v.SourceRevision, 64)) || v.ArtifactDigest != nil && !preparationHex(*v.ArtifactDigest, 64) {
		return ErrInvalidCollaborationTestReport
	}
	return nil
}

func (v *CollaborationTestSubject) UnmarshalJSON(raw []byte) error {
	type wire CollaborationTestSubject
	var w wire
	if decodeTestReportObject(raw, &w, "candidate_id", "candidate_version", "candidate_digest", "binding_digest", "source_revision") != nil ||
		!collaborationRecallHasFields(raw, "artifact_digest") || ValidateCollaborationTestSubject(CollaborationTestSubject(w)) != nil {
		return ErrInvalidCollaborationTestReport
	}
	*v = CollaborationTestSubject(w)
	return nil
}

func (v *CollaborationTestCase) UnmarshalJSON(raw []byte) error {
	type wire CollaborationTestCase
	var w wire
	if decodeTestReportObject(raw, &w, "id", "outcome", "log_index") != nil {
		return ErrInvalidCollaborationTestReport
	}
	*v = CollaborationTestCase(w)
	return nil
}

func (v CollaborationTestLog) MarshalJSON() ([]byte, error) {
	if v.Kind == "inline" {
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Body string `json:"body"`
		}{v.Kind, v.Body})
	}
	return json.Marshal(struct {
		Kind         string `json:"kind"`
		EventID      string `json:"event_id"`
		MessageIndex int    `json:"message_index"`
		BodySHA256   string `json:"body_sha256"`
	}{v.Kind, v.EventID, v.MessageIndex, v.BodySHA256})
}

func (v *CollaborationTestLog) UnmarshalJSON(raw []byte) error {
	type wire CollaborationTestLog
	var w wire
	if decodeTestReportObject(raw, &w, "kind") != nil {
		return ErrInvalidCollaborationTestReport
	}
	switch w.Kind {
	case "record":
		if decodeTestReportObject(raw, &w, "event_id", "message_index", "body_sha256") != nil || !knowledgeAbsent(raw, "body") {
			return ErrInvalidCollaborationTestReport
		}
	case "inline":
		if decodeTestReportObject(raw, &w, "body") != nil || !knowledgeAbsent(raw, "event_id", "message_index", "body_sha256") {
			return ErrInvalidCollaborationTestReport
		}
	default:
		return ErrInvalidCollaborationTestReport
	}
	*v = CollaborationTestLog(w)
	return nil
}

func (v *CollaborationTestReport) UnmarshalJSON(raw []byte) error {
	type wire CollaborationTestReport
	var w wire
	if decodeTestReportObject(raw, &w, "subject", "suite_id", "cases", "logs") != nil || ValidateCollaborationTestReport(CollaborationTestReport(w)) != nil {
		return ErrInvalidCollaborationTestReport
	}
	*v = CollaborationTestReport(w)
	return nil
}

func ValidateCollaborationTestReport(v CollaborationTestReport) error {
	if ValidateCollaborationTestSubject(v.Subject) != nil || !knowledgeText(v.SuiteID, 128) || len(v.Cases) < 1 || len(v.Cases) > 256 || len(v.Logs) < 1 || len(v.Logs) > 64 {
		return ErrInvalidCollaborationTestReport
	}
	seen := make(map[string]bool, len(v.Cases))
	for _, c := range v.Cases {
		if !knowledgeText(c.ID, 256) || seen[c.ID] || c.LogIndex < 0 || c.LogIndex >= len(v.Logs) {
			return ErrInvalidCollaborationTestReport
		}
		seen[c.ID] = true
		switch c.Outcome {
		case "passed", "failed", "error", "skipped":
		default:
			return ErrInvalidCollaborationTestReport
		}
	}
	total := 0
	for _, l := range v.Logs {
		switch l.Kind {
		case "record":
			if !preparationUUID(l.EventID) || l.MessageIndex < 0 || l.MessageIndex >= CollaborationRecordItemLimit || !preparationHex(l.BodySHA256, 64) || l.Body != "" {
				return ErrInvalidCollaborationTestReport
			}
		case "inline":
			if l.EventID != "" || l.MessageIndex != 0 || l.BodySHA256 != "" || !utf8.ValidString(l.Body) || strings.ContainsRune(l.Body, 0) {
				return ErrInvalidCollaborationTestReport
			}
			total += len(l.Body)
		default:
			return ErrInvalidCollaborationTestReport
		}
	}
	if total > CollaborationTestReportLogBodyLimit {
		return ErrInvalidCollaborationTestReport
	}
	encoded, err := json.Marshal(v)
	if err != nil || len(encoded) > CollaborationTestReportCanonicalLimit {
		return ErrInvalidCollaborationTestReport
	}
	return nil
}

func (v CollaborationTestReport) ReportedResult() string {
	result := "passed"
	for _, c := range v.Cases {
		if c.Outcome == "failed" || c.Outcome == "error" {
			return "failed"
		}
		if c.Outcome == "skipped" {
			result = "incomplete"
		}
	}
	return result
}

func ValidateCollaborationTestReportRequest(v CollaborationTestReportRequest) error {
	if v.Version != CollaborationTestReportVersion || !preparationIdentity(v.Identity) || !preparationUUID(v.RequestID) {
		return ErrInvalidCollaborationTestReport
	}
	return ValidateCollaborationTestReport(v.Report)
}

func DecodeCollaborationTestReportRequest(raw []byte) (CollaborationTestReportRequest, error) {
	var out CollaborationTestReportRequest
	if len(raw) > CollaborationTestReportJSONLimit || decodeTestReportObject(raw, &out, "version", "identity", "request_id", "report") != nil || ValidateCollaborationTestReportRequest(out) != nil {
		return CollaborationTestReportRequest{}, ErrInvalidCollaborationTestReport
	}
	return out, nil
}

func DecodeCollaborationTestSubjectResponse(raw []byte) (CollaborationTestSubjectResponse, error) {
	var out CollaborationTestSubjectResponse
	if decodeTestReportObject(raw, &out, "version", "issue_id", "run_id", "subject") != nil || out.Version != 1 || !preparationUUID(out.IssueID) || !preparationUUID(out.RunID) {
		return CollaborationTestSubjectResponse{}, ErrInvalidCollaborationTestReport
	}
	return out, nil
}

func DecodeCollaborationTestReportReceipt(raw []byte) (CollaborationTestReportReceipt, error) {
	var out CollaborationTestReportReceipt
	if decodeTestReportObject(raw, &out, "version", "issue_id", "run_id", "request_id", "report_id", "report_digest", "accepted") != nil || out.Version != 1 || !out.Accepted ||
		!preparationUUID(out.IssueID) || !preparationUUID(out.RunID) || !preparationUUID(out.RequestID) || !preparationUUID(out.ReportID) || !preparationHex(out.ReportDigest, 64) {
		return CollaborationTestReportReceipt{}, ErrInvalidCollaborationTestReport
	}
	return out, nil
}
