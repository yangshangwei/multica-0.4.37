package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type previewGroupingWire struct {
	Kind                string        `json:"kind"`
	TargetIDs           []pgtype.UUID `json:"target_ids"`
	HasExistingInputs   bool          `json:"has_existing_inputs"`
	FollowsPendingBatch bool          `json:"follows_pending_batch"`
}

func previewGrouping(t *testing.T, preview CollaborationCommentPreview, target pgtype.UUID) *previewGroupingWire {
	t.Helper()
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Targets []struct {
			TargetID pgtype.UUID          `json:"target_id"`
			Grouping *previewGroupingWire `json:"grouping"`
		} `json:"targets"`
	}
	if err = json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	for _, item := range wire.Targets {
		if item.TargetID == target {
			return item.Grouping
		}
	}
	t.Fatalf("target %s missing from preview", util.UUIDToString(target))
	return nil
}

func previewMention(target db.CollaborationTarget) string {
	return "[@target](mention://" + target.Kind + "/" + util.UUIDToString(target.ResourceID) + ") "
}

func TestCollaborationCommentPreviewGroupsTypedTargetsAndSplitsSecondSquad(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	a := queueTarget(t, f, f.targetID)
	s, _ := seedQueueSquad(t, f, false)
	other, _ := seedQueueSquad(t, f, false)
	b, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL,hold_version=hold_version+1 WHERE session_id=$1`, f.issue.SessionID)
	input := CollaborationCommentInput{Content: previewMention(a) + previewMention(s) + previewMention(other) + previewMention(b) + previewMention(a)}
	before := controlStateSnapshot(t, f)
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Targets) != 4 {
		t.Fatalf("duplicate typed mention was not deduplicated: %+v", preview.Targets)
	}
	for _, target := range []pgtype.UUID{a.ID, s.ID} {
		group := previewGrouping(t, preview, target)
		if group == nil || group.Kind != "pending_batch" || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{a.ID, s.ID}) || group.HasExistingInputs || group.FollowsPendingBatch {
			t.Fatalf("direct agent and its squad must share one preview batch: %+v", group)
		}
	}
	group := previewGrouping(t, preview, other.ID)
	if group == nil || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{other.ID}) || !group.FollowsPendingBatch || group.HasExistingInputs {
		t.Fatalf("second eligible squad must follow the first batch: %+v", group)
	}
	group = previewGrouping(t, preview, b.ID)
	if group == nil || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{b.ID}) || group.FollowsPendingBatch {
		t.Fatalf("independent agent inherited another channel's ordering: %+v", group)
	}
	input.SuppressedTargetIDs = []pgtype.UUID{s.ID}
	suppressed, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if previewGrouping(t, suppressed, s.ID) != nil {
		t.Fatal("suppressed target still participates in grouping")
	}
	group = previewGrouping(t, suppressed, a.ID)
	if group == nil || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{a.ID, other.ID}) || group.FollowsPendingBatch {
		t.Fatalf("suppression must recompute the remaining batch: %+v", group)
	}
	if after := controlStateSnapshot(t, f); before != after {
		t.Fatal("preview persisted queue, grants, events or execution effects")
	}
	// Submission and the canonical selector own actual FIFO membership.
	content := &collaborationContentFixture{collaborationFixture: f.collaborationFixture, issue: f.issue}
	p := content.input(input.Content, a.ID, s.ID, other.ID, b.ID)
	p.SuppressedTargetIDs = input.SuppressedTargetIDs
	receipt, err := f.svc.CreateComment(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	selection := selectFixtureFIFO(t, f, receipt.Targets[0].ChannelID)
	if selection.target.ID != other.ID || len(selection.inputs) != 2 || selection.inputs[0].TargetID != a.ID || selection.inputs[1].TargetID != other.ID {
		t.Fatalf("preview differs from actual FIFO selection: %+v", selection)
	}
}

func TestCollaborationCommentPreviewPendingInputsRespectEligibility(t *testing.T) {
	for _, scenario := range []string{"eligible", "held", "revoked", "stale_author"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			a := queueTarget(t, f, f.targetID)
			s, _ := seedQueueSquad(t, f, false)
			other, _ := seedQueueSquad(t, f, false)
			submitTarget(t, f, other.ID, f.handler, "earlier private input", pgtype.UUID{})
			f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL,hold_version=hold_version+1 WHERE session_id=$1`, f.issue.SessionID)
			switch scenario {
			case "held":
				f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='switch_hold' WHERE id=$1`, other.ID)
			case "revoked":
				f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now() WHERE target_id=$1`, other.ID)
			case "stale_author":
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
			}
			before := controlStateSnapshot(t, f)
			preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, CollaborationCommentInput{Content: previewMention(s) + previewMention(a)})
			if err != nil {
				t.Fatal(err)
			}
			group := previewGrouping(t, preview, a.ID)
			if group == nil || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{s.ID, a.ID}) || group.HasExistingInputs || group.FollowsPendingBatch != (scenario == "eligible") {
				t.Fatalf("earlier %s input changed the wrong grouping facts: %+v", scenario, group)
			}
			if before != controlStateSnapshot(t, f) {
				t.Fatal("preview cancelled an obsolete author or persisted an invalid target hold")
			}
		})
	}
}

