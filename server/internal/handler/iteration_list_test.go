package handler

import (
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/url"
	"testing"
)

func TestIterationListKeysetAndScopeBinding(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	lifecycleHTTPCreate(t, h, "List alpha")
	lifecycleHTTPCreate(t, h, "List beta")
	lifecycleHTTPCreate(t, h, "List gamma")
	var page struct {
		Items []iteration.Iteration `json:"items"`
		Next  *string               `json:"next_cursor"`
	}
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?limit=1&search=List", "", nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Next == nil {
		t.Fatalf("first page=%+v", page)
	}
	first := page.Items[0].ID
	cursor := *page.Next
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?limit=1&search=List&cursor="+url.QueryEscape(cursor), "", nil)).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0].ID == first {
		t.Fatal("keyset repeated first item")
	}
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?limit=1&search=other&cursor="+url.QueryEscape(cursor), "", nil)).Want(409)
	lifecycleHTTPCreate(t, h, "List added")
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?limit=1&search=List&cursor="+url.QueryEscape(cursor), "", nil)).Want(409)
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?from=2026-02-30", "", nil)).Want(400)
	testutil.Call(t, h.ListIterations, lifecycleHTTPRequest("GET", "iterations?status=mystery", "", nil)).Want(400)
}
