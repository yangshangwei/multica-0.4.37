package daemon

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// TestBudgetUsageRunOpensTheJournalBeforeThePreflightAndStartsTheResendLoop:
// no test starts Run, so the two lines that put the budget usage protocol
// into a running daemon are checked where they stand. Both are statements of
// Run itself, under no condition: the journal is opened once the health port
// is bound, which makes this process the only daemon of its profile, and
// before the preflight; the resend loop starts with the other background
// loops, after the preflight and before the daemon claims its first task.
func TestBudgetUsageRunOpensTheJournalBeforeThePreflightAndStartsTheResendLoop(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Name.Name != "Run" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, isPointer := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !isPointer {
			continue
		}
		if receiver, isIdent := star.X.(*ast.Ident); isIdent && receiver.Name == "Daemon" {
			run = fn
		}
	}
	if run == nil {
		t.Fatal("daemon.go has no method (*Daemon).Run")
	}
	// statements holds the text of every statement of Run's own body.
	statements := make([]string, 0, len(run.Body.List))
	for _, statement := range run.Body.List {
		var text bytes.Buffer
		if err := printer.Fprint(&text, fset, statement); err != nil {
			t.Fatal(err)
		}
		statements = append(statements, text.String())
	}
	// position returns the index of the one statement of Run that match
	// accepts.
	position := func(what string, match func(statement string) bool) int {
		t.Helper()
		found := -1
		for i, statement := range statements {
			if !match(statement) {
				continue
			}
			if found != -1 {
				t.Fatalf("Run has more than one statement that is %s", what)
			}
			found = i
		}
		if found == -1 {
			t.Fatalf("Run has no statement of its own that is %s", what)
		}
		return found
	}
	is := func(want string) func(string) bool {
		return func(statement string) bool { return statement == want }
	}
	startsWith := func(want string) func(string) bool {
		return func(statement string) bool { return strings.HasPrefix(statement, want) }
	}

	healthPort := position("the bind of the health port", startsWith("healthLn, err := d.listenHealth()"))
	journal := position("the opening of the journal", is("d.initializeBudgetUsage()"))
	preflight := position("the preflight", startsWith("if err := d.preflightAuth(ctx); err != nil {"))
	loop := position("the start of the resend loop", is("go d.budgetUsageLoop(ctx, budgetUsageResendInterval)"))
	polling := position("the claiming of tasks", is("err = d.pollLoop(ctx, taskWakeups)"))

	if !(healthPort < journal && journal < preflight) {
		t.Errorf("the journal is opened at statement %d of Run, want after the health port is bound (%d) and before the preflight (%d)",
			journal, healthPort, preflight)
	}
	if !(preflight < loop && loop < polling) {
		t.Errorf("the resend loop starts at statement %d of Run, want after the preflight (%d) and before the daemon claims tasks (%d)",
			loop, preflight, polling)
	}
}
