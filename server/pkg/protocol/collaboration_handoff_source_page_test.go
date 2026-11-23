package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const handoffSourcePageBody = "第一段：选用共享数据库。第二段：已验证迁移。"

// The first page carries a multibyte prefix and points at the next chunk.
func firstHandoffSourcePage() CollaborationHandoffSourcePage {
	next := len("第一段：选用共享数据库。")
	return CollaborationHandoffSourcePage{ProtocolVersion: 1, IssueID: uuid.NewString(), RunID: uuid.NewString(),
		Source: CollaborationHandoffSource{Kind: "provider_result", ID: uuid.NewString(), RunID: uuid.NewString(), Version: "5",
			SHA256: CollaborationHandoffHash(handoffSourcePageBody), Content: handoffSourcePageBody[:next], Truncated: true, Availability: "available", Trust: "data_only"},
		Offset: 0, TotalBytes: len(handoffSourcePageBody), NextOffset: &next}
}

func TestDecodeCollaborationHandoffSourcePageAcceptsEachChunkShape(t *testing.T) {
	first := firstHandoffSourcePage()
	last := first
	last.Offset, last.NextOffset = *first.NextOffset, nil
	last.Source.Content, last.Source.Truncated = handoffSourcePageBody[*first.NextOffset:], false
	note := last
	note.Source.Kind, note.Source.RunID, note.Source.Version = "handoff", "", ""
	delegation := last
	delegation.Source.Kind, delegation.Source.Trust = "delegation", "admitted_assignment"
	empty := last
	empty.Offset, empty.TotalBytes, empty.Source.Content = 0, 0, ""
	for name, page := range map[string]CollaborationHandoffSourcePage{"first": first, "last": last, "handoff": note, "delegation": delegation, "empty": empty} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(page)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeCollaborationHandoffSourcePage(raw)
			if err != nil || !reflect.DeepEqual(got, page) {
				t.Fatalf("valid source page was not decoded exactly: %+v %v", got, err)
			}
		})
	}
}

func TestDecodeCollaborationHandoffSourcePageRejectsMalformedWire(t *testing.T) {
	source := func(o map[string]any) map[string]any { return o["source"].(map[string]any) }
	// Control: the unmutated wire object is valid, so each refusal below is
	// caused by its own mutation.
	control, _ := json.Marshal(handoffWireObject(t, firstHandoffSourcePage()))
	if _, err := DecodeCollaborationHandoffSourcePage(control); err != nil {
		t.Fatalf("control page was refused: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"missing_next_offset":    func(o map[string]any) { delete(o, "next_offset") },
		"missing_total":          func(o map[string]any) { delete(o, "total_bytes") },
		"missing_offset":         func(o map[string]any) { delete(o, "offset") },
		"case_alias_offset":      func(o map[string]any) { delete(o, "offset"); o["Offset"] = 0 },
		"null_source":            func(o map[string]any) { o["source"] = nil },
		"missing_source_content": func(o map[string]any) { delete(source(o), "content") },
		"missing_source_trust":   func(o map[string]any) { delete(source(o), "trust") },
		"unknown_field":          func(o map[string]any) { o["owner_token"] = "forbidden" },
		"unknown_source_field":   func(o map[string]any) { source(o)["authority"] = "granted" },
		"protocol_version":       func(o map[string]any) { o["protocol_version"] = 2 },
		"issue_id":               func(o map[string]any) { o["issue_id"] = "issue" },
		"run_id_uppercase":       func(o map[string]any) { o["run_id"] = strings.ToUpper(o["run_id"].(string)) },
		"source_id":              func(o map[string]any) { source(o)["id"] = "source" },
		"source_run_id":          func(o map[string]any) { source(o)["run_id"] = "run" },
		"source_version":         func(o map[string]any) { source(o)["version"] = "05" },
		"source_kind":            func(o map[string]any) { source(o)["kind"] = "record_checkpoint" },
		"sha256_uppercase":       func(o map[string]any) { source(o)["sha256"] = strings.ToUpper(source(o)["sha256"].(string)) },
		"availability":           func(o map[string]any) { source(o)["availability"] = "purged" },
		"result_trust":           func(o map[string]any) { source(o)["trust"] = "admitted_assignment" },
		"delegation_trust":       func(o map[string]any) { source(o)["kind"] = "delegation" },
		"negative_offset":        func(o map[string]any) { o["offset"] = -1 },
		"next_offset_gap":        func(o map[string]any) { o["next_offset"] = o["next_offset"].(float64) + 1 },
		"next_without_truncated": func(o map[string]any) { source(o)["truncated"] = false },
		"short_final_chunk":      func(o map[string]any) { o["next_offset"] = nil; source(o)["truncated"] = false },
		// Content reaches total_bytes, so only the truncated flag is wrong.
		"final_chunk_claims_truncated": func(o map[string]any) {
			source(o)["content"] = handoffSourcePageBody
			o["next_offset"] = nil
		},
		"content_past_total": func(o map[string]any) { o["total_bytes"] = 1 },
		"empty_chunk_with_next": func(o map[string]any) {
			source(o)["content"] = ""
			o["next_offset"] = 0
		},
	} {
		t.Run(name, func(t *testing.T) {
			object := handoffWireObject(t, firstHandoffSourcePage())
			mutate(object)
			raw, _ := json.Marshal(object)
			if _, err := DecodeCollaborationHandoffSourcePage(raw); err == nil {
				t.Fatal("malformed source page was accepted")
			}
		})
	}
	raw, _ := json.Marshal(firstHandoffSourcePage())
	for name, value := range map[string][]byte{"trailing_value": append(append([]byte{}, raw...), []byte("{}")...), "array": []byte("[]"), "null": []byte("null")} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCollaborationHandoffSourcePage(value); err == nil {
				t.Fatal("malformed source page was accepted")
			}
		})
	}
}
