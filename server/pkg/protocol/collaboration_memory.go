package protocol

// CollaborationMemoryVersion is the only shared-context ("memory") contract a
// host claim can declare. Linux namespace claims never declare it.
const CollaborationMemoryVersion = 1

// CollaborationRunMemory is the shared-context document a run reads. Version is
// an exact decimal string; "0" with empty content is the empty document.
type CollaborationRunMemory struct {
	IssueID  string `json:"issue_id"`
	RunID    string `json:"run_id"`
	Version  string `json:"version"`
	Content  string `json:"content"`
	Bytes    int    `json:"bytes"`
	MaxBytes int    `json:"max_bytes"`
}

// CollaborationRunMemoryUpdate replaces the whole document. BaseVersion is the
// version the agent merged from; RequestID is its retry identity.
type CollaborationRunMemoryUpdate struct {
	RequestID   string `json:"request_id"`
	BaseVersion string `json:"base_version"`
	Content     string `json:"content"`
}

// CollaborationRunMemoryReceipt answers a runtime write. It never repeats the
// document text. Version is null on a refusal; CurrentVersion is the version
// the decision was made against. Bytes is the stored size on success and the
// submitted (redacted) size when the content is too large.
type CollaborationRunMemoryReceipt struct {
	Code           string  `json:"code"`
	Error          string  `json:"error,omitempty"`
	IssueID        string  `json:"issue_id"`
	RunID          string  `json:"run_id"`
	RequestID      string  `json:"request_id"`
	Version        *string `json:"version"`
	CurrentVersion string  `json:"current_version"`
	Bytes          *int    `json:"bytes,omitempty"`
	MaxBytes       int     `json:"max_bytes"`
}
