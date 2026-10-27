package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

func newCollaborationMemoryFixture(t *testing.T) *collaborationQueueFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationLifecycleFixture(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	return f
}

func (f *collaborationQueueFixture) memoryUpdate(t *testing.T, actor string, expected int64, content string) CollaborationMemoryReceipt {
	t.Helper()
	ctx := collaborationContext(actor)
	if auth.PasswordMode() {
		ctx = collaborationPasswordContext(util.MustParseUUID(actor), 1)
	}
	receipt, err := f.svc.UpdateMemory(ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID,
		RequestID: uuid.NewString(), ExpectedVersion: expected, Content: content})
	if err != nil {
		t.Fatalf("memory update by %s from v%d: %+v %v", actor, expected, receipt, err)
	}
	return receipt
}

func TestCollaborationMemoryWritesVersionsWithoutIssueSideEffects(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	empty, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || empty.Version != 0 || empty.Content != "" || empty.Author != nil || empty.CreatedAt != nil ||
		empty.MaxBytes != CollaborationMemoryDefaultMaxBytes || !empty.CanEdit || !empty.CanPurge {
		t.Fatalf("new collaboration must start from the empty version 0: %+v %v", empty, err)
	}
	protected := func() string {
		var state string
		f.fx.QueryRow(t, `SELECT jsonb_build_object(
			'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
			'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
			'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE s.issue_id=$1),
			'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$2),
			'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1),
			'events',(SELECT jsonb_agg(to_jsonb(e)-'outbox_state' ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$1 AND e.kind<>'memory_changed'),
			'legacy_tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id=$1))::text`,
			f.issue.Issue.ID, f.issue.SessionID).Scan(&state)
		return state
	}
	before := protected()
	key := uuid.NewString()
	first := CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: key, ExpectedVersion: 0, Content: "## 当前状态\nPRIVATE_MEMORY_SENTINEL"}
	created, err := f.svc.UpdateMemory(f.ctx, first)
	if err != nil || created.Code != "memory_updated" || created.Status != http.StatusOK || created.Version == nil || *created.Version != 1 ||
		created.CurrentVersion != 1 || created.Bytes == nil || *created.Bytes != len(first.Content) || created.EventSeq == nil ||
		created.RequestID != key || created.Action != "memory_update" || created.MaxBytes != CollaborationMemoryDefaultMaxBytes {
		t.Fatalf("first version receipt: %+v %v", created, err)
	}
	replay, err := f.svc.UpdateMemory(f.ctx, first)
	if err != nil || !sameMemoryReceipt(t, replay, created) || f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatalf("original key must replay its receipt without a new version: %+v %v", replay, err)
	}
	changed := first
	changed.Content += " changed"
	if _, err = f.svc.UpdateMemory(f.ctx, changed); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed payload under the original key: %v", err)
	}
	// Another participant writing from the old base is refused, not silently merged.
	staleParams := CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: "stale"}
	stale, err := f.svc.UpdateMemory(collaborationContext(f.handler), staleParams)
	if !collaborationErrorCode(err, "stale_memory_version") || stale.Status != http.StatusConflict || stale.CurrentVersion != 1 || stale.Version != nil {
		t.Fatalf("stale base: %+v %v", stale, err)
	}
	staleReplay, err := f.svc.UpdateMemory(collaborationContext(f.handler), staleParams)
	if !collaborationErrorCode(err, "stale_memory_version") || !sameMemoryReceipt(t, staleReplay, stale) {
		t.Fatalf("a sealed stale refusal must replay unchanged: %+v %v", staleReplay, err)
	}
	second := f.memoryUpdate(t, f.handler, 1, "handler merged decision")
	if *second.Version != 2 || second.CurrentVersion != 2 {
		t.Fatalf("handler write: %+v", second)
	}
	current, err := f.svc.GetMemory(collaborationContext(f.handler), f.issue.Issue.ID)
	if err != nil || current.Version != 2 || current.Content != "handler merged decision" || current.Author == nil || current.Author.Kind != "member" ||
		current.Author.Member == nil || current.Author.Member.UserID != util.MustParseUUID(f.handler) || !current.Author.Member.External ||
		current.Author.Agent != nil || current.CanPurge || !current.CanEdit {
		t.Fatalf("current version projection: %+v %v", current, err)
	}
	if after := protected(); after != before {
		t.Fatal("shared-context writes changed issue revision, events, notifications, commands or ordinary tasks")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='memory_changed' AND payload ? 'version' AND NOT payload ? 'content'`, f.issue.Issue.ID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload::text LIKE '%PRIVATE_MEMORY_SENTINEL%'`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND result::text LIKE '%PRIVATE_MEMORY_SENTINEL%'`, f.issue.Issue.ID) != 0 {
		t.Fatal("outbox events or the ledger copied shared-context text")
	}
	activity, err := f.svc.ListActivity(f.ctx, f.issue.Issue.ID, CollaborationActivityQuery{Limit: 100})
	if err != nil || len(activity.Items) == 0 {
		t.Fatalf("activity read: %+v %v", activity, err)
	}
	for _, entry := range activity.Items {
		if entry.EventSeq == *created.EventSeq || entry.EventSeq == *second.EventSeq {
			t.Fatalf("shared-context write surfaced as an activity entry: %+v", entry)
		}
	}
	page, err := f.svc.ListMemoryVersions(f.ctx, f.issue.Issue.ID, CollaborationMemoryVersionsQuery{Limit: 1})
	if err != nil || page.CurrentVersion != 2 || len(page.Items) != 1 || page.Items[0].Version != 2 || page.NextCursor == nil || *page.NextCursor != 2 || !page.CanPurge {
		t.Fatalf("history first page: %+v %v", page, err)
	}
	next, err := f.svc.ListMemoryVersions(f.ctx, f.issue.Issue.ID, CollaborationMemoryVersionsQuery{Limit: 1, BeforeVersion: pgtype.Int8{Int64: 2, Valid: true}})
	if err != nil || len(next.Items) != 1 || next.Items[0].Version != 1 || next.NextCursor != nil || next.Items[0].Author.Member.UserID != util.MustParseUUID(f.creator) {
		t.Fatalf("history second page: %+v %v", next, err)
	}
}

func TestCollaborationMemoryContentLimitsAreSealedAndUseTheDeploymentLimit(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	f.svc.MemoryMaxBytes = CollaborationMemoryMinMaxBytes
	invalid, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Content: "before\x00after"})
	if !collaborationErrorCode(err, "invalid_memory_content") || invalid.Status != http.StatusBadRequest {
		t.Fatalf("NUL content must be refused, never sanitized: %+v %v", invalid, err)
	}
	// UTF-8 bytes, not characters: 32 KiB of a three-byte character is far over.
	oversized := strings.Repeat("界", CollaborationMemoryMinMaxBytes/3+1)
	large, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Content: oversized})
	if !collaborationErrorCode(err, "memory_too_large") || large.Status != http.StatusRequestEntityTooLarge || large.Bytes == nil ||
		*large.Bytes != len(oversized) || large.MaxBytes != CollaborationMemoryMinMaxBytes || large.CurrentVersion != 0 {
		t.Fatalf("oversized write must report its bytes and limit: %+v %v", large, err)
	}
	exact := f.memoryUpdate(t, f.creator, 0, strings.Repeat("a", CollaborationMemoryMinMaxBytes))
	if *exact.Version != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatalf("exactly-at-limit write: %+v", exact)
	}
	// Raise the limit, write a larger document, then lower it again: the
	// current version stays readable and reports bytes above max_bytes, but
	// the next write must satisfy the lowered limit.
	f.svc.MemoryMaxBytes = CollaborationMemoryHardMaxBytes
	bigger := f.memoryUpdate(t, f.creator, 1, strings.Repeat("b", CollaborationMemoryMinMaxBytes+1))
	f.svc.MemoryMaxBytes = CollaborationMemoryMinMaxBytes
	view, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || *bigger.Version != 2 || view.Version != 2 || view.Bytes != CollaborationMemoryMinMaxBytes+1 || view.MaxBytes != CollaborationMemoryMinMaxBytes {
		t.Fatalf("an over-limit current version must remain readable after the limit is lowered: %+v %v", view, err)
	}
	refused, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(),
		ExpectedVersion: 2, Content: view.Content})
	if !collaborationErrorCode(err, "memory_too_large") || refused.CurrentVersion != 2 || refused.Version != nil {
		t.Fatalf("the next write must satisfy the lowered limit and report the real current version: %+v %v", refused, err)
	}
	nul, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 2, Content: "\x00"})
	if !collaborationErrorCode(err, "invalid_memory_content") || nul.CurrentVersion != 2 {
		t.Fatalf("content refusal lost the current version: %+v %v", nul, err)
	}
}

func TestCollaborationMemoryRestoreAndPurgeKeepHistory(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	leaked := "PRIVATE_LEAKED_TOKEN in the first version"
	f.memoryUpdate(t, f.creator, 0, leaked)
	f.memoryUpdate(t, f.handler, 1, "second version")
	restored, err := f.svc.RestoreMemoryVersion(collaborationContext(f.handler), CollaborationMemoryRestoreParams{IssueID: f.issue.Issue.ID,
		RequestID: uuid.NewString(), Version: 1, ExpectedVersion: 2})
	if err != nil || restored.Code != "memory_restored" || *restored.Version != 3 || restored.CurrentVersion != 3 {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	current, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || current.Content != leaked || current.RestoredFromVersion == nil || *current.RestoredFromVersion != 1 ||
		current.Author.Member.UserID != util.MustParseUUID(f.handler) {
		t.Fatalf("restore must append the source content as a new signed version: %+v %v", current, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 3 {
		t.Fatal("restore removed intermediate versions")
	}
	missing, err := f.svc.RestoreMemoryVersion(f.ctx, CollaborationMemoryRestoreParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 9, ExpectedVersion: 3})
	if !collaborationErrorCode(err, "memory_version_not_found") || missing.Status != http.StatusNotFound {
		t.Fatalf("restoring a missing version must not look like issue denial: %+v %v", missing, err)
	}
	if _, err = f.svc.GetMemoryVersion(f.ctx, f.issue.Issue.ID, 9); !collaborationErrorCode(err, "memory_version_not_found") {
		t.Fatalf("reading a missing version: %v", err)
	}
	purge := func(ctx context.Context, version int64, key string) (CollaborationMemoryReceipt, error) {
		return f.svc.PurgeMemoryVersion(ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: key, Version: version, Reason: " Contained a credential "})
	}
	if refusal, err := purge(f.ctx, 3, uuid.NewString()); !collaborationErrorCode(err, "memory_version_current") || refusal.Status != http.StatusConflict {
		t.Fatalf("current version purge: %+v %v", refusal, err)
	}
	if _, err = purge(collaborationContext(f.handler), 1, uuid.NewString()); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("a non-initiator purged history: %v", err)
	}
	if _, err = f.svc.PurgeMemoryVersion(f.ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "   "}); !collaborationErrorCode(err, "invalid_reason") {
		t.Fatalf("blank purge reason: %v", err)
	}
	key := uuid.NewString()
	purged, err := purge(f.ctx, 1, key)
	if err != nil || purged.Code != "memory_purged" || *purged.Version != 1 || purged.CurrentVersion != 3 || purged.Bytes == nil || *purged.Bytes != len(leaked) {
		t.Fatalf("initiator purge: %+v %v", purged, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1 AND version=1 AND content='' AND bytes=$3 AND purged_by_user_id=$2
		AND NOT purged_by_admin AND purge_reason='Contained a credential' AND author_user_id=$2`, f.issue.Issue.ID, f.creator, len(leaked)) != 1 {
		t.Fatal("purge did not retain size/authorship or record actor and trimmed reason")
	}
	detail, err := f.svc.GetMemoryVersion(collaborationContext(f.handler), f.issue.Issue.ID, 1)
	if err != nil || detail.Content != nil || !detail.Purged || detail.PurgedAt == nil || detail.PurgedBy == nil ||
		detail.PurgedBy.UserID != util.MustParseUUID(f.creator) || detail.PurgedByAdmin || detail.Bytes != len(leaked) || detail.CurrentVersion != 3 {
		t.Fatalf("purged version detail: %+v %v", detail, err)
	}
	if replay, err := purge(f.ctx, 1, key); err != nil || !sameMemoryReceipt(t, replay, purged) {
		t.Fatalf("purge replay: %+v %v", replay, err)
	}
	if again, err := purge(f.ctx, 1, uuid.NewString()); !collaborationErrorCode(err, "memory_version_purged") || again.Status != http.StatusConflict {
		t.Fatalf("second purge of one version: %+v %v", again, err)
	}
	if blocked, err := f.svc.RestoreMemoryVersion(f.ctx, CollaborationMemoryRestoreParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, ExpectedVersion: 3}); !collaborationErrorCode(err, "memory_version_purged") || blocked.Status != http.StatusConflict {
		t.Fatalf("restoring purged text: %+v %v", blocked, err)
	}
	recovered, err := f.svc.GetMemoryOperation(f.ctx, f.issue.Issue.ID, key)
	if err != nil || !sameMemoryReceipt(t, recovered, purged) {
		t.Fatalf("original-key recovery: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetMemoryOperation(collaborationContext(f.handler), f.issue.Issue.ID, key); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("another actor recovered the initiator's receipt: %v", err)
	}
}

