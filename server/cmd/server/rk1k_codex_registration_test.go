//go:build agentintegration && !windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type rk1kCodexRegistration struct {
	SchemaVersion           int               `json:"schema_version"`
	Authorized              bool              `json:"authorized"`
	Provider                string            `json:"provider"`
	Executable              string            `json:"executable"`
	ExecutableSHA256        string            `json:"executable_sha256"`
	Version                 string            `json:"version"`
	Model                   string            `json:"model"`
	ConfigPath              string            `json:"config_path"`
	ConfigSHA256            string            `json:"config_sha256"`
	TestBinarySHA256        string            `json:"test_binary_sha256"`
	SourceFiles             map[string]string `json:"source_files"`
	Cap                     int               `json:"physical_start_cap"`
	Mode                    string            `json:"mode,omitempty"`
	AttemptLimit            int               `json:"attempt_limit,omitempty"`
	PriorLedger             string            `json:"prior_ledger,omitempty"`
	PriorLedgerSHA256       string            `json:"prior_ledger_sha256,omitempty"`
	PriorActualStarts       int               `json:"prior_actual_starts,omitempty"`
	PriorDebits             int               `json:"prior_debits,omitempty"`
	PriorRegistration       string            `json:"prior_registration,omitempty"`
	PriorRegistrationSHA256 string            `json:"prior_registration_sha256,omitempty"`
	Ledger                  string            `json:"ledger"`
	Output                  string            `json:"output"`
	Database                string            `json:"database"`
	ExpiresAt               time.Time         `json:"expires_at"`
	RegistrationPath        string            `json:"-"`
	RegistrationSHA256      string            `json:"-"`
}

var rk1kActiveReal *rk1kCodexRegistration

func rk1kFileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func rk1kLoadCodexRegistration(path, digest string) (*rk1kCodexRegistration, error) {
	return rk1kLoadCodexRegistrationMode(path, digest, "")
}

func rk1kLoadCodexRegistrationMode(path, digest, entryMode string) (*rk1kCodexRegistration, error) {
	refuse := errors.New("Codex knowledge registration is absent, changed, expired or unauthorized")
	if !filepath.IsAbs(path) || len(digest) != 64 {
		return nil, refuse
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, refuse
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, refuse
	}
	var r rk1kCodexRegistration
	if json.Unmarshal(raw, &r) != nil || r.SchemaVersion != 1 || !r.Authorized || r.Provider != "codex" || r.Cap != 15 || r.Model == "" || (r.Version != "0.165.0" && r.Version != "0.166.0") || time.Now().After(r.ExpiresAt) {
		return nil, refuse
	}
	if !filepath.IsAbs(r.Executable) || !filepath.IsAbs(r.ConfigPath) || !filepath.IsAbs(r.Ledger) || !filepath.IsAbs(r.Output) || !strings.HasPrefix(r.Database, rk1kDatabasePrefix) {
		return nil, refuse
	}
	if entryMode != "" && !rk1kCodexEntryMode(r.Mode, entryMode) {
		return nil, refuse
	}
	if err := rk1kValidateRetryHistory(&r); err != nil {
		return nil, refuse
	}
	for path, want := range map[string]string{r.Executable: r.ExecutableSHA256, r.ConfigPath: r.ConfigSHA256} {
		actual, err := rk1kFileHash(path)
		if err != nil || actual != want || len(want) != 64 {
			return nil, refuse
		}
	}
	self, err := os.Executable()
	if err != nil {
		return nil, refuse
	}
	actual, err := rk1kFileHash(self)
	if err != nil || actual != r.TestBinarySHA256 {
		return nil, refuse
	}
	if len(r.SourceFiles) == 0 {
		return nil, refuse
	}
	for path, want := range r.SourceFiles {
		if !filepath.IsAbs(path) {
			return nil, refuse
		}
		actual, err := rk1kFileHash(path)
		if err != nil || actual != want || len(want) != 64 {
			return nil, refuse
		}
	}
	r.RegistrationPath, r.RegistrationSHA256 = path, digest
	return &r, nil
}

// Metadata probes must report the registered executable version, without
// starting it or discovering credentials. Fake-only fixtures retain 0.165.0.
type rk1kCodexVersionBinding struct {
	Registration string `json:"registration"`
	SHA256       string `json:"registration_sha256"`
}

func rk1kCodexReportedVersion(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "codex-version-binding.json"))
	if errors.Is(err, os.ErrNotExist) {
		if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") == "1" {
			return "", errors.New("real Codex version binding is missing")
		}
		return rk1kCodexFakeVersion, nil
	}
	if err != nil {
		return "", err
	}
	var binding rk1kCodexVersionBinding
	if json.Unmarshal(raw, &binding) != nil {
		return "", errors.New("invalid Codex version binding")
	}
	r, err := rk1kLoadCodexRegistration(binding.Registration, binding.SHA256)
	if err != nil {
		return "", err
	}
	return r.Version, nil
}

type rk1kCodexLedger struct {
	Cap               int              `json:"cap"`
	Mode              string           `json:"mode,omitempty"`
	AttemptLimit      int              `json:"attempt_limit,omitempty"`
	PriorLedgerSHA256 string           `json:"prior_ledger_sha256,omitempty"`
	PriorActualStarts int              `json:"prior_actual_starts,omitempty"`
	PriorDebits       int              `json:"prior_debits,omitempty"`
	Starts            []rk1kCodexStart `json:"starts"`
	Stopped           bool             `json:"stopped"`
}
type rk1kCodexStart struct {
	Sequence        int          `json:"sequence"`
	Identity        rk1kIdentity `json:"identity"`
	Launch          string       `json:"launch"`
	ReservedAt      string       `json:"reserved_at"`
	ActualStarted   bool         `json:"actual_started"`
	StartedAt       string       `json:"started_at,omitempty"`
	ProcessComplete bool         `json:"process_complete"`
	Complete        bool         `json:"complete"`
}
type rk1kCodexReservation struct {
	path     string
	sequence int
	model    string
}

