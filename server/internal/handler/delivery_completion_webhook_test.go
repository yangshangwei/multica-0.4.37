package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeliveryCompletionWebhookTrustRevisionDatabase(t *testing.T) {
	t.Run("github_reconnect_but_not_display", func(t *testing.T) {
		ctx := context.Background()
		id := time.Now().UnixNano()
		input := db.CreateGitHubInstallationParams{WorkspaceID: parseUUID(testWorkspaceID), InstallationID: id, AccountLogin: "trust-owner", AccountType: "User", ConnectedByID: parseUUID(testUserID)}
		first, err := testHandler.Queries.CreateGitHubInstallation(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		dbfx.Cleanup(t, `DELETE FROM github_installation WHERE id=$1`, first.ID)
		if first.TrustRevision != 1 {
			t.Fatalf("initial GitHub trust=%d", first.TrustRevision)
		}
		display, err := testHandler.Queries.RefreshGitHubInstallationDisplay(ctx, db.RefreshGitHubInstallationDisplayParams{ID: first.ID, WorkspaceID: first.WorkspaceID, AccountLogin: "new-display", AccountType: "User"})
		if err != nil || display.TrustRevision != 1 {
			t.Fatalf("display refresh changed trust: %d/%v", display.TrustRevision, err)
		}
		reconnected, err := testHandler.Queries.CreateGitHubInstallation(ctx, input)
		if err != nil || reconnected.ID != first.ID || reconnected.TrustRevision != 2 {
			t.Fatalf("same-ID reconnect failed to advance trust: %+v/%v", reconnected, err)
		}
		if err = testHandler.Queries.AdvanceGitHubInstallationTrust(ctx, id); err != nil {
			t.Fatal(err)
		}
		current, err := testHandler.Queries.GetGitHubInstallationByID(ctx, first.ID)
		if err != nil || current.TrustRevision != 3 {
			t.Fatalf("permission change did not invalidate prior trust: %d/%v", current.TrustRevision, err)
		}
	})
	t.Run("vcs_reconnect_and_secret_rotation", func(t *testing.T) {
		ctx := context.Background()
		box := withVCSBox(t)
		sealed, err := box.Seal([]byte("trust-secret"))
		if err != nil {
			t.Fatal(err)
		}
		input := db.UpsertVCSConnectionParams{WorkspaceID: parseUUID(testWorkspaceID), Provider: "forgejo", InstanceUrl: "https://" + uuid.NewString() + ".example.invalid", AccountLogin: "trust-owner", AccessTokenEncrypted: base64.StdEncoding.EncodeToString(sealed), WebhookSecretEncrypted: base64.StdEncoding.EncodeToString(sealed), ConnectedByID: parseUUID(testUserID)}
		first, err := testHandler.Queries.UpsertVCSConnection(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		dbfx.Cleanup(t, `DELETE FROM vcs_connection WHERE id=$1`, first.ID)
		if first.TrustRevision != 1 {
			t.Fatalf("initial VCS trust=%d", first.TrustRevision)
		}
		input.Provider = "gitea"
		reconnected, err := testHandler.Queries.UpsertVCSConnection(ctx, input)
		if err != nil || reconnected.ID != first.ID || reconnected.TrustRevision != 2 || reconnected.Provider != "gitea" {
			t.Fatalf("provider reconnect did not advance exact connection trust: %+v/%v", reconnected, err)
		}
		rotated, err := testHandler.Queries.RotateVCSConnectionWebhookSecret(ctx, db.RotateVCSConnectionWebhookSecretParams{ID: first.ID, WorkspaceID: first.WorkspaceID, WebhookSecretEncrypted: base64.StdEncoding.EncodeToString(sealed)})
		if err != nil || rotated.TrustRevision != 3 {
			t.Fatalf("secret rotation did not advance trust: %d/%v", rotated.TrustRevision, err)
		}
	})
}

func TestDeliveryCompletionWebhookPublicTrustRevision(t *testing.T) {
	// Response projection is small and independent from completion admission.
	github := githubInstallationToResponse(db.GithubInstallation{ID: parseUUID(uuid.NewString()), WorkspaceID: parseUUID(testWorkspaceID), InstallationID: 37, TrustRevision: 9})
	vcs := testHandler.vcsConnectionToResponse(db.VcsConnection{ID: parseUUID(uuid.NewString()), WorkspaceID: parseUUID(testWorkspaceID), Provider: "forgejo", TrustRevision: 8, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
	if github.TrustRevision != 9 || vcs.TrustRevision != 8 {
		t.Fatal("integration response omitted the exact revision clients must bind")
	}
}

func TestDeliveryCompletionWebhookSignatureRecheckedInStatusTransaction(t *testing.T) {
	for _, provider := range []string{"github", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			f := newCompletionFixture(t)
			issueID := f.artifactFixture.req.ResourceID
			issue, err := f.h.Queries.GetIssue(t.Context(), parseUUID(issueID))
			if err != nil {
				t.Fatal(err)
			}
			identifier := issueToResponse(issue, f.h.getIssuePrefix(t.Context(), issue.WorkspaceID)).Identifier
			var owner, repo string
			dbfx.QueryRow(t, `SELECT repo_owner,repo_name FROM github_pull_request WHERE id=$1`, f.pr).Scan(&owner, &repo)
			secret := "completion-race-" + uuid.NewString()
			installation := time.Now().UnixNano()
			var integration string
			rotate := func() {}
			if provider == "github" {
				t.Setenv("GITHUB_WEBHOOK_SECRET", secret)
				integration = dbfx.Insert(t, "github_installation", testutil.Cols{"workspace_id": f.ws, "installation_id": installation, "account_login": owner, "account_type": "User"})
				dbfx.Exec(t, `UPDATE github_pull_request SET installation_id=$1 WHERE id=$2`, installation, f.pr)
				rotate = func() { t.Setenv("GITHUB_WEBHOOK_SECRET", "new-secret-"+uuid.NewString()) }
			} else {
				box := withVCSBox(t)
				f.h.VCSSecretBox = box
				f.h.cfg.VCSIntegrationEnabled = true
				sealed, sealErr := box.Seal([]byte(secret))
				if sealErr != nil {
					t.Fatal(sealErr)
				}
				integration = dbfx.Insert(t, "vcs_connection", testutil.Cols{"workspace_id": f.ws, "provider": provider, "instance_url": "https://" + uuid.NewString() + ".example.invalid", "account_login": owner, "access_token_encrypted": base64.StdEncoding.EncodeToString(sealed), "webhook_secret_encrypted": base64.StdEncoding.EncodeToString(sealed)})
				pr := dbfx.Insert(t, "vcs_pull_request", testutil.Cols{"workspace_id": f.ws, "connection_id": integration, "provider": provider, "repo_owner": owner, "repo_name": repo, "pr_number": 1, "title": "Delivery", "state": "open", "html_url": "https://example.invalid/pr/1", "head_sha": f.artifactFixture.req.SourceRevision, "pr_created_at": time.Now(), "pr_updated_at": time.Now()})
				dbfx.InsertNoID(t, "issue_vcs_pull_request", testutil.Cols{"issue_id": issueID, "pull_request_id": pr, "reference_only": false}, "issue_id=$1 AND pull_request_id=$2", issueID, pr)
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1 AND pull_request_id=$2`, issueID, f.pr)
				f.pr = pr
				rotate = func() {
					replacement, e := box.Seal([]byte("new-secret-" + uuid.NewString()))
					if e != nil {
						t.Fatal(e)
					}
					// Deliberately leave the epoch unchanged to prove that the
					// final transaction re-authenticates with current secret bytes,
					// not only with a prior epoch/signature success flag.
					dbfx.Exec(t, `UPDATE vcs_connection SET webhook_secret_encrypted=$1 WHERE id=$2`, base64.StdEncoding.EncodeToString(replacement), integration)
				}
			}
			f.request.Consumer = service.DeliveryCompletionConsumer{Kind: "github", InstallationID: integration, PullRequestID: f.pr, ExpectedTrustRevision: 1}
			if provider != "github" {
				f.request.Consumer = service.DeliveryCompletionConsumer{Kind: "vcs", ConnectionID: integration, PullRequestID: f.pr, ExpectedTrustRevision: 1}
			}
			grant := f.create(t, uuid.NewString(), http.StatusCreated)
			clock := time.Now().UTC().Format(time.RFC3339Nano)
			raw, marshalErr := json.Marshal(map[string]any{
				"action": "closed", "installation": map[string]any{"id": installation},
				"repository": map[string]any{"name": repo, "owner": map[string]any{"login": owner, "username": owner}},
				"pull_request": map[string]any{"number": 1, "title": "Delivery " + identifier, "body": "Closes " + identifier,
					"state": "closed", "merged": true, "draft": false, "html_url": "https://example.invalid/pr/1",
					"created_at": clock, "updated_at": clock, "merged_at": clock, "closed_at": clock,
					"head": map[string]any{"ref": "delivery/" + identifier, "sha": f.artifactFixture.req.SourceRevision},
					"user": map[string]any{"login": "delivery", "username": "delivery"}},
			})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(raw)
			sig := hex.EncodeToString(mac.Sum(nil))
			reached := false
			f.h.TxStarter = passwordChangeBeforeBegin{base: f.h.TxStarter, before: func() { reached = true; rotate() }}
			var response *testutil.Response
			if provider == "github" {
				req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/webhooks/github", raw), "X-GitHub-Event", "pull_request", "X-Hub-Signature-256", "sha256="+sig, "X-GitHub-Delivery", uuid.NewString())
				response = testutil.Call(t, f.h.HandleGitHubWebhook, req)
			} else {
				req := vcsWebhookReq(integration, map[string]string{"X-Gitea-Event": "pull_request", "X-Gitea-Signature": sig, "X-Gitea-Delivery": uuid.NewString()}, raw)
				response = testutil.Call(t, f.h.HandleVCSWebhook, req)
			}
			response.Want(http.StatusAccepted)
			var afterStatus string
			var afterRevision int64
			dbfx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, issueID).Scan(&afterStatus, &afterRevision)
			if !reached || afterStatus != issue.Status || afterRevision != issue.Revision || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatal("initial signature success survived current secret drift inside the actual status transaction")
			}
		})
	}
}
