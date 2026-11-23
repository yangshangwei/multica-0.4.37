package service

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

// pickerParticipant seeds participation on an issue id outside the fixture
// workspace; the picker reads only issue_participant.
func pickerParticipant(t *testing.T, f *collaborationFixture, issue, user string, joined time.Time, revoked *time.Time) {
	t.Helper()
	cols := testutil.Cols{"issue_id": issue, "user_id": user, "joined_at": joined}
	if revoked != nil {
		cols["revoked_at"] = *revoked
	}
	f.fx.InsertNoID(t, "issue_participant", cols, "issue_id=$1 AND user_id=$2", issue, user)
}

func pickerSorted(ids ...string) []string { slices.Sort(ids); return ids }

func TestCollaborationRecipientSearchRelatedScopeAndJoinedAfterLeaving(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	suffix := pickerSuffix()
	f.fx.Exec(t, `UPDATE "user" SET name=$2 WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler, f.outsider}, "Related "+suffix)
	mate := pickerAccount(t, f.collaborationFixture, "Related "+suffix)
	f.fx.Member(t, f.workspace, mate, "member")
	early, late := pickerAccount(t, f.collaborationFixture, "Related "+suffix), pickerAccount(t, f.collaborationFixture, "Related "+suffix)
	search := func(q string, directory bool) ([]string, []string) {
		t.Helper()
		result, err := f.svc.SearchRecipients(f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: q, Directory: directory})
		want := "related"
		if directory {
			want = "directory"
		}
		if err != nil || result.Scope != want || !result.SourceMember || result.IssueID.Valid {
			t.Fatalf("search scope %s: %+v, %v", want, result, err)
		}
		return pickerIDs(result.Recent), pickerIDs(result.Recipients)
	}
	// The handler shares only the fixture issue; the mate shares only the workspace.
	if _, got := search(suffix, false); !slices.Equal(got, pickerSorted(f.handler, mate)) {
		t.Fatalf("related scope listed %v, want handler %s and mate %s only", got, f.handler, mate)
	}
	if _, got := search(suffix, true); !slices.Equal(got, pickerSorted(f.handler, mate, f.outsider, early, late)) {
		t.Fatalf("directory scope listed %v", got)
	}
	// The creator left another issue an hour ago: early joined before that, late after.
	now := time.Now()
	left := now.Add(-time.Hour)
	other := uuid.NewString()
	pickerParticipant(t, f.collaborationFixture, other, f.creator, now.Add(-3*time.Hour), &left)
	pickerParticipant(t, f.collaborationFixture, other, early, now.Add(-2*time.Hour), nil)
	pickerParticipant(t, f.collaborationFixture, other, late, now.Add(-30*time.Minute), nil)
	pickerParticipant(t, f.collaborationFixture, uuid.NewString(), f.outsider, now.Add(-time.Minute), nil)
	if _, got := search(suffix, false); !slices.Equal(got, pickerSorted(f.handler, mate, early)) {
		t.Fatalf("related scope listed %v; early %s shared an issue, late %s joined after the caller left", got, early, late)
	}
	shared := uuid.NewString()
	pickerParticipant(t, f.collaborationFixture, shared, f.creator, now.Add(-time.Minute), nil)
	pickerParticipant(t, f.collaborationFixture, shared, f.outsider, now.Add(-time.Minute), nil)
	if _, got := search(suffix, false); !slices.Equal(got, pickerSorted(f.handler, mate, early, f.outsider)) {
		t.Fatalf("a current co-participant is related: %v", got)
	}
	if recent, _ := search("", false); slices.Contains(recent, late) || !slices.Contains(recent, early) {
		t.Fatalf("recent %v must include early %s and exclude late %s", recent, early, late)
	}
}

func TestCollaborationRecipientSearchRecentOrderAndLimit(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	now := time.Now()
	at := func(minutes int) time.Time { return now.Add(-time.Duration(minutes) * time.Minute) }
	var users []string
	for range 7 {
		users = append(users, pickerAccount(t, f.collaborationFixture, "Recent "+pickerSuffix()))
	}
	a, b, c, d, e, late, disabled := users[0], users[1], users[2], users[3], users[4], users[5], users[6]
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, disabled)
	seed := func(creatorJoined, otherJoined int, other string, creatorLeft *int) {
		issue := uuid.NewString()
		var revoked *time.Time
		if creatorLeft != nil {
			value := at(*creatorLeft)
			revoked = &value
		}
		pickerParticipant(t, f.collaborationFixture, issue, f.creator, at(creatorJoined), revoked)
		pickerParticipant(t, f.collaborationFixture, issue, other, at(otherJoined), nil)
	}
	left := 300
	seed(500, 400, a, nil) // shared since 400; the second issue is more recent
	seed(100, 50, a, nil)  // a: 50
	seed(80, 600, b, nil)  // b: 80, the later join of the pair
	seed(400, 200, c, nil) // c: 200
	seed(700, 250, d, nil) // d: 250
	seed(900, 800, e, nil) // e: 800, cut by the five-account limit
	seed(500, 10, late, &left)
	seed(500, 5, disabled, nil)
	want := []string{f.handler, a, b, c, d}
	for _, query := range []CollaborationRecipientSearchQuery{
		{SourceWorkspaceID: util.MustParseUUID(f.workspace)},
		{SourceWorkspaceID: util.MustParseUUID(f.workspace), Directory: true},
		{IssueID: f.issue.Issue.ID},
	} {
		result, err := f.svc.SearchRecipients(f.ctx, query)
		if err != nil || !slices.Equal(pickerIDs(result.Recent), want) {
			t.Fatalf("recent = %v, want %v (late %s, disabled %s, e %s): %v", pickerIDs(result.Recent), want, late, disabled, e, err)
		}
	}
	one := 1
	first, err := f.svc.SearchRecipients(f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Limit: &one})
	if err != nil || first.NextCursor == "" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	for _, query := range []CollaborationRecipientSearchQuery{
		{SourceWorkspaceID: util.MustParseUUID(f.workspace), Cursor: first.NextCursor},
		{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: "Recent"},
	} {
		if result, err := f.svc.SearchRecipients(f.ctx, query); err != nil || len(result.Recent) != 0 {
			t.Fatalf("recent belongs only to the first unfiltered page: %+v, %v", result.Recent, err)
		}
	}
	f.fx.Member(t, f.workspace, c, "member")
	result, err := f.svc.SearchRecipients(f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), MembersOnly: true})
	if err != nil || !slices.Equal(pickerIDs(result.Recent), []string{c}) || *result.Recent[0].External {
		t.Fatalf("members_only recent = %v, want source member %s: %v", pickerIDs(result.Recent), c, err)
	}
}
