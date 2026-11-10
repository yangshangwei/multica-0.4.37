package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
)

type completionWebhookRouterFixture struct {
	*deliveryPolicyRouterFixture
	provider, integration, owner, repo, identifier, secret string
	lastEventID, lastEventDigest                           string
	installation                                           int64
	policyRevision                                         int64
	box                                                    *secretbox.Box
}

func newCompletionWebhookRouterFixture(t *testing.T, provider string) *completionWebhookRouterFixture {
	t.Helper()
	secret := "completion-webhook-" + uuid.NewString()
	t.Setenv("GITHUB_WEBHOOK_SECRET", secret)
	var box *secretbox.Box
	if provider != "github" {
		key := bytes.Repeat([]byte{0x47}, 32)
		t.Setenv("MULTICA_VCS_INTEGRATION_ENABLED", "true")
		t.Setenv("MULTICA_VCS_SECRET_KEY", base64.StdEncoding.EncodeToString(key))
		var err error
		box, err = secretbox.New(key)
		if err != nil {
			t.Fatal(err)
		}
	}
	f := &completionWebhookRouterFixture{deliveryPolicyRouterFixture: newDeliveryPolicyRouterFixture(t, "legacy", true), provider: provider, secret: secret, box: box}
	// Packaging and the physical archive are genuine; only integration account
	// rows are local fixtures, so this test never connects to a remote provider.
	f.archive(t)
	var issue struct {
		Identifier string `json:"identifier"`
	}
	f.call(t, f.token, "GET", "/api/issues/"+f.request.ResourceID, nil).Want(200).JSON(&issue)
	f.identifier = issue.Identifier
	f.fx.QueryRow(t, `SELECT repo_owner,repo_name FROM github_pull_request WHERE id=$1`, f.pr).Scan(&f.owner, &f.repo)
	if provider == "github" {
		f.installation = time.Now().UnixNano()
		f.integration = f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": f.installation, "account_login": f.owner, "account_type": "User", "connected_by_id": f.user})
		f.fx.Exec(t, `UPDATE github_pull_request SET installation_id=$1 WHERE id=$2`, f.installation, f.pr)
	} else {
		sealed, err := box.Seal([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		f.integration = f.fx.Insert(t, "vcs_connection", testutil.Cols{"workspace_id": testWorkspaceID, "provider": provider, "instance_url": "https://" + uuid.NewString() + ".example.invalid", "account_login": f.owner, "access_token_encrypted": base64.StdEncoding.EncodeToString(sealed), "webhook_secret_encrypted": base64.StdEncoding.EncodeToString(sealed), "connected_by_id": f.user})
		f.fx.Cleanup(t, `DELETE FROM vcs_pull_request WHERE connection_id=$1`, f.integration)
		f.fx.Cleanup(t, `DELETE FROM issue_vcs_pull_request WHERE issue_id=$1`, f.request.ResourceID)
		f.fx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1 AND pull_request_id=$2`, f.request.ResourceID, f.pr)
		f.webhook(t, "opened").Want(http.StatusAccepted)
		f.fx.QueryRow(t, `SELECT id::text FROM vcs_pull_request WHERE connection_id=$1 AND pr_number=1`, f.integration).Scan(&f.pr)
	}
	// The final webhook proof starts with the real policy API: only the actual
	// current administrator can enable enforce, and signing remains a separate
	// ordinary-member action after that role is restored.
	var previousRole string
	f.fx.QueryRow(t, `SELECT role FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user).Scan(&previousRole)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
	t.Cleanup(func() {
		f.fx.Exec(t, `UPDATE member SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, previousRole, testWorkspaceID, f.user)
	})
	var previous, enabled deliveryPolicyRouterResponse
	f.call(t, f.token, "GET", f.policyPath(), nil).Want(http.StatusOK).JSON(&previous)
	f.call(t, f.token, "PUT", f.policyPath(), f.policyBody(previous.Revision, "enforce")).Want(http.StatusOK).JSON(&enabled)
	if enabled.Mode != "enforce" || enabled.Revision != previous.Revision+1 || enabled.ProjectID != f.project || enabled.CreatedBy == nil || *enabled.CreatedBy != f.user {
		t.Fatalf("actual administrator policy activation lost its revision/actor: %+v", enabled)
	}
	f.policyRevision = enabled.Revision
	f.fx.Exec(t, `UPDATE member SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, previousRole, testWorkspaceID, f.user)
	var issueRevision, projectRevision int64
	f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&issueRevision)
	f.fx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&projectRevision)
	f.body["expected_issue_revision"] = issueRevision
	f.body["expected_project_revision"] = projectRevision
	return f
}

func (f *completionWebhookRouterFixture) webhook(t *testing.T, action string) *testutil.Response {
	t.Helper()
	state, merged := "open", false
	if action == "closed" {
		state, merged = "closed", true
	}
	clock := time.Now().UTC().Format(time.RFC3339Nano)
	body := map[string]any{
		"action": action,
		"pull_request": map[string]any{
			"number": 1, "title": "Delivery " + f.identifier, "body": "Closes " + f.identifier,
			"state": state, "merged": merged, "draft": false,
			"created_at": clock, "updated_at": clock, "merged_at": clock, "closed_at": clock,
			"html_url": "https://example.invalid/" + f.owner + "/" + f.repo + "/pull/1",
			"head":     map[string]any{"ref": "delivery/" + f.identifier, "sha": f.request.SourceRevision},
			"user":     map[string]any{"login": "completion", "username": "completion"},
		},
		"repository":   map[string]any{"name": f.repo, "owner": map[string]any{"login": f.owner, "username": f.owner}},
		"installation": map[string]any{"id": f.installation},
	}
	if f.provider == "gitlab" {
		gitlabState, gitlabAction := "opened", "open"
		if action == "closed" {
			gitlabState, gitlabAction = "merged", "merge"
		}
		body = map[string]any{
			"object_kind": "merge_request", "user": map[string]any{"username": "completion"},
			"project": map[string]any{"path_with_namespace": f.owner + "/" + f.repo},
			"object_attributes": map[string]any{
				"iid": 1, "title": "Delivery " + f.identifier, "description": "Closes " + f.identifier,
				"state": gitlabState, "action": gitlabAction, "source_branch": "delivery/" + f.identifier,
				"url":        "https://example.invalid/" + f.owner + "/" + f.repo + "/-/merge_requests/1",
				"created_at": clock, "updated_at": clock, "last_commit": map[string]any{"id": f.request.SourceRevision},
			},
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(f.secret))
	_, _ = mac.Write(raw)
	signature := hex.EncodeToString(mac.Sum(nil))
	digest := sha256.Sum256(raw)
	f.lastEventDigest = hex.EncodeToString(digest[:])
	f.lastEventID = uuid.NewString()
	path := "/api/webhooks/github"
	if f.provider != "github" {
		path = "/api/webhooks/vcs/" + f.integration
	}
	req := testutil.JSONRequest("POST", path, raw)
	if f.provider == "github" {
		testutil.WithHeaders(req, "X-GitHub-Event", "pull_request", "X-GitHub-Delivery", f.lastEventID, "X-Hub-Signature-256", "sha256="+signature)
	} else if f.provider == "gitlab" {
		testutil.WithHeaders(req, "X-Gitlab-Event", "Merge Request Hook", "X-Gitlab-Event-UUID", f.lastEventID, "X-Gitlab-Token", f.secret)
	} else {
		testutil.WithHeaders(req, "X-Gitea-Event", "pull_request", "X-Gitea-Delivery", f.lastEventID, "X-Gitea-Signature", signature)
	}
	return testutil.Call(t, f.router.ServeHTTP, req)
}

func TestDeliveryCompletionWebhookRouterRequiresExplicitAuthorization(t *testing.T) {
	for _, provider := range []string{"github", "forgejo", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			f := newCompletionWebhookRouterFixture(t, provider)
			var before string
			var revisionBefore int64
			f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&before, &revisionBefore)
			f.webhook(t, "closed").Want(http.StatusAccepted)
			var after string
			var revisionAfter int64
			f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&after, &revisionAfter)
			if after != before || revisionAfter != revisionBefore {
				t.Fatalf("signed %s webhook completed enforce issue without explicit authorization: %s/%d -> %s/%d", provider, before, revisionBefore, after, revisionAfter)
			}
		})
	}
}

