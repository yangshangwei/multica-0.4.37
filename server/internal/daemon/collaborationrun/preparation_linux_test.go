//go:build linux

package collaborationrun

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
	"golang.org/x/sys/unix"
)

func runPreparedTestFixture() (int, bool) {
	if len(os.Args) == 2 && os.Args[1] == "--prepared-supervisor-fixture" {
		return runPreparedSupervisorFixture(), true
	}
	if os.Args[0] == "/claude" {
		return runPreparedClaudeFixture(), true
	}
	if os.Args[0] == "/git" {
		return runPreparedGitFixture(), true
	}
	if len(os.Args) > 1 && os.Args[1] == "--prepared-test-agent" {
		return runPreparedAgentFixture(), true
	}
	return 0, false
}

func runPreparedAgentFixture() int {
	var input ProviderInput
	if json.NewDecoder(os.Stdin).Decode(&input) != nil || input.Preparation == nil || len(input.Preparation.Attachments) != 1 {
		return 70
	}
	file := input.Preparation.Attachments[0]
	content, err := os.ReadFile(filepath.Join("/work", file.Destination))
	if err != nil || string(content) != "data" || input.Preparation.Invocation.Instructions != "frozen system instructions" {
		return 71
	}
	if os.Getenv("HOME") != "/home/provider" || os.Getenv("ANTHROPIC_API_KEY") != "explicit-native-fixture-key" || os.Getenv("S4_AMBIENT_SECRET") != "" ||
		os.Getenv("S4_EXPLICIT_NATIVE_KEY") != "" || os.Getenv("MULTICA_COLLABORATION_TOKEN") != "mcb_fixture" {
		return 72
	}
	if files, err := os.ReadDir("/home/provider"); err != nil || len(files) != 0 {
		return 73
	}
	if _, err := os.Stat("/.preparation"); !os.IsNotExist(err) {
		return 74
	}
	if os.WriteFile("/agent", []byte("cannot modify the bundle"), 0600) == nil || os.WriteFile(filepath.Join("/inputs", file.Destination), []byte("cannot modify inputs"), 0600) == nil {
		return 75
	}
	if _, err := os.ReadFile(os.Args[3]); err == nil {
		return 76
	}
	if _, err := os.Readlink("/proc/1/fd/5"); err == nil {
		return 77
	}
	if len(os.Args) > 2 && os.Args[2] == "git" {
		content, err := os.ReadFile("/work/committed.txt")
		if err != nil || string(content) != "first commit" {
			return 78
		}
		for _, name := range []string{"/work/.git", "/work/dirty.txt", "/inputs/source.bundle", "/dev/urandom"} {
			if _, err := os.Stat(name); !os.IsNotExist(err) {
				return 79
			}
		}
		processes, err := os.ReadDir("/proc")
		if err != nil {
			return 80
		}
		for _, process := range processes {
			name, _ := os.ReadFile(filepath.Join("/proc", process.Name(), "comm"))
			if strings.TrimSpace(string(name)) == "git-prep-child" {
				return 81
			}
		}
	}
	result := protocol.CollaborationProviderResult{Outcome: "success", Body: "prepared"}
	if len(os.Args) > 2 && os.Args[2] == "entropy" {
		f, err := os.Open("/dev/urandom")
		if err != nil {
			return 102
		}
		var st unix.Stat_t
		if unix.Fstat(int(f.Fd()), &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFCHR || unix.Major(uint64(st.Rdev)) != 1 || unix.Minor(uint64(st.Rdev)) != 9 {
			return 103
		}
		var bytes [32]byte
		_, err = io.ReadFull(f, bytes[:])
		_ = f.Close()
		if err != nil || os.Chmod("/dev/urandom", 0777) == nil {
			return 104
		}
		writer, writeErr := os.OpenFile("/dev/urandom", os.O_WRONLY, 0)
		if writeErr == nil {
			_ = writer.Close()
		}
		result.Record = &protocol.CollaborationRunRecord{Version: 1, Completeness: "complete", Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: fmt.Sprintf("entropy_read=true;write_open=%t;metadata_write=false", writeErr == nil)}}}
	}
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		return 82
	}
	return 0
}

