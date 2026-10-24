// Package collaborationrun owns the separate, fail-closed collaboration runtime.
package collaborationrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var (
	ErrBusy        = errors.New("collaboration run is held by another supervisor")
	ErrUnconfirmed = errors.New("collaboration launch or termination is unconfirmed")
	ErrSealed      = errors.New("collaboration run is permanently sealed")
	ErrUnsupported = errors.New("collaboration containment is unavailable")
)

const (
	phaseRegistered    = "registered"
	phasePermitPending = "permit_pending"
	phaseAuthorized    = "launch_authorized"
	phaseBurned        = "attempt_nonce_burned"
	phaseRecorded      = "process_identity_recorded"
	phaseReaped        = "attempt_reaped"
	phaseNeverLaunched = "never_launched"
	phaseSealed        = "run_sealed"
	phaseProof         = "terminal_proof_persisted"
	phaseAcknowledged  = "proof_acknowledged"
)

type ledgerRecord struct {
	Version          int                               `json:"version"`
	Identity         protocol.CollaborationRunIdentity `json:"identity"`
	TerminationToken string                            `json:"termination_token"`
	Phase            string                            `json:"phase"`
	AttemptSeq       string                            `json:"attempt_seq"`
	AttemptNonce     string                            `json:"attempt_nonce"`
	NamespaceID      string                            `json:"namespace_id"`
	NeverLaunched    bool                              `json:"never_launched"`
	ExitCode         *int                              `json:"exit_code,omitempty"`
	Proof            *protocol.CollaborationRunProof   `json:"proof,omitempty"`
}

func (r ledgerRecord) String() string   { return "collaboration ledger (credentials redacted)" }
func (r ledgerRecord) GoString() string { return r.String() }

// Ledger files are retained as tombstones. A missing/corrupt active record cannot
// be repaired by creating another attempt or by looking up a PID.
type Ledger struct{ dir string }

type RetainedRun struct {
	Identity    protocol.CollaborationRunIdentity
	Unconfirmed bool
	CanProve    bool
}

// Retained supplies restart capacity barriers and sealed-proof recovery without
// exposing credentials. An unreadable record is an error, never an empty queue.
func (l *Ledger) Retained() ([]RetainedRun, error) {
	if err := privateDirectory(l.dir); err != nil {
		return nil, ErrUnconfirmed
	}
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, ErrUnconfirmed
	}
	var retained []RetainedRun
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(l.dir, entry.Name())
		f, err := openPrivateFile(path)
		if err != nil {
			return nil, ErrUnconfirmed
		}
		raw, readErr := io.ReadAll(io.LimitReader(f, 65537))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || len(raw) > 65536 {
			return nil, ErrUnconfirmed
		}
		var record ledgerRecord
		if decodeStrict(raw, &record) != nil || validateIdentity(record.Identity) != nil {
			return nil, ErrUnconfirmed
		}
		id := record.Identity
		digest := sha256.Sum256([]byte(id.DeploymentID + "/" + id.InstallationID + "/" + id.RunID))
		if entry.Name() != hex.EncodeToString(digest[:])+".json" {
			return nil, ErrUnconfirmed
		}
		check := &RunLedger{path: path, deploymentID: id.DeploymentID, installationID: id.InstallationID, runID: id.RunID}
		if check.load() != nil || check.record == nil {
			return nil, ErrUnconfirmed
		}
		record = *check.record
		switch record.Phase {
		case phaseAcknowledged:
			continue
		case phaseReaped, phaseNeverLaunched, phaseSealed, phaseProof:
			retained = append(retained, RetainedRun{Identity: id, CanProve: true})
		case phaseRegistered, phasePermitPending, phaseAuthorized:
			// Nothing can fork before the durable burn. These converge to a
			// never-launched proof and hold no machine capacity meanwhile.
			retained = append(retained, RetainedRun{Identity: id, CanProve: true})
		default:
			retained = append(retained, RetainedRun{Identity: id, Unconfirmed: true})
		}
	}
	return retained, nil
}

type RunLedger struct {
	path                                string
	lock                                *os.File
	deploymentID, installationID, runID string
	record                              *ledgerRecord
	failed                              bool
}

func OpenLedger(dir string) (*Ledger, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("collaboration ledger path must be absolute")
	}
	if err := privateDirectory(dir); err != nil {
		return nil, err
	}
	return &Ledger{dir: dir}, nil
}

func (l *Ledger) Lock(deploymentID, installationID, runID string) (*RunLedger, error) {
	for _, id := range []string{deploymentID, installationID, runID} {
		if !validUUID(id) {
			return nil, errors.New("invalid collaboration ledger scope")
		}
	}
	if err := privateDirectory(l.dir); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(deploymentID + "/" + installationID + "/" + runID))
	path := filepath.Join(l.dir, hex.EncodeToString(digest[:]))
	lock, err := lockPrivateFile(path + ".lock")
	if err != nil {
		return nil, err
	}
	r := &RunLedger{path: path + ".json", lock: lock, deploymentID: deploymentID, installationID: installationID, runID: runID}
	if err := r.load(); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("%w: retained ledger cannot be read", ErrUnconfirmed)
	}
	return r, nil
}

