package collaboration

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
)

func TestKnowledgeHumanSourceAndTargetAuthorityAreIndependent(t *testing.T) {
	p := Principal{UserID: "human", Kind: auth.CredentialJWT}
	s := AccessState{AccountActive: true, Participant: true, Handler: true, StatusCategory: "done", QueueState: "closed"}
	target := KnowledgeTargetAuthority{Kind: "workspace", Exists: true, SourceWorkspace: true}
	c := KnowledgeCapabilitiesFor(p, s, target, "pending", true, false)
	if !c.Propose || c.Approve || !c.Withdraw {
		t.Fatalf("external completed handler: %+v", c)
	}
	s.Handler = false
	if KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Propose {
		t.Fatal("comment/control permission became proposal authority")
	}
	target.Member = true
	target.MemberRole = "admin"
	c = KnowledgeCapabilitiesFor(p, s, target, "pending", true, false)
	if c.Propose || !c.Approve {
		t.Fatalf("confirmer separation: %+v", c)
	}
	s.Participant = false
	c = KnowledgeCapabilitiesFor(p, s, target, "approved", false, false)
	if c.Read || c.Approve || !c.Withdraw {
		t.Fatalf("target-only withdrawal must be content-free: %+v", c)
	}
	for _, mutate := range []func(*Principal){func(p *Principal) { p.AgentActor = true }, func(p *Principal) { p.Restricted = true }, func(p *Principal) { p.Kind = auth.CredentialTask }} {
		bad := p
		mutate(&bad)
		if got := KnowledgeCapabilitiesFor(bad, s, target, "approved", true, false); got != (KnowledgeCapabilities{}) {
			t.Fatalf("machine/restricted capability: %+v", got)
		}
	}
}

func TestKnowledgeSourceMemberCreatorWithoutParticipationCanManage(t *testing.T) {
	p := Principal{UserID: "human", Kind: auth.CredentialJWT}
	s := AccessState{AccountActive: true, SourceMember: true, Creator: true}
	target := KnowledgeTargetAuthority{Kind: "workspace", Exists: true, SourceWorkspace: true}
	for _, state := range []string{"pending", "approved"} {
		c := KnowledgeCapabilitiesFor(p, s, target, state, true, false)
		if !c.Propose || !c.Withdraw || !c.Read || c.Approve {
			t.Fatalf("source Read plus creator must suffice without participation (%s): %+v", state, c)
		}
	}
	s.SourceMember = false
	c := KnowledgeCapabilitiesFor(p, s, target, "approved", true, false)
	if c.Propose || c.Withdraw || c.Read {
		t.Fatalf("creator identity without a current read basis granted authority: %+v", c)
	}
}

func TestKnowledgeConfirmerCanRejectUnavailableOrExpiredPendingWithoutApproval(t *testing.T) {
	p := Principal{UserID: "human", Kind: auth.CredentialPAT}
	s := AccessState{AccountActive: true, SourceMember: true}
	target := KnowledgeTargetAuthority{Kind: "workspace", Exists: true, Member: true, MemberRole: "admin"}
	c := KnowledgeCapabilitiesFor(p, s, target, "pending", false, false)
	if !c.Reject || c.Approve || c.Read {
		t.Fatalf("unavailable pending rejection exposed content or approved it: %+v", c)
	}
	c = KnowledgeCapabilitiesFor(p, s, target, "pending", true, true)
	if !c.Reject || c.Approve {
		t.Fatalf("expired pending proposal could not be rejected safely: %+v", c)
	}
	s.SourceMember = false
	if c := KnowledgeCapabilitiesFor(p, s, target, "pending", false, true); c.Reject || c.Read {
		t.Fatalf("target-only confirmation role gained rejection/read authority: %+v", c)
	}
}

func TestKnowledgeProjectLeadAndProposalTargetPolicy(t *testing.T) {
	p := Principal{UserID: "human", Kind: auth.CredentialPAT}
	s := AccessState{AccountActive: true, Participant: true, Creator: true}
	target := KnowledgeTargetAuthority{Kind: "project", Exists: true, CurrentSourceProject: true, SourceProjectVisible: true}
	if !KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Propose {
		t.Fatal("visible current source project refused")
	}
	target.CurrentSourceProject = false
	if KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Propose {
		t.Fatal("stale current project accepted")
	}
	target.NormallyVisible = true
	target.Member = true
	target.LeadType = "agent"
	target.LeadIsActor = true
	if KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Approve {
		t.Fatal("agent lead approved")
	}
	target.LeadType = "member"
	if !KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Approve {
		t.Fatal("human member lead refused")
	}
	for _, c := range []KnowledgeCapabilities{KnowledgeCapabilitiesFor(p, s, target, "pending", false, false), KnowledgeCapabilitiesFor(p, s, target, "pending", true, true)} {
		if c.Approve {
			t.Fatal("unavailable/expired proposal approved")
		}
	}
	target.Exists = false
	if KnowledgeCapabilitiesFor(p, s, target, "pending", true, false).Propose {
		t.Fatal("deleted target proposed")
	}
}