func TestCollaborationMemoryAuthorityMatrix(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	f.memoryUpdate(t, f.creator, 0, "PRIVATE_MEMORY_SENTINEL")
	// Source members read like run records, but cannot edit without participation.
	sourceOnly := f.fx.User(t, "Source-only member", "source-only-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, f.workspace, sourceOnly, "member")
	viewer := collaborationContext(sourceOnly)
	view, err := f.svc.GetMemory(viewer, f.issue.Issue.ID)
	if err != nil || view.Content != "PRIVATE_MEMORY_SENTINEL" || view.CanEdit || view.CanPurge {
		t.Fatalf("source member read: %+v %v", view, err)
	}
	if _, err = f.svc.UpdateMemory(viewer, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "x"}); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("source member edited shared context: %v", err)
	}
	if _, err = f.svc.RestoreMemoryVersion(viewer, CollaborationMemoryRestoreParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, ExpectedVersion: 1}); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("source member restored shared context: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
	agentHeader := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialJWT, AgentActor: true})
	for name, ctx := range map[string]context.Context{"outsider": collaborationContext(f.outsider), "task": machine, "agent_actor": agentHeader} {
		if _, err = f.svc.GetMemory(ctx, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("%s read shared context: %v", name, err)
		}
		if _, err = f.svc.ListMemoryVersions(ctx, f.issue.Issue.ID, CollaborationMemoryVersionsQuery{Limit: 10}); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("%s listed shared context: %v", name, err)
		}
		_, err = f.svc.UpdateMemory(ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "x"})
		if !collaborationErrorCode(err, "not_found") && !collaborationErrorCode(err, "forbidden") {
			t.Fatalf("%s wrote shared context: %v", name, err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND actor_user_id<>$2`, f.issue.Issue.ID, f.creator) != 0 {
		t.Fatal("a refused principal created a version or reserved an operation")
	}
	// A removed participant keeps authorship but loses edit authority.
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(), epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
	if _, err = f.svc.GetMemory(collaborationContext(f.handler), f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("removed participant still reads: %v", err)
	}
}

func TestCollaborationMemoryClosedIssueIsReadOnlyExceptPurge(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	f.memoryUpdate(t, f.creator, 0, "first")
	f.memoryUpdate(t, f.handler, 1, "second")
	// Memory writes never advance the issue revision used by lifecycle changes.
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "complete", "done", 1, 1)); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || view.CanEdit || !view.CanPurge || view.Version != 2 {
		t.Fatalf("closed issue memory projection: %+v %v", view, err)
	}
	closedKey := uuid.NewString()
	closedUpdate := CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: closedKey, ExpectedVersion: 2, Content: "late"}
	if refusal, err := f.svc.UpdateMemory(collaborationContext(f.handler), closedUpdate); !collaborationErrorCode(err, "issue_closed") || refusal.CurrentVersion != 2 || refusal.Version != nil {
		t.Fatalf("closed update: %+v %v", refusal, err)
	}
	if _, err = f.svc.RestoreMemoryVersion(f.ctx, CollaborationMemoryRestoreParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, ExpectedVersion: 2}); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("closed restore: %v", err)
	}
	if purged, err := f.svc.PurgeMemoryVersion(f.ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "credential"}); err != nil || purged.Code != "memory_purged" {
		t.Fatalf("credential disposal must survive closure: %+v %v", purged, err)
	}
	if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", 2, 2)); err != nil {
		t.Fatal(err)
	}
	if reopened := f.memoryUpdate(t, f.handler, 2, "after reopen"); *reopened.Version != 3 {
		t.Fatalf("reopened issue memory write: %+v", reopened)
	}
	// The original refusal key never turns into a later success.
	if replay, err := f.svc.UpdateMemory(collaborationContext(f.handler), closedUpdate); !collaborationErrorCode(err, "issue_closed") || replay.RequestID != closedKey {
		t.Fatalf("closed refusal replay after reopen: %+v %v", replay, err)
	}
}

func TestCollaborationMemoryConcurrentWritersFromOneBaseCommitOnce(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	f.memoryUpdate(t, f.creator, 0, "base")
	start := make(chan struct{})
	type outcome struct {
		receipt CollaborationMemoryReceipt
		err     error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for _, actor := range []string{f.creator, f.handler} {
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			<-start
			r, err := f.svc.UpdateMemory(collaborationContext(actor), CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID,
				RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "from " + actor})
			results <- outcome{r, err}
		}(actor)
	}
	close(start)
	wg.Wait()
	close(results)
	won, stale := 0, 0
	for result := range results {
		switch {
		case result.err == nil && *result.receipt.Version == 2:
			won++
		case collaborationErrorCode(result.err, "stale_memory_version") && result.receipt.CurrentVersion == 2:
			stale++
		default:
			t.Fatalf("unexpected concurrent outcome: %+v %v", result.receipt, result.err)
		}
	}
	if won != 1 || stale != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 2 {
		t.Fatalf("two writers from one base: won=%d stale=%d", won, stale)
	}
	// The unique (issue, version) index is the final arbiter even if a writer
	// bypasses the issue locks; its lost insert is the same stale refusal.
	f.fx.Exec(t, `INSERT INTO collaboration_memory_version (issue_id, version, content, bytes, author_kind, author_user_id) VALUES ($1, 3, 'x', 1, 'member', $2)`, f.issue.Issue.ID, f.creator)
	if _, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 2, Content: "late"}); !collaborationErrorCode(err, "stale_memory_version") {
		t.Fatalf("stale write after a direct version: %v", err)
	}
}

func TestCollaborationMemoryRealtimePublishesOnlyIssueScope(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	publisher := &collaborationPublisher{}
	worker := &CollaborationRealtime{Queries: f.svc.Queries, TxStarter: f.pool, Publisher: publisher}
	if _, err := worker.DeliverPending(f.ctx, 1000); err != nil {
		t.Fatal(err)
	}
	publisher.frames = nil
	receipt := f.memoryUpdate(t, f.creator, 0, "PRIVATE_MEMORY_SENTINEL")
	if _, err := worker.DeliverPending(f.ctx, 1000); err != nil {
		t.Fatal(err)
	}
	issueFrames := 0
	for _, frame := range publisher.frames {
		if strings.Contains(string(frame.data), "PRIVATE_") {
			t.Fatalf("realtime frame leaked shared context: %s", frame.data)
		}
		// Other pending fixtures may share the outbox; only this issue's
		// participants and source workspace must receive no workbench tick.
		if frame.scope == realtime.ScopeWorkspace && frame.id == util.UUIDToString(f.issue.Issue.WorkspaceID) ||
			frame.scope == realtime.ScopeAccount && (frame.id == f.creator || frame.id == f.handler) {
			t.Fatalf("shared-context change ticked a workbench: %+v", frame)
		}
		if frame.scope == realtime.ScopeIssue && frame.id == util.UUIDToString(f.issue.Issue.ID) {
			var out struct {
				Type    string                      `json:"type"`
				Payload collaborationRealtimeChange `json:"payload"`
			}
			if err := json.Unmarshal(frame.data, &out); err != nil || out.Type != "collaboration:changed" || out.Payload.EventSequence != *receipt.EventSeq || out.Payload.Revision != 1 {
				t.Fatalf("issue invalidation: %s %v", frame.data, err)
			}
			issueFrames++
		}
	}
	if issueFrames != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND outbox_state='pending'`, f.issue.Issue.ID) != 0 {
		t.Fatalf("expected one issue invalidation and a delivered outbox, got %d", issueFrames)
	}
}

