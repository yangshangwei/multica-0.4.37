package collaboration

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/auth"
)

func stageRoles() StageRoles {
	return StageRoles{BusinessOwner: "a", Implementer: "b", ProductApprover: "a", CodeReviewer: "c", QAApprover: "d", ReleaseApprover: "e"}
}

func TestStageRolesAllowExplicitOverlapButSeparateReviewAndRelease(t *testing.T) {
	if code := ValidateStageRoles(stageRoles()); code != "" {
		t.Fatalf("valid configuration with an explicit shared holder refused: %s", code)
	}
	everyone := StageRoles{BusinessOwner: "a", Implementer: "a", ProductApprover: "a", CodeReviewer: "b", QAApprover: "a", ReleaseApprover: "b"}
	if code := ValidateStageRoles(everyone); code != "" {
		t.Fatalf("explicit overlap outside the two separations refused: %s", code)
	}
	self := stageRoles()
	self.CodeReviewer = self.Implementer
	if code := ValidateStageRoles(self); code != "stage_self_review_forbidden" {
		t.Fatalf("implementer may review their own code: %q", code)
	}
	merged := stageRoles()
	merged.ReleaseApprover = merged.QAApprover
	if code := ValidateStageRoles(merged); code != "stage_qa_release_conflict" {
		t.Fatalf("QA and release approval were not separated: %q", code)
	}
	for name, mutate := range map[string]func(*StageRoles){
		"business_owner": func(r *StageRoles) { r.BusinessOwner = "" }, "implementer": func(r *StageRoles) { r.Implementer = "" },
		"product": func(r *StageRoles) { r.ProductApprover = "" }, "reviewer": func(r *StageRoles) { r.CodeReviewer = "" },
		"qa": func(r *StageRoles) { r.QAApprover = "" }, "release": func(r *StageRoles) { r.ReleaseApprover = "" },
	} {
		roles := stageRoles()
		mutate(&roles)
		if code := ValidateStageRoles(roles); code != "invalid_stage_request" {
			t.Fatalf("missing %s accepted: %q", name, code)
		}
	}
	if holders := stageRoles().Holders(); strings.Join(holders, ",") != "a,b,c,d,e" {
		t.Fatalf("holders are not the distinct sorted set: %v", holders)
	}
}

func TestStageProposalContentBoundsAndStableDigest(t *testing.T) {
	content := StageProposalContent{Summary: "split", Children: []StageProposalChild{{Title: "  First <task>  ", Description: "body", Priority: "high"}}}
	normalized, err := NormalizeStageProposal(content)
	if err != nil || normalized.Children[0].Title != "First <task>" {
		t.Fatalf("valid proposal refused or title not trimmed: %+v %v", normalized, err)
	}
	canonical, digest, err := StageProposalDigest(normalized)
	if err != nil || len(digest) != 64 || !strings.Contains(string(canonical), "First <task>") || strings.HasSuffix(string(canonical), "\n") {
		t.Fatalf("canonical content is not stable JSON: %q %q %v", canonical, digest, err)
	}
	_, again, _ := StageProposalDigest(normalized)
	changed := normalized
	changed.Children = []StageProposalChild{{Title: "First <task>", Description: "body.", Priority: "high"}}
	_, other, _ := StageProposalDigest(changed)
	if again != digest || other == digest {
		t.Fatal("digest is unstable or ignores child content")
	}
	many := StageProposalContent{}
	for range StageProposalMaxChildren {
		many.Children = append(many.Children, StageProposalChild{Title: "child", Priority: "none"})
	}
	if _, err = NormalizeStageProposal(many); err != nil {
		t.Fatalf("twenty children refused: %v", err)
	}
	many.Children = append(many.Children, StageProposalChild{Title: "one too many", Priority: "none"})
	for name, bad := range map[string]StageProposalContent{
		"none":        {},
		"too_many":    many,
		"blank_title": {Children: []StageProposalChild{{Title: " \t", Priority: "none"}}},
		"long_title":  {Children: []StageProposalChild{{Title: strings.Repeat("字", StageProposalTitleMaxRunes+1), Priority: "none"}}},
		"priority":    {Children: []StageProposalChild{{Title: "a", Priority: "soon"}}},
		"no_priority": {Children: []StageProposalChild{{Title: "a"}}},
		"description": {Children: []StageProposalChild{{Title: "a", Priority: "none", Description: strings.Repeat("x", StageProposalDescriptionMaxBytes+1)}}},
		"summary":     {Summary: strings.Repeat("x", StageProposalSummaryMaxBytes+1), Children: []StageProposalChild{{Title: "a", Priority: "none"}}},
		"nul":         {Children: []StageProposalChild{{Title: "a\x00b", Priority: "none"}}},
		"utf8":        {Children: []StageProposalChild{{Title: "a", Priority: "none", Description: string([]byte{0xff})}}},
	} {
		if _, err := NormalizeStageProposal(bad); err == nil {
			t.Fatalf("invalid proposal %s accepted", name)
		}
	}
	if StageChildContentDigest("a", "b") == StageChildContentDigest("ab", "") || StageChildContentDigest("a", "b") != StageChildContentDigest("a", "b") {
		t.Fatal("child content digest is ambiguous or unstable")
	}
}

