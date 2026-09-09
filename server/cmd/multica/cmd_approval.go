package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

// `multica approval` — the CLI half of the human approval boundary.
//
// This exists because the boundary is worthless if the agent it governs cannot
// reach it. An Operator agent is told, in instructions it cannot edit, to file a
// request and wait; without a command to file one it would either invent a way
// or proceed anyway. Both are worse than the wait.
//
// Only `request`, `list`, `get` and `cancel` live here. Deciding is deliberately
// absent: the server rejects a decision from a machine credential, so a
// `multica approval approve` would be a command that fails by design for every
// caller that has this CLI in its PATH during a task.

var approvalCmd = &cobra.Command{
	Use:   "approval",
	Short: "Request and track human approval for high-risk actions",
	Long: "High-risk actions — production releases, migrations against live data,\n" +
		"credential access, external announcements, irreversible deletions — need a\n" +
		"person's approval before an agent carries them out.\n\n" +
		"File one request per action, then stop and report. A person approves or\n" +
		"rejects it in Multica; you cannot approve your own request, and an approval\n" +
		"covers only the action it describes.",
}

var approvalRequestCmd = &cobra.Command{
	Use:   "request",
	Short: "File one approval request for one action",
	Args:  cobra.NoArgs,
	RunE:  runApprovalRequest,
}

func runApprovalRequest(cmd *cobra.Command, _ []string) error {
	riskClass, _ := cmd.Flags().GetString("risk-class")
	summary, _ := cmd.Flags().GetString("summary")
	plan, _ := cmd.Flags().GetString("plan")
	planFile, _ := cmd.Flags().GetString("plan-file")
	issueID, _ := cmd.Flags().GetString("issue")
	agentID, _ := cmd.Flags().GetString("agent-id")

	riskClass = strings.TrimSpace(riskClass)
	if !service.IsKnownApprovalRiskClass(riskClass) {
		// Listed rather than described: the caller is usually a model choosing a
		// value, and an exhaustive list is the shortest correct answer.
		return fmt.Errorf("--risk-class must be one of: %s", strings.Join(service.ApprovalRiskClasses, ", "))
	}
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("--summary is required: one line naming the single action you need approved")
	}
	if planFile != "" {
		// A plan is commands, expected output and a rollback — routinely longer
		// than a shell argument should carry.
		contents, err := os.ReadFile(planFile)
		if err != nil {
			return fmt.Errorf("read --plan-file: %w", err)
		}
		// A plan is the artifact a human reads before authorising an action they
		// cannot easily undo. Mojibake here is worse than a failed command:
		// it makes the thing under review unreadable at the moment of decision.
		plan, err = util.DecodeTextFileBytes(contents, "file content for --plan-file")
		if err != nil {
			return err
		}
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	body := map[string]any{
		"risk_class": riskClass,
		"summary":    strings.TrimSpace(summary),
		"plan":       plan,
	}
	if issueID != "" {
		body["issue_id"] = issueID
	}
	// Ignored for an agent caller — the server takes the agent's identity from
	// its task token — and only meaningful when a person files on an agent's
	// behalf.
	if agentID != "" {
		body["agent_id"] = agentID
	}

	var approval map[string]any
	if err := client.PostJSON(ctx, "/api/agent-approvals", body, &approval); err != nil {
		return fmt.Errorf("file approval request: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, approval)
	}
	fmt.Printf("Approval requested: %s (%s)\n", strVal(approval, "id"), strVal(approval, "status"))
	fmt.Println("Stop here and report the plan. Do not begin until this reads 'approved'.")
	return nil
}

var approvalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List approval requests (an agent sees only its own)",
	Args:  cobra.NoArgs,
	RunE:  runApprovalList,
}

