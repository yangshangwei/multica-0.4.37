package service

import (
	"bytes"
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func preparationRegistrationRequest(f *managedCollaborationFixture) protocol.CollaborationRuntimeCapabilitiesRequest {
	claim := preparationClaimRequest(f)
	return protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: claim.ProtocolVersion, DeploymentID: claim.DeploymentID,
		InstallationID: claim.InstallationID, RuntimeID: claim.RuntimeID, BindingID: claim.BindingID, BindingEpoch: claim.BindingEpoch,
		ExpectedRuntimeConfigEpoch: strconv.FormatInt(f.runtime.CollaborationConfigEpoch, 10),
		Capabilities: protocol.CollaborationRuntimeCapabilities{Version: 1, PreparationVersion: 1, OS: "linux", Arch: "amd64",
			Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttachmentFormat: protocol.CollaborationAttachmentFormat,
			Provider: protocol.CollaborationPreparationProvider{Provider: f.runtime.Provider, ProtocolFamily: f.runtime.Provider,
				AdapterID: "static_json_v1", AdapterVersion: 1, BundleDigest: strings.Repeat("a", 64), AccountSource: "runtime_configuration", Resume: "fresh"},
			Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20,
				WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}}
}

func registerPreparationFixture(t *testing.T, f *managedCollaborationFixture) protocol.CollaborationRuntimeCapabilitiesRequest {
	t.Helper()
	p := preparationRegistrationRequest(f)
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func refreshPreparationConsent(t *testing.T, f *managedCollaborationFixture) {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(target), Roster: roster}); err != nil {
		t.Fatal(err)
	}
	f.grant = consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
}

func preparedRuntimeFixture(t *testing.T) *managedCollaborationFixture {
	t.Helper()
	f := newManagedCollaborationFixture(t)
	registerPreparationFixture(t, f)
	refreshPreparationConsent(t, f)
	return f
}

func TestCollaborationRuntimePreparationRegistrationEpochAndMetadataIsolation(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	originalGrant := f.grant
	p := registerPreparationFixture(t, f)
	if f.runtime.CollaborationConfigEpoch != 2 {
		t.Fatalf("registration did not advance the original runtime epoch exactly once: %d", f.runtime.CollaborationConfigEpoch)
	}
	before := f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_kind='runtime' AND resource_id=$1`, f.runtime.ID)
	replay, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p)
	if err != nil || replay.RuntimeConfigEpoch != "2" || replay.Registration == nil || replay.Registration.RuntimeConfigEpoch != "2" {
		t.Fatalf("same-summary registration did not recover its current state: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_kind='runtime' AND resource_id=$1`, f.runtime.ID) != before {
		t.Fatal("same summary produced another invalidation")
	}
	if _, err = f.svc.Queries.UpsertAgentRuntime(f.daemonCtx, db.UpsertAgentRuntimeParams{WorkspaceID: f.runtime.WorkspaceID, DaemonID: f.runtime.DaemonID,
		Name: f.runtime.Name, RuntimeMode: f.runtime.RuntimeMode, Provider: f.runtime.Provider, Status: f.runtime.Status,
		DeviceInfo: f.runtime.DeviceInfo, Metadata: []byte(`{"ordinary_registration":"changed","collaboration_capabilities":{"spoofed":true}}`), OwnerID: f.runtime.OwnerID}); err != nil {
		t.Fatal(err)
	}
	current, err := f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil || current.CollaborationConfigEpoch != f.runtime.CollaborationConfigEpoch || !bytes.Equal(current.CollaborationCapabilities, f.runtime.CollaborationCapabilities) {
		t.Fatalf("ordinary metadata upsert erased or replaced protected capabilities: %v", err)
	}
	if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "old consent must not authorize new bundle"}); err == nil {
		t.Fatal("capability registration bypassed the original owner consent")
	}
	grant, err := f.svc.Queries.GetCollaborationGrant(f.ctx, originalGrant.ID)
	if err != nil || grant.ConfigurationFingerprint != originalGrant.ConfigurationFingerprint || grant.ConfigVersion != originalGrant.ConfigVersion {
		t.Fatalf("registration silently rewrote grant identity: %v", err)
	}
	// Ordinary execution configuration changes invalidate the saved probe epoch,
	// even if the daemon repeats its previous summary and old expected epoch.
	f.fx.Exec(t, `UPDATE agent_runtime SET runtime_mode=CASE WHEN runtime_mode='local' THEN 'cloud' ELSE 'local' END WHERE id=$1`, f.runtime.ID)
	if _, err = f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); !collaborationErrorCode(err, "stale_runtime_configuration") {
		t.Fatalf("old registration renewed itself after a configuration change: %v", err)
	}
	state, err := f.svc.RuntimeCapabilities(f.daemonCtx, f.runtime.ID)
	if err != nil || state.RuntimeConfigEpoch != "3" || state.Registration.RuntimeConfigEpoch != "2" {
		t.Fatalf("current/recorded epoch drift was hidden: %v", err)
	}
}

