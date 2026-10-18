package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// replaceSkillFiles borrows the caller's transaction and already locked skill
// row. Preserve unchanged files so re-importing a bundle does not invalidate
// execution consent or replace file identity merely because it was re-saved.
func replaceSkillFiles(ctx context.Context, q *db.Queries, skillID pgtype.UUID, files []CreateSkillFileRequest) ([]SkillFileResponse, error) {
	existing, err := q.ListSkillFiles(ctx, skillID)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]db.SkillFile, len(existing))
	for _, file := range existing {
		byPath[file.Path] = file
	}
	retained := make(map[string]bool, len(files))
	responses := make([]SkillFileResponse, 0, len(files))
	for _, file := range files {
		path, content := sanitizeNullBytes(file.Path), sanitizeNullBytes(file.Content)
		stored, exists := byPath[path]
		if !exists || stored.Content != content {
			stored, err = q.UpsertSkillFile(ctx, db.UpsertSkillFileParams{SkillID: skillID, Path: path, Content: content})
			if err != nil {
				return nil, err
			}
			byPath[path] = stored
		}
		retained[path] = true
		responses = append(responses, skillFileToResponse(stored))
	}
	for _, file := range existing {
		if !retained[file.Path] {
			if err := q.DeleteSkillFile(ctx, file.ID); err != nil {
				return nil, err
			}
		}
	}
	return responses, nil
}
