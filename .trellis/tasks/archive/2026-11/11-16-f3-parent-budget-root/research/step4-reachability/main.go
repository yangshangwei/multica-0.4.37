// Reachability for the step 4 isolation question (item 8).
//
// The question is not "can this function ever lead to a seam" (through the
// standard library everything leads everywhere: context cancellation reaches
// sync.Once, which reaches every func() in the program). It is "can the code
// that runs inside this transaction call a seam". A seam needs the
// transaction's *db.Queries, and only module code can hand that on. So the
// walk stays in module code:
//
//	calls     static calls, and calls through interfaces and function values
//	          as resolved by VTA (two passes over a CHA graph), when the
//	          callee is module code;
//	closures  every function value a reached function creates or names
//	          (it may be handed to library code that calls it back);
//	iface     (second mode only) every method of a module type that a reached
//	          function converts to an interface (library code may call it:
//	          sort.Interface, json.Marshaler, driver.Valuer, a tracer).
//
// Library bodies are not entered. The result over-approximates what module
// code can run; it is sound up to reflection, unsafe and linkname.
//
// Forward:  from every function that names REPEATABLE READ or SERIALIZABLE.
// Backward: every function that begins a transaction and can reach a seam.
//
// Usage: cg <server dir>
package main

import (
	"fmt"
	"go/constant"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const module = "github.com/multica-ai/multica/server/"

func name(fn *ssa.Function) string { return strings.ReplaceAll(fn.String(), module, "") }

func inModule(fn *ssa.Function) bool {
	for f := fn; f != nil; f = f.Parent() {
		if f.Pkg != nil {
			return strings.HasPrefix(f.Pkg.Pkg.Path(), module)
		}
	}
	return strings.Contains(fn.String(), module)
}

func where(prog *ssa.Program, fn *ssa.Function) string {
	p := prog.Fset.Position(fn.Pos())
	if !p.IsValid() {
		return "-"
	}
	i := strings.Index(p.Filename, "/server/")
	if i < 0 {
		return p.Filename
	}
	return fmt.Sprintf("%s:%d", p.Filename[i+len("/server/"):], p.Line)
}

var (
	seams = []string{
		"internal/service.BindIssueBudgetOnCreate",
		"internal/service.BindIssueBudgetOnReparent",
		"internal/service.TombstoneIssueBudgetMember",
		"(*internal/service.IssueService).CreateInTx",
	}
	writers = []string{
		"(*pkg/db/generated.Queries).CreateIssue",
		"(*pkg/db/generated.Queries).CreateIssueWithOrigin",
		"(*pkg/db/generated.Queries).UpdateIssue",
		"(*pkg/db/generated.Queries).UpdateCollaborationIssueStructure",
		"(*pkg/db/generated.Queries).DetachDirectChildIssues",
		"(*pkg/db/generated.Queries).PrepareWorkspaceDeletionLinks",
		"(*pkg/db/generated.Queries).DeleteIssue",
		"(*pkg/db/generated.Queries).DeleteWorkspaceIssueRoots",
	}
)

// markers returns the isolation statements and levels a function names.
func markers(fn *ssa.Function) []string {
	seen := map[string]bool{}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			for _, op := range in.Operands(nil) {
				k, ok := (*op).(*ssa.Const)
				if !ok || k.Value == nil || k.Value.Kind() != constant.String {
					continue
				}
				s := constant.StringVal(k.Value)
				switch {
				case strings.Contains(strings.ToUpper(s), "ISOLATION LEVEL"), strings.Contains(strings.ToLower(s), "transaction_isolation"):
					seen[strings.TrimSpace(strings.TrimPrefix(s, "SET TRANSACTION ISOLATION LEVEL"))] = true
				case strings.HasSuffix(k.Type().String(), "pgx/v5.TxIsoLevel"):
					seen["pgx:"+s] = true
				}
			}
		}
	}
	var out []string
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func kind(ms []string) string {
	write, readOnly, committed := false, false, false
	for _, m := range ms {
		u := strings.ToUpper(m)
		switch {
		case strings.Contains(u, "REPEATABLE READ") || strings.Contains(u, "SERIALIZABLE"):
			if strings.Contains(u, "READ ONLY") {
				readOnly = true
			} else {
				write = true
			}
		case strings.Contains(u, "READ COMMITTED"):
			committed = true
		}
	}
	switch {
	case write:
		return "RR-WRITE"
	case readOnly:
		return "RR-READ-ONLY"
	case committed:
		return "RC-EXPLICIT"
	}
	return "OTHER"
}

