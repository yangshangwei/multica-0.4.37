package collaboration

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
)

func TestCreatorKeepsLifecycleAfterLeavingSourceWorkspace(t *testing.T) {
	s := AccessState{AccountActive: true, Participant: true, Creator: true, StatusCategory: "todo", QueueState: "paused"}
	p := Principal{UserID: "creator", Kind: auth.CredentialJWT}
	c := CapabilitiesFor(p, s)
	if !c.Read || !c.Comment || !c.Complete || !c.Cancel || !c.Edit || !c.ControlAI {
		t.Fatalf("valid creator lost collaboration rights after workspace departure: %+v", c)
	}
	if c.EditSourceStructure || c.Reorder {
		t.Fatal("departed creator retained source-workspace structure rights")
	}
	s.StatusCategory, s.QueueState = "done", "closed"
	c = CapabilitiesFor(p, s)
	if !c.Reopen || c.ChangeStatus || c.Complete || c.Cancel {
		t.Fatalf("ended issue did not require explicit creator reopen: %+v", c)
	}
}

func TestExternalHandlerCannotReopenThroughOrdinaryStatusChange(t *testing.T) {
	p := Principal{UserID: "handler", Kind: auth.CredentialPAT}
	s := AccessState{AccountActive: true, Participant: true, Handler: true, StatusCategory: "in_progress", QueueState: "active"}
	c := CapabilitiesFor(p, s)
	if !c.Read || !c.Edit || !c.ChangeStatus || !c.Invite || !c.Transfer || c.EditSourceStructure || c.Complete || c.Cancel || c.Reopen {
		t.Fatalf("external handler role mismatch: %+v", c)
	}
	for _, status := range []string{"done", "cancelled", "unrecognized"} {
		s.StatusCategory = status
		if c := CapabilitiesFor(p, s); c.ChangeStatus || c.Reopen {
			t.Fatalf("handler can bypass lifecycle from %s: %+v", status, c)
		}
	}
	s.StatusCategory, s.QueueState = "todo", "closed"
	if CapabilitiesFor(p, s).ChangeStatus {
		t.Fatal("closed session can be reopened by ordinary status change")
	}
}

func TestCollaborationAccessKeepsMembershipAndParticipationIndependent(t *testing.T) {
	p := Principal{UserID: "viewer", Kind: auth.CredentialJWT}
	s := AccessState{AccountActive: true, SourceMember: true, StatusCategory: "todo", QueueState: "paused"}
	c := CapabilitiesFor(p, s)
	if !c.Read || !c.Comment || !c.React || !c.ResolveThread || !c.Reorder || c.ControlAI || c.Edit || c.Transfer || c.Complete {
		t.Fatalf("source membership incorrectly grants collaboration management: %+v", c)
	}
	s.SourceMember, s.Participant = false, true
	c = CapabilitiesFor(p, s)
	if !c.Read || !c.ControlAI || !c.Leave || c.Reorder || c.Edit || c.Invite {
		t.Fatalf("external participant rights incorrect: %+v", c)
	}
	s.Participant = false
	if c = CapabilitiesFor(p, s); c != (Capabilities{}) {
		t.Fatalf("unrelated viewer has object rights: %+v", c)
	}
}

func TestMachineAndRestrictedIdentitiesCannotAcquireHumanRights(t *testing.T) {
	s := AccessState{AccountActive: true, SourceMember: true, Participant: true, Creator: true, Handler: true, StatusCategory: "todo", QueueState: "paused"}
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "plugin", "", "future_credential"} {
		c := CapabilitiesFor(Principal{UserID: "owner", Kind: kind}, s)
		if !c.Reference || c.Read || c.Comment || c.ControlAI || c.Edit || c.Complete || c.Reorder || c.React {
			t.Fatalf("machine/unknown kind %q inherited human authority: %+v", kind, c)
		}
	}
	if c := CapabilitiesFor(Principal{UserID: "owner", Kind: auth.CredentialJWT, Restricted: true}, s); c != (Capabilities{}) {
		t.Fatalf("restricted account acquired authority: %+v", c)
	}
	s.AccountActive = false
	if c := CapabilitiesFor(Principal{UserID: "owner", Kind: auth.CredentialJWT}, s); c != (Capabilities{}) {
		t.Fatalf("disabled account retained authority: %+v", c)
	}
}

func TestPrincipalRequiresAuthenticationContext(t *testing.T) {
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("missing auth context was interpreted as human")
	}
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: "account", Kind: auth.CredentialPAT})
	p, ok := PrincipalFromContext(ctx)
	if !ok || p.UserID != "account" || !p.Human() {
		t.Fatalf("authenticated PAT lost identity: %+v %v", p, ok)
	}
}

func TestLegacyAgentHintsOnlyRemoveHumanCollaborationAuthority(t *testing.T) {
	s := AccessState{AccountActive: true, SourceMember: true, Participant: true, Creator: true, Handler: true, StatusCategory: "todo", QueueState: "paused"}
	for _, kind := range []auth.CredentialKind{auth.CredentialJWT, auth.CredentialPAT, auth.CredentialTask, auth.CredentialCloud, ""} {
		ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: "owner", Kind: kind, AgentActor: true})
		p, ok := PrincipalFromContext(ctx)
		if !ok || p.Kind != kind || !p.AgentActor || p.Human() {
			t.Fatalf("legacy hint changed credential identity or granted human authority: %+v", p)
		}
		if c := CapabilitiesFor(p, s); c != (Capabilities{Reference: true}) {
			t.Fatalf("legacy actor inherited authority beyond a minimal reference: %+v", c)
		}
	}
}

func TestDirectedHandoffsFollowCurrentResponsibility(t *testing.T) {
	p := Principal{UserID: "handler", Kind: auth.CredentialJWT}
	s := AccessState{AccountActive: true, Participant: true, Handler: true, StatusCategory: "blocked", QueueState: "paused"}
	if c := CapabilitiesFor(p, s); !c.RequestInformation || !c.SubmitAcceptance || c.ReturnInformation {
		t.Fatalf("handler without a request to answer: %+v", c)
	}
	s.InformationRecipient = true
	if c := CapabilitiesFor(p, s); !c.ReturnInformation {
		t.Fatalf("current recipient cannot return the request: %+v", c)
	}
	// A recipient pointer alone grants nothing once responsibility has moved.
	s.Handler = false
	if c := CapabilitiesFor(p, s); c.ReturnInformation || c.RequestInformation || c.SubmitAcceptance {
		t.Fatalf("former handler kept directed handoffs: %+v", c)
	}
	s.Handler, s.Creator = true, true
	if c := CapabilitiesFor(p, s); c.SubmitAcceptance || !c.ReturnInformation {
		t.Fatalf("creator-handler acceptance or return mismatch: %+v", c)
	}
	s.Handler = false
	if c := CapabilitiesFor(p, s); !c.RequestInformation || c.ReturnInformation {
		t.Fatalf("creator without responsibility: %+v", c)
	}
	s.Creator, s.Handler, s.StatusCategory, s.QueueState = false, true, "done", "closed"
	if c := CapabilitiesFor(p, s); c.RequestInformation || c.ReturnInformation || c.SubmitAcceptance {
		t.Fatalf("ended issue allows handoffs: %+v", c)
	}
}
