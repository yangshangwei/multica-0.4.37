package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeliveryCompletionPreviewStrictRequest(t *testing.T) {
	good := `{"expected_issue_revision":7,"target_status":" DONE ","approval_id":"11111111-1111-4111-8111-111111111111"}`
	req, err := ParseDeliveryCompletionPreview([]byte(good))
	if err != nil || req.TargetStatus != "done" || req.ExpectedIssueRevision != 7 {
		t.Fatal(req, err)
	}
	for _, body := range []string{
		`{}`, `null`, good + `{}`, strings.Replace(good, `"expected_issue_revision":7`, `"expected_issue_revision":null`, 1),
		strings.Replace(good, `"expected_issue_revision":7`, `"expected_issue_revision":-1`, 1),
		strings.Replace(good, `"expected_issue_revision":7`, `"expected_issue_revision":7.5`, 1),
		strings.Replace(good, `"expected_issue_revision":7`, `"Expected_issue_revision":7`, 1),
		strings.Replace(good, `" DONE "`, `"done\u0000"`, 1),
		strings.TrimSuffix(good, "}") + `,"evaluation_key":null}`,
		strings.TrimSuffix(good, "}") + `,"target_status":"done"}`,
	} {
		if _, err := ParseDeliveryCompletionPreview([]byte(body)); err == nil {
			t.Fatal("accepted malformed preview", body)
		}
	}
}

func TestDeliveryCompletionCandidateCursorStrictBinding(t *testing.T) {
	now := time.Date(2026, 11, 12, 1, 0, 0, 1000, time.UTC)
	c := deliveryCandidateCursor{IssueID: "11111111-1111-4111-8111-111111111111", IssueRevision: 7, WorkspaceID: "22222222-2222-4222-8222-222222222222", ViewerID: "33333333-3333-4333-8333-333333333333", UpperBound: now, CreatedAt: now.Add(-time.Minute), ApprovalID: "44444444-4444-4444-8444-444444444444"}
	raw, _ := json.Marshal(c)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	decoded, err := parseDeliveryCandidateCursor(encoded)
	if err != nil || decoded != c {
		t.Fatal(decoded, err)
	}
	for _, value := range []string{"", encoded + "=", strings.Repeat("x", 2049), base64.RawURLEncoding.EncodeToString([]byte(`{}`)), base64.RawURLEncoding.EncodeToString(append(raw[:len(raw)-1], []byte(`,"issue_revision":8}`)...))} {
		if _, err := parseDeliveryCandidateCursor(value); err == nil {
			t.Fatal("accepted malformed cursor")
		}
	}
}
