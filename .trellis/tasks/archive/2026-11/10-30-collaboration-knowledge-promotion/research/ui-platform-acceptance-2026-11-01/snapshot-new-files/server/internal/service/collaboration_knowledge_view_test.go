package service

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeHumanListDetailHistoryAndTargetSuppressPurgedSourceText(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.SupersedesCandidateID = &p.CandidateID
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RejectKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "PRIVATE_KNOWLEDGE_REASON_QUOTE"}); err != nil {
		t.Fatal(err)
	}
	before, err := f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || before.Candidate.Content == nil || *before.Candidate.Content != p.Content {
		t.Fatalf("available detail: %+v %v", before, err)
	}
	f.fx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=now(),purged_by_user_id=$2,purge_reason='remove source' WHERE issue_id=$1`, f.issue.Issue.ID, f.creator)
	list, err := f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(second.CandidateID))
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.svc.ListTargetKnowledge(f.ctx, p.Target, KnowledgePageQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for name, view := range map[string]any{"source list": list, "detail and history": detail, "target": target} {
		raw, _ := json.Marshal(view)
		if strings.Contains(string(raw), "PRIVATE_KNOWLEDGE") || strings.Contains(string(raw), `"content_sha256"`) || strings.Contains(string(raw), `"sources"`) {
			t.Fatalf("%s leaked derived text/ref/hash: %s", name, raw)
		}
	}
	if detail.Candidate.Availability != "source_unavailable" || len(list.Items) != 2 || len(target.Items) != 1 {
		t.Fatal("purge removed audit routing instead of suppressing content")
	}
}

func TestKnowledgeWithdrawnOrExpiredHistoryRemainsReadableWhenSourceAvailable(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "not needed"}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || view.Candidate.Availability != "available" || view.Candidate.Content == nil || *view.Candidate.Content != p.Content {
		t.Fatalf("withdrawal erased lawful history: %+v %v", view, err)
	}
	p.CandidateID = uuid.NewString()
	p.RequestID = uuid.NewString()
	expires := time.Now().Add(2 * time.Second).UTC()
	p.ExpiresAt = &expires
	if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(expires) + 10*time.Millisecond)
	view, err = f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || view.Candidate.Availability != "available" || view.Candidate.Capabilities.Approve || view.Candidate.Content == nil {
		t.Fatalf("expiry changed lawful history/approval: %+v %v", view, err)
	}
	r, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p))
	if !collaborationErrorCode(err, "knowledge_expired") || r.Code != "knowledge_expired" {
		t.Fatalf("expired approval: %+v %v", r, err)
	}
}

func TestKnowledgeBoundedHistoryRemainsReachableThroughSourcePaging(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	root := p.CandidateID
	for i := 1; i <= 51; i++ {
		r, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p)
		if err != nil || r.Version != strconv.Itoa(i) {
			t.Fatalf("version %d: %+v %v", i, r, err)
		}
		previous := p.CandidateID
		p.CandidateID = uuid.NewString()
		p.RequestID = uuid.NewString()
		p.SupersedesCandidateID = &previous
	}
	detail, err := f.svc.KnowledgeCandidate(f.ctx, util.MustParseUUID(root))
	if err != nil || len(detail.History) != 50 || !detail.HistoryTruncated {
		t.Fatalf("bounded history: %d %v %v", len(detail.History), detail.HistoryTruncated, err)
	}
	query := KnowledgePageQuery{Limit: 20}
	seen := map[string]bool{}
	var checked, until time.Time
	for {
		page, err := f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, query)
		if err != nil {
			t.Fatal(err)
		}
		if checked.IsZero() {
			checked, until = page.CheckedAt, page.ValidUntil
		} else if page.CheckedAt != checked || page.ValidUntil != until {
			t.Fatal("later page renewed the authorization lease")
		}
		for _, item := range page.Items {
			if seen[item.CandidateID] {
				t.Fatal("duplicate version across pages")
			}
			seen[item.CandidateID] = true
			if item.Content != nil {
				t.Fatal("list carried full text")
			}
		}
		if page.NextCursor == nil {
			break
		}
		query.Cursor = *page.NextCursor
	}
	if len(seen) != 51 || !seen[root] {
		t.Fatalf("older history stranded: %d versions root=%v", len(seen), seen[root])
	}
}

func TestKnowledgePagingRejectsExpiredForeignAndMalformedBoundary(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	for i := 0; i < 2; i++ {
		p.CandidateID = uuid.NewString()
		p.RequestID = uuid.NewString()
		if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{Limit: 1})
	if err != nil || page.NextCursor == nil {
		t.Fatalf("cursor setup: %+v %v", page, err)
	}
	if _, err = f.svc.ListKnowledgeCandidates(collaborationContext(f.handler), f.issue.Issue.ID, KnowledgePageQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "invalid_knowledge_request") {
		t.Fatalf("other viewer reused projection boundary: %v", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(*page.NextCursor)
	var cursor knowledgePageCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		t.Fatal(err)
	}
	cursor.CheckedAt = time.Now().Add(-time.Minute)
	cursor.ValidUntil = cursor.CheckedAt.Add(30 * time.Second)
	raw, _ = json.Marshal(cursor)
	if _, err = f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(raw)}); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("expired page boundary renewed: %v", err)
	}
	raw, _ = base64.RawURLEncoding.DecodeString(*page.NextCursor)
	raw = []byte(strings.Replace(string(raw), `"viewer_id"`, `"Viewer_ID"`, 1))
	if _, err = f.svc.ListKnowledgeCandidates(f.ctx, f.issue.Issue.ID, KnowledgePageQuery{Cursor: base64.RawURLEncoding.EncodeToString(raw)}); !collaborationErrorCode(err, "invalid_knowledge_request") {
		t.Fatalf("case alias boundary accepted: %v", err)
	}
}
