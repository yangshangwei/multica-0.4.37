package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type hostKnowledgeFixture struct {
	*launchHandoffFixture
	mu                         sync.Mutex
	preparations               []protocol.CollaborationRunKnowledgePrepare
	authorizations             []protocol.CollaborationRunKnowledgeAuthorization
	contribution               protocol.CollaborationKnowledgeContribution
	refuse                     bool
	losePrepare, loseAuthorize bool
}

func newHostKnowledgeFixture(t *testing.T) *hostKnowledgeFixture {
	t.Helper()
	h := newLaunchHandoffFixture(t, "result")
	h.claim.KnowledgeContext = &protocol.CollaborationRunKnowledgeClaim{ProtocolVersion: 1, Identity: h.claim.Identity, ConfigurationDigest: h.claim.Preparation.Host.ConfigurationDigest, Namespace: h.claim.Identity.RunID}
	f := &hostKnowledgeFixture{launchHandoffFixture: h, contribution: protocol.CollaborationKnowledgeContribution{CandidateID: uuid.NewString(), Version: "2", Content: "PRIVATE_C4_APPROVED_INPUT", Summary: "Approved current version", Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: uuid.NewString()}, SourceIssueID: h.claim.Identity.IssueID, Sources: []protocol.CollaborationKnowledgeSource{}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"collaboration"}}}}
	f.contribution.ContentSHA256 = protocol.CollaborationKnowledgeHash(f.contribution.Content)
	f.contribution.Sources = []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: h.claim.Identity.IssueID, Version: "3", SHA256: protocol.CollaborationKnowledgeHash(h.memoryContent)}}
	original := h.server.Config.Handler
	h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/knowledge-context/") {
			original.ServeHTTP(w, r)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer mdt_fixture" || r.Method != "POST" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("Cookie") != "" {
			t.Error("knowledge private call lost managed credential boundary")
			w.WriteHeader(403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/prepare") {
			var p protocol.CollaborationRunKnowledgePrepare
			if json.NewDecoder(r.Body).Decode(&p) != nil {
				t.Error("invalid preparation request")
				return
			}
			f.preparations = append(f.preparations, p)
			if f.losePrepare {
				f.losePrepare = false
				c, _, _ := w.(http.Hijacker).Hijack()
				_ = c.Close()
				return
			}
			c := f.contribution
			manifest := protocol.TaskKnowledgeManifest{ProtocolVersion: 1, Entries: []protocol.TaskKnowledgeManifestEntry{{CandidateID: c.CandidateID, Version: c.Version, ContentSHA256: c.ContentSHA256, SourceIssueID: c.SourceIssueID, Target: c.Target}}}
			raw, _ := json.Marshal(manifest)
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunKnowledgeBundle{CollaborationRunKnowledgePrepare: p, Manifest: manifest, ManifestSHA256: protocol.CollaborationKnowledgeHash(string(raw)), Contributions: []protocol.CollaborationKnowledgeContribution{c}, Targets: []protocol.TaskKnowledgeTargetRevision{}})
			return
		}
		var p protocol.CollaborationRunKnowledgeAuthorization
		if json.NewDecoder(r.Body).Decode(&p) != nil {
			t.Error("invalid authorization request")
			return
		}
		f.authorizations = append(f.authorizations, p)
		if f.refuse {
			w.WriteHeader(409)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "knowledge_context_changed"})
			return
		}
		if f.loseAuthorize {
			f.loseAuthorize = false
			c, _, _ := w.(http.Hijacker).Hijack()
			_ = c.Close()
			return
		}
		_ = json.NewEncoder(w).Encode(p)
	})
	return f
}

func knowledgeCapturedPrompt(t *testing.T, start map[string]any) string {
	t.Helper()
	var frame struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(start["input"].(string)), &frame) != nil || len(frame.Message.Content) != 1 {
		t.Fatal("missing captured stdin")
	}
	return frame.Message.Content[0].Text
}

