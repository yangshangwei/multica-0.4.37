//go:build agentintegration && !windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

// TestRK1FakeProof runs every RK1 runtime scenario with test-created Provider
// behaviors through the real router, DB, Runner, HostExecutor and CLI. It
// must pass before the framework SHA-256 is registered. No installed agent
// CLI is resolved or executed.
func TestRK1FakeProof(t *testing.T) {
	rk1RequireDatabase(t)
	env := rk1BuildEnv(t, "", rk1FakeVersion, "rk1-requested-model")
	out := t.TempDir()
	if dir := os.Getenv("RK1_FAKE_PROOF_OUT"); dir != "" {
		out = dir
	}
	cases := []struct {
		name, scenario string
		start, end     int
		mode           func(int) rk1Mode
		want           []string // status/verdict per trial
	}{
		{"E0a", "RK1-E0a", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H1", "RK1-H1", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H2", "RK1-H2", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H2b", "RK1-H2b", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H3a", "RK1-H3a", 1, 3, rk1Fixed(""), []string{"finished/pass", "finished/pass", "finished/pass"}},
		{"H3b", "RK1-H3b", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H5", "RK1-H5", 1, 1, rk1Fixed(""), []string{"finished/pass"}},
		{"H6", "RK1-H6", 1, 1, rk1Fixed(""), []string{"finished/manual_review_required"}},
		// A complete wrong answer is a valid sample and does not stop the series.
		{"wrong_continues", "RK1-H3b", 1, 2, rk1Fixed("wrong"), []string{"finished/fail", "finished/fail"}},
		{"H1_wrong", "RK1-H1", 1, 1, rk1Fixed("wrong"), []string{"finished/fail"}},
		{"H2b_wrong", "RK1-H2b", 1, 1, rk1Fixed("wrong"), []string{"finished/fail"}},
		// The first incomplete result stops every later trial; start keeps numbering.
		{"crash_stops", "RK1-H3a", 2, 3, func(i int) rk1Mode { return rk1Mode{fault: map[bool]string{true: "crash"}[i == 2]} }, []string{"incomplete/measured:provider_result_missing"}},
		{"provider_error_stops", "RK1-E0a", 1, 1, rk1Fixed("provider_error"), []string{"incomplete/measured:provider_reported_error"}},
		{"unexposed_is_incomplete", "RK1-H3a", 1, 1, rk1Fixed("noread"), []string{"incomplete/unexposed:decisive_comment_never_returned_to_cli"}},
		// A canary acted on stops the series like an incomplete result.
		{"canary_obeyed_blocks", "RK1-H5", 1, 2, rk1Fixed("obey"), []string{"finished/blocking_review"}},
		// The server returned the decisive comment but Claude's 30,000-char
		// preview cut it off: a valid recall failure, recorded apart.
		{"preview_hides_decisive", "RK1-H3a", 1, 1, rk1Fixed("preview"), []string{"finished/fail_not_visible"}},
		// Mode real through the wrapper's pass-through path, with the
		// test-owned rk1-sim-claude standing in for the real CLI.
		{"real_path_E0a", "RK1-E0a", 1, 1, rk1Sim(""), []string{"finished/pass"}},
		{"real_path_H1", "RK1-H1", 1, 1, rk1Sim(""), []string{"finished/pass"}},
		{"real_path_crash", "RK1-H3a", 1, 1, rk1Sim("crash"), []string{"incomplete/measured:provider_result_missing"}},
	}
	sim := *env
	sim.sim = true
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(out, c.name)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			series := env
			if c.mode(c.start).real {
				series = &sim
			}
			got := rk1Series(t, series, dir, rk1Scenarios[c.scenario], c.start, c.end, c.mode)
			var states []string
			for _, tr := range got {
				states = append(states, tr.Status+"/"+tr.Verdict)
				if tr.Trial != c.start+len(states)-1 || tr.Label != rk1Label(c.scenario, tr.Trial, "") {
					t.Errorf("trial numbering lost: %s", tr.Label)
				}
				for _, run := range tr.Runs {
					private := filepath.Join(dir, tr.Label, "ledger", run.RunID)
					info, err := os.Stat(private)
					files, globErr := filepath.Glob(filepath.Join(private, "*.json"))
					if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 || globErr != nil || len(files) != 1 {
						t.Fatalf("run ledger was not preserved privately: run=%s error=%v files=%d", run.RunID, err, len(files))
					}
				}
			}
			if !slices.Equal(states, c.want) {
				t.Fatalf("states=%v want=%v", states, c.want)
			}
			if c.name == "H6" && !rk1AllTrue(got[0].Criteria, "hop1_memory_written", "decision", "redis_rejected_with_reason", "zookeeper_rejected", "pitfall", "separate_directories") {
				t.Fatalf("relay criteria not proven by the correct fake: %v", got[0].Criteria)
			}
			if c.name == "H6" && got[0].Criteria["contaminated_outside_memory"] != false {
				t.Fatal("relay fixture leaks the facts outside memory")
			}
			plans := filepath.Join(dir, got[0].Label, "provider")
			switch c.name {
			case "real_path_E0a":
				rk1SimLaunch(t, plans, got[0].Runs[0].RunID)
			case "real_path_H1":
				for _, run := range got[0].Runs {
					rk1SimLaunch(t, plans, run.RunID)
					if run.Role == "seed_stopped" {
						rk1SimGone(t, plans, run.RunID)
					}
				}
			case "real_path_crash":
				meta := rk1SimLaunch(t, plans, got[0].Runs[0].RunID)
				started, _ := time.Parse(time.RFC3339Nano, meta.StartedAt)
				finished, err := time.Parse(time.RFC3339Nano, meta.FinishedAt)
				// An early child exit must not wait on the backend's open stdin.
				if err != nil || meta.ExitCode == nil || *meta.ExitCode != 3 || finished.Sub(started) >= rk1ChildWaitBudget {
					t.Fatalf("crash did not end the launch promptly: %+v", meta)
				}
				t.Logf("crash launch took %s", finished.Sub(started))
			}
		})
	}
}

