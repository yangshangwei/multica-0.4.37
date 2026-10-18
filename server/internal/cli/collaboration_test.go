package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func collaborationEnvironmentFixture(t *testing.T) protocol.CollaborationRunIdentity {
	t.Helper()
	id := protocol.CollaborationRunIdentity{DeploymentID: uuid.NewString(), InstallationID: uuid.NewString(), IssueID: uuid.NewString(), SessionID: uuid.NewString(),
		TargetID: uuid.NewString(), ChannelID: uuid.NewString(), BatchID: uuid.NewString(), GroupID: uuid.NewString(), RunID: uuid.NewString(), LaunchNonce: uuid.NewString(), Fence: "1", ClaimGeneration: "1"}
	env, err := CollaborationEnvironment(id, "http://127.0.0.1:1", "mcb_"+strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	return id
}

func TestCollaborationEnvironmentRequiresCompleteMatchingIdentity(t *testing.T) {
	for _, field := range []string{CollaborationTokenEnv, CollaborationServerEnv, CollaborationContextEnv, CollaborationIssueEnv, CollaborationRunEnv} {
		t.Run(field, func(t *testing.T) {
			collaborationEnvironmentFixture(t)
			t.Setenv(field, "")
			if run, err := ReadCollaborationContext(); err == nil || run != nil {
				t.Fatal("partial provider identity could fall back to ordinary configuration")
			}
		})
	}
	for _, scenario := range []string{"other_run", "other_issue", "zero_version", "rounded_version", "duplicate_field", "unknown_field", "server_credentials", "server_query", "ordinary_token"} {
		t.Run(scenario, func(t *testing.T) {
			id := collaborationEnvironmentFixture(t)
			encoded, _ := json.Marshal(id)
			switch scenario {
			case "other_run":
				t.Setenv(CollaborationRunEnv, uuid.NewString())
			case "other_issue":
				t.Setenv(CollaborationIssueEnv, uuid.NewString())
			case "zero_version":
				t.Setenv(CollaborationContextEnv, strings.Replace(string(encoded), `"fence":"1"`, `"fence":"0"`, 1))
			case "rounded_version":
				t.Setenv(CollaborationContextEnv, strings.Replace(string(encoded), `"fence":"1"`, `"fence":"1e3"`, 1))
			case "duplicate_field":
				t.Setenv(CollaborationContextEnv, strings.TrimSuffix(string(encoded), "}")+`,"run_id":"`+id.RunID+`"}`)
			case "unknown_field":
				t.Setenv(CollaborationContextEnv, strings.TrimSuffix(string(encoded), "}")+`,"owner_token":"forbidden"}`)
			case "server_credentials":
				t.Setenv(CollaborationServerEnv, "https://user:secret@server.invalid")
			case "server_query":
				t.Setenv(CollaborationServerEnv, "https://server.invalid?")
			case "ordinary_token":
				t.Setenv(CollaborationTokenEnv, "mul_"+strings.Repeat("a", 64))
			}
			if run, err := ReadCollaborationContext(); err == nil || run != nil {
				t.Fatal("invalid or mixed run identity was accepted")
			}
		})
	}
}

func TestCollaborationHostConfigurationDoesNotSelectProviderIdentity(t *testing.T) {
	for _, name := range []string{CollaborationTokenEnv, CollaborationServerEnv, CollaborationContextEnv, CollaborationIssueEnv, CollaborationRunEnv} {
		t.Setenv(name, "")
	}
	t.Setenv("MULTICA_COLLABORATION_RUNNER_CONFIG", `{"runtime_id":"host_configuration"}`)
	if HasCollaborationContext() {
		t.Fatal("host runner configuration was treated as provider authority")
	}
	if run, err := ReadCollaborationContext(); err != nil || run != nil {
		t.Fatalf("host configuration interfered with ordinary CLI startup: %+v %v", run, err)
	}
}
