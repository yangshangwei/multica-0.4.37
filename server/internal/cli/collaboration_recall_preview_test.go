package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Provider shells may show only a prefix of a long tool result. The printed
// continuation controls must remain visible even when the text needs a file read.
func TestCollaborationRecallPrintingKeepsContinuationBeforeLargeText(t *testing.T) {
	const cursor = "older-comment-page"
	next := 32768
	body := strings.Repeat("x", 32768)
	for _, tc := range []struct {
		name, control, textField string
		value                    any
	}{
		{"comments", `"next_cursor": "older-comment-page"`, `"items"`, protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment]{
			NextCursor: ptrString(cursor), Truncated: true, Items: []protocol.CollaborationRecallComment{{Content: body}}}},
		{"comment_content", `"next_cursor": "older-comment-page"`, `"content"`, protocol.CollaborationCommentContentChunk{Content: body, NextCursor: ptrString(cursor)}},
		{"handoff_source", `"next_offset": 32768`, `"source"`, protocol.CollaborationHandoffSourcePage{Source: protocol.CollaborationHandoffSource{Content: body, Truncated: true}, NextOffset: &next}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := PrintJSON(&output, tc.value); err != nil {
				t.Fatal(err)
			}
			raw := output.Bytes()
			if len(raw) <= 30000 {
				t.Fatal("fixture no longer crosses the provider preview limit")
			}
			if !bytes.Contains(raw[:30000], []byte(tc.control)) || bytes.Index(raw, []byte(tc.control)) > bytes.Index(raw, []byte(tc.textField)) {
				t.Fatal("continuation control is hidden behind large text")
			}
			var decoded any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func ptrString(value string) *string { return &value }
