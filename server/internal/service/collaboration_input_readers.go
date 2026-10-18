package service

import (
	"sort"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The first consumer alone cannot prove that an input was never read. Later
// member and leader claims can include it in their own immutable prompts.
func collaborationInputReaders(input db.CollaborationGroupInput, runs []db.CollaborationRun) ([]db.CollaborationRun, error) {
	readers := make([]db.CollaborationRun, 0)
	foundConsumer := false
	for _, run := range runs {
		if run.GroupID != input.GroupID {
			continue
		}
		reads := run.ID == input.ConsumingRunID
		foundConsumer = foundConsumer || reads
		if run.PromptSnapshot != nil {
			prompt, err := decodeCollaborationRoster[protocol.CollaborationRunInput](run.PromptSnapshot)
			if err != nil {
				return nil, err
			}
			for _, entry := range prompt {
				reads = reads || entry.CommandID == util.UUIDToString(input.CommandID)
			}
		} else if run.AuthorizedAttemptSeq > 0 || run.StartedAt.Valid || run.BusinessOutcome.String == "success" {
			// Historical or inconsistent launches without a prompt cannot prove
			// they did not read this input. Retain their barrier conservatively.
			reads = true
		}
		if reads {
			readers = append(readers, run)
		}
	}
	if !foundConsumer {
		return nil, invalidCollaborationLedger("input consumer missing from group")
	}
	sort.Slice(readers, func(i, j int) bool { return util.UUIDToString(readers[i].ID) < util.UUIDToString(readers[j].ID) })
	return readers, nil
}

func collaborationReadersNeverLaunched(readers []db.CollaborationRun) bool {
	if len(readers) == 0 {
		return false
	}
	for _, run := range readers {
		if !collaborationRunProvenEnded(run) || run.ProofState != "never_launched_proven" {
			return false
		}
	}
	return true
}
