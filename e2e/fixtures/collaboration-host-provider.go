// Build this stdlib-only fixture explicitly for the native host under test.
// It speaks ordinary Claude stream-json, reads the issue through the actual
// Multica CLI, and never supplies a launch permit or termination receipt.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--fixture-kind" {
		fmt.Println("multica-host-e2e-provider-v1")
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("fixture-1 (Claude Code)")
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--fixture-capabilities" {
		fmt.Println(`{"test_report":true,"failure_release":true,"prepared_git_marker":true}`)
		return
	}
	if err := runHostFixture(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func fixtureDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// fixtureAction is a test-scripted business write. Comments and squad activity
// go through the run-local CLI and therefore the same capability, allowlist and
// request-id rules as an agent; write_file only touches the issue workdir.
type fixtureAction struct {
	Kind      string `json:"kind"`
	RequestID string `json:"request_id"`
	Content   string `json:"content"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason"`
	Path      string `json:"path"`
	Fault     string `json:"fault"`
}

func runFixtureAction(cli, issueID, work string, action fixtureAction) map[string]any {
	receipt := map[string]any{"kind": action.Kind, "request_id": action.RequestID}
	var args []string
	switch action.Kind {
	case "test_subject", "test_report":
		return runFixtureTestReport(cli, issueID, work, action)
	case "comment":
		args = []string{"issue", "comment", "add", issueID, "--request-id", action.RequestID, "--content", action.Content, "--output", "json"}
	case "squad_activity":
		args = []string{"squad", "activity", issueID, action.Outcome, "--request-id", action.RequestID, "--reason", action.Reason, "--output", "json"}
	case "write_file":
		if !filepath.IsLocal(action.Path) {
			receipt["error"] = "path was not local"
			return receipt
		}
		target := filepath.Join(work, action.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			receipt["error"] = "directory could not be created"
			return receipt
		}
		if err := os.WriteFile(target, []byte(action.Content), 0o600); err != nil {
			receipt["error"] = "file could not be written"
			return receipt
		}
		receipt["sha256"] = fixtureDigest([]byte(action.Content))
		return receipt
	default:
		receipt["error"] = "unknown action"
		return receipt
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, cli, args...)
	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := stderr.String()
		if len(message) > 2000 {
			message = message[:2000]
		}
		receipt["error"] = strings.TrimSpace(message)
		return receipt
	}
	var result json.RawMessage
	if json.Unmarshal(output, &result) != nil {
		receipt["error"] = "CLI output was not JSON"
		return receipt
	}
	receipt["result"] = result
	return receipt
}

// Do not embed Buffer: io.Copy would use its promoted ReaderFrom and bypass Write.
type fixtureBoundedOutput struct{ buffer bytes.Buffer }

func (b *fixtureBoundedOutput) Len() int       { return b.buffer.Len() }
func (b *fixtureBoundedOutput) Bytes() []byte  { return b.buffer.Bytes() }
func (b *fixtureBoundedOutput) String() string { return b.buffer.String() }

func (b *fixtureBoundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64<<10 {
		return 0, fmt.Errorf("fixture CLI output exceeded 64 KiB")
	}
	return b.buffer.Write(p)
}

func fixtureReportCLI(ctx context.Context, cli string, args []string, input []byte) (json.RawMessage, string, error) {
	command := exec.CommandContext(ctx, cli, args...)
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr fixtureBoundedOutput
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, stderr.String(), err
	}
	raw := bytes.TrimSpace(stdout.Bytes())
	if !json.Valid(raw) {
		return nil, "", fmt.Errorf("fixture CLI response was not JSON")
	}
	return append(json.RawMessage(nil), raw...), "", nil
}

