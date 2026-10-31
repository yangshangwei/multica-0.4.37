package main

import (
	"errors"

	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/spf13/cobra"
)

var issueCommentContentCmd = &cobra.Command{
	Use:   "content <issue-id> <comment-id>",
	Short: "Read a revision-bound comment chunk inside a collaboration run",
	Args:  exactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if handled, err := runCollaborationCLI(cmd, args, "issue comment content"); handled {
			return err
		}
		return errors.New("comment content is available only inside a collaboration run")
	},
}

func init() {
	issueCommentContentCmd.Flags().String("revision", "", "Comment revision from its page; edits require restarting from a new page")
	issueCommentContentCmd.Flags().String("cursor", "", "Next content cursor from the previous chunk")
	issueCommentContentCmd.Flags().Int("max-bytes", protocol.CollaborationRecallDefaultBytes, "JSON chunk byte budget (4096-1048576)")
	issueCommentContentCmd.Flags().String("output", "json", "Output format: json")
}
