package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This subprocess is the fake installed CLI. Its facts come from a file written
// by the separate fake runtime executable, never a receipt supplied by a model.
func TestMaterialHostHelper(t *testing.T) {
	if os.Getenv("MULTICA_MATERIAL_HELPER") != "1" {
		t.Skip("helper subprocess only")
	}
	args := os.Args
	at := 0
	for i, a := range args {
		if a == "--" {
			at = i + 1
			break
		}
	}
	args = args[at:]
	file := os.Getenv("MULTICA_MATERIAL_FIXTURE")
	if len(args) > 0 && args[0] == "runtime" {
		if err := os.WriteFile(file, []byte("approved bytes\n"), 0600); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	if len(args) < 2 || args[0] != "api" {
		os.Exit(4)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Print("HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\n\r\n{}")
		os.Exit(1)
	}
	sha := strings.Repeat("a", 40)
	blob := strings.Repeat("b", 40)
	u, _ := url.Parse(args[1])
	path := u.Path
	var out any
	switch {
	case strings.Contains(path, "/git/commits/"):
		out = map[string]any{"sha": sha, "tree": map[string]string{"sha": sha}}
	case strings.Contains(path, "/git/trees/"):
		out = map[string]any{"sha": sha, "truncated": false, "tree": []any{map[string]string{"path": "prd.md", "mode": "100644", "type": "blob", "sha": blob}}}
	case strings.Contains(path, "/git/blobs/"):
		out = map[string]any{"sha": blob, "encoding": "base64", "size": len(raw), "content": base64.StdEncoding.EncodeToString(raw)}
	default:
		fmt.Print("HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\n\r\n{}")
		os.Exit(1)
	}
	fmt.Print("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n")
	_ = json.NewEncoder(os.Stdout).Encode(out)
	os.Exit(0)
}
func materialFakeHost(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "remote-file")
	t.Setenv("MULTICA_MATERIAL_HELPER", "1")
	t.Setenv("MULTICA_MATERIAL_FIXTURE", fixture)
	exe := filepath.Join(dir, "gh")
	body := "#!/bin/sh\nexec '" + strings.ReplaceAll(bin, "'", "'\\''") + "' -test.run '^TestMaterialHostHelper$' -- \"$@\"\n"
	if err := os.WriteFile(exe, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return exe, fixture
}
func materialTestExecution() protocol.MaterialExecution {
	e := protocol.MaterialExecution{ProtocolVersion: 1, OperationID: uuid.NewString(), TaskID: uuid.NewString(), RuntimeID: uuid.NewString(), WorkspaceID: uuid.NewString(), Action: "verify_baseline", Provider: "github", Repository: "org/repo", BaselineCommit: strings.Repeat("a", 40), Files: []protocol.MaterialFile{{Path: "prd.md", Mode: "100644", Content: "approved bytes\n", SHA256: material.Digest("approved bytes\n")}}}
	e.ExpectedDigest = e.Digest()
	return e
}
func TestMaterialFakeRuntimeIndependentReadback(t *testing.T) {
	exe, fixture := materialFakeHost(t)
	e := materialTestExecution()
	reader, err := newMaterialObserver(e)
	if err != nil {
		t.Fatal(err)
	}
	binding := protocol.MaterialClaimBinding{ClaimGeneration: 7, HostChallenge: strings.Repeat("secret", 8)}
	if _, err := readMaterialObservation(context.Background(), e, binding, reader); err == nil {
		t.Fatal("missing remote content produced evidence")
	}
	// Exercise a real child process, which changes the fixture before the reader.
	if out, err := exec.Command(exe, "runtime").CombinedOutput(); err != nil {
		t.Fatalf("fake runtime: %v %s", err, out)
	}
	got, err := readMaterialObservation(context.Background(), e, binding, reader)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SourceComplete || len(got.Files) != 1 || got.Files[0].Content != e.Files[0].Content || got.Files[0].SHA256 != e.Files[0].SHA256 || got.Files[0].Commit != e.BaselineCommit {
		t.Fatalf("wrong independent facts: %+v", got)
	}
	if err := os.WriteFile(fixture, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = readMaterialObservation(context.Background(), e, binding, reader)
	if err != nil {
		t.Fatal(err)
	}
	if got.Files[0].Content == e.Files[0].Content {
		t.Fatal("observer copied expectation instead of reading actual bytes")
	}
}
func TestMaterialReceiptExactReplayAndSecretBoundary(t *testing.T) {
	exe, fixture := materialFakeHost(t)
	if err := exec.Command(exe, "runtime").Run(); err != nil {
		t.Fatal(err)
	}
	e := materialTestExecution()
	binding := protocol.MaterialClaimBinding{ClaimGeneration: 1, HostChallenge: strings.Repeat("private", 8)}
	var received [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer daemon-only" {
			t.Errorf("wrong host credential")
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(protocol.MaterialOperationResponse{Execution: e, ClaimGeneration: 1})
			return
		}
		raw, _ := io.ReadAll(r.Body)
		received = append(received, raw)
		if len(received) == 1 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("daemon-only")
	run := &materialRun{task: Task{ID: e.TaskID, MaterialExecution: &e, MaterialClaim: &binding}, receiptPath: filepath.Join(t.TempDir(), "pending.json")}
	if err := run.observe(context.Background(), client); err == nil {
		t.Fatal("lost acknowledgment was reported successful")
	}
	if st, err := os.Stat(run.receiptPath); err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("pending receipt is not private/durable")
	}
	if err := os.Remove(fixture); err != nil {
		t.Fatal(err)
	}
	if err := run.observe(context.Background(), client); err != nil {
		t.Fatalf("should replay without rereading: %v", err)
	}
	if len(received) != 2 || !bytes.Equal(received[0], received[1]) {
		t.Fatal("retry changed receipt")
	}
	prompt := BuildPrompt(run.task, "codex")
	if strings.Contains(prompt, binding.HostChallenge) || strings.Contains(prompt, "daemon-only") || strings.Contains(prompt, "issue get ") {
		t.Fatal("secret or generic issue flow entered material prompt")
	}
}
func TestMaterialObserverRefusesHostAndMutation(t *testing.T) {
	tr := materialCLITransport{executable: filepath.Join(t.TempDir(), "does-not-exist"), provider: "github", host: "api.github.com"}
	for _, test := range []struct{ method, url string }{{"POST", "https://api.github.com/repos/org/repo/pulls"}, {"GET", "https://evil.test/repos/org/repo"}, {"GET", "http://api.github.com/repos/org/repo"}} {
		req, _ := http.NewRequest(test.method, test.url, nil)
		if _, err := tr.RoundTrip(req); err == nil {
			t.Fatalf("accepted %v", test)
		}
	}
}
func TestMaterialLocalBridgeRejectsForgedReceipts(t *testing.T) {
	e := materialTestExecution()
	d := &Daemon{}
	d.registerActiveRepoCheckoutTask("mat_test", activeRepoCheckoutTask{Material: &materialRun{task: Task{MaterialExecution: &e}}})
	for _, body := range []string{`{"action":"observe","operation_id":"` + uuid.NewString() + `"}`, `{"action":"observe","operation_id":"` + e.OperationID + `","host_challenge":"fake"}`, `{"action":"observe","operation_id":"` + e.OperationID + `","files":[]}`} {
		req := httptest.NewRequest("POST", "/material", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer mat_test")
		w := httptest.NewRecorder()
		d.materialHandler().ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("forged request got %d", w.Code)
		}
	}
	req := httptest.NewRequest("POST", "/material", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	d.materialHandler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("anonymous observation accepted")
	}
}
func TestMaterialReadOnlyProviderHasNoMutationMethods(t *testing.T) {
	reader, err := materialgit.NewObserver("github", "", "org/repo", materialCLITransport{host: "api.github.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reader.(interface {
		Merge(context.Context, materialgit.MergeExpectation) (materialgit.Review, error)
	}); ok {
		t.Fatal("observer exposes remote mutation methods")
	}
}

func TestMaterialReceiptRestartIgnoresInterruptedWrite(t *testing.T) {
	e := materialTestExecution()
	binding := protocol.MaterialClaimBinding{ClaimGeneration: 1, HostChallenge: strings.Repeat("private", 8)}
	receipt := protocol.MaterialObservation{ProtocolVersion: 1, TaskID: e.TaskID, OperationID: e.OperationID, ExpectedDigest: e.ExpectedDigest, HostChallenge: binding.HostChallenge, ClaimGeneration: 1}
	raw, _ := json.Marshal(receipt)
	var received [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer daemon-only" {
			t.Error("wrong daemon credential")
		}
		body, _ := io.ReadAll(r.Body)
		received = append(received, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	d := New(Config{ServerBaseURL: server.URL, WorkspacesRoot: t.TempDir()}, nil)
	d.client.SetToken("daemon-only")
	dir := d.materialReceiptDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(dir, ".material-pending-interrupted")
	if err := os.WriteFile(partial, []byte(`{"protocol_version":`), 0600); err != nil {
		t.Fatal(err)
	}
	d.replayMaterialObservations(t.Context())
	if len(received) != 0 {
		t.Fatal("partial receipt reached server")
	}
	final := filepath.Join(dir, e.TaskID+"-1.json")
	if err := persistMaterialObservation(final, raw); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(final)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("receipt must be private")
	}
	d.replayMaterialObservations(t.Context())
	if len(received) != 1 || !bytes.Equal(received[0], raw) {
		t.Fatal("restart changed receipt")
	}
	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Fatal("acknowledged receipt retained")
	}
}

