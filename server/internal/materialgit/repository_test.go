package materialgit

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/material"
)

func TestRepositorySnapshotPreservesBinaryAndRejectsPartial(t *testing.T) {
	for _, kind := range []string{"github", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			truncated := false
			mode := "100644"
			binary := []byte{0, 255, 128, 42}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p, _ := url.PathUnescape(r.URL.Path)
				switch {
				case strings.Contains(p, "/git/commits/"):
					writeJSON(w, map[string]any{"sha": baseSHA, "tree": map[string]string{"sha": treeSHA}})
				case strings.Contains(p, "/git/trees/"):
					writeJSON(w, map[string]any{"sha": treeSHA, "truncated": truncated, "tree": []any{map[string]string{"path": "uat.bin", "mode": mode, "type": "blob", "sha": blobSHA}}})
				case strings.Contains(p, "/repository/tree"):
					if truncated {
						writeJSON(w, nil)
					} else {
						writeJSON(w, []any{map[string]string{"path": "uat.bin", "mode": mode, "type": "blob", "id": blobSHA}})
					}
				case strings.HasSuffix(p, "/raw"):
					w.Header().Set("Content-Type", "application/octet-stream")
					_, _ = w.Write(binary)
				case strings.Contains(p, "/blobs/"):
					writeJSON(w, map[string]any{"sha": blobSHA, "encoding": "base64", "size": len(binary), "content": base64.StdEncoding.EncodeToString(binary)})
				default:
					t.Errorf("unexpected read %s", p)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			p, err := newClient(Connection{Provider: kind, BaseURL: server.URL, Repository: "org/repo", Token: "test"}, server.Client(), true)
			if err != nil {
				t.Fatal(err)
			}
			var c *client
			switch x := p.(type) {
			case *githubProvider:
				c = x.client
			case *gitlabProvider:
				c = x.client
			}
			reader := &observer{reader: p, c: c}
			got, err := reader.Snapshot(t.Context(), baseSHA, nil)
			if err != nil || !got.Complete || len(got.Entries) != 1 || got.Entries[0].SHA256 != material.Digest(string(binary)) || got.Entries[0].Content != nil {
				t.Fatalf("binary lost: %+v %v", got, err)
			}
			if _, err := reader.Snapshot(t.Context(), baseSHA, []string{"uat.bin"}); err == nil {
				t.Fatal("managed binary accepted")
			}
			truncated = true
			if _, err := reader.Snapshot(t.Context(), baseSHA, nil); err == nil {
				t.Fatal("partial tree accepted")
			}
			truncated = false
			mode = "120000"
			if _, err := reader.Snapshot(t.Context(), baseSHA, nil); err == nil {
				t.Fatal("symlink accepted")
			}
		})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
