package collaborationrun

import (
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Host records preserve each requested nonce, including an unissued permit.
// Process instances are local facts and never enter public run records.
type hostLaunchRecord struct {
	AttemptSeq       string                                   `json:"attempt_seq"`
	AttemptNonce     string                                   `json:"attempt_nonce"`
	Kind             string                                   `json:"kind"`
	Phase            string                                   `json:"phase"`
	Authorized       *bool                                    `json:"authorized,omitempty"`
	ProcessInstance  string                                   `json:"process_instance,omitempty"`
	PID              int                                      `json:"pid,omitempty"`
	Scope            string                                   `json:"scope,omitempty"`
	Termination      *protocol.CollaborationLaunchTermination `json:"termination,omitempty"`
	TerminalAccepted bool                                     `json:"terminal_accepted,omitempty"`
}

type hostLedgerRecord struct {
	SessionKey          string                                  `json:"session_key"`
	CapabilityDigest    string                                  `json:"capability_digest"`
	ConfigurationDigest string                                  `json:"configuration_digest"`
	Launches            []hostLaunchRecord                      `json:"launches"`
	Stopped             bool                                    `json:"stopped,omitempty"`
	WaitingForSession   bool                                    `json:"waiting_for_session,omitempty"`
	Claim               *protocol.CollaborationRunClaimResponse `json:"claim,omitempty"`
	BusinessToken       string                                  `json:"business_token,omitempty"`
	Outcome             *protocol.CollaborationRunOutcome       `json:"outcome,omitempty"`
	OutcomeRecorded     bool                                    `json:"outcome_recorded,omitempty"`
	OutcomeRejected     bool                                    `json:"outcome_rejected,omitempty"`
}

func validateHostLedgerRecord(r ledgerRecord) error {
	h := r.Host
	if h == nil || !validDigest(h.SessionKey) || !validDigest(h.CapabilityDigest) || !validDigest(h.ConfigurationDigest) ||
		r.NamespaceID != "" || r.ExitCode != nil || len(h.Launches) > protocol.CollaborationHostMaxLaunches {
		return ErrUnconfirmed
	}
	if h.Claim != nil {
		claim := *h.Claim
		key, err := hostSessionKey(claim)
		if err != nil || key != h.SessionKey || claim.Identity != r.Identity || claim.TerminationToken != r.TerminationToken ||
			claim.Preparation.Host.ConfigurationDigest != h.ConfigurationDigest {
			return ErrUnconfirmed
		}
		request := protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion,
			Containment: protocol.CollaborationContainmentHostProcessSupervision, DeploymentID: r.Identity.DeploymentID,
			InstallationID: r.Identity.InstallationID, RuntimeID: claim.HostContext.RuntimeID,
			PreparationVersion: protocol.CollaborationHostPreparationVersion, CapabilityDigest: h.CapabilityDigest}
		if validateClaimResponse(claim, r.Identity.RunID, request) != nil {
			return ErrUnconfirmed
		}
	}
	if h.WaitingForSession && (len(h.Launches) != 0 || h.Claim == nil || h.Stopped || r.Phase != phaseRegistered) {
		return ErrUnconfirmed
	}
	seen := make(map[string]bool)
	for i, launch := range h.Launches {
		if launch.AttemptSeq != strconv.Itoa(i+1) || !validUUID(launch.AttemptNonce) || seen[launch.AttemptNonce] ||
			(launch.Kind != "provider" && launch.Kind != "preparation") {
			return ErrUnconfirmed
		}
		seen[launch.AttemptNonce] = true
		if i > 0 {
			previous := h.Launches[i-1]
			if previous.Termination == nil || !previous.TerminalAccepted || previous.Authorized == nil || !*previous.Authorized {
				return ErrUnconfirmed
			}
		}
		switch launch.Phase {
		case phasePermitPending, phaseAuthorized, phaseBurned:
			if launch.ProcessInstance != "" || launch.PID != 0 || launch.Termination != nil || launch.TerminalAccepted {
				return ErrUnconfirmed
			}
		case phaseRecorded:
			if !validHostProcessInstance(launch.ProcessInstance, launch.PID) || !validHostScope(launch.Scope) || launch.Termination != nil || launch.TerminalAccepted {
				return ErrUnconfirmed
			}
		case phaseReaped, phaseNeverLaunched:
			if launch.Termination == nil || protocol.ValidateCollaborationLaunchTermination(*launch.Termination) != nil ||
				launch.Termination.Identity != r.Identity || launch.Termination.AttemptSeq != launch.AttemptSeq || launch.Termination.AttemptNonce != launch.AttemptNonce {
				return ErrUnconfirmed
			}
			if launch.Termination.Kind == "managed_launch_ended" && (!validHostProcessInstance(launch.ProcessInstance, launch.PID) || launch.Termination.Cleanup.Scope != launch.Scope) {
				return ErrUnconfirmed
			}
			if launch.Termination.Kind == "not_started" && (launch.ProcessInstance != "" || launch.PID != 0) {
				return ErrUnconfirmed
			}
			if launch.TerminalAccepted && launch.Authorized == nil {
				return ErrUnconfirmed
			}
		default:
			return ErrUnconfirmed
		}
	}
	if r.AttemptSeq != strconv.Itoa(len(h.Launches)) || len(h.Launches) == 0 && r.AttemptNonce != "" ||
		len(h.Launches) > 0 && r.AttemptNonce != h.Launches[len(h.Launches)-1].AttemptNonce {
		return ErrUnconfirmed
	}
	switch r.Phase {
	case phaseRegistered:
		if len(h.Launches) != 0 {
			return ErrUnconfirmed
		}
	case phasePermitPending, phaseAuthorized, phaseBurned, phaseRecorded, phaseReaped:
		if len(h.Launches) == 0 || h.Launches[len(h.Launches)-1].Phase != r.Phase && !(r.Phase == phaseReaped && h.Launches[len(h.Launches)-1].Phase == phaseNeverLaunched) {
			return ErrUnconfirmed
		}
	case phaseNeverLaunched, phaseSealed:
		return ErrUnconfirmed // Host sealing is one atomic proof write.
	case phaseProof, phaseAcknowledged:
		if r.Proof == nil || !h.Stopped || protocol.ValidateCollaborationHostProof(*r.Proof) != nil || r.Proof.Identity != r.Identity {
			return ErrUnconfirmed
		}
		launches, err := authorizedHostTerminations(h)
		if err != nil || !reflect.DeepEqual(launches, r.Proof.Launches) {
			return ErrUnconfirmed
		}
	default:
		return ErrUnconfirmed
	}
	if r.Proof != nil && r.Phase != phaseProof && r.Phase != phaseAcknowledged {
		return ErrUnconfirmed
	}
	if h.Outcome != nil && (h.Outcome.Identity != r.Identity || h.Outcome.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion ||
		h.Outcome.Containment != protocol.CollaborationContainmentHostProcessSupervision || !validUUID(h.Outcome.EventID) ||
		!validHostAttempt(h.Outcome.AttemptSeq) || h.Outcome.AttemptSeq != r.AttemptSeq || h.Outcome.AttemptNonce != r.AttemptNonce ||
		protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: h.Outcome.Outcome, Body: h.Outcome.Body, Record: h.Outcome.Record}) != nil) {
		return ErrUnconfirmed
	}
	if h.OutcomeRecorded && h.OutcomeRejected || h.Outcome == nil && (h.OutcomeRecorded || h.OutcomeRejected) {
		return ErrUnconfirmed
	}
	return nil
}

