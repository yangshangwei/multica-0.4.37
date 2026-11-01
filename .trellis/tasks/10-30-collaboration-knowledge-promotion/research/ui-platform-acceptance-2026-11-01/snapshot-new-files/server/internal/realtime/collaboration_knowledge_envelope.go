package realtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errKnowledgeEnvelope = errors.New("invalid knowledge envelope")

// canonicalKnowledgeFrame inspects every outer type before selecting the
// knowledge contract. Last-key wins must not disguise a knowledge frame as an
// unrelated event. Valid unrelated frames retain their original bytes.
func canonicalKnowledgeFrame(raw []byte) ([]byte, bool, error) {
	if !utf8.Valid(raw) {
		return nil, false, errKnowledgeEnvelope
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, false, errKnowledgeEnvelope
	}
	fields := make(map[string]json.RawMessage)
	knowledge, duplicate := false, false
	for d.More() {
		token, err = d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, knowledge, errKnowledgeEnvelope
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, knowledge, errKnowledgeEnvelope
		}
		if _, exists := fields[key]; exists {
			duplicate = true
		}
		fields[key] = value
		if strings.EqualFold(key, "type") {
			var kind string
			if json.Unmarshal(value, &kind) == nil && kind == protocol.EventCollaborationKnowledgeChanged {
				knowledge = true
			}
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return nil, knowledge, errKnowledgeEnvelope
	}
	if err = d.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, knowledge, errKnowledgeEnvelope
	}
	if !knowledge {
		return raw, false, nil
	}
	if duplicate {
		return nil, true, errKnowledgeEnvelope
	}
	for key := range fields {
		if key != "type" && key != "payload" && key != "event_id" {
			return nil, true, errKnowledgeEnvelope
		}
	}
	var frame struct {
		Type    string                                    `json:"type"`
		Payload protocol.CollaborationKnowledgeTargetHint `json:"payload"`
		EventID string                                    `json:"event_id,omitempty"`
	}
	if json.Unmarshal(fields["type"], &frame.Type) != nil || frame.Type != protocol.EventCollaborationKnowledgeChanged || json.Unmarshal(fields["payload"], &frame.Payload) != nil {
		return nil, true, errKnowledgeEnvelope
	}
	if id, exists := fields["event_id"]; exists {
		if json.Unmarshal(id, &frame.EventID) != nil || !strings.HasSuffix(frame.EventID, ":target") {
			return nil, true, errKnowledgeEnvelope
		}
		original := strings.TrimSuffix(frame.EventID, ":target")
		parsed, err := uuid.Parse(original)
		if err != nil || parsed == uuid.Nil || parsed.String() != original {
			return nil, true, errKnowledgeEnvelope
		}
	}
	// New per-send bytes ensure the frame being forwarded has only the values
	// validated here, without mutating a buffer shared by other subscribers.
	canonical, err := json.Marshal(frame)
	if err != nil {
		return nil, true, errKnowledgeEnvelope
	}
	return canonical, true, nil
}
