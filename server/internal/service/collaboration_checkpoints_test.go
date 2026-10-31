package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func checkpointFor(claim protocol.CollaborationRunClaimResponse, permit protocol.CollaborationRunPermitResponse) protocol.CollaborationRecordingCheckpoint {
	return protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, StreamID: uuid.NewString(), Sequence: 1, Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "retained before stop"}}}
}

func TestCollaborationRecordingSealRetainsMaximumCheckpointHistory(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "retain the bounded recording history")
	run, claim := claimHostRun(t, f, caps)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, hostPermitRequest(claim, 1, "provider"))
	if err != nil {
		t.Fatal(err)
	}
	p := checkpointFor(claim, permit)
	for seq := 1; seq <= protocol.CollaborationCheckpointCountLimit; seq++ {
		p.Sequence, p.MessageOffset = seq, seq-1
		p.Messages[0].Body = fmt.Sprintf("message %d", seq)
		ack, err := f.svc.RecordRuntimeCheckpoint(context.Background(), run, permit.BusinessToken, p)
		if err != nil || ack.RetainedThrough != seq {
			t.Fatalf("checkpoint %d: retained=%d: %v", seq, ack.RetainedThrough, err)
		}
	}
	stop := f.issueControl(t)
	stop.ExecutionViewVersion = 6
	if _, err := f.svc.StopQueue(f.ctx, stop); err != nil {
		t.Fatal(err)
	}
	observed := protocol.CollaborationRecordItemLimit
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, StreamID: p.StreamID, ObservedThrough: &observed, Reason: "stopped"}
	first := seal
	for seq := 1; seq <= protocol.CollaborationRecordingSealLimit; seq++ {
		seal.Sequence, seal.AcknowledgedThrough = seq, seq-1
		retained, err := f.svc.SealRuntimeRecording(context.Background(), run, claim.TerminationToken, seal)
		if err != nil || retained.RetainedThrough != protocol.CollaborationRecordItemLimit || retained.ObservedThrough == nil || *retained.ObservedThrough != observed {
			t.Fatalf("seal %d changed the actual retained prefix: %+v: %v", seq, retained, err)
		}
	}
	state, err := readCollaborationRecording(context.Background(), f.svc.Queries, f.issue.Issue.ID, run, math.MaxInt64)
	if err != nil || state.seal == nil || state.seal.Sequence != protocol.CollaborationRecordingSealLimit {
		t.Fatalf("bounded query silently omitted the last seal: %+v: %v", state.seal, err)
	}
	retained, err := f.svc.SealRuntimeRecording(context.Background(), run, claim.TerminationToken, first)
	if err != nil || retained.RetainedThrough != protocol.CollaborationRecordItemLimit {
		t.Fatalf("old original-key replay reduced retained history: %+v: %v", retained, err)
	}
	first.AcknowledgedThrough++
	if _, err := f.svc.SealRuntimeRecording(context.Background(), run, claim.TerminationToken, first); !collaborationErrorCode(err, "recording_seal_conflict") {
		t.Fatalf("changed original seal was accepted: %v", err)
	}
	seal.Sequence++
	if _, err := f.svc.SealRuntimeRecording(context.Background(), run, claim.TerminationToken, seal); !collaborationErrorCode(err, "invalid_recording_seal") {
		t.Fatalf("unbounded seal sequence accepted: %v", err)
	}
	if _, err := f.svc.RecordRuntimeCheckpoint(context.Background(), run, claim.TerminationToken, p); !collaborationErrorCode(err, "business_capability_required") {
		t.Fatalf("termination credential acquired business write purpose: %v", err)
	}
}

