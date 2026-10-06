package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestPullRequestWebhookIterationFacts(t *testing.T) {
	for _, provider := range []string{"github", "forgejo", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			identifier := issueToResponse(issue, testHandler.getIssuePrefix(context.Background(), issue.WorkspaceID)).Identifier
			if provider == "github" {
				const installationID int64 = 891034
				secret := "i1-github-secret"
				t.Setenv("GITHUB_WEBHOOK_SECRET", secret)
				dbfx.Insert(t, "github_installation", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": installationID, "account_login": "i1-close", "account_type": "User"})
				dbfx.Cleanup(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, issueID)
				dbfx.Cleanup(t, `DELETE FROM github_pull_request WHERE workspace_id=$1`, testWorkspaceID)
				firePRWebhook(t, secret, installationID, 431, "Fix "+identifier, "Closes "+identifier, "fix/i1", "merged")
				firePRWebhook(t, secret, installationID, 431, "Fix "+identifier, "Closes "+identifier, "fix/i1", "merged")
			} else if provider == "gitlab" {
				ctx := context.Background()
				box := withVCSBox(t)
				connID := seedVCSConnection(t, ctx, box, "gitlab", "https://gitlab.test")
				t.Cleanup(func() { cleanupVCS(ctx, issueID) })
				raw, _ := json.Marshal(map[string]any{"object_kind": "merge_request", "user": map[string]any{"username": "alice"}, "project": map[string]any{"path_with_namespace": "acme/widget"}, "object_attributes": map[string]any{"iid": 433, "title": "Fix " + identifier, "description": "Closes " + identifier, "state": "merged", "action": "merge", "source_branch": "fix/i1", "url": "https://gitlab.test/acme/widget/-/merge_requests/433", "last_commit": map[string]any{"id": "abc"}}})
				for i := 0; i < 2; i++ {
					testutil.Call(t, testHandler.HandleVCSWebhook, vcsWebhookReq(connID, map[string]string{"X-Gitlab-Event": "Merge Request Hook", "X-Gitlab-Token": vcsTestSecret}, raw)).Want(http.StatusAccepted)
				}
			} else {
				ctx := context.Background()
				box := withVCSBox(t)
				connID := seedVCSConnection(t, ctx, box, "forgejo", "https://forgejo.test")
				t.Cleanup(func() { cleanupVCS(ctx, issueID) })
				raw, _ := json.Marshal(map[string]any{"action": "closed", "pull_request": map[string]any{"number": 432, "html_url": "https://forgejo.test/acme/widget/pulls/432", "title": "Fix " + identifier, "body": "Closes " + identifier, "state": "closed", "merged": true, "merged_at": "2026-10-01T00:00:00Z", "closed_at": "2026-10-01T00:00:00Z", "created_at": "2026-09-30T00:00:00Z", "updated_at": "2026-10-01T00:00:00Z", "head": map[string]any{"ref": "fix/i1", "sha": "abc"}, "user": map[string]any{"username": "octo"}}, "repository": map[string]any{"name": "widget", "owner": map[string]any{"username": "acme"}}})
				for i := 0; i < 2; i++ {
					testutil.Call(t, testHandler.HandleVCSWebhook, vcsWebhookReq(connID, map[string]string{"X-Gitea-Event": "pull_request", "X-Gitea-Signature": giteaSig(raw)}, raw)).Want(http.StatusAccepted)
				}
			}
			var status string
			var count, scope int64
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, issueID).Scan(&status)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			var actorJSON []byte
			dbfx.QueryRow(t, `SELECT actor FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&actorJSON)
			var actor map[string]any
			if err = json.Unmarshal(actorJSON, &actor); err != nil {
				t.Fatal(err)
			}
			if actor["type"] != "system" || actor["source"] != "pull_request_close_aggregate" || actor["id"] != nil || actor["user_id"] != nil {
				t.Fatalf("incorrect system actor: %s", actorJSON)
			}

			if status != "done" || count != 1 || scope != 2 {
				t.Fatalf("webhook completion missing atomic facts: status=%s events=%d scope=%d", status, count, scope)
			}
		})
	}
}

func TestPullRequestWebhookIterationRechecksAndRollsBack(t *testing.T) {
	for _, scenario := range []string{"cancelled_before_lock", "open_pr_before_lock", "recorder_failure"} {
		t.Run(scenario, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			identifier := issueToResponse(issue, testHandler.getIssuePrefix(context.Background(), issue.WorkspaceID)).Identifier
			const installationID int64 = 891035
			secret := "i1-recheck-secret"
			t.Setenv("GITHUB_WEBHOOK_SECRET", secret)
			dbfx.Insert(t, "github_installation", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": installationID, "account_login": "i1-recheck", "account_type": "User"})
			dbfx.Cleanup(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, issueID)
			dbfx.Cleanup(t, `DELETE FROM github_pull_request WHERE workspace_id=$1`, testWorkspaceID)
			original := testHandler.TxStarter
			t.Cleanup(func() { testHandler.TxStarter = original })
			if scenario == "recorder_failure" {
				testHandler.TxStarter = iterationEventFailureStarter{inner: original}
				firePRWebhook(t, secret, installationID, 434, "Fix "+identifier, "Closes "+identifier, "fix/i1", "merged")
			} else {
				reached, release := make(chan struct{}, 1), make(chan struct{})
				testHandler.TxStarter = iterationBeginBarrier{inner: original, reached: reached, release: release}
				done := make(chan struct{})
				go func() {
					firePRWebhook(t, secret, installationID, 434, "Fix "+identifier, "Closes "+identifier, "fix/i1", "merged")
					close(done)
				}()
				waitIterationBarrier(t, reached)
				if scenario == "cancelled_before_lock" {
					writer := *testHandler
					writer.TxStarter = original
					testutil.Call(t, writer.UpdateIssue, withURLParam(newRequest(http.MethodPut, "/api/issues/"+issueID, map[string]any{"status": "cancelled"}), "id", issueID)).Want(http.StatusOK)
				} else {
					firePRWebhook(t, secret, installationID, 435, "Work "+identifier, "", "work/i1", "opened")
				}
				close(release)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("webhook failed to resume")
				}
			}
			var status string
			var revision, scope, events int64
			dbfx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, issueID).Scan(&status, &revision)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
			wantStatus, wantRevision, wantScope, wantEvents := "todo", int64(1), int64(1), int64(0)
			if scenario == "cancelled_before_lock" {
				wantStatus, wantRevision, wantScope, wantEvents = "cancelled", 2, 2, 1
			}
			if status != wantStatus || revision != wantRevision || scope != wantScope || events != wantEvents {
				t.Fatalf("webhook stale policy/partial commit: status=%s revision=%d scope=%d events=%d", status, revision, scope, events)
			}
		})
	}
}
