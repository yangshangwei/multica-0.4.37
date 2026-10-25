package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func hostProtocolFixture() (CollaborationRuntimeCapabilities, CollaborationLaunchTermination) {
	p, _ := preparationProtocolFixture()
	provider := p.Provider
	provider.AdapterID, provider.BundleDigest, provider.Resume, provider.Fallback = "ordinary_backend_v1", "", "isolated", true
	caps := CollaborationRuntimeCapabilities{Version: 2, PreparationVersion: 2, OS: "darwin", Arch: "arm64",
		Containment: CollaborationContainmentHostProcessSupervision, AttachmentFormat: CollaborationAttachmentFormat,
		Provider: provider, Limits: p.Limits, Host: &CollaborationHostCapability{ProviderVersion: "1.2.3", ProviderMode: "local",
			ConfigurationDigest: strings.Repeat("a", 64), SupervisionScopes: []string{"process_group", "direct_process"}}}
	nonce, exit := uuid.NewString(), 0
	terminal := CollaborationLaunchTermination{ProtocolVersion: 3, Identity: p.Identity, Containment: caps.Containment,
		AttemptSeq: "1", AttemptNonce: nonce, LaunchID: nonce, EventID: uuid.NewString(), Kind: "managed_launch_ended",
		Cleanup: CollaborationCleanupObservation{State: "unverified", Scope: "process_group"}, ExitCode: &exit}
	return caps, terminal
}

func TestCollaborationHostCapabilityBindsModeAndConfiguration(t *testing.T) {
	caps, _ := hostProtocolFixture()
	if err := ValidateCollaborationRuntimeCapabilities(caps); err != nil {
		t.Fatal(err)
	}
	digest, err := CollaborationRuntimeCapabilityDigest(caps)
	if err != nil || len(digest) != 64 {
		t.Fatalf("invalid capability digest: %v", err)
	}
	caps.Host.ConfigurationDigest = strings.Repeat("b", 64)
	changed, _ := CollaborationRuntimeCapabilityDigest(caps)
	if digest == changed {
		t.Fatal("managed permission changes reused capability identity")
	}
	for name, mutate := range map[string]func(*CollaborationRuntimeCapabilities){
		"legacy version":     func(c *CollaborationRuntimeCapabilities) { c.Version = 1 },
		"legacy preparation": func(c *CollaborationRuntimeCapabilities) { c.PreparationVersion = 1 },
		"legacy mode":        func(c *CollaborationRuntimeCapabilities) { c.Containment = CollaborationContainmentLinuxPIDNamespace },
		"remote unobserved":  func(c *CollaborationRuntimeCapabilities) { c.Host.ProviderMode = "gateway" },
		"unknown provider": func(c *CollaborationRuntimeCapabilities) {
			c.Provider.Provider, c.Provider.ProtocolFamily = "unknown", "unknown"
		},
		"missing observer": func(c *CollaborationRuntimeCapabilities) { c.Host.SupervisionScopes = nil },
		"unknown scope":    func(c *CollaborationRuntimeCapabilities) { c.Host.SupervisionScopes = []string{"all_descendants"} },
		"duplicate scope": func(c *CollaborationRuntimeCapabilities) {
			c.Host.SupervisionScopes = []string{"direct_process", "direct_process"}
		},
		"missing version":      func(c *CollaborationRuntimeCapabilities) { c.Host.ProviderVersion = "" },
		"private account path": func(c *CollaborationRuntimeCapabilities) { c.Provider.AccountSource = "/private/account" },
		"unbound config":       func(c *CollaborationRuntimeCapabilities) { c.Host.ConfigurationDigest = "" },
	} {
		t.Run(name, func(t *testing.T) {
			bad, _ := hostProtocolFixture()
			mutate(&bad)
			if ValidateCollaborationRuntimeCapabilities(bad) == nil {
				t.Fatal("unsupported combination accepted")
			}
		})
	}
}

func TestCollaborationHostTerminationDoesNotConfuseResultOrCleanup(t *testing.T) {
	_, terminal := hostProtocolFixture()
	if err := ValidateCollaborationLaunchTermination(terminal); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CollaborationLaunchTermination){
		"legacy version":          func(v *CollaborationLaunchTermination) { v.ProtocolVersion = 2 },
		"legacy mode":             func(v *CollaborationLaunchTermination) { v.Containment = CollaborationContainmentLinuxPIDNamespace },
		"different launch":        func(v *CollaborationLaunchTermination) { v.LaunchID = uuid.NewString() },
		"business success":        func(v *CollaborationLaunchTermination) { v.Kind = "success" },
		"EOF":                     func(v *CollaborationLaunchTermination) { v.Kind = "stdout_eof" },
		"timer":                   func(v *CollaborationLaunchTermination) { v.Kind = "timeout" },
		"never started with exit": func(v *CollaborationLaunchTermination) { v.Kind = "not_started" },
		"cleanup not attempted":   func(v *CollaborationLaunchTermination) { v.Cleanup.State = "best_effort" },
		"all process claim":       func(v *CollaborationLaunchTermination) { v.Cleanup.Scope = "all_descendants" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := terminal
			mutate(&bad)
			if ValidateCollaborationLaunchTermination(bad) == nil {
				t.Fatal("invalid lifecycle assertion accepted")
			}
		})
	}
}