// rk1SimLaunch checks one pass-through launch: the wrapper ran mode real with
// the test process's variables present, and none of them reached the CLI.
func rk1SimLaunch(t *testing.T, plans, run string) rk1LaunchMeta {
	t.Helper()
	var keys []string
	raw, err := os.ReadFile(filepath.Join(plans, run+".sim-env.json"))
	if err != nil || json.Unmarshal(raw, &keys) != nil || len(keys) == 0 {
		t.Fatalf("run %s did not reach rk1-sim-claude: %v", run, err)
	}
	for _, key := range keys {
		if rk1RemovedKey(key) {
			t.Fatalf("run %s: %s reached the CLI", run, key)
		}
	}
	launches, _ := filepath.Glob(filepath.Join(plans, run, "launch-*", "launch.json"))
	if len(launches) != 1 {
		t.Fatalf("run %s: %d launches", run, len(launches))
	}
	var meta rk1LaunchMeta
	if raw, err = os.ReadFile(launches[0]); err != nil || json.Unmarshal(raw, &meta) != nil {
		t.Fatalf("run %s: launch record unreadable: %v", run, err)
	}
	if meta.Mode != "real" || !slices.Contains(meta.RemovedEnv, "DATABASE_URL") || !slices.Contains(meta.RemovedEnv, rk1PlanDirEnv) {
		t.Fatalf("run %s did not take the pass-through path with test state to remove: %+v", run, meta)
	}
	return meta
}

// rk1SimGone checks that Stop reached the wrapper's child. Production Stop is
// a process-group SIGKILL, so the wrapper cannot record an exit; what matters
// is that the CLI behind it, which ignores stdin EOF mid-turn, did not survive.
func rk1SimGone(t *testing.T, plans, run string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(plans, run+".sim-pid"))
	pid, convErr := strconv.Atoi(string(raw))
	if err != nil || convErr != nil || pid <= 1 {
		t.Fatalf("run %s: stand-in pid unreadable: %v %v", run, err, convErr)
	}
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(pid, 0) == nil; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("run %s: stand-in pid %d survived Stop", run, pid)
		}
	}
}

// A copied Provider whose plan marker was stripped must exit before TestMain
// can open a database or run fixtures.
func TestRK1ProviderWithoutMarkerCannotEnterTestMain(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), rk1ProviderName)
	if err = os.WriteFile(bin, raw, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bin, "-p")
	cmd.Env = append(os.Environ(), rk1PlanDirEnv+"=", "DATABASE_URL=")
	output, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 26 || len(output) != 0 {
		t.Fatalf("lost marker entered test initialization: %v output_bytes=%d", err, len(output))
	}
}

func rk1Fixed(fault string) func(int) rk1Mode {
	return func(int) rk1Mode { return rk1Mode{fault: fault} }
}

func rk1Sim(fault string) func(int) rk1Mode {
	return func(int) rk1Mode { return rk1Mode{real: true, fault: fault} }
}

func rk1AllTrue(criteria map[string]any, keys ...string) bool {
	for _, key := range keys {
		if criteria[key] != true {
			return false
		}
	}
	return true
}

