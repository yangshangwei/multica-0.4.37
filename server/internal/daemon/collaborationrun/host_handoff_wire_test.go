package collaborationrun

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostHandoffMalformedWireRefusesBeforeLaunchAndPreservesProof(t *testing.T) {
	// The full required-field/retention matrix lives in pkg/protocol. Exercise
	// the actual member snapshot response here, before a bind or process start.
	for _, mode := range []string{"missing_delegation_truncated", "null_delegation_truncated", "invalid_recording", "case_alias", "escaped_case_alias", "escaped_duplicate"} {
		t.Run(mode, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			if mode != "invalid_recording" {
				h.envelope.Sources[1].Truncated = true
				h.envelope.Sources[1].Content = "Only the first part of the admitted delegation."
			}
			original := h.server.Config.Handler
			h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != runPath(h.claim.Identity.RunID, "handoff/snapshots") {
					original.ServeHTTP(w, r)
					return
				}
				response := httptest.NewRecorder()
				original.ServeHTTP(response, r)
				var wire map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil || response.Code != http.StatusOK {
					t.Errorf("invalid fixture response: status=%d err=%v", response.Code, err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				source := wire["sources"].([]any)[1].(map[string]any)
				switch mode {
				case "missing_delegation_truncated":
					delete(source, "truncated")
				case "null_delegation_truncated":
					source["truncated"] = nil
				case "invalid_recording":
					wire["recording"] = map[string]any{"version": 999, "stream_id": "not-a-uuid", "retained_through": -1, "observed_through": -5, "sealed": true, "reason": "anything"}
				}
				raw, err := json.Marshal(wire)
				if err != nil {
					t.Error(err)
					return
				}
				var alias string
				switch mode {
				case "case_alias":
					alias = `"TRUNCATED":false`
				case "escaped_case_alias":
					alias = `"\u0054RUNCATED":false`
				case "escaped_duplicate":
					alias = `"\u0074runcated":false`
				}
				if alias != "" {
					changed := strings.Replace(string(raw), `"truncated":true`, `"truncated":true,`+alias, 1)
					if changed == string(raw) {
						t.Error("fixture failed to append the alias after the canonical flag")
					}
					raw = []byte(changed)
				}
				if _, err := w.Write(raw); err != nil {
					t.Error(err)
				}
			})
			if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); !errors.Is(err, ErrProtocol) {
				t.Errorf("malformed supported handoff did not fail as protocol error: %v", err)
			}
			if starts := h.starts(t); len(starts) != 0 || len(h.bindings) != 0 || len(h.snapshots) != 1 {
				t.Errorf("malformed wire reached launch/binding or was retried: starts=%d bindings=%d snapshots=%d", len(starts), len(h.bindings), len(h.snapshots))
			}
			permit, ok := h.permits["1"]
			if !ok || len(h.permits) != 1 {
				t.Fatalf("expected only the original launch permit, got %d", len(h.permits))
			}
			end, ok := h.terminations["1"]
			if !ok || end.Kind != "not_started" || end.Identity != h.claim.Identity || end.AttemptNonce != permit.AttemptNonce || len(h.proofs) != 1 {
				t.Fatalf("lost original not_started termination/proof: end=%+v proofs=%d", end, len(h.proofs))
			}
			proof := h.proofs[0]
			if proof.Identity != h.claim.Identity || proof.LastAuthorizedAttemptSeq != "1" || proof.SealedAttemptSeq != "1" || proof.AttemptNonce != permit.AttemptNonce || len(proof.Launches) != 1 || proof.Launches[0] != end {
				t.Fatal("handoff refusal changed the original proof identity or launch fact")
			}
		})
	}
}
