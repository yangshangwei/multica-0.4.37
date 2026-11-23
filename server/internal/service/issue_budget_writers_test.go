package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The sqlc queries that insert an issue, write its parent or delete it. A member
// row follows exactly these statements (design section 9.3, acceptance row B60).
var issueBudgetWriterQueries = map[string]string{
	"CreateIssue":                       "insert",
	"CreateIssueWithOrigin":             "insert",
	"UpdateIssue":                       "parent",
	"UpdateCollaborationIssueStructure": "parent",
	"DetachDirectChildIssues":           "parent",
	"PrepareWorkspaceDeletionLinks":     "parent",
	"DeleteIssue":                       "delete",
	"DeleteWorkspaceIssueRoots":         "delete",
}

// Every production call of those queries. Keeping the list here, rather than
// deriving it, is deliberate: a new issue writer is a decision about the budget
// binding, and it should show up as an edit to this list in review.
//
// seam is the binding function the calling function must also call. An entry
// without one carries the reason instead, and a creator without one must pass no
// parent.
var issueBudgetWriterCalls = []struct {
	query    string
	file     string
	function string
	seam     string
	reason   string
}{
	{"CreateIssue", "internal/service/issue.go", "IssueService.CreateInTx", "BindIssueBudgetOnCreate", ""},
	{"CreateIssueWithOrigin", "internal/service/issue.go", "IssueService.CreateInTx", "BindIssueBudgetOnCreate", ""},
	{"CreateIssue", "internal/handler/onboarding_shim.go", "Handler.BootstrapOnboardingRuntime", "", "creates a top-level issue"},
	{"CreateIssue", "internal/handler/onboarding_shim.go", "Handler.BootstrapOnboardingNoRuntime", "", "creates a top-level issue"},
	{"CreateIssueWithOrigin", "internal/service/autopilot.go", "AutopilotService.dispatchCreateIssue", "", "creates a top-level issue"},
	{"UpdateIssue", "internal/handler/issue.go", "Handler.updateIssueAtomicallyOnce", "BindIssueBudgetOnReparent", ""},
	{"UpdateIssue", "internal/handler/issue_move.go", "Handler.MoveIssue", "", "calls the HTTP handler of that name, which ends in updateIssueAtomicallyOnce"},
	{"UpdateCollaborationIssueStructure", "internal/service/collaboration_edit.go", "CollaborationService.EditIssue", "BindIssueBudgetOnReparent", ""},
	{"DetachDirectChildIssues", "internal/handler/issue.go", "Handler.deleteIssuesAndCollectAttachmentURLs", "", "clears the parent, and a member row outlives its parent"},
	{"DeleteIssue", "internal/handler/issue.go", "Handler.deleteIssuesAndCollectAttachmentURLs", "TombstoneIssueBudgetMember", ""},
	{"PrepareWorkspaceDeletionLinks", "internal/handler/workspace.go", "Handler.DeleteWorkspace", "", "workspace deletion writes no budget row"},
	{"DeleteWorkspaceIssueRoots", "internal/handler/workspace.go", "Handler.DeleteWorkspace", "", "workspace deletion writes no budget row"},
}

var (
	issueBudgetWriterQueryName = regexp.MustCompile(`(?m)^-- name: (\w+) :\w+`)
	issueBudgetWriterComment   = regexp.MustCompile(`--[^\n]*`)
	issueBudgetWriterInsert    = regexp.MustCompile(`(?i)\binsert\s+into\s+issue\b`)
	issueBudgetWriterDelete    = regexp.MustCompile(`(?i)\bdelete\s+from\s+issue\b`)
	issueBudgetWriterUpdate    = regexp.MustCompile(`(?i)\bupdate\s+issue\b`)
	issueBudgetWriterParent    = regexp.MustCompile(`(?i)\bparent_issue_id\b`)
)

// issueBudgetWriteKind classifies one SQL statement. An update counts as a
// parent write as soon as it names the column: telling a SET from a WHERE is not
// worth a parser, and a reader that lands here can be listed with its reason.
func issueBudgetWriteKind(sql string) string {
	sql = issueBudgetWriterComment.ReplaceAllString(sql, "")
	switch {
	case issueBudgetWriterInsert.MatchString(sql):
		return "insert"
	case issueBudgetWriterDelete.MatchString(sql):
		return "delete"
	case issueBudgetWriterUpdate.MatchString(sql) && issueBudgetWriterParent.MatchString(sql):
		return "parent"
	}
	return ""
}

type issueBudgetWriterSite struct {
	seams  map[string]bool // binding functions the calling function reaches
	parent string          // source text of the ParentIssueID the call passes
}

