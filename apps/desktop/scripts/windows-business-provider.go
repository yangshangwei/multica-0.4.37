//go:build ignore

// Deterministic acceptance provider. Never contacts an external model.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	root := os.Getenv("MULTICA_BUSINESS_STATE_DIR")
	if root == "" {
		os.Exit(78)
	}
	if _, err := os.Stat(filepath.Join(root, "server-private.json")); err != nil {
		os.Exit(78)
	}
	args := strings.Join(os.Args[1:], " ")
	if strings.Contains(args, "--version") {
		fmt.Println("2.1.100 (Claude Code)")
		return
	}
	if !strings.Contains(args, "--output-format stream-json") {
		os.Exit(64)
	}
	task := os.Getenv("MULTICA_TASK_ID")
	if task == "" || strings.ContainsAny(task, `/\\`) {
		os.Exit(64)
	}
	emit := func(value any) { _ = json.NewEncoder(os.Stdout).Encode(value) }
	emit(map[string]any{"type": "system", "subtype": "init", "session_id": "fixture-" + task, "tools": []string{}, "mcp_servers": []string{}})
	_ = os.WriteFile(filepath.Join(root, "provider-"+task+".json"), []byte(fmt.Sprintf(`{"pid":%d,"task_id":%q}`, os.Getpid(), task)), 0600)
	starts, err := os.OpenFile(filepath.Join(root, "provider-starts.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(78)
	}
	_, _ = fmt.Fprintf(starts, "{\"pid\":%d,\"task_id\":%q}\n", os.Getpid(), task)
	_ = starts.Close()
	for {
		if _, err := os.Stat(filepath.Join(root, "release-"+task)); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	emit(map[string]any{"type": "assistant", "session_id": "fixture-" + task, "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "Windows acceptance fixture completed."}}}})
	emit(map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "Windows acceptance fixture completed.", "session_id": "fixture-" + task, "duration_ms": 1, "num_turns": 1, "total_cost_usd": 0, "usage": map[string]int{"input_tokens": 0, "output_tokens": 0}})
}
