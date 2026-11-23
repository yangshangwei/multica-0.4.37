package collaborationrun

import (
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A squad member run receives the group's root human inputs, so its prompt
// must carry the claim's role and step and say that the member's task is the
// leader's delegation, not the whole request. Other roles keep the plain text.
func TestHostPromptTellsAMemberRunItsRoleAndStep(t *testing.T) {
	parent := "00000000-0000-4000-8000-0000000000aa"
	claim := protocol.CollaborationRunClaimResponse{AgentID: "member-agent",
		HostContext: &protocol.CollaborationHostContext{Role: "member", StepKey: "dispatch-1", ParentRunID: &parent},
		Preparation: &protocol.CollaborationPreparation{},
		Inputs:      []protocol.CollaborationRunInput{{CommandID: "root-command", Body: "Ship the whole feature"}}}
	prompt, err := hostPrompt(claim, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"role":"member"`, `"step_key":"dispatch-1"`, "dispatch_comment"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("member prompt omits %s, so the Provider cannot tell a delegated step from the root request:\n%s", want, prompt)
		}
	}
	claim.HostContext = &protocol.CollaborationHostContext{Role: "leader", StepKey: "root"}
	if prompt, err = hostPrompt(claim, false, nil); err != nil || strings.Contains(prompt, "dispatch_comment") || !strings.Contains(prompt, `"role":"leader"`) {
		t.Fatalf("leader prompt must carry its role without the member delegation note: err=%v\n%s", err, prompt)
	}
}