func TestIssueBudgetWriterInventoryIsComplete(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// .../server/internal/service -> .../server
	root := filepath.Dir(filepath.Dir(wd))

	queryFiles, err := filepath.Glob(filepath.Join(root, "pkg", "db", "queries", "*.sql"))
	if err != nil || len(queryFiles) == 0 {
		t.Fatalf("list the query files under %s: %d found, %v", root, len(queryFiles), err)
	}
	found := map[string]string{}
	for _, path := range queryFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		names := issueBudgetWriterQueryName.FindAllStringSubmatchIndex(text, -1)
		for i, at := range names {
			end := len(text)
			if i+1 < len(names) {
				end = names[i+1][0]
			}
			if kind := issueBudgetWriteKind(text[at[1]:end]); kind != "" {
				found[text[at[2]:at[3]]] = kind
			}
		}
	}
	for name, kind := range found {
		if issueBudgetWriterQueries[name] != kind {
			t.Errorf("query %s is an issue %s write outside the inventory: give its callers a budget seam or a reason, then list it", name, kind)
		}
	}
	for name, kind := range issueBudgetWriterQueries {
		if found[name] != kind {
			t.Errorf("query %s is listed as an issue %s write, the query files say %q", name, kind, found[name])
		}
	}

	sites := map[string][]issueBudgetWriterSite{}
	var actual []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// generated/ only defines the queries, and testdata/ is not compiled.
			if d.Name() == "generated" || d.Name() == "testdata" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			function := "(package level)"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				function = fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) == 1 {
					receiver := fn.Recv.List[0].Type
					if star, ok := receiver.(*ast.StarExpr); ok {
						receiver = star.X
					}
					if id, ok := receiver.(*ast.Ident); ok {
						function = id.Name + "." + function
					}
				}
			}
			seams := map[string]bool{}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.BasicLit:
					if n.Kind != token.STRING {
						return true
					}
					if value, err := strconv.Unquote(n.Value); err == nil && issueBudgetWriteKind(value) != "" {
						t.Errorf("%s:%d: %s writes issue rows in a string literal: use a listed sqlc query, so the budget seam cannot be skipped", rel, fset.Position(n.Pos()).Line, function)
					}
				case *ast.CallExpr:
					switch callee := n.Fun.(type) {
					case *ast.Ident:
						seams[callee.Name] = true
						if callee.Name == "BindIssueBudgetOnCreate" {
							if len(n.Args) != 4 {
								t.Errorf("%s: create seam lost its verified task argument", rel)
							} else {
								arg := n.Args[3]
								text := string(source[fset.Position(arg.Pos()).Offset:fset.Position(arg.End()).Offset])
								if text != "p.VerifiedTaskID" {
									t.Errorf("%s: create seam task identity = %s, want p.VerifiedTaskID", rel, text)
								}
							}
						}
					case *ast.SelectorExpr:
						seams[callee.Sel.Name] = true
						if _, ok := issueBudgetWriterQueries[callee.Sel.Name]; !ok {
							return true
						}
						site := issueBudgetWriterSite{seams: seams, parent: "(not a literal)"}
						if len(n.Args) > 0 {
							if literal, ok := n.Args[len(n.Args)-1].(*ast.CompositeLit); ok {
								site.parent = ""
								for _, element := range literal.Elts {
									pair, ok := element.(*ast.KeyValueExpr)
									if !ok {
										continue
									}
									if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "ParentIssueID" {
										site.parent = string(source[fset.Position(pair.Value.Pos()).Offset:fset.Position(pair.Value.End()).Offset])
									}
								}
							}
						}
						key := callee.Sel.Name + " in " + rel + " " + function
						sites[key] = append(sites[key], site)
						actual = append(actual, key)
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the production sources under %s: %v", root, err)
	}

	var expected []string
	for _, call := range issueBudgetWriterCalls {
		key := call.query + " in " + call.file + " " + call.function
		expected = append(expected, key)
		if (call.seam == "") == (call.reason == "") {
			t.Errorf("%s: list either the seam or the reason it needs none", key)
		}
		for _, site := range sites[key] {
			switch {
			case call.seam != "":
				if !site.seams[call.seam] {
					t.Errorf("%s no longer calls %s", key, call.seam)
				}
			case issueBudgetWriterQueries[call.query] == "insert":
				if site.parent != "" && site.parent != "emptyUUID" && site.parent != "pgtype.UUID{}" {
					t.Errorf("%s passes the parent %s without calling BindIssueBudgetOnCreate", key, site.parent)
				}
			}
		}
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if got, want := strings.Join(actual, "\n"), strings.Join(expected, "\n"); got != want {
		t.Errorf("the production callers of the issue writers changed: give a new caller a budget seam "+
			"(BindIssueBudgetOnCreate, BindIssueBudgetOnReparent, TombstoneIssueBudgetMember) or a reason, then list it\ngot:\n%s\nwant:\n%s", got, want)
	}
}
