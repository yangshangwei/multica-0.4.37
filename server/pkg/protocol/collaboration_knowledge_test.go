package protocol

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const knowledgeTestID = "11111111-1111-4111-8111-111111111111"

func knowledgeProposal(t *testing.T) CollaborationKnowledgeProposal {
	t.Helper()
	return CollaborationKnowledgeProposal{
		ProtocolVersion: 1, CandidateID: knowledgeTestID, RequestID: knowledgeTestID,
		Target:  CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: knowledgeTestID},
		Summary: "A reviewed finding", Content: "Immutable material",
		Sources:       []CollaborationKnowledgeSource{{Kind: "memory", IssueID: knowledgeTestID, Version: "7", SHA256: strings.Repeat("a", 64)}},
		Evidence:      []CollaborationKnowledgeEvidence{},
		Applicability: CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat"}},
	}
}

func TestKnowledgeProposalStrictBoundary(t *testing.T) {
	p := knowledgeProposal(t)
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CollaborationKnowledgeProposal
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(string) string{
		"unknown": func(s string) string {
			return strings.Replace(s, `"protocol_version":1`, `"private":"sentinel","protocol_version":1`, 1)
		},
		"duplicate": func(s string) string {
			return strings.Replace(s, `"protocol_version":1`, `"protocol_version":1,"protocol_version":1`, 1)
		},
		"alias":            func(s string) string { return strings.Replace(s, `"protocol_version"`, `"Protocol_Version"`, 1) },
		"missing nullable": func(s string) string { return strings.Replace(s, `,"expires_at":null`, "", 1) },
		"null required":    func(s string) string { return strings.Replace(s, `"evidence":[]`, `"evidence":null`, 1) },
		"nested unknown": func(s string) string {
			return strings.Replace(s, `"kind":"memory"`, `"kind":"memory","body":"sentinel"`, 1)
		},
		"invalid utf8": func(s string) string { return strings.Replace(s, "Immutable", string([]byte{0xff}), 1) },
		"nul":          func(s string) string { return strings.Replace(s, "Immutable", `\u0000`, 1) },
	} {
		t.Run(name, func(t *testing.T) {
			var got CollaborationKnowledgeProposal
			if err := json.Unmarshal([]byte(change(string(raw))), &got); err == nil {
				t.Fatal("malformed proposal accepted")
			}
		})
	}
}

func TestKnowledgeProposalIdentityAndBudgets(t *testing.T) {
	for name, change := range map[string]func(*CollaborationKnowledgeProposal){
		"protocol":             func(p *CollaborationKnowledgeProposal) { p.ProtocolVersion = 2 },
		"uuid":                 func(p *CollaborationKnowledgeProposal) { p.CandidateID = "not-a-uuid" },
		"request":              func(p *CollaborationKnowledgeProposal) { p.RequestID = "old-non-uuid-key" },
		"overflow version":     func(p *CollaborationKnowledgeProposal) { p.Sources[0].Version = "9223372036854775808" },
		"noncanonical version": func(p *CollaborationKnowledgeProposal) { p.Sources[0].Version = "07" },
		"hash":                 func(p *CollaborationKnowledgeProposal) { p.Sources[0].SHA256 = strings.Repeat("A", 64) },
		"mixed source": func(p *CollaborationKnowledgeProposal) {
			p.Sources = append(p.Sources, CollaborationKnowledgeSource{Kind: "memory", IssueID: "22222222-2222-4222-8222-222222222222", Version: "1", SHA256: strings.Repeat("a", 64)})
		},
		"target":            func(p *CollaborationKnowledgeProposal) { p.Target.Kind = "project" },
		"cross-kind source": func(p *CollaborationKnowledgeProposal) { p.Sources[0].RunID = knowledgeTestID },
		"empty sources":     func(p *CollaborationKnowledgeProposal) { p.Sources = nil },
		"too many refs": func(p *CollaborationKnowledgeProposal) {
			for len(p.Sources) <= CollaborationKnowledgeMaxReferences {
				p.Sources = append(p.Sources, p.Sources[0])
			}
		},
		"body": func(p *CollaborationKnowledgeProposal) {
			p.Content = strings.Repeat("x", CollaborationKnowledgeMaxContentBytes+1)
		},
		"summary": func(p *CollaborationKnowledgeProposal) {
			p.Summary = strings.Repeat("x", CollaborationKnowledgeMaxSummaryBytes+1)
		},
		"empty summary": func(p *CollaborationKnowledgeProposal) { p.Summary = "  " },
		"metadata": func(p *CollaborationKnowledgeProposal) {
			for i := 0; i < 20; i++ {
				p.Evidence = append(p.Evidence, CollaborationKnowledgeEvidence{Kind: "test", Source: &p.Sources[0], Reference: strings.Repeat("x", 1000), Verification: "observed"})
			}
		},
		"surface": func(p *CollaborationKnowledgeProposal) { p.Applicability.Surfaces = []string{"skill"} },
	} {
		t.Run(name, func(t *testing.T) {
			p := knowledgeProposal(t)
			change(&p)
			if ValidateCollaborationKnowledgeProposal(p, knowledgeTestID) == nil {
				t.Fatal("invalid proposal accepted")
			}
		})
	}
	p := knowledgeProposal(t)
	p.Content = strings.Repeat("\x01", CollaborationKnowledgeMaxContentBytes)
	p.Summary = strings.Repeat("界", 682) + "xx"
	if err := ValidateCollaborationKnowledgeProposal(p, knowledgeTestID); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	if len(raw) <= CollaborationKnowledgeMaxContentBytes {
		t.Fatal("fixture did not exercise escaped wire")
	}
	var decoded CollaborationKnowledgeProposal
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Content != p.Content || CollaborationKnowledgeHash(decoded.Content) != CollaborationKnowledgeHash(p.Content) {
		t.Fatal("reviewed content changed")
	}
}

