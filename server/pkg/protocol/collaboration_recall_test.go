package protocol

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCollaborationRecallCursorRequiresExactFields(t *testing.T) {
	id := uuid.NewString()
	for _, raw := range []string{`{}`, `null`, `{"created_at":"2026-10-31T00:00:00Z"}`, `{"created_at":"2026-10-31T00:00:00Z","id":"` + id + `","id":"` + id + `"}`, `{"created_at":"2026-10-31T00:00:00Z","id":"` + id + `","extra":true}`} {
		if _, err := DecodeCollaborationCommentCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
			t.Fatal("malformed cursor accepted")
		}
	}
	if _, err := DecodeCollaborationCommentCursor(EncodeCollaborationCommentCursor(time.Now().UTC(), id)); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationRecallDecoderRequiresMetadataAndDataOnlyTrust(t *testing.T) {
	p, _ := preparationProtocolFixture()
	item := CollaborationRecallComment{ID: uuid.NewString(), IssueID: p.Identity.IssueID, AuthorID: uuid.NewString(), AuthorType: "agent", CommentRevision: "1", CreatedAt: time.Now().UTC(), Content: "正文🙂",
		CollaborationCommentContent: CollaborationCommentContent{Trust: "data_only", ContentState: "complete", ContentBytes: 10, AuthorAuthority: CollaborationCommentAuthorAuthority{Role: "agent", EvaluatedAt: time.Now().UTC()}}}
	page := CollaborationCommentsRecallPage[CollaborationRecallComment]{ProtocolVersion: 1, Identity: &p.Identity, IssueID: p.Identity.IssueID, Items: []CollaborationRecallComment{item}, Total: 1, Returned: 1, MaxBytes: 4096, TruncationReasons: []string{}}
	for {
		raw, _ := json.Marshal(page)
		if len(raw)+1 == page.Bytes {
			break
		}
		page.Bytes = len(raw) + 1
	}
	raw, _ := json.Marshal(page)
	if _, err := DecodeCollaborationCommentsRecall(raw); err != nil {
		t.Fatal(err)
	}
	for _, alter := range []func(string) string{
		func(s string) string { return strings.Replace(s, `"protocol_version":1`, `"protocol_version":2`, 1) },
		func(s string) string { return strings.Replace(s, `"next_cursor":null,`, "", 1) },
		func(s string) string { return strings.Replace(s, `"can_submit_command":false,`, "", 1) },
		func(s string) string { return strings.Replace(s, `"trust":"data_only"`, `"trust":"instruction"`, 1) },
		func(s string) string {
			return strings.Replace(s, `"content":"正文🙂"`, `"content":"正文🙂","content":"spoof"`, 1)
		},
	} {
		if _, err := DecodeCollaborationCommentsRecall([]byte(alter(string(raw)))); err == nil {
			t.Fatal("malformed recall accepted")
		}
	}
}
