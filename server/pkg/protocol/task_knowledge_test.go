package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func taskKnowledgeProtocolSession() TaskKnowledgeSessionBinding {
	return TaskKnowledgeSessionBinding{Provider: "claude", RuntimeID: uuid.NewString(), AgentID: uuid.NewString(), ConfigurationID: "test-config", ScopeKind: "issue", ScopeID: uuid.NewString(), Generation: uuid.NewString()}
}

func TestTaskKnowledgeRequestRejectsPrivateOrAmbiguousFields(t *testing.T) {
	req := TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: time.Now().UTC().Format(time.RFC3339Nano), Session: taskKnowledgeProtocolSession(), InvocationFingerprint: strings.Repeat("a", 64)}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"unknown private field":     strings.Replace(string(raw), `"protocol_version":1`, `"protocol_version":1,"candidate_id":"private"`, 1),
		"duplicate identity":        strings.Replace(string(raw), `"protocol_version":1`, `"protocol_version":1,"protocol_version":1`, 1),
		"case alias":                strings.Replace(string(raw), `"claim_generation"`, `"Claim_Generation"`, 1),
		"missing native generation": strings.Replace(string(raw), `"generation":"`+req.Session.Generation+`"`, `"generation":""`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			var decoded TaskKnowledgePrepareRequest
			if json.Unmarshal([]byte(body), &decoded) == nil {
				t.Fatal("ambiguous or unscoped request was accepted")
			}
		})
	}
	var decoded TaskKnowledgePrepareRequest
	if err = json.Unmarshal(raw, &decoded); err != nil || decoded.Session != req.Session {
		t.Fatalf("valid typed request did not round-trip: %v", err)
	}
}

// H2: a daemon whose reachability gate dropped the claimed native session says
// so on prepare. The field is optional, so an unflagged request keeps its exact
// bytes (and receipt hash), and strict decoding still rejects malformed forms.
func TestTaskKnowledgePrepareRequestCarriesResumeUnavailable(t *testing.T) {
	req := TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: time.Now().UTC().Format(time.RFC3339Nano), Session: taskKnowledgeProtocolSession(), InvocationFingerprint: strings.Repeat("a", 64)}
	req.Session.NativeSessionID = "stale-native"
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "resume_unavailable") {
		t.Fatalf("unflagged request changed its wire bytes: %s", raw)
	}
	flagged := strings.TrimSuffix(string(raw), "}") + `,"resume_unavailable":true}`
	var decoded TaskKnowledgePrepareRequest
	if err := json.Unmarshal([]byte(flagged), &decoded); err != nil {
		t.Fatalf("flagged prepare request was rejected: %v", err)
	}
	again, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(again), `"resume_unavailable":true`) {
		t.Fatalf("flag did not round-trip: %s", again)
	}
	unflaggedHash, err := TaskKnowledgeRequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	if want := CollaborationKnowledgeHash(string(raw)); unflaggedHash != want {
		t.Fatal("unflagged receipt hash changed")
	}
	for name, body := range map[string]string{
		"case alias": strings.TrimSuffix(string(raw), "}") + `,"Resume_Unavailable":true}`,
		"duplicate":  strings.TrimSuffix(string(raw), "}") + `,"resume_unavailable":false,"resume_unavailable":true}`,
		"string":     strings.TrimSuffix(string(raw), "}") + `,"resume_unavailable":"true"}`,
		"number":     strings.TrimSuffix(string(raw), "}") + `,"resume_unavailable":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			var malformed TaskKnowledgePrepareRequest
			if json.Unmarshal([]byte(body), &malformed) == nil {
				t.Fatalf("malformed resume_unavailable was accepted: %s", body)
			}
		})
	}
}

func TestTaskKnowledgeClaimAndNativeGenerationsRemainDistinct(t *testing.T) {
	s := taskKnowledgeProtocolSession()
	if err := ValidateTaskKnowledgeSession(s); err != nil {
		t.Fatal(err)
	}
	s.Generation = "1"
	if ValidateTaskKnowledgeSession(s) == nil {
		t.Fatal("numeric claim generation became a native scope generation")
	}
	for _, generation := range []string{"0", "01", "-1", "9999999999999999999", uuid.NewString()} {
		if ValidateTaskKnowledgeRequest(1, uuid.NewString(), generation, time.Now().UTC().Format(time.RFC3339Nano)) == nil {
			t.Fatalf("invalid decimal claim generation accepted: %s", generation)
		}
	}
}

func TestTaskKnowledgeObservationRejectsMixedFacts(t *testing.T) {
	req := TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: time.Now().UTC().Format(time.RFC3339Nano), AttemptSequence: 1, AttemptNonce: uuid.NewString(), Kind: "native_session", NativeSessionID: "native", Source: "system"}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TaskKnowledgeObservation
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"process_id":42`, `"wait_observed":true`, `"source_body":"private"`} {
		body := strings.TrimSuffix(string(raw), "}") + "," + field + "}"
		if json.Unmarshal([]byte(body), &decoded) == nil {
			t.Fatalf("native identity accepted unrelated/private facts: %s", field)
		}
	}
}

func TestTaskKnowledgeClaimDescriptorHasNoPrivateMaterial(t *testing.T) {
	d := TaskKnowledgeClaimDescriptor{ProtocolVersion: 1, TaskID: uuid.NewString(), RuntimeID: uuid.NewString(), ClaimGeneration: "1", DispatchedAt: time.Now().UTC().Format(time.RFC3339Nano), Provider: "claude", Mode: "local", ConfigurationID: "opaque-runtime", PreparationRequired: true, Session: taskKnowledgeProtocolSession()}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"candidate_id", "source_issue_id", "manifest", "summary", "content", "sha256", "invocation_fingerprint"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Fatalf("private or premature field in claim: %s", key)
		}
	}
}

func TestTaskKnowledgeFenceIsStrictAndSourceFree(t *testing.T) {
	var fence TaskKnowledgeFence
	if json.Unmarshal([]byte(`{"claim_generation":"1","dispatched_at":"2026-11-01T00:00:00Z"}`), &fence) != nil {
		t.Fatal("valid fence rejected")
	}
	for _, body := range []string{`{"claim_generation":"1","claim_generation":"2","dispatched_at":"x"}`, `{"claim_generation":"1","dispatched_at":"x","source_id":"private"}`} {
		if json.Unmarshal([]byte(body), &fence) == nil {
			t.Fatal("ambiguous/private fence accepted")
		}
	}
}

func TestTaskKnowledgeObservationAckRejectsMalformedShapes(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"accepted":null}`, `{"Accepted":true}`, `{"accepted":true,"accepted":false}`, `{"accepted":true,"content":"private"}`, `{"accepted":"true"}`, `[]`, `{"accepted":true} {"accepted":true}`} {
		var ack TaskKnowledgeObservationAck
		if json.Unmarshal([]byte(body), &ack) == nil {
			t.Fatalf("ambiguous acknowledgment accepted: %s", body)
		}
	}
	for _, value := range []bool{true, false} {
		raw, err := json.Marshal(TaskKnowledgeObservationAck{Accepted: value})
		if err != nil {
			t.Fatal(err)
		}
		var ack TaskKnowledgeObservationAck
		if err = json.Unmarshal(raw, &ack); err != nil || ack.Accepted != value {
			t.Fatalf("canonical acknowledgment changed: %s %v", raw, err)
		}
	}
}
