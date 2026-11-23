package collaborationrun

import (
	"github.com/multica-ai/multica/server/pkg/protocol"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostCodexKnowledgeRequiresEphemeralBeforePrivateTurn(t *testing.T) {
	for _, mode := range []string{"knowledge_ephemeral", "knowledge_persistent", "knowledge_missing"} {
		t.Run(mode, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Setenv("COLLAB_HOST_TEST_MODE", "")
			t.Setenv("COLLAB_HANDOFF_CODEX_MODE", mode)
			t.Setenv("COLLAB_HANDOFF_CODEX_COUNTER", filepath.Join(t.TempDir(), "attempts"))
			threadPath := filepath.Join(t.TempDir(), "thread.json")
			t.Setenv("COLLAB_KNOWLEDGE_CODEX_THREAD", threadPath)
			h.runner.Host.Config.Provider = "codex"
			caps, err := h.runner.Host.Config.Capabilities()
			if err != nil {
				t.Fatal(err)
			}
			digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
			if err != nil {
				t.Fatal(err)
			}
			h.request.CapabilityDigest = digest
			h.claim.Preparation.Host, h.claim.Preparation.Provider, h.claim.Preparation.CapabilityDigest = caps.Host, caps.Provider, digest
			if err = protocol.SealCollaborationPreparation(h.claim.Preparation); err != nil {
				t.Fatal(err)
			}
			f := attachHostKnowledgeFixture(t, h)
			err = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)

			if _, statErr := os.Stat(threadPath); statErr != nil {
				t.Fatal("Codex never received ephemeral thread/start", statErr)
			}
			if len(f.authorizations) != 1 {
				t.Fatalf("authorization count=%d", len(f.authorizations))
			}
			if mode != "knowledge_ephemeral" {
				raw, readErr := os.ReadFile(f.capture)
				if readErr != nil || len(raw) != 0 {
					t.Fatal("unconfirmed ephemeral thread received private input")
				}
				if len(f.permits) != 1 {
					t.Fatal("unsupported persistence retried")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			turns := f.starts(t)
			if len(turns) != 1 {
				t.Fatalf("actual turns=%d", len(turns))
			}
			prompt := turns[0]["prompt"].(string)
			if !strings.Contains(prompt, f.contribution.Content) || f.authorizations[0].PromptSHA256 != protocol.CollaborationHandoffHash(prompt) {
				t.Fatal("actual Codex input lost authorized knowledge binding")
			}
			key, err := hostSessionKey(f.claim)
			if err != nil {
				t.Fatal(err)
			}
			session, err := f.runner.Ledger.lockHostSession(key, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if session.record.SessionID != "" {
				t.Fatal("ephemeral knowledge became resumable")
			}
		})
	}
}
