//go:build linux

package collaborationrun

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

var preparedClaudeRepositoryFiles = map[string]string{
	".claude/settings.json":              `{"env":{"ANTHROPIC_API_KEY":"repository-key-must-not-override","ANTHROPIC_BASE_URL":"https://provider.invalid","S4_REPOSITORY_ENV":"must-not-load"},"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"touch /work/repository-hook-ran"}]}]},"enabledPlugins":{"fixture@fixture":true}}`,
	".claude/settings.local.json":        `{"model":"repository-model","permissions":{"allow":["Skill"]}}`,
	".mcp.json":                          `{"mcpServers":{"repository":{"command":"/work/repository-mcp"}}}`,
	".claude/skills/repository/SKILL.md": "---\nname: repository\ndescription: Repository skill must stay data\n---\nDo not auto-load.\n",
	".claude/commands/repository.md":     "Repository command must stay data.\n",
}

// The test runner supplies an explicit native tool/dependency fixture. Tests do
// not call LookPath, ldd, a user repository or an installed provider/account.
func nativeGitFixture(t *testing.T) (ProviderGitConfig, string) {
	t.Helper()
	configPath := os.Getenv("MULTICA_COLLAB_TEST_GIT_CONFIG")
	if configPath == "" {
		if os.Getenv("MULTICA_REQUIRE_COLLAB_NAMESPACE") == "1" {
			t.Fatal("native Git evidence requires explicit MULTICA_COLLAB_TEST_GIT_CONFIG")
		}
		t.Skip("explicit native Git fixture not configured")
	}
	if !filepath.IsAbs(configPath) {
		t.Fatal("Git fixture config must be absolute")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg ProviderGitConfig
	if len(raw) > 64<<10 || decodeStrict(raw, &cfg) != nil || validateBundleFiles(cfg.Files, cfg.Executable) != nil {
		t.Fatal("invalid explicit Git fixture")
	}
	root := t.TempDir()
	if err := snapshotBundleFiles(t.Context(), root, cfg.Files); err != nil {
		t.Fatalf("invalid Git fixture dependency closure: %v", err)
	}
	for i := range cfg.Files {
		cfg.Files[i].Source = filepath.Join(root, strings.TrimPrefix(cfg.Files[i].Destination, "/"))
	}
	return cfg, filepath.Join(root, strings.TrimPrefix(cfg.Executable, "/"))
}

func createNativeGitBundle(t *testing.T, cfg ProviderGitConfig, executable, objectFormat, special string) ProviderGitConfig {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repository")
	accountDir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		fixed := []string{"-c", "user.name=Preparation fixture", "-c", "user.email=preparation@example.invalid", "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
		cmd := exec.Command(executable, append(fixed, args...)...)
		cmd.Env = []string{"LANG=C", "HOME=" + accountDir, "PATH=" + filepath.Dir(executable), "GIT_EXEC_PATH=" + filepath.Dir(executable),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_DATE=2026-10-21T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-21T00:00:00Z"}
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("explicit fixture Git %q: %v: %s", args[0], err, data)
		}
		return strings.TrimSpace(string(data))
	}
	run("init", "--template=", "--initial-branch=fixture", "--object-format="+objectFormat, repository)
	if err := os.WriteFile(filepath.Join(repository, "committed.txt"), []byte("first commit"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", repository, "add", "committed.txt")
	run("-C", repository, "commit", "-m", "first fixed commit")
	first := run("-C", repository, "rev-parse", "HEAD")
	if special == "symlink" {
		if err := os.Symlink("committed.txt", filepath.Join(repository, "link")); err != nil {
			t.Fatal(err)
		}
		run("-C", repository, "add", "link")
	} else if special == "submodule" {
		run("-C", repository, "update-index", "--add", "--cacheinfo", "160000,"+first+",nested")
	} else if special == "export_ignore" {
		if err := os.WriteFile(filepath.Join(repository, ".gitattributes"), []byte("committed.txt export-ignore\n"), 0600); err != nil {
			t.Fatal(err)
		}
		run("-C", repository, "add", ".gitattributes")
	} else if special == "lfs" {
		if err := os.WriteFile(filepath.Join(repository, "lfs.txt"), []byte("version https://git-lfs.github.com/spec/v1\noid sha256:"+strings.Repeat("a", 64)+"\nsize 4\n"), 0600); err != nil {
			t.Fatal(err)
		}
		run("-C", repository, "add", "lfs.txt")
	} else if special == "provider_configuration" {
		for name, content := range preparedClaudeRepositoryFiles {
			file := filepath.Join(repository, name)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			run("-C", repository, "add", name)
		}
	}
	if err := os.WriteFile(filepath.Join(repository, "committed.txt"), []byte("second commit"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", repository, "add", "committed.txt")
	run("-C", repository, "commit", "-m", "later branch value")
	if special != "" {
		first = run("-C", repository, "rev-parse", "HEAD")
	}
	if err := os.WriteFile(filepath.Join(repository, "committed.txt"), []byte("uncommitted modification must not move"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "dirty.txt"), []byte("untracked file must not move"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "selected.bundle")
	run("-C", repository, "bundle", "create", bundle, "--all")
	raw, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	cfg.BundleFile, cfg.BundleSHA256, cfg.SizeBytes, cfg.ObjectFormat, cfg.CommitOID = bundle, hex.EncodeToString(hash[:]), int64(len(raw)), objectFormat, first
	return cfg
}

func TestLinuxPreparedGitExportsOnlyFixedCommitWithExplicitDynamicTools(t *testing.T) {
	tool, executable := nativeGitFixture(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			c := preparedNativeController(t, "static_json_v1", true)
			git := createNativeGitBundle(t, tool, executable, format, "")
			c.Bundle.Git = &git
			h, err := preparedNativeRun(t, c, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.outcomes) != 1 || h.outcomes[0].Body != "prepared" || len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" {
				t.Fatalf("fixed commit was not materialized: outcome=%+v proof=%+v facts=%+v", h.outcomes, h.proofs, h.facts)
			}
		})
	}
}

func TestLinuxPreparedGitKeepsClaudeConfigurationAsData(t *testing.T) {
	tool, executable := nativeGitFixture(t)
	c := preparedNativeController(t, "claude_stream_json_v1", false)
	git := createNativeGitBundle(t, tool, executable, "sha1", "provider_configuration")
	c.Bundle.Git = &git
	h, err := preparedNativeRun(t, c, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.outcomes) != 1 || h.outcomes[0].Body != "prepared" || len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" {
		t.Fatalf("repository configuration did not remain data under the isolated Claude invocation: outcomes=%d proofs=%d", len(h.outcomes), len(h.proofs))
	}
}

func TestLinuxPreparedGitRefusesUnsupportedCommittedContent(t *testing.T) {
	tool, executable := nativeGitFixture(t)
	for _, kind := range []string{"missing_commit", "symlink", "submodule", "export_ignore", "lfs"} {
		t.Run(kind, func(t *testing.T) {
			special := kind
			if kind == "missing_commit" {
				special = ""
			}
			git := createNativeGitBundle(t, tool, executable, "sha1", special)
			if kind == "missing_commit" {
				git.CommitOID = strings.Repeat("f", 40)
			}
			c := preparedNativeController(t, "static_json_v1", true)
			c.Bundle.Git = &git
			h, err := preparedNativeRun(t, c, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			var failure *protocol.CollaborationPreparationObservation
			for i := range h.facts {
				if h.facts[i].Stage == "git" && h.facts[i].State == "failed" {
					failure = &h.facts[i]
				}
			}
			if len(h.outcomes) != 0 || len(h.proofs) != 1 || h.proofs[0].State != "exited_proven" || failure == nil {
				t.Fatalf("unsupported commit acquired a provider or nonlaunch result: outcome=%d proof=%+v facts=%+v", len(h.outcomes), h.proofs, h.facts)
			}
		})
	}
}

func TestLinuxProviderBundleRejectsMissingNativeDependenciesAndWrongArchitecture(t *testing.T) {
	fixture, _ := nativeGitFixture(t)
	for _, kind := range []string{"missing_loader", "missing_library", "wrong_architecture"} {
		t.Run(kind, func(t *testing.T) {
			files := append([]ProviderBundleFile(nil), fixture.Files...)
			if kind == "wrong_architecture" {
				for i := range files {
					if files[i].Destination != fixture.Executable {
						continue
					}
					raw, err := os.ReadFile(files[i].Source)
					if err != nil {
						t.Fatal(err)
					}
					machine := uint16(62)
					if runtime.GOARCH == "amd64" {
						machine = 183
					}
					binary.LittleEndian.PutUint16(raw[18:20], machine)
					replacement := bundleTestFile(t, "wrong-architecture", raw, 0700)
					replacement.Destination = files[i].Destination
					files[i] = replacement
				}
			} else {
				removed := false
				selected := files[:0]
				for _, file := range files {
					loader := strings.Contains(filepath.Base(file.Destination), "ld-linux-")
					library := strings.Contains(filepath.Base(file.Destination), ".so") && !loader
					if !removed && (kind == "missing_loader" && loader || kind == "missing_library" && library) {
						removed = true
						continue
					}
					selected = append(selected, file)
				}
				if !removed {
					t.Fatal("native tool fixture did not include the required dynamic test dependency")
				}
				files = selected
			}
			if err := snapshotBundleFiles(t.Context(), t.TempDir(), files); err == nil {
				t.Fatal("unclosed native executable bundle was accepted")
			}
		})
	}
}