func TestAdminCollaborationMemoryPurgeRequiresDisabledCreatorAndAuditsRealActor(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	for version, content := range []string{"PRIVATE_LEAKED_TOKEN", "clean summary"} {
		f.fx.Insert(t, "collaboration_memory_version", testutil.Cols{"issue_id": f.issue.Issue.ID, "version": version + 1, "content": content,
			"bytes": len(content), "author_kind": "member", "author_user_id": f.creator})
	}
	p := AdminCollaborationMemoryPurgeParams{OrganizationID: f.org, IssueID: f.issue.Issue.ID, IdempotencyKey: util.MustParseUUID(uuid.NewString()),
		Version: 1, Reason: "Reported credential in shared context", RequestID: uuid.NewString()}
	if _, err := f.actions.PurgeMemoryVersion(f.adminCtx, p); !adminCollaborationMemoryErrorCode(err, "creator_not_disabled") ||
		f.fx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, p.IssueID) != 0 {
		t.Fatalf("active creator permitted administration: %v", err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	current := p
	current.Version, current.IdempotencyKey = 2, util.MustParseUUID(uuid.NewString())
	if _, err := f.actions.PurgeMemoryVersion(f.adminCtx, current); !adminCollaborationMemoryErrorCode(err, "memory_version_current") ||
		f.fx.Count(t, `SELECT count(*) FROM admin_operation WHERE target_id=$1`, p.IssueID) != 0 {
		t.Fatalf("current version purge left an administration record: %v", err)
	}
	result, err := f.actions.PurgeMemoryVersion(f.adminCtx, p)
	if err != nil || result.MemoryResult == nil || result.MemoryResult.Code != "memory_purged" || result.MemoryResult.Version != 1 ||
		result.MemoryResult.CurrentVersion != 2 || result.MemoryResult.Bytes != len("PRIVATE_LEAKED_TOKEN") ||
		result.MemoryResult.CreatorUserID != util.MustParseUUID(f.creator) || result.Operation.ActorID != f.admin {
		t.Fatalf("administrator purge: %+v %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1 AND version=1 AND content='' AND purged_by_admin AND purged_by_user_id=$2 AND purge_reason=$3`, p.IssueID, f.admin, p.Reason) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND actor_user_id=$2 AND reason=$3 AND before_state->>'creator_user_id'=$4 AND after_state->>'purged'='true'`, result.Operation.ID, f.admin, p.Reason, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND actor_user_id=$2 AND kind='admin_purge_memory' AND completed_at IS NOT NULL`, p.IssueID, f.admin) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, p.IssueID, f.admin) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM admin_audit_event WHERE operation_id=$1 AND (before_state::text LIKE '%PRIVATE_%' OR after_state::text LIKE '%PRIVATE_%')`, result.Operation.ID) != 0 {
		t.Fatal("administrator purge lost its real actor, original creator, ledgers or exposed text")
	}
	replay, err := f.actions.PurgeMemoryVersion(f.adminCtx, p)
	if err != nil || !replay.Replayed || replay.MemoryResult == nil || *replay.MemoryResult != *result.MemoryResult {
		t.Fatalf("administrator purge replay: %+v %v", replay, err)
	}
	recovered, err := f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
	if err != nil || recovered.MemoryResult == nil || *recovered.MemoryResult != *result.MemoryResult {
		t.Fatalf("administrator original-key recovery: %+v %v", recovered, err)
	}
	changed := p
	changed.Reason = "Another reason"
	if _, err = f.actions.PurgeMemoryVersion(f.adminCtx, changed); !adminCollaborationMemoryErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed administrator payload under one key: %v", err)
	}
	page, err := f.actions.ListMemoryVersions(f.adminCtx, f.org, p.IssueID, CollaborationMemoryVersionsQuery{Limit: 10})
	if err != nil || page.Scope != f.org || page.CurrentVersion != 2 || len(page.Items) != 2 || !page.Items[1].Purged || !page.Items[1].PurgedByAdmin || page.Items[1].PurgedBy != nil {
		t.Fatalf("administrator metadata page: %+v %v", page, err)
	}
	if encoded, _ := json.Marshal(page); strings.Contains(string(encoded), "clean summary") || strings.Contains(string(encoded), `"content"`) {
		t.Fatalf("administrator history exposed shared-context text: %s", encoded)
	}
	// Readers see an administrator purge without the administrator identity.
	participant := adminCollaborationContext(util.MustParseUUID(f.handler), 1)
	versions, err := f.svc.ListMemoryVersions(participant, p.IssueID, CollaborationMemoryVersionsQuery{Limit: 10})
	if err != nil || len(versions.Items) != 2 || !versions.Items[1].PurgedByAdmin || versions.Items[1].PurgedBy != nil {
		t.Fatalf("participant view of an administrator purge: %+v %v", versions, err)
	}
}

func adminCollaborationMemoryErrorCode(err error, code string) bool {
	var platform *PlatformAdminError
	return errors.As(err, &platform) && platform.Code == code
}

func sameMemoryReceipt(t *testing.T, a, b CollaborationMemoryReceipt) bool {
	t.Helper()
	first, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(first) == string(second)
}

func claimSupportedMemoryRun(t *testing.T, f *managedCollaborationFixture, caps protocol.CollaborationRuntimeCapabilities) (pgtype.UUID, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); err != nil {
		t.Fatal(err)
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("memory runtime candidate: %+v %v", listed, err)
	}
	runID := util.MustParseUUID(listed.RunIDs[0])
	request := hostClaimRequest(t, f, caps)
	request.MemoryVersion = protocol.CollaborationMemoryVersion
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil {
		t.Fatalf("supported memory claim: %v", err)
	}
	return runID, claim
}

