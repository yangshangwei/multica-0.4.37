package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func outputReceiptFixture(c *CollaborationContext, request, agent string, file CollaborationOutputFile) map[string]any {
	return map[string]any{"comment_id": request, "run_ids": []string{}, "continuation_run_id": nil, "attachment": map[string]any{
		"id": uuid.NewString(), "draft_id": nil, "issue_id": c.Identity.IssueID, "intended_issue_id": nil,
		"source_workspace_id": uuid.NewString(), "uploader_user_id": nil, "uploader_agent_id": agent, "run_id": c.Identity.RunID, "attempt_seq": "1",
		"name": file.Name, "content_type": http.DetectContentType(file.Data), "size_bytes": len(file.Data), "version": "1",
		"created_at": "2026-11-19T01:02:03.123456Z", "delivered_at": "2026-11-19T01:02:03.123456Z"}}
}

func TestCollaborationOutputO09FileReadUsesProcessCWD(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.WriteFile("output.txt", []byte("original local output"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"output.txt", filepath.Join(cwd, "output.txt")} {
		got, err := ReadCollaborationOutputFile(name)
		if err != nil || got.Name != "output.txt" || string(got.Data) != "original local output" {
			t.Fatalf("contained regular file %q lost its actual bytes: %+v %v", name, got, err)
		}
	}
	external := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(external, []byte("outside output"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, "escape.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("output.txt", "inside.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadCollaborationOutputFile("inside.txt"); err != nil || string(got.Data) != "original local output" {
		t.Fatalf("contained symlink lost its pinned CWD read: %v", err)
	}
	large, err := os.Create("oversized.bin")
	if err != nil {
		t.Fatal(err)
	}
	if err = large.Truncate(collaborationOutputMaxBytes + 1); err != nil {
		_ = large.Close()
		t.Fatal(err)
	}
	if err = large.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "https://example.test/file.txt", "file:///etc/passwd", "~/.secret", "missing.txt", ".", external, "escape.txt", "oversized.bin", "../external.txt"} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadCollaborationOutputFile(name); err == nil {
				t.Fatalf("invalid, nonregular, escaping or oversized file %q was accepted", name)
			}
		})
	}
}

func TestCollaborationOutputO09TypedReceiptRequiresCompleteFileIdentity(t *testing.T) {
	id := collaborationEnvironmentFixture(t)
	c := &CollaborationContext{Identity: id}
	request, agent := uuid.NewString(), uuid.NewString()
	file := CollaborationOutputFile{Name: "original.txt", Data: []byte("original output")}
	valid := outputReceiptFixture(c, request, agent, file)
	raw, _ := json.Marshal(valid)
	if out, err := decodeCollaborationOutputReceipt(raw, c, request, agent, file); err != nil || out.Attachment == nil {
		t.Fatalf("complete original receipt refused: %v", err)
	}
	for _, scenario := range []string{"comment", "no_attachment", "id", "issue_id", "run_id", "uploader_agent_id", "uploader_user_id", "draft_id", "intended_issue_id", "attempt_seq", "name", "content_type", "size_bytes", "version", "created_at", "delivered_at", "missing_nullable", "missing_size", "null_size", "run_ids", "continuation", "duplicate_field", "trailing_json"} {
		t.Run(scenario, func(t *testing.T) {
			value := outputReceiptFixture(c, request, agent, file)
			a := value["attachment"].(map[string]any)
			switch scenario {
			case "comment":
				value["comment_id"] = uuid.NewString()
			case "no_attachment":
				delete(value, "attachment")
			case "id":
				a["id"] = uuid.Nil.String()
			case "issue_id", "run_id", "uploader_agent_id", "uploader_user_id", "draft_id", "intended_issue_id":
				a[scenario] = uuid.NewString()
			case "attempt_seq":
				a[scenario] = "01"
			case "name", "content_type":
				a[scenario] = "changed"
			case "size_bytes":
				a[scenario] = len(file.Data) + 1
			case "version":
				a[scenario] = "2"
			case "created_at", "delivered_at":
				a[scenario] = nil
			case "missing_nullable":
				delete(a, "uploader_user_id")
			case "missing_size":
				delete(a, "size_bytes")
			case "null_size":
				a["size_bytes"] = nil
			case "run_ids":
				value["run_ids"] = nil
			case "continuation":
				value["continuation_run_id"] = "not-a-run"
			}
			data, _ := json.Marshal(value)
			if scenario == "duplicate_field" {
				data = []byte(strings.TrimSuffix(string(data), "}") + `,"comment_id":"` + request + `"}`)
			}
			if scenario == "trailing_json" {
				data = append(data, []byte(`{}`)...)
			}
			if _, err := decodeCollaborationOutputReceipt(data, c, request, agent, file); err == nil {
				t.Fatalf("incomplete or uncorrelated %s receipt was accepted", scenario)
			}
		})
	}
}

