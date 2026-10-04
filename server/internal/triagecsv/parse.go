// Package triagecsv validates CSV intake without creating tasks or side effects.
package triagecsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxBytes            = 5 * 1024 * 1024
	MaxRows             = 1000
	MaxTitleRunes       = 500
	MaxDescriptionBytes = 1024 * 1024
)

type Row struct {
	Number    int
	Cells     []string
	Values    map[string]string
	Warnings  []string
	Errors    []string
	Duplicate bool
}

type Preview struct {
	Headers []string
	Mapping map[string]string
	Rows    []Row
}

type Failure struct {
	Number int
	Cells  []string
	Error  string
}

func Parse(data []byte, mapping map[string]string) (Preview, error) {
	if len(data) > MaxBytes {
		return Preview{}, fmt.Errorf("CSV exceeds the %d MiB limit", MaxBytes/1024/1024)
	}
	if !utf8.Valid(data) {
		return Preview{}, errors.New("CSV must be UTF-8 encoded; convert the file to UTF-8 before importing")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	r := csv.NewReader(bytes.NewReader(data))
	headers, err := r.Read()
	if err != nil {
		return Preview{}, errors.New("CSV must contain a valid header row")
	}
	seen := make(map[string]bool, len(headers))
	for i, header := range headers {
		headers[i] = strings.TrimSpace(header)
		key := strings.ToLower(headers[i])
		if key == "" || seen[key] {
			return Preview{}, errors.New("CSV headers must be nonempty and unique")
		}
		seen[key] = true
	}
	resolved, err := resolveMapping(headers, mapping)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{Headers: headers, Mapping: resolved, Rows: []Row{}}
	externalIDs := map[string]bool{}
	for {
		cells, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil && !errors.Is(readErr, csv.ErrFieldCount) {
			return Preview{}, fmt.Errorf("invalid CSV: %w", readErr)
		}
		if len(preview.Rows) >= MaxRows {
			return Preview{}, fmt.Errorf("CSV exceeds the %d data row limit", MaxRows)
		}
		row := Row{Number: len(preview.Rows) + 1, Cells: cells, Values: map[string]string{}, Warnings: []string{}, Errors: []string{}}
		if readErr != nil {
			row.Errors = append(row.Errors, "Column count does not match the header")
		}
		for i, header := range headers {
			if i >= len(cells) {
				continue
			}
			value := cells[i]
			if strings.ContainsRune(value, 0) {
				row.Errors = append(row.Errors, fmt.Sprintf("Column %s contains an unsupported NUL character", header))
				continue
			}
			field := resolved[header]
			if field == "ignore" {
				if strings.TrimSpace(value) != "" {
					row.Warnings = append(row.Warnings, fmt.Sprintf("Column %s is not adopted", header))
				}
				continue
			}
			if field != "description" {
				value = strings.TrimSpace(value)
			}
			row.Values[field] = value
		}
		validateRow(&row)
		if ext := row.Values["external_id"]; ext != "" {
			row.Duplicate = externalIDs[ext]
			if row.Duplicate {
				row.Warnings = append(row.Warnings, "External ID repeats within this file; skipped unless explicitly included")
			}
			externalIDs[ext] = true
		}
		preview.Rows = append(preview.Rows, row)
	}
	return preview, nil
}

func WriteFailures(w io.Writer, headers []string, failures []Failure) error {
	cw := csv.NewWriter(w)
	columns := make([]string, 0, len(headers)+2)
	for _, h := range headers {
		columns = append(columns, safeSpreadsheetCell(h))
	}
	columns = append(columns, "source_row", "error")
	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, failure := range failures {
		cells := make([]string, len(headers), len(headers)+2)
		for i := range headers {
			if i < len(failure.Cells) {
				cells[i] = safeSpreadsheetCell(failure.Cells[i])
			}
		}
		cells = append(cells, strconv.Itoa(failure.Number), safeSpreadsheetCell(failure.Error))
		if err := cw.Write(cells); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

var fieldAliases = map[string][]string{
	"title":       {"title", "summary", "标题", "任务标题"},
	"description": {"description", "details", "描述", "详情"},
	"priority":    {"priority", "优先级"},
	"labels":      {"labels", "label", "标签"},
	"project":     {"project", "candidate_project", "项目", "候选项目"},
	"assignee":    {"assignee", "candidate_assignee", "负责人", "候选负责人"},
	"start_date":  {"start_date", "start date", "开始日期"},
	"due_date":    {"due_date", "due date", "deadline", "截止日期"},
	"source_url":  {"source_url", "source url", "url", "原始链接", "来源链接"},
	"external_id": {"external_id", "external id", "外部编号"},
}

func resolveMapping(headers []string, overrides map[string]string) (map[string]string, error) {
	knownHeaders := map[string]bool{}
	for _, h := range headers {
		knownHeaders[h] = true
	}
	for h, field := range overrides {
		if !knownHeaders[h] {
			return nil, fmt.Errorf("mapping references unknown column %q", h)
		}
		if _, ok := fieldAliases[field]; !ok && field != "ignore" {
			return nil, fmt.Errorf("unsupported mapping field %q", field)
		}
	}
	result := map[string]string{}
	used := map[string]bool{}
	// Reserve explicit fields before inferring any other column.
	for _, field := range overrides {
		if field == "ignore" {
			continue
		}
		if used[field] {
			return nil, fmt.Errorf("multiple columns map to %s", field)
		}
		used[field] = true
	}
	for _, h := range headers {
		if field, ok := overrides[h]; ok {
			result[h] = field
			continue
		}
		field := "ignore"
		for canonical, aliases := range fieldAliases {
			for _, alias := range aliases {
				if strings.EqualFold(h, alias) {
					field = canonical
					break
				}
			}
			if field != "ignore" {
				break
			}
		}
		if field != "ignore" {
			if used[field] {
				field = "ignore"
			} else {
				used[field] = true
			}
		}
		result[h] = field
	}
	return result, nil
}

func validateRow(row *Row) {
	if row.Values["title"] == "" {
		row.Errors = append(row.Errors, "Title is required")
	}
	if utf8.RuneCountInString(row.Values["title"]) > MaxTitleRunes {
		row.Errors = append(row.Errors, fmt.Sprintf("Title exceeds %d characters", MaxTitleRunes))
	}
	if len(row.Values["description"]) > MaxDescriptionBytes {
		row.Errors = append(row.Errors, "Description exceeds 1 MiB")
	}
	switch row.Values["priority"] {
	case "":
		row.Values["priority"] = "none"
	case "urgent", "high", "medium", "low", "none":
	default:
		row.Errors = append(row.Errors, "Priority must be urgent, high, medium, low or none")
	}
	for _, field := range []string{"start_date", "due_date"} {
		if value := row.Values[field]; value != "" {
			if d, err := time.Parse("2006-01-02", value); err != nil || d.Format("2006-01-02") != value {
				row.Errors = append(row.Errors, field+" must be a valid YYYY-MM-DD date")
			}
		}
	}
	start, err1 := time.Parse("2006-01-02", row.Values["start_date"])
	due, err2 := time.Parse("2006-01-02", row.Values["due_date"])
	if err1 == nil && err2 == nil && due.Before(start) {
		row.Errors = append(row.Errors, "Due date must not be before start date")
	}
}

func safeSpreadsheetCell(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
