//go:build agentintegration && !windows

package main

// Code-route preparation is separate from consumed P1/O1 evidence. These tests
// exercise the real human resource API and task claims without launching a model.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/handler"
)

func secRemFullClone(t *testing.T, source, destination string) {
	t.Helper()
	command := exec.Command("git", "clone", "--no-hardlinks", "--no-local", source, destination)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("clone synthetic role repository: %s %v", out, err)
	}
	info, err := os.Lstat(filepath.Join(destination, ".git"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("role checkout must contain its own real Git metadata")
	}
}

func TestSecurityRemediationDirectCodeResourcePreparation(t *testing.T) {
	// The P1 fake registration is reused solely to construct an isolated actual
	// fixture; this entry never calls a provider or changes scenario expectations.
	secRemAuthorizeDirectCLI(t, "fake")
	cfg := secRemLoadHarness(t, "fake")
	f := newSecRemPlatformFixture(t, cfg)
	base := filepath.Join(cfg.OutputDir, "synthetic-seed")
	if err := os.Mkdir(base, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "marker.txt"), []byte("synthetic project resource probe\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Synthetic Fixture"}, {"config", "user.email", "fixture@example.test"}, {"add", "marker.txt"}, {"commit", "-m", "Synthetic immutable fixture baseline"}} {
		command := exec.Command("git", args...)
		command.Dir = base
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("seed repository %v: %s %v", args, out, err)
		}
	}
	one, two := filepath.Join(cfg.OutputDir, "role-one"), filepath.Join(cfg.OutputDir, "role-two")
	secRemFullClone(t, base, one)
	secRemFullClone(t, base, two)
	var project struct {
		ID string `json:"id"`
	}
	f.human(http.MethodPost, "/api/projects", map[string]any{"title": "Synthetic code route preparation", "execution_squads": []any{}}, 201, &project)
	var resource handler.ProjectResourceResponse
	f.human(http.MethodPost, "/api/projects/"+project.ID+"/resources", map[string]any{"resource_type": "local_directory", "resource_ref": map[string]any{"local_path": one, "daemon_id": f.daemonID, "label": "Synthetic role checkout", "execution_mode": "in_place"}}, 201, &resource)
	var implementer string
	for id, role := range f.roles {
		if role == "implementer" {
			implementer = id
		}
	}
	if implementer == "" {
		t.Fatal("template implementer missing")
	}
	var issue handler.IssueResponse
	f.human(http.MethodPost, "/api/issues", map[string]any{"title": "Synthetic resource binding check", "description": "Read only the synthetic marker; no model is invoked in this preparation.", "project_id": project.ID, "status": "todo", "assignee_type": "agent", "assignee_id": implementer}, 201, &issue)
	f.issue = issue.ID
	before := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id=$1`, f.workspace)
	f.human(http.MethodPut, "/api/projects/"+project.ID+"/resources/"+resource.ID, map[string]any{"resource_ref": map[string]any{"local_path": two, "daemon_id": f.daemonID, "label": "Synthetic next role checkout", "execution_mode": "in_place"}}, 200, &resource)
	after := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id=$1`, f.workspace)
	if before != 1 || after != before {
		t.Fatalf("resource switch queued work: %d -> %d", before, after)
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET last_seen_at=clock_timestamp(),status='online' WHERE id=$1`, f.runtime)
	task, err := f.driver.Claim(t.Context())
	if err != nil || task == nil || task.AgentID != implementer || task.IsLeaderTask || task.IssueID != f.issue || !strings.HasPrefix(task.AuthToken, "mat_") {
		t.Fatalf("actual code claim mismatch %t %v", task != nil, err)
	}
	matched := 0
	for _, entry := range task.ProjectResources {
		if entry.ResourceType != "local_directory" {
			continue
		}
		var ref struct {
			LocalPath     string `json:"local_path"`
			DaemonID      string `json:"daemon_id"`
			ExecutionMode string `json:"execution_mode"`
		}
		if json.Unmarshal(entry.ResourceRef, &ref) != nil {
			t.Fatal("invalid actual resource")
		}
		if ref.DaemonID == f.daemonID {
			matched++
			if ref.LocalPath != two || ref.ExecutionMode != "in_place" {
				t.Fatalf("claim froze wrong role resource %+v", ref)
			}
		}
	}
	if matched != 1 {
		t.Fatalf("claim resource matches=%d", matched)
	}
	copyTask := *task
	copyTask.AuthToken = "[credential]"
	copyTask.RemoteMCPDaemonToken = ""
	if err := secRemWriteJSON(filepath.Join(cfg.OutputDir, "code-resource-claim.json"), copyTask); err != nil {
		t.Fatal(err)
	}
	f.snapshot("code-resource-after")
	if err := secRemWriteJSON(filepath.Join(cfg.OutputDir, "code-resource-verdict.json"), map[string]any{"evidence_type": "platform-preparation-no-provider", "passed": true, "task_count_before": before, "task_count_after": after, "actual_claim_resource": two, "role": "implementer", "provider_calls": 0, "run_report_exercised": false, "note": fmt.Sprint("Actual human resource switch; no model or patch")}); err != nil {
		t.Fatal(err)
	}
}

func secRemLoadCodeHarness(t *testing.T, mode string) secRemHarnessConfig {
	t.Helper()
	// Entry gates precede provider discovery, version probes and account reads.
	if mode == "real" && os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("real security-remediation execution requires explicit opt-in")
	}
	if mode == "fake" && os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("fake harness refuses real-agent opt-in")
	}
	body, err := secRemPrivateFile(os.Getenv("SECREM_HARNESS_CONFIG"), os.Getenv("SECREM_HARNESS_CONFIG_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg secRemHarnessConfig
	if err := json.Unmarshal(body, &cfg); err != nil || cfg.Mode != mode || (cfg.Scenario != "C1" && cfg.Scenario != "D1") ||
		!strings.HasPrefix(cfg.Database, "secrem_v2_real_20261120_") || !filepath.IsAbs(cfg.OutputDir) ||
		!filepath.IsAbs(cfg.ClaimManifestDir) || cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > 1200 ||
		cfg.Model == "" || len(cfg.SourceCommit) != 40 || len(cfg.FixtureManifestSHA256) != 64 {
		t.Fatal("harness registration is incomplete or unsupported")
	}
	if !filepath.IsAbs(cfg.SourceRoot) || !filepath.IsAbs(cfg.FixtureManifestPath) {
		t.Fatal("source and fixture paths must be explicit")
	}
	head, err := exec.Command("git", "-C", cfg.SourceRoot, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != cfg.SourceCommit {
		t.Fatal("source commit differs from registration")
	}
	manifest, err := os.ReadFile(cfg.FixtureManifestPath)
	if err != nil {
		t.Fatal("fixture manifest unavailable")
	}
	hash := sha256.Sum256(manifest)
	if hex.EncodeToString(hash[:]) != cfg.FixtureManifestSHA256 {
		t.Fatal("fixture manifest changed")
	}
	if _, err := os.Stat(cfg.OutputDir); err == nil {
		t.Fatal("output directory already exists; consumed trials cannot be overwritten")
	}
	if mode == "real" {
		admission, err := secRemPrivateFile(cfg.AdmissionReceipt, cfg.AdmissionSHA256)
		if err != nil {
			t.Fatal("reviewed transport admission receipt required")
		}
		var reviewed struct {
			Verdict                   string            `json:"verdict"`
			Scope                     string            `json:"scope"`
			SourceCommit              string            `json:"source_commit"`
			FixtureManifestSHA256     string            `json:"fixture_manifest_sha256"`
			TransportProjectionSHA256 string            `json:"transport_projection_sha256"`
			ProviderSHA256            string            `json:"provider_sha256"`
			CLISHA256                 string            `json:"cli_sha256"`
			HarnessSHA256             string            `json:"harness_sha256"`
			Model                     string            `json:"model"`
			ThinkingLevel             string            `json:"thinking_level"`
			AccountAlias              string            `json:"account_alias"`
			SourceHashes              map[string]string `json:"source_hashes"`
		}
		if json.Unmarshal(admission, &reviewed) != nil || reviewed.Verdict != "approved" || reviewed.Scope != cfg.Scenario || reviewed.SourceCommit != cfg.SourceCommit || reviewed.FixtureManifestSHA256 != cfg.FixtureManifestSHA256 || reviewed.TransportProjectionSHA256 != cfg.TransportProjectionSHA256 || reviewed.ProviderSHA256 != cfg.ProviderSHA256 || reviewed.CLISHA256 != cfg.CLISHA256 || reviewed.HarnessSHA256 != cfg.HarnessSHA256 || reviewed.Model != cfg.Model || reviewed.ThinkingLevel != cfg.ThinkingLevel || reviewed.AccountAlias != cfg.AccountAlias || !reflect.DeepEqual(reviewed.SourceHashes, cfg.SourceHashes) {
			t.Fatal("admission receipt does not bind this exact trial")
		}
	}

	self, err := os.Executable()
	if err != nil || secRemBoundExecutable(self, cfg.HarnessSHA256) != nil {
		t.Fatal("harness binary differs from registration")
	}
	if len(cfg.SourceHashes) == 0 {
		t.Fatal("source file bindings required")
	}
	for path, want := range cfg.SourceHashes {
		if !filepath.IsAbs(path) || secRemBoundExecutable(path, want) != nil {
			t.Fatal("source file binding changed")
		}
	}
	var fixtureFiles map[string]string
	if json.Unmarshal(manifest, &fixtureFiles) != nil || len(fixtureFiles) == 0 {
		t.Fatal("invalid fixture inventory")
	}
	for rel, want := range fixtureFiles {
		if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			t.Fatal("unsafe fixture path")
		}
		path := filepath.Join(filepath.Dir(filepath.Dir(cfg.FixtureManifestPath)), rel)
		if secRemBoundExecutable(path, want) != nil {
			t.Fatal("frozen fixture bytes changed")
		}
	}

	for _, pair := range [][2]string{{cfg.ProviderExecutable, cfg.ProviderSHA256}, {cfg.CLIPath, cfg.CLISHA256}} {
		if err := secRemBoundExecutable(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := secRemPrivateFile(cfg.CodexSeedConfig, cfg.CodexSeedSHA256); err != nil {
		t.Fatal("task-only Codex seed config required")
	}
	transportBody, err := secRemPrivateFile(cfg.TransportConfig, cfg.TransportConfigSHA256)
	if err != nil {
		t.Fatal("task-only transport config required")
	}
	if mode == "real" {
		var projection map[string]any
		decoder := json.NewDecoder(bytes.NewReader(transportBody))
		decoder.UseNumber()
		if decoder.Decode(&projection) != nil {
			t.Fatal("transport projection invalid")
		}
		delete(projection, "registration_sha256")
		delete(projection, "registration_file")
		canonical, err := json.Marshal(projection)
		if err != nil {
			t.Fatal("transport projection marshal failed")
		}
		hash := sha256.Sum256(canonical)
		if hex.EncodeToString(hash[:]) != cfg.TransportProjectionSHA256 {
			t.Fatal("transport content differs from reviewed projection")
		}
	}
	var actual string
	if testPool == nil || testPool.QueryRow(t.Context(), "SELECT current_database()").Scan(&actual) != nil || actual != cfg.Database {
		t.Fatal("harness requires its exact owned migrated database")
	}
	if mode == "real" {
		t.Setenv("SECREM_REAL_EXECUTION_AUTHORIZED", "1")
	} else {
		t.Setenv("SECREM_REAL_EXECUTION_AUTHORIZED", "")
	}
	return cfg
}

type secRemCodeTrial struct {
	f                                         *secRemPlatformFixture
	project, resource, seed, initial, current string
	creator, creatorToken                     string
	clones                                    map[string]string
	handoffs                                  []map[string]string
	protected                                 map[string]string
	runs                                      []secRemDirectO1Run
	phase                                     int
}

func secRemTrustedGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("trusted initial fixture Git: %s %v", out, err)
	}
	return strings.TrimSpace(string(out))
}

func secRemGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	configRaw, configErr := os.ReadFile(os.Getenv("SECREM_HARNESS_CONFIG"))
	if configErr != nil {
		t.Fatal(configErr)
	}
	var config secRemHarnessConfig
	if json.Unmarshal(configRaw, &config) != nil {
		t.Fatal("task configuration unavailable for contained Git")
	}
	launcher := filepath.Join(config.SourceRoot, ".trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/contained_git_role_handoff.py")
	commandArgs := append([]string{launcher, "inspect", "--repo", repo, "--"}, args...)
	command := exec.Command("python3", commandArgs...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("synthetic Git operation %s: %s %v", args[0], out, err)
	}
	return strings.TrimSpace(string(out))
}

func secRemTestHashes(t *testing.T, repo string) map[string]string {
	t.Helper()
	root, rootErr := filepath.EvalSymlinks(filepath.Join(repo, "tests"))
	if rootErr != nil || root != filepath.Join(repo, "tests") {
		t.Fatal("tests root must be a real current-role directory")
	}
	out := map[string]string{}
	err := filepath.WalkDir(filepath.Join(repo, "tests"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("test symlink refused")
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular test artifact refused")
		}
		if strings.Contains(path, "__pycache__") {
			return nil
		}
		fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		file := os.NewFile(uintptr(fd), path)
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			return fmt.Errorf("test artifact changed kind")
		}
		raw, err := io.ReadAll(io.LimitReader(file, 4<<20))
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if len(raw) >= 4<<20 {
			return fmt.Errorf("test artifact exceeds limit")
		}
		rel, _ := filepath.Rel(repo, path)
		sum := sha256.Sum256(raw)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func newSecRemCodeTrial(t *testing.T, cfg secRemHarnessConfig) *secRemCodeTrial {
	f := newSecRemPlatformFixture(t, cfg)
	kind := "code"
	if cfg.Scenario == "D1" {
		kind = "dependency"
	}
	fixture := filepath.Join(filepath.Dir(filepath.Dir(cfg.FixtureManifestPath)), "fixtures/repos", kind)
	seed := filepath.Join(cfg.OutputDir, "fixture-seed")
	if err := os.CopyFS(seed, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Synthetic Fixture"}, {"config", "user.email", "fixture@example.test"}, {"config", "core.hooksPath", "/dev/null"}, {"add", "."}, {"commit", "-m", "Synthetic fixture baseline"}} {
		secRemTrustedGit(t, seed, args...)
	}
	trial := &secRemCodeTrial{f: f, seed: seed, initial: secRemGit(t, seed, "rev-parse", "HEAD"), clones: map[string]string{}, protected: secRemTestHashes(t, seed)}
	trial.current = trial.initial
	trial.creator = f.owner
	trial.creatorToken = f.ownerToken
	if cfg.Scenario == "C1" {
		docs, err := os.ReadFile(filepath.Join(cfg.SourceRoot, "apps/docs/content/docs/squads.zh.mdx"))
		if err != nil {
			t.Fatal(err)
		}
		start := strings.Index(string(docs), "```text\n# 漏洞验证\n")
		if start < 0 {
			t.Fatal("documented verifier instructions missing")
		}
		body := string(docs)[start+len("```text\n"):]
		end := strings.Index(body, "\n```")
		if end < 0 {
			t.Fatal("documented verifier instructions unclosed")
		}
		body = body[:end]
		var verifier struct {
			ID string `json:"id"`
		}
		f.human(http.MethodPost, "/api/agents", map[string]any{"name": "漏洞验证试点", "description": "Synthetic independent verifier pilot", "instructions": body, "runtime_id": f.runtime, "model": cfg.Model, "thinking_level": cfg.ThinkingLevel, "permission_mode": "private"}, 201, &verifier)
		f.human(http.MethodPost, "/api/squads/"+f.squad.ID+"/members", map[string]any{"member_type": "agent", "member_id": verifier.ID, "role": "漏洞验证"}, 201, nil)
		f.roles[verifier.ID] = "vulnerability-verifier"
	}
	for _, role := range f.roles {
		if role == "security-remediation-lead" {
			continue
		}
		path := filepath.Join(cfg.OutputDir, "roles", role)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		secRemFullClone(t, seed, path)
		secRemTrustedGit(t, path, "config", "user.name", "Synthetic "+role)
		secRemTrustedGit(t, path, "config", "user.email", role+"@example.test")
		secRemTrustedGit(t, path, "config", "core.hooksPath", "/dev/null")
		exclude := "AGENTS.md\n.agent_context/\n.agents/\n.codex/\n.multica/\n.secrem-*\n__pycache__/\n*.pyc\n"
		if err := os.WriteFile(filepath.Join(path, ".git/info/exclude"), []byte(exclude), 0600); err != nil {
			t.Fatal(err)
		}
		trial.clones[role] = path
	}
	var project struct {
		ID string `json:"id"`
	}
	f.human(http.MethodPost, "/api/projects", map[string]any{"title": "Synthetic " + cfg.Scenario + " code acceptance", "execution_squads": []any{}}, 201, &project)
	trial.project = project.ID
	var resource handler.ProjectResourceResponse
	f.human(http.MethodPost, "/api/projects/"+project.ID+"/resources", map[string]any{"resource_type": "local_directory", "resource_ref": map[string]any{"local_path": trial.clones["security-reviewer"], "daemon_id": f.daemonID, "label": "Synthetic isolated role checkout", "execution_mode": "in_place"}}, 201, &resource)
	trial.resource = resource.ID
	if cfg.Scenario == "D1" {
		retained := secRemRetainedTB{t}
		other := f.fx.User(retained, "Independent synthetic security owner", "other-"+f.workspace+"@example.test")
		f.fx.Member(retained, f.workspace, other, "member")
		for id := range f.roles {
			f.human(http.MethodPut, "/api/agents/"+id, map[string]any{"permission_mode": "public_to", "invocation_targets": []map[string]any{{"target_type": "workspace"}}}, 200, nil)
		}
		f.human(http.MethodDelete, "/api/squads/"+f.squad.ID+"/members", map[string]any{"member_type": "member", "member_id": f.owner}, 204, nil)
		f.human(http.MethodPost, "/api/squads/"+f.squad.ID+"/members", map[string]any{"member_type": "member", "member_id": other, "role": "安全负责人"}, 201, nil)
		f.owner = other
		token, err := generateTestJWT(other, "other-"+f.workspace+"@example.test", "Independent synthetic security owner")
		if err != nil {
			t.Fatal(err)
		}
		f.ownerToken = token
	}
	return trial
}