func runApprovalList(cmd *cobra.Command, _ []string) error {
	status, _ := cmd.Flags().GetString("status")

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	path := "/api/agent-approvals"
	if strings.TrimSpace(status) != "" {
		path += "?status=" + strings.TrimSpace(status)
	}
	var resp struct {
		Approvals []map[string]any `json:"approvals"`
	}
	if err := client.GetJSON(ctx, path, &resp); err != nil {
		return fmt.Errorf("list approval requests: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, resp.Approvals)
	}
	if len(resp.Approvals) == 0 {
		fmt.Fprintln(os.Stderr, "No approval requests found.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tRISK CLASS\tSUMMARY")
	for _, approval := range resp.Approvals {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			strVal(approval, "id"), strVal(approval, "status"),
			strVal(approval, "risk_class"), strVal(approval, "summary"))
	}
	return w.Flush()
}

var approvalGetCmd = &cobra.Command{
	Use:   "get <approval-id>",
	Short: "Read one approval request, including whether a person decided",
	Args:  cobra.ExactArgs(1),
	RunE:  runApprovalGet,
}

func runApprovalGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var approval map[string]any
	if err := client.GetJSON(ctx, "/api/agent-approvals/"+args[0], &approval); err != nil {
		return fmt.Errorf("get approval request: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, approval)
	}
	fmt.Printf("%s\n", strVal(approval, "summary"))
	fmt.Printf("status: %s\nrisk class: %s\n", strVal(approval, "status"), strVal(approval, "risk_class"))
	if note := strVal(approval, "decision_note"); note != "" && note != "-" {
		fmt.Printf("reviewer note: %s\n", note)
	}
	return nil
}

var approvalExecutedCmd = &cobra.Command{
	Use:   "executed <approval-id>",
	Short: "Record what happened when you carried out an approved action",
	Long: "Records the result of an approved action, including a partial failure.\n\n" +
		"Rejected unless the request is approved AND your autonomy level is operator,\n" +
		"so this cannot be used to walk an unapproved action forward.",
	Args: cobra.ExactArgs(1),
	RunE: runApprovalExecuted,
}

func runApprovalExecuted(cmd *cobra.Command, args []string) error {
	note, _ := cmd.Flags().GetString("note")

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var approval map[string]any
	if err := client.PostJSON(ctx, "/api/agent-approvals/"+args[0]+"/execution",
		map[string]any{"note": note}, &approval); err != nil {
		return fmt.Errorf("record execution: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, approval)
	}
	fmt.Printf("Recorded: %s (%s)\n", strVal(approval, "id"), strVal(approval, "status"))
	return nil
}

var approvalCancelCmd = &cobra.Command{
	Use:   "cancel <approval-id>",
	Short: "Withdraw a request whose plan no longer applies",
	Args:  cobra.ExactArgs(1),
	RunE:  runApprovalCancel,
}

func runApprovalCancel(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var approval map[string]any
	if err := client.PostJSON(ctx, "/api/agent-approvals/"+args[0]+"/cancel", map[string]any{}, &approval); err != nil {
		return fmt.Errorf("cancel approval request: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, approval)
	}
	fmt.Printf("Cancelled: %s\n", strVal(approval, "id"))
	return nil
}

func init() {
	approvalRequestCmd.Flags().String("risk-class", "", "Action class: "+strings.Join(service.ApprovalRiskClasses, ", "))
	approvalRequestCmd.Flags().String("summary", "", "One line naming the single action needing approval")
	approvalRequestCmd.Flags().String("plan", "", "The plan a reviewer reads: commands, expected effect, rollback")
	approvalRequestCmd.Flags().String("plan-file", "", "Read the plan from a file instead of --plan")
	approvalRequestCmd.Flags().String("issue", "", "Issue this action belongs to")
	approvalRequestCmd.Flags().String("agent-id", "", "Agent to file for (people only; an agent's identity comes from its task token)")
	approvalRequestCmd.Flags().String("output", "", "Output format: json")

	approvalListCmd.Flags().String("status", "", "Filter: pending, approved, rejected, executed, cancelled")
	approvalListCmd.Flags().String("output", "", "Output format: json")

	approvalGetCmd.Flags().String("output", "", "Output format: json")

	approvalExecutedCmd.Flags().String("note", "", "What actually happened, including a partial failure")
	approvalExecutedCmd.Flags().String("output", "", "Output format: json")

	approvalCancelCmd.Flags().String("output", "", "Output format: json")

	approvalCmd.AddCommand(approvalRequestCmd)
	approvalCmd.AddCommand(approvalListCmd)
	approvalCmd.AddCommand(approvalGetCmd)
	approvalCmd.AddCommand(approvalExecutedCmd)
	approvalCmd.AddCommand(approvalCancelCmd)
}