func hostOutcomePending(h *hostLedgerRecord) bool {
	return h.Outcome != nil && !h.OutcomeRecorded && !h.OutcomeRejected
}

func validHostScope(scope string) bool {
	return scope == "process_group" || scope == "windows_job" || scope == "direct_process"
}

func validHostProcessInstance(instance string, pid int) bool {
	if pid < 1 || len(instance) > 256 {
		return false
	}
	parts := strings.Split(instance, ":")
	if len(parts) == 2 && parts[0] == "handle" {
		return validUUID(parts[1])
	}
	if len(parts) != 4 {
		return false
	}
	if parts[0] == "linux" {
		return validUUID(parts[1]) && parts[2] == strconv.Itoa(pid) && positiveDecimal(parts[3])
	}
	if parts[0] != "darwin" && parts[0] != "windows" || parts[1] != strconv.Itoa(pid) {
		return false
	}
	for _, value := range parts[2:] {
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil || strconv.FormatUint(n, 10) != value {
			return false
		}
	}
	return true
}

func retainedHostRun(r ledgerRecord) RetainedRun {
	retained := RetainedRun{Identity: r.Identity, Host: true, SessionKey: r.Host.SessionKey, CanProve: true}
	if r.Host.Claim != nil && r.Host.Claim.HostContext != nil {
		retained.RuntimeID = r.Host.Claim.HostContext.RuntimeID
	}
	if r.Host.WaitingForSession {
		retained.Waiting, retained.CanProve = true, false
	}
	for _, launch := range r.Host.Launches {
		if launch.Termination == nil && (launch.Phase == phaseBurned || launch.Phase == phaseRecorded) {
			retained.Unconfirmed, retained.CanProve = true, false
		}
	}
	return retained
}