func (x *secRemCodeTrial) synchronize(t *testing.T, role string) string {
	t.Helper()
	repo := x.clones[role]
	if repo == "" {
		t.Fatalf("no isolated checkout for %s", role)
	}
	for _, handoff := range x.handoffs {
		head := secRemGit(t, repo, "rev-parse", "HEAD")
		if head == handoff["head"] {
			continue
		}
		if head != handoff["base"] {
			continue
		}
		command := exec.Command("python3", filepath.Join(x.f.cfg.SourceRoot, ".trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/contained_git_role_handoff.py"), "import", "--repo", repo, "--handoff", handoff["path"], "--manifest-sha256", handoff["sha"])
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("actual artifact import refused %s %v", out, err)
		}
	}
	if secRemGit(t, repo, "rev-parse", "HEAD") != x.current {
		t.Fatal("role checkout did not receive exact current artifact")
	}
	return repo
}

func (x *secRemCodeTrial) nextRole(t *testing.T) string {
	t.Helper()
	var agent string
	x.f.fx.QueryRow(t, `SELECT agent_id::text FROM agent_task_queue WHERE issue_id=$1 AND status='queued' ORDER BY priority DESC,created_at,id LIMIT 1`, x.f.issue).Scan(&agent)
	role := x.f.roles[agent]
	if role == "" {
		t.Fatal("next actual queue agent not in roster")
	}
	return role
}

