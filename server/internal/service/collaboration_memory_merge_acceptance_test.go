package service

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationMemoryConflictRefreshMergeRetainsBothFacts(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	const baseText = "Shared baseline: inspect the release before approval."
	f.memoryUpdate(t, f.creator, 0, baseText)
	predecessor, err := f.svc.Queries.GetCollaborationMemoryVersion(f.ctx, db.GetCollaborationMemoryVersionParams{IssueID: f.issue.Issue.ID, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	type writer struct {
		actor, fact string
		request     CollaborationMemoryUpdateParams
	}
	writers := []writer{
		{actor: f.creator, fact: "Fact A: the migration rehearsal preserved all existing rows."},
		{actor: f.handler, fact: "Fact B: the Desktop restart restored the paused task."},
	}
	for i := range writers {
		view, err := f.svc.GetMemory(collaborationContext(writers[i].actor), f.issue.Issue.ID)
		if err != nil || view.Version != 1 || view.Content != baseText {
			t.Fatalf("writer did not observe the same predecessor: %v", err)
		}
		writers[i].request = CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(),
			ExpectedVersion: view.Version, Content: view.Content + "\n" + writers[i].fact}
	}
	type result struct {
		writer  int
		receipt CollaborationMemoryReceipt
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(writers))
	for i, w := range writers {
		go func() {
			<-start
			receipt, err := f.svc.UpdateMemory(collaborationContext(w.actor), w.request)
			results <- result{i, receipt, err}
		}()
	}
	close(start)
	winner, loser := -1, -1
	var refusal CollaborationMemoryReceipt
	for range writers {
		out := <-results
		switch {
		case out.err == nil && out.receipt.Status == http.StatusOK && out.receipt.Version != nil && *out.receipt.Version == 2:
			if winner != -1 {
				t.Fatal("two writers replaced the same memory predecessor")
			}
			winner = out.writer
		case collaborationErrorCode(out.err, "stale_memory_version") && out.receipt.Status == http.StatusConflict && out.receipt.CurrentVersion == 2 && out.receipt.Version == nil:
			loser, refusal = out.writer, out.receipt
		default:
			t.Fatalf("unexpected concurrent memory receipt: %+v %v", out.receipt, out.err)
		}
	}
	if winner == -1 || loser == -1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 2 {
		t.Fatal("concurrent replacement did not preserve exactly one winner and one refusal")
	}
	winningVersion, err := f.svc.Queries.GetCollaborationMemoryVersion(f.ctx, db.GetCollaborationMemoryVersionParams{IssueID: f.issue.Issue.ID, Version: 2})
	if err != nil {
		t.Fatal(err)
	}
	loserCtx := collaborationContext(writers[loser].actor)
	latest, err := f.svc.GetMemory(loserCtx, f.issue.Issue.ID)
	if err != nil || latest.Version != 2 || latest.Content != writers[winner].request.Content || latest.Author == nil ||
		latest.Author.Member == nil || latest.Author.Member.UserID != util.MustParseUUID(writers[winner].actor) {
		t.Fatalf("refresh did not return the winner's current content and author: %v", err)
	}
	merged := CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: writers[loser].request.RequestID,
		ExpectedVersion: latest.Version, Content: latest.Content + "\n" + writers[loser].fact}
	if _, err = f.svc.UpdateMemory(loserCtx, merged); !collaborationErrorCode(err, "idempotency_conflict") ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 2 {
		t.Fatalf("a changed retry repurposed the refused intent: %v", err)
	}
	merged.RequestID = uuid.NewString()
	accepted, err := f.svc.UpdateMemory(loserCtx, merged)
	if err != nil || accepted.Version == nil || *accepted.Version != 3 || accepted.CurrentVersion != 3 ||
		accepted.RequestID != merged.RequestID || accepted.RequestID == refusal.RequestID {
		t.Fatalf("explicit new-intent merge failed: %+v %v", accepted, err)
	}
	current, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || current.Version != 3 || current.Content != merged.Content || current.Bytes != len(merged.Content) || current.Author == nil ||
		current.Author.Member == nil || current.Author.Member.UserID != util.MustParseUUID(writers[loser].actor) {
		t.Fatalf("merged document lost its replacement author/version: %v", err)
	}
	for _, w := range writers {
		if strings.Count(current.Content, w.fact) != 1 {
			t.Fatal("refresh/merge lost or duplicated one author's distinct fact")
		}
	}
	for _, saved := range []db.CollaborationMemoryVersion{predecessor, winningVersion} {
		retained, err := f.svc.Queries.GetCollaborationMemoryVersion(f.ctx, db.GetCollaborationMemoryVersionParams{IssueID: f.issue.Issue.ID, Version: saved.Version})
		if err != nil || !reflect.DeepEqual(retained, saved) {
			t.Fatalf("merge rewrote immutable predecessor v%d: %v", saved.Version, err)
		}
	}
	history, err := f.svc.ListMemoryVersions(loserCtx, f.issue.Issue.ID, CollaborationMemoryVersionsQuery{Limit: 10})
	if err != nil || history.CurrentVersion != 3 || len(history.Items) != 3 || history.NextCursor != nil {
		t.Fatalf("complete retained version history: %v", err)
	}
	for i, want := range []struct {
		version int64
		author  string
		content string
	}{{3, writers[loser].actor, merged.Content}, {2, writers[winner].actor, writers[winner].request.Content}, {1, f.creator, baseText}} {
		item := history.Items[i]
		detail, err := f.svc.GetMemoryVersion(loserCtx, f.issue.Issue.ID, want.version)
		if err != nil || item.Version != want.version || item.Author == nil || item.Author.Kind != "member" || item.Author.Member == nil ||
			item.Author.Member.UserID != util.MustParseUUID(want.author) || item.Purged || item.CreatedAt.IsZero() ||
			detail.Content == nil || *detail.Content != want.content || !reflect.DeepEqual(detail.Author, item.Author) {
			t.Fatalf("v%d lost content/author provenance in public history: %v", want.version, err)
		}
	}
	replayed, err := f.svc.UpdateMemory(loserCtx, writers[loser].request)
	if !collaborationErrorCode(err, "stale_memory_version") || !sameMemoryReceipt(t, replayed, refusal) {
		t.Fatalf("later merge changed the original refusal receipt: %+v %v", replayed, err)
	}
	replayed, err = f.svc.UpdateMemory(loserCtx, merged)
	if err != nil || !sameMemoryReceipt(t, replayed, accepted) || f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 3 {
		t.Fatalf("merged intent replay created another version: %v", err)
	}
}