func begins(fn *ssa.Function) bool {
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			call, ok := in.(ssa.CallInstruction)
			if !ok {
				continue
			}
			c := call.Common()
			callee := ""
			if c.IsInvoke() {
				callee = c.Method.Name()
			} else if s := c.StaticCallee(); s != nil {
				callee = s.Name()
			}
			if callee == "Begin" || callee == "BeginTx" || callee == "BeginFunc" || callee == "BeginTxFunc" {
				return true
			}
		}
	}
	return false
}

// beginsVia reports how a function comes to own a transaction: it calls Begin
// itself, or it calls a module helper that begins one and hands it back.
func beginsVia(fn *ssa.Function, depth int) string {
	if begins(fn) {
		return "begins"
	}
	if depth > 3 {
		return ""
	}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			call, ok := in.(ssa.CallInstruction)
			if !ok {
				continue
			}
			callee := call.Common().StaticCallee()
			if callee == nil || !inModule(callee) || !returnsTx(callee) {
				continue
			}
			if beginsVia(callee, depth+1) != "" {
				return "begins through " + name(callee)
			}
		}
	}
	return ""
}

func returnsTx(fn *ssa.Function) bool {
	results := fn.Signature.Results()
	for i := 0; i < results.Len(); i++ {
		if strings.Contains(results.At(i).Type().String(), "pgx/v5.Tx") {
			return true
		}
	}
	return false
}

