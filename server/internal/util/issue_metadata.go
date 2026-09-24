package util

import "encoding/json"

// IssueMetadataForResponse preserves the primitive metadata contract expected
// by installed clients. Lifecycle evidence remains structured in storage for
// internal history readers, but its reserved object and array values travel as
// JSON strings on the wire. Other metadata is left unchanged.
func IssueMetadataForResponse(raw []byte) map[string]any {
	metadata := JSONObjectOrEmpty(raw)
	for _, key := range []string{"lifecycle_handoff", "lifecycle_handoff_history", "lifecycle_rca_evidence", "lifecycle_rca_unknowns"} {
		value := metadata[key]
		switch value.(type) {
		case map[string]any, []any:
			if encoded, err := json.Marshal(value); err == nil {
				metadata[key] = string(encoded)
			}
		}
	}
	return metadata
}