func runPreparedClaudeFixture() int {
	argument := func(name string) string {
		for i, value := range os.Args {
			if value == name && i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		}
		return ""
	}
	if argument("--append-system-prompt") != "frozen system instructions" || argument("--model") != "fixture-model" || argument("--effort") != "high" ||
		argument("--mcp-config") != `{"mcpServers":{}}` || os.Getenv("IS_SANDBOX") != "1" || os.Getenv("ANTHROPIC_API_KEY") != "explicit-native-fixture-key" || os.Getenv("S4_AMBIENT_SECRET") != "" {
		return 83
	}
	// These are process-boundary checks. The pure argument suite owns the
	// option matrix; this fixture never substitutes for an actual Claude smoke.
	if !slices.Contains(os.Args, "--setting-sources=") || !slices.Contains(os.Args, "--disable-slash-commands") ||
		!slices.Contains(os.Args, "--strict-mcp-config") || argument("--settings") != `{"disableAllHooks":true}` {
		return 105
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"control_request","request_id":"native-control","request":{"subtype":"can_use_tool","tool_name":"Read","input":{"run_in_background":true,"count":9007199254740993}}}`)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	user, control := false, false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"type":"user"`) {
			var frame struct {
				Message struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal(scanner.Bytes(), &frame) != nil || len(frame.Message.Content) != 1 {
				return 84
			}
			var input ProviderInput
			if json.Unmarshal([]byte(frame.Message.Content[0].Text), &input) != nil || input.Preparation == nil {
				return 85
			}
			file := input.Preparation.Attachments[0]
			if data, err := os.ReadFile(filepath.Join("/work", file.Destination)); err != nil || string(data) != "data" {
				return 86
			}
			if input.Preparation.Git != nil {
				for name, expected := range preparedClaudeRepositoryFiles {
					data, err := os.ReadFile(filepath.Join("/work", name))
					if err != nil || string(data) != expected {
						return 106
					}
				}
				if os.Getenv("ANTHROPIC_BASE_URL") != "" || os.Getenv("S4_REPOSITORY_ENV") != "" {
					return 107
				}
			}
			user = true
		} else if strings.Contains(line, `"type":"control_response"`) {
			if !strings.Contains(line, `"count":9007199254740993`) || !strings.Contains(line, `"run_in_background":false`) {
				return 87
			}
			control = true
		}
		if user && control {
			break
		}
	}
	if !user || !control {
		return 88
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"assistant","message":{"content":[{"type":"text","text":"prepared"}]}}`)
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":false,"result":"prepared","usage":{"input_tokens":9007199254740993,"output_tokens":0},"total_cost_usd":1e-10}`)
	return 0
}

func runPreparedGitFixture() int {
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("MULTICA_COLLABORATION_TOKEN") != "" || os.Getenv("S4_AMBIENT_SECRET") != "" {
		return 89
	}
	if len(os.Args) > 1 && os.Args[1] == "--git-intermediate" {
		fd := os.NewFile(3, "test-ready")
		cmd := exec.Command("/git", "--git-orphan")
		cmd.ExtraFiles, cmd.Stdout, cmd.Stderr, cmd.SysProcAttr = []*os.File{fd}, os.Stdout, os.Stderr, &syscall.SysProcAttr{Setsid: true}
		if cmd.Start() != nil {
			return 90
		}
		return 0
	}
	if len(os.Args) > 1 && os.Args[1] == "--git-orphan" {
		_ = os.WriteFile("/proc/self/comm", []byte("git-prep-child\n"), 0600)
		object, err := os.Open("/inputs/source.bundle")
		if err != nil {
			return 91
		}
		defer object.Close()
		ready := os.NewFile(3, "test-ready")
		_, _ = ready.Write([]byte{1})
		_ = ready.Close()
		// Keep the object and stdout open after both parent processes exit.
		// The outer tar reader sees EOF only after the nested namespace kills us.
		time.Sleep(60 * time.Second)
		return 0
	}
	args := os.Args[1:]
	for len(args) >= 2 && (args[0] == "-c" || args[0] == "-C") {
		args = args[2:]
	}
	if len(args) == 0 {
		return 92
	}
	switch args[0] {
	case "init":
		if os.MkdirAll("/work/repository", 0700) != nil {
			return 93
		}
	case "bundle":
		if data, err := os.ReadFile("/inputs/source.bundle"); err != nil || string(data) != "fixture with extra unexported Git objects" && string(data) != "fixture blocking Git archive" {
			return 94
		}
	case "cat-file":
		_, _ = fmt.Fprintln(os.Stdout, "commit")
	case "ls-tree":
		_, _ = fmt.Fprintf(os.Stdout, "100644 blob %s %d\tcommitted.txt\x00", gitTestBlob("first commit"), len("first commit"))
	case "archive":
		if data, _ := os.ReadFile("/inputs/source.bundle"); string(data) == "fixture blocking Git archive" {
			_ = os.WriteFile("/proc/self/comm", []byte("git-prep-block\n"), 0600)
			time.Sleep(60 * time.Second)
			return 101
		}
		r, w, err := os.Pipe()
		if err != nil {
			return 95
		}
		defer r.Close()
		defer w.Close()
		cmd := exec.Command("/git", "--git-intermediate")
		cmd.ExtraFiles, cmd.Stdout, cmd.Stderr, cmd.SysProcAttr = []*os.File{w}, os.Stdout, os.Stderr, &syscall.SysProcAttr{Setsid: true}
		if cmd.Run() != nil {
			return 96
		}
		_ = w.Close()
		var started [1]byte
		if _, err := io.ReadFull(r, started[:]); err != nil || started[0] != 1 {
			return 97
		}
		archive := tar.NewWriter(os.Stdout)
		if archive.WriteHeader(&tar.Header{Name: "committed.txt", Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len("first commit"))}) != nil {
			return 98
		}
		_, _ = archive.Write([]byte("first commit"))
		if archive.Close() != nil {
			return 99
		}
	default:
		return 100
	}
	return 0
}