func (r *RunLedger) registerHost(claim protocol.CollaborationRunClaimResponse, sessionKey, capabilityDigest, configurationDigest string) error {
	if r.record != nil || validateIdentity(claim.Identity) != nil || claim.Identity.DeploymentID != r.deploymentID ||
		claim.Identity.InstallationID != r.installationID || claim.Identity.RunID != r.runID || claim.TerminationToken == "" ||
		claim.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || claim.Containment != protocol.CollaborationContainmentHostProcessSupervision ||
		!validDigest(sessionKey) || !validDigest(capabilityDigest) || !validDigest(configurationDigest) {
		return ErrUnconfirmed
	}
	next := ledgerRecord{Version: 2, Identity: claim.Identity, TerminationToken: claim.TerminationToken, Phase: phaseRegistered, AttemptSeq: "0",
		Host: &hostLedgerRecord{SessionKey: sessionKey, CapabilityDigest: capabilityDigest, ConfigurationDigest: configurationDigest,
			Launches: []hostLaunchRecord{}, Claim: &claim, WaitingForSession: true}}
	if err := validateHostLedgerRecord(next); err != nil {
		return err
	}
	return r.persist(next)
}

func (r *RunLedger) hostNext() (ledgerRecord, error) {
	if r.failed || r.lock == nil || r.record == nil || r.record.Version != 2 || r.record.Host == nil {
		return ledgerRecord{}, ErrUnconfirmed
	}
	if r.record.Phase == phaseProof || r.record.Phase == phaseAcknowledged {
		return ledgerRecord{}, ErrSealed
	}
	next := *r.record
	host := *r.record.Host
	host.Launches = append([]hostLaunchRecord(nil), host.Launches...)
	next.Host = &host
	return next, nil
}

func (r *RunLedger) hostSessionAcquired() error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	next.Host.WaitingForSession = false
	return r.persist(next)
}

func (r *RunLedger) requestHostPermit(kind string) (protocol.CollaborationRunPermitRequest, error) {
	next, err := r.hostNext()
	if err != nil {
		return protocol.CollaborationRunPermitRequest{}, err
	}
	h := next.Host
	if h.Stopped || h.WaitingForSession || len(h.Launches) >= protocol.CollaborationHostMaxLaunches || kind != "provider" && kind != "preparation" {
		return protocol.CollaborationRunPermitRequest{}, ErrSealed
	}
	if len(h.Launches) > 0 {
		prior := h.Launches[len(h.Launches)-1]
		if prior.Termination == nil || !prior.TerminalAccepted || prior.Authorized == nil || !*prior.Authorized {
			return protocol.CollaborationRunPermitRequest{}, ErrUnconfirmed
		}
	}
	launch := hostLaunchRecord{AttemptSeq: strconv.Itoa(len(h.Launches) + 1), AttemptNonce: uuid.NewString(), Kind: kind, Phase: phasePermitPending}
	h.Launches = append(h.Launches, launch)
	next.AttemptSeq, next.AttemptNonce, next.Phase = launch.AttemptSeq, launch.AttemptNonce, phasePermitPending
	if err := r.persist(next); err != nil {
		return protocol.CollaborationRunPermitRequest{}, err
	}
	return r.hostPermitRequest(), nil
}

