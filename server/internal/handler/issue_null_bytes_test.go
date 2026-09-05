package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateIssueRemovesNullBytesFromTextFields(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "title\x00with-null",
		"description": "description\x00with-null",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { deleteTestIssue(t, issue.ID) })

	if strings.Contains(issue.Title, "\x00") || issue.Description == nil || strings.Contains(*issue.Description, "\x00") {
		description := "<nil>"
		if issue.Description != nil {
			description = *issue.Description
		}
		t.Fatalf("response retained NUL byte: title=%q description=%q", issue.Title, description)
	}
	var title, description string
	if err := testPool.QueryRow(context.Background(), `SELECT title, description FROM issue WHERE id = $1`, issue.ID).Scan(&title, &description); err != nil {
		t.Fatalf("load created issue: %v", err)
	}
	if title != "titlewith-null" || description != "descriptionwith-null" {
		t.Fatalf("stored text was not sanitized: title=%q description=%q", title, description)
	}
}

func TestUpdateIssueRemovesNullBytesFromTextFields(t *testing.T) {
	issueID := createTestIssue(t, "initial title", "todo", "none")
	t.Cleanup(func() { deleteTestIssue(t, issueID) })

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"title":       "updated\x00title",
		"description": "updated\x00description",
	})
	req = withURLParam(req, "id", issueID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var title, description string
	if err := testPool.QueryRow(context.Background(), `SELECT title, description FROM issue WHERE id = $1`, issueID).Scan(&title, &description); err != nil {
		t.Fatalf("load updated issue: %v", err)
	}
	if title != "updatedtitle" || description != "updateddescription" {
		t.Fatalf("stored text was not sanitized: title=%q description=%q", title, description)
	}
}