func TestCollaborationRuntimePreparationRegistrationAuthorityAndCompleteSummary(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	p := preparationRegistrationRequest(f)
	for name, change := range map[string]func(*protocol.CollaborationRuntimeCapabilitiesRequest){
		"partial summary": func(p *protocol.CollaborationRuntimeCapabilitiesRequest) {
			p.Capabilities.Limits = protocol.CollaborationPreparationLimits{}
		},
		"other installation":       func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.InstallationID = uuid.NewString() },
		"other binding":            func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.BindingID = uuid.NewString() },
		"other binding epoch":      func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.BindingEpoch = "2" },
		"other provider":           func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.Capabilities.Provider.Provider = "claude" },
		"unsupported architecture": func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.Capabilities.Arch = "mips" },
		"unsupported OS":           func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.Capabilities.OS = "darwin" },
		"stale config":             func(p *protocol.CollaborationRuntimeCapabilitiesRequest) { p.ExpectedRuntimeConfigEpoch = "2" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := p
			change(&bad)
			if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, bad); err == nil {
				t.Fatal("invalid capability registration was accepted")
			}
		})
	}
	for name, ctx := range map[string]context.Context{"anonymous": context.Background(), "human owner": f.ctx,
		"ordinary task": auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask}),
		"stale account": auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: f.creator, Version: 2, Kind: "daemon_token", WorkspaceID: f.workspace,
			DaemonID: f.runtime.DaemonID.String, BindingID: util.UUIDToString(f.binding.ID), BindingEpoch: 1})} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.RegisterRuntimeCapabilities(ctx, p); err == nil {
				t.Fatal("unrelated credential registered execution capabilities")
			}
			if _, err := f.svc.RuntimeCapabilities(ctx, f.runtime.ID); err == nil {
				t.Fatal("unrelated credential read the managed capability summary")
			}
		})
	}
	row, err := f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil || row.CollaborationCapabilities != nil || row.CollaborationConfigEpoch != 1 {
		t.Fatalf("rejected reports left partial capability state: %v", err)
	}
}

func TestCollaborationRuntimePreparationRegistrationUnknownCommit(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(strconv.FormatBool(committed), func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			p := preparationRegistrationRequest(f)
			fault := &queueFaultStarter{TxStarter: f.pool, commitFailure: !committed, lostCommit: committed}
			f.svc.TxStarter = fault
			if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p); err == nil || fault.begins.Load() != 1 {
				t.Fatalf("uncertain registration was guessed or replayed: %v", err)
			}
			f.svc.TxStarter = f.pool
			state, err := f.svc.RuntimeCapabilities(f.daemonCtx, f.runtime.ID)
			if err != nil || (state.Registration != nil) != committed {
				t.Fatalf("GET recovery disagrees with actual commit: %v", err)
			}
			registered, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, p)
			if err != nil || registered.RuntimeConfigEpoch != "2" || registered.Registration == nil || !reflect.DeepEqual(registered.Registration.Capabilities, p.Capabilities) {
				t.Fatalf("same full report could not reconcile exactly once: %v", err)
			}
		})
	}
}
