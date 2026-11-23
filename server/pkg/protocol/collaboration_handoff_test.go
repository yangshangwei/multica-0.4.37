package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func testHandoffEnvelope() CollaborationHandoffEnvelope {
	return CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: "11111111-1111-4111-8111-111111111111", RunID: "22222222-2222-4222-8222-222222222222", Mode: "live", EventSequence: "0", Authority: CollaborationHandoffAuthority{Status: "todo"}, Assignment: CollaborationHandoffAssignment{Role: "agent"}, Memory: CollaborationHandoffMemory{Version: "0", Availability: "absent"}, Inputs: CollaborationHandoffInputs{Items: []CollaborationHandoffInput{}}, Results: CollaborationHandoffResults{Items: []CollaborationHandoffResult{}}, WakeupCauses: CollaborationHandoffCauses{Items: []CollaborationHandoffCause{}, Completeness: "unknown"}, Sources: []CollaborationHandoffSource{}, Missing: []string{}}
}

func TestCollaborationHandoffPreservesEscapedAdmittedAssignment(t *testing.T) {
	for _, body := range []string{strings.Repeat("<", 64<<10), strings.Repeat("&", 64<<10), strings.Repeat("\x01", 64<<10), strings.Repeat("中", (64<<10)/3) + "x"} {
		v := testHandoffEnvelope()
		v.Sources = []CollaborationHandoffSource{{Kind: "delegation", ID: v.IssueID, RunID: v.RunID, Version: "1", SHA256: CollaborationHandoffHash(body), Content: body, Availability: "available", Trust: "admitted_assignment"}}
		if err := ValidateCollaborationHandoff(v); err != nil {
			t.Errorf("legal 64 KiB assignment refused (first byte %x): %v", body[0], err)
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var restored CollaborationHandoffEnvelope
		if err = json.Unmarshal(raw, &restored); err != nil || restored.Sources[0].Content != body || restored.Sources[0].SHA256 != CollaborationHandoffHash(body) {
			t.Fatalf("assignment wire/hash changed: %v", err)
		}
	}
}

func TestCollaborationHandoffRejectsOversizedRawJSON(t *testing.T) {
	raw, err := json.Marshal(testHandoffEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw[:len(raw)-1], []byte(strings.Repeat(" ", 2<<20)+"}")...)
	var v CollaborationHandoffEnvelope
	if json.Unmarshal(raw, &v) == nil {
		t.Fatal("accepted a raw JSON envelope beyond the wire ceiling")
	}
}

func TestCollaborationHandoffMaterialBudgetIncludesMetadata(t *testing.T) {
	v := testHandoffEnvelope()
	v.Sources = []CollaborationHandoffSource{{Kind: "provider_result", ID: v.IssueID, RunID: v.RunID, Version: "1", SHA256: CollaborationHandoffHash(""), Availability: "available", Trust: "data_only"}}
	metadata, _, err := CollaborationHandoffSizes(v)
	if err != nil {
		t.Fatal(err)
	}
	v.Sources[0].Content = strings.Repeat("\x01", CollaborationHandoffMaxBytes-metadata)
	v.Sources[0].SHA256 = CollaborationHandoffHash(v.Sources[0].Content)
	material, wire, err := CollaborationHandoffSizes(v)
	if err != nil || material != CollaborationHandoffMaxBytes || wire <= material || wire > CollaborationHandoffMaxJSONBytes || ValidateCollaborationHandoff(v) != nil {
		t.Fatalf("valid material boundary: material=%d wire=%d: %v", material, wire, err)
	}
	v.Authority.Status += "x"
	if ValidateCollaborationHandoff(v) == nil {
		t.Fatal("metadata was not charged to the material budget")
	}
}

func TestCollaborationHandoffRejectsFalseCompletenessAndUnknownVersion(t *testing.T) {
	base := testHandoffEnvelope()
	if err := ValidateCollaborationHandoff(base); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CollaborationHandoffEnvelope){
		func(v *CollaborationHandoffEnvelope) { v.ProtocolVersion = 2 },
		func(v *CollaborationHandoffEnvelope) { v.Inputs.Items = nil },
		func(v *CollaborationHandoffEnvelope) { v.Inputs.NextCursor = "unexpected" },
		func(v *CollaborationHandoffEnvelope) { v.Memory.Availability = "available"; v.Memory.Version = "1" },
		func(v *CollaborationHandoffEnvelope) {
			v.Sources = append(v.Sources, CollaborationHandoffSource{Kind: "system_instruction", ID: v.IssueID})
		},
	} {
		v := testHandoffEnvelope()
		mutate(&v)
		if ValidateCollaborationHandoff(v) == nil {
			t.Fatalf("accepted malformed handoff: %+v", v)
		}
	}
}

func TestCollaborationHandoffWireRejectsUnrecognizedAuthority(t *testing.T) {
	v := testHandoffEnvelope()
	raw, _ := json.Marshal(v)
	var roundtrip CollaborationHandoffEnvelope
	if err := json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatal(err)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"allow_all_tools":true}`)...)
	if json.Unmarshal(raw, &roundtrip) == nil {
		t.Fatal("unknown authority field accepted")
	}
}