func TestMaterialClaimRefusesUnresolvedProfileFallback(t *testing.T) {
	e := materialTestExecution()
	e.DaemonID = "registered-host"
	e.ProfileID = uuid.NewString()
	e.ExpectedDigest = e.Digest()
	b := protocol.MaterialClaimBinding{ClaimGeneration: 1, HostChallenge: strings.Repeat("private", 8)}
	task := Task{ID: e.TaskID, RuntimeID: e.RuntimeID, WorkspaceID: e.WorkspaceID, MaterialExecution: &e, MaterialClaim: &b}
	d := New(Config{DaemonID: e.DaemonID, WorkspacesRoot: t.TempDir()}, nil)
	d.runtimeIndex[e.RuntimeID] = Runtime{ID: e.RuntimeID, Provider: "claude", ProfileID: e.ProfileID}
	if _, err := d.prepareMaterialRun(t.Context(), &task); err == nil || err.Error() != "material_runtime_profile_unavailable" {
		t.Fatalf("unresolved custom profile could fall back: %v", err)
	}
	d.runtimeIndex[e.RuntimeID] = Runtime{ID: e.RuntimeID, Provider: "claude"}
	if _, err := d.prepareMaterialRun(t.Context(), &task); err == nil || err.Error() != "material_runtime_changed" {
		t.Fatalf("changed profile accepted: %v", err)
	}
}

func TestMaterialObserverRefusesGitLabInstancePrefix(t *testing.T) {
	e := materialTestExecution()
	e.Provider, e.InstanceURL = "gitlab", "https://gitlab.example.test/prefix"
	if _, err := newMaterialObserver(e); err == nil || err.Error() != "material_instance_prefix_unsupported" {
		t.Fatalf("prefixed API could reach wrong path: %v", err)
	}
}

func TestMaterialHostOutputDoesNotReuseWireFraming(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "host-cli")
	script := "#!/bin/sh\nprintf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\nContent-Length: 2\\r\\nContent-Encoding: gzip\\r\\n\\r\\n{\\n  \"value\": true\\n}\\n'\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	transport := materialCLITransport{executable: executable, provider: "gitlab", host: "gitlab.com", env: materialObserverEnvironment()}
	request, _ := http.NewRequest("GET", "https://gitlab.com/api/v4/projects/org%2Frepo", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct{ Value bool }
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || !body.Value {
		t.Fatalf("CLI JSON was truncated by wire length: %v", err)
	}
	if response.Header.Get("Content-Encoding") != "" || response.ContentLength != -1 {
		t.Fatal("decoded CLI output retained wire framing")
	}
}
