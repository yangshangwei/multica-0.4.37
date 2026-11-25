package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// These requests cross an actual HTTP listener and NewRouter/Auth. Task rows
// are fixtures; the claimed variants additionally issue credentials through the
// daemon claim/start API without launching a provider.
func TestCommentManagementAuthorityHTTP(t *testing.T) {
	runCommentManagementAuthorityHTTP(t, []string{"legacy", "password"}, false)
}

func TestCommentManagementClaimedLegacyHTTP(t *testing.T) {
	runCommentManagementAuthorityHTTP(t, []string{"legacy"}, true)
}

func TestCommentManagementClaimedPasswordHTTP(t *testing.T) {
	runCommentManagementAuthorityHTTP(t, []string{"password"}, true)
}

func runCommentManagementAuthorityHTTP(t *testing.T, modes []string, claimed bool) {
	t.Helper()
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
			t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
			t.Setenv("MULTICA_CLOUD_URL", "")
			bus := events.New()
			var eventCount atomic.Int64
			bus.SubscribeAll(func(events.Event) { eventCount.Add(1) })
			srv := httptest.NewServer(NewRouter(testPool, realtime.NewHub(), bus, analytics.NoopClient{}, nil))
			t.Cleanup(srv.Close)
			for _, role := range []string{"owner", "admin", "member"} {
				t.Run(role, func(t *testing.T) {
					base := testutil.New(testPool, testWorkspaceID, testUserID)
					ws := base.Workspace(t, "Comment authority", "comment-authority-"+uuid.NewString())
					user := base.User(t, "Comment actor", uuid.NewString()+"@example.test")
					member := base.Member(t, ws, user, role)
					fx := testutil.New(testPool, ws, user)
					version := int64(0)
					if mode == "password" {
						version = 1
						fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": user, "username": "comment-" + uuid.NewString(), "password_hash": "unused-signed-jwt-fixture", "session_version": version, "must_change_password": false}, "user_id=$1", user)
					}
					humanJWT := controlledArtifactJWT(t, user, version, time.Now().Add(time.Hour))
					pat, err := auth.GeneratePATToken()
					if err != nil {
						t.Fatal(err)
					}
					patID := fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": user, "name": "Comment authority", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "auth_version": version, "expires_at": time.Now().Add(time.Hour)})
					runtime := fx.Runtime(t, "Comment actor runtime")
					agent := fx.Agent(t, "Comment actor", runtime)
					other := fx.User(t, "Other author", uuid.NewString()+"@example.test")
					fx.Member(t, ws, other, "member")
					otherAgent := fx.Agent(t, "Other agent", runtime)
					issue := fx.Issue(t, "Comment authority")
					status := "running"
					if claimed {
						status = "queued"
					}
					task := fx.Task(t, agent, testutil.Cols{"status": status, "runtime_id": runtime, "issue_id": issue, "originator_user_id": user, "accountable_user_id": user, "claim_generation": 1})
					mat, err := auth.GenerateAgentTaskToken()
					if err != nil {
						t.Fatal(err)
					}
					insertToken := func() string {
						return fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(mat), "task_id": task, "agent_id": agent, "workspace_id": ws, "user_id": user, "auth_version": version, "expires_at": time.Now().Add(time.Hour)})
					}
					var tokenID string
					if claimed {
						fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, task)
						raw := commentAuthorityHTTP(t, srv, humanJWT, ws, http.MethodPost, "/api/daemon/runtimes/"+runtime+"/tasks/claim", nil, nil, http.StatusOK)
						var result struct {
							Task *struct {
								ID        string `json:"id"`
								AgentID   string `json:"agent_id"`
								IssueID   string `json:"issue_id"`
								AuthToken string `json:"auth_token"`
							} `json:"task"`
						}
						if err := json.Unmarshal(raw, &result); err != nil {
							t.Fatal(err)
						}
						if result.Task == nil || result.Task.ID != task || result.Task.AgentID != agent || result.Task.IssueID != issue || !strings.HasPrefix(result.Task.AuthToken, "mat_") {
							t.Fatal("claim did not issue the exact task credential")
						}
						mat = result.Task.AuthToken
						fx.QueryRow(t, `SELECT id::text FROM task_token WHERE token_hash=$1 AND user_id=$2 AND workspace_id=$3 AND task_id=$4 AND agent_id=$5`, auth.HashToken(mat), user, ws, task, agent).Scan(&tokenID)
						commentAuthorityHTTP(t, srv, humanJWT, ws, http.MethodPost, "/api/daemon/tasks/"+task+"/start", nil, nil, http.StatusOK)
						var started string
						fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, task).Scan(&started)
						if started != "running" {
							t.Fatal("issued task did not start")
						}
					} else {
						tokenID = insertToken()
					}

					daemonToken, err := auth.GenerateDaemonToken()
					if err != nil {
						t.Fatal(err)
					}
					fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(daemonToken), "workspace_id": ws, "daemon_id": uuid.NewString(), "user_id": user, "auth_version": version, "expires_at": time.Now().Add(time.Hour)})

					for _, credential := range []struct {
						name, token string
						machine     bool
					}{{"jwt", humanJWT, false}, {"pat", pat, false}, {"task", mat, true}} {
						for _, author := range []string{"self", "other-human", "other-agent"} {
							for _, operation := range []string{"body", "attachments", "delete", "spoof"} {
								t.Run(credential.name+"/"+author+"/"+operation, func(t *testing.T) {
									authorType, authorID := "member", user
									if author == "other-human" {
										authorID = other
									}
									if author == "other-agent" {
										authorType, authorID = "agent", otherAgent
									}
									if author == "self" && credential.machine {
										authorType, authorID = "agent", agent
									}
									comment := fx.Comment(t, issue, "Original body", testutil.Cols{"author_type": authorType, "author_id": authorID})
									fx.Insert(t, "attachment", testutil.Cols{"workspace_id": ws, "issue_id": issue, "comment_id": comment, "uploader_type": authorType, "uploader_id": authorID, "filename": "retained.txt", "url": "https://example.invalid/retained.txt", "content_type": "text/plain", "size_bytes": 1})
									allowed := author == "self" || (!credential.machine && role != "member")

									if !allowed {
										// A denied edit/delete must not cancel a real queued consumer.
										fx.Task(t, otherAgent, testutil.Cols{"status": "queued", "runtime_id": runtime, "issue_id": issue, "trigger_comment_id": comment})
									}
									before := commentAuthoritySnapshot(t, fx, ws)
									beforeEvents := eventCount.Load()
									method := http.MethodPut
									body := map[string]any{"content": "Changed body", "expected_revision": 1}
									if operation == "attachments" {
										body["content"] = "Original body"
										body["attachment_ids"] = []string{}
									}
									if operation == "delete" {
										method = http.MethodDelete
										body = nil
									}
									headers := map[string]string{}
									if operation == "spoof" {
										headers = map[string]string{"X-Actor-Source": "member", "X-User-ID": other, "X-Agent-ID": otherAgent, "X-Task-ID": uuid.NewString()}
									}
									want := http.StatusForbidden
									if allowed {
										want = http.StatusOK
										if operation == "delete" {
											want = http.StatusNoContent
										}
									}
									commentAuthorityHTTP(t, srv, credential.token, ws, method, "/api/comments/"+comment, body, headers, want)
									if !allowed {
										if after := commentAuthoritySnapshot(t, fx, ws); after != before {
											t.Fatal("denied comment mutation changed persisted state")
										}
										if eventCount.Load() != beforeEvents {
											t.Fatal("denied comment mutation published an event")
										}
										return
									}
									if operation == "delete" {
										if fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1`, comment) != 0 {
											t.Fatal("authorized deletion did not delete comment")
										}
										return
									}
									var gotType, gotID, content string
									var revision int64
									fx.QueryRow(t, `SELECT author_type,author_id::text,content,revision FROM comment WHERE id=$1`, comment).Scan(&gotType, &gotID, &content, &revision)
									if gotType != authorType || gotID != authorID || content != body["content"] || revision != 2 {
										t.Fatalf("authorized edit lost authorship/revision: %s/%s revision=%d content=%q", gotType, gotID, revision, content)
									}
									if operation == "attachments" && fx.Count(t, `SELECT count(*) FROM attachment WHERE comment_id=$1`, comment) != 0 {
										t.Fatal("authorized attachment clear retained binding")
									}
									// A stale human/self edit must keep the first successful edit intact.
									after := commentAuthoritySnapshot(t, fx, ws)
									commentAuthorityHTTP(t, srv, credential.token, ws, http.MethodPut, "/api/comments/"+comment, map[string]any{"content": "Stale", "expected_revision": 1}, nil, http.StatusConflict)
									if commentAuthoritySnapshot(t, fx, ws) != after {
										t.Fatal("revision conflict changed persisted state")
									}
								})
							}
						}
					}

					// Invalid authority must fail before even the legitimate self path.
					own := fx.Comment(t, issue, "Own agent comment", testutil.Cols{"author_type": "agent", "author_id": agent})
					foreignWS := base.Workspace(t, "Foreign comments", "foreign-comments-"+uuid.NewString())
					foreignFX := testutil.New(testPool, foreignWS, other)
					foreignIssue := foreignFX.Issue(t, "Foreign issue")
					foreignComment := foreignFX.Comment(t, foreignIssue, "Foreign comment")
					cases := []struct {
						name           string
						setup, restore func()
						token, comment string
						status         int
					}{
						{"cross-workspace", func() {}, func() {}, mat, foreignComment, 404},
						{"daemon-token-not-general-auth", func() {}, func() {}, daemonToken, own, 401},
						{"revoked-task-token", func() { fx.Exec(t, `DELETE FROM task_token WHERE id=$1`, tokenID) }, func() { tokenID = insertToken() }, mat, own, 401},
						{"expired-task-token", func() { fx.Exec(t, `UPDATE task_token SET expires_at=now()-interval '1 minute' WHERE id=$1`, tokenID) }, func() { fx.Exec(t, `UPDATE task_token SET expires_at=now()+interval '1 hour' WHERE id=$1`, tokenID) }, mat, own, 401},
						{"terminal-task", func() {
							commentAuthorityHTTP(t, srv, humanJWT, ws, http.MethodPost, "/api/tasks/"+task+"/cancel", nil, nil, http.StatusOK)
						}, func() {
							fx.Exec(t, `UPDATE agent_task_queue SET status='running' WHERE id=$1`, task)
							tokenID = insertToken()
						}, mat, own, 401},
						{"restricted-reviewer", func() {
							fx.Exec(t, `UPDATE agent SET template_key='code-reviewer',template_version=2 WHERE id=$1`, agent)
						}, func() { fx.Exec(t, `UPDATE agent SET template_key='',template_version=0 WHERE id=$1`, agent) }, mat, own, 403},
						{"revoked-pat", func() { fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID) }, func() { fx.Exec(t, `UPDATE personal_access_token SET revoked=false WHERE id=$1`, patID) }, pat, own, 401},
						{"removed-membership", func() { fx.Exec(t, `DELETE FROM member WHERE id=$1`, member) }, func() { member = fx.Member(t, ws, user, role) }, mat, own, 404},
					}
					if claimed {
						// Keep the API-issued token through every control; actual cancellation
						// revokes it last. Never restore it with a test-inserted token row.
						ordered := cases[:0]
						var terminal int
						for i, c := range cases {
							if c.name == "terminal-task" {
								terminal = i
							}
						}
						terminalCase := cases[terminal]
						terminalCase.restore = func() {}
						for _, c := range cases {
							if c.name != "terminal-task" && c.name != "revoked-task-token" {
								ordered = append(ordered, c)
							}
						}
						cases = append(ordered, terminalCase)
					}
					for _, c := range cases {
						t.Run(c.name, func(t *testing.T) {
							c.setup()
							defer c.restore()
							before := commentAuthoritySnapshot(t, fx, ws)
							foreignBefore := commentAuthoritySnapshot(t, foreignFX, foreignWS)
							beforeEvents := eventCount.Load()
							for _, method := range []string{http.MethodPut, http.MethodDelete} {
								commentAuthorityHTTP(t, srv, c.token, ws, method, "/api/comments/"+c.comment, map[string]any{"content": "Forbidden"}, nil, c.status)
							}
							if commentAuthoritySnapshot(t, fx, ws) != before || commentAuthoritySnapshot(t, foreignFX, foreignWS) != foreignBefore || eventCount.Load() != beforeEvents {
								t.Fatal("invalid authority changed state or published an event")
							}
						})
					}
				})
			}
		})
	}
}

func commentAuthorityHTTP(t *testing.T, srv *httptest.Server, token, workspace, method, path string, body any, headers map[string]string, want int) []byte {
	t.Helper()
	req := testutil.JSONRequest(method, srv.URL+path, body)
	req.RequestURI = ""
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-ID", workspace)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: want %d, got %d: %s", method, path, want, resp.StatusCode, raw)
	}
	return raw
}

// Compare full ordered rows, including timestamps, revisions, deleted/bound
// attachments, dispatch/cancellation state and inbox deliveries. Credential
// last-used timestamps are deliberately not business mutation state.
func commentAuthoritySnapshot(t *testing.T, fx *testutil.Fixture, workspace string) string {
	t.Helper()
	var result string
	fx.QueryRow(t, `SELECT jsonb_build_object(
 'comments',(SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]') FROM comment c WHERE workspace_id=$1),
 'attachments',(SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]') FROM attachment a WHERE workspace_id=$1),
 'issues',(SELECT coalesce(jsonb_agg(to_jsonb(i) ORDER BY i.id),'[]') FROM issue i WHERE workspace_id=$1),
 'tasks',(SELECT coalesce(jsonb_agg(to_jsonb(q) ORDER BY q.id),'[]') FROM agent_task_queue q JOIN agent a ON a.id=q.agent_id WHERE a.workspace_id=$1),
 'inbox',(SELECT coalesce(jsonb_agg(to_jsonb(n) ORDER BY n.id),'[]') FROM inbox_item n WHERE workspace_id=$1)
 )::text`, workspace).Scan(&result)
	return result
}

// Local verifier exercises the mcn_ authentication branch without a Cloud
// account. It is not evidence about a deployed Fleet verifier.
func TestCommentManagementCloudPrincipalHTTP(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			user := base.User(t, "Cloud owner", uuid.NewString()+"@example.test")
			ws := base.Workspace(t, "Cloud comment authority", "cloud-comment-"+uuid.NewString())
			base.Member(t, ws, user, role)
			fx := testutil.New(testPool, ws, user)
			other := fx.User(t, "Comment author", uuid.NewString()+"@example.test")
			issue := fx.Issue(t, "Cloud comment authority")
			cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/pat/verify" {
					t.Errorf("unexpected verifier path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "owner_id": user})
			}))
			t.Cleanup(cloud.Close)
			t.Setenv("MULTICA_CLOUD_URL", cloud.URL)
			srv := httptest.NewServer(controlledArtifactRouter(t))
			t.Cleanup(srv.Close)
			for _, method := range []string{http.MethodPut, http.MethodDelete} {
				t.Run(method, func(t *testing.T) {
					comment := fx.Comment(t, issue, "Other human", testutil.Cols{"author_id": other})
					before := commentAuthoritySnapshot(t, fx, ws)
					commentAuthorityHTTP(t, srv, "mcn_local-comment-fixture", ws, method, "/api/comments/"+comment, map[string]any{"content": "Unchanged", "attachment_ids": []string{}}, map[string]string{"X-Actor-Source": "member"}, 403)
					if commentAuthoritySnapshot(t, fx, ws) != before {
						t.Fatal("cloud principal mutated another author's comment")
					}
				})
			}
		})
	}
}
