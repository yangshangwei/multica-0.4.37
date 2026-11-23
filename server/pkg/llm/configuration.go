package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ConfigurationFingerprint identifies the immutable model, endpoint, credential
// and retry inputs captured when this client was constructed. Only the digest
// is retained. A custom HTTPClient's behavior and network routing are outside
// this configuration identity; transport pointer addresses are not stable IDs.
func (c *Client) ConfigurationFingerprint() string {
	if c == nil {
		return ""
	}
	return c.configurationFingerprint
}

func fingerprintConfiguration(cfg Config, model string, retry RetryBudget) string {
	// NewClient reads these SDK defaults at construction, not on each request.
	// Capture the same inputs here, respecting this package's explicit overrides.
	environment := make(map[string]string)
	for _, entry := range os.Environ() {
		if name, value, ok := strings.Cut(entry, "="); ok {
			environment[name] = value
		}
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		var present bool
		base, present = environment["OPENAI_BASE_URL"]
		if !present {
			base = "https://api.openai.com/v1/"
		}
	}
	if parsed, err := url.Parse(base); err == nil {
		if !strings.HasSuffix(parsed.Path, "/") {
			parsed.Path += "/"
		}
		base = parsed.String()
	}
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		key = environment["OPENAI_API_KEY"]
	}
	keyHash := sha256.Sum256([]byte(key))
	headers := make(map[string]string)
	for name, header := range map[string]string{"OPENAI_ORG_ID": "OpenAI-Organization", "OPENAI_PROJECT_ID": "OpenAI-Project"} {
		if value, present := environment[name]; present {
			headers[http.CanonicalHeaderKey(header)] = value
		}
	}
	for _, line := range strings.Split(environment["OPENAI_CUSTOM_HEADERS"], "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok {
			headers[http.CanonicalHeaderKey(strings.TrimSpace(name))] = strings.TrimSpace(value)
		}
	}
	if key != "" {
		// Chat completion bearer authentication replaces a custom Authorization.
		delete(headers, "Authorization")
	}
	encoded, err := json.Marshal(struct {
		Version      int               `json:"version"`
		Enabled      bool              `json:"enabled"`
		BaseURL      string            `json:"base_url"`
		APIKeySHA256 string            `json:"api_key_sha256"`
		Model        string            `json:"model"`
		MaxRetries   int               `json:"max_retries"`
		TimeoutNanos int64             `json:"timeout_nanos"`
		Headers      map[string]string `json:"headers"`
	}{
		Version: 1, Enabled: strings.TrimSpace(cfg.APIKey) != "" || strings.TrimSpace(cfg.BaseURL) != "",
		BaseURL: base, APIKeySHA256: hex.EncodeToString(keyHash[:]), Model: model,
		MaxRetries: retry.MaxRetries, TimeoutNanos: int64(retry.RequestTimeout), Headers: headers,
	})
	if err != nil {
		// This fixed scalar/string-map shape cannot fail JSON encoding.
		panic("llm: configuration fingerprint encoding failed")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
