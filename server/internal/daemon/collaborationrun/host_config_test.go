package collaborationrun

import (
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestHostConfigurationRejectsIgnoredArgumentsAndPrivateSessionSelectors(t *testing.T) {
	for _, test := range []struct {
		name, provider string
		arguments      []string
		allowed        bool
	}{
		{"ordinary budget", "claude", []string{"--max-budget-usd", "1.00"}, true},
		{"ignored protocol override", "claude", []string{"--output-format", "text"}, false},
		{"private session", "claude", []string{"--resume", "private-session"}, false},
		{"compact resume", "claude", []string{"-rprivate-session"}, false},
		{"latest conversation", "claude", []string{"-c"}, false},
		{"private directory", "codex", []string{"--cd", "/private"}, false},
		{"ignored opencode extra", "opencode", []string{"--verbose"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := HostConfig{Provider: test.provider, ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "fixture-account",
				ExtraArgs: test.arguments, Limits: preparationTestCapabilities().Limits}
			if err := config.Validate(); (err == nil) != test.allowed {
				t.Fatalf("invocation validation: %v", err)
			}
		})
	}
}

func TestHostExecutorRefusesUnfrozenExecutionOptions(t *testing.T) {
	f := newHostRunFixture(t, "result")
	for name, options := range map[string]agent.ExecOptions{
		"model":           {Model: "unfrozen-model"},
		"MCP":             {McpConfig: []byte(`{"mcpServers":{}}`)},
		"private session": {ResumeSessionID: hostFixtureSession},
		"max turns":       {MaxTurns: 3},
		"gateway":         {OpenclawMode: "gateway"},
	} {
		t.Run(name, func(t *testing.T) {
			executor := *f.runner.Host
			executor.Options = options
			if err := executor.Probe(t.Context()); err == nil {
				t.Fatal("unfrozen/ignored invocation was admitted")
			}
		})
	}
}