func TestCollaborationOutputO09MultipartPreservesOriginalIntentAcrossUnknownACK(t *testing.T) {
	id := collaborationEnvironmentFixture(t)
	c := &CollaborationContext{Identity: id, Token: "mcb_" + strings.Repeat("a", 64)}
	request, parent, agent := uuid.NewString(), uuid.NewString(), uuid.NewString()
	file := CollaborationOutputFile{Name: "original.txt", Data: []byte("original output bytes")}
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != c.Path("/comments/attachments") || r.Header.Get("Authorization") != "Bearer "+c.Token || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
			t.Errorf("output request escaped its original run authority: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if !reflect.DeepEqual(r.MultipartForm.Value, map[string][]string{"comment_id": {request}, "parent_id": {parent}, "content": {"original body"}}) || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
			t.Error("multipart changed identity/body/parent or accepted authority fields")
		}
		part, err := r.MultipartForm.File["file"][0].Open()
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil || !bytes.Equal(data, file.Data) || r.MultipartForm.File["file"][0].Filename != file.Name {
			t.Error("multipart changed the original file name or bytes")
		}
		if posts.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(outputReceiptFixture(c, request, agent, file))
	}))
	defer server.Close()
	c.Server = server.URL
	ctx := t.Context()
	if _, err := c.CreateCommentAttachment(ctx, c.Client(), request, parent, "original body", agent, file); err == nil || !strings.Contains(FormatError(err, false), request) || posts.Load() != 1 {
		t.Fatalf("unknown original ACK was retried automatically or lost its key: posts=%d err=%v", posts.Load(), err)
	}
	if _, err := c.CreateCommentAttachment(ctx, c.Client(), request, parent, "original body", agent, file); err != nil || posts.Load() != 2 {
		t.Fatalf("explicit original-file retry did not recover: posts=%d err=%v", posts.Load(), err)
	}
}

func TestCollaborationOutputO09RejectsRedirectAndMalformedSuccess(t *testing.T) {
	for _, scenario := range []string{"redirect", "wrong-file", "truncated", "forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			id := collaborationEnvironmentFixture(t)
			c := &CollaborationContext{Identity: id, Token: "mcb_" + strings.Repeat("a", 64)}
			request, agent := uuid.NewString(), uuid.NewString()
			file := CollaborationOutputFile{Name: "original.txt", Data: []byte("original output")}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch scenario {
				case "redirect":
					w.Header().Set("Location", "/ordinary-profile-trap")
					w.WriteHeader(http.StatusTemporaryRedirect)
				case "wrong-file":
					value := outputReceiptFixture(c, request, agent, file)
					value["attachment"].(map[string]any)["run_id"] = uuid.NewString()
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(value)
				case "truncated":
					w.WriteHeader(http.StatusCreated)
					_, _ = io.WriteString(w, `{"comment_id":"`+request+`"`)
				case "forbidden":
					w.WriteHeader(http.StatusForbidden)
					_, _ = io.WriteString(w, `{"code":"business_capability_required"}`)
				}
			}))
			defer server.Close()
			c.Server = server.URL
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			_, err := c.CreateCommentAttachment(ctx, c.Client(), request, "", "original body", agent, file)
			if err == nil || requests.Load() != 1 {
				t.Fatalf("invalid output %s reached a follow-up request or passed: requests=%d err=%v", scenario, requests.Load(), err)
			}
			if scenario != "forbidden" && !strings.Contains(FormatError(err, false), request) {
				t.Fatal("unconfirmed output lost the original retry UUID")
			}
		})
	}
}