// The tiny deterministic case reads the selected Git file. The fault switch is
// a fixture-only duplicate-side-effect injection, never a production override.
func fixturePaymentCases(raw []byte, fault string) ([]map[string]any, []map[string]string, error) {
	var marker struct {
		Version         int      `json:"version"`
		Revision        int      `json:"revision"`
		Callbacks       []string `json:"callbacks"`
		ExpectedEffects int      `json:"expected_effects"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 16<<10 || decoder.Decode(&marker) != nil || marker.Version != 1 || marker.Revision < 1 ||
		len(marker.Callbacks) < 1 || len(marker.Callbacks) > 16 || marker.ExpectedEffects < 1 || marker.ExpectedEffects > 16 ||
		(fault != "none" && fault != "duplicate_effect") {
		return nil, nil, fmt.Errorf("prepared test marker or fault selection was invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, nil, fmt.Errorf("prepared test marker had trailing data")
	}
	seen, effects := map[string]bool{}, 0
	for _, callback := range marker.Callbacks {
		if callback == "" || len(callback) > 128 {
			return nil, nil, fmt.Errorf("prepared callback identity was invalid")
		}
		if !seen[callback] || fault == "duplicate_effect" {
			effects++
		}
		seen[callback] = true
	}
	outcome := "passed"
	if effects != marker.ExpectedEffects {
		outcome = "failed"
	}
	cases := []map[string]any{{"id": "prepared-source", "outcome": "passed", "log_index": 0},
		{"id": "duplicate-callback", "outcome": outcome, "log_index": 1}}
	logs := []map[string]string{{"kind": "inline", "body": fmt.Sprintf("Synthetic prepared Git marker revision %d; SHA-256 %s", marker.Revision, fixtureDigest(raw))},
		{"kind": "inline", "body": fmt.Sprintf("Synthetic callback case: callbacks=%d effects=%d expected=%d injected_fault=%s", len(marker.Callbacks), effects, marker.ExpectedEffects, fault)}}
	return cases, logs, nil
}

func runFixtureTestReport(cli, issueID, work string, action fixtureAction) map[string]any {
	receipt := map[string]any{"kind": action.Kind, "request_id": action.RequestID}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	subjectArgs := []string{"collaboration", "test-report", "subject", issueID, "--output", "json"}
	subject, stderr, err := fixtureReportCLI(ctx, cli, subjectArgs, nil)
	if action.Kind == "test_subject" {
		if err != nil {
			for _, code := range []string{"test_subject_unavailable", "test_source_unbound"} {
				if strings.Contains(stderr, code) {
					receipt["result"] = map[string]any{"available": false, "code": code, "command": subjectArgs}
					return receipt
				}
			}
			receipt["error"] = "CLI test subject failed without a recognized unavailable result"
			return receipt
		}
		receipt["result"] = map[string]any{"available": true, "subject": subject, "stdout_sha256": fixtureDigest(subject), "command": subjectArgs}
		return receipt
	}
	if err != nil {
		receipt["error"] = "CLI test subject was unavailable"
		return receipt
	}
	var binding struct {
		Version int             `json:"version"`
		IssueID string          `json:"issue_id"`
		RunID   string          `json:"run_id"`
		Subject json.RawMessage `json:"subject"`
	}
	if json.Unmarshal(subject, &binding) != nil || binding.Version != 1 || binding.IssueID != issueID || !fixtureUUID.MatchString(binding.RunID) || !json.Valid(binding.Subject) {
		receipt["error"] = "CLI test subject identity was invalid"
		return receipt
	}
	markerFile, err := os.Open(filepath.Join(work, "stage-regression.json"))
	if err != nil {
		receipt["error"] = "prepared test marker was missing"
		return receipt
	}
	marker, readErr := io.ReadAll(io.LimitReader(markerFile, (16<<10)+1))
	closeErr := markerFile.Close()
	cases, logs, err := fixturePaymentCases(marker, action.Fault)
	if readErr != nil || closeErr != nil || err != nil {
		receipt["error"] = "prepared test marker was invalid"
		return receipt
	}
	report, err := json.Marshal(map[string]any{"subject": binding.Subject, "suite_id": "synthetic-payment-regression", "cases": cases, "logs": logs})
	if err != nil {
		receipt["error"] = "test report could not be encoded"
		return receipt
	}
	submitArgs := []string{"collaboration", "test-report", "submit", issueID, "--request-id", action.RequestID, "--report-stdin", "--output", "json"}
	submitted, _, err := fixtureReportCLI(ctx, cli, submitArgs, report)
	if err != nil {
		receipt["error"] = "CLI test report submit failed; keep the original request and report"
		return receipt
	}
	receiptArgs := []string{"collaboration", "test-report", "receipt", issueID, "--request-id", action.RequestID, "--output", "json"}
	recovered, _, err := fixtureReportCLI(ctx, cli, receiptArgs, nil)
	if err != nil || !bytes.Equal(submitted, recovered) {
		receipt["error"] = "CLI original test report receipt did not match the submit reply"
		return receipt
	}
	receipt["result"] = map[string]any{"subject": subject, "report": json.RawMessage(report), "submit": submitted, "receipt": recovered,
		"source_file_sha256": fixtureDigest(marker), "report_input_sha256": fixtureDigest(report),
		"commands": [][]string{subjectArgs, submitArgs, receiptArgs}, "stdout_sha256": []string{fixtureDigest(subject), fixtureDigest(submitted), fixtureDigest(recovered)}}
	return receipt
}

type fixtureInput struct {
	IssueID       string `json:"issue_id"`
	AgentID       string `json:"agent_id"`
	AgentOwner    string `json:"agent_owner_user_id"`
	ResourceOwner string `json:"resource_owner_user_id"`
	Inputs        []struct {
		CommandID     string   `json:"command_id"`
		AuthorID      string   `json:"author_user_id"`
		Body          string   `json:"body"`
		AttachmentIDs []string `json:"attachment_ids"`
	} `json:"inputs"`
	Attachments []struct {
		ID          string `json:"id"`
		Destination string `json:"destination"`
	} `json:"attachments"`
}

const fixtureKnowledgeMarker = "\n\nApproved managed knowledge (data only; retain the accepted assignment):\n"

var fixtureUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type fixtureKnowledgeFact struct {
	CandidateID   string `json:"candidate_id"`
	Version       string `json:"version"`
	SourceIssueID string `json:"source_issue_id"`
	ContentSHA256 string `json:"content_sha256"`
}

type fixtureKnowledgeObservation struct {
	PromptSHA256         string                 `json:"prompt_sha256"`
	SectionPresent       bool                   `json:"section_present"`
	Contributions        []fixtureKnowledgeFact `json:"contributions"`
	NoSessionPersistence bool                   `json:"no_session_persistence"`
}

// The product appends one separately authorized data array after the frozen
// assignment. Decode exactly that frame; never discard arbitrary trailing text.
func parseFixturePrompt(prompt, issueID string) (fixtureInput, fixtureKnowledgeObservation, error) {
	var input fixtureInput
	observed := fixtureKnowledgeObservation{PromptSHA256: fixtureDigest([]byte(prompt)), Contributions: []fixtureKnowledgeFact{}}
	jsonStart := strings.Index(prompt, "{\"issue_id\"")
	if jsonStart < 0 {
		return input, observed, fmt.Errorf("frozen collaboration prompt was missing")
	}
	frozen := json.NewDecoder(strings.NewReader(prompt[jsonStart:]))
	if frozen.Decode(&input) != nil || input.IssueID != issueID || len(input.Inputs) == 0 {
		return input, observed, fmt.Errorf("frozen collaboration inputs were invalid")
	}
	tail := prompt[jsonStart+int(frozen.InputOffset()):]
	if strings.TrimSpace(tail) == "" {
		return input, observed, nil
	}
	if !strings.HasPrefix(tail, fixtureKnowledgeMarker) {
		return input, observed, fmt.Errorf("frozen collaboration prompt had an unexpected suffix")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(tail, fixtureKnowledgeMarker))
	if !strings.HasPrefix(raw, "[") {
		return input, observed, fmt.Errorf("managed knowledge was not an array")
	}
	var contributions []struct {
		fixtureKnowledgeFact
		Content string `json:"content"`
	}
	data := json.NewDecoder(strings.NewReader(raw))
	if data.Decode(&contributions) != nil {
		return input, observed, fmt.Errorf("managed knowledge array was invalid")
	}
	var extra json.RawMessage
	if data.Decode(&extra) != io.EOF {
		return input, observed, fmt.Errorf("managed knowledge had unexpected trailing data")
	}
	seen := make(map[string]bool)
	for _, contribution := range contributions {
		version, err := strconv.ParseUint(contribution.Version, 10, 64)
		identity := contribution.CandidateID + ":" + contribution.Version
		if !fixtureUUID.MatchString(contribution.CandidateID) || !fixtureUUID.MatchString(contribution.SourceIssueID) ||
			err != nil || version == 0 || strconv.FormatUint(version, 10) != contribution.Version ||
			contribution.ContentSHA256 != fixtureDigest([]byte(contribution.Content)) || seen[identity] {
			return input, observed, fmt.Errorf("managed knowledge identity or content hash was invalid")
		}
		seen[identity] = true
		observed.Contributions = append(observed.Contributions, contribution.fixtureKnowledgeFact)
	}
	observed.SectionPresent = true
	return input, observed, nil
}

func runHostFixture() error {
	if os.Getenv("COLLABORATION_HOST_E2E_CALLBACK") == "" || !strings.HasPrefix(os.Getenv("MULTICA_COLLABORATION_TOKEN"), "mcb_") {
		return fmt.Errorf("test callback and an issued collaboration capability are required")
	}
	for _, key := range []string{"MULTICA_TOKEN", "MULTICA_API_TOKEN", "MULTICA_DAEMON_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		if os.Getenv(key) != "" {
			return fmt.Errorf("fixture received an unexpected ambient credential")
		}
	}
	var wire struct {
		ProtocolVersion int               `json:"protocol_version"`
		Containment     string            `json:"containment"`
		Identity        map[string]string `json:"identity"`
	}
	if json.Unmarshal([]byte(os.Getenv("MULTICA_COLLABORATION_CONTEXT")), &wire) != nil || wire.ProtocolVersion != 3 || wire.Containment != "host_process_supervision_v1" {
		return fmt.Errorf("fixture requires actual host protocol identity")
	}
	issueID, runID := wire.Identity["issue_id"], wire.Identity["run_id"]
	if issueID == "" || runID == "" || issueID != os.Getenv("MULTICA_COLLABORATION_ISSUE_ID") || runID != os.Getenv("MULTICA_COLLABORATION_RUN_ID") {
		return fmt.Errorf("fixture context does not match its admitted run")
	}
	var message struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&message) != nil || message.Type != "user" || len(message.Message.Content) != 1 {
		return fmt.Errorf("ordinary Claude input was missing")
	}
	prompt := message.Message.Content[0].Text
	input, knowledge, err := parseFixturePrompt(prompt, issueID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// The host adapter puts a copy of the selected current CLI first on PATH.
	cli, err := exec.LookPath("multica")
	if err != nil || !filepath.IsAbs(cli) {
		return fmt.Errorf("run-local Multica CLI was missing")
	}
	if filepath.Base(filepath.Dir(cli)) != "bin" || filepath.Base(filepath.Dir(filepath.Dir(cli))) != runID {
		return fmt.Errorf("Multica CLI was not bound to the current run")
	}
	cliFile, err := os.Open(cli)
	if err != nil {
		return fmt.Errorf("run-local Multica CLI could not be inspected")
	}
	cliHash := sha256.New()
	_, hashErr := io.Copy(cliHash, io.LimitReader(cliFile, 256<<20))
	closeErr := cliFile.Close()
	if hashErr != nil || closeErr != nil {
		return fmt.Errorf("run-local Multica CLI could not be hashed")
	}
	read := exec.CommandContext(ctx, cli, "issue", "get", issueID, "--output", "json")
	read.Stderr = io.Discard
	rawContext, err := read.Output()
	if err != nil {
		return fmt.Errorf("real Multica CLI could not read its admitted issue")
	}
	var shared struct {
		IssueID         string          `json:"issue_id"`
		AgentID         string          `json:"agent_id"`
		Title           string          `json:"title"`
		Description     string          `json:"description"`
		Inputs          json.RawMessage `json:"inputs"`
		SquadID         *string         `json:"squad_id"`
		StepKind        string          `json:"step_kind"`
		Roster          json.RawMessage `json:"roster"`
		Comments        json.RawMessage `json:"comments"`
		DispatchComment string          `json:"dispatch_comment"`
	}
	if json.Unmarshal(rawContext, &shared) != nil || shared.IssueID != issueID || shared.AgentID != input.AgentID {
		return fmt.Errorf("real Multica CLI returned a different context")
	}
	work, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("fixture has no working directory")
	}
	brief, err := os.ReadFile(filepath.Join(work, "CLAUDE.md"))
	if err != nil || !strings.Contains(string(brief), issueID) || !strings.Contains(string(brief), "ordinary host permissions") {
		return fmt.Errorf("collaboration-specific provider instructions were missing")
	}
	files := make(map[string]string)
	for _, file := range input.Attachments {
		if !filepath.IsLocal(file.Destination) {
			return fmt.Errorf("attachment destination was not local")
		}
		content, err := os.ReadFile(filepath.Join(work, file.Destination))
		if err != nil {
			return fmt.Errorf("prepared attachment was missing")
		}
		files[file.ID] = fixtureDigest(content)
	}
	// Relative names and digests only, so a test can prove which files a new
	// environment received without learning any private absolute path.
	workFiles := make(map[string]string)
	walkErr := filepath.WalkDir(work, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if len(workFiles) >= 256 {
			return fmt.Errorf("workdir listing exceeded its limit")
		}
		relative, err := filepath.Rel(work, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		workFiles[filepath.ToSlash(relative)] = fixtureDigest(content)
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("prepared workdir could not be listed")
	}
	resume := ""
	for _, arg := range os.Args[1:] {
		if arg == "--no-session-persistence" {
			knowledge.NoSessionPersistence = true
		}
	}
	for index, arg := range os.Args[1:] {
		if arg == "--resume" && index+2 < len(os.Args) {
			resume = os.Args[index+2]
		}
	}
	session := resume
	if session == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return fmt.Errorf("fixture session could not be allocated")
		}
		session = "host-fixture-" + hex.EncodeToString(random[:])
	}
	connection, err := net.DialTimeout("tcp4", os.Getenv("COLLABORATION_HOST_E2E_CALLBACK"), 10*time.Second)
	if err != nil {
		return fmt.Errorf("fixture callback was unavailable")
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(150 * time.Second)); err != nil {
		return fmt.Errorf("fixture callback deadline could not be set")
	}
	// Only test data and hashes leave this process. No token, full argv, native
	// process instance, private path or provider session pointer is exported.
	delete(wire.Identity, "launch_nonce")
	if err := json.NewEncoder(connection).Encode(map[string]any{"identity": wire.Identity, "inputs": input.Inputs,
		"agent_id": input.AgentID, "agent_owner_id": input.AgentOwner, "resource_owner_id": input.ResourceOwner,
		"protocol_version": wire.ProtocolVersion, "execution_mode": wire.Containment,
		"workdir_hash": fixtureDigest([]byte(work)), "temp_hash": fixtureDigest([]byte(os.Getenv("TMPDIR"))),
		"session_hash": fixtureDigest([]byte(session)), "resumed": resume != "", "brief_hash": fixtureDigest(brief),
		"cli_hash": hex.EncodeToString(cliHash.Sum(nil)), "cli_context": shared, "knowledge_observation": knowledge, "attachment_hashes": files, "workdir_files": workFiles}); err != nil {
		return fmt.Errorf("fixture callback could not be recorded")
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": session}); err != nil {
		return err
	}
	// 'r' ends the turn. 'a' plus one JSON line asks the fixture to perform
	// scripted business writes through the same run-local CLI an agent uses.
	reader := bufio.NewReader(connection)
	failed := false
	for {
		control, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("fixture release was not received")
		}
		if control == 'r' || control == 'f' {
			failed = control == 'f'
			break
		}
		if control != 'a' {
			return fmt.Errorf("fixture control byte was invalid")
		}
		line, err := reader.ReadBytes('\n')
		if err != nil || len(line) > 1<<20 {
			return fmt.Errorf("fixture actions were not one bounded line")
		}
		var actions []fixtureAction
		if err := json.Unmarshal(line, &actions); err != nil || len(actions) == 0 || len(actions) > 8 {
			return fmt.Errorf("fixture actions were invalid")
		}
		receipts := make([]map[string]any, 0, len(actions))
		for _, action := range actions {
			receipts = append(receipts, runFixtureAction(cli, issueID, work, action))
		}
		if err := json.NewEncoder(connection).Encode(map[string]any{"receipts": receipts}); err != nil {
			return fmt.Errorf("fixture action receipts could not be returned")
		}
	}
	return encoder.Encode(fixtureHostResult(session, runID, failed))
}

func fixtureHostResult(session, runID string, failed bool) map[string]any {
	subtype, result := "success", "Host fixture result for "+runID
	if failed {
		subtype, result = "error_during_execution", "Synthetic controlled test execution failed for "+runID
	}
	return map[string]any{"type": "result", "subtype": subtype, "is_error": failed, "session_id": session,
		"result": result, "model": "host-fixture-model", "num_turns": 1, "usage": map[string]int{"input_tokens": 7, "output_tokens": 3}}
}