func TestKnowledgeApprovalDecisionAndPublicHint(t *testing.T) {
	approve := `{"protocol_version":1,"request_id":"` + knowledgeTestID + `","expected_state_revision":"1","expected_version":"9223372036854775807","expected_content_sha256":"` + strings.Repeat("a", 64) + `"}`
	var a CollaborationKnowledgeApproveRequest
	if err := json.Unmarshal([]byte(approve), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(strings.Replace(approve, `"expected_version":"9223372036854775807",`, "", 1)), &a); !errors.Is(err, ErrCollaborationKnowledgeRevisionRequired) {
		t.Fatalf("missing review observation must map to 428: %v", err)
	}
	for _, raw := range []string{strings.Replace(approve, `"expected_version":"9223372036854775807",`, "", 1), strings.Replace(approve, `"1"`, `"0"`, 1), strings.Replace(approve, `"protocol_version":1`, `"protocol_version":null`, 1)} {
		if json.Unmarshal([]byte(raw), &a) == nil {
			t.Fatal("incomplete approval accepted")
		}
	}
	var d CollaborationKnowledgeDecisionRequest
	if json.Unmarshal([]byte(`{"protocol_version":1,"request_id":"`+knowledgeTestID+`","expected_state_revision":"1","reason":"reviewed"}`), &d) != nil {
		t.Fatal("valid decision rejected")
	}
	if json.Unmarshal([]byte(`{"protocol_version":1,"request_id":"`+knowledgeTestID+`","expected_state_revision":"1","reason":" "}`), &d) == nil {
		t.Fatal("blank decision accepted")
	}
	hint := `{"target_kind":"workspace","target_id":"` + knowledgeTestID + `","knowledge_revision":"0"}`
	var h CollaborationKnowledgeTargetHint
	if err := json.Unmarshal([]byte(hint), &h); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal([]byte(strings.Replace(hint, `"target_kind"`, `"candidate_id":"`+knowledgeTestID+`","target_kind"`, 1)), &h) == nil {
		t.Fatal("private hint field accepted")
	}
}