func TestHostKnowledgeActualInputPreservesHandoffAndFreshNamespace(t *testing.T) {
	f := newHostKnowledgeFixture(t)
	legacy := f.claim
	legacy.KnowledgeContext = nil
	legacyKey, err := hostSessionKey(legacy)
	if err != nil {
		t.Fatal(err)
	}
	session, err := f.runner.Ledger.lockHostSession(legacyKey, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err = session.save(hostFixtureSession, false); err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
	if err = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.starts(t)
	if len(starts) != 1 {
		t.Fatalf("physical launches=%d", len(starts))
	}
	prompt := knowledgeCapturedPrompt(t, starts[0])
	if !strings.Contains(prompt, f.contribution.Content) || !strings.Contains(prompt, f.envelope.Sources[1].Content) {
		t.Fatal("actual stdin lost knowledge or original delegation")
	}
	for _, input := range f.claim.Inputs {
		if !strings.Contains(prompt, input.Body) || !strings.Contains(prompt, input.AuthorUserID) {
			t.Fatal("knowledge replaced accepted input/author")
		}
	}
	if len(f.authorizations) != 1 || f.authorizations[0].PromptSHA256 != protocol.CollaborationHandoffHash(prompt) || f.bindings[0].PromptSHA256 != f.authorizations[0].PromptSHA256 {
		t.Fatal("physical stdin lacks exact shared binding")
	}
	for _, arg := range starts[0]["args"].([]any) {
		if strings.Contains(arg.(string), "resume") || arg == "-c" || arg == "--continue" {
			t.Fatal("knowledge invocation resumed native history")
		}
	}
	key, err := hostSessionKey(f.claim)
	if err != nil || key == legacyKey || !strings.Contains(starts[0]["cwd"].(string), key) {
		t.Fatal("fresh namespace is not effective cwd")
	}
	for _, dir := range []string{f.runner.Ledger.dir, f.runner.Host.Directory} {
		if err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if strings.Contains(string(raw), f.contribution.Content) {
				t.Errorf("private input persisted in %s", path)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	session, err = f.runner.Ledger.lockHostSession(key, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.record.SessionID != "" {
		t.Fatal("knowledge native pointer became reusable")
	}
}

func TestHostKnowledgeLostRepliesKeepOriginalManifestAndAttempt(t *testing.T) {
	f := newHostKnowledgeFixture(t)
	f.losePrepare, f.loseAuthorize = true, true
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	if len(f.preparations) != 2 || f.preparations[0] != f.preparations[1] || len(f.authorizations) != 2 || f.authorizations[0] != f.authorizations[1] || len(f.starts(t)) != 1 {
		t.Fatal("uncertainty replaced original request or replayed business input")
	}
}

func TestHostKnowledgeFirstRefusalPreservesNeverLaunchedProof(t *testing.T) {
	f := newHostKnowledgeFixture(t)
	f.refuse = true
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err == nil {
		t.Fatal("knowledge refusal ignored")
	}
	if len(f.preparations) != 1 || len(f.authorizations) != 1 || len(f.starts(t)) != 0 || len(f.outcomes) != 0 || len(f.proofs) != 1 || f.proofs[0].State != "never_launched_proven" {
		t.Fatal("refusal started provider or starved original proof")
	}
	before := len(f.preparations)
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	if len(f.preparations) != before || len(f.starts(t)) != 0 {
		t.Fatal("recovery replayed refused input")
	}
}

func TestHostKnowledgeLaterPhysicalLaunchRefusalKeepsFirstTermination(t *testing.T) {
	f := newHostKnowledgeFixture(t)
	if err := f.runner.Host.Probe(t.Context()); err != nil {
		t.Fatal(err)
	}
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	key, err := hostSessionKey(f.claim)
	if err != nil {
		t.Fatal(err)
	}
	if err = local.registerHost(f.claim, key, f.request.CapabilityDigest, f.claim.Preparation.Host.ConfigurationDigest); err != nil {
		t.Fatal(err)
	}
	if err = local.hostSessionAcquired(); err != nil {
		t.Fatal(err)
	}
	s := &hostSupervisor{runner: f.runner, local: local, claim: f.claim}
	s.lifecycle = agent.NewLaunchLifecycle(s)
	if err = s.authorize(t.Context(), agent.LaunchKindProvider); err != nil {
		t.Fatal(err)
	}
	s.prepared, err = f.runner.Host.prepare(t.Context(), f.claim, key, s.businessToken, f.runner.Client, s.lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	memory, err := f.runner.hostMemory(t.Context(), f.claim.Identity, s.businessToken())
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := f.runner.hostHandoff(t.Context(), f.claim, s.businessToken(), memory, nil)
	if err != nil {
		t.Fatal(err)
	}
	opts := f.runner.Host.Options
	opts.Cwd, opts.Lifecycle = s.prepared.Work, s.lifecycle
	prompt, err := hostPromptWithHandoff(f.claim, false, memory, handoff)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err = s.prepareKnowledge(t.Context(), prompt, opts, handoff)
	if err != nil {
		t.Fatal(err)
	}
	s.setHandoff(handoff, prompt)
	env, err := f.runner.Host.environment(f.claim, s.prepared, f.runner.Client.baseURL, s.businessToken())
	if err != nil {
		t.Fatal(err)
	}
	backend, err := agent.ResolveBackend("claude", agent.Config{ExecutablePath: f.runner.Host.Config.Executable, Env: env, Logger: f.runner.Host.logger(), BuiltinRuntime: true, RuntimeID: f.claim.HostContext.RuntimeID})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	record := newHostRecording(s.prepared, f.runner.Host.Config.Executable)
	result, got, err := f.runner.Host.execute(ctx, cancel, backend, prompt, opts, record)
	if err != nil || !got || result.Usage == nil {
		t.Fatalf("first real launch: %v", err)
	}
	f.refuse = true
	_, _, err = f.runner.Host.execute(ctx, cancel, backend, prompt, opts, record)
	if err == nil {
		t.Fatal("second physical launch ignored revoke")
	}
	_ = local.stopHost()
	if err = f.runner.recoverHost(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	if len(f.starts(t)) != 1 || len(f.terminations) != 2 || len(f.proofs) != 1 || f.proofs[0].State != "host_exit_observed" {
		t.Fatal("later refusal lost earlier effects/termination or replayed input")
	}
	if f.authorizations[0].AttemptNonce == f.authorizations[1].AttemptNonce {
		t.Fatal("second launch reused physical attempt")
	}
}

func TestHostKnowledgeUnsupportedAndDowngradeCannotSelectNamespace(t *testing.T) {
	f := newHostKnowledgeFixture(t)
	if validateHostKnowledgeClaim(f.claim, 0) == nil {
		t.Fatal("unnegotiated client accepted managed namespace")
	}
	other := f.claim
	other.Identity.RunID = uuid.NewString()
	d := *other.KnowledgeContext
	d.Identity = other.Identity
	d.Namespace = other.Identity.RunID
	other.KnowledgeContext = &d
	a, _ := hostSessionKey(f.claim)
	b, _ := hostSessionKey(other)
	if a == b {
		t.Fatal("separate knowledge runs share native history")
	}
	for _, args := range [][]string{{"--continue"}, {"--resume", hostFixtureSession}, {"--settings", "custom"}, {"--unknown"}} {
		f.runner.Host.Config.ExtraArgs = args
		if f.runner.Host.supportsKnowledge() {
			t.Fatalf("unsafe runtime args negotiated: %v", args)
		}
	}
	f.runner.Host.Config.ExtraArgs = nil
	f.runner.Host.Options.Knowledge = &agent.KnowledgeLaunchIntent{}
	if f.runner.Host.Probe(t.Context()) == nil {
		t.Fatal("ordinary knowledge lifecycle leaked into collaboration")
	}
	legacy := f.claim
	legacy.KnowledgeContext = nil
	if err := validateHostKnowledgeClaim(legacy, 1); err != nil {
		t.Fatal("old server unsupported response no longer usable")
	}
	if reflect.DeepEqual(legacy.KnowledgeContext, f.claim.KnowledgeContext) {
		t.Fatal("fixture did not cover legacy response")
	}
}

func TestHostKnowledgeRetainedRunRejectsOptionOverridesBeforeDisclosure(t *testing.T){
	f:=newHostKnowledgeFixture(t)
	for _,change:=range []func(*agent.ExecOptions){
		func(o *agent.ExecOptions){o.ClaudeSettingsPath="/private/unapproved/settings.json"},
		func(o *agent.ExecOptions){o.CustomArgs=[]string{"--continue"}},
		func(o *agent.ExecOptions){o.ResumeSessionID=hostFixtureSession},
	}{
		o:=f.runner.Host.Options;change(&f.runner.Host.Options)
		if f.runner.Host.supportsKnowledge(){t.Error("retained host options bypassed knowledge capability guard")}
		f.runner.Host.Options=o
	}
}
