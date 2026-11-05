package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// Exercise the real command boundary: a saved comment must keep its successful
// write result and complete JSON even when admission of a mentioned target fails.
func TestIssueCommentDelegationReceipts(t *testing.T) {
	const issueID = "11111111-1111-4111-8111-111111111111"
	const commentID = "22222222-2222-4222-8222-222222222222"
	const agentID = "33333333-3333-4333-8333-333333333333"
	const squadID = "44444444-4444-4444-8444-444444444444"
	receipt := func(kind, id, status string) string {
		return fmt.Sprintf(`{"target_type":%q,"target_id":%q,"status":%q,"reason_code":%q}`, kind, id, status, status)
	}
	queued := receipt("agent", agentID, "queued")
	blocked := receipt("squad", squadID, "blocked")
	cases := []struct {
		name     string
		outcomes string // Empty means the field is absent, as on older servers.
		warning  string
	}{
		{"older_server", "", ""},
		{"no_mentions", "[]", ""},
		{"queued", "[" + queued + "]", ""},
		{"coalesced", "[" + receipt("squad", squadID, "coalesced") + "]", ""},
		{"deferred", "[" + receipt("agent", agentID, "deferred") + "]", ""},
		{"blocked", "[" + blocked + "]", "0 handled, 1 blocked, 0 unconfirmed"},
		{"partial", "[" + queued + "," + blocked + "]", "1 handled, 1 blocked, 0 unconfirmed"},
		{"unknown_status", "[" + receipt("agent", agentID, "new_status") + "]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"unknown_target_type", "[" + receipt("member", agentID, "queued") + "]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"invalid_target_id", "[" + receipt("agent", "not-a-uuid", "queued") + "]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"missing_target", `[{"status":"queued","reason_code":"queued"}]`, "0 handled, 0 blocked, 1 unconfirmed"},
		{"wrong_field_type", `[{"target_type":"agent","target_id":42,"status":"queued","reason_code":"queued"}]`, "0 handled, 0 blocked, 1 unconfirmed"},
		{"missing_reason", fmt.Sprintf(`[{"target_type":"agent","target_id":%q,"status":"queued"}]`, agentID), "0 handled, 0 blocked, 1 unconfirmed"},
		{"null_entry", "[null]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"scalar_entry", `["untrusted receipt"]`, "0 handled, 0 blocked, 1 unconfirmed"},
		{"object_container", `{"status":"queued"}`, "0 handled, 0 blocked, 1 unconfirmed"},
		{"null_container", "null", "0 handled, 0 blocked, 1 unconfirmed"},
		{"matching_duplicate", "[" + queued + "," + queued + "]", ""},
		{"contradictory_duplicate", "[" + queued + "," + receipt("agent", agentID, "blocked") + "]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"contradictory_duplicate_stays_unconfirmed", "[" + queued + "," + receipt("agent", agentID, "blocked") + "," + queued + "]", "0 handled, 0 blocked, 1 unconfirmed"},
		{"malformed_duplicate_after_handled", fmt.Sprintf(`[%s,{"target_type":"agent","target_id":%q,"status":"queued"}]`, queued, agentID), "0 handled, 0 blocked, 1 unconfirmed"},
		{"malformed_duplicate_before_handled", fmt.Sprintf(`[{"target_type":"agent","target_id":%q,"status":"queued"},%s]`, agentID, queued), "0 handled, 0 blocked, 1 unconfirmed"},
		{"malformed_duplicate_after_blocked", fmt.Sprintf(`[%s,{"target_type":"squad","target_id":%q,"status":null,"reason_code":"blocked"}]`, blocked, squadID), "0 handled, 0 blocked, 1 unconfirmed"},
		{"future_reason", fmt.Sprintf(`[{"target_type":"agent","target_id":%q,"status":"queued","reason_code":"future_reason"}]`, agentID), ""},
		{"untrusted_reason", fmt.Sprintf(`[{"target_type":"agent","target_id":%q,"status":"blocked","reason_code":"UNTRUSTED\u001b[2J"}]`, agentID), "0 handled, 1 blocked, 0 unconfirmed"},
	}
	for _, operation := range []string{"add", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					chdirWithDaemonTaskMarker(t)
					response := `{"id":"` + commentID + `","revision":7,"extra":{"preserve":"future response field"}`
					if tc.outcomes != "" {
						response += `,"trigger_outcomes":` + tc.outcomes
					}
					response += "}"
					var requests atomic.Int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						wantMethod, wantPath := http.MethodPost, "/api/issues/"+issueID+"/comments"
						if operation == "update" {
							wantMethod, wantPath = http.MethodPut, "/api/comments/"+commentID
						}
						if r.Method != wantMethod || r.URL.Path != wantPath {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					}))
					defer srv.Close()
					setCLITestServerEnv(t, srv.URL)
					t.Setenv("MULTICA_TOKEN", "mat_test-token")
					cmd := newIssueCommentAddTestCmd()
					if operation == "update" {
						cmd = newIssueCommentUpdateTestCmd()
						_ = cmd.Flags().Set("expected-revision", "6")
					}
					_ = cmd.Flags().Set("content", "Please inspect the assigned change.")
					stderr := captureStderr(t)
					defer stderr.restore()
					out, err := captureStdout(t, func() error {
						if operation == "update" {
							return runIssueCommentUpdate(cmd, []string{commentID})
						}
						return runIssueCommentAdd(cmd, []string{issueID})
					})
					if err != nil || requests.Load() != 1 {
						t.Fatalf("saved comment result: error=%v requests=%d, want success and one write", err, requests.Load())
					}
					var got, want map[string]any
					if err := json.Unmarshal([]byte(out), &got); err != nil {
						t.Fatalf("stdout is not JSON: %q: %v", out, err)
					}
					if err := json.Unmarshal([]byte(response), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("response fields changed: got %#v want %#v", got, want)
					}
					message := stderr.read()
					if tc.warning == "" {
						if strings.Contains(message, "Warning:") {
							t.Errorf("unexpected warning for handled/legacy response: %s", message)
						}
					} else if !strings.Contains(message, "Warning:") || !strings.Contains(message, tc.warning) || !strings.Contains(message, "Do not repost") {
						t.Errorf("missing delegation warning %q with no-repost guidance: %s", tc.warning, message)
					}
					if strings.Contains(message, "UNTRUSTED") || strings.Contains(message, "\x1b") {
						t.Errorf("warning echoed untrusted reason: %q", message)
					}
				})
			}
		})
	}
}

