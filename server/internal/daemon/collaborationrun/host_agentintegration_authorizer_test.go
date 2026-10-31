//go:build agentintegration

package collaborationrun

import (
	"net/http"
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

func TestHostProviderAccountSmokeAuthorizesOnlyNegotiatedClaimQueries(t *testing.T) {
	f := newHostRunFixture(t, "result")
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeHostAccountSmokeRequest(f, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		original.ServeHTTP(w, r)
	})
	if _, err := f.runner.Client.Claim(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatalf("current daemon claim negotiation was refused: %v", err)
	}
	for _, tc := range []struct {
		name, method, action, query, token string
		allowed                            bool
	}{
		{"legacy", "POST", "claim", "", "mdt_fixture", true},
		{"both", "POST", "claim", "handoff_version=1&recording_version=1", "mdt_fixture", true},
		{"handoff_only", "POST", "claim", "handoff_version=1", "mdt_fixture", true},
		{"unknown", "POST", "claim", "handoff_version=1&extra=1", "mdt_fixture", false},
		{"duplicate", "POST", "claim", "handoff_version=1&handoff_version=1", "mdt_fixture", false},
		{"newer_version", "POST", "claim", "handoff_version=2", "mdt_fixture", false},
		{"misplaced", "POST", "permit", "handoff_version=1", "mdt_fixture", false},
		{"wrong_method", "GET", "claim", "handoff_version=1", "mdt_fixture", false},
		{"wrong_token", "POST", "claim", "handoff_version=1", "mcb_" + strings.Repeat("1", 64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := runPath(f.claim.Identity.RunID, tc.action)
			if tc.query != "" {
				target += "?" + tc.query
			}
			r := httptest.NewRequest(tc.method, target, nil)
			r.Header.Set("Authorization", "Bearer "+tc.token)
			if got := authorizeHostAccountSmokeRequest(f, r); got != tc.allowed {
				t.Fatalf("authorized=%v want=%v", got, tc.allowed)
			}
		})
	}
}
