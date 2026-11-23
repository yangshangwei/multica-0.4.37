package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// budgetFixtureDir holds the frozen examples of the budget HTTP shape (design
// 12.9). The server lane reads the same files: it decodes each one into its
// own structs and must encode the same bytes again.
const budgetFixtureDir = "../../internal/handler/testdata/issue_budget_http_v1"

// budgetFixture reads one fixture file.
func budgetFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(budgetFixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// budgetFixtureAnswers names, for every answer among the fixture files, the
// struct the commands read it into.
var budgetFixtureAnswers = map[string]func() any{
	"write-201-enable.json":               func() any { return new(budgetWriteAnswer) },
	"write-201-enable-no-gaps.json":       func() any { return new(budgetWriteAnswer) },
	"write-200-replayed-set-limit.json":   func() any { return new(budgetWriteAnswer) },
	"operation-committed.json":            func() any { return new(budgetOperationCommitted) },
	"operation-committed-stale.json":      func() any { return new(budgetOperationCommitted) },
	"operation-unknown.json":              func() any { return new(budgetOperationUnknown) },
	"issue-binding.json":                  func() any { return new(budgetIssueBinding) },
	"show-open.json":                      func() any { return new(budgetShow) },
	"show-known.json":                     func() any { return new(budgetShow) },
	"show-no-limit.json":                  func() any { return new(budgetShow) },
	"show-unavailable.json":               func() any { return new(budgetShow) },
	"show-tombstoned-off.json":            func() any { return new(budgetShow) },
	"observations-page.json":              func() any { return new(budgetObservationPage) },
	"observations-last-page.json":         func() any { return new(budgetObservationPage) },
	"observations-empty.json":             func() any { return new(budgetObservationPage) },
	"attempts-page.json":                  func() any { return new(budgetAttemptPage) },
	"attempts-unavailable.json":           func() any { return new(budgetAttemptPage) },
	"attempts-empty.json":                 func() any { return new(budgetAttemptPage) },
	"error-budget-forbidden.json":         func() any { return new(budgetErrorBody) },
	"error-not-found.json":                func() any { return new(budgetErrorBody) },
	"error-policy-revision-conflict.json": func() any { return new(budgetErrorBody) },
}

// budgetFixtureRequests names, for every request among the fixture files, the
// struct the commands encode.
var budgetFixtureRequests = map[string]func() any{
	"enable-request.json":              func() any { return new(budgetEnableRequest) },
	"operation-set-limit-request.json": func() any { return new(budgetSetLimitRequest) },
	"operation-set-mode-request.json":  func() any { return new(budgetSetModeRequest) },
	"operation-close-request.json":     func() any { return new(budgetOperationRequest) },
	"operation-reopen-request.json":    func() any { return new(budgetOperationRequest) },
}

// Every fixture file is one compact JSON document on one line, ends with one
// line feed, is valid UTF-8, has no key twice, and holds nothing the structs of
// the commands do not know: it is read with DisallowUnknownFields at every
// depth. A request is also encoded again from its struct and must give the
// bytes of the file, which pins the order of its fields.
func TestIssueBudgetFixtureFiles(t *testing.T) {
	entries, err := os.ReadDir(budgetFixtureDir)
	if err != nil {
		t.Fatalf("read the fixture directory: %v", err)
	}
	var onDisk []string
	for _, entry := range entries {
		onDisk = append(onDisk, entry.Name())
	}
	var known []string
	for name := range budgetFixtureAnswers {
		known = append(known, name)
	}
	for name := range budgetFixtureRequests {
		known = append(known, name)
	}
	sort.Strings(onDisk)
	sort.Strings(known)
	if strings.Join(onDisk, "\n") != strings.Join(known, "\n") {
		t.Fatalf("the fixture directory and this test disagree about the files:\non disk: %v\nchecked: %v", onDisk, known)
	}

	for _, name := range known {
		t.Run(name, func(t *testing.T) {
			data := budgetFixture(t, name)
			if !utf8.Valid(data) {
				t.Fatal("the file is not valid UTF-8")
			}
			if !bytes.HasSuffix(data, []byte("\n")) || bytes.Count(data, []byte("\n")) != 1 || bytes.ContainsRune(data, '\r') {
				t.Fatalf("the file must be one line that ends with one line feed; it has %d line feeds", bytes.Count(data, []byte("\n")))
			}
			document := bytes.TrimSuffix(data, []byte("\n"))
			var compact bytes.Buffer
			if err := json.Compact(&compact, document); err != nil {
				t.Fatalf("the file is not one JSON document: %v", err)
			}
			if !bytes.Equal(compact.Bytes(), document) {
				t.Fatal("the JSON document is not in compact form")
			}
			duplicate, err := budgetFixtureDuplicateKey(document)
			if err != nil {
				t.Fatalf("walk the document: %v", err)
			}
			if duplicate != "" {
				t.Fatalf("key %s is there twice", duplicate)
			}

			if newAnswer, ok := budgetFixtureAnswers[name]; ok {
				// The whole file, line feed included: that is what a command
				// receives.
				if err := readBudgetAnswer(data, newAnswer(), true); err != nil {
					t.Fatalf("strict reading: %v (decoder: %v)", err, errors.Unwrap(err))
				}
				if err := understandBudgetAnswer(data, newAnswer()); err != nil {
					t.Fatalf("the reading of the commands: %v (decoder: %v)", err, errors.Unwrap(err))
				}
				return
			}
			request := budgetFixtureRequests[name]()
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(request); err != nil {
				t.Fatalf("strict decoding of the request: %v", err)
			}
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("encode the request again: %v", err)
			}
			if !bytes.Equal(encoded, document) {
				t.Fatalf("the request struct does not encode the bytes of the file:\nfile:    %s\nencoded: %s", document, encoded)
			}
		})
	}
}