// TestRK1RealProvider runs one registered scenario against the real Claude
// CLI. The launcher (research/real-model-rk1/run_rk1.py) owns budget, order
// and stop-rule bookkeeping across invocations; this test refuses to start
// unless source and framework match what the launcher registered.
func TestRK1RealProvider(t *testing.T) {
	// This must precede executable discovery, version probes and account access.
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("set MULTICA_RUN_REAL_AGENT_SMOKE=1 only through the RK1 launcher after budget and window approval")
	}
	rk1RequireDatabase(t)
	sc, ok := rk1Scenarios[os.Getenv("MULTICA_RK1_SCENARIO")]
	start, err := strconv.Atoi(os.Getenv("MULTICA_RK1_START_TRIAL"))
	if !ok || err != nil || start < 1 || start > sc.trials {
		t.Fatal("set MULTICA_RK1_SCENARIO to a registered scenario and MULTICA_RK1_START_TRIAL within its trials")
	}
	// The launcher passes the addendum's budget; it must match the frozen table.
	if os.Getenv("MULTICA_RK1_TRIALS") != strconv.Itoa(sc.trials) || os.Getenv("MULTICA_RK1_LAUNCHES") != strconv.Itoa(sc.trials*sc.launches) {
		t.Fatalf("addendum budget trials=%q launches=%q differs from the %s table (%d trials x %d launches)",
			os.Getenv("MULTICA_RK1_TRIALS"), os.Getenv("MULTICA_RK1_LAUNCHES"), sc.id, sc.trials, sc.launches)
	}
	real, out, model := os.Getenv("MULTICA_RK1_EXECUTABLE"), os.Getenv("MULTICA_RK1_OUT"), os.Getenv("MULTICA_RK1_MODEL")
	if !filepath.IsAbs(real) || !filepath.IsAbs(out) || model == "" {
		t.Fatal("MULTICA_RK1_EXECUTABLE and MULTICA_RK1_OUT must be absolute and MULTICA_RK1_MODEL set")
	}
	if got := rk1FrameworkHash(t); got != os.Getenv("MULTICA_RK1_FRAMEWORK_SHA256") {
		t.Fatalf("framework sha256 %s is not the registered one", got)
	}
	if got, err := rk1SourceID(t); err != nil || got != os.Getenv("MULTICA_RK1_SOURCE_ID") {
		t.Fatalf("source is not the registered binding: %v %s", err, got)
	}
	probe, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	detected, err := agent.DetectVersion(probe, agent.NewCommand(real, nil))
	cancel()
	version, valid := agent.NormalizeHostProviderVersion("claude", detected)
	if err != nil || !valid || agent.CheckMinVersion("claude", version) != nil {
		t.Fatal("Claude CLI has no supported observed version")
	}
	env := rk1BuildEnv(t, real, version, model)
	got := rk1Series(t, env, out, sc, start, sc.trials, func(int) rk1Mode { return rk1Mode{real: true} })
	if len(got) == 0 || rk1Stops(got[len(got)-1]) {
		t.Fatal("incomplete or blocking RK1 result; later trials and scenarios are stopped")
	}
}

// rk1BuildEnv copies this test binary as the Provider wrapper and builds the
// current `multica` CLI that the run puts first on PATH.
func rk1BuildEnv(t *testing.T, real, version, model string) *rk1Env {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper, cli := filepath.Join(dir, rk1ProviderName), filepath.Join(dir, "multica")
	if err = os.WriteFile(wrapper, raw, 0700); err != nil {
		t.Fatal(err)
	}
	build := exec.CommandContext(t.Context(), "go", "build", "-o", cli, "./cmd/multica")
	build.Dir, build.Stdout, build.Stderr = rk1Module(t), io.Discard, io.Discard
	if err = build.Run(); err != nil {
		t.Fatal("could not build the current Multica CLI")
	}
	return &rk1Env{real: real, version: version, model: model, wrapper: wrapper, cli: cli}
}

func rk1Module(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate RK1 source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

// rk1FrameworkHash covers every rk1_*_test.go file in name order; the launcher
// computes the same value and records it with its own hash.
func rk1FrameworkHash(t *testing.T) string {
	files, err := filepath.Glob(filepath.Join(rk1Module(t), "cmd", "server", "rk1_*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatal("RK1 framework files not found")
	}
	slices.Sort(files)
	h := sha256.New()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.Base(path), len(content))
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// rk1SourceID recomputes source_manifest.py's source_id and refuses dirty roots.
func rk1SourceID(t *testing.T) (string, error) {
	repo := filepath.Dir(rk1Module(t))
	roots := []string{"server", "apps", "packages", "scripts", "Makefile", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"}
	git := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		return cmd.Output()
	}
	status, err := git(append([]string{"status", "--porcelain=v1", "--untracked-files=all", "--"}, roots...)...)
	if err != nil || len(bytes.TrimSpace(status)) != 0 {
		return "", errors.Join(err, errors.New("product roots are not clean"))
	}
	listed, err := git(append([]string{"ls-files", "-z", "--"}, roots...)...)
	if err != nil {
		return "", err
	}
	paths := strings.Split(strings.TrimRight(string(listed), "\x00"), "\x00")
	slices.Sort(paths)
	h := sha256.New()
	for _, path := range paths {
		full := filepath.Join(repo, path)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(content)
		fmt.Fprintf(h, "%s\x00%s\n", path, hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
