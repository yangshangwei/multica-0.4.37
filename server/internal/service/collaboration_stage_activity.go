package service

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/multica-ai/multica/server/internal/iteration"
)

// Select the frozen safe metadata union; never serialize arbitrary event data.
func stageActivityProjection(kind string, raw json.RawMessage) (json.RawMessage, bool) {
	if kind == "stage_execution_admitted" || kind == "stage_execution_observed" || kind == "stage_rollback_observed" {
		return nil, true
	}
	var fields []string
	switch kind {
	case "stage_roles_configured", "stage_roles_unchanged":
		fields = []string{"role_revision"}
	case "stage_proposal_created":
		fields = []string{"proposal_id", "candidate_id", "version"}
	case "stage_proposal_revised":
		fields = []string{"proposal_id", "candidate_id", "version", "parent_candidate_id"}
	case "stage_proposal_accepted", "stage_proposal_rejected":
		fields = []string{"proposal_id", "candidate_id", "version", "decision_id"}
	case "stage_scope_confirmed":
		fields = []string{"decision_id", "scope_revision", "lifecycle_epoch"}
	case "stage_code_candidate_created":
		fields = []string{"candidate_id", "version", "parent_candidate_id"}
	case "stage_decision_recorded":
		fields = []string{"candidate_id", "decision_id", "step"}
	case "stage_decision_revoked":
		fields = []string{"decision_id", "revocation_id"}
	case "stage_advanced":
		fields = []string{"candidate_id", "current", "stage_revision"}
	default:
		return nil, false
	}
	if iteration.ValidateObjectJSON(raw) != nil {
		return nil, false
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return nil, false
	}
	safe := make(map[string]json.RawMessage, len(fields))
	for _, key := range fields {
		value, ok := values[key]
		if !ok {
			return nil, false
		}
		if key == "parent_candidate_id" && kind == "stage_code_candidate_created" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			safe[key] = value
			continue
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			return nil, false
		}
		valid := false
		switch {
		case strings.HasSuffix(key, "_id"):
			valid = stageUUID(text)
		case key == "step":
			valid = text == "code_review" || text == "qa" || text == "release" || text == "rollback"
		case key == "current":
			valid = stageTarget(text)
		default:
			valid = stagePositiveVersion(text)
		}
		if !valid {
			return nil, false
		}
		safe[key] = value
	}
	encoded, err := json.Marshal(safe)
	return encoded, err == nil
}