func TestCollaborationHostProofSealsEveryLaunch(t *testing.T) {
	_, first := hostProtocolFixture()
	second := first
	second.AttemptSeq, second.AttemptNonce, second.EventID = "2", uuid.NewString(), uuid.NewString()
	second.LaunchID = second.AttemptNonce
	second.Cleanup = CollaborationCleanupObservation{State: "best_effort", Scope: "direct_process", Attempted: true}
	proof := CollaborationRunProof{ProtocolVersion: 3, Identity: first.Identity, ReceiptID: uuid.NewString(),
		LastAuthorizedAttemptSeq: "2", SealedAttemptSeq: "2", AttemptNonce: second.AttemptNonce,
		State: "host_exit_observed", Containment: first.Containment, Launches: []CollaborationLaunchTermination{first, second}}
	var err error
	proof.Cleanup, err = SummarizeCollaborationCleanup(proof.Launches)
	if err != nil || proof.Cleanup.State != "unverified" || proof.Cleanup.Scope != "mixed" || !proof.Cleanup.Attempted {
		t.Fatalf("cleanup overclaimed scope: %+v %v", proof.Cleanup, err)
	}
	if err = ValidateCollaborationHostProof(proof); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CollaborationRunProof){
		"legacy strong proof":   func(p *CollaborationRunProof) { p.State = "exited_proven" },
		"missing launch":        func(p *CollaborationRunProof) { p.Launches = p.Launches[:1] },
		"unsealed launch":       func(p *CollaborationRunProof) { p.SealedAttemptSeq = "1" },
		"different final nonce": func(p *CollaborationRunProof) { p.AttemptNonce = uuid.NewString() },
		"false cleanup": func(p *CollaborationRunProof) {
			p.Cleanup = &CollaborationCleanupObservation{State: "tracked_scope_complete", Scope: "direct_process"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := proof
			mutate(&bad)
			if ValidateCollaborationHostProof(bad) == nil {
				t.Fatal("incomplete or cross-mode proof accepted")
			}
		})
	}
}

func TestCollaborationLegacyRuntimeWireRemainsUnchanged(t *testing.T) {
	for _, value := range []any{CollaborationRunPermitRequest{}, CollaborationRunOutcome{}, CollaborationRunProof{}, CollaborationRunClaimResponse{}} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"protocol_version", "launch_descriptor", "launches", "cleanup", "host_context"} {
			if strings.Contains(string(raw), `"`+field+`"`) {
				t.Fatalf("legacy DTO gained host field %s", field)
			}
		}
	}
}

func TestCollaborationHostCleanupRequiresCompleteStrictFacts(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"state":"unverified","scope":"direct_process"}`,
		`{"state":"unverified","scope":"direct_process","attempted":null}`,
		`{"state":"unverified","scope":"direct_process","attempted":false,"attempted":true}`,
		`{"state":"unverified","scope":"direct_process","attempted":false,"all_descendants":true}`} {
		var cleanup CollaborationCleanupObservation
		if json.Unmarshal([]byte(raw), &cleanup) == nil {
			t.Fatalf("incomplete/ambiguous cleanup accepted: %s", raw)
		}
	}
}

func TestCollaborationHostPreparationObservationVersions(t *testing.T) {
	_, terminal := hostProtocolFixture()
	observation := CollaborationPreparationObservation{ProtocolVersion: 3, Containment: terminal.Containment, Identity: terminal.Identity,
		EventID: uuid.NewString(), AttemptSeq: "3", AttemptNonce: uuid.NewString(), ManifestDigest: strings.Repeat("a", 64), Stage: "git", State: "prepared"}
	if err := ValidateCollaborationPreparationObservation(observation); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CollaborationPreparationObservation){
		"legacy version with host mode": func(o *CollaborationPreparationObservation) { o.ProtocolVersion = 0 },
		"host version with legacy mode": func(o *CollaborationPreparationObservation) {
			o.Containment = CollaborationContainmentLinuxPIDNamespace
		},
		"unknown version":    func(o *CollaborationPreparationObservation) { o.ProtocolVersion = 4 },
		"excess attempts":    func(o *CollaborationPreparationObservation) { o.AttemptSeq = "65" },
		"ambiguous sequence": func(o *CollaborationPreparationObservation) { o.AttemptSeq = "03" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := observation
			mutate(&bad)
			if ValidateCollaborationPreparationObservation(bad) == nil {
				t.Fatal("invalid version/attempt accepted")
			}
		})
	}
	observation.ProtocolVersion, observation.Containment, observation.AttemptSeq = 0, "", "1"
	if err := ValidateCollaborationPreparationObservation(observation); err != nil {
		t.Fatal("legacy first-attempt contract changed")
	}
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{`"protocol_version":0,`, `"protocol_version":null,`, `"containment":"",`} {
		var legacy CollaborationPreparationObservation
		if json.Unmarshal([]byte("{"+extra+string(raw[1:])), &legacy) == nil {
			t.Fatal("legacy observation accepted host metadata")
		}
	}
}