func permitMemoryRun(t *testing.T, f *managedCollaborationFixture, runID pgtype.UUID, claim protocol.CollaborationRunClaimResponse) string {
	t.Helper()
	request := hostPermitRequest(claim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, request)
	if err != nil {
		t.Fatalf("memory runtime permit: %v", err)
	}
	return permit.BusinessToken
}

func TestCollaborationRuntimeMemoryFreezesBaselineAndReanchorsAfterPurge(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	f.memoryUpdate(t, f.creator, 0, "initial decision")
	submitHostCommand(t, f, f.creator, "continue from shared context")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)

	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !run.MemoryMode.Valid || run.MemoryMode.String != "supported" || run.MemoryBaseVersion.Valid {
		t.Fatalf("claim did not persist stable memory gate: %+v %v", run, err)
	}
	base, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || base.Version != "1" || base.Content != "initial decision" {
		t.Fatalf("initial runtime memory base: %+v %v", base, err)
	}
	f.memoryUpdate(t, f.handler, 1, "newer human decision")
	frozen, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || frozen.Version != "1" || frozen.Content != "initial decision" {
		t.Fatalf("runtime base changed after first read: %+v %v", frozen, err)
	}
	if run, err = f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID); err != nil || !run.MemoryBaseVersion.Valid || run.MemoryBaseVersion.Int64 != 1 {
		t.Fatalf("frozen base was not persisted: %+v %v", run, err)
	}

	if _, err = f.svc.PurgeMemoryVersion(f.ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "remove stale credential"}); err != nil {
		t.Fatalf("purge historical base: %v", err)
	}
	reanchored, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || reanchored.Version != "2" || reanchored.Content != "newer human decision" {
		t.Fatalf("purged base was not re-anchored: %+v %v", reanchored, err)
	}
	if run, err = f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID); err != nil || !run.MemoryBaseVersion.Valid || run.MemoryBaseVersion.Int64 != 2 {
		t.Fatalf("re-anchored base was not persisted: %+v %v", run, err)
	}
	requestID := util.MustParseUUID(uuid.NewString())
	receipt, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, CollaborationRuntimeMemoryParams{RequestID: requestID, BaseVersion: 2, Content: "agent follow-up"})
	if err != nil || status != http.StatusOK || receipt.Code != "memory_updated" || receipt.Version == nil || *receipt.Version != "3" || receipt.CurrentVersion != "3" {
		t.Fatalf("runtime write after re-anchor: %+v status=%d err=%v", receipt, status, err)
	}
}

