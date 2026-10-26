package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func collaborationCommandAllowed(operation string) bool {
	switch operation {
	case "issue get", "issue comment list", "issue comment add", "squad activity",
		"collaboration memory show", "collaboration memory update":
		return true
	default:
		return false
	}
}

func guardCollaborationCommand(cmd *cobra.Command, args []string) error {
	run, err := cli.ReadCollaborationContext()
	if err != nil || run == nil {
		return err
	}
	operation := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
	return validateCollaborationCLI(run, cmd, operation, args)
}

func validateCollaborationCLI(run *cli.CollaborationContext, cmd *cobra.Command, operation string, args []string) error {
	if !collaborationCommandAllowed(operation) {
		return errors.New("command is unavailable inside a collaboration run")
	}
	if len(args) == 0 || args[0] != run.Identity.IssueID {
		return errors.New("collaboration commands require the admitted issue UUID")
	}
	allowed := map[string]bool{"output": true, "debug": true}
	if operation == "issue comment add" {
		allowed["content"], allowed["content-stdin"], allowed["parent"], allowed["request-id"] = true, true, true, true
	}
	if operation == "squad activity" {
		allowed["reason"], allowed["request-id"] = true, true
	}
	if operation == "collaboration memory update" {
		allowed["base-version"], allowed["request-id"], allowed["content-stdin"], allowed["content-file"] = true, true, true, true
	}
	var unsupported string
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if !allowed[flag.Name] && unsupported == "" {
			unsupported = flag.Name
		}
	})
	if unsupported != "" {
		return fmt.Errorf("--%s is unavailable inside a collaboration run", unsupported)
	}
	if operation == "issue comment add" || operation == "squad activity" || operation == "collaboration memory update" {
		request, _ := cmd.Flags().GetString("request-id")
		id, err := uuid.Parse(request)
		if err != nil || id == uuid.Nil || id.String() != request {
			return errors.New("collaboration writes require --request-id <UUID>; reuse it with the unchanged command after an uncertain result")
		}
	}
	return nil
}

func runCollaborationCLI(cmd *cobra.Command, args []string, operation string) (bool, error) {
	run, err := cli.ReadCollaborationContext()
	if err != nil {
		return true, err
	}
	if run == nil {
		return false, nil
	}
	if err = validateCollaborationCLI(run, cmd, operation, args); err != nil {
		return true, err
	}
	requestID, _ := cmd.Flags().GetString("request-id")
	var body any
	switch operation {
	case "issue comment add":
		content, present, err := resolveTextFlag(cmd, "content")
		if err != nil {
			return true, err
		}
		if !present || strings.TrimSpace(content) == "" {
			return true, errors.New("--content or --content-stdin is required")
		}
		if err := guardLocalPathLinks(content, "comment body", "Files are unavailable in this collaboration runtime."); err != nil {
			return true, err
		}
		payload := map[string]any{"comment_id": requestID, "content": content}
		if parent, _ := cmd.Flags().GetString("parent"); parent != "" {
			id, err := uuid.Parse(parent)
			if err != nil || id == uuid.Nil || id.String() != parent {
				return true, errors.New("--parent must be a comment UUID")
			}
			payload["parent_id"] = parent
		}
		body = payload
	case "squad activity":
		if len(args) != 2 || (args[1] != "action" && args[1] != "no_action" && args[1] != "failed") {
			return true, errors.New("squad outcome must be action, no_action, or failed")
		}
		reason, _ := cmd.Flags().GetString("reason")
		body = map[string]string{"event_id": requestID, "outcome": args[1], "reason": reason}
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	client := run.Client()
	view, err := run.Read(ctx, client)
	if err != nil {
		return true, err
	}
	output, _ := cmd.Flags().GetString("output")
	switch operation {
	case "issue get":
		if output == "table" {
			cli.PrintTable(cmd.OutOrStdout(), []string{"ID", "TITLE", "DESCRIPTION", "RUN", "ROLE"}, [][]string{{view.IssueID, view.Title, view.Description, view.Identity.RunID, view.StepKind}})
			return true, nil
		}
		return true, cli.PrintJSON(cmd.OutOrStdout(), view)
	case "issue comment list":
		if output == "table" {
			rows := make([][]string, 0, len(view.Comments))
			for _, comment := range view.Comments {
				rows = append(rows, []string{comment.ID, comment.AuthorType, comment.AuthorID, comment.Content, comment.CreatedAt})
			}
			cli.PrintTable(cmd.OutOrStdout(), []string{"ID", "TYPE", "AUTHOR", "CONTENT", "CREATED"}, rows)
			return true, nil
		}
		return true, cli.PrintJSON(cmd.OutOrStdout(), view.Comments)
	case "issue comment add", "squad activity":
		path := "/comments"
		if operation == "squad activity" {
			path = "/squad-activity"
		}
		var result map[string]any
		if err = client.PostJSON(ctx, run.Path(path), body, &result); err != nil {
			return true, cli.CollaborationError(err, requestID)
		}
		if operation == "issue comment add" && result["comment_id"] != requestID || operation == "squad activity" && result["code"] != "squad_activity_recorded" {
			return true, cli.CollaborationError(errors.New("invalid collaboration write receipt"), requestID)
		}
		if output == "table" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Collaboration operation recorded: %s\n", requestID)
			return true, nil
		}
		return true, cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	return true, errors.New("command is unavailable inside a collaboration run")
}