type preparedNativeHTTP struct {
	mu               sync.Mutex
	request          protocol.CollaborationRunClaimRequest
	claim            protocol.CollaborationRunClaimResponse
	permit           protocol.CollaborationRunPermitRequest
	checks           int
	denyAt           int
	attachmentStatus int
	outcomes         []protocol.CollaborationRunOutcome
	proofs           []protocol.CollaborationRunProof
	facts            []protocol.CollaborationPreparationObservation
	stop             bool
}

func (h *preparedNativeHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, "/claim"):
		_ = json.NewEncoder(w).Encode(h.claim)
	case strings.HasSuffix(r.URL.Path, "/permit"):
		_ = json.NewDecoder(r.Body).Decode(&h.permit)
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{Identity: h.claim.Identity, AttemptSeq: h.permit.AttemptSeq, AttemptNonce: h.permit.AttemptNonce, BusinessToken: "mcb_fixture"})
	case strings.HasSuffix(r.URL.Path, "/stop-intent"):
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: h.claim.Identity, StopRequested: h.stop})
	case strings.HasSuffix(r.URL.Path, "/preparation"):
		h.checks++
		if r.Header.Get("Authorization") != "Bearer mcb_fixture" || h.denyAt > 0 && h.checks >= h.denyAt {
			w.WriteHeader(403)
			return
		}
		_ = json.NewEncoder(w).Encode(h.claim.Preparation)
	case strings.HasSuffix(r.URL.Path, "/content"):
		file := h.claim.Preparation.Attachments[0]
		if h.attachmentStatus != 0 {
			w.WriteHeader(h.attachmentStatus)
			return
		}
		if r.Header.Get("Authorization") != "Bearer mcb_fixture" || r.URL.Query().Get("version") != file.Version {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("X-Multica-Run-ID", h.claim.Identity.RunID)
		w.Header().Set("X-Multica-Attachment-ID", file.ID)
		w.Header().Set("X-Multica-Attachment-Version", file.Version)
		_, _ = io.WriteString(w, "data")
	case strings.HasSuffix(r.URL.Path, "/observations"):
		var fact protocol.CollaborationPreparationObservation
		if json.NewDecoder(r.Body).Decode(&fact) != nil || fact.Identity != h.claim.Identity || fact.AttemptNonce != h.permit.AttemptNonce ||
			fact.ManifestDigest != h.claim.Preparation.ManifestDigest || protocol.ValidateCollaborationPreparationObservation(fact) != nil {
			w.WriteHeader(400)
			return
		}
		h.facts = append(h.facts, fact)
		_ = json.NewEncoder(w).Encode(protocol.CollaborationPreparationObservationReceipt{Identity: fact.Identity, EventID: fact.EventID, ManifestDigest: fact.ManifestDigest, Recorded: true})
	case strings.HasSuffix(r.URL.Path, "/outcome"):
		var outcome protocol.CollaborationRunOutcome
		if json.NewDecoder(r.Body).Decode(&outcome) != nil {
			w.WriteHeader(400)
			return
		}
		h.outcomes = append(h.outcomes, outcome)
		w.WriteHeader(204)
	case strings.HasSuffix(r.URL.Path, "/proof"):
		var proof protocol.CollaborationRunProof
		if json.NewDecoder(r.Body).Decode(&proof) != nil {
			w.WriteHeader(400)
			return
		}
		h.proofs = append(h.proofs, proof)
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{Identity: proof.Identity, ReceiptID: proof.ReceiptID, Accepted: true})
	default:
		w.WriteHeader(404)
	}
}