func TestCollaborationCheckpointStopKeepsOriginalPrefixAndContentFreeACK(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "retain interrupted discoveries")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
	if err != nil {
		t.Fatal(err)
	}
	p := checkpointFor(claim, permit)
	ack, err := f.svc.RecordRuntimeCheckpoint(context.Background(), id, permit.BusinessToken, p)
	if err != nil || !ack.Accepted || ack.RetainedThrough != 1 {
		t.Fatalf("checkpoint: %+v %v", ack, err)
	}
	stop := f.issueControl(t)
	stop.ExecutionViewVersion = 6
	if _, err := f.svc.StopQueue(f.ctx, stop); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), id, permit.BusinessToken, p); err != nil {
		t.Fatalf("lost ACK cannot confirm original: %v", err)
	}
	changed := p
	changed.Messages = []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "new after stop"}}
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), id, permit.BusinessToken, changed); !collaborationErrorCode(err, "checkpoint_conflict") {
		t.Fatalf("changed retry: %v", err)
	}
	changed = p
	changed.Sequence = 2
	changed.MessageOffset = 1
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), id, permit.BusinessToken, changed); err == nil {
		t.Fatal("accepted new stopped content")
	}
	observed := 2
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, StreamID: p.StreamID, ObservedThrough: &observed, AcknowledgedThrough: 0, Reason: "stopped"}
	retained, err := f.svc.SealRuntimeRecording(context.Background(), id, claim.TerminationToken, seal)
	if err != nil || retained.RetainedThrough != 1 || !retained.Sealed {
		t.Fatalf("lost ACK erased actual retained prefix: %+v %v", retained, err)
	}
	if _, err = f.svc.SealRuntimeRecording(context.Background(), id, claim.TerminationToken, seal); err != nil {
		t.Fatal(err)
	}
	// A later original-key seal may add knowledge without replacing its past payload.
	advanced := seal
	advanced.Sequence = 2
	advanced.AcknowledgedThrough = 1
	if _, err = f.svc.SealRuntimeRecording(context.Background(), id, claim.TerminationToken, advanced); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.SealRuntimeRecording(context.Background(), id, claim.TerminationToken, seal); err != nil {
		t.Fatalf("old seal ACK was lost after metadata advanced: %v", err)
	}
	advanced.Sequence = 3
	advanced.ObservedThrough = nil
	if _, err = f.svc.SealRuntimeRecording(context.Background(), id, claim.TerminationToken, advanced); !collaborationErrorCode(err, "recording_seal_conflict") {
		t.Fatalf("seal removed known observed bound: %v", err)
	}
	viewer := WithCollaborationExecutionViewVersion(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), 6)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if page.Recording.Status != "partial" || page.Recording.Retention == nil || page.Recording.Retention.RetainedThrough != 1 {
		t.Fatalf("incomplete history not visible: %+v", page.Recording)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='record_checkpoint'`, id) != 1 {
		t.Fatal("checkpoint was duplicated")
	}
}

func TestCollaborationCheckpointOutcomeOverlapAndInternalEventsStayReadable(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "record stream overlap")
	id, claim := claimHostRun(t, f, caps)
	launch := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, id, launch)
	if err != nil {
		t.Fatal(err)
	}
	p := checkpointFor(claim, permit)
	p.Messages[0].Body = "secret mcb_" + strings.Repeat("a", 64)
	if _, err = f.svc.RecordRuntimeCheckpoint(context.Background(), id, permit.BusinessToken, p); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"handoff_snapshot", "handoff_binding", "wakeup_pending", "wakeup_assigned"} {
		f.fx.Exec(t, `INSERT INTO collaboration_run_event(issue_id,run_id,producer_event_id,seq,kind,payload) VALUES($1,$2,$3,(SELECT max(seq)+1 FROM collaboration_run_event WHERE run_id=$2),$4,'{}')`, f.issue.Issue.ID, id, fmt.Sprintf("internal-%d", i), kind)
	}
	outcome := protocol.CollaborationRunOutcome{ProtocolVersion: 3, Containment: launch.Containment, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, EventID: uuid.NewString(), Outcome: "success", Body: "final", Record: &protocol.CollaborationRunRecord{Version: 2, StreamID: p.StreamID, Completeness: "complete", Messages: append(p.Messages, protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: "final suffix"})}}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), id, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	viewer := WithCollaborationExecutionViewVersion(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), 6)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	bodies := []string{}
	for {
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate item %s", item.ID)
			}
			seen[item.ID] = true
			if item.Kind == "message" {
				bodies = append(bodies, item.Body)
			}
		}
		if page.NextCursor == nil {
			break
		}
		page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 1, Cursor: *page.NextCursor})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(bodies) != 2 || strings.Contains(bodies[0], "mcb_") || bodies[1] != "final suffix" {
		t.Fatalf("incorrect stream projection: %v", bodies)
	}
}
