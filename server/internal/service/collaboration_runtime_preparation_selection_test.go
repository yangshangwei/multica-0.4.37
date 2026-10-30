package service

import (
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func preparationSelectionFile(id string, size int64) protocol.CollaborationPreparationAttachment {
	return protocol.CollaborationPreparationAttachment{ID: id, Version: "1", Name: id, ContentType: "text/plain", SizeBytes: size}
}

func preparationSelectionCaps(fileCount int, perFile, total, workspace int64) *protocol.CollaborationRuntimeCapabilities {
	return &protocol.CollaborationRuntimeCapabilities{Limits: protocol.CollaborationPreparationLimits{
		FileCount: fileCount, PerFileBytes: perFile, TotalInputBytes: total, WorkspaceBytes: workspace,
	}}
}

func TestSelectPreparationAttachmentsSkipsOptionalFilesOutsideRuntimeBudget(t *testing.T) {
	candidates := []protocol.CollaborationPreparationAttachment{
		preparationSelectionFile("referenced", 4),
		preparationSelectionFile("too-large", 20),
		preparationSelectionFile("fits", 4),
	}
	selected, err := selectPreparationAttachments(candidates, map[string]bool{"referenced": true}, preparationSelectionCaps(3, 8, 8, 8))
	if err != nil {
		t.Fatalf("optional history should be skippable: %v", err)
	}
	ids := map[string]bool{}
	for _, file := range selected {
		ids[file.ID] = true
	}
	if len(selected) != 2 || !ids["fits"] || !ids["referenced"] || ids["too-large"] {
		t.Fatalf("selected files = %#v, want fits and referenced only", selected)
	}
}

func TestSelectPreparationAttachmentsRejectsReferencedFileOverRuntimeBudget(t *testing.T) {
	_, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{preparationSelectionFile("referenced", 9)},
		map[string]bool{"referenced": true},
		preparationSelectionCaps(3, 8, 20, 20),
	)
	if !collaborationErrorCode(err, "runtime_preparation_limits") {
		t.Fatalf("referenced file over per-file limit returned %v", err)
	}
}

func TestSelectPreparationAttachmentsRejectsTooManyReferencedFiles(t *testing.T) {
	_, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{
			preparationSelectionFile("one", 1), preparationSelectionFile("two", 1),
		},
		map[string]bool{"one": true, "two": true},
		preparationSelectionCaps(1, 8, 20, 20),
	)
	if !collaborationErrorCode(err, "runtime_preparation_limits") {
		t.Fatalf("referenced count over file-count limit returned %v", err)
	}
}

func TestSelectPreparationAttachmentsRejectsMissingReferencedFile(t *testing.T) {
	_, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{preparationSelectionFile("other", 1)},
		map[string]bool{"missing": true},
		preparationSelectionCaps(2, 8, 20, 20),
	)
	if !collaborationErrorCode(err, "runtime_preparation_unavailable") {
		t.Fatalf("missing referenced file returned %v", err)
	}
}

func TestSelectPreparationAttachmentsSkipsOptionalFilesOutsideTotalBudget(t *testing.T) {
	selected, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{
			preparationSelectionFile("referenced", 4),
			preparationSelectionFile("optional", 5),
		},
		map[string]bool{"referenced": true},
		preparationSelectionCaps(2, 8, 4, 8),
	)
	if err != nil || len(selected) != 1 || selected[0].ID != "referenced" {
		t.Fatalf("optional file exceeded total budget: selected=%#v err=%v", selected, err)
	}
}

func TestSelectPreparationAttachmentsAccountsForGitWorkspaceBudget(t *testing.T) {
	caps := preparationSelectionCaps(2, 8, 8, 8)
	caps.Git = &protocol.CollaborationPreparationGit{}
	caps.Limits.GitExpandedBytes = 5
	selected, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{
			preparationSelectionFile("referenced", 3),
			preparationSelectionFile("optional", 1),
		},
		map[string]bool{"referenced": true}, caps,
	)
	if err != nil || len(selected) != 1 || selected[0].ID != "referenced" {
		t.Fatalf("optional file exceeded Git workspace budget: selected=%#v err=%v", selected, err)
	}

	_, err = selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{preparationSelectionFile("referenced", 4)},
		map[string]bool{"referenced": true}, caps,
	)
	if !collaborationErrorCode(err, "runtime_preparation_limits") {
		t.Fatalf("referenced file over Git workspace budget returned %v", err)
	}
}

func TestSelectPreparationAttachmentsSkipsOptionalFilesOverTotalInputBudget(t *testing.T) {
	selected, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{
			preparationSelectionFile("referenced", 6), preparationSelectionFile("optional", 6),
		},
		map[string]bool{"referenced": true},
		preparationSelectionCaps(2, 8, 8, 20),
	)
	if err != nil || len(selected) != 1 || selected[0].ID != "referenced" {
		t.Fatalf("optional total-budget overflow returned files=%#v err=%v", selected, err)
	}
}

func TestSelectPreparationAttachmentsReservesGitWorkspaceBudget(t *testing.T) {
	caps := preparationSelectionCaps(2, 8, 20, 12)
	caps.Limits.GitExpandedBytes = 6
	caps.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: "commit", BundleDigest: "digest"}
	selected, err := selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{
			preparationSelectionFile("referenced", 5), preparationSelectionFile("optional", 5),
		},
		map[string]bool{"referenced": true}, caps,
	)
	if err != nil || len(selected) != 1 || selected[0].ID != "referenced" {
		t.Fatalf("optional Git workspace overflow returned files=%#v err=%v", selected, err)
	}
	_, err = selectPreparationAttachments(
		[]protocol.CollaborationPreparationAttachment{preparationSelectionFile("referenced", 5)},
		map[string]bool{"referenced": true},
		func() *protocol.CollaborationRuntimeCapabilities {
			c := preparationSelectionCaps(1, 8, 20, 10)
			c.Limits.GitExpandedBytes = 6
			c.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: "commit", BundleDigest: "digest"}
			return c
		}(),
	)
	if !collaborationErrorCode(err, "runtime_preparation_limits") {
		t.Fatalf("referenced Git workspace overflow returned %v", err)
	}
}
