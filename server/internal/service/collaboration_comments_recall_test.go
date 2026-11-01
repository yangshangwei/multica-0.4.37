package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func recallTestIdentity() protocol.CollaborationRunIdentity {
	return protocol.CollaborationRunIdentity{DeploymentID: uuid.NewString(), InstallationID: uuid.NewString(), IssueID: uuid.NewString(), SessionID: uuid.NewString(),
		TargetID: uuid.NewString(), ChannelID: uuid.NewString(), BatchID: uuid.NewString(), GroupID: uuid.NewString(), RunID: uuid.NewString(), LaunchNonce: uuid.NewString(), Fence: "1", ClaimGeneration: "1"}
}

func TestCollaborationRecallBudgetCountsEscapingEnvelopeAndAdvances(t *testing.T) {
	id := recallTestIdentity()
	makeComment := func(body string) protocol.CollaborationRecallComment {
		return protocol.CollaborationRecallComment{ID: uuid.NewString(), IssueID: id.IssueID, AuthorType: "agent", AuthorID: uuid.NewString(), Content: body, CommentRevision: "1", CreatedAt: time.Now().UTC(),
			CollaborationCommentContent: protocol.CollaborationCommentContent{Trust: "data_only", ContentState: "complete", ContentBytes: len(body), AuthorAuthority: protocol.CollaborationCommentAuthorAuthority{Role: "agent", EvaluatedAt: time.Now().UTC()}}}
	}
	giant := makeComment(strings.Repeat("<>&你🙂\n", 2000))
	small := makeComment("older")
	p := CollaborationCommentsRecallQuery{Limit: 50, MaxBytes: 4096}
	page, err := budgetCollaborationComments(id.IssueID, &id, []protocol.CollaborationRecallComment{giant, small}, 2, false, p,
		func(c protocol.CollaborationRecallComment) string {
			return protocol.EncodeCollaborationCommentCursor(c.CreatedAt, c.ID)
		},
		func(c protocol.CollaborationRecallComment) protocol.CollaborationRecallComment {
			c.Content = ""
			c.ContentState = "omitted"
			c.OmissionReason = "byte_budget"
			c.ContentRef = "/api/collaboration/runtime/runs/" + id.RunID + "/comments/" + c.ID + "/content?revision=1"
			return c
		})
	if err != nil || page.Returned != 1 || page.Items[0].ID != giant.ID || page.Items[0].ContentState != "omitted" || page.NextCursor == nil || !page.Truncated {
		t.Fatalf("oversized first row did not make explicit progress: %+v %v", page, err)
	}
	raw, _ := json.Marshal(page)
	if len(raw)+1 != page.Bytes || page.Bytes > p.MaxBytes {
		t.Fatalf("page budget mismatch: %d/%d", len(raw)+1, page.Bytes)
	}
	if _, err := protocol.DecodeCollaborationCommentsRecall(raw); err != nil {
		t.Fatal(err)
	}
	cursor, err := protocol.DecodeCollaborationCommentCursor(*page.NextCursor)
	if err != nil || cursor.ID != giant.ID {
		t.Fatal("page did not advance past its placeholder")
	}
	page, err = budgetCollaborationComments(id.IssueID, &id, []protocol.CollaborationRecallComment{small, small}, 2, false, p, func(c protocol.CollaborationRecallComment) string { return c.ID }, func(c protocol.CollaborationRecallComment) protocol.CollaborationRecallComment { return c })
	if err != nil || page.NextCursor != nil || page.Truncated {
		t.Fatal("final page retained an earlier next cursor", err)
	}
	// Even an empty page must respect its envelope budget.
	id.DeploymentID = strings.Repeat("x", 5000)
	_, err = budgetCollaborationComments(id.IssueID, &id, []protocol.CollaborationRecallComment{}, 0, false, p, func(c protocol.CollaborationRecallComment) string { return c.ID }, func(c protocol.CollaborationRecallComment) protocol.CollaborationRecallComment { return c })
	if !collaborationErrorCode(err, "budget_too_small") {
		t.Fatalf("metadata escaped budget: %v", err)
	}
}

