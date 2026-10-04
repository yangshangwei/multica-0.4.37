package triagecsv

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestParseQuotedBOMAndIgnoredExecutionFields(t *testing.T) {
	p, err := Parse([]byte("\xef\xbb\xbf标题,描述,优先级,状态,迭代,外部编号\r\n\"修复,登录\",\"first line\n@agent second line\",high,in_progress,sprint1,EXT-1\r\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 1 || p.Rows[0].Number != 1 {
		t.Fatalf("rows = %#v", p.Rows)
	}
	r := p.Rows[0]
	if r.Values["title"] != "修复,登录" || r.Values["description"] != "first line\n@agent second line" || r.Values["external_id"] != "EXT-1" {
		t.Fatalf("decoded values = %#v", r.Values)
	}
	if len(r.Warnings) != 2 || len(r.Errors) != 0 || p.Mapping["状态"] != "ignore" || p.Mapping["迭代"] != "ignore" {
		t.Fatalf("execution columns must warn without adopting: %#v, mapping %#v", r, p.Mapping)
	}
}

func TestParseRejectsWholeInvalidFiles(t *testing.T) {
	cases := map[string][]byte{
		"invalid utf8":     {'t', 'i', 't', 'l', 'e', '\n', 0xff},
		"oversize":         bytes.Repeat([]byte("x"), MaxBytes+1),
		"too many rows":    []byte("title\n" + strings.Repeat("work\n", MaxRows+1)),
		"duplicate header": []byte("title, title\na,b"),
		"empty header":     []byte("title,\na,b"),
		"empty file":       {},
		"bad quote":        []byte("title\n\"unfinished"),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(input, nil); err == nil {
				t.Fatal("expected file rejection before preview")
			}
		})
	}
}

func TestParseRowsKeepValidationErrorsAndWarnings(t *testing.T) {
	p, err := Parse([]byte("title,priority,start_date,due_date,external_id,unknown\n,high,2026-02-30,,E1,x\nValid,invalid,2026-10-04,2026-10-03,E1,\nFine,none,2028-02-29,2028-03-01,E2,\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 3 {
		t.Fatalf("rows=%d", len(p.Rows))
	}
	if len(p.Rows[0].Errors) < 2 || len(p.Rows[0].Warnings) != 1 {
		t.Fatalf("missing title/date/ignored error feedback: %#v", p.Rows[0])
	}
	if len(p.Rows[1].Errors) < 2 || !p.Rows[1].Duplicate {
		t.Fatalf("priority/reversed date/duplicate expected: %#v", p.Rows[1])
	}
	if len(p.Rows[2].Errors) != 0 || p.Rows[2].Duplicate {
		t.Fatalf("valid leap day rejected: %#v", p.Rows[2])
	}
}

func TestParseMappingCanOverrideAndIgnore(t *testing.T) {
	p, err := Parse([]byte("Summary,Details,DoNotUse\nTask,Context,ignored"), map[string]string{"Summary": "title", "Details": "description", "DoNotUse": "ignore"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rows[0].Values["title"] != "Task" || p.Rows[0].Values["description"] != "Context" || len(p.Rows[0].Errors) != 0 {
		t.Fatalf("mapping lost: %#v", p)
	}
	for _, mapping := range []map[string]string{{"Summary": "status"}, {"Summary": "title", "Details": "title"}, {"missing": "title"}} {
		if _, err := Parse([]byte("Summary,Details\nTask,Context"), mapping); err == nil {
			t.Fatalf("invalid mapping accepted: %#v", mapping)
		}
	}
}

func TestParseBoundsFieldSizeAndNULWithoutDroppingValidRows(t *testing.T) {
	p, err := Parse([]byte("title,description\n"+strings.Repeat("界", MaxTitleRunes+1)+",fine\nGood,bad\x00text\nFine,valid\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 3 || len(p.Rows[0].Errors) == 0 || len(p.Rows[1].Errors) == 0 || len(p.Rows[2].Errors) != 0 {
		t.Fatalf("row validation = %#v", p.Rows)
	}
}

func TestFailureCSVPreservesCellsAndNeutralizesFormula(t *testing.T) {
	var out bytes.Buffer
	err := WriteFailures(&out, []string{"title", "description"}, []Failure{{Number: 2, Cells: []string{"=SUM(A1)", "line1\n\"quoted, cell\""}, Error: "bad date"}, {Number: 3, Cells: []string{" \t@SUM(A1)", "normal"}, Error: "retry"}})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][0] != "'=SUM(A1)" || rows[1][1] != "line1\n\"quoted, cell\"" || rows[1][2] != "2" || rows[1][3] != "bad date" || !strings.HasPrefix(rows[2][0], "'") {
		t.Fatalf("export = %#v", rows)
	}
}
