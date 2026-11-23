package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

// Shared context ("collaboration memory") exists only inside an admitted
// collaboration run. Both commands use the run's mcb_ capability and refuse
// any other issue; there is no ordinary-profile form.

var collaborationCmd = &cobra.Command{
	Use:   "collaboration",
	Short: "Commands available inside a collaboration run",
}

var collaborationMemoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Read or rewrite the issue's shared collaboration context",
}

var collaborationMemoryShowCmd = newCollaborationMemoryShowCmd()
var collaborationMemoryUpdateCmd = newCollaborationMemoryUpdateCmd()

func init() {
	collaborationMemoryCmd.AddCommand(collaborationMemoryShowCmd)
	collaborationMemoryCmd.AddCommand(collaborationMemoryUpdateCmd)
	collaborationCmd.AddCommand(collaborationMemoryCmd)
}

func newCollaborationMemoryShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <issue-id>",
		Short: "Print the latest shared-context version",
		Long: `Print the latest version of the admitted issue's shared context.

Run it when another collaborator may have written since this run started; the
version your input's memory field carries is the one this run began from.`,
		Args: exactArgs(1),
		RunE: runCollaborationMemoryShow,
	}
	cmd.Flags().String("output", "table", "Output format: table or json")
	return cmd
}

func newCollaborationMemoryUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "Replace the shared context with a rewritten document",
		Long: `Replace the whole shared-context document with the current state.

Rewrite the document rather than appending to it. --base-version is the version
you merged from; if someone wrote a newer one first, the update is refused with
stale_memory_version: run 'memory show', merge, and update from that version.
Reuse the same --request-id when retrying an uncertain result.`,
		Args: exactArgs(1),
		RunE: runCollaborationMemoryUpdate,
	}
	cmd.Flags().String("base-version", "", "Version you merged from (0 for the empty document)")
	cmd.Flags().String("request-id", "", "Write UUID; reuse with the unchanged command after an uncertain result")
	cmd.Flags().Bool("content-stdin", false, "Read the document from stdin")
	cmd.Flags().String("content-file", "", "Read the document from a UTF-8 file inside the working directory")
	cmd.Flags().String("output", "table", "Output format: table or json")
	return cmd
}

// collaborationMemoryRun returns the admitted run after the shared command and
// flag checks. Outside a run there is nothing to address.
func collaborationMemoryRun(cmd *cobra.Command, args []string, operation string) (*cli.CollaborationContext, error) {
	run, err := cli.ReadCollaborationContext()
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, errors.New("collaboration memory is only available inside a collaboration run")
	}
	if err := validateCollaborationCLI(run, cmd, operation, args); err != nil {
		return nil, err
	}
	return run, nil
}

func runCollaborationMemoryShow(cmd *cobra.Command, args []string) error {
	run, err := collaborationMemoryRun(cmd, args, "collaboration memory show")
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	memory, err := run.ReadMemory(ctx, run.Client())
	if err != nil {
		return err
	}
	if output, _ := cmd.Flags().GetString("output"); output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), memory)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "version %s · %d/%d bytes\n", memory.Version, memory.Bytes, memory.MaxBytes)
	if memory.Content == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "(empty)")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), memory.Content)
	return nil
}

func runCollaborationMemoryUpdate(cmd *cobra.Command, args []string) error {
	run, err := collaborationMemoryRun(cmd, args, "collaboration memory update")
	if err != nil {
		return err
	}
	base, _ := cmd.Flags().GetString("base-version")
	if n, err := strconv.ParseInt(base, 10, 64); err != nil || n < 0 || strconv.FormatInt(n, 10) != base {
		return errors.New("--base-version must be the version you merged from (0 for the empty document)")
	}
	// resolveTextFlag's own messages name --content and --allow-external-file,
	// neither of which this command has.
	file, _ := cmd.Flags().GetString("content-file")
	if stdin, _ := cmd.Flags().GetBool("content-stdin"); stdin && file != "" {
		return errors.New("--content-stdin and --content-file are mutually exclusive")
	}
	if file != "" {
		if within, err := fileWithinWorkingDir(file); err != nil || !within {
			return fmt.Errorf("--content-file %q must be inside the working directory", file)
		}
	}
	content, present, err := resolveTextFlag(cmd, "content")
	if err != nil {
		return err
	}
	if !present {
		return errors.New("--content-stdin or --content-file is required")
	}
	if err := rejectLocalPathLinks(content, "collaboration memory", "Files are unavailable to other collaborators; point to commits, branches or issue attachments instead."); err != nil {
		return err
	}
	requestID, _ := cmd.Flags().GetString("request-id")
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	receipt, err := run.UpdateMemory(ctx, run.Client(), protocol.CollaborationRunMemoryUpdate{RequestID: requestID, BaseVersion: base, Content: content})
	if err != nil {
		return err
	}
	if output, _ := cmd.Flags().GetString("output"); output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), receipt)
	}
	bytes := 0
	if receipt.Bytes != nil {
		bytes = *receipt.Bytes
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Collaboration memory saved as version %s (%d/%d bytes).\n", *receipt.Version, bytes, receipt.MaxBytes)
	return nil
}
