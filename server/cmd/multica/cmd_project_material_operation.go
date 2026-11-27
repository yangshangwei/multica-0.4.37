package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func materialOperationCommands() []*cobra.Command {
	var commands []*cobra.Command
	for _, action := range []string{"operation", "observe", "permit"} {
		cmd := &cobra.Command{Use: action + " <operation-uuid>", Short: map[string]string{"operation": "Read the current task's immutable material operation", "observe": "Ask the local daemon to independently read remote material facts", "permit": "Request current authorization for the exact material stage"}[action], Args: exactArgs(1)}
		if action == "observe" {
			cmd.Flags().String("review-url", "", "Optional review locator hint (never an authority or host override)")
		}
		if action == "permit" {
			cmd.Flags().String("request-id", "", "Original canonical request UUID (required)")
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error { return runMaterialOperation(cmd, action, args[0]) }
		commands = append(commands, cmd)
	}
	return commands
}
func runMaterialOperation(cmd *cobra.Command, action, id string) error {
	if !materialCLIUUID(id) {
		return errors.New("operation ID must be a canonical nonzero UUID")
	}
	port, err := strconv.Atoi(os.Getenv("MULTICA_DAEMON_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("active local daemon port required")
	}
	token := os.Getenv("MULTICA_TOKEN")
	if !strings.HasPrefix(token, cli.TaskTokenPrefix) {
		return errors.New("active task credential required")
	}
	input := map[string]string{"operation_id": id, "action": action}
	if action == "observe" {
		input["review_url"], _ = cmd.Flags().GetString("review-url")
		if len(input["review_url"]) > 2048 {
			return errors.New("review URL too long")
		}
	}
	if action == "permit" {
		input["request_id"], _ = cmd.Flags().GetString("request-id")
		if !materialCLIUUID(input["request_id"]) {
			return errors.New("explicit canonical request ID required")
		}
	}
	raw, _ := json.Marshal(input)
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/material", port), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("local material request unconfirmed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("local material operation refused or unconfirmed; retain the original request ID before retrying")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, protocol.MaxMaterialObservationBytes+1))
	if err != nil || len(body) > protocol.MaxMaterialObservationBytes {
		return errors.New("invalid local material response")
	}
	switch action {
	case "operation":
		var out protocol.MaterialExecution
		if json.Unmarshal(body, &out) != nil || out.OperationID != id || out.ProtocolVersion != 1 || out.ExpectedDigest != out.Digest() {
			return errors.New("invalid operation descriptor")
		}
		return cli.PrintJSON(cmd.OutOrStdout(), out)
	case "observe":
		var out struct {
			Recorded bool `json:"recorded"`
		}
		if json.Unmarshal(body, &out) != nil || !out.Recorded {
			return errors.New("observation unconfirmed")
		}
		return cli.PrintJSON(cmd.OutOrStdout(), out)
	default:
		var out protocol.MaterialPermit
		if json.Unmarshal(body, &out) != nil || out.OperationID != id || out.Action == "" || out.ExpectedDigest == "" || !out.ExpiresAt.After(time.Now()) {
			return errors.New("invalid material permit")
		}
		return cli.PrintJSON(cmd.OutOrStdout(), out)
	}
}
