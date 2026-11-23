package main

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var issueCommentReceiptCmd = &cobra.Command{
	Use:   "receipt <issue-id> <key>",
	Short: "Read the original durable delegation receipt without dispatching work",
	Args:  cobra.ExactArgs(2),
	RunE:  runIssueCommentReceipt,
}

func validDelegationKey(key string) bool {
	id, err := uuid.Parse(key)
	return err == nil && id != uuid.Nil && id.String() == key
}

func runIssueCommentReceipt(cmd *cobra.Command, args []string) error {
	if handled, err := runCollaborationCLI(cmd, args, "issue comment receipt"); handled {
		return err
	}
	if !validDelegationKey(args[1]) {
		return fmt.Errorf("receipt key must be a canonical nonzero UUID")
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	issueRef, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	var receipt map[string]any
	if err := client.GetJSON(ctx, "/api/issues/"+url.PathEscape(issueRef.ID)+"/delegation-receipts/"+args[1], &receipt); err != nil {
		return fmt.Errorf("read delegation receipt: %w", err)
	}
	warnDurableDelegationReceipt(os.Stderr, receipt, args[1], "")
	return cli.PrintJSON(os.Stdout, receipt)
}

// A recorded receipt confirms retained admission facts, never completed work.
// Preserve the response and successful write status even on older servers that
// silently ignore the key, while making that missing guarantee visible.
func warnDurableDelegationReceipt(w io.Writer, raw any, key, commentID string) {
	receipt, _ := raw.(map[string]any)
	storedKey, _ := receipt["key"].(string)
	storedComment, _ := receipt["comment_id"].(string)
	state, _ := receipt["state"].(string)
	targets, hasTargets := receipt["targets"].([]any)
	if storedKey != key || !validDelegationKey(storedComment) || (commentID != "" && storedComment != commentID) || state != "recorded" || !hasTargets {
		fmt.Fprintln(w, "Warning: durable delegation is unconfirmed. Do not repost or choose a new key; use `multica issue comment receipt <issue-id> <key>` for read-only recovery. A missing receipt on an older server is not an idempotency guarantee.")
		return
	}
	warnCommentTriggerOutcomes(w, map[string]any{"trigger_outcomes": targets})
	for _, rawTarget := range targets {
		target, _ := rawTarget.(map[string]any)
		status, _ := target["status"].(string)
		if status != "queued" && status != "coalesced" && status != "deferred" {
			continue
		}
		taskID, _ := target["task_id"].(string)
		agentID, _ := target["executing_agent_id"].(string)
		if !validDelegationKey(taskID) || !validDelegationKey(agentID) {
			fmt.Fprintln(w, "Warning: durable delegation has an unconfirmed task identity. Do not repost; inspect the original receipt with an authorized follow-up.")
			return
		}
	}
}
