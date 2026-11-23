package service

import "testing"

func TestKnowledgeAutomationRuleOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, intent, support, reuse, relation, predecessor, want string
		refs                                                      []string
	}{
		{name: "new supported reusable", intent: "create", support: "supported", reuse: "reusable", relation: "novel", want: "accept"},
		{name: "new duplicate", intent: "create", support: "supported", reuse: "reusable", relation: "duplicate", refs: []string{"c1"}, want: "duplicate"},
		{name: "same exact predecessor", intent: "revalidate", support: "supported", reuse: "reusable", relation: "duplicate", predecessor: "c1", refs: []string{"c1"}, want: "accept"},
		{name: "authorized predecessor revision", intent: "revise", support: "supported", reuse: "reusable", relation: "conflict", predecessor: "c1", refs: []string{"c1"}, want: "accept"},
		{name: "predecessor and another conflict", intent: "revise", support: "supported", reuse: "reusable", relation: "conflict", predecessor: "c1", refs: []string{"c1", "c2"}, want: "review_pending"},
		{name: "unrelated duplicate during review", intent: "revalidate", support: "supported", reuse: "reusable", relation: "duplicate", predecessor: "c1", refs: []string{"c2"}, want: "review_pending"},
		{name: "unchanged conflicting predecessor", intent: "revalidate", support: "supported", reuse: "reusable", relation: "conflict", predecessor: "c1", refs: []string{"c1"}, want: "review_pending"},
		{name: "missing exact predecessor", intent: "revise", support: "supported", reuse: "reusable", relation: "novel", want: "review_pending"},
		{name: "unsupported summary or statement", intent: "create", support: "unsupported", reuse: "reusable", relation: "novel", want: "review_pending"},
		{name: "uncertain support", intent: "create", support: "uncertain", reuse: "reusable", relation: "novel", want: "review_pending"},
		{name: "transient conclusion", intent: "create", support: "supported", reuse: "transient", relation: "novel", want: "transient"},
		{name: "uncertain comparison", intent: "create", support: "supported", reuse: "reusable", relation: "uncertain", want: "review_pending"},
		{name: "unknown work intent", intent: "approve", support: "supported", reuse: "reusable", relation: "novel", want: "review_pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := knowledgeAutomationCandidate{Ref: "k1", Intent: tc.intent}
			check := knowledgeAutomationCheck{Ref: "k1", Support: tc.support, Reuse: tc.reuse, Relation: tc.relation, RelatedRefs: tc.refs, Reason: "PRIVATE MODEL REASON"}
			outcome, reason := knowledgeAutomationRuleOutcome(candidate, check, tc.predecessor)
			if outcome != tc.want || reason == "" || reason == check.Reason {
				t.Fatalf("outcome=%q reason=%q want=%q", outcome, reason, tc.want)
			}
		})
	}
}

func TestKnowledgeAutomationContentNormalizationPreservesMeaningfulBytes(t *testing.T) {
	input := " Never trim  SPACE.\r\nKeep Case, NOT and 12.\rFinal line. "
	want := " Never trim  SPACE.\nKeep Case, NOT and 12.\nFinal line. "
	if got := normalizeKnowledgeAutomationContent(input); got != want {
		t.Fatalf("normalization changed semantic bytes: %q", got)
	}
}
