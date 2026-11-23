package main

import (
	"errors"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

// Handoff reads exist only inside an admitted collaboration run. They use the
// run's mcb_ capability, refuse any other issue and have no ordinary-profile form.

var collaborationHandoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Read the issue's live handoff and its full source text",
}

var collaborationHandoffShowCmd = newCollaborationHandoffShowCmd()
var collaborationHandoffSourceCmd = newCollaborationHandoffSourceCmd()

func init() {
	collaborationHandoffCmd.AddCommand(collaborationHandoffShowCmd)
	collaborationHandoffCmd.AddCommand(collaborationHandoffSourceCmd)
	collaborationCmd.AddCommand(collaborationHandoffCmd)
}

func newCollaborationHandoffShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <issue-id>",
		Short: "Print the run's live handoff envelope as JSON",
		Long: `Print this run's handoff envelope as JSON, read now (mode "live").

This is not the startup snapshot in your input's handoff field: that snapshot
is frozen when the run starts, while this read reflects current issue state.
Results, inputs and wakeup causes return at most 32 items per page. When a list
has truncated=true, run again with --cursor <next_cursor>; when an input has
readers_truncated=true, use --cursor <readers_cursor>. Cursors from the startup
snapshot also work here. A cursor pages only the list it came from; the other
lists in that response restart at their first page, so read only that list and
follow its own cursor until it has none.
A source with truncated=true carries only a prefix; read its full text with
'multica collaboration handoff source'.`,
		Args: exactArgs(1),
		RunE: runCollaborationHandoffShow,
	}
	cmd.Flags().String("cursor", "", "next_cursor or readers_cursor from a previous handoff page")
	cmd.Flags().String("output", "json", "Output format: json")
	return cmd
}

func newCollaborationHandoffSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source <issue-id> <source-id>",
		Short: "Print one chunk of a handoff source's full text as JSON",
		Long: `Print one chunk (up to 32 KiB) of a handoff source's full text as JSON.

Copy the reference from the source entry in a handoff envelope: id as
<source-id>, kind as --kind, run_id as --source-run-id, version as --version
and sha256 as --sha256. A handoff-kind source has no run_id or version; omit
both flags. Start at offset 0, then run again with --offset <next_offset> until
next_offset is null. stale_handoff_source means the source no longer matches
its reference: do not join its chunks with other text.`,
		Args: exactArgs(2),
		RunE: runCollaborationHandoffSource,
	}
	cmd.Flags().String("kind", "", "Source kind: handoff, delegation, provider_result or squad_activity")
	cmd.Flags().String("source-run-id", "", "The source's run_id (omit for a handoff-kind source)")
	cmd.Flags().String("version", "", "The source's version (omit for a handoff-kind source)")
	cmd.Flags().String("sha256", "", "The source's sha256")
	cmd.Flags().Int("offset", 0, "Byte offset: 0 or next_offset from the previous chunk")
	cmd.Flags().String("output", "json", "Output format: json")
	return cmd
}

// collaborationHandoffRun returns the admitted run after the shared command and
// flag checks. Outside a run there is nothing to address.
func collaborationHandoffRun(cmd *cobra.Command, args []string, operation string) (*cli.CollaborationContext, error) {
	run, err := cli.ReadCollaborationContext()
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, errors.New("collaboration handoff is only available inside a collaboration run")
	}
	if err := validateCollaborationCLI(run, cmd, operation, args); err != nil {
		return nil, err
	}
	if output, _ := cmd.Flags().GetString("output"); output != "json" {
		return nil, errors.New("collaboration handoff prints only --output json")
	}
	return run, nil
}

func runCollaborationHandoffShow(cmd *cobra.Command, args []string) error {
	run, err := collaborationHandoffRun(cmd, args, "collaboration handoff show")
	if err != nil {
		return err
	}
	cursor, _ := cmd.Flags().GetString("cursor")
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	envelope, err := run.ReadHandoff(ctx, run.Client(), cursor)
	if err != nil {
		return err
	}
	return cli.PrintJSON(cmd.OutOrStdout(), envelope)
}

func runCollaborationHandoffSource(cmd *cobra.Command, args []string) error {
	run, err := collaborationHandoffRun(cmd, args, "collaboration handoff source")
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return errors.New("handoff source requires the issue UUID and source UUID")
	}
	ref := cli.CollaborationHandoffReference{SourceID: args[1]}
	ref.Kind, _ = cmd.Flags().GetString("kind")
	ref.SourceRunID, _ = cmd.Flags().GetString("source-run-id")
	ref.Version, _ = cmd.Flags().GetString("version")
	ref.SHA256, _ = cmd.Flags().GetString("sha256")
	ref.Offset, _ = cmd.Flags().GetInt("offset")
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	page, err := run.ReadHandoffSource(ctx, run.Client(), ref)
	if err != nil {
		return err
	}
	return cli.PrintJSON(cmd.OutOrStdout(), page)
}