func preparedNativeController(t *testing.T, adapter string, git bool) *NamespaceController {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	file := bundleTestFile(t, "agent", raw, 0700)
	secret := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(secret, []byte("host credential sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	mode := "boundary"
	if git {
		mode = "git"
	}
	bundle := &ProviderBundleConfig{Version: 1, Provider: "claude", ProtocolFamily: "claude", AdapterID: adapter, Entrypoint: "/agent",
		Files: []ProviderBundleFile{file}, Arguments: []string{"--prepared-test-agent", mode, secret}, Limits: preparationTestCapabilities().Limits,
		Account: &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "ANTHROPIC_API_KEY", SourceEnv: "S4_EXPLICIT_NATIVE_KEY"}}}}
	bundle.Limits.TimeoutSeconds = 20
	if adapter == "claude_stream_json_v1" {
		bundle.Entrypoint, bundle.Files[0].Destination, bundle.Arguments = "/claude", "/claude", nil
	}
	if git {
		gitFile := bundleTestFile(t, "git", raw, 0700)
		objects := bundleTestFile(t, "fixture.bundle", []byte("fixture with extra unexported Git objects"), 0600)
		bundle.Git = &ProviderGitConfig{BundleFile: objects.Source, BundleSHA256: objects.SHA256, SizeBytes: objects.SizeBytes, ObjectFormat: "sha1", CommitOID: strings.Repeat("a", 40),
			Executable: "/git", Files: []ProviderBundleFile{gitFile}}
	}
	t.Setenv("S4_EXPLICIT_NATIVE_KEY", "explicit-native-fixture-key")
	t.Setenv("S4_AMBIENT_SECRET", "must-not-enter-the-namespace")
	return &NamespaceController{Bundle: bundle, HelperExecutable: self, Directory: filepath.Join(t.TempDir(), "sandboxes")}
}

func preparedNativeRun(t *testing.T, c *NamespaceController, denyAt, attachmentStatus int, hooks ...func(*preparedNativeHTTP)) (*preparedNativeHTTP, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := c.Probe(ctx); err != nil {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatalf("required native preparation containment: %v", err)
		}
		t.Skipf("native preparation kernel unavailable: %v", err)
	}
	handler, server := preparedNativeHTTPFixture(t, c, denyAt, attachmentStatus)
	defer server.Close()
	client, err := NewClient(server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Ledger: newTestLedger(t), Client: client, Controller: c, RuntimeConfigEpoch: "5", StopPollInterval: 20 * time.Millisecond}
	for _, hook := range hooks {
		hook(handler)
	}
	err = runner.Run(ctx, handler.claim.Identity.RunID, handler.request)
	return handler, err
}

func preparedNativeHTTPFixture(t *testing.T, c *NamespaceController, denyAt, attachmentStatus int) (*preparedNativeHTTP, *httptest.Server) {
	t.Helper()
	request, claim := preparationTestClaim(t)
	caps, err := c.Bundle.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	request.BundleDigest = caps.Provider.BundleDigest
	claim.Preparation.Provider, claim.Preparation.Limits, claim.Preparation.Git = caps.Provider, caps.Limits, caps.Git
	claim.Preparation.Invocation = protocol.CollaborationPreparationInvocation{Instructions: "frozen system instructions", Model: "fixture-model", ThinkingLevel: "high"}
	if err := protocol.SealCollaborationPreparation(claim.Preparation); err != nil {
		t.Fatal(err)
	}
	handler := &preparedNativeHTTP{claim: claim, request: request, denyAt: denyAt, attachmentStatus: attachmentStatus}
	server := httptest.NewServer(handler)
	return handler, server
}