func TestStageScopeStateNeedsExplicitCurrentConfirmation(t *testing.T) {
	base := StageScopeFacts{Managed: true, RoleRevision: 1, ScopeDecision: true, ScopeLifecycle: 3, RootLifecycle: 3,
		ScopePolicyRevision: 4, PolicyLastDisabled: 2, SignerCurrent: true}
	if StageScopeState(base) != "current" || len(StageMissing(base)) != 0 {
		t.Fatalf("current confirmation not recognised: %s %v", StageScopeState(base), StageMissing(base))
	}
	for name, tc := range map[string]struct {
		mutate  func(*StageScopeFacts)
		state   string
		missing string
	}{
		"first_enable":   {func(f *StageScopeFacts) { f.ScopeDecision, f.RoleRevision = false, 0 }, "none", "roles,scope"},
		"roles_only":     {func(f *StageScopeFacts) { f.RoleRevision = 0 }, "current", "roles"},
		"reopened":       {func(f *StageScopeFacts) { f.RootLifecycle = 5 }, "lifecycle_changed", "scope"},
		"re_enabled":     {func(f *StageScopeFacts) { f.PolicyLastDisabled = 4 }, "policy_changed", "scope"},
		"signer_changed": {func(f *StageScopeFacts) { f.SignerCurrent = false }, "signer_changed", "scope"},
	} {
		facts := base
		tc.mutate(&facts)
		if StageScopeState(facts) != tc.state || strings.Join(StageMissing(facts), ",") != tc.missing {
			t.Fatalf("%s: state %s missing %v", name, StageScopeState(facts), StageMissing(facts))
		}
	}
	unmanaged := base
	unmanaged.Managed, unmanaged.RoleRevision, unmanaged.ScopeDecision = false, 0, false
	if len(StageMissing(unmanaged)) != 0 {
		t.Fatal("an unmanaged root reported missing stage facts")
	}
}

func TestStageChildExecutionOnlyGatesMachineOriginUnderManagedRoot(t *testing.T) {
	for name, tc := range map[string]struct {
		facts StageChildFacts
		code  string
	}{
		"unmanaged":            {StageChildFacts{MachineOrigin: true}, ""},
		"human_child":          {StageChildFacts{Managed: true, RootLifecycle: 2}, ""},
		"human_stale_mapping":  {StageChildFacts{Managed: true, Mapped: true, ApprovedLifecycle: 1, RootLifecycle: 2}, ""},
		"machine_unmapped":     {StageChildFacts{Managed: true, MachineOrigin: true, RootLifecycle: 1}, "stage_child_unapproved"},
		"machine_current":      {StageChildFacts{Managed: true, MachineOrigin: true, Mapped: true, ApprovedLifecycle: 2, RootLifecycle: 2}, ""},
		"machine_after_reopen": {StageChildFacts{Managed: true, MachineOrigin: true, Mapped: true, ApprovedLifecycle: 1, RootLifecycle: 3}, "stage_scope_unconfirmed"},
	} {
		if code := StageChildExecutionRefusal(tc.facts); code != tc.code {
			t.Fatalf("%s: %q", name, code)
		}
	}
}

func TestStageMachineCreationUsesVerifiedCredentialNotSelfReport(t *testing.T) {
	human := Principal{UserID: "u", Kind: auth.CredentialJWT}
	for name, tc := range map[string]struct {
		creator       string
		principal     Principal
		authenticated bool
		machine       bool
	}{
		"human":                  {"member", human, true, false},
		"pat":                    {"member", Principal{UserID: "u", Kind: auth.CredentialPAT}, true, false},
		"internal_human_channel": {"member", Principal{}, false, false},
		"agent_creator":          {"agent", human, true, true},
		"task_token":             {"member", Principal{UserID: "u", Kind: auth.CredentialTask}, true, true},
		"agent_headers":          {"member", Principal{UserID: "u", Kind: auth.CredentialJWT, AgentActor: true}, true, true},
		"restricted":             {"member", Principal{UserID: "u", Kind: auth.CredentialJWT, Restricted: true}, true, true},
		"internal_agent":         {"agent", Principal{}, false, true},
	} {
		if got := StageMachineCreation(tc.creator, tc.principal, tc.authenticated); got != tc.machine {
			t.Fatalf("%s: machine=%v", name, got)
		}
	}
}
