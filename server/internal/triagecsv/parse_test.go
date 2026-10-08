package triagecsv

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The import dialog offers this file as the downloadable template. Parsing it
// here keeps its headers on the server's aliases and its examples valid.
func TestClientImportTemplateMapsEveryColumn(t *testing.T) {
	raw, err := os.ReadFile("../../../packages/views/triage/triage-csv-template.json")
	if err != nil {
		t.Fatal(err)
	}
	var template struct {
		Fields  []string `json:"fields"`
		Locales map[string]struct {
			Headers []string   `json:"headers"`
			Rows    [][]string `json:"rows"`
		} `json:"locales"`
	}
	if err = json.Unmarshal(raw, &template); err != nil {
		t.Fatal(err)
	}
	supported := make([]string, 0, len(fieldAliases))
	for field := range fieldAliases {
		supported = append(supported, field)
	}
	fields := append([]string(nil), template.Fields...)
	sort.Strings(supported)
	sort.Strings(fields)
	if strings.Join(fields, ",") != strings.Join(supported, ",") {
		t.Fatalf("template fields %v must cover mappable fields %v", template.Fields, supported)
	}
	if len(template.Locales) != 2 {
		t.Fatalf("template locales = %d, want en and zh-Hans", len(template.Locales))
	}
	for locale, variant := range template.Locales {
		var file bytes.Buffer
		w := csv.NewWriter(&file)
		if err = w.Write(variant.Headers); err == nil {
			err = w.WriteAll(variant.Rows)
		}
		if err != nil {
			t.Fatal(err)
		}
		p, err := Parse(append([]byte("\xef\xbb\xbf"), file.Bytes()...), nil)
		if err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		for i, header := range variant.Headers {
			if p.Mapping[header] != template.Fields[i] {
				t.Fatalf("%s: header %q maps to %q, want %q", locale, header, p.Mapping[header], template.Fields[i])
			}
		}
		if len(p.Rows) != len(variant.Rows) {
			t.Fatalf("%s: rows = %d, want %d", locale, len(p.Rows), len(variant.Rows))
		}
		for _, row := range p.Rows {
			if len(row.Errors) != 0 || len(row.Warnings) != 0 || row.Duplicate {
				t.Fatalf("%s: example row %d must parse cleanly: %#v", locale, row.Number, row)
			}
		}
	}
}

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

func TestParseNormalizesPriorityLabelsAndKeepsOriginalCells(t *testing.T) {
	cases := []struct{ input, want string }{
		{"紧急", "urgent"}, {"高", "high"}, {"中", "medium"}, {"低", "low"},
		{"无优先级", "none"}, {"未指定优先级", "none"}, {" 高 ", "high"},
		{"High", "high"}, {"URGENT", "urgent"}, {"", "none"},
	}
	var file strings.Builder
	file.WriteString("标题,优先级\n")
	for _, c := range cases {
		file.WriteString("Task,\"" + c.input + "\"\n")
	}
	file.WriteString("Task,最高\nTask,中等\n")
	p, err := Parse([]byte(file.String()), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		row := p.Rows[i]
		if len(row.Errors) != 0 || row.Values["priority"] != c.want {
			t.Fatalf("priority %q = %q, errors %v; want %q", c.input, row.Values["priority"], row.Errors, c.want)
		}
		if row.Cells[1] != c.input {
			t.Fatalf("original cell %q replaced by %q; failure export needs the source text", c.input, row.Cells[1])
		}
	}
	for _, row := range p.Rows[len(cases):] {
		if len(row.Errors) != 1 || !strings.Contains(row.Errors[0], "紧急") {
			t.Fatalf("unknown label %q must fail with the accepted values: %v", row.Cells[1], row.Errors)
		}
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
