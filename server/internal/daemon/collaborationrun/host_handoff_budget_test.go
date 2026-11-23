package collaborationrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestHostHandoffBudgetEscapedAssignmentReachesProviderIntact(t *testing.T) {
	for _, tc := range []struct {
		name string
		unit string
	}{
		{name: "html_less_than", unit: "<"},
		{name: "html_ampersand", unit: "&"},
		{name: "control_character", unit: "\x01"},
		{name: "multibyte_and_escapes", unit: "<&\x01<&\x01<&\x01<&\x01中🙂"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			const assignmentBytes = 64 << 10
			const prefix, suffix = "Admitted assignment begins:\n", "\nAdmitted assignment ends 中🙂"
			remaining := assignmentBytes - len(prefix) - len(suffix)
			assignment := prefix + strings.Repeat(tc.unit, remaining/len(tc.unit)) + strings.Repeat("x", remaining%len(tc.unit)) + suffix
			if len(assignment) != assignmentBytes || !utf8.ValidString(assignment) {
				t.Fatal("fixture must contain exactly 64 KiB of valid UTF-8")
			}
			h.envelope.Sources[1].Content = assignment
			h.envelope.Sources[1].SHA256 = protocol.CollaborationHandoffHash(assignment)
			if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
				t.Fatalf("legal escaped assignment was refused: %v", err)
			}
			starts := h.starts(t)
			if len(starts) != 1 || len(h.snapshots) != 1 || len(h.bindings) != 1 {
				t.Fatalf("starts=%d snapshots=%d bindings=%d", len(starts), len(h.snapshots), len(h.bindings))
			}
			prompt, input := hostStartPrompt(t, starts[0])
			var handoff protocol.CollaborationHandoffEnvelope
			if err := json.Unmarshal(input["handoff"], &handoff); err != nil {
				t.Fatalf("fake provider received an invalid handoff: %v", err)
			}
			material, wire, err := protocol.CollaborationHandoffSizes(handoff)
			if err != nil || material > protocol.CollaborationHandoffMaxBytes || wire <= protocol.CollaborationHandoffMaxBytes || wire > protocol.CollaborationHandoffMaxJSONBytes {
				t.Fatalf("fixture did not cross the former JSON limit within both new budgets: material=%d wire=%d err=%v", material, wire, err)
			}
			if len(handoff.Sources) != len(h.envelope.Sources) || handoff.Sources[1] != h.envelope.Sources[1] {
				t.Fatal("actual provider stdin lost or truncated the exact admitted assignment")
			}
			binding := h.bindings[0]
			if binding.PromptSHA256 != protocol.CollaborationHandoffHash(prompt) || binding.SnapshotID != handoff.SnapshotID || handoff.SnapshotID != h.snapshots[0].RequestID {
				t.Fatal("binding does not match the complete prompt read by the fake provider")
			}
			permit, ok := h.permits[binding.AttemptSeq]
			if !ok || permit.AttemptNonce != binding.AttemptNonce || permit.LaunchDescriptor.Kind != "provider" {
				t.Fatal("prompt binding lost the actual provider launch identity")
			}
			t.Logf("assignment_bytes=%d material_bytes=%d wire_bytes=%d provider_starts=1 exact_assignment=true prompt_hash_matches=true", len(assignment), material, wire)
		})
	}
}

