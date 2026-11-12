package db

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestKnowledgeTaskMetadataNeverUsesGeneratedPublicJSON(t *testing.T) {
	version := pgtype.Int4{Int32: 1, Valid: true}
	private := []byte(`{"candidate_id":"PRIVATE_CANDIDATE","manifest_sha256":"PRIVATE_HASH"}`)
	for name, row := range map[string]any{
		"task":             AgentTaskQueue{Status: "running", KnowledgeContextVersion: version, KnowledgeDelivery: private},
		"derived task row": CancelDeferredEscalationsForIssueAgentRow{Status: "cancelled", KnowledgeContextVersion: version, KnowledgeDelivery: private},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"knowledge_delivery", "knowledge_context_version"} {
				if _, ok := fields[key]; ok {
					t.Fatalf("server-only field %s reached task JSON", key)
				}
			}
			if _, ok := fields["status"]; !ok {
				t.Fatal("ordinary public task fields disappeared")
			}
		})
	}
}

func TestKnowledgeGeneratedMetadataTagInventory(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok || len(field.Names) != 1 {
				return true
			}
			name := field.Names[0].Name
			tag := ""
			if field.Tag != nil {
				tag, _ = strconv.Unquote(field.Tag.Value)
			}
			jsonTag := reflect.StructTag(tag).Get("json")
			if name == "KnowledgeDelivery" || name == "KnowledgeContextVersion" {
				count++
				if jsonTag != "-" {
					t.Errorf("%s: %s remains public: %q", path, name, jsonTag)
				}
			}
			if jsonTag == "knowledge_delivery" || jsonTag == "knowledge_context_version" {
				t.Errorf("%s: aliased metadata field %s remains public", path, name)
			}
			return true
		})
	}
	if count < 4 {
		t.Fatalf("metadata inventory did not include base and derived task fields: %d", count)
	}
	t.Logf("verified %d generated server-only metadata fields", count)
}
