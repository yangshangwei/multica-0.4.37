package collaborationrun

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func hostGitFixture(t *testing.T, unsupported bool) *HostGitConfig {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("native Git tool is not installed")
	}
	git, err = filepath.Abs(git)
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(t.TempDir(), "source")
	run := func(args ...string) string {
		t.Helper()
		fixed := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "user.name=Host input fixture", "-c", "user.email=fixture@example.invalid", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
		cmd := exec.Command(git, append(fixed, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("test Git %s: %v: %s", args[0], err, data)
		}
		return strings.TrimSpace(string(data))
	}
	run("init", "--template=", "--initial-branch=fixture", repository)
	if err := os.WriteFile(filepath.Join(repository, "committed.txt"), []byte("selected commit"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", repository, "add", "committed.txt")
	run("-C", repository, "commit", "-m", "selected fixture")
	commit := run("-C", repository, "rev-parse", "HEAD")
	if unsupported {
		if err := os.WriteFile(filepath.Join(repository, ".gitattributes"), []byte("committed.txt export-ignore\n"), 0600); err != nil {
			t.Fatal(err)
		}
		run("-C", repository, "add", ".gitattributes")
		run("-C", repository, "commit", "-m", "unsupported fixture")
		commit = run("-C", repository, "rev-parse", "HEAD")
	}
	if err := os.WriteFile(filepath.Join(repository, "committed.txt"), []byte("private uncommitted change"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "private.txt"), []byte("private untracked content"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "source.bundle")
	run("-C", repository, "bundle", "create", bundle, "--all")
	raw, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return &HostGitConfig{Executable: git, BundleFile: bundle, BundleDigest: hex.EncodeToString(sum[:]), SizeBytes: int64(len(raw)), ObjectFormat: "sha1", CommitOID: commit}
}

func TestHostGitPreparationRegistersEachHelperAndOnlyDeliversSelectedCommit(t *testing.T) {
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "fixed_commit", true: "unsupported_attributes"}[unsupported], func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			f.runner.Host.Config.Git = hostGitFixture(t, unsupported)
			caps, err := f.runner.Host.Config.Capabilities()
			if err != nil {
				t.Fatal(err)
			}
			digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
			if err != nil {
				t.Fatal(err)
			}
			f.request.CapabilityDigest = digest
			f.claim.Preparation.Host, f.claim.Preparation.Git, f.claim.Preparation.CapabilityDigest = caps.Host, caps.Git, digest
			if err := protocol.SealCollaborationPreparation(f.claim.Preparation); err != nil {
				t.Fatal(err)
			}
			err = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
			if unsupported {
				if err == nil || errors.Is(err, ErrUnconfirmed) {
					t.Fatalf("unsupported committed input must fail with known helper termination: %v", err)
				}
				if len(f.starts(t)) != 0 {
					t.Fatal("provider launched despite invalid frozen Git input")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			starts := f.starts(t)
			if len(starts) != 1 {
				t.Fatal("provider did not launch exactly once")
			}
			work := starts[0]["cwd"].(string)
			data, err := os.ReadFile(filepath.Join(work, "committed.txt"))
			if err != nil || string(data) != "selected commit" {
				t.Fatalf("selected input was replaced with private dirty content: %v", err)
			}
			if _, err := os.Stat(filepath.Join(work, "private.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("private untracked file was copied")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.permits) != 7 || len(f.terminations) != 7 || len(f.proofs) != 1 || f.proofs[0].SealedAttemptSeq != "7" {
				t.Fatalf("Git helpers were outside launch accounting: permits=%d terminations=%d", len(f.permits), len(f.terminations))
			}
			for seq, permit := range f.permits {
				if seq != "7" && permit.LaunchDescriptor.Kind != "preparation" {
					t.Fatal("Git helper was disguised as a provider attempt")
				}
			}
		})
	}
}