// budgetFixtureDuplicateKey walks one JSON document token by token and returns
// the path of the first key that is there twice in one object, or "". It does
// not share anything with the reading code it is used to check.
func budgetFixtureDuplicateKey(document []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var walk func(path string) (string, error)
	walk = func(path string) (string, error) {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		delim, isDelim := token.(json.Delim)
		if !isDelim {
			return "", nil
		}
		seen := map[string]bool{}
		for index := 0; decoder.More(); index++ {
			child := fmt.Sprintf("%s[%d]", path, index)
			if delim == '{' {
				keyToken, err := decoder.Token()
				if err != nil {
					return "", err
				}
				key, _ := keyToken.(string)
				child = path + "." + key
				if seen[key] {
					return child, nil
				}
				seen[key] = true
			}
			if duplicate, err := walk(child); err != nil || duplicate != "" {
				return duplicate, err
			}
		}
		_, err = decoder.Token() // the closing bracket
		return "", err
	}
	return walk("")
}

// The walk above must find a repeated key wherever it is, or the check of the
// files proves nothing.
func TestIssueBudgetFixtureCheckFindsARepeatedKey(t *testing.T) {
	for document, want := range map[string]string{
		`{"a":1,"b":{"c":[1,{"d":null}]},"e":"a"}`: "",
		`{"a":1,"a":1}`:                                 ".a",
		`{"a":1,"b":2,"a":null}`:                        ".a",
		`{"a":{"b":1,"c":2,"b":3}}`:                     ".a.b",
		`{"a":[{"b":1},{"b":1,"c":{"d":"x","d":"x"}}]}`: ".a[1].c.d",
		`[{"a":1},{"a":1,"a":2}]`:                       "[1].a",
		`{"detail":{"by_reason":{"x":1,"y":2,"x":3}}}`:  ".detail.by_reason.x",
		`{"a":{"b":1},"c":{"b":1}}`:                     "",
		`{"a":[{"b":1},{"b":1}]}`:                       "",
	} {
		got, err := budgetFixtureDuplicateKey([]byte(document))
		if err != nil || got != want {
			t.Errorf("walk %s = %q, %v; want %q", document, got, err, want)
		}
	}
}

// Strict reading refuses a key the structs do not know at every depth, and the
// reading of the commands ignores the same key at every depth.
func TestIssueBudgetStrictReadingReachesEveryDepth(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		after   string // the text an unknown key is put in front of
	}{
		{"write-201-enable.json", `"replayed":`},
		{"write-201-enable.json", `"operation_id":`},
		{"operation-committed.json", `"operation_kind":`},
		{"show-open.json", `"budget_root_id":`},
		{"show-open.json", `"actual_known_ticks":`},
		{"show-open.json", `"reason":"lost_contact"`},
		{"show-open.json", `"pre_activation":`},
		{"show-open.json", `"included_attempts":`},
		{"observations-page.json", `"kind":"coverage_gap"`},
		{"attempts-page.json", `"task_id":"1f2e3d4c`},
		{"attempts-page.json", `"status":"held"`},
		{"error-not-found.json", `"error":`},
	} {
		data := budgetFixture(t, tc.fixture)
		if bytes.Count(data, []byte(tc.after)) < 1 {
			t.Fatalf("%s does not hold %s", tc.fixture, tc.after)
		}
		changed := bytes.Replace(data, []byte(tc.after), []byte(`"added_by_a_newer_server":{"x":[1]},`+tc.after), 1)
		newAnswer := budgetFixtureAnswers[tc.fixture]
		if err := readBudgetAnswer(changed, newAnswer(), true); err == nil {
			t.Errorf("%s with an unknown key before %s: strict reading accepted it", tc.fixture, tc.after)
		}
		if err := understandBudgetAnswer(changed, newAnswer()); err != nil {
			t.Errorf("%s with an unknown key before %s: the commands did not ignore it: %v", tc.fixture, tc.after, err)
		}
	}
}