func TestCollaborationCommentPreviewDescribesSquadContinuationWithoutJoining(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	s := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	f.submitManaged(t, f.creator, "root instruction")
	f.resumeManaged(t)
	_, _, _ = f.claimManaged(t)
	input := CollaborationCommentInput{Content: previewMention(s)}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	group := previewGrouping(t, preview, s.ID)
	if group == nil || group.Kind != "squad_continuation" || group.HasExistingInputs || group.FollowsPendingBatch || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{s.ID}) {
		t.Fatalf("active squad should explain a possible next leader turn: %+v", group)
	}
	if preview.Targets[0].CommandID.Valid || preview.Targets[0].GroupID.Valid || preview.Targets[0].Code == "command_joined_squad" || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("preview claimed or persisted a continuation join")
	}
	joined := f.submitManaged(t, f.creator, "earlier unread input")
	if joined.Code != "command_joined_squad" {
		t.Fatalf("canonical submission did not agree with the preview: %+v", joined)
	}
	preview, err = f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	group = previewGrouping(t, preview, s.ID)
	if group == nil || group.Kind != "squad_continuation" || !group.HasExistingInputs {
		t.Fatalf("unread joined input should merge into the next leader turn: %+v", group)
	}
	other := managedQueueSquad(t, f)
	f.resumeManaged(t)
	f.targetID = other.ID
	pending := f.submitManaged(t, f.creator, "another squad must keep its FIFO position")
	if pending.Code != "command_queued" {
		t.Fatalf("different squad joined the current group: %+v", pending)
	}
	preview, err = f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	group = previewGrouping(t, preview, s.ID)
	if group == nil || group.Kind != "pending_batch" || !group.FollowsPendingBatch || group.HasExistingInputs {
		t.Fatalf("earlier eligible squad must prevent continuation joining: %+v", group)
	}
}

func TestCollaborationCommentPreviewPendingMergePreservesHeldAndDeniedTargets(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	a := queueTarget(t, f, f.targetID)
	s, _ := seedQueueSquad(t, f, false)
	revoked, grant := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	f.submit(t, f.creator, "private previous input")
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL WHERE id=$1`, a.ID)
	f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now() WHERE id=$1`, grant.ID)
	before := controlStateSnapshot(t, f)
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, CollaborationCommentInput{Content: previewMention(a) + previewMention(s) + previewMention(revoked)})
	if err != nil {
		t.Fatal(err)
	}
	group := previewGrouping(t, preview, a.ID)
	if group == nil || group.Kind != "pending_batch" || !group.HasExistingInputs || group.FollowsPendingBatch || !reflect.DeepEqual(group.TargetIDs, []pgtype.UUID{a.ID}) {
		t.Fatalf("eligible pending input was omitted or a held target joined: %+v", group)
	}
	if previewGrouping(t, preview, s.ID) != nil || preview.Targets[1].State != "waiting" || preview.Targets[1].HoldReason == nil || preview.Targets[2].State != "blocked" || preview.Targets[2].Grouping != nil {
		t.Fatalf("held/denied targets acquired grouping authority: %+v", preview.Targets)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private previous input", util.UUIDToString(grant.ID), util.UUIDToString(grant.RuntimeID), util.UUIDToString(revoked.ID)} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("preview exposed private input or unauthorized resource metadata: %s", private)
		}
	}
	if before != controlStateSnapshot(t, f) {
		t.Fatal("preview persisted changes to held/denied input")
	}
}

