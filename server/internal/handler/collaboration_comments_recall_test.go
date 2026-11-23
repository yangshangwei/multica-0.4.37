package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRecallHTTPRejectsAmbientCredentials(t *testing.T) {
	bare := &Handler{}
	for _, endpoint := range []http.HandlerFunc{bare.ListCollaborationRuntimeComments, bare.GetCollaborationRuntimeCommentContent} {
		for _, token := range []string{"", "mul_human", "mdt_daemon", "mct_" + strings.Repeat("a", 64)} {
			r := withURLParams(newRequest(http.MethodGet, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/comments", nil), "runId", uuid.NewString(), "commentId", uuid.NewString())
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			testutil.Call(t, endpoint, r).Want(http.StatusUnauthorized)
		}
	}
}

func TestCollaborationRecallHTTPBudgetAndLegacyFullComment(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	body := strings.Repeat("<>&中文🙂", 3000)
	dbfx.Comment(t, issue, body)
	r := collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil)
	r.URL.RawQuery = "max_bytes=4096"
	var page protocol.CollaborationCommentsRecallPage[service.CollaborationCommentView]
	response := testutil.Call(t, testHandler.ListCollaborationComments, r).Want(http.StatusOK)
	response.JSON(&page)
	if page.Bytes != response.Body.Len() || page.Bytes > 4096 || page.Items[0].ContentState != "omitted" || page.Items[0].CanEdit || !page.Truncated {
		t.Fatal("wire budget or placeholder contract failed")
	}
	var legacy struct {
		Items []service.CollaborationCommentView `json:"items"`
	}
	testutil.Call(t, testHandler.ListCollaborationComments, collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil)).Want(http.StatusOK).JSON(&legacy)
	if len(legacy.Items) != 1 || legacy.Items[0].Content != body {
		t.Fatal("old human read lost full comment")
	}
	for _, query := range []string{"max_bytes=1", "max_bytes=1048577", "max_bytes=4096&limit=101", "max_bytes=4096&cursor=invalid", "max_bytes=4096&max_bytes=8192"} {
		r := collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil)
		r.URL.RawQuery = query
		testutil.Call(t, testHandler.ListCollaborationComments, r).Want(http.StatusBadRequest)
	}
}