func (r *RunLedger) Close() error {
	if r.lock == nil {
		return nil
	}
	err := unlockPrivateFile(r.lock)
	closeErr := r.lock.Close()
	r.lock = nil
	return errors.Join(err, closeErr)
}

func (r *RunLedger) load() error {
	f, err := openPrivateFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(b) > 65536 {
		return ErrUnconfirmed
	}
	var record ledgerRecord
	if err := decodeStrict(b, &record); err != nil {
		return err
	}
	if record.Version != 1 || validateIdentity(record.Identity) != nil || record.Identity.DeploymentID != r.deploymentID || record.Identity.InstallationID != r.installationID || record.Identity.RunID != r.runID || record.TerminationToken == "" {
		return ErrUnconfirmed
	}
	unrequested := record.AttemptSeq == "0" && record.AttemptNonce == ""
	requested := record.AttemptSeq == "1" && validUUID(record.AttemptNonce)
	switch record.Phase {
	case phaseRegistered:
		if !unrequested {
			return ErrUnconfirmed
		}
	case phasePermitPending, phaseAuthorized, phaseBurned, phaseRecorded, phaseReaped:
		if !requested {
			return ErrUnconfirmed
		}
	case phaseNeverLaunched, phaseSealed, phaseProof, phaseAcknowledged:
		// A registration abandoned before any permit request seals attempt 0.
		if !requested && !(unrequested && record.NeverLaunched) {
			return ErrUnconfirmed
		}
	default:
		return ErrUnconfirmed
	}
	if (record.Phase == phaseReaped || record.Phase == phaseSealed || record.Phase == phaseProof || record.Phase == phaseAcknowledged) && !record.NeverLaunched && (record.NamespaceID == "" || record.ExitCode == nil) {
		return ErrUnconfirmed
	}
	if record.Proof != nil && (record.Proof.Identity != record.Identity || record.Proof.AttemptNonce != record.AttemptNonce || record.Proof.SealedAttemptSeq != record.AttemptSeq || record.Proof.LastAuthorizedAttemptSeq != record.AttemptSeq || !validUUID(record.Proof.ReceiptID)) {
		return ErrUnconfirmed
	}
	if (record.Phase == phaseProof || record.Phase == phaseAcknowledged) && record.Proof == nil {
		return ErrUnconfirmed
	}
	r.record = &record
	return nil
}

func (r *RunLedger) persist(next ledgerRecord) error {
	if r.failed || r.lock == nil {
		return ErrUnconfirmed
	}
	b, err := json.Marshal(next)
	if err == nil {
		err = atomicPrivateWrite(r.path, b)
	}
	if err != nil {
		r.failed = true
		return fmt.Errorf("%w: ledger durability failed", ErrUnconfirmed)
	}
	r.record = &next
	return nil
}

func (r *RunLedger) Register(id protocol.CollaborationRunIdentity, token string) error {
	if r.record != nil {
		return ErrUnconfirmed
	}
	if validateIdentity(id) != nil || id.DeploymentID != r.deploymentID || id.InstallationID != r.installationID || id.RunID != r.runID || token == "" {
		return errors.New("invalid collaboration registration")
	}
	return r.persist(ledgerRecord{Version: 1, Identity: id, TerminationToken: token, Phase: phaseRegistered, AttemptSeq: "0"})
}

func (r *RunLedger) checkPhase(expected string) error {
	if r.failed || r.record == nil || r.lock == nil {
		return ErrUnconfirmed
	}
	if r.record.Phase == phaseSealed || r.record.Phase == phaseProof || r.record.Phase == phaseAcknowledged {
		return ErrSealed
	}
	if r.record.Phase != expected {
		return ErrUnconfirmed
	}
	return nil
}

// RequestPermit is durable before the network request: a lost permit response
// must not look like a fresh run after restart.
func (r *RunLedger) RequestPermit(seq, nonce string) error {
	if err := r.checkPhase(phaseRegistered); err != nil {
		return err
	}
	// This slice intentionally has no provider retry/fallback path.
	if seq != "1" || !validUUID(nonce) {
		return errors.New("invalid first collaboration attempt")
	}
	next := *r.record
	next.Phase, next.AttemptSeq, next.AttemptNonce = phasePermitPending, seq, nonce
	return r.persist(next)
}

func (r *RunLedger) Authorize() error { return r.advance(phasePermitPending, phaseAuthorized) }
func (r *RunLedger) Burn() error      { return r.advance(phaseAuthorized, phaseBurned) }

func (r *RunLedger) advance(from, to string) error {
	if err := r.checkPhase(from); err != nil {
		return err
	}
	next := *r.record
	next.Phase = to
	return r.persist(next)
}

