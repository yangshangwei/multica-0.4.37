package taskfailure

import "testing"

func TestKnowledgeRefusalCodesStayTerminalAcrossNormalization(t *testing.T) {
	for _, reason := range []Reason{ReasonKnowledgeContextChanged, ReasonKnowledgeAuthorizationDenied, ReasonKnowledgeBusy, ReasonKnowledgeUnavailable, ReasonKnowledgeInvocationUnsupported} {
		if !reason.IsKnowledgeRefusal() || reason.IsAgentError() || Classify(reason.String()) != reason || NormalizeDaemonReason(ReasonAgentProviderNetwork.String(), reason.String()) != reason || NormalizeDaemonReason(reason.String(), "network timeout") != reason {
			t.Fatalf("knowledge refusal lost its platform classification: %s", reason)
		}
	}
	if Classify("provider said: knowledge_context_changed").IsKnowledgeRefusal() {
		t.Fatal("untrusted prose became a platform refusal code")
	}
}