func TestCollaborationRuntimeMemoryUnsupportedClaimRefusesReadAndWrite(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "legacy runtime")
	f.resumeManaged(t)
	runID, request, claim := f.claimManaged(t)
	if request.MemoryVersion != 0 || claim.Identity.RunID != util.UUIDToString(runID) {
		t.Fatalf("legacy claim unexpectedly advertised memory: request=%+v claim=%+v", request, claim)
	}
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !run.MemoryMode.Valid || run.MemoryMode.String != "unsupported" {
		t.Fatalf("unsupported claim mode was not persisted: %+v %v", run, err)
	}
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatalf("legacy runtime permit: %v", err)
	}
	if _, err = f.svc.RuntimeMemoryBase(f.daemonCtx, runID, permit.BusinessToken); !collaborationErrorCode(err, "memory_unsupported") {
		t.Fatalf("unsupported runtime read was allowed: %v", err)
	}
	receipt, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, permit.BusinessToken, CollaborationRuntimeMemoryParams{RequestID: util.MustParseUUID(uuid.NewString()), BaseVersion: 0, Content: "must not write"})
	if !collaborationErrorCode(err, "memory_unsupported") || status != 0 || receipt.Code != "" {
		t.Fatalf("unsupported runtime write was not a stable refusal: %+v status=%d err=%v", receipt, status, err)
	}
	if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID); count != 0 {
		t.Fatalf("unsupported runtime created %d memory versions", count)
	}
}

