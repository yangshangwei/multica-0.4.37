package main

import (
	"fmt"
	"io"
	"strings"
)

// warnCommentTriggerOutcomes consumes only the immediate write receipt. A
// missing array is compatible with older servers, not proof of delegation.
// The caller keeps the saved comment's exit status and untouched JSON response.
func warnCommentTriggerOutcomes(w io.Writer, response map[string]any) {
	raw, present := response["trigger_outcomes"]
	if !present {
		return
	}
	type receipt struct{ status, reason string }
	byTarget := make(map[string]receipt)
	handled, blocked, unconfirmed := 0, 0, 0
	outcomes, ok := raw.([]any)
	if !ok {
		unconfirmed++
	}
	for _, item := range outcomes {
		entry, ok := item.(map[string]any)
		if !ok {
			unconfirmed++
			continue
		}
		kind, _ := entry["target_type"].(string)
		id, _ := entry["target_id"].(string)
		status, _ := entry["status"].(string)
		reason, _ := entry["reason_code"].(string)
		if (kind != "agent" && kind != "squad") || !uuidRegexp.MatchString(id) {
			unconfirmed++
			continue
		}
		key := kind + "/" + strings.ToLower(id)
		current := receipt{status, reason}
		if status == "" || strings.TrimSpace(reason) == "" {
			// Once a target is identified, a malformed receipt invalidates
			// that target instead of counting a separate unknown entry.
			current = receipt{}
		}
		if previous, seen := byTarget[key]; seen && previous != current {
			// Conflicting receipts cannot establish admission. Once ambiguous,
			// later entries must not restore an apparently successful status.
			current = receipt{}
		}
		byTarget[key] = current
	}
	for _, result := range byTarget {
		switch result.status {
		case "queued", "coalesced", "deferred":
			handled++
		case "blocked":
			blocked++
		default:
			unconfirmed++
		}
	}
	if blocked+unconfirmed > 0 {
		// Do not echo arbitrary server fields into the terminal. Exact target
		// identities and stable reasons remain available in the JSON output.
		fmt.Fprintf(w, "Warning: comment saved, but delegation is not fully confirmed (%d handled, %d blocked, %d unconfirmed). Do not repost it; inspect trigger_outcomes from this response when available.\n", handled, blocked, unconfirmed)
	}
}
