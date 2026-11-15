//go:build agentintegration && !windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type rk1kCodexMigrationArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type rk1kCodexMigrationBinding struct {
	Executable       string `json:"executable"`
	ExecutableSHA256 string `json:"executable_sha256"`
	Version          string `json:"version"`
	Model            string `json:"model"`
	ConfigPath       string `json:"config_path"`
	ConfigSHA256     string `json:"config_sha256"`
}

type rk1kCodexBindingMigration struct {
	SchemaVersion         int                        `json:"schema_version"`
	Kind                  string                     `json:"kind"`
	Approved              bool                       `json:"approved"`
	ExpiresAt             time.Time                  `json:"expires_at"`
	Previous              rk1kCodexMigrationBinding  `json:"previous"`
	Current               rk1kCodexMigrationBinding  `json:"current"`
	CurrentSourceSHA256   string                     `json:"current_source_sha256"`
	TestBinarySHA256      string                     `json:"test_binary_sha256"`
	PriorRegistration     rk1kCodexMigrationArtifact `json:"prior_registration"`
	PriorLedger           rk1kCodexMigrationArtifact `json:"prior_ledger"`
	Wrapper               rk1kCodexMigrationArtifact `json:"wrapper"`
	UserAuthorization     rk1kCodexMigrationArtifact `json:"user_authorization"`
	ProtocolReview        rk1kCodexMigrationArtifact `json:"protocol_review"`
	IndependentReview     rk1kCodexMigrationArtifact `json:"independent_review"`
	Cap                   int                        `json:"cap"`
	PriorActualStarts     int                        `json:"prior_actual_starts"`
	NewStartLimit         int                        `json:"new_start_limit"`
	Order                 []string                   `json:"order"`
	StopOnFirstIncomplete bool                       `json:"stop_on_first_incomplete"`
	RealExecutionApproved bool                       `json:"real_execution_approved"`
	ClientName            string                     `json:"client_name"`
	ClientTitle           string                     `json:"client_title"`
	ClientVersion         string                     `json:"client_version"`
}

type rk1kCodexMigrationReview struct {
	SchemaVersion           int    `json:"schema_version"`
	Kind                    string `json:"kind"`
	Verdict                 string `json:"verdict"`
	BlockingFindingsOpen    int    `json:"blocking_findings_open"`
	Version                 string `json:"version"`
	ExecutableSHA256        string `json:"executable_sha256"`
	ConfigSHA256            string `json:"config_sha256"`
	CurrentSourceSHA256     string `json:"current_source_sha256"`
	TestBinarySHA256        string `json:"test_binary_sha256"`
	WrapperSHA256           string `json:"wrapper_sha256"`
	PriorRegistrationSHA256 string `json:"prior_registration_sha256"`
	PriorLedgerSHA256       string `json:"prior_ledger_sha256"`
	ProtocolReviewSHA256    string `json:"protocol_review_sha256"`
	UserAuthorizationSHA256 string `json:"user_authorization_sha256"`
}

type rk1kCodexProtocolReview struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Version       string `json:"version"`
	Compatible    bool   `json:"compatible"`
}

func rk1kCodexMigrationTuple(r *rk1kCodexRegistration) rk1kCodexMigrationBinding {
	return rk1kCodexMigrationBinding{r.Executable, r.ExecutableSHA256, r.Version, r.Model, r.ConfigPath, r.ConfigSHA256}
}