func (x *secRemCodeTrial) run(t *testing.T, label, role, mode string, leader bool) secRemDirectO1Run {
	t.Helper()
	f := x.f
	beforeQueue := secRemDirectO1TaskCount(t, f)
	var repo string
	if !leader {
		repo = x.synchronize(t, role)
		actualOwner, actualToken := f.owner, f.ownerToken
		f.owner, f.ownerToken = x.creator, x.creatorToken
		f.human(http.MethodPut, "/api/projects/"+x.project+"/resources/"+x.resource, map[string]any{"resource_ref": map[string]any{"local_path": repo, "daemon_id": f.daemonID, "label": "Synthetic " + role + " checkout", "execution_mode": "in_place"}}, 200, nil)
		f.owner, f.ownerToken = actualOwner, actualToken
		if secRemDirectO1TaskCount(t, f) != beforeQueue {
			t.Fatal("human role resource switch queued work")
		}
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET last_seen_at=clock_timestamp(),status='online' WHERE id=$1`, f.runtime)
	task, err := f.driver.Claim(t.Context())
	if err != nil || task == nil || task.IssueID != f.issue || f.roles[task.AgentID] != role || task.IsLeaderTask != leader || !strings.HasPrefix(task.AuthToken, "mat_") {
		t.Fatalf("%s claim mismatch claimed=%t err=%v", label, task != nil, err)
	}
	if task.Agent == nil || task.Agent.Model != f.cfg.Model || task.Agent.ThinkingLevel != f.cfg.ThinkingLevel {
		t.Fatal("claim model/effort differs")
	}
	if !leader {
		matched := 0
		for _, entry := range task.ProjectResources {
			if entry.ResourceType != "local_directory" {
				continue
			}
			var ref struct {
				LocalPath string `json:"local_path"`
				DaemonID  string `json:"daemon_id"`
			}
			if json.Unmarshal(entry.ResourceRef, &ref) != nil {
				t.Fatal("resource decode")
			}
			if ref.DaemonID == f.daemonID {
				matched++
				if ref.LocalPath != repo {
					t.Fatal("claim used different role checkout")
				}
			}
		}
		if matched != 1 {
			t.Fatal("claim needs exact local role resource")
		}
	}
	if role == "security-reviewer" || role == "code-reviewer" || role == "migration-reviewer" {
		var raw []byte
		f.fx.QueryRow(t, `SELECT api_capability FROM task_token WHERE task_id=$1 AND token_hash=$2`, task.ID, auth.HashToken(task.AuthToken)).Scan(&raw)
		capability, err := auth.ParseTaskCapability(raw)
		if err != nil || capability.Profile != auth.TaskCapabilityReviewer || capability.IssueID != f.issue {
			t.Fatal("reviewer not server restricted")
		}
	}
	manifest := map[string]any{"task_id": task.ID, "issue_id": f.issue, "squad_id": f.squad.ID, "agent_id": task.AgentID, "workspace_id": f.workspace, "role": role, "role_mode": mode, "prepared_repo_count": 0, "scenario": f.cfg.Scenario, "phase": label, "is_leader_task": leader, "owner_ids": []string{f.owner}, "read_only_code": mode != "implementer", "no_repository": leader, "leader_ignores_project_resources": leader, "repo_count": len(task.Repos), "project_resource_count": len(task.ProjectResources), "issue_identifier": task.IssueIdentifier, "trigger_comment_id": task.TriggerCommentID, "trigger_thread_id": task.TriggerThreadID, "resume_session_id": task.PriorSessionID, "execution_mode": "official-cli-direct", "project_id": x.project, "baseline_commit": x.current, "protected_test_dir": "tests", "protected_test_sha256": x.protected}
	if leader {
		ignored := []string{}
		for _, entry := range task.ProjectResources {
			if entry.ResourceType != "local_directory" {
				continue
			}
			var ref struct {
				LocalPath string `json:"local_path"`
				DaemonID  string `json:"daemon_id"`
			}
			if json.Unmarshal(entry.ResourceRef, &ref) != nil {
				t.Fatal("leader project resource decode failed")
			}
			if ref.DaemonID == f.daemonID {
				ignored = append(ignored, ref.LocalPath)
			}
		}
		manifest["ignored_project_paths"] = ignored
	}
	if !leader {
		manifest["repo_root"] = repo
		manifest["prepared_repo_count"] = 1
	} else {
		manifest["role_mode"] = "leader_scratch"
	}
	if err := secRemWriteJSON(filepath.Join(f.cfg.ClaimManifestDir, task.ID+".json"), manifest); err != nil {
		t.Fatal(err)
	}
	copyTask := *task
	copyTask.AuthToken = "[credential]"
	copyTask.RemoteMCPDaemonToken = ""
	if err := secRemWriteJSON(filepath.Join(f.cfg.OutputDir, label+"-claim.json"), copyTask); err != nil {
		t.Fatal(err)
	}
	result, runErr := f.driver.Run(t.Context(), *task)
	if err := secRemWriteJSON(filepath.Join(f.cfg.OutputDir, label+"-result.json"), map[string]any{"result": result, "run_error": fmt.Sprint(runErr)}); err != nil {
		t.Fatal(err)
	}
	if runErr == nil {
		f.driver.Report(t.Context(), *task, result)
	}
	run := secRemDirectO1Run{TaskID: task.ID, Role: role, Leader: leader, Result: result, Snapshot: f.snapshot(label + "-after")}
	x.runs = append(x.runs, run)
	if runErr != nil || result.Status != "completed" {
		secRemWriteJSON(filepath.Join(f.cfg.OutputDir, "code-verdict.json"), map[string]any{"scenario": f.cfg.Scenario, "verdict": "incomplete", "failed_step": label, "status": result.Status})
		t.Fatalf("%s incomplete %v %s", label, runErr, result.Status)
	}
	if !leader {
		after := secRemGit(t, repo, "rev-parse", "HEAD")
		if mode == "verifier" || mode == "implementer" {
			if after == x.current {
				t.Fatal("writer produced no actual commit")
			}
			x.export(t, repo, after, role, task.ID, mode)
		} else if after != x.current || secRemGit(t, repo, "status", "--porcelain", "--untracked-files=all") != "" {
			t.Fatal("read-only role changed repository")
		}
	}
	return run
}

func (x *secRemCodeTrial) export(t *testing.T, repo, head, role, task, mode string) {
	t.Helper()
	out := filepath.Join(x.f.cfg.OutputDir, "handoffs", fmt.Sprintf("%02d-%s", len(x.handoffs)+1, role))
	if err := os.MkdirAll(filepath.Dir(out), 0700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(x.f.cfg.OutputDir, fmt.Sprintf("protected-%02d.json", len(x.handoffs)))
	if err := secRemWriteJSON(protected, x.protected); err != nil {
		t.Fatal(err)
	}
	allowed := []string{"tests"}
	if mode == "implementer" {
		if x.f.cfg.Scenario == "C1" {
			allowed = []string{"workspace_reader.py"}
		} else {
			allowed = []string{"application.py", "dependency.lock.json"}
		}
	}
	args := []string{filepath.Join(x.f.cfg.SourceRoot, ".trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts/contained_git_role_handoff.py"), "export", "--repo", repo, "--base", x.current, "--head", head, "--output", out, "--actor-id", x.agentID(role), "--task-id", task, "--role", role, "--protected-json", protected}
	for _, path := range allowed {
		args = append(args, "--allow", path)
	}
	if output, err := exec.Command("python3", args...).CombinedOutput(); err != nil {
		t.Fatalf("actual role artifact refused: %s %v", output, err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	x.handoffs = append(x.handoffs, map[string]string{"base": x.current, "head": head, "path": out, "sha": hex.EncodeToString(sum[:])})
	x.current = head
	if mode == "verifier" {
		x.protected = secRemTestHashes(t, repo)
	}
}

func (x *secRemCodeTrial) agentID(role string) string {
	for id, key := range x.f.roles {
		if key == role {
			return id
		}
	}
	return ""
}

func (x *secRemCodeTrial) humanGate(t *testing.T, gate string, reviews []handler.CommentResponse, decision string, mention bool) {
	t.Helper()
	f := x.f
	if mention {
		decision = "[@本小队](mention://squad/" + f.squad.ID + ") " + decision
	}
	bindings := []map[string]string{}
	latest := time.Time{}
	for _, review := range reviews {
		sum := sha256.Sum256([]byte(review.Content))
		bindings = append(bindings, map[string]string{"comment_id": review.ID, "author_id": review.AuthorID, "source_task_id": secRemSourceTask(review.SourceTaskID), "content_sha256": hex.EncodeToString(sum[:])})
		at, err := time.Parse(time.RFC3339, review.CreatedAt)
		if err != nil {
			t.Fatal(err)
		}
		if at.After(latest) {
			latest = at
		}
	}
	decisionHash := sha256.Sum256([]byte(decision))
	request := map[string]any{"scenario": f.cfg.Scenario, "gate": gate, "issue_id": f.issue, "owner_id": f.owner, "reviews": bindings, "decision": decision, "decision_sha256": hex.EncodeToString(decisionHash[:]), "code_commit": x.current}
	pending := filepath.Join(f.cfg.OutputDir, "human-"+gate+"-pending.json")
	if err := secRemWriteJSON(pending, request); err != nil {
		t.Fatal(err)
	}
	if f.cfg.Mode == "real" {
		permitPath := filepath.Join(f.cfg.OutputDir, "human-"+gate+"-approved.json")
		for {
			if info, err := os.Lstat(permitPath); err == nil {
				if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
					t.Fatal("human decision permit must be private regular file")
				}
				raw, err := os.ReadFile(permitPath)
				if err != nil {
					t.Fatal(err)
				}
				var permit map[string]any
				if json.Unmarshal(raw, &permit) != nil || permit["approved"] != true {
					t.Fatal("human decision not approved")
				}
				for _, key := range []string{"scenario", "gate", "issue_id", "owner_id", "decision_sha256", "code_commit"} {
					if !reflect.DeepEqual(permit[key], request[key]) {
						t.Fatal("human permit changed decision binding")
					}
				}
				rawRefs, _ := json.Marshal(bindings)
				var canonical any
				json.Unmarshal(rawRefs, &canonical)
				if !reflect.DeepEqual(permit["reviews"], canonical) {
					t.Fatal("human permit changed reviewer binding")
				}
				break
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			select {
			case <-time.After(500 * time.Millisecond):
			case <-t.Context().Done():
				t.Fatal("human decision interrupted")
			}
		}
	}
	if delay := time.Until(latest.Add(2 * time.Second)); delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal("human ordering wait interrupted")
		}
	}
	var release handler.CommentResponse
	f.human(http.MethodPost, "/api/issues/"+f.issue+"/comments", map[string]any{"content": decision}, 201, &release)
	if release.AuthorType != "member" || release.AuthorID != f.owner || release.Revision != 1 {
		t.Fatal("actual owner release identity/revision invalid")
	}
	if err := secRemWriteJSON(filepath.Join(f.cfg.OutputDir, "human-"+gate+"-release.json"), release); err != nil {
		t.Fatal(err)
	}
}

func (x *secRemCodeTrial) reviewBatch(t *testing.T, gate string, want []string) []handler.CommentResponse {
	t.Helper()
	remaining := map[string]bool{}
	for _, role := range want {
		remaining[role] = true
	}
	results := []handler.CommentResponse{}
	for len(remaining) > 0 {
		role := x.nextRole(t)
		if !remaining[role] {
			t.Fatalf("%s dispatched unexpected reviewer %s", gate, role)
		}
		before := secRemDirectO1TaskCount(t, x.f)
		run := x.run(t, gate+"-"+role, role, "reviewer", false)
		comment := secRemDirectO1Comment(t, x.f, run.TaskID)
		if comment.Type != "comment" || comment.AuthorType != "agent" || comment.AuthorID != x.agentID(role) || secRemSourceTask(comment.SourceTaskID) != run.TaskID {
			t.Fatal("review result provenance invalid")
		}
		if secRemDirectO1TaskCount(t, x.f) != before {
			t.Fatal("restricted reviewer result dispatched work")
		}
		results = append(results, comment)
		delete(remaining, role)
	}
	return results
}

func secRemRunCodeRoute(t *testing.T, mode string) {
	secRemAuthorizeDirectCLI(t, mode)
	cfg := secRemLoadCodeHarness(t, mode)
	x := newSecRemCodeTrial(t, cfg)
	f := x.f
	base := filepath.Dir(filepath.Dir(cfg.FixtureManifestPath))
	raw, err := os.ReadFile(filepath.Join(base, "fixtures/expected-actions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Cases []struct {
			ID    string `json:"id"`
			Issue string `json:"issue"`
		} `json:"cases"`
	}
	if json.Unmarshal(raw, &expected) != nil {
		t.Fatal("invalid frozen expectations")
	}
	prefix := "C1-"
	if cfg.Scenario == "D1" {
		prefix = "D1-"
	}
	description := ""
	for _, c := range expected.Cases {
		if strings.HasPrefix(c.ID, prefix) {
			description = c.Issue
		}
	}
	if description == "" {
		t.Fatal("frozen code issue missing")
	}
	description += "\n\n本任务只使用隔离的本地合成 Git 仓库，初始基线提交：" + x.initial + "。每位执行成员领取时工作目录是当前已交接的独立完整副本；角色交接保留原提交和作者。不得推送外部仓库。"
	if err := secRemWriteJSON(filepath.Join(cfg.OutputDir, "code-input.json"), map[string]any{"scenario": cfg.Scenario, "issue_description": description, "baseline_commit": x.initial, "fixture_manifest_sha256": cfg.FixtureManifestSHA256}); err != nil {
		t.Fatal(err)
	}
	var issue handler.IssueResponse
	f.human(http.MethodPost, "/api/issues", map[string]any{"title": "合成安全修复 " + cfg.Scenario, "description": description, "project_id": x.project, "status": "todo", "assignee_type": "squad", "assignee_id": f.squad.ID}, 201, &issue)
	f.issue = issue.ID
	f.before = f.snapshot("before")
	x.run(t, "intake", "security-remediation-lead", "leader", true)
	reviewers := []string{"security-reviewer"}
	if cfg.Scenario == "D1" {
		reviewers = append(reviewers, "migration-reviewer")
	}
	g1 := x.reviewBatch(t, "G1", reviewers)
	decision := "已读本轮全部审查结果。G1逐项结论：定级high；处置修复；路由bug-fix；期限2026-12-20；知情范围need-to-know。只在本地合成仓库执行最小修复，不访问生产、真实凭据或第三方系统。"
	if cfg.Scenario == "C1" {
		decision += "类型code；由漏洞验证成员独立复现，先交付失败测试及同类写法清单，之后由实现工程师修复；补丁作者不得改复现测试。"
	} else {
		decision += "类型dependency；复现not-applicable，以本地锁文件、调用可达性和合成扫描器证据为准，保持应用返回str的兼容契约。"
	}
	x.humanGate(t, "G1", g1, decision, cfg.Scenario == "C1")
	x.run(t, "G1-release-leader", "security-remediation-lead", "leader", true)
	if cfg.Scenario == "C1" {
		x.run(t, "reproduction", "vulnerability-verifier", "verifier", false)
		x.run(t, "reproduction-return-leader", "security-remediation-lead", "leader", true)
	}
	x.run(t, "repair", "implementer", "implementer", false)
	x.run(t, "repair-return-leader", "security-remediation-lead", "leader", true)
	if cfg.Scenario == "C1" {
		x.run(t, "verifier-rerun", "vulnerability-verifier", "verifier_recheck", false)
		x.run(t, "verification-return-leader", "security-remediation-lead", "leader", true)
	}
	x.run(t, "regression", "qa-engineer", "qa", false)
	x.run(t, "regression-return-leader", "security-remediation-lead", "leader", true)
	g2reviewers := []string{"code-reviewer", "security-reviewer"}
	if cfg.Scenario == "D1" {
		g2reviewers = append(g2reviewers, "migration-reviewer")
	}
	g2 := x.reviewBatch(t, "G2", g2reviewers)
	decision = "已读本轮全部针对修复提交 " + x.current + " 的复审结果。G2逐项结论：接受本次修复与独立验证；确认变体清单完整；回移范围none，仅本合成基线。风险判断仅限本次测试与已记录证据，不作生产安全保证。"
	if cfg.Scenario == "C1" {
		decision += "确认read_summary为另一个需处理的同类变体，由队长单独建一个不指派、backlog、无父任务的任务，不并入当前补丁，由人工排期。"
	} else {
		decision += "确认依赖使用点清单，未发现另需排期的变体；合成扫描前后与间接依赖变化按原始输出记录。"
	}
	x.humanGate(t, "G2", g2, decision, cfg.Scenario == "D1")
	x.run(t, "closing", "security-remediation-lead", "leader", true)
	x.finish(t)
}

func secRemCodeRosterUnchanged(before, after json.RawMessage) bool {
	var a, b []map[string]any
	if json.Unmarshal(before, &a) != nil || json.Unmarshal(after, &b) != nil {
		return false
	}
	clean := func(rows []map[string]any) map[string]map[string]any {
		out := map[string]map[string]any{}
		for _, row := range rows {
			delete(row, "updated_at")
			delete(row, "status")
			out[fmt.Sprint(row["id"])] = row
		}
		return out
	}
	return reflect.DeepEqual(clean(a), clean(b))
}

func (x *secRemCodeTrial) finish(t *testing.T) {
	t.Helper()
	f := x.f
	f.after = f.snapshot("final")
	if err := secRemDirectO1Diff(t, f); err != nil {
		t.Fatal(err)
	}
	resources := f.rows(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]') FROM project_resource r WHERE workspace_id=$1`, f.workspace)
	if err := secRemWriteJSON(filepath.Join(f.cfg.OutputDir, "code-resources-final.json"), resources); err != nil {
		t.Fatal(err)
	}
	var status string
	f.fx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.issue).Scan(&status)
	var tasks, pending, variants, badVariants, variantRuns int
	f.fx.QueryRow(t, `SELECT count(*),count(*) FILTER(WHERE t.status<>'completed') FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id=$1`, f.workspace).Scan(&tasks, &pending)
	f.fx.QueryRow(t, `SELECT count(*),count(*) FILTER(WHERE status<>'backlog' OR assignee_id IS NOT NULL OR parent_issue_id IS NOT NULL OR COALESCE(assignee_type,'')<>'') FROM issue WHERE workspace_id=$1 AND id<>$2`, f.workspace, f.issue).Scan(&variants, &badVariants)
	f.fx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1 AND id<>$2)`, f.workspace, f.issue).Scan(&variantRuns)
	var provenanceBad int
	f.fx.QueryRow(t, `SELECT count(*) FROM comment c LEFT JOIN agent_task_queue q ON q.id=c.source_task_id WHERE c.workspace_id=$1 AND c.author_type='agent' AND c.type='comment' AND (q.id IS NULL OR c.author_id<>q.agent_id OR c.issue_id<>q.issue_id)`, f.workspace).Scan(&provenanceBad)
	var offIssueEffects int
	f.fx.QueryRow(t, `SELECT count(*) FROM comment WHERE workspace_id=$1 AND issue_id<>$2`, f.workspace, f.issue).Scan(&offIssueEffects)
	wantTasks, wantVariants := 14, 1
	if f.cfg.Scenario == "D1" {
		wantTasks, wantVariants = 12, 0
	}
	criteria := map[string]bool{"comment_actor_source_binding": provenanceBad == 0, "no_off_issue_comments": offIssueEffects == 0, "agent_config_unchanged": secRemCodeRosterUnchanged(f.before["workspace_agents"], f.after["workspace_agents"]), "squad_unchanged": reflect.DeepEqual(f.before["workspace_squads"], f.after["workspace_squads"]), "members_unchanged": reflect.DeepEqual(f.before["workspace_members"], f.after["workspace_members"]), "exact_role_run_count": len(x.runs) == wantTasks && tasks == wantTasks, "all_actual_tasks_complete": pending == 0, "issue_in_review_not_done": status == "in_review", "expected_unassigned_backlog_variants": variants == wantVariants && badVariants == 0 && variantRuns == 0, "fix_commit_differs_from_baseline": x.current != x.initial}
	if f.cfg.Scenario == "C1" {
		criteria["separate_test_and_patch_commits"] = len(x.handoffs) == 2 && x.handoffs[0]["head"] == x.handoffs[1]["base"]
	} else {
		criteria["one_dependency_patch_artifact"] = len(x.handoffs) == 1
	}
	verdict := "platform-checks-passed-semantic-review-pending"
	for _, pass := range criteria {
		if !pass {
			verdict = "disproved-in-this-sample"
		}
	}
	if err := secRemWriteJSON(filepath.Join(f.cfg.OutputDir, "code-verdict.json"), map[string]any{"scenario": f.cfg.Scenario, "execution_mode": "official-cli-direct", "evidence_type": f.cfg.Mode, "verdict": verdict, "criteria": criteria, "initial_commit": x.initial, "fix_commit": x.current, "handoffs": x.handoffs, "protected_tests": x.protected, "manual_review_required": []string{"actual failure before patch and success after patch, exact test bytes", "reviewer/human identity, timestamps, commit references and complete decisions", "no forbidden operations or altered test boundaries", "complete source/role/time evidence and unresolved limitations"}}); err != nil {
		t.Fatal(err)
	}
	if verdict != "platform-checks-passed-semantic-review-pending" {
		t.Fatalf("code route criteria failed %v", criteria)
	}
}

func TestSecurityRemediationDirectC1Fake(t *testing.T) { secRemRunCodeRoute(t, "fake") }
func TestSecurityRemediationDirectC1Real(t *testing.T) { secRemRunCodeRoute(t, "real") }
func TestSecurityRemediationDirectD1Fake(t *testing.T) { secRemRunCodeRoute(t, "fake") }
func TestSecurityRemediationDirectD1Real(t *testing.T) { secRemRunCodeRoute(t, "real") }

func secRemSourceTask(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