func TestLinuxPreparedRunMaterializesFilesAndUsesFreshExplicitAccount(t *testing.T) {
	for _, adapter := range []string{"static_json_v1", "claude_stream_json_v1"} {
		t.Run(adapter, func(t *testing.T) {
			h, err := preparedNativeRun(t, preparedNativeController(t, adapter, false), 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" || len(h.outcomes) != 1 || h.outcomes[0].Body != "prepared" || h.checks != 3 {
				t.Fatalf("missing prepared execution: proof=%+v outcomes=%+v checks=%d facts=%+v", h.proofs, h.outcomes, h.checks, h.facts)
			}
			if adapter == "claude_stream_json_v1" {
				record := h.outcomes[0].Record
				if record == nil || record.Usage == nil || record.Usage.InputTokens == nil || *record.Usage.InputTokens != "9007199254740993" ||
					record.Usage.OutputTokens == nil || *record.Usage.OutputTokens != "0" || record.Usage.TotalTokens != nil || record.Usage.CostUSD == nil || *record.Usage.CostUSD != "0.0000000001" {
					t.Fatal("native codec lost exact or missing usage")
				}
			}
		})
	}
}

func TestLinuxPreparedRunDistinguishesPreburnAndContainedFailure(t *testing.T) {
	for _, afterBurn := range []bool{false, true} {
		t.Run(fmt.Sprint(afterBurn), func(t *testing.T) {
			denyAt, status := 0, 503
			wantProof := "never_launched_proven"
			if afterBurn {
				denyAt, status, wantProof = 3, 0, "exited_proven"
			}
			h, err := preparedNativeRun(t, preparedNativeController(t, "static_json_v1", false), denyAt, status)
			if !afterBurn && err == nil {
				t.Fatal("download failure was hidden")
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.proofs) != 1 || h.proofs[0].State != wantProof || len(h.outcomes) != 0 || len(h.facts) == 0 || h.facts[len(h.facts)-1].State != "failed" {
				t.Fatalf("preparation failure fabricated launch/outcome/proof: proofs=%+v outcomes=%d facts=%+v err=%v", h.proofs, len(h.outcomes), h.facts, err)
			}
		})
	}
}

func TestLinuxPreparedGitReapsDetachedObjectReaderBeforeProvider(t *testing.T) {
	h, err := preparedNativeRun(t, preparedNativeController(t, "static_json_v1", true), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" || len(h.outcomes) != 1 || h.outcomes[0].Body != "prepared" {
		t.Fatalf("contained Git export did not reach provider after subtree exit: proofs=%+v outcomes=%+v facts=%+v", h.proofs, h.outcomes, h.facts)
	}
	gitPrepared := false
	for _, fact := range h.facts {
		gitPrepared = gitPrepared || fact.Stage == "git" && fact.State == "prepared"
	}
	if !gitPrepared {
		t.Fatal("Git preparation fact missing")
	}
}

func TestLinuxPreparedEntropyRequiresExplicitProviderDevice(t *testing.T) {
	c := preparedNativeController(t, "static_json_v1", false)
	c.Bundle.Devices = []string{"/dev/urandom"}
	c.Bundle.Arguments[1] = "entropy"
	h, err := preparedNativeRun(t, c, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.outcomes) != 1 || h.outcomes[0].Record == nil || len(h.outcomes[0].Record.Messages) != 1 ||
		h.outcomes[0].Record.Messages[0].Body != "entropy_read=true;write_open=true;metadata_write=false" {
		t.Fatalf("actual explicit character-device semantics changed: outcomes=%+v proofs=%+v", h.outcomes, h.proofs)
	}
}

func TestLinuxPreparationProbeNeverReadsAccountOrGitInputs(t *testing.T) {
	c := preparedNativeController(t, "static_json_v1", true)
	c.Bundle.Git.BundleFile = filepath.Join(t.TempDir(), "must-not-open.bundle")
	c.Bundle.Git.Files[0].Source = filepath.Join(t.TempDir(), "must-not-open-git")
	c.Bundle.Account.Environment[0].SourceEnv = "S4_EXPLICIT_EMPTY_PROBE_ACCOUNT"
	t.Setenv("S4_EXPLICIT_EMPTY_PROBE_ACCOUNT", "")
	if err := c.Probe(t.Context()); err != nil {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatalf("business-free probe failed: %v", err)
		}
		t.Skipf("native probe unavailable: %v", err)
	}
}