func TestKnowledgeCompleteMaterialAndWireBudgets(t *testing.T) {
	p := knowledgeProposal(t)
	item := CollaborationKnowledgeContribution{CandidateID: knowledgeTestID, Version: "1", ContentSHA256: CollaborationKnowledgeHash(p.Content), Target: p.Target, SourceIssueID: knowledgeTestID, Summary: p.Summary, Content: p.Content, Sources: p.Sources, Evidence: p.Evidence, Applicability: p.Applicability}
	items := make([]CollaborationKnowledgeContribution, 16)
	for i := range items {
		items[i] = item
	}
	if err := ValidateCollaborationKnowledgeTargetBudget(items); err != nil {
		t.Fatal(err)
	}
	if ValidateCollaborationKnowledgeTargetBudget(append(items, item)) == nil {
		t.Fatal("17 contributions accepted")
	}
	items = items[:2]
	items[0].Content = strings.Repeat("x", 64<<10)
	items[1].Content = items[0].Content
	if ValidateCollaborationKnowledgeTargetBudget(items) == nil {
		t.Fatal("metadata was not charged on top of 128 KiB bodies")
	}
	items = items[:1]
	items[0].Content = strings.Repeat("\x01", 64<<10)
	material, wire, err := CollaborationKnowledgeSizes(items)
	if err != nil || material >= 128<<10 || wire <= 256<<10 {
		t.Fatalf("material=%d wire=%d err=%v", material, wire, err)
	}
	if ValidateCollaborationKnowledgeRuntimeBudget(items, CollaborationKnowledgeMaxRuntimeJSONBytes+1) == nil {
		t.Fatal("raw wire excess accepted")
	}
}

func TestKnowledgeExactBudgetEdges(t *testing.T) {
	p := knowledgeProposal(t)
	item := CollaborationKnowledgeContribution{CandidateID: knowledgeTestID, Version: "1", ContentSHA256: CollaborationKnowledgeHash(p.Content), Target: p.Target, SourceIssueID: knowledgeTestID, Summary: p.Summary, Content: p.Content, Sources: p.Sources, Evidence: p.Evidence, Applicability: p.Applicability}
	items := []CollaborationKnowledgeContribution{item, item}
	material, _, err := CollaborationKnowledgeSizes(items)
	if err != nil {
		t.Fatal(err)
	}
	extra := CollaborationKnowledgeMaxTargetBytes - material
	items[0].Content += strings.Repeat("x", extra/2)
	items[1].Content += strings.Repeat("x", extra-extra/2)
	if err := ValidateCollaborationKnowledgeTargetBudget(items); err != nil {
		t.Fatal("exact complete target limit refused:", err)
	}
	items[1].Content += "x"
	if ValidateCollaborationKnowledgeTargetBudget(items) == nil {
		t.Fatal("one byte over complete target limit accepted")
	}
	p = knowledgeProposal(t)
	for len(p.Sources) < CollaborationKnowledgeMaxReferences {
		p.Sources = append(p.Sources, p.Sources[0])
	}
	if err := ValidateCollaborationKnowledgeProposal(p, knowledgeTestID); err != nil {
		t.Fatal("exact reference limit refused:", err)
	}
	if CollaborationKnowledgeMaxRequestBytes != 520192 {
		t.Fatalf("HTTP escape envelope changed: %d", CollaborationKnowledgeMaxRequestBytes)
	}
	raw, _ := json.Marshal(knowledgeProposal(t))
	raw = append(raw, []byte(strings.Repeat(" ", CollaborationKnowledgeMaxRequestBytes-len(raw)))...)
	var got CollaborationKnowledgeProposal
	// Direct decoder invocation checks the HTTP boundary including trailing
	// JSON whitespace, which encoding/json strips before calling UnmarshalJSON.
	if err := got.UnmarshalJSON(raw); err != nil {
		t.Fatal("exact HTTP budget refused:", err)
	}
	if got.UnmarshalJSON(append(raw, ' ')) == nil {
		t.Fatal("HTTP limit+1 accepted")
	}
	item.Content = string([]byte{0xff})
	if _, _, err := CollaborationKnowledgeSizes([]CollaborationKnowledgeContribution{item}); err == nil {
		t.Fatal("budget silently repaired invalid UTF-8")
	}
}

func TestKnowledgeProposalExpiryPreservesOriginalDecode(t *testing.T) {
	p := knowledgeProposal(t)
	expires := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	p.ExpiresAt = &expires
	raw, _ := json.Marshal(p)
	var got CollaborationKnowledgeProposal
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal("expired original intent cannot be decoded for recovery:", err)
	}
	if ValidateCollaborationKnowledgeProposalAt(got, knowledgeTestID, expires) == nil {
		t.Fatal("new expired proposal accepted")
	}
	if err := ValidateCollaborationKnowledgeProposalAt(got, knowledgeTestID, expires.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
}
