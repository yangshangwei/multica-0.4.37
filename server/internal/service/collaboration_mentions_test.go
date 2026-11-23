package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationMentionParserKeepsTypedIdentityAndExplicitTargets(t *testing.T) {
	const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	content := "[@person](mention://member/" + id + ") [@all](mention://all/all) " +
		"[@agent](mention://agent/" + id + ") [@again](mention://agent/" + strings.ToUpper(id) + ") " +
		"[@squad](mention://squad/" + id + ")"
	mentions := parseCollaborationMentions(content)
	if !mentions.All || !mentions.People[util.MustParseUUID(id)] || len(mentions.People) != 1 {
		t.Fatalf("lost independent human mentions: %+v", mentions)
	}
	if len(mentions.Targets) != 2 || mentions.Targets[0].Kind != collaboration.TargetAgent || mentions.Targets[1].Kind != collaboration.TargetSquad ||
		mentions.Targets[0].ResourceID != mentions.Targets[1].ResourceID {
		t.Fatalf("typed targets must deduplicate UUID case while preserving agent/squad roles: %+v", mentions.Targets)
	}
}

func TestCollaborationMentionParserLeavesPlainNamesAndUnresolvedTargetsInert(t *testing.T) {
	for _, content := range []string{"@AI @person", "[@AI](mention://agent/-)", "[@all](mention://all/aaaaaaaa)", "[issue](mention://issue/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa)"} {
		mentions := parseCollaborationMentions(content)
		if mentions.All || len(mentions.People) != 0 || len(mentions.Targets) != 0 {
			t.Fatalf("unresolved content gained authority: %+v", mentions)
		}
	}
}

func TestCollaborationCommentPreviewIsReadOnlyAndKeepsIndependentPeopleAndTargets(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	input := CollaborationCommentInput{Content: "[@H](mention://member/" + f.handler + ") " +
		"[@self](mention://member/" + f.creator + ") [@outside](mention://member/" + f.outsider + ") " +
		"[@A](mention://agent/" + util.UUIDToString(target.ResourceID) + ") " +
		"[@invalid](mention://agent/" + uuid.NewString() + ")"}
	beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
	preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil || len(preview.Targets) != 2 || preview.Targets[0].State != "waiting" || preview.Targets[0].WaitingReason != "issue_paused" ||
		preview.Targets[0].CommandID.Valid || preview.Targets[1].State != "blocked" || preview.Targets[1].TargetID.Valid ||
		len(preview.Recipients) != 1 || preview.Recipients[0].Recipient.UserID != util.MustParseUUID(f.handler) || !preview.Recipients[0].IsMention {
		t.Fatalf("preview mixed effects incorrectly: %+v %v", preview, err)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Targets []struct {
			HoldReason  *string `json:"hold_reason"`
			HoldVersion *string `json:"hold_version"`
		} `json:"targets"`
	}
	if err = json.Unmarshal(encoded, &wire); err != nil || len(wire.Targets) != 2 || wire.Targets[0].HoldVersion == nil ||
		*wire.Targets[0].HoldVersion != strconv.FormatInt(target.HoldVersion, 10) || wire.Targets[0].HoldReason == nil || *wire.Targets[0].HoldReason != target.HoldReason.String ||
		wire.Targets[1].HoldVersion != nil || wire.Targets[1].HoldReason != nil {
		t.Fatalf("hold feedback lost exact versions or invented unavailable-target state: %s, %v", encoded, err)
	}
	input.SuppressedTargetIDs = append(input.SuppressedTargetIDs, target.ID)
	suppressed, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, input)
	if err != nil || suppressed.Targets[0].State != "suppressed" || len(suppressed.Recipients) != 1 {
		t.Fatalf("suppression changed text recipients: %+v %v", suppressed, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != beforeEvents {
		t.Fatal("preview performed a persistence side effect")
	}
}

func TestCollaborationCommentPreviewDoesNotPromoteSourceMembershipToExecution(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	preview, err := f.svc.PreviewComment(collaborationContext(f.outsider), f.issue.Issue.ID, CollaborationCommentInput{
		Content: "[@A](mention://agent/" + util.UUIDToString(target.ResourceID) + ")"})
	if err != nil || len(preview.Targets) != 1 || preview.Targets[0].State != "blocked" || preview.Targets[0].Code != "not_participant" || preview.WriteContext != nil {
		t.Fatalf("source-only preview gained queue authority: %+v %v", preview, err)
	}
}
