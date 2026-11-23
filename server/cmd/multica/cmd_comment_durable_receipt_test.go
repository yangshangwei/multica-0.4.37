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

const durableCLIKey = "55555555-5555-4555-8555-555555555555"
const durableCLIIssue = "11111111-1111-4111-8111-111111111111"
const durableCLIComment = "22222222-2222-4222-8222-222222222222"

func TestIssueCommentDurableWrite(t *testing.T) {
	for _, tc := range []struct {
		name, receipt string
		warning       bool
	}{
		{"recorded", fmt.Sprintf(`{"key":%q,"state":"recorded","comment_id":%q,"targets":[]}`, durableCLIKey, durableCLIComment), false},
		{"unknown", fmt.Sprintf(`{"key":%q,"state":"unknown","comment_id":%q,"targets":[]}`, durableCLIKey, durableCLIComment), true},
		{"unknown_table", fmt.Sprintf(`{"key":%q,"state":"unknown","comment_id":%q,"targets":[]}`, durableCLIKey, durableCLIComment), true},
		{"legacy_missing", "", true},
		{"malformed", `"untrusted"`, true},
		{"wrong_key", fmt.Sprintf(`{"key":%q,"state":"recorded","comment_id":%q,"targets":[]}`, durableCLIIssue, durableCLIComment), true},
		{"wrong_comment", fmt.Sprintf(`{"key":%q,"state":"recorded","comment_id":%q,"targets":[]}`, durableCLIKey, durableCLIIssue), true},
		{"future_state", fmt.Sprintf(`{"key":%q,"state":"future","comment_id":%q,"targets":[]}`, durableCLIKey, durableCLIComment), true},
		{"missing_targets", fmt.Sprintf(`{"key":%q,"state":"recorded","comment_id":%q}`, durableCLIKey, durableCLIComment), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chdirWithDaemonTaskMarker(t)
			response := fmt.Sprintf(`{"id":%q,"future":{"retained":true},"trigger_outcomes":[]`, durableCLIComment)
			if tc.receipt != "" {
				response += `,"delegation_receipt":` + tc.receipt
			}
			response += "}"
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/api/issues/"+durableCLIIssue+"/comments" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Idempotency-Key") != durableCLIKey {
					t.Errorf("missing exact key: %q", r.Header.Get("Idempotency-Key"))
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body, map[string]any{"content": "Inspect this change."}) {
					t.Errorf("body changed: %#v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				fmt.Fprint(w, response)
			}))
			defer srv.Close()
			setCLITestServerEnv(t, srv.URL)
			t.Setenv("MULTICA_TOKEN", "mat_test-token")
			cmd := newIssueCommentAddTestCmd()
			cmd.Flags().String("idempotency-key", "", "")
			_ = cmd.Flags().Set("idempotency-key", durableCLIKey)
			_ = cmd.Flags().Set("content", "Inspect this change.")
			if tc.name == "unknown_table" {
				_ = cmd.Flags().Set("output", "table")
			}
			stderr := captureStderr(t)
			defer stderr.restore()
			out, err := captureStdout(t, func() error { return runIssueCommentAdd(cmd, []string{durableCLIIssue}) })
			if err != nil || calls.Load() != 1 {
				t.Fatalf("error=%v calls=%d", err, calls.Load())
			}
			if tc.name == "unknown_table" {
				if out != "" {
					t.Fatalf("table write produced stdout: %s", out)
				}
			} else {
				var got, want any
				if err = json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal([]byte(response), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("changed response: %s", out)
				}
			}
			warning := stderr.read()
			if strings.Contains(warning, "durable delegation") != tc.warning {
				t.Errorf("warning=%q want=%v", warning, tc.warning)
			}
			if tc.warning && !strings.Contains(warning, "Do not repost") {
				t.Error("missing no-repost guidance")
			}
		})
	}
}

func TestIssueCommentDurableInputRefusesBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		attach    bool
	}{
		{"bad_uuid", "bad", false}, {"zero_uuid", "00000000-0000-0000-0000-000000000000", false},
		{"uppercase", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", false}, {"blank", "", false}, {"attachment", durableCLIKey, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chdirWithDaemonTaskMarker(t)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{}`) }))
			defer srv.Close()
			setCLITestServerEnv(t, srv.URL)
			t.Setenv("MULTICA_TOKEN", "mat_test-token")
			cmd := newIssueCommentAddTestCmd()
			cmd.Flags().String("idempotency-key", "", "")
			_ = cmd.Flags().Set("idempotency-key", tc.key)
			_ = cmd.Flags().Set("content", "Inspect this change.")
			if tc.attach {
				_ = cmd.Flags().Set("attachment", "https://example.invalid/file")
			}
			err := runIssueCommentAdd(cmd, []string{durableCLIIssue})
			if err == nil || calls.Load() != 0 {
				t.Errorf("error=%v calls=%d, want local refusal", err, calls.Load())
			}
		})
	}
}

func TestIssueCommentDurableTransportDoesNotRetry(t *testing.T) {
	chdirWithDaemonTaskMarker(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()
	setCLITestServerEnv(t, srv.URL)
	t.Setenv("MULTICA_TOKEN", "mat_test-token")
	cmd := newIssueCommentAddTestCmd()
	cmd.Flags().String("idempotency-key", "", "")
	_ = cmd.Flags().Set("idempotency-key", durableCLIKey)
	_ = cmd.Flags().Set("content", "Inspect this change.")
	if err := runIssueCommentAdd(cmd, []string{durableCLIIssue}); err == nil {
		t.Fatal("expected uncertain transport error")
	}
	if calls.Load() != 1 {
		t.Errorf("calls=%d want one", calls.Load())
	}
}

func TestIssueCommentDurableRead(t *testing.T) {
	cmd, remaining, err := rootCmd.Find([]string{"issue", "comment", "receipt"})
	if err != nil || len(remaining) != 0 || cmd != issueCommentReceiptCmd {
		t.Fatalf("receipt registration: %v %v", remaining, err)
	}
	if issueCommentAddCmd.Flags().Lookup("idempotency-key") == nil || issueCommentUpdateCmd.Flags().Lookup("idempotency-key") != nil {
		t.Fatal("key must be registered only for add")
	}
	for _, status := range []int{200, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			chdirWithDaemonTaskMarker(t)
			var calls atomic.Int32
			response := fmt.Sprintf(`{"key":%q,"state":"unknown","comment_id":%q,"targets":[],"future":true}`, durableCLIKey, durableCLIComment)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "GET" || r.URL.Path != "/api/issues/"+durableCLIIssue+"/delegation-receipts/"+durableCLIKey || r.Header.Get("Idempotency-Key") != "" {
					t.Errorf("unexpected recovery request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, response)
			}))
			defer srv.Close()
			setCLITestServerEnv(t, srv.URL)
			t.Setenv("MULTICA_TOKEN", "mat_test-token")
			stderr := captureStderr(t)
			defer stderr.restore()
			out, err := captureStdout(t, func() error { return runIssueCommentReceipt(cmd, []string{durableCLIIssue, durableCLIKey}) })
			if calls.Load() != 1 || (err != nil) != (status != 200) {
				t.Fatalf("calls=%d error=%v", calls.Load(), err)
			}
			if status != 200 {
				if out != "" {
					t.Fatal("failed read produced JSON")
				}
				return
			}
			var got, want any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(response), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) || !strings.Contains(stderr.read(), "Do not repost") {
				t.Fatalf("recovery mutated response or hid uncertainty: %s", out)
			}
		})
	}
}