func TestCollaborationRuntimeMemoryRedactsBeforeLimitAndStorage(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "redact runtime context")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)
	secret := "mcb_" + strings.Repeat("a", 64)
	raw := strings.Repeat("a", CollaborationMemoryMinMaxBytes-64) + " " + secret
	redacted := redact.Text(raw)
	if len(raw) <= CollaborationMemoryMinMaxBytes || len(redacted) > CollaborationMemoryMinMaxBytes || strings.Contains(redacted, secret) {
		t.Fatalf("fixture did not cross the configured limit after redaction: raw=%d redacted=%d", len(raw), len(redacted))
	}
	f.svc.MemoryMaxBytes = CollaborationMemoryMinMaxBytes
	receipt, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, CollaborationRuntimeMemoryParams{RequestID: util.MustParseUUID(uuid.NewString()), BaseVersion: 0, Content: raw})
	if err != nil || status != http.StatusOK || receipt.Code != "memory_updated" || receipt.Bytes == nil || *receipt.Bytes != len(redacted) || receipt.MaxBytes != CollaborationMemoryMinMaxBytes {
		t.Fatalf("redacted content was measured after the limit: %+v status=%d err=%v", receipt, status, err)
	}
	row, err := f.svc.Queries.GetCollaborationMemoryCurrent(f.ctx, f.issue.Issue.ID)
	if err != nil || row.Content != redacted || int(row.Bytes) != len(redacted) || strings.Contains(row.Content, secret) {
		t.Fatalf("stored runtime content was not redacted: %+v %v", row, err)
	}
}