func (r *RunLedger) RecordProcess(namespaceID string) error {
	if err := r.checkPhase(phaseBurned); err != nil {
		return err
	}
	if namespaceID == "" {
		return ErrUnconfirmed
	}
	next := *r.record
	next.Phase, next.NamespaceID = phaseRecorded, namespaceID
	return r.persist(next)
}

// Reap is called only after the namespace controller waited for its init. A
// provider exit, stdio EOF or disappearance of a process group is insufficient.
func (r *RunLedger) Reap(exitCode int) error {
	if err := r.checkPhase(phaseRecorded); err != nil {
		return err
	}
	next := *r.record
	next.Phase, next.ExitCode = phaseReaped, &exitCode
	return r.persist(next)
}

// NeverLaunched is legal only before the durable burn. A burned or later
// record can never return here, so it can never return to authorization.
func (r *RunLedger) NeverLaunched() error {
	if r.failed || r.record == nil || r.lock == nil {
		return ErrUnconfirmed
	}
	switch r.record.Phase {
	case phaseRegistered, phasePermitPending, phaseAuthorized:
	case phaseSealed, phaseProof, phaseAcknowledged:
		return ErrSealed
	default:
		return ErrUnconfirmed
	}
	next := *r.record
	next.Phase, next.NeverLaunched = phaseNeverLaunched, true
	return r.persist(next)
}

func (r *RunLedger) Seal() (protocol.CollaborationRunProof, error) {
	if r.failed || r.record == nil {
		return protocol.CollaborationRunProof{}, ErrUnconfirmed
	}
	if r.record.Phase == phaseProof || r.record.Phase == phaseAcknowledged {
		return *r.record.Proof, nil
	}
	if r.record.Phase != phaseReaped && r.record.Phase != phaseNeverLaunched && r.record.Phase != phaseSealed {
		return protocol.CollaborationRunProof{}, ErrUnconfirmed
	}
	if r.record.Phase != phaseSealed {
		next := *r.record
		next.Phase = phaseSealed
		if err := r.persist(next); err != nil {
			return protocol.CollaborationRunProof{}, err
		}
	}
	state := "exited_proven"
	if r.record.NeverLaunched {
		state = "never_launched_proven"
	}
	proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: r.record.Identity, LastAuthorizedAttemptSeq: r.record.AttemptSeq, SealedAttemptSeq: r.record.AttemptSeq, AttemptNonce: r.record.AttemptNonce, State: state, Containment: protocol.CollaborationContainmentLinuxPIDNamespace, ExitCode: r.record.ExitCode}
	next := *r.record
	next.Phase, next.Proof = phaseProof, &proof
	if err := r.persist(next); err != nil {
		return protocol.CollaborationRunProof{}, err
	}
	return proof, nil
}

func (r *RunLedger) Acknowledge(receiptID string) error {
	if r.record == nil || r.record.Proof == nil || r.record.Proof.ReceiptID != receiptID || (r.record.Phase != phaseProof && r.record.Phase != phaseAcknowledged) {
		return ErrUnconfirmed
	}
	if r.record.Phase == phaseAcknowledged {
		return nil
	}
	next := *r.record
	next.Phase = phaseAcknowledged
	return r.persist(next)
}

func privateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || !ownedByCurrentUser(info) {
		return errors.New("collaboration storage must be a private owned directory")
	}
	return nil
}

func atomicPrivateWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := privateDirectory(dir); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".collaboration-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func validUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}

func positiveDecimal(s string) bool {
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == s
}

func validateIdentity(id protocol.CollaborationRunIdentity) error {
	for _, value := range []string{id.DeploymentID, id.InstallationID, id.IssueID, id.SessionID, id.TargetID, id.ChannelID, id.BatchID, id.GroupID, id.RunID, id.LaunchNonce} {
		if !validUUID(value) {
			return errors.New("invalid collaboration run identity")
		}
	}
	if !positiveDecimal(id.Fence) || !positiveDecimal(id.ClaimGeneration) {
		return errors.New("invalid collaboration run version")
	}
	return nil
}

func decodeStrict(data []byte, out any) error {
	check := json.NewDecoder(bytes.NewReader(data))
	check.UseNumber()
	if err := checkJSONValue(check, 0); err != nil {
		return errors.New("invalid collaboration record")
	}
	if _, err := check.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing collaboration record")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid collaboration record")
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("trailing collaboration record")
	}
	return nil
}

// Reject duplicate fields before decoding: last-key-wins would make fenced
// protocol identities ambiguous between independent parsers.
func checkJSONValue(d *json.Decoder, depth int) error {
	if depth > 64 {
		return ErrProtocol
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrProtocol
	}
	keys := make(map[string]bool)
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] {
				return ErrProtocol
			}
			keys[name] = true
		}
		if err := checkJSONValue(d, depth+1); err != nil {
			return err
		}
	}
	end, err := d.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return ErrProtocol
	}
	return nil
}
