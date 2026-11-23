package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
)

func TestCollaborationHostHarnessDiagnosticsExcludePrivateData(t *testing.T) {
	var output bytes.Buffer
	logger := (collaborationHostRouterInput{Diagnostics: true}).logger(&output)
	private := "private-credential-and-path"
	logger.With("token", private).WithGroup(private).Error(
		"collaboration ledger unavailable; refusing further machine claims", "error", private,
		slog.Group(private, "nested", private))
	logger.Warn(private, "token", private)
	logger.Info("collaboration proof remains pending")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("diagnostic did not emit exactly one JSON event: %v", err)
	}
	if len(record) != 3 || record["message"] != "collaboration ledger unavailable; refusing further machine claims" ||
		record["level"] != "ERROR" || strings.Contains(output.String(), private) {
		t.Fatal("diagnostic lost its fixed message or disclosed excluded fields")
	}
	if _, err := time.Parse(time.RFC3339Nano, record["time"].(string)); err != nil {
		t.Fatal("diagnostic has no parseable observation time")
	}
}

func TestCollaborationHostHarnessDiagnosticsAreExplicit(t *testing.T) {
	for _, body := range []string{`{}`, `{"diagnostics":false}`} {
		var input collaborationHostRouterInput
		if err := json.Unmarshal([]byte(body), &input); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		input.logger(&output).Error("collaboration ledger unavailable; refusing further machine claims")
		if output.Len() != 0 {
			t.Fatal("ordinary host fixture changed its empty diagnostic output contract")
		}
	}
}

func TestCollaborationHostHarnessContextDiagnostic(t *testing.T) {
	if got := collaborationHostHarnessContextState(context.Background()); got != "none" {
		t.Fatalf("live poller context reported %q", got)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := collaborationHostHarnessContextState(canceled); got != "canceled" {
		t.Fatalf("canceled poller context reported %q", got)
	}
	expired, end := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer end()
	if got := collaborationHostHarnessContextState(expired); got != "deadline" {
		t.Fatalf("expired poller context reported %q", got)
	}
}

func TestCollaborationHostHarnessGitUsesExistingConfigValidation(t *testing.T) {
	input := collaborationHostRouterInput{}
	config, err := input.hostConfiguration()
	if err != nil || config.Git != nil {
		t.Fatalf("default Git changed: %+v %v", config, err)
	}
	owned := t.TempDir()
	input.Git = &collaborationrun.HostGitConfig{Executable: filepath.Join(owned, "git"), BundleFile: filepath.Join(owned, "source.bundle"), BundleDigest: strings.Repeat("a", 64), SizeBytes: 128, ObjectFormat: "sha1", CommitOID: strings.Repeat("b", 40)}
	config, err = input.hostConfiguration()
	if err != nil || config.Git == nil || *config.Git != *input.Git {
		t.Fatalf("Git input not forwarded before registration: %+v %v", config, err)
	}
	for _, kind := range []string{"relative_git", "relative_bundle", "bad_digest", "wrong_object_format", "empty_size"} {
		copy := *input.Git
		switch kind {
		case "relative_git":
			copy.Executable = "git"
		case "relative_bundle":
			copy.BundleFile = "source.bundle"
		case "bad_digest":
			copy.BundleDigest = "caller-attestation"
		case "wrong_object_format":
			copy.ObjectFormat = "sha256"
		case "empty_size":
			copy.SizeBytes = 0
		}
		invalid := collaborationHostRouterInput{Git: &copy}
		if _, err := invalid.hostConfiguration(); err == nil || strings.Contains(err.Error(), owned) {
			t.Fatalf("%s did not safely refuse: %v", kind, err)
		}
	}
}

func TestCollaborationHostHarnessTimeoutBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want time.Duration
	}{{"default", `{}`, 3 * time.Minute}, {"minimum", `{"agent_timeout_ms":1000}`, time.Second},
		{"e2e_budget", `{"agent_timeout_ms":15000}`, 15 * time.Second}, {"maximum", `{"agent_timeout_ms":180000}`, 3 * time.Minute},
		{"null", `{"agent_timeout_ms":null}`, 0}, {"zero", `{"agent_timeout_ms":0}`, 0},
		{"negative", `{"agent_timeout_ms":-1000}`, 0}, {"too_short", `{"agent_timeout_ms":999}`, 0},
		{"too_long", `{"agent_timeout_ms":180001}`, 0}, {"string", `{"agent_timeout_ms":"15000"}`, 0},
		{"fraction", `{"agent_timeout_ms":1000.5}`, 0}, {"overflow", `{"agent_timeout_ms":18446744073709551615}`, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			var input collaborationHostRouterInput
			if err := json.Unmarshal([]byte(tc.body), &input); err != nil {
				t.Fatal(err)
			}
			got, err := input.executionTimeout()
			if tc.want == 0 {
				if err == nil {
					t.Fatal("an explicit invalid test timeout was silently treated as the default")
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("test execution deadline = %s, want %s: %v", got, tc.want, err)
			}
		})
	}
}

func TestCollaborationHostHarnessHoldsOnlyCommittedReceiptAcknowledgements(t *testing.T) {
	for _, operation := range []string{"outcome", "proof"} {
		for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusServiceUnavailable} {
			t.Run(operation+"/"+http.StatusText(status), func(t *testing.T) {
				var commits atomic.Int64
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if _, err := io.Copy(io.Discard, r.Body); err != nil {
						t.Error(err)
					}
					if status == http.StatusOK {
						commits.Add(1)
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"accepted":true}`))
				}))
				defer server.Close()
				wire := []byte(`{"protocol_version":3,"containment":"host_process_supervision_v1","event_id":"original-event","receipt_id":"original-receipt","attempt_seq":"1","attempt_nonce":"private-launch","outcome":"failure"}`)
				request, err := http.NewRequest(http.MethodPost, server.URL+"/api/collaboration/runtime/runs/original-run/"+operation, bytes.NewReader(wire))
				if err != nil {
					t.Fatal(err)
				}
				transport := &collaborationHostTransport{capacity: &collaborationCapacityObserver{}, holdReceiptACKs: true}
				response, err := transport.RoundTrip(request)
				if status == http.StatusOK {
					if err == nil || response != nil || commits.Load() != 1 {
						t.Fatal("the fault did not occur after the real test server committed its receipt")
					}
				} else {
					if err != nil || response == nil || response.StatusCode != status || commits.Load() != 0 {
						t.Fatal("the fixture relabelled a server refusal or failure as a lost committed acknowledgement")
					}
					_ = response.Body.Close()
				}
				transport.mu.Lock()
				defer transport.mu.Unlock()
				if len(transport.facts) != 1 {
					t.Fatal("the original HTTP request was not recorded once")
				}
				fact := transport.facts[0]
				digest := sha256.Sum256(wire)
				if fact.Operation != operation || fact.RunID != "original-run" || fact.Status != status ||
					fact.RequestSHA256 != hex.EncodeToString(digest[:]) || fact.AcknowledgementHeld != (status == http.StatusOK) {
					t.Fatal("the observation lost the original request hash or actual response status")
				}
			})
		}
	}
}
