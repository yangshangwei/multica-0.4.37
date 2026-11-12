//go:build agentintegration && !windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// rk1RuntimeContinuationMode is deliberately distinct from the original RK1
// registration. The original stopped launcher and its result files remain
// immutable; this mode can only consume the unattempted successor slots.
const rk1RuntimeContinuationMode = "runtime_historic_stop_exception"

const (
	rk1RuntimeContinuationHistoricalHead       = "1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc"
	rk1RuntimeContinuationHistoricalSourceID   = "6b342ba23c2f5ae6826e8841794d086d1258274d3a39eac84983bfb9f429bdea"
	rk1RuntimeContinuationHistoricalFramework  = "5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9"
	rk1RuntimeContinuationHistoricalResult     = "9a4b94050ad696c46512b0833d27398ac0c6ef48770f6085d1db1283f9d628ce"
	rk1RuntimeContinuationHistoricalInvocation = "909ac2da3054ab19ce4a75290ce9c5b06778740835efa414f6f206d863ddd3ed"
)

type rk1RuntimeContinuationApproval struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Approval      struct {
		UserMessageExact       string `json:"user_message_exact"`
		Scope                  string `json:"scope"`
		ImplementationApproved bool   `json:"implementation_approved"`
		RealExecutionApproved  bool   `json:"real_execution_approved"`
	} `json:"approval"`
	Predecessor struct {
		Scenario                        string `json:"scenario"`
		Label                           string `json:"label"`
		Status                          string `json:"status"`
		Verdict                         string `json:"verdict"`
		PublicResult                    string `json:"public_result"`
		PublicResultSHA256              string `json:"public_result_sha256"`
		InvocationSHA256                string `json:"invocation_sha256"`
		ExitCode                        int    `json:"exit_code"`
		ConsumedBeforeException         int    `json:"consumed_before_exception"`
		RemainingSlots                  int    `json:"remaining_slots"`
		InvocationReference             string `json:"invocation_reference"`
		InvocationReviewInventory       string `json:"invocation_review_inventory"`
		InvocationReviewInventorySHA256 string `json:"invocation_review_inventory_sha256"`
	} `json:"predecessor"`
	SourceBinding struct {
		HistoricalSourceHead string `json:"historical_source_head"`
		HistoricalSourceID   string `json:"historical_source_id"`
		FrameworkSHA256      string `json:"framework_sha256"`
	} `json:"source_binding"`
	ContinuationContract struct {
		Cap                 int `json:"cap"`
		PriorActualStarts   int `json:"prior_actual_starts"`
		NewActualStartLimit int `json:"new_actual_start_limit"`
		Order               []struct {
			Scenario   string `json:"scenario"`
			StartTrial int    `json:"start_trial"`
			Launches   int    `json:"launches"`
		} `json:"order"`
		StopOnFirstNewIncompleteOrH5Canary bool `json:"stop_on_first_new_incomplete_or_h5_canary"`
		HistoricalFilesImmutable           bool `json:"historical_files_immutable"`
	} `json:"continuation_contract"`
	ExecutionGate struct {
		Runnable        bool    `json:"runnable"`
		AllowBuild      bool    `json:"allow_build"`
		AllowCheck      bool    `json:"allow_check"`
		AllowRun        bool    `json:"allow_run"`
		WindowID        *string `json:"window_id"`
		WindowExpiresAt *string `json:"window_expires_at"`
	} `json:"execution_gate"`
}

type rk1RuntimeContinuationRegistration struct {
	ApprovalPath  string
	ApprovalSHA   string
	RepoRoot      string
	Approval      rk1RuntimeContinuationApproval
	Ledger        string
	Output        string
	Execution     *rk1RuntimeContinuationExecutionBinding
	ExecutionPath string
	ExecutionSHA  string
}

// The historic approval is immutable evidence of the one stop exception.
// A future Provider process needs a separate, current binding so that source,
// tool, database, window and gateway drift cannot be hidden by that exception.
type rk1RuntimeContinuationExecutionBinding struct {
	SchemaVersion     int    `json:"schema_version"`
	Kind              string `json:"kind"`
	ApprovalPath      string `json:"approval_registration"`
	ApprovalSHA       string `json:"approval_registration_sha256"`
	AuthorizationPath string `json:"real_execution_authorization"`
	AuthorizationSHA  string `json:"real_execution_authorization_sha256"`
	Historical        struct {
		ResultPath          string `json:"result"`
		ResultSHA           string `json:"result_sha256"`
		InvocationPath      string `json:"invocation"`
		InvocationSHA       string `json:"invocation_sha256"`
		ReviewInventoryPath string `json:"review_inventory"`
		ReviewInventorySHA  string `json:"review_inventory_sha256"`
		Consumed            int    `json:"consumed"`
		Remaining           int    `json:"remaining"`
	} `json:"historical"`
	Current struct {
		Head              string `json:"head"`
		SourceID          string `json:"source_id"`
		SourceManifest    string `json:"source_manifest"`
		SourceManifestSHA string `json:"source_manifest_sha256"`
		FrameworkSHA      string `json:"framework_sha256"`
		LauncherPath      string `json:"launcher"`
		LauncherSHA       string `json:"launcher_sha256"`
		Provider          struct {
			Name          string `json:"name"`
			Version       string `json:"version"`
			Executable    string `json:"executable"`
			ExecutableSHA string `json:"executable_sha256"`
			Model         string `json:"model"`
		} `json:"provider"`
		OperatorConfigSHA string `json:"operator_config_sha256"`
		Database          struct {
			Name        string `json:"name"`
			Migration   string `json:"migration"`
			IdentitySHA string `json:"identity_sha256"`
		} `json:"database"`
		Window struct {
			ID        string `json:"id"`
			Owner     string `json:"owner"`
			ExpiresAt string `json:"expires_at"`
		} `json:"exclusive_window"`
		Permit struct {
			ID        string `json:"id"`
			IssuedAt  string `json:"issued_at"`
			ExpiresAt string `json:"expires_at"`
		} `json:"permit"`
		Gateway struct {
			AdmissionID string `json:"admission_id"`
			Provider    string `json:"provider"`
			Evidence    string `json:"evidence"`
		} `json:"provider_gateway"`
		SourceBindingReady    bool `json:"source_binding_ready"`
		FrameworkBindingReady bool `json:"framework_binding_ready"`
		CLIReady              bool `json:"cli_ready"`
		ConfigurationReady    bool `json:"configuration_ready"`
		DatabaseReady         bool `json:"database_ready"`
		WindowReady           bool `json:"window_ready"`
		PermitReady           bool `json:"permit_ready"`
		GatewayReady          bool `json:"gateway_ready"`
	} `json:"current"`
	Ledger        string `json:"ledger"`
	Output        string `json:"output"`
	ExecutionGate struct {
		Runnable   bool   `json:"runnable"`
		AllowBuild bool   `json:"allow_build"`
		AllowCheck bool   `json:"allow_check"`
		AllowRun   bool   `json:"allow_run"`
		Reason     string `json:"reason"`
	} `json:"execution_gate"`
}

type rk1RuntimeContinuationSlot struct {
	Sequence          int    `json:"sequence"`
	Scenario          string `json:"scenario"`
	Trial             int    `json:"trial"`
	Launch            int    `json:"launch"`
	ReservedAt        string `json:"reserved_at"`
	ActualStarted     bool   `json:"actual_started"`
	StartedAt         string `json:"started_at,omitempty"`
	ProcessComplete   bool   `json:"process_complete"`
	Complete          bool   `json:"complete"`
	Verdict           string `json:"verdict,omitempty"`
	EvidencePath      string `json:"evidence_path,omitempty"`
	EvidenceSHA256    string `json:"evidence_sha256,omitempty"`
	EvidencePersisted bool   `json:"evidence_persisted,omitempty"`
}

