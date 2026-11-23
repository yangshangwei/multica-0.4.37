package budget

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/bits"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The tests in this file look at how a reservation remembers its lifecycle
// facts: the set of reasons it keeps beside the unknown items. What a hold
// entry gets, told without that set, is in hold_test.go.

// declaredConstants reads the string constants of one type from the source of
// the package, which is the directory a test runs in. It finds the constants
// declared with their type, which is how model.go declares every reason.
func declaredConstants(t *testing.T, typeName string) []string {
	t.Helper()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var values []string
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
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				constant := spec.(*ast.ValueSpec)
				if id, ok := constant.Type.(*ast.Ident); !ok || id.Name != typeName {
					continue
				}
				for _, v := range constant.Values {
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("%s: a %s constant is not a string literal", fset.Position(v.Pos()), typeName)
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
					}
					values = append(values, value)
				}
			}
		}
	}
	if len(values) == 0 {
		t.Fatalf("found no %s constant in the .go files of the working directory", typeName)
	}
	sort.Strings(values)
	return values
}

// Every hold reason the package declares has one bit of its own, so the set
// of a reservation tells its reasons apart. A reason added without a bit
// would never be on record: each of its holds would be taken as a new fact.
func TestEveryHoldReasonHasItsOwnBit(t *testing.T) {
	covered := make([]string, len(allHoldReasons))
	for i, reason := range allHoldReasons {
		covered[i] = string(reason)
	}
	sort.Strings(covered)
	if declared := declaredConstants(t, "HoldReason"); !reflect.DeepEqual(declared, covered) {
		t.Fatalf("the package declares the hold reasons %q, the tests cover %q", declared, covered)
	}

	owner := map[holdSet]HoldReason{}
	for _, reason := range allHoldReasons {
		bit := holdBit(reason)
		if bits.OnesCount8(uint8(bit)) != 1 {
			t.Errorf("%s has the bits %08b, want one bit", reason, bit)
		}
		if other, taken := owner[bit]; taken {
			t.Errorf("%s and %s share the bits %08b", reason, other, bit)
		}
		owner[bit] = reason
		// The three lists of reasons in ledger.go name the same five.
		if err := validateEntry(holdEntry("hold-1", "r1", reason)); err != nil {
			t.Errorf("%s has a bit and is refused as a hold reason: %v", reason, err)
		}
		if !isLifecycleReason(UnknownReason(reason)) {
			t.Errorf("%s has a bit and is not a lifecycle reason", reason)
		}
	}
	for _, other := range []HoldReason{"", "held", "Timeout", "timeout ", "settled"} {
		if bit := holdBit(other); bit != 0 {
			t.Errorf("%q is not a hold reason and has the bits %08b", other, bit)
		}
	}

	// The set stands for a search of the unknown list, so only a hold entry
	// may leave a lifecycle reason there. Every other item carries one of the
	// declared unknown reasons, and none of those is a hold reason.
	for _, reason := range declaredConstants(t, "UnknownReason") {
		if holdBit(HoldReason(reason)) != 0 || isLifecycleReason(UnknownReason(reason)) {
			t.Errorf("the unknown reason %q is a hold reason as well", reason)
		}
	}
}

// Over random histories that move forward by apply and by With in turn, the
// set of every reservation is what its accepted hold entries recorded, no
// more and no less. Until the reservation settles that is also what the
// unknown list shows for it, each reason once, and the reservation is held
// exactly when the set is not empty.
func TestHoldSetIsWhatTheAcceptedHoldsRecorded(t *testing.T) {
	// How often a set that says something was checked: held, the set of a
	// reservation that has not settled; several, one with more than one
	// reason; kept, the set a settled reservation recorded before.
	var seen struct{ held, several, kept int }
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		l := NewLedger()
		recorded := map[string]holdSet{}
		for step, accepted := 0, 0; step < 80; step++ {
			e := randomEntryWithHolds(rng, accepted)
			context := fmt.Sprintf("seed %d step %d entry %+v", seed, step, e)
			var (
				rs  []Receipt
				err error
			)
			if rng.Intn(2) == 0 {
				rs, err = l.apply(e)
			} else if next, receipts, withErr := l.With(e); withErr == nil {
				l, rs = next, receipts
			} else {
				err = withErr
			}
			if err != nil || rs[0].Replayed {
				continue
			}
			accepted++
			if e.Kind == KindHold && rs[0].Disposition == DispositionAccepted {
				recorded[e.ReservationID] |= holdBit(e.HoldReason)
			}

			listed, items := map[string]holdSet{}, map[string]int{}
			for _, item := range l.unknown {
				if isLifecycleReason(item.Reason) {
					listed[item.ReservationID] |= holdBit(HoldReason(item.Reason))
					items[item.ReservationID]++
				}
			}
			for id, r := range l.reservations {
				if r.holds != recorded[id] {
					t.Fatalf("%s: %s has the set %08b, its accepted holds recorded %08b", context, id, r.holds, recorded[id])
				}
				if r.state == reservationSettled {
					if items[id] != 0 {
						t.Fatalf("%s: the settled %s still has %d lifecycle items", context, id, items[id])
					}
					if r.holds != 0 {
						seen.kept++
					}
					continue
				}
				if listed[id] != r.holds || items[id] != bits.OnesCount8(uint8(r.holds)) {
					t.Fatalf("%s: %s has the set %08b, the unknown list has %08b in %d items", context, id, r.holds, listed[id], items[id])
				}
				if (r.state == reservationHeld) != (r.holds != 0) {
					t.Fatalf("%s: %s is in state %d with the set %08b", context, id, r.state, r.holds)
				}
				if r.holds != 0 {
					seen.held++
				}
				if bits.OnesCount8(uint8(r.holds)) > 1 {
					seen.several++
				}
			}
		}
	}
	// The generator is seeded, so these counts are the same on every run: the
	// set of a reservation that has not settled was checked 12349 times with
	// a reason in it and 8449 times with more than one, and a settled
	// reservation kept what it had recorded 3499 times.
	t.Logf("checked: %+v", seen)
	if seen.held < 7000 || seen.several < 5000 || seen.kept < 2000 {
		t.Fatalf("the generator no longer reaches every case: %+v", seen)
	}
}
