package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientHostContextExtensionsAreExplicitAndVersioned(t *testing.T) {
	f := newHostRunFixture(t, "success")
	for _, tc := range []struct {
		name      string
		handoff   int
		recording int
		wantError bool
	}{
		{name: "older server"},
		{name: "supported", handoff: 1, recording: 1},
		{name: "handoff only", handoff: 1},
		{name: "recording only", recording: 1},
		{name: "unknown handoff", handoff: 2, wantError: true},
		{name: "unknown recording", recording: 2, wantError: true},
		{name: "negative version", recording: -1, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("handoff_version") != "1" || r.URL.Query().Get("recording_version") != "1" {
					t.Error("host did not explicitly negotiate context extensions")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["handoff_version"] != nil || body["recording_version"] != nil {
					t.Error("extensions changed the strict legacy claim body")
				}
				claim := f.claim
				claim.HandoffVersion, claim.RecordingVersion = tc.handoff, tc.recording
				_ = json.NewEncoder(w).Encode(claim)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Claim(context.Background(), f.claim.Identity.RunID, f.request)
			if (err != nil) != tc.wantError {
				t.Fatalf("claim error=%v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

func TestClientNamespaceCannotAcquireHostContextExtensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Error("namespace claim requested host-only extensions")
		}
		claim := testClaim()
		claim.HandoffVersion, claim.RecordingVersion = 1, 1
		_ = json.NewEncoder(w).Encode(claim)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Claim(context.Background(), testIdentity().RunID, testClaimRequest()); err == nil {
		t.Fatal("namespace claim acquired host-only features")
	}
}
