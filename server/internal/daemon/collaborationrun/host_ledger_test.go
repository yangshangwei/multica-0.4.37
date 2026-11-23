package collaborationrun

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func newHostTestLedger(t *testing.T) *Ledger {
	t.Helper()
	l, err := OpenLedger(filepath.Join(t.TempDir(), "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func registerHostTestRun(t *testing.T, l *Ledger) *RunLedger {
	t.Helper()
	r, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &HostConfig{Provider: "claude", ProviderVersion: "fixture-1", ProviderMode: "local", Limits: preparationTestCapabilities().Limits}
	request, claim := hostTestClaim(t, cfg.capabilityShape(strings.Repeat("c", 64)))
	key, err := hostSessionKey(claim)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.registerHost(claim, key, request.CapabilityDigest, claim.Preparation.Host.ConfigurationDigest); err != nil {
		t.Fatal(err)
	}
	if err := r.hostSessionAcquired(); err != nil {
		t.Fatal(err)
	}
	return r
}

func hostTestPermit(t *testing.T, r *RunLedger) protocol.CollaborationRunPermitRequest {
	t.Helper()
	p, err := r.requestHostPermit("provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.authorizeHost(protocol.CollaborationRunPermitResponse{Identity: p.Identity, AttemptSeq: p.AttemptSeq,
		AttemptNonce: p.AttemptNonce, BusinessToken: "mcb_" + strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	return p
}

func hostTestAcknowledge(t *testing.T, r *RunLedger, authorized bool) {
	t.Helper()
	last := r.record.Host.Launches[len(r.record.Host.Launches)-1]
	if err := r.acknowledgeHostLaunch(protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
		Identity: r.record.Identity, LaunchID: last.AttemptNonce, EventID: last.Termination.EventID, Accepted: true, Authorized: authorized}); err != nil {
		t.Fatal(err)
	}
}

func TestHostLedgerRetryNeedsAcceptedPriorWaitAndStopSealsEveryAttempt(t *testing.T) {
	l := newHostTestLedger(t)
	r := registerHostTestRun(t, l)
	for attempt := 0; attempt < 2; attempt++ {
		p := hostTestPermit(t, r)
		if err := r.burnHost(p.AttemptNonce); err != nil {
			t.Fatal(err)
		}
		if _, err := r.requestHostPermit("provider"); !errors.Is(err, ErrUnconfirmed) {
			t.Fatalf("another launch was allowed before Wait: %v", err)
		}
		if err := r.recordHostProcess(p.AttemptNonce, "handle:"+uuid.NewString(), "process_group", 1234); err != nil {
			t.Fatal(err)
		}
		cleanup := protocol.CollaborationCleanupObservation{State: "unverified", Scope: "process_group"}
		if err := r.observeHostTermination(p.AttemptNonce, "managed_launch_ended", cleanup, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := r.requestHostPermit("provider"); !errors.Is(err, ErrUnconfirmed) {
			t.Fatalf("another launch was allowed before the server accepted Wait: %v", err)
		}
		hostTestAcknowledge(t, r, true)
	}
	if _, err := r.Seal(); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("unsealed provider retry path produced a terminal receipt: %v", err)
	}
	if err := r.stopHost(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.requestHostPermit("provider"); !errors.Is(err, ErrSealed) {
		t.Fatalf("Stop admitted a fallback: %v", err)
	}
	proof, err := r.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if proof.State != "host_exit_observed" || proof.SealedAttemptSeq != "2" || len(proof.Launches) != 2 || proof.Cleanup.State != "unverified" {
		t.Fatalf("host termination must retain both waits and limited cleanup: %+v", proof)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	replay, err := r.Seal()
	if err != nil || !reflect.DeepEqual(replay, proof) {
		t.Fatalf("restart changed the original receipt: %v", err)
	}
}

func TestHostLedgerUnissuedPermitTombstoneDoesNotInventAnAuthorizedLaunch(t *testing.T) {
	l := newHostTestLedger(t)
	r := registerHostTestRun(t, l)
	defer r.Close()
	p, err := r.requestHostPermit("provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.observeHostTermination(p.AttemptNonce, "not_started", protocol.CollaborationCleanupObservation{State: "unverified", Scope: "direct_process"}, nil); err != nil {
		t.Fatal(err)
	}
	hostTestAcknowledge(t, r, false)
	if _, err := r.requestHostPermit("provider"); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("an unissued permit was retried with a replacement identity: %v", err)
	}
	if err := r.stopHost(); err != nil {
		t.Fatal(err)
	}
	proof, err := r.Seal()
	if err != nil || proof.SealedAttemptSeq != "0" || proof.State != "never_launched_proven" || len(proof.Launches) != 0 {
		t.Fatalf("unissued permit was counted as a launch: %+v, %v", proof, err)
	}
	if len(r.record.Host.Launches) != 1 || r.record.Host.Launches[0].AttemptNonce != p.AttemptNonce {
		t.Fatal("settlement deleted the original nonce tombstone")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	replayed, err := restarted.Seal()
	if err != nil || !reflect.DeepEqual(replayed, proof) || len(restarted.record.Host.Launches) != 1 ||
		restarted.record.Host.Launches[0].AttemptNonce != p.AttemptNonce {
		t.Fatalf("unissued-permit proof replay lost original proof or nonce: %v", err)
	}
}

func TestHostLedgerRetainsUnknownLaunchWithoutReinterpretingLinuxProof(t *testing.T) {
	l := newHostTestLedger(t)
	r := registerTestRun(t, l)
	path := r.path
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	// A host supervisor crashed after burning its first nonce. The same
	// installation must retain its capacity and session ownership on restart.
	record := map[string]any{
		"version": 2, "identity": testIdentity(), "termination_token": "retained-token",
		"phase": phaseBurned, "attempt_seq": "1", "attempt_nonce": "00000000-0000-4000-8000-000000000011",
		"namespace_id": "", "never_launched": false,
		"host": map[string]any{
			"session_key":          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"capability_digest":    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"configuration_digest": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			"launches": []any{map[string]any{
				"attempt_seq": "1", "attempt_nonce": "00000000-0000-4000-8000-000000000011",
				"kind": "provider", "phase": phaseBurned,
			}},
		},
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	retained, err := l.Retained()
	if err != nil {
		t.Fatalf("valid v2 host ledger must remain readable on restart: %v", err)
	}
	if len(retained) != 1 || !retained[0].Unconfirmed || retained[0].CanProve {
		t.Fatalf("unknown host launch lost its barrier: %+v", retained)
	}
	r, err = l.Lock(testIdentity().DeploymentID, testIdentity().InstallationID, testIdentity().RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Seal(); err == nil {
		t.Fatal("unknown host launch acquired a legacy Linux proof")
	}
	if err := r.NeverLaunched(); err == nil {
		t.Fatal("burned host launch was reset to never launched")
	}
}