func TestCollaborationRuntimeMemoryPurgedWriteRetainsExactReplayIdentity(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "idempotent purged runtime context")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)
	params := CollaborationRuntimeMemoryParams{RequestID: util.MustParseUUID(uuid.NewString()), BaseVersion: 0, Content: "first text"}
	first, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK {
		t.Fatalf("first runtime write: %+v status=%d err=%v", first, status, err)
	}
	f.memoryUpdate(t, f.handler, 1, "clean current text")
	if _, err = f.svc.PurgeMemoryVersion(f.ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Version: 1, Reason: "reported historical text"}); err != nil {
		t.Fatal(err)
	}
	replay, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK || !sameProtocolMemoryReceipt(replay, first) || replay.CurrentVersion != "2" {
		t.Fatalf("original purged write replay: %+v status=%d err=%v", replay, status, err)
	}
	changed := params
	changed.Content = "other text" // Equal byte length must not stand in for payload equality.
	if len(changed.Content) != len(params.Content) {
		t.Fatal("fixture must submit different text with identical length")
	}
	conflict, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, changed)
	if err != nil || status != http.StatusConflict || conflict.Code != "memory_write_conflict" || conflict.Version != nil {
		t.Fatalf("changed same-length purged replay was accepted: %+v status=%d err=%v", conflict, status, err)
	}
	if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID); count != 2 {
		t.Fatalf("purged replay changed version count: %d", count)
	}
}

func TestCollaborationRuntimeMemoryEmptyBaseStaysFrozenWhileLatestReadAdvances(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "begin before any shared context")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	// A later duplicate claim never changes the mode recorded by the first one.
	request := hostClaimRequest(t, f, caps)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request); err != nil {
		t.Fatal(err)
	}
	token := permitMemoryRun(t, f, runID, claim)
	base, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || base.Version != "0" || base.Content != "" || base.Bytes != 0 {
		t.Fatalf("empty initial base: %+v %v", base, err)
	}
	f.memoryUpdate(t, f.creator, 0, "created while the run was active")
	frozen, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || frozen.Version != "0" || frozen.Content != "" {
		t.Fatalf("empty base was overwritten: %+v %v", frozen, err)
	}
	latest, err := f.svc.RuntimeMemory(f.daemonCtx, runID, token)
	if err != nil || latest.Version != "1" || latest.Content != "created while the run was active" {
		t.Fatalf("latest runtime read remained frozen: %+v %v", latest, err)
	}
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || run.MemoryMode.String != "supported" || !run.MemoryBaseVersion.Valid || run.MemoryBaseVersion.Int64 != 0 {
		t.Fatalf("recorded empty baseline/mode: %+v %v", run, err)
	}
}

func TestCollaborationRuntimeMemoryRedactionDefinesReplayButNotHumanText(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "redacted replay identity")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)
	secrets := []string{"sk-" + strings.Repeat("a", 32), "mcb_" + strings.Repeat("a", 64),
		"mct_" + strings.Repeat("b", 64), "ey" + strings.Repeat("a", 12) + "." + strings.Repeat("b", 16) + "." + strings.Repeat("c", 16)}
	raw := strings.Join(secrets, "\n")
	redacted := redact.Text(raw)
	for _, secret := range secrets {
		if strings.Contains(redacted, secret) {
			t.Fatal("fixture secret did not exercise the existing redaction path")
		}
	}
	params := CollaborationRuntimeMemoryParams{RequestID: util.MustParseUUID(uuid.NewString()), Content: raw}
	first, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK || first.Version == nil || *first.Version != "1" {
		t.Fatalf("first redacted write: %+v status=%d err=%v", first, status, err)
	}
	params.Content = redacted
	replay, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK || !sameProtocolMemoryReceipt(replay, first) {
		t.Fatalf("replay compared raw rather than redacted text: %+v status=%d err=%v", replay, status, err)
	}
	params.BaseVersion = 1
	if conflict, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params); err != nil || status != http.StatusConflict || conflict.Code != "memory_write_conflict" {
		t.Fatalf("changed base retained old replay identity: %+v status=%d err=%v", conflict, status, err)
	}
	row, err := f.svc.Queries.GetCollaborationMemoryCurrent(f.ctx, f.issue.Issue.ID)
	if err != nil || row.Content != redacted || row.AuthorKind != "agent" || row.AgentID != f.grant.AgentID || row.RunID != runID || row.ResourceOwnerUserID != f.runtime.OwnerID {
		t.Fatalf("AI write lost redaction or attribution: %+v %v", row, err)
	}
	f.memoryUpdate(t, f.creator, 1, raw)
	current, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID)
	if err != nil || current.Content != raw || current.Version != 2 || current.Author.Kind != "member" {
		t.Fatalf("human text was redacted: %+v %v", current, err)
	}
}

func TestCollaborationRuntimeMemoryRejectsStoppedCancelledAndRevokedRuns(t *testing.T) {
	for _, action := range []string{"stop", "cancel", "revoke_grant"} {
		t.Run(action, func(t *testing.T) {
			f, caps := newHostCollaborationFixture(t)
			submitHostCommand(t, f, f.creator, "write only while currently admitted")
			runID, claim := claimSupportedMemoryRun(t, f, caps)
			token := permitMemoryRun(t, f, runID, claim)
			params := CollaborationRuntimeMemoryParams{RequestID: util.MustParseUUID(uuid.NewString()), Content: "before authority changed"}
			if _, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params); err != nil || status != http.StatusOK {
				t.Fatalf("admitted write: status=%d err=%v", status, err)
			}
			switch action {
			case "stop":
				stop := f.issueControl(t)
				stop.ExecutionViewVersion = 6
				if _, err := f.svc.StopQueue(f.ctx, stop); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
				cancel := lifecycleParams(t, f.collaborationQueueFixture, "cancel", "cancelled", 1, 1)
				cancel.ExecutionViewVersion = 6
				if _, err := f.svc.ChangeLifecycle(f.ctx, cancel); err != nil {
					t.Fatal(err)
				}
			case "revoke_grant":
				if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: hostQueueRequest(f), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
					t.Fatal(err)
				}
			}
			// An original-key replay is still business access and must not bypass
			// the current capability gate after stop or revocation.
			for _, request := range []CollaborationRuntimeMemoryParams{params, {RequestID: util.MustParseUUID(uuid.NewString()), BaseVersion: 1, Content: "late write"}} {
				if receipt, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, request); err == nil || status != 0 || receipt.Code != "" {
					t.Fatalf("late memory write was accepted: %+v status=%d err=%v", receipt, status, err)
				}
			}
			if _, err := f.svc.RuntimeMemory(f.daemonCtx, runID, token); err == nil {
				t.Fatal("revoked runtime still read shared context")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
				t.Fatal("late request created a version")
			}
		})
	}
}