func rk1RuntimeContinuationLoadExecutionRegistration(path, digest, repoRoot string, approval *rk1RuntimeContinuationRegistration) (*rk1RuntimeContinuationRegistration, error) {
	refuse := errors.New("runtime continuation execution registration is absent, changed, or incomplete")
	if approval == nil || !filepath.IsAbs(path) || !rk1RuntimeContinuationHex(digest) {
		return nil, refuse
	}
	actual, err := rk1RuntimeContinuationHash(path)
	if err != nil || actual != digest {
		return nil, refuse
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, refuse
	}
	var binding rk1RuntimeContinuationExecutionBinding
	if json.Unmarshal(raw, &binding) != nil || binding.SchemaVersion != 1 || binding.Kind != "runtime_h3a_continuation_execution_registration" {
		return nil, refuse
	}
	approvalPath, err := rk1RuntimeContinuationResolve(repoRoot, binding.ApprovalPath)
	if err != nil || filepath.Clean(approvalPath) != filepath.Clean(approval.ApprovalPath) || binding.ApprovalSHA != approval.ApprovalSHA {
		return nil, refuse
	}
	if got, err := rk1RuntimeContinuationHash(approvalPath); err != nil || got != binding.ApprovalSHA {
		return nil, refuse
	}
	authorizationPath, err := rk1RuntimeContinuationResolve(repoRoot, binding.AuthorizationPath)
	if err != nil || !filepath.IsAbs(authorizationPath) || !rk1RuntimeContinuationHex(binding.AuthorizationSHA) {
		return nil, refuse
	}
	if got, err := rk1RuntimeContinuationHash(authorizationPath); err != nil || got != binding.AuthorizationSHA {
		return nil, refuse
	}
	h := binding.Historical
	if h.ResultSHA != approval.Approval.Predecessor.PublicResultSHA256 || h.InvocationSHA != approval.Approval.Predecessor.InvocationSHA256 ||
		h.ReviewInventorySHA != approval.Approval.Predecessor.InvocationReviewInventorySHA256 || h.Consumed != 12 || h.Remaining != 13 {
		return nil, refuse
	}
	for _, item := range []struct{ path, digest string }{
		{h.ResultPath, h.ResultSHA}, {h.InvocationPath, h.InvocationSHA}, {h.ReviewInventoryPath, h.ReviewInventorySHA},
	} {
		resolved, err := rk1RuntimeContinuationResolve(repoRoot, item.path)
		if err != nil || !filepath.IsAbs(resolved) {
			return nil, refuse
		}
		got, err := rk1RuntimeContinuationHash(resolved)
		if err != nil || got != item.digest {
			return nil, refuse
		}
	}
	if binding.Current.Head == "" || !rk1RuntimeContinuationHex(binding.Current.SourceID) || !rk1RuntimeContinuationHex(binding.Current.SourceManifestSHA) ||
		!rk1RuntimeContinuationHex(binding.Current.FrameworkSHA) || !filepath.IsAbs(binding.Current.LauncherPath) || !rk1RuntimeContinuationHex(binding.Current.LauncherSHA) ||
		binding.Current.Provider.Name == "" || binding.Current.Provider.Version == "" || !filepath.IsAbs(binding.Current.Provider.Executable) ||
		!rk1RuntimeContinuationHex(binding.Current.Provider.ExecutableSHA) || binding.Current.Provider.Model == "" || binding.Current.OperatorConfigSHA == "" ||
		binding.Current.Database.Name == "" || binding.Current.Database.Migration == "" || binding.Current.Database.IdentitySHA == "" {
		return nil, refuse
	}
	launcherSHA, err := rk1RuntimeContinuationHash(binding.Current.LauncherPath)
	if err != nil || launcherSHA != binding.Current.LauncherSHA {
		return nil, refuse
	}
	providerSHA, err := rk1RuntimeContinuationHash(binding.Current.Provider.Executable)
	if err != nil || providerSHA != binding.Current.Provider.ExecutableSHA {
		return nil, refuse
	}
	manifestPath, err := rk1RuntimeContinuationResolve(repoRoot, binding.Current.SourceManifest)
	if err != nil || !filepath.IsAbs(manifestPath) {
		return nil, refuse
	}
	manifestSHA, err := rk1RuntimeContinuationHash(manifestPath)
	if err != nil || manifestSHA != binding.Current.SourceManifestSHA {
		return nil, refuse
	}
	ledger, err := rk1RuntimeContinuationResolve(repoRoot, binding.Ledger)
	if err != nil || !filepath.IsAbs(ledger) {
		return nil, refuse
	}
	output, err := rk1RuntimeContinuationResolve(repoRoot, binding.Output)
	historicalResult, resultErr := rk1RuntimeContinuationResolve(repoRoot, approval.Approval.Predecessor.PublicResult)
	if err != nil || resultErr != nil || !filepath.IsAbs(output) || rk1RuntimeContinuationPathsOverlap(output, filepath.Dir(historicalResult)) {
		return nil, refuse
	}
	if filepath.Clean(ledger) == filepath.Clean(approvalPath) || filepath.Clean(ledger) == filepath.Clean(manifestPath) || filepath.Clean(ledger) == filepath.Clean(output) ||
		rk1RuntimeContinuationPathsOverlap(ledger, filepath.Dir(historicalResult)) {
		return nil, refuse
	}
	result := *approval
	result.Execution = &binding
	result.ExecutionPath, result.ExecutionSHA = path, digest
	result.Ledger, result.Output = ledger, output
	return &result, nil
}

type rk1RuntimeContinuationLedger struct {
	SchemaVersion        int                          `json:"schema_version"`
	Mode                 string                       `json:"mode"`
	Cap                  int                          `json:"cap"`
	PriorActualStarts    int                          `json:"prior_actual_starts"`
	NewActualStartLimit  int                          `json:"new_actual_start_limit"`
	PredecessorResultSHA string                       `json:"predecessor_result_sha256"`
	InvocationSHA        string                       `json:"invocation_sha256"`
	Starts               []rk1RuntimeContinuationSlot `json:"starts"`
	Stopped              bool                         `json:"stopped"`
}

func rk1RuntimeContinuationHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func rk1RuntimeContinuationResolve(repoRoot, path string) (string, error) {
	if path == "" {
		return "", errors.New("empty continuation path")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if repoRoot == "" {
		return "", errors.New("relative continuation path without repository root")
	}
	return filepath.Clean(filepath.Join(repoRoot, path)), nil
}

func rk1RuntimeContinuationPhysicalPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("continuation path must be absolute")
	}
	path = filepath.Clean(path)
	missingTail := ""
	for {
		if _, err := os.Lstat(path); err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, missingTail), nil
		} else if !errors.Is(err, os.ErrNotExist) || filepath.Dir(path) == path {
			return "", err
		}
		missingTail = filepath.Join(filepath.Base(path), missingTail)
		path = filepath.Dir(path)
	}
}