func (r *RunLedger) hostPermitRequest() protocol.CollaborationRunPermitRequest {
	h := r.record.Host
	launch := h.Launches[len(h.Launches)-1]
	return protocol.CollaborationRunPermitRequest{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion, Identity: r.record.Identity,
		AttemptSeq: launch.AttemptSeq, AttemptNonce: launch.AttemptNonce, Containment: protocol.CollaborationContainmentHostProcessSupervision,
		LaunchDescriptor: &protocol.CollaborationLaunchDescriptor{Kind: launch.Kind, CapabilityDigest: h.CapabilityDigest, ConfigurationDigest: h.ConfigurationDigest}}
}

func (r *RunLedger) authorizeHost(permit protocol.CollaborationRunPermitResponse) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	if next.Phase != phasePermitPending || permit.Identity != next.Identity || permit.AttemptSeq != next.AttemptSeq || permit.AttemptNonce != next.AttemptNonce ||
		!businessPreparationToken(permit.BusinessToken) || permit.BusinessToken == next.TerminationToken {
		return ErrUnconfirmed
	}
	authorized := true
	launch := &next.Host.Launches[len(next.Host.Launches)-1]
	launch.Phase, launch.Authorized = phaseAuthorized, &authorized
	next.Phase, next.Host.BusinessToken = phaseAuthorized, permit.BusinessToken
	return r.persist(next)
}

func (r *RunLedger) burnHost(nonce string) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	if next.Host.Stopped || next.Phase != phaseAuthorized || nonce != next.AttemptNonce {
		return ErrUnconfirmed
	}
	next.Phase, next.Host.Launches[len(next.Host.Launches)-1].Phase = phaseBurned, phaseBurned
	return r.persist(next)
}

func (r *RunLedger) recordHostProcess(nonce, instance, scope string, pid int) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	if next.Phase != phaseBurned || nonce != next.AttemptNonce || !validHostProcessInstance(instance, pid) || !validHostScope(scope) {
		return ErrUnconfirmed
	}
	launch := &next.Host.Launches[len(next.Host.Launches)-1]
	launch.Phase, launch.ProcessInstance, launch.Scope, launch.PID = phaseRecorded, instance, scope, pid
	next.Phase = phaseRecorded
	return r.persist(next)
}

// The caller must have observed native Start refusing to create a process, or
// have kept the launch before Start. A generic Start error is not this evidence.
func (r *RunLedger) observeHostTermination(nonce, kind string, cleanup protocol.CollaborationCleanupObservation, exitCode *int) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	if len(next.Host.Launches) == 0 || nonce != next.AttemptNonce {
		return ErrUnconfirmed
	}
	launch := &next.Host.Launches[len(next.Host.Launches)-1]
	if launch.Termination != nil {
		return ErrUnconfirmed
	}
	if kind == "not_started" {
		if launch.ProcessInstance != "" || launch.Phase != phasePermitPending && launch.Phase != phaseAuthorized && launch.Phase != phaseBurned {
			return ErrUnconfirmed
		}
	} else if kind != "managed_launch_ended" || launch.Phase != phaseRecorded || cleanup.Scope != launch.Scope {
		return ErrUnconfirmed
	}
	observation := protocol.CollaborationLaunchTermination{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion, Identity: next.Identity,
		Containment: protocol.CollaborationContainmentHostProcessSupervision, AttemptSeq: launch.AttemptSeq, AttemptNonce: nonce, LaunchID: nonce,
		EventID: uuid.NewString(), Kind: kind, Cleanup: cleanup, ExitCode: exitCode}
	if protocol.ValidateCollaborationLaunchTermination(observation) != nil {
		return ErrUnconfirmed
	}
	launch.Termination, launch.Phase, next.Phase = &observation, phaseReaped, phaseReaped
	if kind == "not_started" {
		launch.Phase = phaseNeverLaunched
	}
	return r.persist(next)
}

func (r *RunLedger) acknowledgeHostLaunch(receipt protocol.CollaborationLaunchTerminationReceipt) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	for i := range next.Host.Launches {
		launch := &next.Host.Launches[i]
		if launch.AttemptNonce != receipt.LaunchID {
			continue
		}
		if launch.Termination == nil || receipt.Identity != next.Identity || receipt.EventID != launch.Termination.EventID || !receipt.Accepted ||
			receipt.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || launch.Authorized != nil && *launch.Authorized != receipt.Authorized ||
			!receipt.Authorized && launch.Termination.Kind != "not_started" {
			return ErrUnconfirmed
		}
		launch.Authorized, launch.TerminalAccepted = &receipt.Authorized, true
		return r.persist(next)
	}
	return ErrUnconfirmed
}

