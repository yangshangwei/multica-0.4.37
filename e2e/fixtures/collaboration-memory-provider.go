// Build this stdlib-only fixture explicitly for the native host under test.
// It speaks ordinary Claude stream-json and reads/writes shared context through
// the actual run-local Multica CLI. It never supplies a permit or exit receipt.
package main

import (
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
	if err := runHostFixture(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func fixtureDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
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
		Memory        *struct {
			Version int64 `json:"version"`
			Content string `json:"content"`
		} `json:"memory"`
		Inputs []struct {
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
	if decodeErr := json.NewDecoder(strings.NewReader(prompt[jsonStart:])).Decode(&input); decodeErr != nil {
		return fmt.Errorf("frozen collaboration inputs were invalid: %w", decodeErr)
	}
	if input.IssueID != issueID || len(input.Inputs) == 0 {
		return fmt.Errorf("frozen collaboration inputs were invalid: issue=%q expected=%q inputs=%d", input.IssueID, issueID, len(input.Inputs))
	}
	if input.Memory == nil {
		return fmt.Errorf("shared context was not injected into the host prompt")
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
		IssueID     string          `json:"issue_id"`
		AgentID     string          `json:"agent_id"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Inputs      json.RawMessage `json:"inputs"`
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
	// Test content is intentionally synthetic. It belongs only in this launch's
	// input and must never be copied into an instruction file reused by a run.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		text, readErr := os.ReadFile(filepath.Join(work, name))
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("provider instruction file could not be checked")
		}
		if input.Memory.Content != "" && strings.Contains(string(text), input.Memory.Content) {
			return fmt.Errorf("shared-context text leaked into a reusable instruction file")
		}
	}
	show := exec.CommandContext(ctx, cli, "collaboration", "memory", "show", issueID, "--output", "json")
	show.Stderr = io.Discard
	shown, err := show.Output()
	var latest struct {
		IssueID string `json:"issue_id"`
		RunID   string `json:"run_id"`
		Version string `json:"version"`
		Content string `json:"content"`
	}
	if err != nil || json.Unmarshal(shown, &latest) != nil || latest.IssueID != issueID || latest.RunID != runID ||
		latest.Version != strconv.FormatInt(input.Memory.Version, 10) || latest.Content != input.Memory.Content {
		return fmt.Errorf("run-local memory show did not match the injected baseline")
	}
	content := input.Memory.Content + "\n\nDecision by agent " + input.AgentID + ": preserve prior findings when handing off."
	update := exec.CommandContext(ctx, cli, "collaboration", "memory", "update", issueID,
		"--base-version", strconv.FormatInt(input.Memory.Version, 10), "--request-id", runID, "--content-stdin", "--output", "json")
	update.Stdin, update.Stderr = strings.NewReader(content), io.Discard
	updated, err := update.Output()
	var receipt struct {
		Code    string `json:"code"`
		IssueID string `json:"issue_id"`
		RunID   string `json:"run_id"`
		Version string `json:"version"`
	}
	if err != nil || json.Unmarshal(updated, &receipt) != nil || receipt.Code != "memory_updated" || receipt.IssueID != issueID || receipt.RunID != runID {
		return fmt.Errorf("run-local memory update did not commit a signed version")
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
		"cli_hash": hex.EncodeToString(cliHash.Sum(nil)), "cli_context": shared, "attachment_hashes": files,
		"memory_input": input.Memory, "memory_shown": latest, "memory_written": receipt, "instruction_files_clean": true}); err != nil {
		return fmt.Errorf("fixture callback could not be recorded")
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": session}); err != nil {
		return err
	}
	var release [1]byte
	if _, err := io.ReadFull(connection, release[:]); err != nil || release[0] != 'r' {
		return fmt.Errorf("fixture release was not received")
	}
	return encoder.Encode(map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": session,
		"result": "Host fixture result for " + runID, "model": "host-fixture-model", "num_turns": 1,
		"usage": map[string]int{"input_tokens": 7, "output_tokens": 3}})
}
