package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexKnowledgeNamespaceBindsSourceConfiguration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	path := filepath.Join(home, "config.toml")
	if err := os.WriteFile(path, []byte("model_provider = 'first'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	d := New(Config{ServerBaseURL: "http://127.0.0.1", DaemonID: "test-daemon"}, nil)
	entry := AgentEntry{Path: filepath.Join(home, "test-codex"), Model: "test-model"}
	first := d.knowledgeConfigurationID("test-runtime", "codex", entry, "0.165.0")
	if first == "" {
		t.Fatal("readable Codex config refused")
	}
	t.Setenv("ANTHROPIC_BASE_URL", "https://unrelated.invalid")
	if second := d.knowledgeConfigurationID("test-runtime", "codex", entry, "0.165.0"); first != second {
		t.Fatal("Claude settings changed Codex namespace")
	}
	if err := os.WriteFile(path, []byte("model_provider = 'second'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if second := d.knowledgeConfigurationID("test-runtime", "codex", entry, "0.165.0"); first == second {
		t.Fatal("changed Codex configuration retained old namespace")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if value := d.knowledgeConfigurationID("test-runtime", "codex", entry, "0.165.0"); value != "" {
		t.Fatal("unreadable config accepted as absent")
	}
}
