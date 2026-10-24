package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCollaborationPreparationInvocationRequiresExplicitSafeFields(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"instructions":"","model":""}`, `{"instructions":null,"model":"","thinking_level":""}`,
		`{"instructions":"one","instructions":"two","model":"","thinking_level":""}`, `{"instructions":"","model":"","thinking_level":"","custom_env":{}}`} {
		var value CollaborationPreparationInvocation
		if json.Unmarshal([]byte(raw), &value) == nil {
			t.Fatal("partial/ambiguous invocation accepted")
		}
	}
	for _, p := range []CollaborationPreparationInvocation{{}, {Instructions: "exact\nprivate instructions", Model: "claude-opus-5[1m]", ThinkingLevel: "xhigh"}} {
		if err := ValidateCollaborationPreparationInvocation(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []CollaborationPreparationInvocation{{Instructions: strings.Repeat("x", CollaborationInvocationInstructionsLimit+1)},
		{Instructions: "embedded\x00NUL"}, {Instructions: string([]byte{0xff})}, {Model: "--other-option"}, {Model: "model\nsecret"},
		{Model: strings.Repeat("x", CollaborationInvocationModelLimit+1)}, {ThinkingLevel: "high;command"}, {ThinkingLevel: strings.Repeat("h", 65)}} {
		if ValidateCollaborationPreparationInvocation(p) == nil {
			t.Fatal("unsafe invocation accepted")
		}
	}
	p, _ := preparationProtocolFixture()
	if err := SealCollaborationPreparation(&p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "invocation")
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var missing CollaborationPreparation
	if json.Unmarshal(raw, &missing) == nil {
		t.Fatal("missing invocation silently defaulted during manifest decoding")
	}
}

func preparationProtocolFixture() (CollaborationPreparation, []CollaborationRunInput) {
	id := CollaborationRunIdentity{DeploymentID: uuid.NewString(), InstallationID: uuid.NewString(), IssueID: uuid.NewString(),
		SessionID: uuid.NewString(), TargetID: uuid.NewString(), ChannelID: uuid.NewString(), BatchID: uuid.NewString(),
		GroupID: uuid.NewString(), RunID: uuid.NewString(), Fence: "1", ClaimGeneration: "1", LaunchNonce: uuid.NewString()}
	file := CollaborationPreparationAttachment{ID: uuid.NewString(), Version: "1", Name: "../../display-only.txt", ContentType: "text/plain", SizeBytes: 4}
	file.Destination = CollaborationPreparationAttachmentDestination(file.ID, file.Version)
	inputs := []CollaborationRunInput{{CommandID: uuid.NewString(), AttachmentIDs: []string{file.ID}}}
	p := CollaborationPreparation{Version: 1, Identity: id, CommandIDs: []string{inputs[0].CommandID},
		Resource: CollaborationPreparationResource{AgentID: uuid.NewString(), RuntimeID: uuid.NewString(), OwnerUserID: uuid.NewString(),
			ExecutionWorkspaceID: uuid.NewString(), TargetEpoch: "1", ConfigVersion: "1", RuntimeConfigEpoch: "2"},
		Provider: CollaborationPreparationProvider{Provider: "codex", ProtocolFamily: "codex", AdapterID: "static_json_v1", AdapterVersion: 1,
			BundleDigest: strings.Repeat("a", 64), AccountSource: "runtime_configuration", Resume: "fresh"},
		Attachments: []CollaborationPreparationAttachment{file}, Limits: CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20,
			TotalInputBytes: 256 << 20, WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}
	return p, inputs
}

func TestCollaborationPreparationBindsManifestAndActualPrompt(t *testing.T) {
	p, inputs := preparationProtocolFixture()
	if err := SealCollaborationPreparation(&p); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCollaborationPreparation(&p, p.Identity, inputs); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CollaborationPreparation){
		"other run":           func(p *CollaborationPreparation) { p.Identity.RunID = uuid.NewString() },
		"other prompt":        func(p *CollaborationPreparation) { p.CommandIDs = []string{uuid.NewString()} },
		"missing input file":  func(p *CollaborationPreparation) { p.Attachments = []CollaborationPreparationAttachment{} },
		"path traversal":      func(p *CollaborationPreparation) { p.Attachments[0].Destination = "../private" },
		"duplicate file":      func(p *CollaborationPreparation) { p.Attachments = append(p.Attachments, p.Attachments[0]) },
		"rounded version":     func(p *CollaborationPreparation) { p.Attachments[0].Version = "01" },
		"input too large":     func(p *CollaborationPreparation) { p.Attachments[0].SizeBytes = p.Limits.PerFileBytes + 1 },
		"secret account path": func(p *CollaborationPreparation) { p.Provider.AccountSource = "/home/owner/.config" },
		"resume":              func(p *CollaborationPreparation) { p.Provider.Resume = "previous-session" },
		"fallback":            func(p *CollaborationPreparation) { p.Provider.Fallback = true },
		"unknown adapter":     func(p *CollaborationPreparation) { p.Provider.AdapterID = "host-exec" },
		"mutable git": func(p *CollaborationPreparation) {
			p.Git = &CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: "HEAD", BundleDigest: strings.Repeat("b", 64)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad, _ := preparationProtocolFixture()
			bad.Identity, bad.CommandIDs, bad.Attachments = p.Identity, append([]string(nil), p.CommandIDs...), append([]CollaborationPreparationAttachment(nil), p.Attachments...)
			mutate(&bad)
			_ = SealCollaborationPreparation(&bad)
			if ValidateCollaborationPreparation(&bad, p.Identity, inputs) == nil {
				t.Fatal("invalid preparation accepted")
			}
		})
	}
	p.Attachments[0].Name = "another display name"
	if ValidateCollaborationPreparation(&p, p.Identity, inputs) == nil {
		t.Fatal("changed manifest accepted with the previous digest")
	}
}

func TestCollaborationPreparationObservationHasNoOutcomeOrProofAuthority(t *testing.T) {
	p, _ := preparationProtocolFixture()
	if err := SealCollaborationPreparation(&p); err != nil {
		t.Fatal(err)
	}
	event := CollaborationPreparationObservation{Identity: p.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: uuid.NewString(),
		ManifestDigest: p.ManifestDigest, Stage: "attachments", State: "prepared"}
	if err := ValidateCollaborationPreparationObservation(event); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"success", "exited_proven", "never_launched_proven", "launch_authorized"} {
		event.State = state
		if ValidateCollaborationPreparationObservation(event) == nil {
			t.Fatalf("preparation accepted execution state %q", state)
		}
	}
	event.State, event.Code = "failed", "attachment_unavailable"
	if err := ValidateCollaborationPreparationObservation(event); err != nil {
		t.Fatal(err)
	}
	event.Code = "private provider stderr /secret"
	if ValidateCollaborationPreparationObservation(event) == nil {
		t.Fatal("arbitrary private content accepted as a preparation code")
	}
}