func main() {
	initial, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Dir: os.Args[1]}, "./...")
	if err != nil {
		panic(err)
	}
	if packages.PrintErrors(initial) > 0 {
		os.Exit(1)
	}
	prog, _ := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()
	all := ssautil.AllFunctions(prog)
	graph := vta.CallGraph(all, cha.CallGraph(prog))
	graph = vta.CallGraph(all, graph)
	byName := map[string]*ssa.Function{}
	moduleFunctions := 0
	for fn := range all {
		byName[name(fn)] = fn
		if inModule(fn) {
			moduleFunctions++
		}
	}
	find := func(n string) *ssa.Function {
		fn := byName[n]
		if fn == nil {
			panic("no function " + n)
		}
		return fn
	}
	fmt.Printf("functions: %d (%d in the module), call graph nodes: %d\n", len(all), moduleFunctions, len(graph.Nodes))

	// The module-only successor relation. label says why the edge exists.
	type edge struct {
		to    *ssa.Function
		label string
	}
	successors := func(fn *ssa.Function, iface bool) []edge {
		var out []edge
		if node := graph.Nodes[fn]; node != nil {
			for _, e := range node.Out {
				if inModule(e.Callee.Func) {
					out = append(out, edge{e.Callee.Func, "calls"})
				}
			}
		}
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				for _, op := range in.Operands(nil) {
					if f, ok := (*op).(*ssa.Function); ok && inModule(f) {
						out = append(out, edge{f, "names"})
					}
				}
				mi, ok := in.(*ssa.MakeInterface)
				if !iface || !ok {
					continue
				}
				T := mi.X.Type()
				methods := prog.MethodSets.MethodSet(T)
				for i := 0; i < methods.Len(); i++ {
					if m := prog.MethodValue(methods.At(i)); m != nil && inModule(m) {
						out = append(out, edge{m, "converts " + types.TypeString(T, func(p *types.Package) string { return p.Name() }) + " to an interface, method"})
					}
				}
			}
		}
		return out
	}
	type step struct {
		from  *ssa.Function
		label string
	}
	reach := func(roots []*ssa.Function, iface bool) map[*ssa.Function]step {
		from := map[*ssa.Function]step{}
		queue := append([]*ssa.Function(nil), roots...)
		for _, r := range roots {
			from[r] = step{}
		}
		for len(queue) > 0 {
			fn := queue[0]
			queue = queue[1:]
			for _, e := range successors(fn, iface) {
				if _, ok := from[e.to]; !ok {
					from[e.to] = step{fn, e.label}
					queue = append(queue, e.to)
				}
			}
		}
		return from
	}
	path := func(from map[*ssa.Function]step, to *ssa.Function) string {
		var parts []string
		for fn := to; fn != nil; {
			s := from[fn]
			if s.from == nil {
				parts = append([]string{name(fn)}, parts...)
				break
			}
			parts = append([]string{"-[" + s.label + "]-> " + name(fn)}, parts...)
			fn = s.from
		}
		return strings.Join(parts, "\n          ")
	}
	callers := func(fn *ssa.Function) []*ssa.Function {
		var out []*ssa.Function
		if node := graph.Nodes[fn]; node != nil {
			for _, e := range node.In {
				if inModule(e.Caller.Func) {
					out = append(out, e.Caller.Func)
				}
			}
		}
		return out
	}

	// passed returns the function values handed to fn at its module call
	// sites. A caller that only forwards its own parameter is followed up.
	var passed func(fn *ssa.Function, depth int) []*ssa.Function
	passed = func(fn *ssa.Function, depth int) []*ssa.Function {
		var out []*ssa.Function
		node := graph.Nodes[fn]
		if node == nil || depth > 4 {
			return out
		}
		for _, e := range node.In {
			if e.Site == nil || !inModule(e.Caller.Func) {
				continue
			}
			forwarded := false
			for _, arg := range e.Site.Common().Args {
				switch v := arg.(type) {
				case *ssa.MakeClosure:
					out = append(out, v.Fn.(*ssa.Function))
				case *ssa.Function:
					out = append(out, v)
				case *ssa.Parameter:
					if _, ok := v.Type().Underlying().(*types.Signature); ok {
						forwarded = true
					}
				}
			}
			if forwarded {
				out = append(out, passed(e.Caller.Func, depth+1)...)
			}
		}
		return out
	}

	type opener struct {
		fn *ssa.Function
		ms []string
		k  string
	}
	var openers []opener
	for fn := range all {
		if !inModule(fn) {
			continue
		}
		if ms := markers(fn); len(ms) > 0 {
			openers = append(openers, opener{fn, ms, kind(ms)})
		}
	}
	sort.Slice(openers, func(i, j int) bool {
		if openers[i].k != openers[j].k {
			return openers[i].k > openers[j].k
		}
		return where(prog, openers[i].fn) < where(prog, openers[j].fn)
	})

	targets := append(append([]string{}, seams...), writers...)
	fmt.Println("\n== forward: every module function that names an isolation level")
	for _, o := range openers {
		roots := []*ssa.Function{o.fn}
		note := ""
		// A helper that hands its transaction back: what the caller does
		// with it runs at the same level.
		for frontier := []*ssa.Function{o.fn}; len(frontier) > 0; {
			var next []*ssa.Function
			for _, fn := range frontier {
				if !returnsTx(fn) {
					continue
				}
				for _, c := range callers(fn) {
					known := false
					for _, r := range roots {
						known = known || r == c
					}
					if !known {
						roots = append(roots, c)
						next = append(next, c)
						note += "\n      returns its transaction to " + name(c)
					}
				}
			}
			frontier = next
		}
		handed := map[*ssa.Function]bool{}
		for _, f := range passed(o.fn, 0) {
			if inModule(f) && !handed[f] {
				handed[f] = true
				roots = append(roots, f)
			}
		}
		if len(handed) > 0 {
			var names []string
			for f := range handed {
				names = append(names, name(f)+" ["+where(prog, f)+"]")
			}
			sort.Strings(names)
			note += fmt.Sprintf("\n      is handed %d function values at its call sites", len(handed))
			if o.k == "RR-WRITE" {
				note += ":\n        " + strings.Join(names, "\n        ")
			}
		}
		fmt.Printf("\n  %-12s %s  [%s]\n      names %v%s\n", o.k, name(o.fn), where(prog, o.fn), o.ms, note)
		for _, iface := range []bool{false, true} {
			mode := "calls + closures          "
			if iface {
				mode = "calls + closures + iface  "
			}
			from := reach(roots, iface)
			var hits []string
			for _, target := range targets {
				if _, ok := from[find(target)]; ok {
					hits = append(hits, "REACHES "+target+"\n          "+path(from, find(target)))
				}
			}
			verdict := "no seam, no issue writer"
			if len(hits) > 0 {
				verdict = fmt.Sprintf("%d of the %d targets", len(hits), len(targets))
			}
			fmt.Printf("      %s reaches %5d module functions: %s\n", mode, len(from), verdict)
			for _, h := range hits {
				if o.k == "RC-EXPLICIT" || iface {
					h = strings.SplitN(h, "\n", 2)[0]
				}
				fmt.Println("        " + h)
			}
		}
	}

	fmt.Println("\n== backward (calls + closures): functions that begin a transaction or name a level, and can reach a seam")
	reverse := map[*ssa.Function][]*ssa.Function{}
	for fn := range all {
		if !inModule(fn) {
			continue
		}
		for _, e := range successors(fn, false) {
			reverse[e.to] = append(reverse[e.to], fn)
		}
	}
	for _, target := range seams {
		seen := map[*ssa.Function]bool{find(target): true}
		queue := []*ssa.Function{find(target)}
		for len(queue) > 0 {
			fn := queue[0]
			queue = queue[1:]
			for _, c := range reverse[fn] {
				if !seen[c] {
					seen[c] = true
					queue = append(queue, c)
				}
			}
		}
		var lines []string
		for fn := range seen {
			via := beginsVia(fn, 0)
			if via == "" && len(markers(fn)) == 0 {
				continue
			}
			flags := []string{}
			if via != "" {
				flags = append(flags, via)
			}
			if ms := markers(fn); len(ms) > 0 {
				flags = append(flags, fmt.Sprintf("%s %v", kind(ms), ms))
			}
			lines = append(lines, fmt.Sprintf("  %-88s [%s] %s", name(fn), where(prog, fn), strings.Join(flags, " ")))
		}
		sort.Strings(lines)
		var direct []string
		if node := graph.Nodes[find(target)]; node != nil {
			for _, e := range node.In {
				if !inModule(e.Caller.Func) {
					direct = append(direct, "  NOT MODULE CODE: "+e.Caller.Func.String())
					continue
				}
				at := "-"
				if e.Site != nil {
					p := prog.Fset.Position(e.Site.Pos())
					if i := strings.Index(p.Filename, "/server/"); i >= 0 {
						at = fmt.Sprintf("%s:%d", p.Filename[i+len("/server/"):], p.Line)
					}
				}
				direct = append(direct, fmt.Sprintf("  called at %-52s in %s", at, name(e.Caller.Func)))
			}
		}
		for _, c := range reverse[find(target)] {
			named := true
			if node := graph.Nodes[find(target)]; node != nil {
				for _, e := range node.In {
					named = named && e.Caller.Func != c
				}
			}
			if named {
				direct = append(direct, "  named (not called) in "+name(c))
			}
		}
		sort.Strings(direct)
		fmt.Printf("\n%s: %d module functions can reach it; of those:\n%s\n direct references:\n%s\n", target, len(seen), strings.Join(lines, "\n"), strings.Join(direct, "\n"))
	}
}
