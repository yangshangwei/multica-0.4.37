package budget

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
)

// The benchmarks measure what a per-process ledger cache pays: deriving the
// next state from a published ledger, loading a root from its stored entries,
// and the heap a loaded ledger keeps.
//
//	go test ./internal/budget -run '^$' -bench . -benchmem
//	go test ./internal/budget -run '^$' -bench LoadedLedgerMemory -benchtime 1x
//
// A ledger of n entries is made of attempts. Identifiers are 36 bytes, the
// length of a UUID in text form.

var benchSizes = [...]int{1_000, 10_000, 100_000}

// A shape is what the attempts of a ledger look like. All but the last have 20
// entries per attempt.
const (
	// benchCumulative: an unsized reserve, 18 running totals of one meter scope
	// (a provider session) with a reported cost, and a settlement.
	benchCumulative = "cumulative"
	// benchDelta: the same with 18 events, each in a meter scope of its own (a
	// provider request).
	benchDelta = "delta"
	// benchMixed: attempts of the cumulative shape take turns with attempts of
	// an unsized reserve and 19 events whose cost is an estimate. An estimate
	// stays in the unknown set and keeps its reservation from settling, so the
	// unknown list grows with the ledger: half of its entries are in it.
	benchMixed = "mixed"
	// benchHeld: an unsized reserve and a lifecycle fact, two entries per
	// attempt and no settlement. It is what a root holds while nothing reports
	// usage. Every entry leaves an item in the unknown list.
	benchHeld = "held"
)

var benchShapes = [...]string{benchCumulative, benchDelta, benchMixed, benchHeld}

const benchAttemptEntries = 20

// benchPriceDigest has the length of a price snapshot digest and stands for
// none.
const benchPriceDigest = "0000000000000000000000000000000000000000000000000000000000000000"

// The ID spaces keep entry, reservation, scope and invocation identifiers
// apart.
const (
	benchEntrySpace = iota + 1
	benchReservationSpace
	benchScopeSpace
	benchInvocationSpace
)

func benchID(space, attempt, n int) string {
	return fmt.Sprintf("%08x-%04x-4000-8000-%012x", attempt, space, n)
}

func benchReserve(attempt int) Entry {
	return Entry{ID: benchID(benchEntrySpace, attempt, 0), Kind: KindReserve, ReservationID: benchID(benchReservationSpace, attempt, 0), Unsized: true}
}

// benchUsage is the k-th observation of an attempt.
func benchUsage(attempt, k int, estimated, ownScope bool) Usage {
	u := Usage{
		ScopeID: benchID(benchScopeSpace, attempt, 0), InvocationID: benchID(benchInvocationSpace, attempt, 0),
		Mode: ModeCumulative, Sequence: Count(k),
		Tokens: TokenCounts{Input: Count(1_000 * k), Output: Count(100 * k), CacheRead: Count(5_000 * k)},
		Cost:   Cost{Basis: CostReported, Ticks: Ticks(1_000_000 * k), Currency: CurrencyUSD},
	}
	if ownScope {
		u.ScopeID, u.InvocationID = benchID(benchScopeSpace, attempt, k), benchID(benchInvocationSpace, attempt, k)
		u.Mode, u.Sequence = ModeDelta, 0
	}
	if estimated {
		u.Cost.Basis, u.Cost.PriceVersion, u.Cost.PriceDigest = CostEstimated, "bench-prices-1", benchPriceDigest
	}
	return u
}

func benchEntries(shape string, n int) []Entry {
	entries := make([]Entry, 0, n)
	for attempt := 0; len(entries) < n; attempt++ {
		reserve := benchReserve(attempt)
		entries = append(entries, reserve)
		if shape == benchHeld {
			if len(entries) < n {
				entries = append(entries, holdEntry(benchID(benchEntrySpace, attempt, 1), reserve.ReservationID, HoldLostContact))
			}
			continue
		}
		estimated := shape == benchMixed && attempt%2 == 1
		for k := 1; k < benchAttemptEntries && len(entries) < n; k++ {
			e := Entry{ID: benchID(benchEntrySpace, attempt, k), Kind: KindUsage, ReservationID: reserve.ReservationID}
			if k == benchAttemptEntries-1 && !estimated {
				e = settleEntry(e.ID, reserve.ReservationID)
			} else {
				e.Usage = benchUsage(attempt, k, estimated, estimated || shape == benchDelta)
			}
			entries = append(entries, e)
		}
	}
	return entries
}

// benchStored is the stored form of entries, as the ledger table holds it.
func benchStored(b *testing.B, entries []Entry) [][]byte {
	b.Helper()
	stored := make([][]byte, len(entries))
	for i, e := range entries {
		raw, _, err := EncodeEntry(e)
		if err != nil {
			b.Fatal(err)
		}
		stored[i] = raw
	}
	return stored
}

