package budget

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The tests in this file hold the package to what the Ledger type promises:
// nothing it exports modifies a Ledger, so one may be shared and read by any
// number of goroutines. What each method returns is checked in the tests of
// that method; here only that it is a read.

// ledgerMethods is every exported method of *Ledger, in the order reflection
// lists them. Each of them only reads its receiver.
//
// When this list grows: the new method must not write to its receiver and
// must not return a part of it, and it needs a read in sharedLedgerReads, so
// that TestSharedLedgerIsOnlyReadThroughItsExportedMethods runs it beside the
// others under -race. A method that has to modify the ledger stays unexported.
var ledgerMethods = []string{"HasReservation", "Len", "Projection", "Reservation", "With"}

// sharedLedgerWant is what a reader of the ledger published() builds must
// see. It is taken from a second ledger, built the same way, that no reader
// touches.
type sharedLedgerWant struct {
	projection Projection
	// derived and receipts are what With(later()...) gives.
	derived  *Ledger
	receipts []Receipt
}

// sharedLedgerReads has a read of a shared ledger through each exported
// method. A read fails on a result other than the one the published state
// gives. It then writes to everything it was handed: a result that was a part
// of the ledger would change what the other readers see, and under -race the
// write is reported.
var sharedLedgerReads = map[string]func(l *Ledger, want sharedLedgerWant) error{
	"Len": func(l *Ledger, _ sharedLedgerWant) error {
		if n := l.Len(); n != 7 {
			return fmt.Errorf("Len = %d, want 7", n)
		}
		return nil
	},
	"HasReservation": func(l *Ledger, _ sharedLedgerWant) error {
		if !l.HasReservation("r1") || !l.HasReservation("r2") || l.HasReservation("r3") || l.HasReservation("") {
			return fmt.Errorf("HasReservation: r1 %v, r2 %v, r3 %v, empty %v; want true, true, false, false",
				l.HasReservation("r1"), l.HasReservation("r2"), l.HasReservation("r3"), l.HasReservation(""))
		}
		return nil
	},
	"Reservation": func(l *Ledger, _ sharedLedgerWant) error {
		r1, ok1 := l.Reservation("r1")
		r2, ok2 := l.Reservation("r2")
		r3, ok3 := l.Reservation("r3")
		if !ok1 || r1 != (ReservationProjection{Status: ReservationHeld, Reserved: 100, ActualKnown: 30, Unspent: 70}) ||
			!ok2 || r2 != (ReservationProjection{Status: ReservationOpen, Unsized: true}) ||
			ok3 || r3 != (ReservationProjection{}) {
			return fmt.Errorf("Reservation: r1 %+v, %v; r2 %+v, %v; r3 %+v, %v", r1, ok1, r2, ok2, r3, ok3)
		}
		// A reading holds no reference (TestReservationReadingHoldsNoReference),
		// so there is nothing of the ledger in it to write to.
		return nil
	},
	"Projection": func(l *Ledger, want sharedLedgerWant) error {
		p := l.Projection()
		if !reflect.DeepEqual(p, want.projection) {
			return fmt.Errorf("Projection = %+v, want %+v", p, want.projection)
		}
		for i := range p.Unknown {
			p.Unknown[i] = UnknownItem{Reason: "overwritten"}
		}
		for i := range p.Coverage.NotApplicable {
			p.Coverage.NotApplicable[i] = "overwritten"
		}
		return nil
	},
	"With": func(l *Ledger, want sharedLedgerWant) error {
		derived, receipts, err := l.With(later()...)
		if err != nil || !reflect.DeepEqual(derived, want.derived) || !reflect.DeepEqual(receipts, want.receipts) {
			return fmt.Errorf("With = %+v, %v; want %+v", receipts, err, want.receipts)
		}
		// The result is the caller's own. This writes to every part of it.
		if _, err := derived.apply(
			cumulative("own-1", "r1", "scope-1", 3, reported(95)),
			delta("own-2", "r3", "scope-3", Cost{}),
			holdEntry("own-3", "r3", HoldTimeout),
			Entry{ID: "own-4", Kind: KindExclusion, Detail: "plugin_external_call"},
		); err != nil {
			return fmt.Errorf("writing to the derived ledger: %v", err)
		}
		for i := range receipts {
			receipts[i] = Receipt{}
		}
		// A refused batch and an empty one read the ledger just the same.
		if failed, rs, err := l.With(later()[0], Entry{ID: "bad", Kind: "refund"}); !errors.Is(err, ErrInvalidEntry) || failed != nil || rs != nil {
			return fmt.Errorf("With of a refused batch = %v, %+v, %v", failed, rs, err)
		}
		if same, rs, err := l.With(); err != nil || len(rs) != 0 || same == l || same.Len() != 7 {
			return fmt.Errorf("With of no entries = %v, %+v, %v", same, rs, err)
		}
		return nil
	},
}

