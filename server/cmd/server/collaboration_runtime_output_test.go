package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationRuntimeOutputRouteRequiresBusinessCapability(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	counts := map[string]int{}
	for _, table := range []string{"comment", "collaboration_attachment", "collaboration_run", "collaboration_event"} {
		counts[table] = fx.Count(t, "SELECT count(*) FROM "+table)
	}
	for _, tc := range []struct {
		name, credential string
	}{
		{name: "anonymous"},
		{name: "human_profile", credential: "Bearer " + testToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if err := writer.WriteField("comment_id", uuid.NewString()); err != nil {
				t.Fatal(err)
			}
			if err := writer.WriteField("content", "Runtime output belongs to its Agent."); err != nil {
				t.Fatal(err)
			}
			file, err := writer.CreateFormFile("file", "output.txt")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = file.Write([]byte("Agent output route fixture.\n")); err != nil {
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			request := testutil.JSONRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/comments/attachments", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			if tc.credential != "" {
				request.Header.Set("Authorization", tc.credential)
			}
			response := testutil.Call(t, testServer.Config.Handler.ServeHTTP, request).Want(http.StatusUnauthorized)
			if response.Map()["code"] != "invalid_collaboration_capability" {
				t.Fatal("runtime output route did not reject a non-business credential")
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("runtime output refusal must retain private response headers")
			}
		})
	}
	for table, before := range counts {
		if after := fx.Count(t, "SELECT count(*) FROM "+table); after != before {
			t.Fatalf("unauthorized output changed %s: before=%d after=%d", table, before, after)
		}
	}
}
