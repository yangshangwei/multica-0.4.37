package handler

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

func TestIssueBudgetManagementFrozenWire(t *testing.T) {
	files, err := filepath.Glob("testdata/issue_budget_http_v1/*.json")
	if err != nil || len(files) != 26 {
		t.Fatalf("frozen fixtures = %d, %v", len(files), err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			var value any
			name := filepath.Base(file)
			switch {
			case name == "enable-request.json":
				value = &issueBudgetEnableBody{}
			case name == "operation-set-limit-request.json":
				value = &issueBudgetSetLimitBody{}
			case name == "operation-set-mode-request.json":
				value = &issueBudgetSetModeBody{}
			case strings.HasSuffix(name, "-request.json"):
				if name != "operation-close-request.json" && name != "operation-reopen-request.json" {
					t.Fatalf("unclaimed fixture %q", name)
				}
				value = &issueBudgetOperationBody{}
			case strings.HasPrefix(name, "write-"):
				value = &service.IssueBudgetWriteResponse{}
			case strings.HasPrefix(name, "operation-committed"):
				value = &service.IssueBudgetOperationLookup{}
			case name == "operation-unknown.json":
				value = &service.IssueBudgetOperationUnknown{}
			case strings.HasPrefix(name, "error-"):
				value = &issueBudgetErrorBody{}
			case name == "issue-binding.json":
				value = &service.IssueBudgetBinding{}
			case strings.HasPrefix(name, "show-"):
				value = &service.IssueBudgetShow{}
			case strings.HasPrefix(name, "observations-"):
				value = &service.IssueBudgetObservationPage{}
			case strings.HasPrefix(name, "attempts-"):
				value = &service.IssueBudgetAttemptPage{}
			default:
				t.Fatalf("unclaimed fixture %q", name)
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			d := json.NewDecoder(bytes.NewReader(raw))
			d.DisallowUnknownFields()
			if err := d.Decode(value); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value)
			if err != nil || !bytes.Equal(append(encoded, '\n'), raw) {
				t.Fatalf("wire drift (%v)\ngot  %s\nwant %s", err, encoded, raw)
			}
		})
	}
	value := service.IssueBudgetWriteResponse{Result: service.IssueBudgetOperationResult{Reason: "a<b&c"}}
	raw, err := json.Marshal(value)
	var decoded service.IssueBudgetWriteResponse
	if err != nil || json.Unmarshal(raw, &decoded) != nil || decoded.Result.Reason != value.Result.Reason {
		t.Fatalf("escaped reason did not survive JSON: %s (%v)", raw, err)
	}
}

func TestIssueBudgetManagementStrictBody(t *testing.T) {
	good := `{"request_key":"3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10","reason":"track","limit_ticks":null,"attempt_reserve_ticks":null}`
	decode := func(raw string) error {
		return decodeIssueBudgetBody([]byte(raw), &issueBudgetEnableBody{}, "request_key", "reason", "limit_ticks", "attempt_reserve_ticks")
	}
	if err := decode(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(good, `"reason":"track"`, `"Reason":"track"`, 1),
		strings.Replace(good, `"reason":"track"`, `"reason":null`, 1),
		strings.Replace(good, `"reason":"track"`, `"reason":true`, 1),
		strings.Replace(good, `"limit_ticks":null,`, "", 1),
		strings.Replace(good, `"limit_ticks":null`, `"limit_ticks":0`, 1),
		strings.Replace(good, `"limit_ticks":null`, `"limit_ticks":"01"`, 1),
		strings.Replace(good, `"limit_ticks":null`, `"limit_ticks":"+1"`, 1),
		strings.Replace(good, `"limit_ticks":null`, `"limit_ticks":"-1"`, 1),
		strings.Replace(good, `"limit_ticks":null`, `"limit_ticks":"9223372036854775808"`, 1),
		strings.Replace(good, `"reason":"track"`, `"reason":"track","reason":"again"`, 1),
		strings.Replace(good, `"reason":"track"`, `"reason":"track","unknown":1`, 1),
		good + `{}`, `null`, `[]`, strings.Replace(good, "track", string([]byte{255}), 1),
	} {
		if err := decode(bad); err == nil {
			t.Errorf("accepted malformed body %q", bad)
		}
	}
	base := `{"kind":"close","request_key":"3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10","reason":"track","expected_policy_revision":"1"}`
	if _, _, err := parseIssueBudgetOperation([]byte(base)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(base, `"1"}`, `1}`, 1), strings.Replace(base, `"1"}`, `null}`, 1),
		strings.Replace(base, `"close"`, `"unknown"`, 1), strings.Replace(base, `"1"}`, `"1","mode":"off"}`, 1),
		strings.Replace(base, "3f0e6f0a", "3F0E6F0A", 1), strings.Replace(base, "3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10", "00000000-0000-0000-0000-000000000000", 1),
	} {
		if _, _, err := parseIssueBudgetOperation([]byte(bad)); err == nil {
			t.Errorf("accepted malformed operation %s", bad)
		}
	}
}

func TestIssueBudgetManagementPageParameters(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		limit int
	}{{"", 50}, {"limit=1", 1}, {"limit=100", 100}} {
		if p, err := parseIssueBudgetPage(tc.raw); err != nil || p.Limit != tc.limit {
			t.Fatalf("%q = %+v %v", tc.raw, p, err)
		}
	}
	for _, bad := range []string{"limit=0", "limit=101", "limit=-1", "limit=01", "limit=text", "limit=1&limit=2", "cursor=x&cursor=x", "cursor=", "cursor=invalid", "cursor=%zz", "limit=1;cursor=x"} {
		if _, err := parseIssueBudgetPage(bad); err == nil {
			t.Errorf("accepted page parameters %q", bad)
		}
	}
}
