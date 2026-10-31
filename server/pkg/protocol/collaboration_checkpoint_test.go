package protocol

import (
	"strings"
	"testing"
)

func checkpointFixture() CollaborationRecordingCheckpoint {
	id := "8f64b727-95a4-44a4-8881-d2113b28ec19"
	return CollaborationRecordingCheckpoint{Version: 1, Identity: CollaborationRunIdentity{DeploymentID: id, InstallationID: id, IssueID: id, RunID: id, SessionID: id, TargetID: id, ChannelID: id, BatchID: id, GroupID: id, LaunchNonce: id, Fence: "1", ClaimGeneration: "1"}, AttemptSeq: "1", AttemptNonce: id, StreamID: id, Sequence: 1, MessageOffset: 0, Messages: []CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "retained"}}}
}

func TestCollaborationCheckpointBoundsAndKinds(t *testing.T) {
	valid := checkpointFixture()
	if err := ValidateCollaborationRecordingCheckpoint(valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*CollaborationRecordingCheckpoint){
		"version":  func(p *CollaborationRecordingCheckpoint) { p.Version = 2 },
		"stream":   func(p *CollaborationRecordingCheckpoint) { p.StreamID = "bad" },
		"sequence": func(p *CollaborationRecordingCheckpoint) { p.Sequence = 0 },
		"offset":   func(p *CollaborationRecordingCheckpoint) { p.MessageOffset = CollaborationRecordItemLimit },
		"empty":    func(p *CollaborationRecordingCheckpoint) { p.Messages = []CollaborationRecordedMessage{} },
		"body": func(p *CollaborationRecordingCheckpoint) {
			p.Messages = []CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: strings.Repeat("a", CollaborationCheckpointBodyLimit+1)}}
		},
		"aggregate": func(p *CollaborationRecordingCheckpoint) {
			p.Messages = []CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: strings.Repeat("a", CollaborationCheckpointBodyLimit/2+1)}, {Kind: "message", Role: "assistant", Body: strings.Repeat("b", CollaborationCheckpointBodyLimit/2)}}
		},
		"tool shape": func(p *CollaborationRecordingCheckpoint) {
			p.Messages = []CollaborationRecordedMessage{{Kind: "tool_result", CallID: "call", Body: "result"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := valid
			mutate(&p)
			if ValidateCollaborationRecordingCheckpoint(p) == nil {
				t.Fatal("accepted invalid checkpoint")
			}
		})
	}
}

func TestCollaborationCheckpointRecordVersionCompatibility(t *testing.T) {
	p := checkpointFixture()
	for _, version := range []int{1, 2} {
		r := &CollaborationRunRecord{Version: version, Completeness: "complete", Messages: p.Messages}
		if version == 2 {
			r.StreamID = p.StreamID
			r.MessageOffset = 4
		}
		if err := ValidateCollaborationRunRecord(r); err != nil {
			t.Fatal(err)
		}
	}
	r := &CollaborationRunRecord{Version: 1, Completeness: "complete", Messages: p.Messages, StreamID: p.StreamID}
	if ValidateCollaborationRunRecord(r) == nil {
		t.Fatal("legacy recording accepted stream metadata")
	}
	r.Version = 2
	r.MessageOffset = CollaborationRecordItemLimit
	if ValidateCollaborationRunRecord(r) == nil {
		t.Fatal("record accepted suffix past item budget")
	}
}

func TestCollaborationRecordingSealHasOnlyBoundedMetadata(t *testing.T) {
	c := checkpointFixture()
	observed := 1
	p := CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: c.Identity, AttemptSeq: c.AttemptSeq, AttemptNonce: c.AttemptNonce, StreamID: c.StreamID, ObservedThrough: &observed, Reason: "stopped"}
	if err := ValidateCollaborationRecordingSeal(p); err != nil {
		t.Fatal(err)
	}
	p.AcknowledgedThrough = 2
	if ValidateCollaborationRecordingSeal(p) == nil {
		t.Fatal("accepted acknowledged count beyond observed")
	}
	p.AcknowledgedThrough = 0
	p.Reason = "free text result"
	if ValidateCollaborationRecordingSeal(p) == nil {
		t.Fatal("accepted arbitrary seal reason")
	}
	p.Reason = "completed"
	p.ObservedThrough = nil
	if ValidateCollaborationRecordingSeal(p) == nil {
		t.Fatal("unknown tail labeled complete")
	}
}

func TestCollaborationRecordingSealAdvancesBoundedKnowledge(t *testing.T) {
	c := checkpointFixture()
	prior := CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: c.Identity, AttemptSeq: c.AttemptSeq, AttemptNonce: c.AttemptNonce, StreamID: c.StreamID, Reason: "stopped"}
	if err := ValidateCollaborationRecordingSealAdvance(nil, prior); err != nil {
		t.Fatal(err)
	}
	observed := 3
	next := prior
	next.Sequence, next.AcknowledgedThrough, next.ObservedThrough = 2, 1, &observed
	if err := ValidateCollaborationRecordingSealAdvance(&prior, next); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CollaborationRecordingSeal){
		"same key":                 func(v *CollaborationRecordingSeal) { v.Sequence = 2 },
		"gap":                      func(v *CollaborationRecordingSeal) { v.Sequence = 4 },
		"ack decreases":            func(v *CollaborationRecordingSeal) { v.AcknowledgedThrough = 0 },
		"observed becomes unknown": func(v *CollaborationRecordingSeal) { v.ObservedThrough = nil },
		"observed decreases":       func(v *CollaborationRecordingSeal) { n := 2; v.ObservedThrough = &n },
		"reason changes":           func(v *CollaborationRecordingSeal) { v.Reason = "completed" },
		"run changes":              func(v *CollaborationRecordingSeal) { v.Identity.Fence = "2" },
		"stream changes":           func(v *CollaborationRecordingSeal) { v.StreamID = "4a19a69f-ed39-4438-b0dd-f29b852f306a" },
		"past limit":               func(v *CollaborationRecordingSeal) { v.Sequence = CollaborationRecordingSealLimit + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			v := next
			v.Sequence = 3
			mutate(&v)
			if ValidateCollaborationRecordingSealAdvance(&next, v) == nil {
				t.Fatal("accepted non-monotonic recording seal")
			}
		})
	}
	prior.Sequence = CollaborationRecordingSealLimit
	next = prior
	next.Sequence++
	if ValidateCollaborationRecordingSealAdvance(&prior, next) == nil || ValidateCollaborationRecordingSealAdvance(nil, prior) == nil {
		t.Fatal("seal sequence did not enforce initial and final bounds")
	}
}
