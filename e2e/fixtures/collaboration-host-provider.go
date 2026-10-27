// Build this stdlib-only fixture explicitly for the native host under test.
// It speaks ordinary Claude stream-json, reads the issue through the actual
// Multica CLI, and never supplies a launch permit or termination receipt.
package main

import (
	"bufio"
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
}

func runFixtureAction(cli, issueID, work string, action fixtureAction) map[string]any {
	receipt := map[string]any{"kind": action.Kind, "request_id": action.RequestID}
	var args []string
	switch action.Kind {
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
	jsonStart := strings.Index(prompt, "{\"issue_id\"")
	if jsonStart < 0 {
		return fmt.Errorf("frozen collaboration prompt was missing")
	}
	var input struct {
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
	if json.Unmarshal([]byte(prompt[jsonStart:]), &input) != nil || input.IssueID != issueID || len(input.Inputs) == 0 {
		return fmt.Errorf("frozen collaboration inputs were invalid")
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
		"cli_hash": hex.EncodeToString(cliHash.Sum(nil)), "cli_context": shared, "attachment_hashes": files, "workdir_files": workFiles}); err != nil {
		return fmt.Errorf("fixture callback could not be recorded")
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": session}); err != nil {
		return err
	}
	// 'r' ends the turn. 'a' plus one JSON line asks the fixture to perform
	// scripted business writes through the same run-local CLI an agent uses.
	reader := bufio.NewReader(connection)
	for {
		control, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("fixture release was not received")
		}
		if control == 'r' {
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
	return encoder.Encode(map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": session,
		"result": "Host fixture result for " + runID, "model": "host-fixture-model", "num_turns": 1,
		"usage": map[string]int{"input_tokens": 7, "output_tokens": 3}})
}
