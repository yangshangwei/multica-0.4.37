package collaborationrun

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func bundleTestFile(t *testing.T, name string, data []byte, mode os.FileMode) ProviderBundleFile {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, name)
	if err := os.WriteFile(source, data, mode); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	kind := "readonly"
	if mode&0100 != 0 {
		kind = "executable"
	}
	return ProviderBundleFile{Source: source, Destination: "/" + name, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(data)), Mode: kind}
}

func bundleTestConfig(t *testing.T) ProviderBundleConfig {
	t.Helper()
	file := bundleTestFile(t, "agent", []byte("explicit fixture executable"), 0700)
	return ProviderBundleConfig{Version: 1, Provider: "claude", ProtocolFamily: "claude", AdapterID: "static_json_v1", Entrypoint: "/agent",
		Files: []ProviderBundleFile{file}, Arguments: []string{"--fixture"}, Limits: preparationTestCapabilities().Limits}
}

func TestProviderBundleRejectsImplicitPathsOptionsAndCredentials(t *testing.T) {
	for _, kind := range []string{"valid", "relative_source", "escaping_destination", "work_override", "supervisor_override", "duplicate", "prefix_collision", "zero_limits", "provider", "family", "adapter", "claude_args", "ambient_credential", "unknown_credential", "missing_account", "unknown_device", "duplicate_device"} {
		t.Run(kind, func(t *testing.T) {
			cfg := bundleTestConfig(t)
			switch kind {
			case "relative_source":
				cfg.Files[0].Source = "agent"
			case "escaping_destination":
				cfg.Files[0].Destination = "/a/../agent"
			case "work_override":
				cfg.Files[0].Destination = "/work/agent"
			case "supervisor_override":
				cfg.Files[0].Destination = "/.preparation/helper"
			case "duplicate":
				cfg.Files = append(cfg.Files, cfg.Files[0])
			case "prefix_collision":
				other := cfg.Files[0]
				other.Destination = "/agent/child"
				cfg.Files = append(cfg.Files, other)
			case "zero_limits":
				cfg.Limits.TimeoutSeconds = 0
			case "provider":
				cfg.Provider = "unknown"
			case "family":
				cfg.ProtocolFamily = "codex"
			case "adapter":
				cfg.AdapterID = "codex_app_server_v1"
			case "claude_args":
				cfg.AdapterID = "claude_stream_json_v1"
				cfg.Account = &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "ANTHROPIC_API_KEY", SourceEnv: "S4_TEST_ACCOUNT"}}}
			case "ambient_credential":
				cfg.Account = &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "ANTHROPIC_API_KEY", SourceEnv: "MULTICA_DAEMON_TOKEN"}}}
			case "unknown_credential":
				cfg.Account = &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "NODE_OPTIONS", SourceEnv: "S4_TEST_ACCOUNT"}}}
			case "missing_account":
				cfg.AdapterID, cfg.Arguments = "claude_stream_json_v1", nil
			case "unknown_device":
				cfg.Devices = []string{"/dev/sda"}
			case "duplicate_device":
				cfg.Devices = []string{"/dev/urandom", "/dev/urandom"}
			}
			err := cfg.Validate()
			if (err == nil) != (kind == "valid") {
				t.Fatalf("configuration kind=%s err=%v", kind, err)
			}
		})
	}
}

func TestProviderBundleDigestUsesFrozenFilesAndConfigurationWithoutSecrets(t *testing.T) {
	cfg := bundleTestConfig(t)
	cfg.Account = &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "ANTHROPIC_API_KEY", SourceEnv: "S4_TEST_ACCOUNT"}}}
	t.Setenv("S4_TEST_ACCOUNT", "fixture-first-secret")
	first, err := cfg.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("S4_TEST_ACCOUNT", "fixture-rotated-secret")
	second, err := cfg.Capabilities()
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("capability computation read or hashed a credential value")
	}
	cfg.Arguments = []string{"--another-explicit-mode"}
	third, err := cfg.Capabilities()
	if err != nil || first.Provider.BundleDigest == third.Provider.BundleDigest {
		t.Fatal("invocation change did not change its nonsecret configuration digest")
	}
	cfg.Devices = []string{"/dev/urandom"}
	fourth, err := cfg.Capabilities()
	if err != nil || third.Provider.BundleDigest == fourth.Provider.BundleDigest {
		t.Fatal("explicit device access did not change the capability digest")
	}
}

func TestProviderBundleSnapshotRejectsSymlinksMutationAndNonregularFiles(t *testing.T) {
	for _, kind := range []string{"valid", "symlink", "parent_symlink", "changed", "oversized", "directory", "shared_writable"} {
		t.Run(kind, func(t *testing.T) {
			file := bundleTestFile(t, "explicit", []byte("fixed bytes"), 0600)
			switch kind {
			case "symlink", "parent_symlink":
				link := filepath.Join(filepath.Dir(file.Source), "link")
				target := file.Source
				if kind == "parent_symlink" {
					target = filepath.Dir(target)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				file.Source = link
				if kind == "parent_symlink" {
					file.Source = filepath.Join(link, "explicit")
				}
			case "changed":
				if err := os.WriteFile(file.Source, []byte("other bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				file.SizeBytes--
			case "directory":
				file.Source = filepath.Dir(file.Source)
			case "shared_writable":
				if err := os.Chmod(file.Source, 0666); err != nil {
					t.Fatal(err)
				}
			}
			dest := filepath.Join(t.TempDir(), "snapshot")
			err := snapshotBundleFile(t.Context(), file, dest)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("snapshot kind=%s err=%v", kind, err)
			}
			if kind == "valid" {
				got, readErr := os.ReadFile(dest)
				if readErr != nil || string(got) != "fixed bytes" {
					t.Fatal("snapshot did not preserve verified bytes")
				}
			}
		})
	}
}

func TestProviderBundleAccountSnapshotOnlyReadsExplicitProviderKey(t *testing.T) {
	cfg := bundleTestConfig(t)
	cfg.Account = &ProviderAccountConfig{Environment: []ProviderCredentialEnvironment{{Name: "ANTHROPIC_API_KEY", SourceEnv: "S4_EXPLICIT_KEY"}}}
	t.Setenv("S4_EXPLICIT_KEY", "test-provider-key")
	t.Setenv("ANTHROPIC_API_KEY", "ambient-secret-must-not-appear")
	t.Setenv("MULTICA_DAEMON_TOKEN", "mdt_unrelated")
	t.Setenv("NODE_OPTIONS", "--eval ambient")
	env, err := cfg.accountEnvironment()
	if err != nil || !reflect.DeepEqual(env, []string{"ANTHROPIC_API_KEY=test-provider-key"}) {
		t.Fatalf("unexpected explicit account snapshot: count=%d err=%v", len(env), err)
	}
	for _, secret := range []string{"", "mdt_do-not-copy", "mat_do-not-copy", "mcb_do-not-copy", "line\nbreak"} {
		t.Setenv("S4_EXPLICIT_KEY", secret)
		if _, err := cfg.accountEnvironment(); err == nil {
			t.Fatal("invalid provider credential accepted")
		} else if strings.Contains(err.Error(), secret) && secret != "" {
			t.Fatal("error exposed credential bytes")
		}
	}
}
