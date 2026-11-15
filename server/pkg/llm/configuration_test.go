package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func clearLLMSDKEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_ORG_ID", "OPENAI_PROJECT_ID", "OPENAI_CUSTOM_HEADERS"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigurationFingerprintEffectiveDefaultsAndNonDisclosure(t *testing.T) {
	clearLLMSDKEnvironment(t)
	base := New(Config{APIKey: "private-test-key"}).ConfigurationFingerprint()
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(base) {
		t.Fatalf("fingerprint is not SHA256: %q", base)
	}
	equivalent := New(Config{APIKey: " private-test-key ", BaseURL: "https://api.openai.com/v1", DefaultModel: FallbackModel, MaxRetries: retries(DefaultMaxRetries)})
	if equivalent.ConfigurationFingerprint() != base {
		t.Fatal("explicit equivalent defaults changed the fingerprint")
	}
	for _, cfg := range []Config{
		{APIKey: "different-test-key"},
		{APIKey: "private-test-key", BaseURL: "https://private-gateway.test/v1"},
		{APIKey: "private-test-key", DefaultModel: "different-model"},
		{APIKey: "private-test-key", MaxRetries: retries(0)},
	} {
		got := New(cfg).ConfigurationFingerprint()
		if got == base || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(got) {
			t.Fatal("effective configuration change did not produce a distinct SHA256")
		}
		for _, private := range []string{"private-test-key", "different-test-key", "private-gateway.test"} {
			if strings.Contains(got, private) {
				t.Fatal("fingerprint exposed private construction input")
			}
		}
	}
	var absent *Client
	if absent.ConfigurationFingerprint() != "" {
		t.Fatal("nil client invented a configuration identity")
	}
}

func TestConfigurationFingerprintSnapshotsInheritedSDKInputs(t *testing.T) {
	clearLLMSDKEnvironment(t)
	t.Setenv("OPENAI_API_KEY", "first-private-key")
	t.Setenv("OPENAI_ORG_ID", "first-private-org")
	t.Setenv("OPENAI_PROJECT_ID", "first-private-project")
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Private-Header: first-value")
	var authorization, organization, project, custom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization, organization, project, custom = r.Header.Get("Authorization"), r.Header.Get("OpenAI-Organization"), r.Header.Get("OpenAI-Project"), r.Header.Get("X-Private-Header")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(srv.Close)
	client := New(Config{BaseURL: srv.URL, MaxRetries: retries(0)})
	first := client.ConfigurationFingerprint()
	for _, name := range []string{"OPENAI_API_KEY", "OPENAI_ORG_ID", "OPENAI_PROJECT_ID", "OPENAI_CUSTOM_HEADERS"} {
		before := New(Config{BaseURL: srv.URL}).ConfigurationFingerprint()
		t.Setenv(name, name+"-changed")
		if name == "OPENAI_CUSTOM_HEADERS" {
			t.Setenv(name, "X-Private-Header: changed-value")
		}
		if New(Config{BaseURL: srv.URL}).ConfigurationFingerprint() == before {
			t.Fatalf("changed inherited %s did not change identity", name)
		}
	}
	if client.ConfigurationFingerprint() != first {
		t.Fatal("existing client identity followed later process environment")
	}
	if _, err := client.GenerateJSONResult(context.Background(), "", "Return JSON.", "Test input", 0, 100); err != nil {
		t.Fatalf("original configured request failed: %v", err)
	}
	if authorization != "Bearer first-private-key" || organization != "first-private-org" || project != "first-private-project" || custom != "first-value" {
		t.Fatal("request no longer used its construction-time SDK identity")
	}
}

func TestConfigurationFingerprintCanonicalHeadersAndOverrides(t *testing.T) {
	clearLLMSDKEnvironment(t)
	config := Config{APIKey: "explicit-key", BaseURL: "https://explicit.test/v1"}
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Trace: ignored\nx-tag: two\nX-Trace: final")
	first := New(config).ConfigurationFingerprint()
	t.Setenv("OPENAI_CUSTOM_HEADERS", "X-Tag: two\nx-trace: final\nignored line")
	t.Setenv("OPENAI_API_KEY", "ambient-key")
	t.Setenv("OPENAI_BASE_URL", "https://ambient.test/v1")
	if New(config).ConfigurationFingerprint() != first {
		t.Fatal("equivalent headers or overridden SDK defaults changed identity")
	}
	config.BaseURL += "/"
	config.HTTPClient = &countingHTTPClient{}
	if New(config).ConfigurationFingerprint() != first {
		t.Fatal("equivalent URL or test transport pointer changed configuration identity")
	}
}

func TestAmbientSDKConfigurationCannotEnableAnUnconfiguredClient(t *testing.T) {
	clearLLMSDKEnvironment(t)
	t.Setenv("OPENAI_API_KEY", "ambient-private-key")
	t.Setenv("OPENAI_BASE_URL", "http://127.0.0.1:1")
	transport := &countingHTTPClient{}
	client := New(Config{HTTPClient: transport})
	if client.Enabled() {
		t.Fatal("SDK defaults enabled an unconfigured assist layer")
	}
	result, err := client.GenerateJSONResult(context.Background(), "", "Return JSON.", "Private input", 0, 100)
	if !errors.Is(err, ErrNotConfigured) || transport.requests.Load() != 0 || result.Usage.Status != UsageUnknown {
		t.Fatal("disabled client sent a request or invented usage")
	}
}
