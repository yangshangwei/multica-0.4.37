//go:build agentintegration

package collaborationrun

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// runHost reads the shared-context base with the run's business token after
// its permit and before launching the Provider, so the real-account smoke
// authorizer must admit that call like /context. This checks only the
// authorizer; it never resolves an agent CLI.
func TestHostProviderAccountSmokeAuthorizesSharedContextBase(t *testing.T) {
	fixture := &hostRunFixture{t: t, permits: map[string]protocol.CollaborationRunPermitRequest{"1": {}}}
	fixture.claim.Identity.RunID = "00000000-0000-4000-8000-000000000001"
	r := httptest.NewRequest("POST", "/api/collaboration/runtime/runs/"+fixture.claim.Identity.RunID+"/memory/base", strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer mcb_"+strings.Repeat("1", 64))
	if !authorizeHostAccountSmokeRequest(fixture, r) {
		t.Fatal("the real-account smoke refuses the shared-context base read that every host run makes before launching its Provider")
	}
	r.Header.Set("Authorization", "Bearer mcb_"+strings.Repeat("2", 64))
	if authorizeHostAccountSmokeRequest(fixture, r) {
		t.Fatal("the shared-context base read must require a business token from an issued permit")
	}
}