func TestHostHandoffBudgetEnforcesRawHTTPBoundaryBeforeLaunch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		extraByte int
		wantError bool
	}{
		{name: "exact_wire_cap"},
		{name: "one_byte_over_wire_cap", extraByte: 1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			responseBytes := protocol.CollaborationHandoffMaxJSONBytes + tc.extraByte
			original := h.server.Config.Handler
			h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != runPath(h.claim.Identity.RunID, "handoff/snapshots") {
					original.ServeHTTP(w, r)
					return
				}
				// Transform only the fixture's wire representation, preserving
				// its normal authorization and logical snapshot response.
				recorded := httptest.NewRecorder()
				original.ServeHTTP(recorded, r)
				if recorded.Code != http.StatusOK {
					t.Errorf("fixture snapshot status=%d", recorded.Code)
					w.WriteHeader(recorded.Code)
					return
				}
				raw := bytes.TrimSuffix(recorded.Body.Bytes(), []byte("\n"))
				body := append(bytes.Clone(raw), bytes.Repeat([]byte(" "), responseBytes-len(raw)-1)...)
				body = append(body, '\n')
				if !json.Valid(body) || len(body) != responseBytes {
					t.Error("wire boundary fixture is not complete JSON with its final newline")
				}
				_, _ = w.Write(body)
			})
			err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request)
			if tc.wantError {
				if !errors.Is(err, ErrProtocol) || len(h.starts(t)) != 0 || len(h.bindings) != 0 {
					t.Fatalf("oversize HTTP response reached launch: err=%v starts=%d bindings=%d", err, len(h.starts(t)), len(h.bindings))
				}
				if end, ok := h.terminations["1"]; !ok || end.Kind != "not_started" || len(h.proofs) != 1 {
					t.Fatal("wire budget rejection lost original unlaunched proof cleanup")
				}
			} else if err != nil || len(h.starts(t)) != 1 || len(h.bindings) != 1 {
				t.Fatalf("response at the shared wire limit was refused: err=%v starts=%d bindings=%d", err, len(h.starts(t)), len(h.bindings))
			}
			if len(h.snapshots) != 1 {
				t.Fatalf("wire limit response caused snapshot retries: reads=%d", len(h.snapshots))
			}
			t.Logf("raw_http_bytes=%d protocol_rejected=%t provider_starts=%d", responseBytes, tc.wantError, len(h.starts(t)))
		})
	}
}

func TestHostHandoffBudgetRejectsInvalidMaterialBeforeLaunch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*protocol.CollaborationHandoffEnvelope)
	}{
		{name: "decoded_content_plus_metadata", mutate: func(v *protocol.CollaborationHandoffEnvelope) {
			otherContent := 0
			for i, source := range v.Sources {
				if i != 2 {
					otherContent += len(source.Content)
				}
			}
			v.Sources[2].Content = strings.Repeat("x", protocol.CollaborationHandoffMaxBytes-otherContent)
			v.Sources[2].SHA256 = protocol.CollaborationHandoffHash(v.Sources[2].Content)
		}},
		{name: "metadata_budget", mutate: func(v *protocol.CollaborationHandoffEnvelope) {
			v.Missing = []string{strings.Repeat("x", protocol.CollaborationHandoffMaxBytes)}
		}},
		{name: "malformed_source", mutate: func(v *protocol.CollaborationHandoffEnvelope) {
			v.Sources[1].SHA256 = "not-a-sha256"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			tc.mutate(&h.envelope)
			material, wire, err := protocol.CollaborationHandoffSizes(h.envelope)
			if err != nil || wire >= protocol.CollaborationHandoffMaxJSONBytes {
				t.Fatalf("invalid material must fit within the independent wire limit: wire=%d err=%v", wire, err)
			}
			if tc.name != "malformed_source" && material <= protocol.CollaborationHandoffMaxBytes {
				t.Fatalf("material fixture did not exceed the decoded budget: material=%d", material)
			}
			err = h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request)
			if !errors.Is(err, ErrProtocol) || len(h.starts(t)) != 0 || len(h.bindings) != 0 || len(h.snapshots) != 1 {
				t.Fatalf("invalid material reached launch or was retried: err=%v starts=%d bindings=%d reads=%d", err, len(h.starts(t)), len(h.bindings), len(h.snapshots))
			}
			if end, ok := h.terminations["1"]; !ok || end.Kind != "not_started" || len(h.proofs) != 1 {
				t.Fatal("invalid material rejection lost original unlaunched proof cleanup")
			}
			t.Logf("material_bytes=%d wire_bytes=%d protocol_rejected=true provider_starts=0", material, wire)
		})
	}
}