func (f *completionWebhookRouterFixture) authorization(t *testing.T) completionAuthorizationRouterResponse {
	t.Helper()
	kind, integrationField := "github", "installation_id"
	if f.provider != "github" {
		kind, integrationField = "vcs", "connection_id"
	}
	body := completionAuthorizationRouterBody(t, f.deliveryPolicyRouterFixture, f.policyRevision, map[string]any{
		"kind": kind, integrationField: f.integration, "expected_trust_revision": int64(1), "pull_request_id": f.pr,
	})
	var out completionAuthorizationRouterResponse
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(http.StatusCreated).JSON(&out)
	return out
}

func TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState(t *testing.T) {
	for _, provider := range []string{"github", "forgejo", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			f := newCompletionWebhookRouterFixture(t, provider)
			archive := f.archive(t)
			grant := f.authorization(t)
			f.webhook(t, "closed").Want(http.StatusAccepted)
			var status string
			var revision int64
			f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status, &revision)
			if status != "done" || revision <= grant.IssueRevision {
				t.Fatalf("exact signed webhook did not consume completion: %s/%d", status, revision)
			}
			var actor, integration, operation, event, digest, result string
			var human *string
			f.fx.QueryRow(t, `SELECT actor_kind,actor_user_id::text,integration_id::text,operation_id::text,provider_event_id,provider_event_digest,status_after FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID).
				Scan(&actor, &human, &integration, &operation, &event, &digest, &result)
			wantActor := "github"
			if provider != "github" {
				wantActor = "vcs"
			}
			if actor != wantActor || human != nil || integration != f.integration || operation != grant.OperationID || event != f.lastEventID || digest != f.lastEventDigest || result != "done" {
				t.Fatal("terminal receipt lost actual integration/event identity or impersonated the issuer")
			}
			before := completionAuthorizationRouterRows(t, f.deliveryPolicyRouterFixture)
			f.webhook(t, "closed").Want(http.StatusAccepted)
			var afterRevision int64
			f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&afterRevision)
			if afterRevision != revision || completionAuthorizationRouterRows(t, f.deliveryPolicyRouterFixture) != before || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("redelivery repeated completion or changed retained source evidence")
			}
		})
	}
}

func TestDeliveryCompletionWebhookRouterTrustChangeRefuses(t *testing.T) {
	for _, scenario := range []string{"github_reconnect", "github_config", "vcs_reconnect", "vcs_secret", "vcs_disconnect"} {
		t.Run(scenario, func(t *testing.T) {
			provider := "github"
			if strings.HasPrefix(scenario, "vcs_") {
				provider = "forgejo"
			}
			f := newCompletionWebhookRouterFixture(t, provider)
			grant := f.authorization(t)
			switch scenario {
			case "github_reconnect":
				f.fx.Exec(t, `UPDATE github_installation SET trust_revision=trust_revision+1 WHERE id=$1`, f.integration)
			case "github_config":
				f.secret = "rotated-" + uuid.NewString()
				t.Setenv("GITHUB_WEBHOOK_SECRET", f.secret)
			case "vcs_reconnect":
				f.fx.Exec(t, `UPDATE vcs_connection SET trust_revision=trust_revision+1 WHERE id=$1`, f.integration)
			case "vcs_secret":
				f.secret = "rotated-" + uuid.NewString()
				sealed, err := f.box.Seal([]byte(f.secret))
				if err != nil {
					t.Fatal(err)
				}
				f.fx.Exec(t, `UPDATE vcs_connection SET webhook_secret_encrypted=$1,trust_revision=trust_revision+1 WHERE id=$2`, base64.StdEncoding.EncodeToString(sealed), f.integration)
			case "vcs_disconnect":
				f.fx.Exec(t, `DELETE FROM vcs_connection WHERE id=$1`, f.integration)
			}
			want := http.StatusAccepted
			if scenario == "vcs_disconnect" {
				want = http.StatusNotFound
			}
			f.webhook(t, "closed").Want(want)
			var status string
			var revision int64
			f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status, &revision)
			if status == "done" || revision != grant.IssueRevision || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatal("changed or removed integration consumed an old authorization")
			}
		})
	}
}

func TestDeliveryCompletionWebhookRouterAmbiguousGrantAndOpenSibling(t *testing.T) {
	for _, scenario := range []string{"ambiguous", "open_sibling"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCompletionWebhookRouterFixture(t, "github")
			grant := f.authorization(t)
			if scenario == "ambiguous" {
				f.authorization(t)
			} else {
				other := f.fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": f.installation, "repo_owner": f.owner, "repo_name": f.repo, "pr_number": 2, "title": "Still open sibling", "state": "open", "html_url": "https://example.invalid/pull/2", "head_sha": f.request.SourceRevision, "pr_created_at": time.Now(), "pr_updated_at": time.Now()})
				f.fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": f.request.ResourceID, "pull_request_id": other, "reference_only": false, "close_intent": false}, "issue_id=$1 AND pull_request_id=$2", f.request.ResourceID, other)
			}
			f.webhook(t, "closed").Want(http.StatusAccepted)
			var status string
			var revision int64
			f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status, &revision)
			if status == "done" || revision != grant.IssueRevision || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.issue_id=$1`, f.request.ResourceID) != 0 {
				t.Fatal("ambiguous authorization or still-open aggregate was treated as completion permission")
			}
		})
	}
}

func TestDeliveryCompletionWebhookRouterRejectsWrongProviderBinding(t *testing.T) {
	f := newCompletionWebhookRouterFixture(t, "forgejo")
	f.fx.Exec(t, `UPDATE vcs_pull_request SET provider='gitlab' WHERE id=$1`, f.pr)
	body := completionAuthorizationRouterBody(t, f.deliveryPolicyRouterFixture, f.policyRevision, map[string]any{"kind": "vcs", "connection_id": f.integration, "expected_trust_revision": int64(1), "pull_request_id": f.pr})
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, body).Want(http.StatusConflict)
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
		t.Fatal("a PR attributed to another provider became an exact integration authorization")
	}
}
