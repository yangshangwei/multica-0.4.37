package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func testClaimRequest() protocol.CollaborationRunClaimRequest {
	return protocol.CollaborationRunClaimRequest{ProtocolVersion: 2, DeploymentID: testIdentity().DeploymentID, InstallationID: testIdentity().InstallationID, RuntimeID: testIdentity().TargetID, BindingID: testIdentity().GroupID, BindingEpoch: "1", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
}

func testClaim() protocol.CollaborationRunClaimResponse {
	return protocol.CollaborationRunClaimResponse{Identity: testIdentity(), Inputs: []protocol.CollaborationRunInput{
		{CommandID: testIdentity().IssueID, AuthorUserID: testIdentity().DeploymentID, ParticipantEpoch: "1", AccountEpoch: "1", LifecycleEpoch: "1", Body: "first immutable input", AttachmentIDs: []string{}},
		{CommandID: testIdentity().SessionID, AuthorUserID: testIdentity().InstallationID, ParticipantEpoch: "2", AccountEpoch: "3", LifecycleEpoch: "1", Body: "second immutable input", AttachmentIDs: []string{}},
	}, AgentID: testIdentity().TargetID, ResourceOwnerID: testIdentity().SessionID, ExecutionWorkspaceID: testIdentity().BatchID, TerminationToken: "termination-only"}
}

func TestClientPreservesAllInputAuthorsAndNarrowCredentials(t *testing.T) {
	var methods []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("Cookie") != "" {
			t.Error("ambient authorization leaked")
		}
		methods = append(methods, r.Method)
		if strings.HasSuffix(r.URL.Path, "/claim") {
			if r.Header.Get("Authorization") != "Bearer mdt_test" {
				t.Error("claim did not use managed credential")
			}
			_ = json.NewEncoder(w).Encode(testClaim())
		} else if strings.HasSuffix(r.URL.Path, "/stop-intent") {
			if r.Header.Get("Authorization") != "Bearer termination-only" {
				t.Error("stop used business or managed credential")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{Identity: testIdentity(), StopRequested: true})
		} else {
			t.Error("unexpected path")
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	c, err := NewClient(s.URL, "mdt_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := c.Claim(context.Background(), testIdentity().RunID, testClaimRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(claim.Inputs) != 2 || claim.Inputs[1].AuthorUserID == claim.Inputs[0].AuthorUserID || claim.Inputs[1].ParticipantEpoch != "2" {
		t.Fatal("batch lost a human authorization")
	}
	stop, err := c.StopIntent(context.Background(), testIdentity(), claim.TerminationToken)
	if err != nil || !stop.StopRequested {
		t.Fatalf("stop=%v err=%v", stop.StopRequested, err)
	}
	if len(methods) != 2 {
		t.Fatal("unexpected retry")
	}
}

func TestClientRejectsBadSecondAuthorAndDuplicateIdentityField(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claim := testClaim()
			claim.Inputs[1].AccountEpoch = "0"
			if duplicate {
				claim.Inputs[1].AccountEpoch = "3"
			}
			b, _ := json.Marshal(claim)
			if duplicate {
				b = []byte(strings.Replace(string(b), `"fence":"1"`, `"fence":"2","fence":"1"`, 1))
			}
			_, _ = w.Write(b)
		}))
		c, err := NewClient(s.URL, "mdt_test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Claim(context.Background(), testIdentity().RunID, testClaimRequest()); err == nil {
			t.Fatal("accepted corrupt or ambiguous claim")
		}
		s.Close()
	}
}

func TestClientNeverForwardsTokensAcrossRedirects(t *testing.T) {
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	c, err := NewClient(source.URL, "mdt_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Claim(context.Background(), testIdentity().RunID, testClaimRequest()); err == nil {
		t.Fatal("accepted redirect")
	}
	if calls != 0 {
		t.Fatal("followed credential redirect")
	}
	if _, err := NewClient(source.URL, "mul_ordinary_pat", nil); err == nil {
		t.Fatal("ordinary PAT became collaboration runtime auth")
	}
}

func TestClientMissingStopIntentIsNotPermissionToContinue(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"identity": testIdentity()})
	}))
	defer s.Close()
	client, err := NewClient(s.URL, "mdt_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.StopIntent(context.Background(), testIdentity(), "termination-only"); err == nil {
		t.Fatal("missing stop predicate became false")
	}
}

func TestClientBusinessOutcomesUseWireValues(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "succeeded", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			record := protocol.CollaborationRunOutcome{Identity: testIdentity(), EventID: testIdentity().IssueID,
				AttemptSeq: "1", AttemptNonce: testIdentity().LaunchNonce, Outcome: outcome, Body: "explicit provider result"}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != runPath(record.Identity.RunID, "outcome") || r.Header.Get("Authorization") != "Bearer business-only" {
					t.Error("outcome used the wrong endpoint or credential")
				}
				var received protocol.CollaborationRunOutcome
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil || received != record {
					t.Error("outcome lost its exact business value or attempt identity")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_test", nil)
			if err != nil {
				t.Fatal(err)
			}
			err = client.Outcome(t.Context(), record, "business-only")
			if outcome == "success" || outcome == "failure" {
				if err != nil || calls != 1 {
					t.Fatalf("business outcome %q was not sent: calls=%d err=%v", outcome, calls, err)
				}
			} else if err == nil || calls != 0 {
				t.Fatalf("command/group terminal %q was accepted as a business outcome", outcome)
			}
		})
	}
}