// benchLoad is a load with nothing cached: decode every stored entry, which
// is where each is held to its stored form, and replay them into an empty
// ledger.
func benchLoad(b *testing.B, stored [][]byte) *Ledger {
	b.Helper()
	entries := make([]Entry, len(stored))
	for i, raw := range stored {
		e, err := DecodeEntry(EntryFormVersion, raw)
		if err != nil {
			b.Fatal(err)
		}
		entries[i] = e
	}
	return benchReplay(b, entries)
}

func benchReplay(b *testing.B, entries []Entry) *Ledger {
	b.Helper()
	l, receipts, err := NewLedger().With(entries...)
	if err != nil {
		b.Fatal(err)
	}
	if l.Len() != len(entries) {
		b.Fatalf("ledger holds %d of %d entries", l.Len(), len(entries))
	}
	for _, r := range receipts {
		if r.Disposition != DispositionAccepted {
			b.Fatalf("receipt %+v", r)
		}
	}
	return l
}

func benchEachLedger(b *testing.B, run func(b *testing.B, entries []Entry)) {
	for _, shape := range benchShapes {
		for _, n := range benchSizes {
			b.Run(fmt.Sprintf("%s/entries=%d", shape, n), func(b *testing.B) { run(b, benchEntries(shape, n)) })
		}
	}
}

// BenchmarkWith is one write to a cached root: the published ledger of n
// entries and the reserve entry of one more attempt.
func BenchmarkWith(b *testing.B) {
	benchEachLedger(b, func(b *testing.B, entries []Entry) {
		base, one := benchReplay(b, entries), benchReserve(len(entries))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, err := base.With(one); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkProjection is what each write pays again to refresh the figures
// kept on the root.
func BenchmarkProjection(b *testing.B) {
	benchEachLedger(b, func(b *testing.B, entries []Entry) {
		base := benchReplay(b, entries)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			base.Projection()
		}
	})
}

// BenchmarkReplay is the replay of n decoded entries into an empty ledger.
func BenchmarkReplay(b *testing.B) {
	benchEachLedger(b, func(b *testing.B, entries []Entry) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchReplay(b, entries)
		}
	})
}

// BenchmarkLoad is BenchmarkReplay with the decoding of the n stored forms.
func BenchmarkLoad(b *testing.B) {
	benchEachLedger(b, func(b *testing.B, entries []Entry) {
		stored := benchStored(b, entries)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchLoad(b, stored)
		}
	})
}

// benchKinds has one entry of each kind the first package writes.
func benchKinds() map[string]Entry {
	reserve := benchReserve(0)
	return map[string]Entry{
		"reserve": reserve,
		"hold":    holdEntry(benchID(benchEntrySpace, 0, 1), reserve.ReservationID, HoldLostContact),
		"usage":   {ID: benchID(benchEntrySpace, 0, 2), Kind: KindUsage, ReservationID: reserve.ReservationID, Usage: benchUsage(0, 2, false, true)},
	}
}

// BenchmarkDecodeEntry is the decoding of one stored entry. "stored" is
// DecodeEntry, which holds the bytes to the stored form as well; "plain" is
// encoding/json alone, which is all a load did before DecodeEntry.
func BenchmarkDecodeEntry(b *testing.B) {
	for name, e := range benchKinds() {
		raw, _, err := EncodeEntry(e)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name+"/stored", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := DecodeEntry(EntryFormVersion, raw); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(name+"/plain", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var plain Entry
				if err := json.Unmarshal(raw, &plain); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEncodeEntry is what a write pays for each new entry: the check of
// the entry, its stored form and the digest of it.
func BenchmarkEncodeEntry(b *testing.B) {
	for name, e := range benchKinds() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, _, err := EncodeEntry(e); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// liveHeap is the size of the reachable heap.
func liveHeap() uint64 {
	var stats runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

// BenchmarkLoadedLedgerMemory logs the heap a ledger keeps after a load from
// storage, in bytes per entry: the ledger and the strings of its entries. The
// decoded entry list is garbage by then. The figure is in the log line, not
// in ns/op.
func BenchmarkLoadedLedgerMemory(b *testing.B) {
	benchEachLedger(b, func(b *testing.B, entries []Entry) {
		stored := benchStored(b, entries)
		// What the decoder sets up once is not part of a ledger.
		benchLoad(b, stored[:benchAttemptEntries])
		var kept uint64
		for i := 0; i < b.N; i++ {
			before := liveHeap()
			l := benchLoad(b, stored)
			after := liveHeap()
			runtime.KeepAlive(l)
			kept = 0
			if after > before {
				kept = after - before
			}
		}
		b.Logf("%d entries keep %d bytes, %d bytes per entry", len(stored), kept, kept/uint64(len(stored)))
	})
}