func (r *RunLedger) stopHost() error {
	next, err := r.hostNext()
	if errors.Is(err, ErrSealed) {
		return nil
	}
	if err != nil {
		return err
	}
	next.Host.Stopped, next.Host.WaitingForSession = true, false
	return r.persist(next)
}

func (r *RunLedger) recordHostOutcome(outcome protocol.CollaborationRunOutcome) error {
	next, err := r.hostNext()
	if err != nil {
		return err
	}
	if next.Host.Outcome != nil {
		if reflect.DeepEqual(*next.Host.Outcome, outcome) {
			return nil
		}
		return ErrUnconfirmed
	}
	if outcome.Identity != next.Identity || outcome.AttemptSeq != next.AttemptSeq || outcome.AttemptNonce != next.AttemptNonce ||
		outcome.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || outcome.Containment != protocol.CollaborationContainmentHostProcessSupervision ||
		protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: outcome.Outcome, Body: outcome.Body, Record: outcome.Record}) != nil {
		return ErrProtocol
	}
	next.Host.Outcome = &outcome
	return r.persist(next)
}

func (r *RunLedger) acknowledgeHostOutcome(rejected bool) error {
	// An outcome receipt can arrive after the run's final proof. Only its
	// acknowledgement flags change; the sealed execution can never reopen.
	if r.failed || r.lock == nil || r.record == nil || r.record.Version != 2 || r.record.Host == nil || r.record.Host.Outcome == nil {
		return ErrUnconfirmed
	}
	next := *r.record
	host := *r.record.Host
	next.Host = &host
	next.Host.OutcomeRecorded, next.Host.OutcomeRejected = !rejected, rejected
	return r.persist(next)
}

func authorizedHostTerminations(h *hostLedgerRecord) ([]protocol.CollaborationLaunchTermination, error) {
	var launches []protocol.CollaborationLaunchTermination
	for i, launch := range h.Launches {
		if launch.Termination == nil || !launch.TerminalAccepted || launch.Authorized == nil {
			return nil, ErrUnconfirmed
		}
		if *launch.Authorized {
			launches = append(launches, *launch.Termination)
		} else if i != len(h.Launches)-1 || launch.Termination.Kind != "not_started" {
			return nil, ErrUnconfirmed
		}
	}
	return launches, nil
}

func (r *RunLedger) sealHost() (protocol.CollaborationRunProof, error) {
	if r.record.Phase == phaseProof || r.record.Phase == phaseAcknowledged {
		return *r.record.Proof, nil
	}
	next, err := r.hostNext()
	if err != nil {
		return protocol.CollaborationRunProof{}, err
	}
	if !next.Host.Stopped {
		return protocol.CollaborationRunProof{}, ErrUnconfirmed
	}
	launches, err := authorizedHostTerminations(next.Host)
	if err != nil {
		return protocol.CollaborationRunProof{}, err
	}
	cleanup, err := protocol.SummarizeCollaborationCleanup(launches)
	if err != nil {
		return protocol.CollaborationRunProof{}, err
	}
	state := "never_launched_proven"
	for _, launch := range launches {
		if launch.Kind == "managed_launch_ended" {
			state = "host_exit_observed"
		}
	}
	nonce := ""
	if len(launches) > 0 {
		nonce = launches[len(launches)-1].AttemptNonce
	}
	proof := protocol.CollaborationRunProof{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion, ReceiptID: uuid.NewString(), Identity: next.Identity,
		LastAuthorizedAttemptSeq: strconv.Itoa(len(launches)), SealedAttemptSeq: strconv.Itoa(len(launches)), AttemptNonce: nonce, State: state,
		Containment: protocol.CollaborationContainmentHostProcessSupervision, Launches: launches, Cleanup: cleanup}
	if protocol.ValidateCollaborationHostProof(proof) != nil {
		return protocol.CollaborationRunProof{}, ErrUnconfirmed
	}
	next.Phase, next.Proof = phaseProof, &proof
	if err := r.persist(next); err != nil {
		return protocol.CollaborationRunProof{}, err
	}
	return proof, nil
}