func TestCollaborationCommentPreviewContinuationRespectsNewMentionOrder(t *testing.T) {
	for _, squadFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "active_squad_first", false: "other_squad_first"}[squadFirst], func(t *testing.T) {
			f, _ := newManagedSquadFixture(t)
			active := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			other := managedQueueSquad(t, f)
			f.submitManaged(t, f.creator, "root input")
			f.resumeManaged(t)
			_, _, _ = f.claimManaged(t)
			selected := []db.CollaborationTarget{other, active}
			if squadFirst {
				selected[0], selected[1] = active, other
			}
			p := CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: previewMention(selected[0]) + previewMention(selected[1])}}
			for _, target := range selected {
				p.TargetVersions = append(p.TargetVersions, CollaborationCommentTargetVersion{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion})
			}
			preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, p.CollaborationCommentInput)
			if err != nil {
				t.Fatal(err)
			}
			group := previewGrouping(t, preview, active.ID)
			if group == nil || (group.Kind == "squad_continuation") != squadFirst || group.FollowsPendingBatch == squadFirst {
				t.Fatalf("new mention order did not fence a continuation behind pending work: %+v", group)
			}
			receipt, err := f.svc.CreateComment(f.ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range receipt.Targets {
				if target.TargetID == active.ID && (target.Code == "command_joined_squad") != squadFirst {
					t.Fatalf("preview did not match canonical submission order: %+v", receipt)
				}
			}
		})
	}
}

func TestCollaborationCommentPreviewContinuationStopsAtCurrentBoundaries(t *testing.T) {
	for _, scenario := range []string{"gate_disabled", "issue_paused", "channel_paused", "held", "sealed", "installation_stopped", "stale_frozen_grant"} {
		t.Run(scenario, func(t *testing.T) {
			f, _ := newManagedSquadFixture(t)
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			f.submitManaged(t, f.creator, "root input")
			f.resumeManaged(t)
			_, _, claim := f.claimManaged(t)
			switch scenario {
			case "gate_disabled":
				f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false WHERE id=1`)
			case "issue_paused":
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='paused' WHERE id=$1`, f.issue.SessionID)
			case "channel_paused":
				f.fx.Exec(t, `UPDATE collaboration_channel SET queue_state='paused' WHERE id=$1`, claim.Identity.ChannelID)
				f.fx.Exec(t, `UPDATE collaboration_target SET queue_state='paused' WHERE channel_id=$1`, claim.Identity.ChannelID)
			case "held":
				f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='switch_hold' WHERE id=$1`, target.ID)
			case "sealed":
				f.fx.Exec(t, `UPDATE collaboration_execution_group SET expansion_sealed_at=now() WHERE id=$1`, claim.Identity.GroupID)
			case "installation_stopped":
				f.fx.Exec(t, `UPDATE managed_installation SET admission='stopped' WHERE id=$1`, f.installation.ID)
			case "stale_frozen_grant":
				f.fx.Exec(t, `UPDATE collaboration_grant SET epoch=epoch+1 WHERE target_id=$1`, target.ID)
			}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, CollaborationCommentInput{Content: previewMention(target)})
			if err != nil {
				t.Fatal(err)
			}
			group := previewGrouping(t, preview, target.ID)
			if scenario == "held" {
				if group != nil {
					t.Fatalf("held input gained a grouping: %+v", group)
				}
			} else if group == nil || group.Kind != "pending_batch" || group.HasExistingInputs || group.FollowsPendingBatch {
				t.Fatalf("%s was shown as a continuation: %+v", scenario, group)
			}
			if before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatal("preview changed control or frozen execution state")
			}
		})
	}
}

func TestCollaborationCommentPreviewSplitsAtRuntimeInputBudget(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	a := queueTarget(t, f, f.targetID)
	s, _ := seedQueueSquad(t, f, false)
	for range 30 {
		f.submit(t, f.creator, strings.Repeat("x", 64<<10))
	}
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL WHERE session_id=$1`, f.issue.SessionID)
	mentions := previewMention(a) + previewMention(s)
	input := CollaborationCommentInput{Content: mentions + strings.Repeat("y", (64<<10)-len(mentions))}
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	first, second := previewGrouping(t, preview, a.ID), previewGrouping(t, preview, s.ID)
	if first == nil || second == nil || !reflect.DeepEqual(first.TargetIDs, []pgtype.UUID{a.ID}) || !first.HasExistingInputs || first.FollowsPendingBatch ||
		!reflect.DeepEqual(second.TargetIDs, []pgtype.UUID{s.ID}) || second.HasExistingInputs || !second.FollowsPendingBatch {
		t.Fatalf("bounded runtime input must split otherwise compatible targets: %+v %+v", first, second)
	}
	content := &collaborationContentFixture{collaborationFixture: f.collaborationFixture, issue: f.issue}
	receipt, err := f.svc.CreateComment(f.ctx, content.input(input.Content, a.ID, s.ID))
	if err != nil {
		t.Fatal(err)
	}
	selected := selectFixtureFIFO(t, f, receipt.Targets[0].ChannelID)
	if len(selected.inputs) != 31 || selected.inputs[30].TargetID != a.ID || selected.refusal != "batch_capacity_exceeded" {
		t.Fatalf("preview split differs from canonical runtime budget: count=%d refusal=%s", len(selected.inputs), selected.refusal)
	}
}
