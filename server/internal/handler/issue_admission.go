package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/multica-ai/multica/server/internal/admission"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func writeIssueAdmissionError(w http.ResponseWriter, err error) bool {
	var blocked *admission.Blocked
	if !errors.As(err, &blocked) {
		return false
	}
	writeJSON(w, http.StatusConflict, map[string]any{"code": "triage_review_required", "error": blocked.Error(), "issue_id": uuidToString(blocked.IssueID), "issue_path": "/issues/" + uuidToString(blocked.IssueID)})
	return true
}

func issueAdmissionMutation(issue db.Issue, fields map[string]json.RawMessage) error {
	if admission.Formal(issue.AdmissionStatus) {
		return nil
	}
	for _, field := range []string{"status", "assignee_type", "assignee_id", "project_id", "parent_issue_id", "stage", "position"} {
		if _, ok := fields[field]; ok {
			return &admission.Blocked{IssueID: issue.ID}
		}
	}
	return nil
}
