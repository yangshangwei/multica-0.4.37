package main

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{Use: "test-report", Short: "Record test evidence for this run's frozen source", Long: "One immutable test report per run attempt. Reports are runtime-reported evidence; they do not approve QA or advance a stage. To preserve a failure and test again, start a new explicitly authorized run."}
	for _, action := range []string{"subject", "submit", "receipt"} {
		cmd.AddCommand(newCollaborationTestReportAction(action))
	}
	collaborationCmd.AddCommand(cmd)
}

func newCollaborationTestReportAction(action string) *cobra.Command {
	short := map[string]string{"subject": "Print the source subject frozen when this run was claimed", "submit": "Submit one immutable test report from stdin", "receipt": "Confirm the original report request without submitting again"}[action]
	cmd := &cobra.Command{Use: action + " <issue-id>", Short: short, Args: exactArgs(1)}
	cmd.Flags().String("output", "json", "Output format: json")
	if action != "subject" {
		cmd.Flags().String("request-id", "", "Original report request UUID; keep it and the unchanged report after an uncertain response")
	}
	if action == "submit" {
		cmd.Flags().Bool("report-stdin", false, "Read the bounded report JSON object from stdin")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		run, err := cli.ReadCollaborationContext()
		if err != nil {
			return err
		}
		if run == nil {
			return errors.New("test-report is only available inside a collaboration run")
		}
		if err = validateCollaborationCLI(run, cmd, "collaboration test-report "+action, args); err != nil {
			return err
		}
		output, _ := cmd.Flags().GetString("output")
		if output != "json" {
			return errors.New("test-report supports only --output json")
		}
		request, _ := cmd.Flags().GetString("request-id")
		var report protocol.CollaborationTestReport
		if action == "submit" {
			stdin, _ := cmd.Flags().GetBool("report-stdin")
			if !stdin {
				return errors.New("--report-stdin is required")
			}
			raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), protocol.CollaborationTestReportJSONLimit+1))
			if err != nil {
				return err
			}
			if len(raw) > protocol.CollaborationTestReportJSONLimit {
				return errors.New("test report exceeds the 2 MiB input limit")
			}
			if json.Unmarshal(raw, &report) != nil || protocol.ValidateCollaborationTestReport(report) != nil {
				return errors.New("invalid test report JSON; use the frozen subject and bounded cases/logs")
			}
		}
		ctx, cancel := cli.APIContext(cmd.Context())
		defer cancel()
		client := run.Client()
		var result any
		switch action {
		case "subject":
			result, err = run.ReadTestSubject(ctx, client)
		case "submit":
			result, err = run.SubmitTestReport(ctx, client, request, report)
		case "receipt":
			result, err = run.ReadTestReportReceipt(ctx, client, request)
		default:
			return errors.New("unknown test-report operation")
		}
		if err != nil {
			return err
		}
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	return cmd
}
