package iteration

import "time"

// Preview is a complete authorized comparison set, never a page of members.
type Preview struct {
	WorkspaceID   string                `json:"workspace_id"`
	ActorUserID   string                `json:"actor_user_id"`
	Draft         Draft                 `json:"draft"`
	PreviewHash   string                `json:"preview_hash"`
	PreviewedAt   time.Time             `json:"previewed_at"`
	StartPreview  *StartDatePreview     `json:"start_preview"`
	Iterations    []Iteration           `json:"iterations"`
	Issues        []PreviewIssue        `json:"issues"`
	Statistics    map[string]Statistics `json:"statistics"`
	Recipients    []string              `json:"recipients"`
	InvalidItems  []InvalidItem         `json:"invalid_items"`
	TotalAffected int                   `json:"total_affected"`
	Complete      bool                  `json:"complete"`
}
type PreviewIssue struct {
	IssueID               string           `json:"issue_id"`
	Identifier            string           `json:"identifier"`
	Revision              int64            `json:"revision"`
	SourceID              *string          `json:"source_id"`
	StatusCategory        string           `json:"status_category"`
	Project               PreviewReference `json:"project"`
	Assignee              PreviewReference `json:"assignee"`
	Title                 string           `json:"title"`
	RunningExecutionCount int64            `json:"running_execution_count"`
	RolloverCount         int32            `json:"rollover_count"`
}
type PreviewReference struct {
	ID        *string `json:"id"`
	Name      *string `json:"name"`
	Type      *string `json:"type"`
	Available bool    `json:"available"`
}
type InvalidItem struct {
	IssueID *string `json:"issue_id"`
	Code    string  `json:"code"`
}
