package collaborationrun

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func hostSessionClaim() protocol.CollaborationRunClaimResponse {
	return protocol.CollaborationRunClaimResponse{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
		Containment: protocol.CollaborationContainmentHostProcessSupervision, Identity: testIdentity(), AgentID: uuid.NewString(),
		ResourceOwnerID: uuid.NewString(), HostContext: &protocol.CollaborationHostContext{AgentOwnerUserID: uuid.NewString(), RuntimeID: uuid.NewString(),
			Role: "leader", StepKey: "leader", SessionGeneration: "1"},
		Preparation: &protocol.CollaborationPreparation{Provider: protocol.CollaborationPreparationProvider{Provider: "claude"},
			Host: &protocol.CollaborationHostCapability{ConfigurationDigest: strings.Repeat("c", 64)}}}
}

func TestHostSessionKeySeparatesOwnersDeploymentsIssuesRolesAndGenerations(t *testing.T) {
	claim := hostSessionClaim()
	want, err := hostSessionKey(claim)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*protocol.CollaborationRunClaimResponse){
		"deployment":     func(c *protocol.CollaborationRunClaimResponse) { c.Identity.DeploymentID = uuid.NewString() },
		"issue":          func(c *protocol.CollaborationRunClaimResponse) { c.Identity.SessionID = uuid.NewString() },
		"resource owner": func(c *protocol.CollaborationRunClaimResponse) { c.ResourceOwnerID = uuid.NewString() },
		"agent owner":    func(c *protocol.CollaborationRunClaimResponse) { c.HostContext.AgentOwnerUserID = uuid.NewString() },
		"agent":          func(c *protocol.CollaborationRunClaimResponse) { c.AgentID = uuid.NewString() },
		"runtime":        func(c *protocol.CollaborationRunClaimResponse) { c.HostContext.RuntimeID = uuid.NewString() },
		"channel":        func(c *protocol.CollaborationRunClaimResponse) { c.Identity.ChannelID = uuid.NewString() },
		"generation":     func(c *protocol.CollaborationRunClaimResponse) { c.HostContext.SessionGeneration = "2" },
		"role":           func(c *protocol.CollaborationRunClaimResponse) { c.HostContext.Role = "member" },
	} {
		t.Run(name, func(t *testing.T) {
			other := claim
			context := *claim.HostContext
			other.HostContext = &context
			change(&other)
			got, err := hostSessionKey(other)
			if err != nil || got == want {
				t.Fatalf("%s did not separate session identity: %v", name, err)
			}
		})
	}
	// A new continuation/run and alias target in the same actual lane can use
	// this issue's own session; no human handoff identity appears in its key.
	claim.Identity.RunID, claim.Identity.TargetID = uuid.NewString(), uuid.NewString()
	claim.HostContext.Role, claim.HostContext.StepKey = "continuation", "continuation:2"
	got, err := hostSessionKey(claim)
	if err != nil || got != want {
		t.Fatalf("compatible continuation lost its session: %v", err)
	}
	claim.HostContext.Role = "member"
	member, err := hostSessionKey(claim)
	if err != nil {
		t.Fatal(err)
	}
	claim.Identity.GroupID = uuid.NewString()
	otherGroup, err := hostSessionKey(claim)
	if err != nil || member == otherGroup {
		t.Fatalf("member session escaped its execution group: %v", err)
	}
}

func TestHostSessionLockKeepsUnknownOwnershipAfterSupervisorRestart(t *testing.T) {
	l := newHostTestLedger(t)
	r := registerHostTestRun(t, l)
	key := r.record.Host.SessionKey
	owner, err := l.lockHostSession(key, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.lockHostSession(key, uuid.NewString()); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("two runs acquired one session: %v", err)
	}
	p := hostTestPermit(t, r)
	if err := r.burnHost(p.AttemptNonce); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	// OS locks are gone. The durable unknown record must still own this key.
	if _, err := l.lockHostSession(key, uuid.NewString()); !errors.Is(err, ErrSessionBusy) {
		t.Fatalf("supervisor restart discarded logical session ownership: %v", err)
	}
}
