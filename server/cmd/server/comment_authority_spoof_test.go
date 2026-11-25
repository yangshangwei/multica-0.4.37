package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Existing agent/task IDs are attribution hints, not proof that this credential
// may mutate the agent's comments. Include terminal and caller-owned pairs.
func TestCommentManagementValidActorHintsHTTP(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
			t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
			t.Setenv("MULTICA_CLOUD_URL", "")
			kinds := []string{"jwt", "pat"}
			if mode == "legacy" {
				kinds = append(kinds, "cloud")
			}
			for _, kind := range kinds {
				t.Run(kind, func(t *testing.T) {
					for _, role := range []string{"member", "owner", "admin"} {
						t.Run(role, func(t *testing.T) {
							base := testutil.New(testPool, testWorkspaceID, testUserID)
							ws := base.Workspace(t, "Valid actor hints", "comment-hints-"+uuid.NewString())
							user := base.User(t, "Hint sender", uuid.NewString()+"@example.test")
							base.Member(t, ws, user, role)
							fx := testutil.New(testPool, ws, user)
							version := int64(0)
							if mode == "password" {
								version = 1
								fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": user, "username": "hints-" + uuid.NewString(), "password_hash": "unused-signed-jwt-fixture", "session_version": version, "must_change_password": false}, "user_id=$1", user)
							}
							token := controlledArtifactJWT(t, user, version, time.Now().Add(time.Hour))
							if kind == "pat" {
								var err error
								token, err = auth.GeneratePATToken()
								if err != nil {
									t.Fatal(err)
								}
								fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": user, "name": "Hints", "token_hash": auth.HashToken(token), "token_prefix": token[:12], "auth_version": version, "expires_at": time.Now().Add(time.Hour)})
							}
							if kind == "cloud" {
								token = "mcn_local-valid-hints"
								fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
									if r.URL.Path != "/api/v1/pat/verify" {
										t.Error("unexpected verifier path")
										w.WriteHeader(404)
										return
									}
									_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "owner_id": user})
								}))
								t.Cleanup(fleet.Close)
								t.Setenv("MULTICA_CLOUD_URL", fleet.URL)
							}
							victim := fx.User(t, "Unrelated agent owner", uuid.NewString()+"@example.test")
							fx.Member(t, ws, victim, "member")
							victimFX := testutil.New(testPool, ws, victim)
							runtime := victimFX.Runtime(t, "Unrelated runtime")
							agent := victimFX.Agent(t, "Unrelated author", runtime)
							ownRuntime := fx.Runtime(t, "Sender runtime")
							ownAgent := fx.Agent(t, "Sender's agent", ownRuntime)
							issue := fx.Issue(t, "Valid hints boundary")
							consumer := victimFX.Agent(t, "Queued consumer", runtime)
							bus := events.New()
							var mu sync.Mutex
							var published []events.Event
							bus.SubscribeAll(func(e events.Event) { mu.Lock(); defer mu.Unlock(); published = append(published, e) })
							srv := httptest.NewServer(NewRouter(testPool, realtime.NewHub(), bus, analytics.NoopClient{}, nil))
							t.Cleanup(srv.Close)
							eventSnapshot := func() []events.Event { mu.Lock(); defer mu.Unlock(); return append([]events.Event(nil), published...) }
							for _, state := range []string{"running", "completed"} {
								t.Run(state, func(t *testing.T) {
									task := victimFX.Task(t, agent, testutil.Cols{"status": state, "runtime_id": runtime, "issue_id": issue})
									ownTask := fx.Task(t, ownAgent, testutil.Cols{"status": state, "runtime_id": ownRuntime, "issue_id": issue})
									for _, target := range []string{"unrelated-agent", "owned-agent", "human-self", "other-human"} {
										t.Run(target, func(t *testing.T) {
											pairAgent, pairTask, authorType, authorID := agent, task, "agent", agent
											if target == "owned-agent" {
												pairAgent, pairTask, authorID = ownAgent, ownTask, ownAgent
											}
											if target == "human-self" {
												authorType, authorID = "member", user
											}
											if target == "other-human" {
												authorType, authorID = "member", victim
											}
											for _, operation := range []string{"body", "attachments", "delete"} {
												t.Run(operation, func(t *testing.T) {
													comment := fx.Comment(t, issue, "Retained comment", testutil.Cols{"author_type": authorType, "author_id": authorID})
													fx.Insert(t, "attachment", testutil.Cols{"workspace_id": ws, "issue_id": issue, "comment_id": comment, "uploader_type": authorType, "uploader_id": authorID, "filename": "retained.txt", "url": "https://example.invalid/retained.txt", "content_type": "text/plain", "size_bytes": 1})
													allowed := target == "human-self" || (kind != "cloud" && role != "member")
													if !allowed {
														fx.Task(t, consumer, testutil.Cols{"status": "queued", "runtime_id": runtime, "issue_id": issue, "trigger_comment_id": comment})
													}
													before := commentAuthoritySnapshot(t, fx, ws)
													beforeEvents := len(eventSnapshot())
													method := http.MethodPut
													body := map[string]any{"content": "Edited comment", "expected_revision": 1}
													want := http.StatusForbidden
													if operation == "attachments" {
														body["content"] = "Retained comment"
														body["attachment_ids"] = []string{}
													}
													if operation == "delete" {
														method = http.MethodDelete
														body = nil
													}
													if allowed {
														want = http.StatusOK
														if operation == "delete" {
															want = http.StatusNoContent
														}
													}
													// Both IDs exist, match each other, and are in the current workspace.
													headers := map[string]string{"X-Agent-ID": pairAgent, "X-Task-ID": pairTask, "X-Actor-Source": "task_token", "X-User-ID": victim}
													commentAuthorityHTTP(t, srv, token, ws, method, "/api/comments/"+comment, body, headers, want)
													afterEvents := eventSnapshot()
													if !allowed {
														if commentAuthoritySnapshot(t, fx, ws) != before || len(afterEvents) != beforeEvents {
															t.Fatal("valid actor hints changed denied business state or events")
														}
														return
													}
													eventType := "comment:updated"
													if operation == "delete" {
														eventType = "comment:deleted"
													}
													found := false
													for _, e := range afterEvents[beforeEvents:] {
														if e.Type == eventType {
															found = true
															if e.ActorType != "member" || e.ActorID != user {
																t.Fatal("hint pair replaced the authenticated mutation operator")
															}
														}
													}
													if !found {
														t.Fatal("authorized mutation did not publish its comment event")
													}
													if operation != "delete" {
														var gotType, gotID string
														var revision int64
														var sourceTask *string
														fx.QueryRow(t, `SELECT author_type,author_id::text,revision,source_task_id::text FROM comment WHERE id=$1`, comment).Scan(&gotType, &gotID, &revision, &sourceTask)
														if gotType != authorType || gotID != authorID || revision != 2 || sourceTask != nil {
															t.Fatal("member mutation changed authorship or inherited hinted task authority")
														}
													}
												})
											}
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}