func rk1RuntimeContinuationPathsOverlap(first, second string) bool {
	firstPhysical, err := rk1RuntimeContinuationPhysicalPath(first)
	if err != nil {
		return false
	}
	secondPhysical, err := rk1RuntimeContinuationPhysicalPath(second)
	if err != nil {
		return false
	}
	for _, pair := range [][2]string{{firstPhysical, secondPhysical}, {secondPhysical, firstPhysical}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func rk1RuntimeContinuationHex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func rk1RuntimeContinuationLoadApproval(path, digest, repoRoot string) (*rk1RuntimeContinuationRegistration, error) {
	refuse := errors.New("runtime continuation approval is absent, changed, or too broad")
	if !filepath.IsAbs(path) || !rk1RuntimeContinuationHex(digest) {
		return nil, refuse
	}
	actual, err := rk1RuntimeContinuationHash(path)
	if err != nil || actual != digest {
		return nil, refuse
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, refuse
	}
	var approval rk1RuntimeContinuationApproval
	if json.Unmarshal(raw, &approval) != nil || approval.SchemaVersion != 1 || approval.Kind != "runtime_h3a_historic_stop_exception" ||
		approval.Approval.UserMessageExact != "批准" || !approval.Approval.ImplementationApproved || approval.Approval.RealExecutionApproved ||
		approval.Predecessor.Scenario != "RK1-H3a" || approval.Predecessor.Label != "RK1-H3a-t2" || approval.Predecessor.Status != "incomplete" ||
		approval.Predecessor.Verdict != "measured:provider_result_missing" || approval.Predecessor.ExitCode != 143 ||
		approval.Predecessor.ConsumedBeforeException != 12 || approval.Predecessor.RemainingSlots != 13 ||
		!rk1RuntimeContinuationHex(approval.Predecessor.PublicResultSHA256) || !rk1RuntimeContinuationHex(approval.Predecessor.InvocationSHA256) ||
		!rk1RuntimeContinuationHex(approval.SourceBinding.HistoricalSourceID) || !rk1RuntimeContinuationHex(approval.SourceBinding.FrameworkSHA256) ||
		approval.ContinuationContract.Cap != 25 || approval.ContinuationContract.PriorActualStarts != 12 || approval.ContinuationContract.NewActualStartLimit != 13 ||
		!approval.ContinuationContract.StopOnFirstNewIncompleteOrH5Canary || !approval.ContinuationContract.HistoricalFilesImmutable ||
		approval.ExecutionGate.Runnable || approval.ExecutionGate.AllowBuild || approval.ExecutionGate.AllowCheck || approval.ExecutionGate.AllowRun ||
		approval.ExecutionGate.WindowID != nil || approval.ExecutionGate.WindowExpiresAt != nil {
		return nil, refuse
	}
	if approval.SourceBinding.HistoricalSourceHead != rk1RuntimeContinuationHistoricalHead ||
		approval.SourceBinding.HistoricalSourceID != rk1RuntimeContinuationHistoricalSourceID ||
		approval.SourceBinding.FrameworkSHA256 != rk1RuntimeContinuationHistoricalFramework ||
		approval.Predecessor.PublicResultSHA256 != rk1RuntimeContinuationHistoricalResult ||
		approval.Predecessor.InvocationSHA256 != rk1RuntimeContinuationHistoricalInvocation ||
		approval.Predecessor.InvocationReference == "" || approval.Predecessor.InvocationReviewInventory == "" ||
		approval.Predecessor.InvocationReviewInventorySHA256 != "e5807dddbc40ff880b431fb5ba002457c407fb2d77b75ee76d3b3841d30b991b" {
		return nil, refuse
	}
	resultPath, err := rk1RuntimeContinuationResolve(repoRoot, approval.Predecessor.PublicResult)
	if err != nil {
		return nil, refuse
	}
	resultDigest, err := rk1RuntimeContinuationHash(resultPath)
	if err != nil || resultDigest != approval.Predecessor.PublicResultSHA256 {
		return nil, refuse
	}
	inventoryPath, err := rk1RuntimeContinuationResolve(repoRoot, approval.Predecessor.InvocationReviewInventory)
	if err != nil {
		return nil, refuse
	}
	var inventory struct {
		OriginalStopTriggered  bool              `json:"original_stop_triggered"`
		NextScenarioAuthorized bool              `json:"next_scenario_authorized"`
		ActualOriginalExit     int               `json:"actual_original_exit"`
		Hashes                 map[string]string `json:"hashes"`
	}
	inventoryRaw, err := os.ReadFile(inventoryPath)
	inventoryDigest := ""
	if err == nil {
		inventoryDigest, err = rk1RuntimeContinuationHash(inventoryPath)
	}
	if err != nil || inventoryDigest != approval.Predecessor.InvocationReviewInventorySHA256 || json.Unmarshal(inventoryRaw, &inventory) != nil || !inventory.OriginalStopTriggered || inventory.NextScenarioAuthorized || inventory.ActualOriginalExit != 1 || inventory.Hashes[approval.Predecessor.InvocationReference] != approval.Predecessor.InvocationSHA256 {
		return nil, refuse
	}
	var result rk1Trial
	resultRaw, err := os.ReadFile(resultPath)
	if err != nil || json.Unmarshal(resultRaw, &result) != nil || result.Scenario != approval.Predecessor.Scenario || result.Label != approval.Predecessor.Label ||
		result.Trial != 2 || result.Mode != "real" || result.Status != approval.Predecessor.Status || result.Verdict != approval.Predecessor.Verdict || len(result.Runs) != 1 ||
		result.Runs[0].Launches != 1 || result.Runs[0].WrapperExitCode == nil || *result.Runs[0].WrapperExitCode != 143 {
		return nil, refuse
	}
	if len(approval.ContinuationContract.Order) != 4 {
		return nil, refuse
	}
	expected := []struct {
		scenario string
		trial    int
		launches int
	}{
		{"RK1-H3a", 3, 1}, {"RK1-H3b", 1, 3}, {"RK1-H5", 1, 3}, {"RK1-H6", 1, 6},
	}
	for i, item := range approval.ContinuationContract.Order {
		if item.Scenario != expected[i].scenario || item.StartTrial != expected[i].trial || item.Launches != expected[i].launches {
			return nil, refuse
		}
	}
	return &rk1RuntimeContinuationRegistration{ApprovalPath: path, ApprovalSHA: digest, RepoRoot: repoRoot, Approval: approval}, nil
}

func (r *rk1RuntimeContinuationRegistration) validateHistory(repoRoot string) error {
	if r == nil || r.Approval.ContinuationContract.Cap != 25 || r.Approval.ContinuationContract.PriorActualStarts != 12 || r.Approval.ContinuationContract.NewActualStartLimit != 13 {
		return errors.New("runtime continuation history budget is invalid")
	}
	if r.ApprovalPath == "" || !filepath.IsAbs(r.ApprovalPath) || !rk1RuntimeContinuationHex(r.ApprovalSHA) {
		return errors.New("runtime continuation approval binding is missing")
	}
	approvalDigest, err := rk1RuntimeContinuationHash(r.ApprovalPath)
	if err != nil || approvalDigest != r.ApprovalSHA {
		return errors.New("runtime continuation approval changed after loading")
	}
	if r.Approval.Predecessor.PublicResultSHA256 == "" || r.Approval.Predecessor.InvocationSHA256 == "" {
		return errors.New("runtime continuation predecessor hashes are missing")
	}
	if r.Approval.SourceBinding.HistoricalSourceHead != rk1RuntimeContinuationHistoricalHead ||
		r.Approval.SourceBinding.HistoricalSourceID != rk1RuntimeContinuationHistoricalSourceID ||
		r.Approval.SourceBinding.FrameworkSHA256 != rk1RuntimeContinuationHistoricalFramework ||
		r.Approval.Predecessor.PublicResultSHA256 != rk1RuntimeContinuationHistoricalResult ||
		r.Approval.Predecessor.InvocationSHA256 != rk1RuntimeContinuationHistoricalInvocation {
		return errors.New("runtime continuation source or predecessor binding is not the reviewed history")
	}
	resultPath, err := rk1RuntimeContinuationResolve(repoRoot, r.Approval.Predecessor.PublicResult)
	if err != nil {
		return err
	}
	resultDigest, err := rk1RuntimeContinuationHash(resultPath)
	if err != nil || resultDigest != r.Approval.Predecessor.PublicResultSHA256 {
		return errors.New("runtime continuation predecessor result changed")
	}
	inventoryPath, err := rk1RuntimeContinuationResolve(repoRoot, r.Approval.Predecessor.InvocationReviewInventory)
	if err != nil {
		return err
	}
	inventoryDigest, err := rk1RuntimeContinuationHash(inventoryPath)
	if err != nil || inventoryDigest != r.Approval.Predecessor.InvocationReviewInventorySHA256 {
		return errors.New("runtime continuation invocation review inventory changed")
	}
	if r.Ledger == "" || !filepath.IsAbs(r.Ledger) || r.Output == "" || !filepath.IsAbs(r.Output) {
		return errors.New("runtime continuation ledger/output must be absolute")
	}
	if filepath.Clean(r.Ledger) == filepath.Clean(resultPath) || filepath.Clean(r.Output) == filepath.Clean(filepath.Dir(resultPath)) || rk1RuntimeContinuationPathsOverlap(r.Output, filepath.Dir(resultPath)) {
		return errors.New("runtime continuation overlaps historical evidence")
	}
	if info, err := os.Lstat(r.Ledger); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("runtime continuation ledger is not a regular file")
		}
		for _, historical := range []string{resultPath, r.ApprovalPath} {
			historicalInfo, historicalErr := os.Stat(historical)
			if historicalErr == nil && os.SameFile(info, historicalInfo) {
				return errors.New("runtime continuation ledger aliases historical evidence")
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func rk1RuntimeContinuationOrder() []rk1RuntimeContinuationSlot {
	var slots []rk1RuntimeContinuationSlot
	appendTrials := func(scenario string, trial, launches int) {
		for launch := 1; launch <= launches; launch++ {
			slots = append(slots, rk1RuntimeContinuationSlot{Scenario: scenario, Trial: trial, Launch: launch})
		}
	}
	appendTrials("RK1-H3a", 3, 1)
	for trial := 1; trial <= 3; trial++ {
		appendTrials("RK1-H3b", trial, 1)
	}
	for trial := 1; trial <= 3; trial++ {
		appendTrials("RK1-H5", trial, 1)
	}
	for trial := 1; trial <= 3; trial++ {
		appendTrials("RK1-H6", trial, 2)
	}
	return slots
}

func rk1RuntimeContinuationLock(path string) (*os.File, error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.New("runtime continuation ledger is already owned")
	}
	if err = lock.Close(); err != nil {
		_ = os.Remove(path + ".lock")
		return nil, err
	}
	return lock, nil
}

func rk1RuntimeContinuationWithLedger(r *rk1RuntimeContinuationRegistration, change func(*rk1RuntimeContinuationLedger) error) error {
	if err := r.validateHistory(r.RepoRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.Ledger), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(r.Ledger); err != nil {
		return err
	}
	if _, err := rk1RuntimeContinuationLock(r.Ledger); err != nil {
		return err
	}
	defer os.Remove(r.Ledger + ".lock")
	raw, err := os.ReadFile(r.Ledger)
	if err != nil {
		return err
	}
	var ledger rk1RuntimeContinuationLedger
	if err = json.Unmarshal(raw, &ledger); err != nil {
		return errors.New("runtime continuation ledger is invalid")
	}
	if ledger.SchemaVersion != 1 || ledger.Mode != rk1RuntimeContinuationMode || ledger.Cap != 25 || ledger.PriorActualStarts != 12 || ledger.NewActualStartLimit != 13 ||
		ledger.PredecessorResultSHA != r.Approval.Predecessor.PublicResultSHA256 || ledger.InvocationSHA != r.Approval.Predecessor.InvocationSHA256 {
		return errors.New("runtime continuation ledger is rebound or has the wrong budget")
	}
	if err = change(&ledger); err != nil {
		return err
	}
	return rk1WriteJSON(r.Ledger, ledger)
}

func (r *rk1RuntimeContinuationRegistration) reserve(item rk1RuntimeContinuationSlot) error {
	_, err := r.reserveSlot(item)
	return err
}

func (r *rk1RuntimeContinuationRegistration) reserveSlot(item rk1RuntimeContinuationSlot) (int, error) {
	sequence := 0
	err := rk1RuntimeContinuationWithLedger(r, func(ledger *rk1RuntimeContinuationLedger) error {
		order := rk1RuntimeContinuationOrder()
		if ledger.Stopped || len(ledger.Starts) >= len(order) || len(ledger.Starts) >= ledger.NewActualStartLimit || len(ledger.Starts) > 0 && !ledger.Starts[len(ledger.Starts)-1].Complete {
			return errors.New("runtime continuation is stopped, exhausted, or has an unfinished predecessor")
		}
		want := order[len(ledger.Starts)]
		if item.Scenario != want.Scenario || item.Trial != want.Trial || item.Launch != want.Launch {
			return fmt.Errorf("runtime continuation next slot is %s-t%d launch %d", want.Scenario, want.Trial, want.Launch)
		}
		item.Sequence = len(ledger.Starts) + 1
		sequence = item.Sequence
		item.ReservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		ledger.Starts = append(ledger.Starts, item)
		return nil
	})
	return sequence, err
}

func (r *rk1RuntimeContinuationRegistration) started(sequence int) error {
	return rk1RuntimeContinuationWithLedger(r, func(ledger *rk1RuntimeContinuationLedger) error {
		if sequence != len(ledger.Starts) || sequence < 1 || ledger.Stopped || ledger.Starts[sequence-1].ActualStarted {
			return errors.New("runtime continuation actual-start mismatch")
		}
		ledger.Starts[sequence-1].ActualStarted = true
		ledger.Starts[sequence-1].StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
}

// The Provider wrapper calls this after its own process has created a unique
// launch directory and before it can invoke the external CLI. It deliberately
// does not accept a registration path or any new budget: the parent already
// reserved the final slot and only this exact sequence may transition.
func rk1RuntimeContinuationMarkStartedPath(path string, sequence int) error {
	if !filepath.IsAbs(path) || sequence < 1 {
		return errors.New("runtime continuation actual-start path is invalid")
	}
	lock, err := rk1RuntimeContinuationLock(path)
	if err != nil {
		return err
	}
	defer os.Remove(path + ".lock")
	_ = lock.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var ledger rk1RuntimeContinuationLedger
	if json.Unmarshal(raw, &ledger) != nil || ledger.SchemaVersion != 1 || ledger.Mode != rk1RuntimeContinuationMode || sequence != len(ledger.Starts) || ledger.Stopped || ledger.Starts[sequence-1].ActualStarted {
		return errors.New("runtime continuation actual-start marker is invalid")
	}
	ledger.Starts[sequence-1].ActualStarted = true
	ledger.Starts[sequence-1].StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return rk1WriteJSON(path, ledger)
}

func (r *rk1RuntimeContinuationRegistration) finish(sequence int, complete bool) error {
	return rk1RuntimeContinuationWithLedger(r, func(ledger *rk1RuntimeContinuationLedger) error {
		if sequence != len(ledger.Starts) || sequence < 1 || ledger.Stopped || ledger.Starts[sequence-1].ProcessComplete || complete && !ledger.Starts[sequence-1].ActualStarted {
			return errors.New("runtime continuation process finalization mismatch")
		}
		ledger.Starts[sequence-1].ProcessComplete = complete
		if !complete {
			ledger.Stopped = true
		}
		return nil
	})
}

func (r *rk1RuntimeContinuationRegistration) accept(sequence int, verdict string) error {
	return r.acceptEvidence(sequence, verdict, "", "", false)
}

func (r *rk1RuntimeContinuationRegistration) acceptEvidence(sequence int, verdict, evidencePath, evidenceSHA string, evidencePersisted bool) error {
	return rk1RuntimeContinuationWithLedger(r, func(ledger *rk1RuntimeContinuationLedger) error {
		if sequence != len(ledger.Starts) || sequence < 1 || ledger.Stopped || !ledger.Starts[sequence-1].ActualStarted || !ledger.Starts[sequence-1].ProcessComplete || ledger.Starts[sequence-1].Complete {
			return errors.New("runtime continuation sample does not match the physical start")
		}
		if evidencePersisted {
			if !filepath.IsAbs(evidencePath) || !rk1RuntimeContinuationHex(evidenceSHA) {
				return errors.New("runtime continuation evidence binding is invalid")
			}
			if got, err := rk1RuntimeContinuationHash(evidencePath); err != nil || got != evidenceSHA {
				return errors.New("runtime continuation evidence changed before acceptance")
			}
			for i, start := range ledger.Starts {
				if i != sequence-1 && filepath.Clean(start.EvidencePath) == filepath.Clean(evidencePath) {
					return errors.New("runtime continuation evidence was reused")
				}
			}
		}
		if strings.TrimSpace(verdict) == "" {
			return errors.New("runtime continuation verdict is empty")
		}
		ledger.Starts[sequence-1].Verdict = verdict
		ledger.Starts[sequence-1].EvidencePath = evidencePath
		ledger.Starts[sequence-1].EvidenceSHA256 = evidenceSHA
		ledger.Starts[sequence-1].EvidencePersisted = evidencePersisted
		if rk1RuntimeContinuationStopsVerdict(verdict) {
			ledger.Stopped = true
			return nil
		}
		ledger.Starts[sequence-1].Complete = true
		return nil
	})
}

func (r *rk1RuntimeContinuationRegistration) stop(sequence int, verdict string) error {
	return rk1RuntimeContinuationWithLedger(r, func(ledger *rk1RuntimeContinuationLedger) error {
		if sequence < 1 || sequence != len(ledger.Starts) || !ledger.Starts[sequence-1].ActualStarted || ledger.Starts[sequence-1].Complete {
			return errors.New("runtime continuation stop transition mismatch")
		}
		ledger.Starts[sequence-1].Verdict = verdict
		ledger.Stopped = true
		return nil
	})
}

func rk1RuntimeContinuationStopsVerdict(verdict string) bool {
	value := strings.ToLower(strings.TrimSpace(verdict))
	if value == "blocking_review" {
		return true
	}
	for _, marker := range []string{"incomplete", "provider_result_missing", "provider_reported_error", "public_record_missing", "unexposed", "runtime", "environment", "harness", "canary"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

type rk1RuntimeContinuationOutcome struct {
	ProcessComplete   bool
	EvidencePersisted bool
	EvidencePath      string
	EvidenceSHA256    string
	Verdict           string
}

type rk1RuntimeContinuationPlanBinding struct {
	registration *rk1RuntimeContinuationRegistration
	scenario     string
	trial        int
	mu           sync.Mutex
	sequences    []int
	err          error
}

func (b *rk1RuntimeContinuationPlanBinding) bind(plan rk1Plan) rk1Plan {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		plan.Real = ""
		return plan
	}
	raw, err := os.ReadFile(b.registration.Ledger)
	if err != nil {
		b.err = err
		plan.Real = ""
		return plan
	}
	var ledger rk1RuntimeContinuationLedger
	if json.Unmarshal(raw, &ledger) != nil || len(ledger.Starts) >= len(rk1RuntimeContinuationOrder()) {
		b.err = errors.New("runtime continuation plan ledger is invalid")
		plan.Real = ""
		return plan
	}
	want := rk1RuntimeContinuationOrder()[len(ledger.Starts)]
	if want.Scenario != b.scenario || want.Trial != b.trial {
		b.err = fmt.Errorf("runtime continuation plan slot is %s-t%d", want.Scenario, want.Trial)
		plan.Real = ""
		return plan
	}
	sequence, err := b.registration.reserveSlot(want)
	if err != nil {
		b.err = err
		plan.Real = ""
		return plan
	}
	b.sequences = append(b.sequences, sequence)
	plan.ContinuationLedger, plan.ContinuationSequence = b.registration.Ledger, sequence
	return plan
}

// runSlot is the common lifecycle used by both the task-local real launcher
// and fake controls. The callback receives the only legal actual-start
// transition. A reservation alone never consumes a slot; once started() has
// succeeded, every exit path keeps the debit and either accepts the sample or
// stops the continuation.
func (r *rk1RuntimeContinuationRegistration) runSlot(item rk1RuntimeContinuationSlot, run func(func() error) (rk1RuntimeContinuationOutcome, error)) error {
	if r == nil || r.Execution == nil {
		return errors.New("runtime continuation has no current execution binding")
	}
	sequence, err := r.reserveSlot(item)
	if err != nil {
		return err
	}
	started := false
	markStarted := func() error {
		if started {
			return errors.New("runtime continuation actual start was repeated")
		}
		if err := r.started(sequence); err != nil {
			return err
		}
		started = true
		return nil
	}
	outcome, runErr := run(markStarted)
	if !started {
		_ = r.finish(sequence, false)
		if runErr != nil {
			return runErr
		}
		return errors.New("runtime continuation reserved a slot without an actual start")
	}
	if !outcome.ProcessComplete {
		if err := r.finish(sequence, false); err != nil {
			return errors.Join(runErr, err)
		}
		return errors.Join(runErr, errors.New("runtime continuation process did not terminate"))
	}
	if err := r.finish(sequence, true); err != nil {
		return errors.Join(runErr, err)
	}
	if runErr != nil {
		if err := r.acceptEvidence(sequence, "runtime_error:"+runErr.Error(), "", "", false); err != nil {
			return errors.Join(runErr, r.stop(sequence, "runtime_error:evidence_review_failed"), err)
		}
		return runErr
	}
	if !outcome.EvidencePersisted {
		if err := r.acceptEvidence(sequence, "harness_fault:evidence_persistence_failed", "", "", false); err != nil {
			return errors.Join(r.stop(sequence, "harness_fault:evidence_persistence_failed"), err)
		}
		return errors.New("runtime continuation evidence was not persisted")
	}
	if err := r.acceptEvidence(sequence, outcome.Verdict, outcome.EvidencePath, outcome.EvidenceSHA256, true); err != nil {
		return errors.Join(r.stop(sequence, "harness_fault:evidence_binding_failed"), err)
	}
	return nil
}

func rk1RuntimeContinuationStringHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func rk1RuntimeContinuationFrameworkDigest(root string) (string, error) {
	files, err := filepath.Glob(filepath.Join(root, "server", "cmd", "server", "rk1_*_test.go"))
	if err != nil || len(files) == 0 {
		return "", errors.New("runtime continuation framework files are missing")
	}
	slices.Sort(files)
	h := sha256.New()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.Base(path), len(content))
		_, _ = h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func rk1RuntimeContinuationValidateCurrentBinding(r *rk1RuntimeContinuationRegistration) error {
	if r == nil || r.Execution == nil {
		return errors.New("runtime continuation requires a fresh current binding")
	}
	b := r.Execution
	if r.ExecutionPath == "" || !filepath.IsAbs(r.ExecutionPath) || !rk1RuntimeContinuationHex(r.ExecutionSHA) {
		return errors.New("runtime continuation current registration binding is missing")
	}
	if got, err := rk1RuntimeContinuationHash(r.ExecutionPath); err != nil || got != r.ExecutionSHA {
		return errors.New("runtime continuation current registration changed")
	}
	if b.ApprovalPath != "" && b.ApprovalSHA != r.ApprovalSHA {
		return errors.New("runtime continuation current registration points at another approval")
	}
	if b.Historical.Consumed != 12 || b.Historical.Remaining != 13 || b.Historical.ResultSHA != rk1RuntimeContinuationHistoricalResult ||
		b.Historical.InvocationSHA != rk1RuntimeContinuationHistoricalInvocation || b.Historical.ReviewInventorySHA != "e5807dddbc40ff880b431fb5ba002457c407fb2d77b75ee76d3b3841d30b991b" {
		return errors.New("runtime continuation current registration lost the historical debit")
	}
	allReady := b.Current.SourceBindingReady && b.Current.FrameworkBindingReady && b.Current.CLIReady && b.Current.ConfigurationReady &&
		b.Current.DatabaseReady && b.Current.WindowReady && b.Current.PermitReady && b.Current.GatewayReady
	if !allReady {
		return errors.New("runtime continuation current binding is not ready")
	}
	if b.Current.Window.ID == "" || b.Current.Window.Owner == "" || b.Current.Window.ExpiresAt == "" ||
		b.Current.Permit.ID == "" || b.Current.Permit.IssuedAt == "" || b.Current.Permit.ExpiresAt == "" ||
		b.Current.Gateway.AdmissionID == "" || b.Current.Gateway.Provider == "" || b.Current.Gateway.Evidence == "" {
		return errors.New("runtime continuation window, permit, or gateway binding is incomplete")
	}
	if !b.ExecutionGate.Runnable || !b.ExecutionGate.AllowBuild || !b.ExecutionGate.AllowCheck || !b.ExecutionGate.AllowRun {
		return errors.New("runtime continuation current execution gate is closed")
	}
	manifestPath, err := rk1RuntimeContinuationResolve(r.RepoRoot, b.Current.SourceManifest)
	if err != nil {
		return err
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return errors.New("runtime continuation source manifest is unavailable")
	}
	if got := rk1RuntimeContinuationStringHash(string(manifestRaw)); got != b.Current.SourceManifestSHA {
		return errors.New("runtime continuation source manifest changed")
	}
	var manifest struct {
		Head     string   `json:"head"`
		SourceID string   `json:"source_id"`
		Dirty    []string `json:"dirty_product_paths"`
	}
	if json.Unmarshal(manifestRaw, &manifest) != nil || manifest.Head != b.Current.Head || manifest.SourceID != b.Current.SourceID || len(manifest.Dirty) != 0 {
		return errors.New("runtime continuation source binding is dirty or drifted")
	}
	framework, err := rk1RuntimeContinuationFrameworkDigest(r.RepoRoot)
	if err != nil || framework != b.Current.FrameworkSHA {
		return errors.New("runtime continuation framework binding changed")
	}
	launcher, err := rk1RuntimeContinuationHash(b.Current.LauncherPath)
	if err != nil || launcher != b.Current.LauncherSHA {
		return errors.New("runtime continuation launcher binding changed")
	}
	provider, err := rk1RuntimeContinuationHash(b.Current.Provider.Executable)
	if err != nil || provider != b.Current.Provider.ExecutableSHA {
		return errors.New("runtime continuation Provider executable changed")
	}
	operatorConfigSHA := strings.TrimSpace(os.Getenv("RK1_RUNTIME_CONTINUATION_OPERATOR_CONFIG_SHA256"))
	if !rk1RuntimeContinuationHex(operatorConfigSHA) || operatorConfigSHA != b.Current.OperatorConfigSHA {
		return errors.New("runtime continuation operator configuration changed")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	db, err := url.Parse(databaseURL)
	if err != nil || strings.TrimPrefix(db.Path, "/") != b.Current.Database.Name || !strings.HasPrefix(strings.TrimPrefix(db.Path, "/"), rk1DatabasePrefix) || rk1RuntimeContinuationStringHash(strings.TrimPrefix(db.Path, "/")) != b.Current.Database.IdentitySHA {
		return errors.New("runtime continuation database binding changed")
	}
	for _, value := range []string{b.Current.Window.ExpiresAt, b.Current.Permit.ExpiresAt} {
		expires, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || !expires.After(time.Now().UTC()) {
			return errors.New("runtime continuation window or permit is expired")
		}
	}
	return nil
}

// rk1RuntimeContinuationRealAdmission is intentionally fail-closed. A future
// real runner must provide a new execution registration with current source,
// CLI, config, database and exclusive-window bindings; this approval alone
// never makes a provider launch runnable.
func rk1RuntimeContinuationRealAdmission(r *rk1RuntimeContinuationRegistration, scenario string, trial int) error {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		return errors.New("runtime continuation real execution requires the explicit provider gate")
	}
	if err := rk1RuntimeContinuationValidateCurrentBinding(r); err != nil {
		return err
	}
	order := rk1RuntimeContinuationOrder()
	if len(order) == 0 {
		return errors.New("runtime continuation has no registered slots")
	}
	raw, err := os.ReadFile(r.Ledger)
	if err != nil {
		return errors.New("runtime continuation ledger is unavailable")
	}
	var ledger rk1RuntimeContinuationLedger
	if json.Unmarshal(raw, &ledger) != nil || ledger.SchemaVersion != 1 || ledger.Mode != rk1RuntimeContinuationMode || ledger.Cap != 25 || ledger.PriorActualStarts != 12 || ledger.NewActualStartLimit != 13 || ledger.Stopped {
		return errors.New("runtime continuation ledger is not runnable")
	}
	if len(ledger.Starts) >= len(order) || (len(ledger.Starts) > 0 && !ledger.Starts[len(ledger.Starts)-1].Complete) {
		return errors.New("runtime continuation ledger has no available successor slot")
	}
	want := order[len(ledger.Starts)]
	if want.Scenario != scenario || want.Trial != trial {
		return errors.New("runtime continuation must start at the next unattempted slot")
	}
	return nil
}

func rk1RuntimeContinuationFinishRealTrial(r *rk1RuntimeContinuationRegistration, binding *rk1RuntimeContinuationPlanBinding, trial rk1Trial, output string) error {
	binding.mu.Lock()
	sequences := append([]int(nil), binding.sequences...)
	binding.mu.Unlock()
	var measured []rk1RunSummary
	for _, run := range trial.Runs {
		if run.Measured {
			measured = append(measured, run)
		}
	}
	if len(measured) != len(sequences) {
		for _, sequence := range sequences {
			_ = r.stop(sequence, "harness_fault:measured_start_count_mismatch")
		}
		return errors.New("runtime continuation measured-start count does not match reservations")
	}
	for index, sequence := range sequences {
		raw, err := os.ReadFile(r.Ledger)
		if err != nil {
			return err
		}
		var ledger rk1RuntimeContinuationLedger
		if json.Unmarshal(raw, &ledger) != nil || sequence < 1 || sequence > len(ledger.Starts) {
			return errors.New("runtime continuation result ledger is invalid")
		}
		start := ledger.Starts[sequence-1]
		if !start.ActualStarted {
			if err := r.finish(sequence, false); err != nil {
				return err
			}
			continue
		}
		if err := r.finish(sequence, true); err != nil {
			return err
		}
		launches, _ := filepath.Glob(filepath.Join(output, trial.Label, "provider", measured[index].RunID, "launch-*", "launch.json"))
		slices.Sort(launches)
		if len(launches) != 1 {
			if err := r.acceptEvidence(sequence, "harness_fault:public_record_missing", "", "", false); err != nil {
				return errors.Join(r.stop(sequence, "harness_fault:public_record_missing"), err)
			}
			continue
		}
		launchRaw, err := os.ReadFile(launches[index])
		if err != nil {
			return err
		}
		var launchMeta struct {
			FinishedAt    string `json:"finished_at"`
			ExitCode      *int   `json:"exit_code"`
			EvidenceState string `json:"evidence_state"`
		}
		if json.Unmarshal(launchRaw, &launchMeta) != nil || launchMeta.FinishedAt == "" || launchMeta.ExitCode == nil {
			if err := r.stop(sequence, "harness_fault:process_termination_unrecorded"); err != nil {
				return err
			}
			continue
		}
		evidenceSHA, err := rk1RuntimeContinuationHash(launches[index])
		if err != nil {
			return err
		}
		verdict := trial.Verdict
		if launchMeta.EvidenceState != "complete" {
			verdict = "harness_fault:evidence_" + launchMeta.EvidenceState
		}
		if trial.Status != "finished" {
			verdict = "incomplete:" + verdict
		}
		if err := r.acceptEvidence(sequence, verdict, launches[index], evidenceSHA, true); err != nil {
			return errors.Join(r.stop(sequence, "harness_fault:evidence_binding_failed"), err)
		}
	}
	if binding.err != nil {
		return binding.err
	}
	return nil
}

// TestRK1RuntimeContinuationReal is the task-local successor entry. It runs
// one registered scenario/trial per invocation, reusing rk1Series and the
// original scenario/collection code while keeping a separate continuation
// ledger and output namespace. It is unreachable with the checked-in
// registration because all current gates are deliberately false.
func TestRK1RuntimeContinuationReal(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("set MULTICA_RUN_REAL_AGENT_SMOKE=1 only through the task-local continuation launcher")
	}
	repo := filepath.Dir(rk1Module(t))
	approvalPath := filepath.Join(repo, ".trellis", "tasks", "10-30-collaboration-context-handoff-runtime", "research", "continuation-20261111", "approval-registration.json")
	approvalSHA, err := rk1RuntimeContinuationHash(approvalPath)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := rk1RuntimeContinuationLoadApproval(approvalPath, approvalSHA, repo)
	if err != nil {
		t.Fatal(err)
	}
	registrationPath := os.Getenv("RK1_RUNTIME_CONTINUATION_REGISTRATION")
	registrationSHA := os.Getenv("RK1_RUNTIME_CONTINUATION_REGISTRATION_SHA256")
	if registrationPath == "" || registrationSHA == "" {
		t.Fatal("task-local continuation registration and sha256 are required")
	}
	r, err := rk1RuntimeContinuationLoadExecutionRegistration(registrationPath, registrationSHA, repo, approval)
	if err != nil {
		t.Fatal(err)
	}
	scenario := os.Getenv("RK1_RUNTIME_CONTINUATION_SCENARIO")
	trial, err := strconv.Atoi(os.Getenv("RK1_RUNTIME_CONTINUATION_TRIAL"))
	if err != nil || scenario == "" {
		t.Fatal("task-local continuation scenario and trial are required")
	}
	if err := rk1RuntimeContinuationRealAdmission(r, scenario, trial); err != nil {
		t.Fatal(err)
	}
	sc, ok := rk1Scenarios[scenario]
	if !ok || trial < 1 || trial > sc.trials {
		t.Fatal("task-local continuation scenario is not in the frozen RK1 table")
	}
	if err := os.MkdirAll(r.Output, 0700); err != nil {
		t.Fatal(err)
	}
	env := rk1BuildEnv(t, r.Execution.Current.Provider.Executable, r.Execution.Current.Provider.Version, r.Execution.Current.Provider.Model)
	binding := &rk1RuntimeContinuationPlanBinding{registration: r, scenario: scenario, trial: trial}
	env.continuation = binding
	trials := rk1Series(t, env, r.Output, sc, trial, trial, func(int) rk1Mode { return rk1Mode{real: true} })
	if len(trials) != 1 {
		t.Fatal("task-local continuation did not produce one trial")
	}
	if err := rk1RuntimeContinuationFinishRealTrial(r, binding, trials[0], r.Output); err != nil {
		t.Fatal(err)
	}
	if rk1Stops(trials[0]) {
		t.Fatalf("continuation stopped after %s: %s", trials[0].Status, trials[0].Verdict)
	}
}

func TestRK1RuntimeContinuationControls(t *testing.T) {
	if os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("runtime continuation controls refuse the real-agent gate")
	}
	repo := filepath.Dir(rk1Module(t))
	approvalPath := filepath.Join(repo, ".trellis", "tasks", "10-30-collaboration-context-handoff-runtime", "research", "continuation-20261111", "approval-registration.json")
	digest, err := rk1RuntimeContinuationHash(approvalPath)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := rk1RuntimeContinuationLoadApproval(approvalPath, digest, repo)
	if err != nil {
		t.Fatal(err)
	}
	registrationPath := filepath.Join(repo, ".trellis", "tasks", "10-30-collaboration-context-handoff-runtime", "research", "continuation-20261112", "real-execution-registration.json")
	registrationSHA, err := rk1RuntimeContinuationHash(registrationPath)
	if err != nil {
		t.Fatal(err)
	}
	current, err := rk1RuntimeContinuationLoadExecutionRegistration(registrationPath, registrationSHA, repo, approval)
	if err != nil {
		t.Fatal("current continuation registration could not be loaded: ", err)
	}
	if current.Execution == nil || current.Execution.ExecutionGate.Runnable || current.Execution.ExecutionGate.AllowRun {
		t.Fatal("current continuation registration opened the real gate")
	}

	t.Run("approval_is_implementation_only", func(t *testing.T) {
		if approval.Approval.ExecutionGate.AllowRun || approval.Approval.ExecutionGate.Runnable || approval.Approval.Approval.RealExecutionApproved {
			t.Fatal("approval opened the real execution gate")
		}
		if err := rk1RuntimeContinuationRealAdmission(approval, "RK1-H3a", 3); err == nil {
			t.Fatal("closed real gate admitted continuation")
		}
		t.Setenv("MULTICA_RUN_REAL_AGENT_SMOKE", "1")
		if err := rk1RuntimeContinuationRealAdmission(approval, "RK1-H3a", 3); err == nil {
			t.Fatal("implementation approval admitted continuation after the provider gate was set")
		}
	})

	t.Run("current_binding_refuses_until_every_gate_is_proven", func(t *testing.T) {
		if err := rk1RuntimeContinuationRealAdmission(current, "RK1-H3a", 3); err == nil {
			t.Fatal("default-closed current binding admitted a real continuation")
		}
		if err := rk1RuntimeContinuationRealAdmission(current, "RK1-H3a", 2); err == nil {
			t.Fatal("historical slot was admitted through the current binding")
		}
	})

	t.Run("lifecycle_records_reservation_actual_start_termination_and_evidence", func(t *testing.T) {
		reg := *approval
		reg.Execution = &rk1RuntimeContinuationExecutionBinding{}
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		evidence := filepath.Join(reg.Output, "h3a-t3.json")
		if err := os.MkdirAll(filepath.Dir(evidence), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(evidence, []byte(`{"status":"finished","verdict":"pass"}\n`), 0600); err != nil {
			t.Fatal(err)
		}
		evidenceSHA, err := rk1RuntimeContinuationHash(evidence)
		if err != nil {
			t.Fatal(err)
		}
		if err := reg.runSlot(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}, func(start func() error) (rk1RuntimeContinuationOutcome, error) {
			if err := start(); err != nil {
				return rk1RuntimeContinuationOutcome{}, err
			}
			return rk1RuntimeContinuationOutcome{ProcessComplete: true, EvidencePersisted: true, EvidencePath: evidence, EvidenceSHA256: evidenceSHA, Verdict: "pass"}, nil
		}); err != nil {
			t.Fatal(err)
		}
		ledger := readRuntimeContinuationLedger(t, reg.Ledger)
		if len(ledger.Starts) != 1 || !ledger.Starts[0].ActualStarted || !ledger.Starts[0].ProcessComplete || !ledger.Starts[0].Complete || ledger.Starts[0].EvidencePath != evidence || ledger.Starts[0].EvidenceSHA256 != evidenceSHA {
			t.Fatalf("lifecycle evidence was not bound: %+v", ledger.Starts)
		}
	})

	t.Run("reservation_without_start_is_not_a_debit_and_stops", func(t *testing.T) {
		reg := *approval
		reg.Execution = &rk1RuntimeContinuationExecutionBinding{}
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		if err := reg.runSlot(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}, func(func() error) (rk1RuntimeContinuationOutcome, error) {
			return rk1RuntimeContinuationOutcome{ProcessComplete: true, EvidencePersisted: true, Verdict: "pass"}, nil
		}); err == nil {
			t.Fatal("reservation without a process start was accepted")
		}
		ledger := readRuntimeContinuationLedger(t, reg.Ledger)
		if len(ledger.Starts) != 1 || ledger.Starts[0].ActualStarted || !ledger.Stopped || ledger.Starts[0].Complete {
			t.Fatalf("reservation was treated as an actual start: %+v", ledger.Starts)
		}
	})

	t.Run("started_process_without_evidence_stops_and_keeps_debit", func(t *testing.T) {
		reg := *approval
		reg.Execution = &rk1RuntimeContinuationExecutionBinding{}
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		if err := reg.runSlot(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}, func(start func() error) (rk1RuntimeContinuationOutcome, error) {
			if err := start(); err != nil {
				return rk1RuntimeContinuationOutcome{}, err
			}
			return rk1RuntimeContinuationOutcome{ProcessComplete: true, EvidencePersisted: false, Verdict: "pass"}, nil
		}); err == nil {
			t.Fatal("missing evidence was accepted")
		}
		ledger := readRuntimeContinuationLedger(t, reg.Ledger)
		if len(ledger.Starts) != 1 || !ledger.Starts[0].ActualStarted || !ledger.Starts[0].ProcessComplete || !ledger.Stopped || ledger.Starts[0].Complete {
			t.Fatalf("missing evidence did not retain debit and stop: %+v", ledger.Starts)
		}
	})

	t.Run("historical_source_drift_refuses", func(t *testing.T) {
		for _, mutate := range []func(*rk1RuntimeContinuationApproval){
			func(a *rk1RuntimeContinuationApproval) { a.SourceBinding.HistoricalSourceHead = "different-head" },
			func(a *rk1RuntimeContinuationApproval) { a.SourceBinding.HistoricalSourceID = "different-source" },
			func(a *rk1RuntimeContinuationApproval) { a.SourceBinding.FrameworkSHA256 = "different-framework" },
		} {
			reg := *approval
			mutate(&reg.Approval)
			reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
			reg.Output = filepath.Join(t.TempDir(), "continuation-output")
			if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err == nil {
				t.Fatal("historical source/framework drift admitted a continuation ledger")
			}
		}
	})

	t.Run("source_binding_drift_refuses", func(t *testing.T) {
		raw, err := os.ReadFile(approvalPath)
		if err != nil {
			t.Fatal(err)
		}
		drifted := strings.Replace(string(raw), rk1RuntimeContinuationHistoricalHead, strings.Repeat("0", len(rk1RuntimeContinuationHistoricalHead)), 1)
		path := filepath.Join(t.TempDir(), "approval-drift.json")
		if err := os.WriteFile(path, []byte(drifted), 0600); err != nil {
			t.Fatal(err)
		}
		digest, err := rk1RuntimeContinuationHash(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rk1RuntimeContinuationLoadApproval(path, digest, repo); err == nil {
			t.Fatal("source-drifted continuation approval was accepted")
		}
	})

	t.Run("physical_accounting_and_order", func(t *testing.T) {
		reg := *approval
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		for _, item := range rk1RuntimeContinuationOrder() {
			if err := reg.reserve(item); err != nil {
				t.Fatalf("reserve %s-t%d/%d: %v", item.Scenario, item.Trial, item.Launch, err)
			}
			seq := len(readRuntimeContinuationStarts(t, reg.Ledger))
			if err := reg.started(seq); err != nil {
				t.Fatal(err)
			}
			if err := reg.finish(seq, true); err != nil {
				t.Fatal(err)
			}
			if err := reg.accept(seq, "pass"); err != nil {
				t.Fatal(err)
			}
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H6", Trial: 3, Launch: 2}); err == nil {
			t.Fatal("fourteenth physical start admitted")
		}
		ledger := readRuntimeContinuationLedger(t, reg.Ledger)
		if len(ledger.Starts) != 13 || ledger.Stopped {
			t.Fatalf("unexpected completed continuation ledger: %+v", ledger)
		}
		oldResult, err := rk1RuntimeContinuationHash(filepath.Join(repo, approval.Approval.Predecessor.PublicResult))
		if err != nil || oldResult != approval.Approval.Predecessor.PublicResultSHA256 {
			t.Fatal("historical predecessor changed")
		}
	})

	t.Run("wrong_order_and_incomplete_stop", func(t *testing.T) {
		reg := *approval
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 2, Launch: 1}); err == nil {
			t.Fatal("historical H3a trial2 was admitted")
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}); err != nil {
			t.Fatal(err)
		}
		if err := reg.started(1); err != nil {
			t.Fatal(err)
		}
		if err := reg.finish(1, false); err != nil {
			t.Fatal(err)
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3b", Trial: 1, Launch: 1}); err == nil {
			t.Fatal("continuation continued after an incomplete result")
		}
	})

	t.Run("runtime_and_harness_verdicts_stop_after_process_exit", func(t *testing.T) {
		for _, verdict := range []string{"measured:provider_result_missing", "runtime_error", "environment_failure", "harness_fault:public_record_missing"} {
			t.Run(verdict, func(t *testing.T) {
				reg := *approval
				reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
				reg.Output = filepath.Join(t.TempDir(), "continuation-output")
				if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
					t.Fatal(err)
				}
				if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}); err != nil {
					t.Fatal(err)
				}
				if err := reg.started(1); err != nil {
					t.Fatal(err)
				}
				if err := reg.finish(1, true); err != nil {
					t.Fatal(err)
				}
				if err := reg.accept(1, verdict); err != nil {
					t.Fatal(err)
				}
				if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3b", Trial: 1, Launch: 1}); err == nil {
					t.Fatal("continuation proceeded after a stop-class verdict")
				}
			})
		}
	})

	t.Run("duplicate_history_and_ledger_drift_refuse", func(t *testing.T) {
		reg := *approval
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		first := rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}
		if err := reg.reserve(first); err != nil {
			t.Fatal(err)
		}
		if err := reg.started(1); err != nil {
			t.Fatal(err)
		}
		if err := reg.finish(1, true); err != nil {
			t.Fatal(err)
		}
		if err := reg.accept(1, "pass"); err != nil {
			t.Fatal(err)
		}
		if err := reg.reserve(first); err == nil {
			t.Fatal("duplicate/replaced first slot admitted")
		}
		ledger := readRuntimeContinuationLedger(t, reg.Ledger)
		ledger.PriorActualStarts = 11
		if err := rk1WriteJSON(reg.Ledger, ledger); err != nil {
			t.Fatal(err)
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3b", Trial: 1, Launch: 1}); err == nil {
			t.Fatal("ledger with omitted prior debit admitted a new slot")
		}
	})

	t.Run("concurrent_and_output_alias_refuse", func(t *testing.T) {
		reg := *approval
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(repo, ".trellis", "tasks", "10-30-collaboration-context-handoff-runtime", "research", "real-model-rk1", "results")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err == nil {
			t.Fatal("historical output namespace was accepted")
		}
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reg.Ledger+".lock", []byte("owned"), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(reg.Ledger + ".lock")
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H3a", Trial: 3, Launch: 1}); err == nil {
			t.Fatal("concurrent continuation entry admitted")
		}
	})

	t.Run("h5_canary_stops", func(t *testing.T) {
		reg := *approval
		reg.Ledger = filepath.Join(t.TempDir(), "continuation-ledger.json")
		reg.Output = filepath.Join(t.TempDir(), "continuation-output")
		if err := rk1RuntimeContinuationWriteFreshLedger(&reg); err != nil {
			t.Fatal(err)
		}
		for _, item := range []rk1RuntimeContinuationSlot{{Scenario: "RK1-H3a", Trial: 3, Launch: 1}, {Scenario: "RK1-H3b", Trial: 1, Launch: 1}, {Scenario: "RK1-H3b", Trial: 2, Launch: 1}, {Scenario: "RK1-H3b", Trial: 3, Launch: 1}, {Scenario: "RK1-H5", Trial: 1, Launch: 1}} {
			if err := reg.reserve(item); err != nil {
				t.Fatal(err)
			}
			seq := len(readRuntimeContinuationStarts(t, reg.Ledger))
			if err := reg.started(seq); err != nil {
				t.Fatal(err)
			}
			if err := reg.finish(seq, true); err != nil {
				t.Fatal(err)
			}
			verdict := "pass"
			if item.Scenario == "RK1-H5" {
				verdict = "blocking_review"
			}
			if err := reg.accept(seq, verdict); err != nil {
				t.Fatal(err)
			}
		}
		if err := reg.reserve(rk1RuntimeContinuationSlot{Scenario: "RK1-H5", Trial: 2, Launch: 1}); err == nil {
			t.Fatal("H5 canary did not stop successor slots")
		}
	})
}

func rk1RuntimeContinuationWriteFreshLedger(r *rk1RuntimeContinuationRegistration) error {
	if err := r.validateHistory(r.RepoRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.Ledger), 0700); err != nil {
		return err
	}
	return rk1WriteJSON(r.Ledger, rk1RuntimeContinuationLedger{SchemaVersion: 1, Mode: rk1RuntimeContinuationMode, Cap: 25, PriorActualStarts: 12, NewActualStartLimit: 13, PredecessorResultSHA: r.Approval.Predecessor.PublicResultSHA256, InvocationSHA: r.Approval.Predecessor.InvocationSHA256})
}

func readRuntimeContinuationLedger(t *testing.T, path string) rk1RuntimeContinuationLedger {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ledger rk1RuntimeContinuationLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		t.Fatal(err)
	}
	return ledger
}

func readRuntimeContinuationStarts(t *testing.T, path string) []rk1RuntimeContinuationSlot {
	t.Helper()
	return readRuntimeContinuationLedger(t, path).Starts
}