// This digest uses the registration's absolute paths, not a different checkout.
func rk1kCodexMigrationSourceDigest(source map[string]string) string {
	paths := make([]string, 0, len(source))
	for path := range source {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	h := sha256.New()
	for _, path := range paths {
		fmt.Fprintf(h, "%s\x00%s\n", path, source[path])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// New versions are never admitted by a range or by the legacy full/retry
// entries. The history boundary validates the complete migration before use.
func rk1kCodexMigrationVersionAllowed(r *rk1kCodexRegistration) bool {
	if r.MigrationRecord == "" && r.MigrationRecordSHA256 == "" {
		return r.Version == "0.165.0" || r.Version == "0.166.0"
	}
	return r.Mode == rk1kFullContinuationMode && r.Version == "0.167.0" && r.MigrationRecord != "" && r.MigrationRecordSHA256 != ""
}

func rk1kCodexMigrationArtifactIntact(a rk1kCodexMigrationArtifact) bool {
	if !filepath.IsAbs(a.Path) || len(a.SHA256) != 64 {
		return false
	}
	info, err := os.Lstat(a.Path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	digest, err := rk1kFileHash(a.Path)
	return err == nil && digest == a.SHA256
}

// This approves one exact successor binding only. Historical authorizations,
// ledgers and their account equality remain immutable; gateway/window/permit
// admission is separate and cannot be granted by this preparatory record.
func rk1kValidateCodexBindingMigration(r, retry *rk1kCodexRegistration) error {
	refuse := errors.New("Codex binding migration is missing, changed, expired or not independently reviewed")
	if r.Mode != rk1kFullContinuationMode || r.Version != "0.167.0" || retry.Version != "0.166.0" || r.Model != retry.Model || r.Cap != 17 || r.PriorActualStarts != 2 || r.PriorDebits != 2 || r.AttemptLimit != 15 {
		return refuse
	}
	bound := rk1kCodexMigrationArtifact{r.MigrationRecord, r.MigrationRecordSHA256}
	var m rk1kCodexBindingMigration
	if !rk1kCodexMigrationArtifactIntact(bound) || rk1kReadBoundJSON(bound.Path, bound.SHA256, &m) != nil {
		return refuse
	}
	if m.SchemaVersion != 1 || m.Kind != "codex_knowledge_binding_migration" || !m.Approved || time.Now().After(m.ExpiresAt) || r.ExpiresAt.After(m.ExpiresAt) || m.RealExecutionApproved ||
		m.Previous != rk1kCodexMigrationTuple(retry) || m.Current != rk1kCodexMigrationTuple(r) ||
		len(r.SourceFiles) == 0 || m.CurrentSourceSHA256 != rk1kCodexMigrationSourceDigest(r.SourceFiles) || len(m.TestBinarySHA256) != 64 || m.TestBinarySHA256 != r.TestBinarySHA256 ||
		m.PriorRegistration != (rk1kCodexMigrationArtifact{r.PriorRegistration, r.PriorRegistrationSHA256}) || m.PriorLedger != (rk1kCodexMigrationArtifact{r.PriorLedger, r.PriorLedgerSHA256}) ||
		m.Cap != 17 || m.PriorActualStarts != 2 || m.NewStartLimit != 15 || !slices.Equal(m.Order, []string{"E0b", "K1", "K2", "K3"}) || !m.StopOnFirstIncomplete ||
		m.ClientName != "multica-agent-sdk" || m.ClientTitle != "Multica Agent SDK" || m.ClientVersion != "0.2.0" {
		return refuse
	}
	for _, artifact := range []rk1kCodexMigrationArtifact{m.PriorRegistration, m.PriorLedger, m.Wrapper, m.UserAuthorization, m.ProtocolReview, m.IndependentReview} {
		if !rk1kCodexMigrationArtifactIntact(artifact) {
			return refuse
		}
	}
	var protocol rk1kCodexProtocolReview
	if rk1kReadBoundJSON(m.ProtocolReview.Path, m.ProtocolReview.SHA256, &protocol) != nil || protocol.SchemaVersion != 1 || protocol.Kind != "codex_app_server_protocol_review" || protocol.Version != r.Version || !protocol.Compatible {
		return refuse
	}
	var review rk1kCodexMigrationReview
	if rk1kReadBoundJSON(m.IndependentReview.Path, m.IndependentReview.SHA256, &review) != nil || review.SchemaVersion != 1 || review.Kind != "codex_knowledge_binding_migration_review" ||
		(review.Verdict != "pass" && review.Verdict != "pass-with-notes") || review.BlockingFindingsOpen != 0 ||
		review.Version != r.Version || review.ExecutableSHA256 != r.ExecutableSHA256 || review.ConfigSHA256 != r.ConfigSHA256 || review.CurrentSourceSHA256 != m.CurrentSourceSHA256 || review.TestBinarySHA256 != r.TestBinarySHA256 ||
		review.WrapperSHA256 != m.Wrapper.SHA256 || review.PriorRegistrationSHA256 != r.PriorRegistrationSHA256 || review.PriorLedgerSHA256 != r.PriorLedgerSHA256 || review.ProtocolReviewSHA256 != m.ProtocolReview.SHA256 || review.UserAuthorizationSHA256 != m.UserAuthorization.SHA256 {
		return refuse
	}
	return nil
}

func rk1kMigrationControl(t *testing.T) (*rk1kContinuationControl, rk1kCodexBindingMigration, rk1kCodexMigrationReview) {
	t.Helper()
	c := rk1kNewContinuationControl(t)
	r := &c.current
	dir := filepath.Dir(r.Ledger)
	write := func(name string, body []byte) rk1kCodexMigrationArtifact {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		digest, err := rk1kFileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		return rk1kCodexMigrationArtifact{path, digest}
	}
	executable := write("new-native", []byte("inert current native"))
	config := write("new-config", []byte("inert current account configuration"))
	r.Executable, r.ExecutableSHA256 = executable.Path, executable.SHA256
	r.ConfigPath, r.ConfigSHA256 = config.Path, config.SHA256
	r.Version = "0.167.0"
	m := rk1kCodexBindingMigration{
		SchemaVersion: 1, Kind: "codex_knowledge_binding_migration", Approved: true,
		ExpiresAt: r.ExpiresAt, Previous: rk1kCodexMigrationTuple(&c.retry), Current: rk1kCodexMigrationTuple(r),
		CurrentSourceSHA256: rk1kCodexMigrationSourceDigest(r.SourceFiles), TestBinarySHA256: r.TestBinarySHA256,
		PriorRegistration: rk1kCodexMigrationArtifact{r.PriorRegistration, r.PriorRegistrationSHA256},
		PriorLedger:       rk1kCodexMigrationArtifact{r.PriorLedger, r.PriorLedgerSHA256},
		Wrapper:           write("wrapper.js", []byte("inert current wrapper")),
		UserAuthorization: write("authorization.json", []byte(`{"scope":"test-only migration preparation"}`)),
		ProtocolReview:    write("protocol.json", []byte(`{"schema_version":1,"kind":"codex_app_server_protocol_review","version":"0.167.0","compatible":true}`)),
		IndependentReview: rk1kCodexMigrationArtifact{Path: filepath.Join(dir, "review.json")},
		Cap:               17, PriorActualStarts: 2, NewStartLimit: 15, Order: []string{"E0b", "K1", "K2", "K3"},
		StopOnFirstIncomplete: true, ClientName: "multica-agent-sdk", ClientTitle: "Multica Agent SDK", ClientVersion: "0.2.0",
	}
	review := rk1kCodexMigrationReview{SchemaVersion: 1, Kind: "codex_knowledge_binding_migration_review", Verdict: "pass-with-notes",
		Version: r.Version, ExecutableSHA256: r.ExecutableSHA256, ConfigSHA256: r.ConfigSHA256,
		CurrentSourceSHA256: m.CurrentSourceSHA256, TestBinarySHA256: r.TestBinarySHA256, WrapperSHA256: m.Wrapper.SHA256,
		PriorRegistrationSHA256: r.PriorRegistrationSHA256, PriorLedgerSHA256: r.PriorLedgerSHA256,
		ProtocolReviewSHA256: m.ProtocolReview.SHA256, UserAuthorizationSHA256: m.UserAuthorization.SHA256}
	r.MigrationRecord = filepath.Join(dir, "migration.json")
	rk1kPublishMigrationControl(t, r, &m, review)
	return c, m, review
}

func rk1kPublishMigrationControl(t *testing.T, r *rk1kCodexRegistration, m *rk1kCodexBindingMigration, review rk1kCodexMigrationReview) {
	t.Helper()
	write := func(path string, value any) string {
		if err := rk1WriteJSON(path, value); err != nil {
			t.Fatal(err)
		}
		digest, err := rk1kFileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	m.IndependentReview.SHA256 = write(m.IndependentReview.Path, review)
	r.MigrationRecordSHA256 = write(r.MigrationRecord, m)
}

func TestRK1KnowledgeCodexMigrationControls(t *testing.T) {
	t.Run("exact_0167_and_new_account_binding", func(t *testing.T) {
		c, _, _ := rk1kMigrationControl(t)
		path := filepath.Join(filepath.Dir(c.current.Ledger), "registration.json")
		if err := rk1WriteJSON(path, c.current); err != nil {
			t.Fatal(err)
		}
		digest, err := rk1kFileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := rk1kLoadCodexRegistrationMode(path, digest, rk1kFullContinuationMode)
		if err != nil {
			t.Fatalf("exact approved migration was refused: %v", err)
		}
		if loaded.Version != "0.167.0" || loaded.ConfigSHA256 == c.retry.ConfigSHA256 {
			t.Fatal("current binding was not retained independently of the historical account")
		}
	})
	for _, name := range []string{"absent", "missing_binding", "record_drift", "unapproved", "expired", "patch", "suffix", "old_version", "wrong_previous", "current_native", "current_config", "current_model", "source_map", "test_binary", "prior_registration", "prior_ledger", "wrapper_drift", "authorization_drift", "protocol_drift", "protocol_incompatible", "review_drift", "review_unapproved", "review_blocking", "review_binding", "cap", "prior_count", "new_limit", "order", "stop_rule", "real_grant", "client_name", "client_title", "client_version", "full_mode", "retry_mode"} {
		t.Run(name, func(t *testing.T) {
			c, m, review := rk1kMigrationControl(t)
			r := &c.current
			switch name {
			case "unapproved":
				m.Approved = false
			case "expired":
				m.ExpiresAt = time.Now().Add(-time.Hour)
			case "patch":
				r.Version = "0.167.1"
				m.Current.Version = r.Version
				review.Version = r.Version
			case "suffix":
				r.Version = "0.167.0-dev"
				m.Current.Version = r.Version
				review.Version = r.Version
			case "old_version":
				r.Version = "0.166.0"
				m.Current.Version = r.Version
				review.Version = r.Version
			case "wrong_previous":
				m.Previous.ConfigSHA256 = strings.Repeat("0", 64)
			case "current_native":
				m.Current.ExecutableSHA256 = strings.Repeat("0", 64)
			case "current_config":
				m.Current.ConfigSHA256 = strings.Repeat("0", 64)
			case "current_model":
				r.Model = "different-model"
				m.Current.Model = r.Model
			case "source_map":
				r.SourceFiles[filepath.Join(filepath.Dir(r.Ledger), "extra.go")] = strings.Repeat("0", 64)
			case "test_binary":
				m.TestBinarySHA256 = strings.Repeat("0", 64)
			case "prior_registration":
				m.PriorRegistration.SHA256 = strings.Repeat("0", 64)
			case "prior_ledger":
				m.PriorLedger.SHA256 = strings.Repeat("0", 64)
			case "review_unapproved":
				review.Verdict = "pending"
			case "review_blocking":
				review.BlockingFindingsOpen = 1
			case "review_binding":
				review.ConfigSHA256 = strings.Repeat("0", 64)
			case "cap":
				m.Cap = 18
			case "prior_count":
				m.PriorActualStarts = 1
			case "new_limit":
				m.NewStartLimit = 16
			case "order":
				m.Order = []string{"K1", "E0b", "K2", "K3"}
			case "stop_rule":
				m.StopOnFirstIncomplete = false
			case "real_grant":
				m.RealExecutionApproved = true
			case "client_name":
				m.ClientName = "codex_cli_rs"
			case "client_title":
				m.ClientTitle = "Codex"
			case "client_version":
				m.ClientVersion = "new-version"
			case "full_mode":
				r.Mode, r.Cap = "full", 15
			case "retry_mode":
				r.Mode, r.Cap = rk1kE0bRetryMode, 15
			}
			rk1kPublishMigrationControl(t, r, &m, review)
			if name == "missing_binding" {
				r.MigrationRecord, r.MigrationRecordSHA256 = "", ""
			}
			mutate := map[string]string{"absent": r.MigrationRecord, "record_drift": r.MigrationRecord, "wrapper_drift": m.Wrapper.Path, "authorization_drift": m.UserAuthorization.Path, "protocol_drift": m.ProtocolReview.Path, "protocol_incompatible": m.ProtocolReview.Path, "review_drift": m.IndependentReview.Path}[name]
			if mutate != "" {
				if name == "absent" {
					if err := os.Remove(mutate); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(mutate, []byte(`{"compatible":false}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "protocol_incompatible" {
				if err := rk1WriteJSON(m.ProtocolReview.Path, rk1kCodexProtocolReview{SchemaVersion: 1, Kind: "codex_app_server_protocol_review", Version: "0.167.0", Compatible: false}); err != nil {
					t.Fatal(err)
				}
				m.ProtocolReview.SHA256, _ = rk1kFileHash(m.ProtocolReview.Path)
				review.ProtocolReviewSHA256 = m.ProtocolReview.SHA256
				rk1kPublishMigrationControl(t, r, &m, review)
			}
			if err := rk1kValidateContinuationHistory(r); err == nil {
				t.Fatal("invalid migration passed the history boundary")
			}
		})
	}
	t.Run("current_files_rechecked_by_real_entry", func(t *testing.T) {
		for _, field := range []string{"native", "config", "source"} {
			t.Run(field, func(t *testing.T) {
				c, _, _ := rk1kMigrationControl(t)
				path := filepath.Join(filepath.Dir(c.current.Ledger), "registration.json")
				if err := rk1WriteJSON(path, c.current); err != nil {
					t.Fatal(err)
				}
				digest, err := rk1kFileHash(path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := rk1kLoadCodexRegistrationMode(path, digest, rk1kFullContinuationMode); err != nil {
					t.Fatalf("positive control is not admitted: %v", err)
				}
				mutate := c.current.Executable
				if field == "config" {
					mutate = c.current.ConfigPath
				} else if field == "source" {
					for source := range c.current.SourceFiles {
						mutate = source
						break
					}
				}
				if err := os.WriteFile(mutate, []byte("changed after review"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := rk1kLoadCodexRegistrationMode(path, digest, rk1kFullContinuationMode); err == nil {
					t.Fatal("current file drift bypassed real-entry binding")
				}
			})
		}
	})
	t.Run("migrated_budget_and_incomplete_stop", func(t *testing.T) {
		for _, incomplete := range []bool{false, true} {
			t.Run(fmt.Sprintf("incomplete_%v", incomplete), func(t *testing.T) {
				c, _, _ := rk1kMigrationControl(t)
				r := &c.current
				beforeFirst, beforeRetry := c.retry.PriorLedgerSHA256, r.PriorLedgerSHA256
				for i := 0; i < 15; i++ {
					id := rk1kContinuationIdentity()
					slot, err := rk1kReserveCodexLedger(r, id, filepath.Join(r.Output, "provider", id.key(), "launch-1"))
					if err != nil {
						t.Fatalf("new start %d: %v", i+1, err)
					}
					if err := slot.started(); err != nil {
						t.Fatal(err)
					}
					if err := slot.finish(!incomplete); err != nil {
						t.Fatal(err)
					}
					if incomplete {
						break
					}
					if err := rk1kAcceptCodexSample(r, rk1kSample{Identity: id, State: "finished", Verdict: "pass"}); err != nil {
						t.Fatal(err)
					}
				}
				id := rk1kContinuationIdentity()
				if _, err := rk1kReserveCodexLedger(r, id, filepath.Join(r.Output, "provider", id.key(), "launch-1")); err == nil {
					t.Fatal("migration bypassed physical cap or incomplete stop")
				}
				afterFirst, err := rk1kFileHash(c.retry.PriorLedger)
				if err != nil {
					t.Fatal(err)
				}
				afterRetry, err := rk1kFileHash(r.PriorLedger)
				if err != nil {
					t.Fatal(err)
				}
				if beforeFirst != afterFirst || beforeRetry != afterRetry {
					t.Fatal("migration changed historical debits")
				}
			})
		}
	})
}
