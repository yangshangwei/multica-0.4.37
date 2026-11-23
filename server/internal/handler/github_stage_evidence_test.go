package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageGitHubLinkRaceDB struct {
	db.DBTX
	beforeLink func()
	called     bool
}

func (d *stageGitHubLinkRaceDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if !d.called && strings.Contains(sql, "INSERT INTO issue_pull_request") {
		d.called = true
		d.beforeLink()
	}
	return d.DBTX.Exec(ctx, sql, args...)
}

// Synthetic signed callbacks use the real webhook handler but no GitHub API.
// A delayed event must change neither the newer head nor its derived links.
func TestStageGitHubOlderEventCannotRestoreOldHeadOrLinks(t *testing.T) {
	t.Setenv("GITHUB_APP_ID", "")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "synthetic-stage-webhook-secret")
	s := stageHTTPFixture(t)
	s.fx.Exec(t, `UPDATE workspace SET issue_prefix='STG' WHERE id=$1`, s.fx.WorkspaceID)
	installation := int64(time.Now().UnixNano())
	s.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "installation_id": installation, "account_login": "synthetic", "account_type": "User"})
	currentIssue := s.fx.Issue(t, "Current code child", testutil.Cols{"parent_issue_id": s.issue})
	oldIssue := s.fx.Issue(t, "Old callback child", testutil.Cols{"parent_issue_id": s.issue})
	current, err := s.h.Queries.GetIssue(t.Context(), parseUUID(currentIssue))
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.h.Queries.GetIssue(t.Context(), parseUUID(oldIssue))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := s.h.Queries.GetWorkspace(t.Context(), parseUUID(s.fx.WorkspaceID))
	if err != nil {
		t.Fatal(err)
	}
	repo, now := uuid.NewString(), time.Now().UTC().Truncate(time.Second)
	s.fx.Cleanup(t, `DELETE FROM github_pull_request WHERE workspace_id=$1`, s.fx.WorkspaceID)
	s.fx.Cleanup(t, `DELETE FROM issue_pull_request WHERE pull_request_id IN (SELECT id FROM github_pull_request WHERE workspace_id=$1)`, s.fx.WorkspaceID)
	fire := func(number int32, head string, updated time.Time) {
		payload := map[string]any{"action": "synchronize", "installation": map[string]any{"id": installation},
			"repository": map[string]any{"owner": map[string]any{"login": "synthetic"}, "name": repo},
			"pull_request": map[string]any{"number": 1, "title": workspace.IssuePrefix + "-" + int64Text(int64(number)) + " synthetic code", "state": "open",
				"html_url": "https://example.invalid/synthetic", "created_at": now.Add(-time.Hour).Format(time.RFC3339), "updated_at": updated.Format(time.RFC3339),
				"head": map[string]any{"sha": head, "ref": "synthetic-branch"}}}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte("synthetic-stage-webhook-secret"))
		_, _ = mac.Write(raw)
		r := testutil.WithHeaders(testutil.JSONRequest(http.MethodPost, "/api/webhooks/github", nil), "X-GitHub-Event", "pull_request", "X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		testutil.Call(t, s.h.HandleGitHubWebhook, r).Want(http.StatusAccepted)
	}
	head := strings.Repeat("b", 40)
	fire(current.Number, head, now)
	if s.fx.Count(t, `SELECT count(*) FROM issue_pull_request WHERE issue_id=$1 AND NOT reference_only`, currentIssue) != 1 {
		t.Fatal("newer synthetic callback did not establish the current link")
	}
	fire(old.Number, strings.Repeat("a", 40), now.Add(-time.Minute))
	if s.fx.Count(t, `SELECT count(*) FROM github_pull_request WHERE workspace_id=$1 AND head_sha=$2`, s.fx.WorkspaceID, head) != 1 ||
		s.fx.Count(t, `SELECT count(*) FROM issue_pull_request WHERE issue_id=$1`, oldIssue) != 0 {
		t.Fatal("older callback restored its head or derived an old issue link")
	}
	// Provider timestamps have finite precision. An equal timestamp with a
	// different head is ambiguous, not proof that the incoming head is newer.
	s.fx.Exec(t, `UPDATE github_pull_request SET snapshot_complete=true WHERE workspace_id=$1`, s.fx.WorkspaceID)
	fire(old.Number, strings.Repeat("a", 40), now)
	if s.fx.Count(t, `SELECT count(*) FROM github_pull_request WHERE workspace_id=$1 AND head_sha=$2 AND NOT snapshot_complete`, s.fx.WorkspaceID, head) != 1 ||
		s.fx.Count(t, `SELECT count(*) FROM issue_pull_request WHERE issue_id=$1`, oldIssue) != 0 {
		t.Fatal("ambiguous equal-timestamp callback changed the head/link or left strict CI usable")
	}
	// The event may have won the upsert and then lose to a newer observation
	// before deriving its link. The link write needs the same source fence.
	newHead := strings.Repeat("d", 40)
	probe := &stageGitHubLinkRaceDB{DBTX: s.fx.Pool, beforeLink: func() {
		s.fx.Exec(t, `UPDATE github_pull_request SET head_sha=$2,pr_updated_at=$3 WHERE workspace_id=$1`, s.fx.WorkspaceID, newHead, now.Add(2*time.Minute))
	}}
	original := s.h.Queries
	s.h.Queries = db.New(probe)
	fire(old.Number, strings.Repeat("c", 40), now.Add(time.Minute))
	s.h.Queries = original
	if !probe.called || s.fx.Count(t, `SELECT count(*) FROM issue_pull_request WHERE issue_id=$1`, oldIssue) != 0 {
		t.Fatal("event that lost after upsert still derived an old issue link")
	}
}