func TestIssueCommentDelegationReceiptsTableAndUncertainWrite(t *testing.T) {
	const issueID = "11111111-1111-4111-8111-111111111111"
	const commentID = "22222222-2222-4222-8222-222222222222"
	for _, operation := range []string{"add", "update"} {
		for _, failure := range []string{"none", "malformed_json", "connection_lost"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				chdirWithDaemonTaskMarker(t)
				var requests atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if failure == "connection_lost" {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Errorf("hijack response: %v", err)
							return
						}
						_ = connection.Close()
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if failure == "malformed_json" {
						fmt.Fprint(w, `{"id":`)
						return
					}
					fmt.Fprintf(w, `{"id":%q,"trigger_outcomes":[{"target_type":"agent","target_id":%q,"status":"blocked","reason_code":"invocation_not_allowed"}]}`, commentID, issueID)
				}))
				defer srv.Close()
				setCLITestServerEnv(t, srv.URL)
				t.Setenv("MULTICA_TOKEN", "mat_test-token")
				cmd := newIssueCommentAddTestCmd()
				if operation == "update" {
					cmd = newIssueCommentUpdateTestCmd()
					_ = cmd.Flags().Set("expected-revision", "1")
				}
				_ = cmd.Flags().Set("content", "Please review.")
				_ = cmd.Flags().Set("output", "table")
				stderr := captureStderr(t)
				defer stderr.restore()
				out, err := captureStdout(t, func() error {
					if operation == "update" {
						return runIssueCommentUpdate(cmd, []string{commentID})
					}
					return runIssueCommentAdd(cmd, []string{issueID})
				})
				if requests.Load() != 1 || out != "" {
					t.Fatalf("table command wrote output or retried: requests=%d stdout=%q", requests.Load(), out)
				}
				message := stderr.read()
				if failure != "none" {
					if err == nil || message != "" {
						t.Fatalf("uncertain write reported success: error=%v stderr=%q", err, message)
					}
				} else if err != nil || !strings.Contains(message, "Warning:") || !strings.Contains(message, "1 blocked") {
					t.Fatalf("table output hid a blocked delegation: error=%v stderr=%q", err, message)
				}
			})
		}
	}
}