func rk1kWithCodexLedger(path string, change func(*rk1kCodexLedger) error) error {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("Codex execution ledger unavailable or concurrently owned")
	}
	defer os.Remove(path + ".lock")
	if err := lock.Close(); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var ledger rk1kCodexLedger
	if json.Unmarshal(raw, &ledger) != nil || ledger.Cap != 15 {
		return errors.New("invalid Codex execution ledger")
	}
	if err := change(&ledger); err != nil {
		return err
	}
	return rk1WriteJSON(path, ledger)
}

func rk1kReserveCodex(plan rk1Plan, id rk1kIdentity, launch string) (*rk1kCodexReservation, error) {
	r, err := rk1kLoadCodexRegistration(plan.Params["registration"], plan.Params["registration_sha256"])
	if err != nil || r == nil || plan.Real != r.Executable || plan.Fault != "" || plan.Params["seed"] == "true" {
		return nil, errors.New("unbound Codex real execution")
	}
	return rk1kReserveCodexLedger(r, id, launch)
}

func rk1kReserveCodexLedger(r *rk1kCodexRegistration, id rk1kIdentity, launch string) (*rk1kCodexReservation, error) {
	if err := rk1kValidateRetryHistory(r); err != nil {
		return nil, err
	}
	reservation := &rk1kCodexReservation{path: r.Ledger, model: r.Model}
	err := rk1kWithCodexLedger(r.Ledger, func(l *rk1kCodexLedger) error {
		if err := rk1kValidateLedgerMode(r, l, id, launch); err != nil {
			return err
		}
		if l.Stopped || len(l.Starts) >= l.Cap || (len(l.Starts) > 0 && !l.Starts[len(l.Starts)-1].Complete) {
			return errors.New("Codex ledger exhausted or previous execution incomplete")
		}
		for _, start := range l.Starts {
			if start.Identity.key() == id.key() {
				return errors.New("duplicate Codex physical execution refused")
			}
		}
		reservation.sequence = len(l.Starts) + 1
		l.Starts = append(l.Starts, rk1kCodexStart{Sequence: reservation.sequence, Identity: id, Launch: launch, ReservedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		return nil
	})
	return reservation, err
}

func (r *rk1kCodexReservation) started() error {
	return rk1kWithCodexLedger(r.path, func(l *rk1kCodexLedger) error {
		if r.sequence < 1 || r.sequence != len(l.Starts) || l.Stopped || l.Starts[r.sequence-1].ActualStarted {
			return errors.New("Codex actual-start ledger mismatch")
		}
		l.Starts[r.sequence-1].ActualStarted = true
		l.Starts[r.sequence-1].StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
}

func (r *rk1kCodexReservation) finish(complete bool) error {
	return rk1kWithCodexLedger(r.path, func(l *rk1kCodexLedger) error {
		if l.Stopped || r.sequence < 1 || r.sequence != len(l.Starts) || l.Starts[r.sequence-1].Complete || complete && !l.Starts[r.sequence-1].ActualStarted {
			return errors.New("Codex ledger finalization mismatch")
		}
		l.Starts[r.sequence-1].ProcessComplete = complete
		l.Stopped = !complete
		return nil
	})
}

func rk1kAcceptCodexSample(r *rk1kCodexRegistration, sample rk1kSample) error {
	return rk1kWithCodexLedger(r.Ledger, func(l *rk1kCodexLedger) error {
		if len(l.Starts) == 0 {
			return errors.New("sample has no Codex reservation")
		}
		last := &l.Starts[len(l.Starts)-1]
		if l.Stopped || last.Identity.key() != sample.Identity.key() || !last.ActualStarted || !last.ProcessComplete || last.Complete {
			return errors.New("sample does not match completed Codex process")
		}
		if sample.State != "finished" {
			l.Stopped = true
			return nil
		}
		last.Complete = true
		return nil
	})
}

func rk1kStopCodexLedger(r *rk1kCodexRegistration) error {
	return rk1kWithCodexLedger(r.Ledger, func(l *rk1kCodexLedger) error { l.Stopped = true; return nil })
}

func (f *rk1kFixture) executionPlan(plan rk1Plan) rk1Plan {
	if f.real != nil && plan.Params["seed"] != "true" {
		plan.Mode, plan.Real = "real", f.real.Executable
		plan.Fault = ""
		plan.Params = map[string]string{"registration": f.real.RegistrationPath, "registration_sha256": f.real.RegistrationSHA256}
	}
	return plan
}

func TestRK1KnowledgeCodexFakeProof(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("Codex fake proof refuses real gate")
	}
	t.Setenv("RK1K_TEST_PROVIDER", "codex")
	rk1kRunProof(t)
}

func TestRK1KnowledgeCodexRegistrationGuard(t *testing.T) {
	if _, err := rk1kLoadCodexRegistration("", ""); err == nil {
		t.Fatal("unbound real registration admitted")
	}
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := rk1WriteJSON(path, rk1kCodexLedger{Cap: 15, Starts: []rk1kCodexStart{{Sequence: 1}}, Stopped: true}); err != nil {
		t.Fatal(err)
	}
	r := &rk1kCodexReservation{path: path, sequence: 2}
	if r.finish(true) == nil {
		t.Fatal("wrong physical reservation was finalized")
	}
}
