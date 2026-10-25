package agent

import "testing"

func TestNormalizeHostProviderVersion(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ provider, input, want string }{
		{"claude", "2.1.5 (Claude Code)", "2.1.5"},
		{"claude", "v2.1.5", "2.1.5"},
		{"claude", " 2.1.5-beta.2+build.7 (Claude Code)\n", "2.1.5-beta.2+build.7"},
		{"codex", "codex-cli 0.118.0", "0.118.0"},
		{"codex", "codex-cli 0.118.0-alpha.1+build.5", "0.118.0-alpha.1+build.5"},
		{"codex", "0.118.0-alpha.2+build.5", "0.118.0-alpha.2+build.5"},
		{"opencode", "1.17.7", "1.17.7"},
		{"opencode", "opencode v1.17.7", "1.17.7"},
		{"opencode", "OpenCode 1.17.7-preview.1", "1.17.7-preview.1"},
		{"claude", "fixture-1", "fixture-1"},
		{"codex", "fixture-2-alpha.1", "fixture-2-alpha.1"},
		{"claude", "", ""},
		{"claude", "unknown", ""},
		{"claude", "fixture-ready", ""},
		{"claude", "2.1.5 or 2.2.0 (Claude Code)", ""},
		{"claude", "2.1.5\n2.2.0", ""},
		{"claude", "codex-cli 2.1.5", ""},
		{"codex", "0.118.0 (wrapper 1.0.0)", ""},
		{"codex", "codex-cli 0.118.0/other", ""},
		{"opencode", "opencode OpenCode 1.17.7", ""},
		{"opencode", "version unavailable", ""},
		{"future-provider", "1.2.3", ""},
	} {
		t.Run(test.provider+"/"+test.input, func(t *testing.T) {
			got, ok := NormalizeHostProviderVersion(test.provider, test.input)
			if got != test.want || ok != (test.want != "") {
				t.Fatalf("NormalizeHostProviderVersion(%q, %q) = %q, %v; want %q", test.provider, test.input, got, ok, test.want)
			}
		})
	}
}
