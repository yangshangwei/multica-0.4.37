package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestSanitizeIssueCreateParamsRemovesNullBytes(t *testing.T) {
	params := IssueCreateParams{
		Title:       "title\x00with-null",
		Description: pgtype.Text{String: "description\x00with-null", Valid: true},
	}

	got := sanitizeIssueCreateParams(params)
	if got.Title != "titlewith-null" {
		t.Fatalf("title = %q, want NUL removed", got.Title)
	}
	if !got.Description.Valid || got.Description.String != "descriptionwith-null" {
		t.Fatalf("description = %#v, want NUL removed", got.Description)
	}
}