func TestCollaborationRecallChunksReassembleUTF8AndRejectCrossBinding(t *testing.T) {
	identity := recallTestIdentity()
	comment := uuid.NewString()
	body := strings.Repeat("中文🙂 <>&\n\\\"", 3000)
	p := CollaborationCommentContentQuery{Revision: "9007199254740993", MaxBytes: 4096}
	var assembled strings.Builder
	var first string
	for n := 0; n < 200; n++ {
		chunk, err := chunkCollaborationComment(identity, comment, body, p)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(chunk)
		if chunk.Bytes != len(raw)+1 || chunk.Bytes > p.MaxBytes || chunk.Offset != assembled.Len() || !utf8.ValidString(chunk.Content) {
			t.Fatal("incorrect content chunk boundary/budget")
		}
		if _, err := protocol.DecodeCollaborationCommentContent(raw); err != nil {
			t.Fatal(err)
		}
		assembled.WriteString(chunk.Content)
		if chunk.Complete {
			break
		}
		if chunk.NextCursor == nil || *chunk.NextCursor == p.Cursor {
			t.Fatal("chunk did not progress")
		}
		p.Cursor = *chunk.NextCursor
		if first == "" {
			first = p.Cursor
		}
	}
	if assembled.String() != body {
		t.Fatalf("UTF8 body lost bytes: %d/%d", assembled.Len(), len(body))
	}
	p.Cursor = first
	p.Revision = "2"
	if _, err := chunkCollaborationComment(identity, comment, body, p); !collaborationErrorCode(err, "invalid_cursor") {
		t.Fatal("mixed revisions accepted")
	}
	p.Revision = "9007199254740993"
	if _, err := chunkCollaborationComment(identity, uuid.NewString(), body, p); !collaborationErrorCode(err, "invalid_cursor") {
		t.Fatal("another comment accepted")
	}
}

func TestCollaborationRecallRuntimePagesFulltextAndCurrentAuthority(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "admitted input")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, run, claim)
	issue := util.UUIDToString(f.issue.Issue.ID)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	at := time.Now().UTC().Truncate(time.Second)
	want := make(map[string]bool)
	for n := 0; n < 205; n++ {
		author := f.creator
		if n%2 == 0 {
			author = f.outsider
		}
		id := f.fx.Comment(t, issue, "comment 中文🙂 <>&", testutil.Cols{"author_id": author, "created_at": at})
		want[id] = true
	}
	body := strings.Repeat("<>&中文🙂\n", 2500)
	longID := f.fx.Comment(t, issue, body, testutil.Cols{"author_id": f.creator, "created_at": at.Add(time.Second)})
	want[longID] = true
	f.fx.Comment(t, issue, "", testutil.Cols{"author_id": f.creator, "deleted_at": at})
	p := CollaborationCommentsRecallQuery{Limit: 31, MaxBytes: 4096}
	seen := make(map[string]bool)
	for n := 0; n < 220; n++ {
		page, err := f.svc.RecallRuntimeComments(context.Background(), run, token, p)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 206 || page.Bytes > p.MaxBytes || len(page.Items) == 0 {
			t.Fatalf("bad visible page total/budget: %+v", page)
		}
		for i, item := range page.Items {
			if !want[item.ID] || seen[item.ID] {
				t.Fatal("skipped/deleted/duplicate comment", item.ID)
			}
			if item.Trust != "data_only" {
				t.Fatal("ordinary comment gained instruction trust")
			}
			if item.AuthorID == f.outsider && (item.AuthorAuthority.CanSubmitCommand || item.AuthorAuthority.Role != "source_member") {
				t.Fatal("source membership became submission authority")
			}
			if item.AuthorID == f.creator && !item.AuthorAuthority.CanSubmitCommand {
				t.Fatal("current participant capability missing")
			}
			if i > 0 && page.Items[i-1].CreatedAt.Equal(item.CreatedAt) && page.Items[i-1].ID > item.ID {
				t.Fatal("same timestamp page order is unstable")
			}
			seen[item.ID] = true
		}
		if n == 0 && (page.Items[0].ID != longID || page.Items[0].ContentState != "omitted") {
			t.Fatal("long comment did not preserve recoverable identity")
		}
		if page.NextCursor == nil {
			break
		}
		if *page.NextCursor == p.Cursor {
			t.Fatal("cursor cycle")
		}
		p.Cursor = *page.NextCursor
	}
	if len(seen) != len(want) {
		t.Fatalf("only recalled %d of %d comments", len(seen), len(want))
	}
	legacy, err := f.svc.RuntimeContext(context.Background(), run, token)
	if err != nil || len(legacy.Comments) != 200 {
		t.Fatal("legacy context newest 200 changed", err)
	}
	human, err := f.svc.RecallComments(f.ctx, f.issue.Issue.ID, CollaborationCommentsRecallQuery{Limit: 50, MaxBytes: 4096})
	if err != nil || human.Total != 207 {
		t.Fatal("human tombstone visibility changed", err)
	}
	cp := CollaborationCommentContentQuery{Revision: "1", MaxBytes: 4096}
	var assembled strings.Builder
	for n := 0; n < 100; n++ {
		chunk, err := f.svc.RuntimeCommentContent(context.Background(), run, token, util.MustParseUUID(longID), cp)
		if err != nil {
			t.Fatal(err)
		}
		assembled.WriteString(chunk.Content)
		if chunk.Complete {
			break
		}
		cp.Cursor = *chunk.NextCursor
	}
	if assembled.String() != body {
		t.Fatal("runtime fulltext lost content")
	}
	f.fx.Exec(t, `UPDATE comment SET revision=revision+1,content='edited' WHERE id=$1`, longID)
	if _, err := f.svc.RuntimeCommentContent(context.Background(), run, token, util.MustParseUUID(longID), cp); !collaborationErrorCode(err, "stale_comment_revision") {
		t.Fatal("edited content was spliced into an old revision", err)
	}
	f.fx.Exec(t, `UPDATE comment SET deleted_at=now() WHERE id=$1`, longID)
	cp.Revision = "2"
	cp.Cursor = ""
	if _, err := f.svc.RuntimeCommentContent(context.Background(), run, token, util.MustParseUUID(longID), cp); !collaborationErrorCode(err, "comment_not_found") {
		t.Fatal("deleted content remained readable", err)
	}
	if _, err := f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	p.Cursor = ""
	if _, err := f.svc.RecallRuntimeComments(context.Background(), run, token, p); err == nil {
		t.Fatal("Stop did not revoke recall")
	}
	if _, err := f.svc.RuntimeCommentContent(context.Background(), run, token, util.MustParseUUID(longID), cp); err == nil {
		t.Fatal("Stop did not revoke content recall")
	}
}