// exportedFuncsTakingALedger reads the source of the package, which is the
// directory a test runs in, and returns the exported functions, and the
// exported methods of other types, that have a Ledger among their parameters.
// Such a function could do to a Ledger what no method of it may.
func exportedFuncsTakingALedger(t *testing.T) []string {
	t.Helper()
	mentionsLedger := func(fields *ast.FieldList) bool {
		found := false
		ast.Inspect(fields, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "Ledger" {
				found = true
			}
			return !found
		})
		return found
	}
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	parsedFuncs := 0
	fset := token.NewFileSet()
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, f.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			parsedFuncs++
			// The methods of Ledger itself are the ones reflection lists.
			if !fn.Name.IsExported() || (fn.Recv != nil && mentionsLedger(fn.Recv)) {
				continue
			}
			if mentionsLedger(fn.Type.Params) {
				found = append(found, fn.Name.Name)
			}
		}
	}
	if parsedFuncs == 0 {
		t.Fatal("found no function in the .go files of the working directory")
	}
	return found
}

// Outside the package a Ledger can only be read. The exported methods are the
// read-only ones and no others, the type has no exported field, and no
// exported function takes a Ledger to work on.
func TestNothingExportedCanModifyALedger(t *testing.T) {
	typ := reflect.TypeOf((*Ledger)(nil))
	var methods []string
	for i := 0; i < typ.NumMethod(); i++ {
		methods = append(methods, typ.Method(i).Name)
	}
	if !reflect.DeepEqual(methods, ledgerMethods) {
		t.Fatalf("*Ledger exports the methods %q, the read-only ones are %q", methods, ledgerMethods)
	}
	for i := 0; i < typ.Elem().NumField(); i++ {
		if field := typ.Elem().Field(i); field.IsExported() {
			t.Errorf("Ledger has the exported field %s", field.Name)
		}
	}
	if found := exportedFuncsTakingALedger(t); len(found) != 0 {
		t.Errorf("the exported functions %q take a Ledger", found)
	}

	// Every method in the list is run on a shared ledger by the test below.
	var covered []string
	for name := range sharedLedgerReads {
		covered = append(covered, name)
	}
	sort.Strings(covered)
	if !reflect.DeepEqual(covered, ledgerMethods) {
		t.Fatalf("the shared ledger is read through %q, the exported methods are %q", covered, ledgerMethods)
	}
}

// One ledger, published once, is read through every exported method by
// several goroutines at a time, With among them. Each sees the published
// state throughout, and the ledger is the published one afterwards. Under
// -race a method that writes to its receiver is reported, even if it puts
// everything back.
func TestSharedLedgerIsOnlyReadThroughItsExportedMethods(t *testing.T) {
	shared, twin := published(t), published(t)
	want := sharedLedgerWant{projection: twin.Projection()}
	wantFigures(t, want.projection, 30, 0, 0, 70, 0)
	wantUnknown(t, want.projection, ReasonReservationUnsized, ReasonCostAbsent, "lost_contact", ReasonCoverageGap)
	var err error
	if want.derived, want.receipts, err = twin.With(later()...); err != nil || len(want.receipts) != 6 {
		t.Fatalf("With on the twin = %+v, %v", want.receipts, err)
	}

	const perMethod, rounds = 3, 100
	var wg sync.WaitGroup
	for name, read := range sharedLedgerReads {
		for g := 0; g < perMethod; g++ {
			wg.Add(1)
			go func(name string, read func(*Ledger, sharedLedgerWant) error) {
				defer wg.Done()
				for i := 0; i < rounds; i++ {
					if err := read(shared, want); err != nil {
						t.Errorf("%s on the shared ledger, round %d: %v", name, i, err)
						return
					}
				}
			}(name, read)
		}
	}
	wg.Wait()
	if !reflect.DeepEqual(shared, published(t)) {
		t.Fatal("the shared ledger is no longer what was published")
	}
}