func TestCollaborationRuntimeMemoryUpgradeDoesNotChangeConsentOrConfiguration(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	f.memoryUpdate(t, f.creator, 0, "handoff decision")
	submitHostCommand(t, f, f.creator, "old daemon has no shared-context support")
	oldID, oldClaim := claimHostRun(t, f, caps)
	oldRun, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, oldID)
	if err != nil || oldRun.MemoryMode.String != "unsupported" {
		t.Fatalf("old host claim did not record unsupported: %+v %v", oldRun, err)
	}
	launch := hostPermitRequest(oldClaim, 1, "provider")
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, oldID, launch)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RecordRuntimeOutcome(context.Background(), oldID, permit.BusinessToken, protocol.CollaborationRunOutcome{ProtocolVersion: 3, Containment: launch.Containment,
		Identity: oldClaim.Identity, AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, EventID: uuid.NewString(), Outcome: "success", Body: "old host run finished"}); err != nil {
		t.Fatal(err)
	}
	end := hostLaunchEnded(launch, true)
	if _, err := f.svc.AcceptRuntimeLaunchTermination(context.Background(), oldID, oldClaim.TerminationToken, end); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), oldID, oldClaim.TerminationToken, hostFinalProof(t, oldClaim, end)); err != nil {
		t.Fatal(err)
	}
	before := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	submitHostCommand(t, f, f.creator, "upgraded daemon declares memory only in the next claim")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)
	base, err := f.svc.RuntimeMemoryBase(f.daemonCtx, runID, token)
	if err != nil || base.Version != "1" || base.Content != "handoff decision" {
		t.Fatalf("upgraded run did not read shared context: %+v %v", base, err)
	}
	runtime, err := f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil || runtime.CollaborationConfigEpoch != f.runtime.CollaborationConfigEpoch || string(runtime.CollaborationCapabilities) != string(f.runtime.CollaborationCapabilities) {
		t.Fatalf("claim support changed registered configuration: %+v %v", runtime, err)
	}
	grant, err := f.svc.Queries.GetCollaborationGrant(f.ctx, f.grant.ID)
	if err != nil || grant != f.grant {
		t.Fatalf("claim support changed owner consent: %+v %v", grant, err)
	}
	after := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if after.HoldReason != before.HoldReason || after.HoldVersion != before.HoldVersion || after.TargetEpoch != before.TargetEpoch || after.ConfigVersion != before.ConfigVersion {
		t.Fatal("claim support changed target authorization or hold state")
	}
	oldRun, err = f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, oldID)
	if err != nil || oldRun.MemoryMode.String != "unsupported" {
		t.Fatalf("upgrade relabeled the old run: %+v %v", oldRun, err)
	}
}

func TestCollaborationRuntimeMemoryReplayAndChangedPayloadConflict(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.creator, "idempotent runtime context")
	runID, claim := claimSupportedMemoryRun(t, f, caps)
	token := permitMemoryRun(t, f, runID, claim)
	requestID := util.MustParseUUID(uuid.NewString())
	params := CollaborationRuntimeMemoryParams{RequestID: requestID, BaseVersion: 0, Content: "agent write"}
	first, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK || first.Code != "memory_updated" || first.Version == nil || *first.Version != "1" {
		t.Fatalf("first runtime write: %+v status=%d err=%v", first, status, err)
	}
	f.memoryUpdate(t, f.handler, 1, "parallel human write")
	replay, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, params)
	if err != nil || status != http.StatusOK || !sameProtocolMemoryReceipt(replay, first) || replay.CurrentVersion != "2" {
		t.Fatalf("same request replay was not stable after another version: %+v status=%d err=%v first=%+v", replay, status, err, first)
	}
	changed := params
	changed.Content = "different payload"
	conflict, status, err := f.svc.UpdateRuntimeMemory(f.daemonCtx, runID, token, changed)
	if err != nil || status != http.StatusConflict || conflict.Code != "memory_write_conflict" || conflict.Version != nil || conflict.CurrentVersion != "2" {
		t.Fatalf("changed replay payload was accepted: %+v status=%d err=%v", conflict, status, err)
	}
	if count := f.fx.Count(t, `SELECT count(*) FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID); count != 2 {
		t.Fatalf("replay conflict created an extra version: %d", count)
	}
}

func sameProtocolMemoryReceipt(a, b protocol.CollaborationRunMemoryReceipt) bool {
	return a.Code == b.Code && a.Error == b.Error && a.IssueID == b.IssueID && a.RunID == b.RunID && a.RequestID == b.RequestID &&
		a.Version != nil && b.Version != nil && *a.Version == *b.Version && a.Bytes != nil && b.Bytes != nil && *a.Bytes == *b.Bytes && a.MaxBytes == b.MaxBytes
}