// Parent PRD R4 asks runtime comment reads for the author's display name, as
// the human comment list already has (audit RECALL-1). Agent authors keep the
// human path's boundary: their type and ID, no name.
func TestCollaborationRuntimeCommentsCarryMemberDisplayNames(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "admitted input")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, run, claim)
	issue := util.UUIDToString(f.issue.Issue.ID)
	agent := util.UUIDToString(f.grant.AgentID)
	byCreator := f.fx.Comment(t, issue, "we rejected option B", testutil.Cols{"author_id": f.creator})
	byHandler := f.fx.Comment(t, issue, "handler note", testutil.Cols{"author_id": f.handler})
	byAgent := f.fx.Comment(t, issue, "agent note", testutil.Cols{"author_type": "agent", "author_id": agent})
	want := map[string]string{byCreator: "Collaboration creator", byHandler: "External handler", byAgent: ""}

	page, err := f.svc.RecallRuntimeComments(context.Background(), run, token, CollaborationCommentsRecallQuery{Limit: 50, MaxBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, item := range page.Items {
		name, ok := want[item.ID]
		if !ok {
			continue
		}
		found++
		if item.AuthorName != name {
			t.Fatalf("recall comment %s author name %q, want %q", item.ID, item.AuthorName, name)
		}
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := protocol.DecodeCollaborationCommentsRecall(raw)
	if found != len(want) || err != nil || !reflect.DeepEqual(decoded.Items, page.Items) {
		t.Fatalf("recall page lost names or the strict decoder refused them: found=%d err=%v", found, err)
	}

	legacy, err := f.svc.RuntimeContext(context.Background(), run, token)
	if err != nil {
		t.Fatal(err)
	}
	found = 0
	for _, item := range legacy.Comments {
		name, ok := want[util.UUIDToString(item.ID)]
		if !ok {
			continue
		}
		found++
		if item.AuthorName != name {
			t.Fatalf("legacy context comment author name %q, want %q", item.AuthorName, name)
		}
	}
	if found != len(want) {
		t.Fatalf("legacy context returned %d of %d seeded comments", found, len(want))
	}
}
