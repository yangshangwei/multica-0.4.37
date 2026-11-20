package budget

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// The tests in this file are about the stored form of an entry: the bytes
// EncodeEntry gives and DecodeEntry takes back. The form of one usage entry
// is pinned byte for byte in ledger_test.go (pinnedStoredForm) and that of an
// unsized reserve in unsized_test.go.

// EntryFormVersion is untyped: it goes into the integer column of a row and
// into DecodeEntry alike.
var (
	_ int32 = EntryFormVersion
	_ int   = EntryFormVersion
)

// step2StoredForm is the stored form the pinned entry had after step 2,
// before the unsized key. It is the one older form that was ever written, and
// only into test databases.
const step2StoredForm = `{"id":"e1","kind":"usage","reservation_id":"r1","reserved_ticks":"0","hold_reason":"","proof":{"final_usage":false,"scope_complete":false,"no_further_execution":false},"gap_reason":"","detail":"","usage":{"scope_id":"scope-1","invocation_id":"inv-scope-1","mode":"cumulative","sequence":"2","overlap":"","baseline_event_id":"","supersedes":"","correction_authorized":false,"tokens":{"input":"1000001","output":"7","cache_read":"0","cache_write":"0"},"cost":{"basis":"reported","ticks":"30","currency":"USD","zero_basis":"","price_version":"","price_digest":""}}}`

// replayErrors are the errors a ledger gives for an entry. A row that cannot
// be decoded must not look like any of them.
var replayErrors = []error{
	ErrInvalidEntry, ErrNegativeAmount, ErrConflict, ErrUnknownReservation,
	ErrModeMismatch, ErrOverflow, ErrReleaseProofIncomplete,
}

// encoded is EncodeEntry for an entry that has a stored form.
func encoded(t *testing.T, e Entry) ([]byte, string) {
	t.Helper()
	raw, digest, err := EncodeEntry(e)
	if err != nil {
		t.Fatalf("EncodeEntry(%+v): %v", e, err)
	}
	return raw, digest
}

// notStoredForm reports why DecodeEntry's answer is not a refusal of the
// bytes as such: no entry, and the one error that says so.
func notStoredForm(got Entry, err error) error {
	if !errors.Is(err, ErrInvalidStoredForm) || got != (Entry{}) {
		return fmt.Errorf("DecodeEntry = %+v, %v; want no entry and ErrInvalidStoredForm", got, err)
	}
	for _, other := range replayErrors {
		if errors.Is(err, other) {
			return fmt.Errorf("the error %q is also %q", err, other)
		}
	}
	return nil
}

// reformed is the pinned stored form with its one occurrence of old replaced.
func reformed(t *testing.T, old, new string) string {
	t.Helper()
	if n := strings.Count(pinnedStoredForm, old); n != 1 {
		t.Fatalf("the pinned stored form has %d occurrences of %s", n, old)
	}
	return strings.Replace(pinnedStoredForm, old, new, 1)
}

// outsideStrings returns the offsets of a JSON text that are not inside a
// string literal, the end included. A space put at one of them stands between
// two tokens or splits a literal such as false: it never changes the value of
// a string.
func outsideStrings(raw []byte) []int {
	var at []int
	inString := false
	for i := 0; i < len(raw); i++ {
		if !inString {
			at = append(at, i)
		}
		switch {
		case inString && raw[i] == '\\':
			i++
		case raw[i] == '"':
			inString = !inString
		}
	}
	return append(at, len(raw))
}

// inserted is raw with one byte put in at offset i.
func inserted(raw []byte, i int, b byte) []byte {
	out := make([]byte, 0, len(raw)+1)
	out = append(out, raw[:i]...)
	out = append(out, b)
	return append(out, raw[i:]...)
}

// awkwardText draws a short text, valid UTF-8, of what JSON writes in a way
// of its own: a quote, a backslash and text that reads like an escape, the
// characters encoding/json escapes for HTML, control characters, the two
// Unicode line separators, and runes of two, three and four bytes, the
// replacement character among them.
func awkwardText(rng *rand.Rand) string {
	pieces := [...]string{
		"a", "Z", "0", " ", "'", "/", `"`, `\`, `\u0041`, "<", ">", "&",
		"\n", "\t", "\r", "\x00", "\x1f", "\x7f", "\u2028", "\u2029",
		"é", "预", "🙂", "\ufffd",
	}
	var b strings.Builder
	for n := 1 + rng.Intn(8); n > 0; n-- {
		b.WriteString(pieces[rng.Intn(len(pieces))])
	}
	return b.String()
}

// withAwkwardText puts such text into the fields that take any text for the
// kind of the entry. It leaves the reservation alone.
func withAwkwardText(rng *rand.Rand, e Entry) Entry {
	e.ID += awkwardText(rng)
	switch e.Kind {
	case KindHold, KindGap, KindExclusion:
		e.Detail += awkwardText(rng)
	case KindUsage:
		e.Usage.ScopeID += awkwardText(rng)
		e.Usage.InvocationID += awkwardText(rng)
		if e.Usage.Supersedes != "" {
			e.Usage.Supersedes += awkwardText(rng)
		}
	}
	return e
}

// Over the entries of the random generators, valid and not: EncodeEntry
// refuses exactly the entries a ledger refuses on sight, with the ledger's
// error. Every other entry has one stored form. It is the JSON encoding the
// form has been since step 2, its digest is the one Digest gives, DecodeEntry
// gives the entry back, and the bytes with anything put in, cut off or read
// as another version are not a stored form.
func TestStoredFormRoundTripsForEveryEntryALedgerCouldHold(t *testing.T) {
	generators := [...]func(*rand.Rand, int) Entry{randomEntry, randomEntryWithUnsized, randomEntryWithHolds}
	// How many entries were encoded, by kind, how many of those carried the
	// awkward text, and how many were refused.
	var seen struct {
		kinds            map[EntryKind]int
		awkward, refused int
	}
	seen.kinds = map[EntryKind]int{}
	// The digest of every stored form in a row, to hold one toolchain's forms
	// against another's.
	forms := sha256.New()
	empty := NewLedger()
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		for step := 0; step < 90; step++ {
			e := generators[step%len(generators)](rng, step)
			awkward := rng.Intn(3) == 0
			if awkward {
				e = withAwkwardText(rng, e)
			}
			context := fmt.Sprintf("seed %d step %d entry %+v", seed, step, e)

			// What a ledger refuses whatever it holds, it refuses when it is empty.
			_, _, ledgerErr := empty.With(e)
			raw, digest, err := EncodeEntry(e)
			if errors.Is(ledgerErr, ErrInvalidEntry) || errors.Is(ledgerErr, ErrNegativeAmount) {
				if err == nil || err.Error() != ledgerErr.Error() || errors.Is(err, ErrInvalidEntry) != errors.Is(ledgerErr, ErrInvalidEntry) || raw != nil || digest != "" {
					t.Fatalf("%s: EncodeEntry = %s, %q, %v; the ledger says %v", context, raw, digest, err, ledgerErr)
				}
				seen.refused++
				continue
			}
			if err != nil {
				t.Fatalf("%s: EncodeEntry: %v; the ledger says %v", context, err, ledgerErr)
			}
			seen.kinds[e.Kind]++
			if awkward {
				seen.awkward++
			}
			forms.Write(raw)
			forms.Write([]byte{'\n'})

			plain, err := json.Marshal(e)
			if err != nil || !bytes.Equal(raw, plain) {
				t.Fatalf("%s: stored form %s, JSON encoding %s, %v", context, raw, plain, err)
			}
			sum := sha256.Sum256(append([]byte("budget-entry-v1\n"), raw...))
			if own, err := e.Digest(); err != nil || digest != own || digest != hex.EncodeToString(sum[:]) {
				t.Fatalf("%s: digest %q, Digest() %q, %v, SHA-256 of the stored form %x", context, digest, own, err, sum)
			}
			back, err := DecodeEntry(EntryFormVersion, raw)
			if err != nil || back != e {
				t.Fatalf("%s: DecodeEntry = %+v, %v", context, back, err)
			}
			if again, againDigest, err := EncodeEntry(back); err != nil || !bytes.Equal(again, raw) || againDigest != digest {
				t.Fatalf("%s: the decoded entry encodes to %s, %q, %v", context, again, againDigest, err)
			}

			version := []int{0, EntryFormVersion + 1, -EntryFormVersion}[rng.Intn(3)]
			if err := notStoredForm(DecodeEntry(version, raw)); err != nil {
				t.Fatalf("%s: as version %d: %v", context, version, err)
			}
			at := outsideStrings(raw)
			spaced := inserted(raw, at[rng.Intn(len(at))], " \n\t\r"[rng.Intn(4)])
			if err := notStoredForm(DecodeEntry(EntryFormVersion, spaced)); err != nil {
				t.Fatalf("%s: with white space as in %q: %v", context, spaced, err)
			}
			if err := notStoredForm(DecodeEntry(EntryFormVersion, raw[:rng.Intn(len(raw))])); err != nil {
				t.Fatalf("%s: cut short: %v", context, err)
			}
		}
	}
	// The generators are seeded, so these counts are the same on every run.
	// The floors fail the test if a change to a generator stops a case from
	// being reached.
	t.Logf("encoded: %v, %d of them with awkward text; refused: %d; SHA-256 of the stored forms: %x", seen.kinds, seen.awkward, seen.refused, forms.Sum(nil))
	for _, kind := range []EntryKind{KindReserve, KindUsage, KindHold, KindSettle, KindGap, KindExclusion} {
		if seen.kinds[kind] < 1000 {
			t.Fatalf("only %d %s entries were encoded: %v", seen.kinds[kind], kind, seen.kinds)
		}
	}
	if seen.awkward < 5000 || seen.refused < 1500 {
		t.Fatalf("the generators no longer reach every case: %d with awkward text, %d refused", seen.awkward, seen.refused)
	}
}

// DecodeEntry takes one byte string for an entry and no other. Each case here
// is a way of writing the pinned entry, or something near it, that a JSON
// decoder may well read; lenient says that encoding/json reads it as exactly
// the pinned entry.
func TestDecodeEntryRefusesWhatIsNotTheStoredForm(t *testing.T) {
	want := pinnedEntry()
	if got, err := DecodeEntry(EntryFormVersion, []byte(pinnedStoredForm)); err != nil || got != want {
		t.Fatalf("the stored form itself: DecodeEntry = %+v, %v", got, err)
	}
	if step2 := reformed(t, `"unsized":false,`, ""); step2 != step2StoredForm {
		t.Fatalf("the step 2 form is not the pinned form without the unsized key:\n%s\n%s", step2StoredForm, step2)
	}

	for _, tc := range []struct {
		name, text string
		lenient    bool
	}{
		{"an extra key", reformed(t, `,"usage":{`, `,"note":"x","usage":{`), true},
		{"an extra key inside the usage", reformed(t, `"cost":{`, `"note":"x","cost":{`), true},
		{"an extra key at the end", reformed(t, `}}}`, `}},"note":"x"}`), true},
		{"a missing key", reformed(t, `"detail":"",`, ""), true},
		{"a missing key inside the usage", reformed(t, `"overlap":"",`, ""), true},
		{"the step 2 form, without the unsized key", step2StoredForm, true},
		{"two keys swapped", reformed(t, `"gap_reason":"","detail":""`, `"detail":"","gap_reason":""`), true},
		{"two keys of the token counts swapped", reformed(t, `"input":"1000001","output":"7"`, `"output":"7","input":"1000001"`), true},
		{"trailing data", pinnedStoredForm + "{}", false},
		{"a second entry behind the first", pinnedStoredForm + pinnedStoredForm, false},
		{"a trailing line feed", pinnedStoredForm + "\n", true},
		{"a trailing space", pinnedStoredForm + " ", true},
		{"a leading space", " " + pinnedStoredForm, true},
		{"a space after a colon", reformed(t, `{"id":"e1"`, `{"id": "e1"`), true},
		{"a duplicate key", reformed(t, `"detail":"",`, `"detail":"","detail":"",`), true},
		{"a duplicate key with another value in front", reformed(t, `{"id":"e1",`, `{"id":"e0","id":"e1",`), true},
		{"a key in another letter case", reformed(t, `"detail":`, `"Detail":`), true},
		{"a key in capitals", reformed(t, `{"id":`, `{"ID":`), true},
		{"an escape for a plain letter", reformed(t, `{"id":"e1"`, `{"id":"\u00651"`), true},
		{"null for a text", reformed(t, `"detail":""`, `"detail":null`), true},
		{"null for a flag", reformed(t, `"unsized":false`, `"unsized":null`), true},
		{"a number for an amount", reformed(t, `"reserved_ticks":"0"`, `"reserved_ticks":0`), false},
		{"an amount beyond the 64-bit range", reformed(t, `"reserved_ticks":"0"`, `"reserved_ticks":"9223372036854775808"`), false},
		{"an amount with a sign", reformed(t, `"ticks":"30"`, `"ticks":"-30"`), false},
		{"an amount with a zero in front", reformed(t, `"ticks":"30"`, `"ticks":"030"`), false},
		{"a count of no digits", reformed(t, `"sequence":"2"`, `"sequence":""`), false},
		{"a byte that is not UTF-8 in a text", reformed(t, `{"id":"e1"`, "{\"id\":\"e\xff\""), false},
		{"a byte order mark in front", "\xef\xbb\xbf" + pinnedStoredForm, false},
		{"an array of one entry", "[" + pinnedStoredForm + "]", false},
		{"an empty object", "{}", false},
		{"null", "null", false},
		{"nothing", "", false},
	} {
		var plain Entry
		if lenient := json.Unmarshal([]byte(tc.text), &plain) == nil && plain == want; lenient != tc.lenient {
			t.Errorf("%s: encoding/json alone reads it as the pinned entry: %v, want %v", tc.name, lenient, tc.lenient)
		}
		if err := notStoredForm(DecodeEntry(EntryFormVersion, []byte(tc.text))); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// Every cut of a stored form, and a stored form with white space anywhere
// outside its strings, is refused. A byte taken out is refused too, unless
// what is left is the stored form of another entry: no two byte strings stand
// for one entry.
func TestDecodeEntryRefusesEveryCutAndEveryInsertedSpace(t *testing.T) {
	hold := holdEntry("hold-1", "r1", HoldLostContact)
	hold.Detail = `say "<when>" & why \ so`
	var cuts, spaces, removals, others int
	for _, e := range []Entry{pinnedEntry(), unsizedReserve("reserve-1", "r1"), hold} {
		raw, _ := encoded(t, e)
		for i := 0; i < len(raw); i++ {
			if err := notStoredForm(DecodeEntry(EntryFormVersion, raw[:i])); err != nil {
				t.Fatalf("%s cut to %d bytes: %v", e.ID, i, err)
			}
			cuts++
		}
		for _, i := range outsideStrings(raw) {
			for _, space := range []byte(" \n\t\r") {
				if err := notStoredForm(DecodeEntry(EntryFormVersion, inserted(raw, i, space))); err != nil {
					t.Fatalf("%s with %q at offset %d: %v", e.ID, space, i, err)
				}
				spaces++
			}
		}
		for i := range raw {
			less := append(append([]byte{}, raw[:i]...), raw[i+1:]...)
			got, err := DecodeEntry(EntryFormVersion, less)
			if err != nil {
				if err := notStoredForm(got, err); err != nil {
					t.Fatalf("%s without the byte at offset %d: %v", e.ID, i, err)
				}
				removals++
				continue
			}
			// What is left is a form of its own, of another entry, and the
			// digest of these bytes is the digest of that entry.
			if again, _ := json.Marshal(got); got == e || !bytes.Equal(again, less) {
				t.Fatalf("%s without the byte at offset %d is read as %+v", e.ID, i, got)
			}
			sum := sha256.Sum256(append([]byte("budget-entry-v1\n"), less...))
			if own, err := got.Digest(); err != nil || own != hex.EncodeToString(sum[:]) {
				t.Fatalf("%s without the byte at offset %d: Digest() = %q, %v; the bytes have %x", e.ID, i, own, err, sum)
			}
			others++
		}
	}
	t.Logf("refused %d cuts, %d inserted spaces and %d removed bytes; %d removed bytes left the form of another entry", cuts, spaces, removals, others)
	if cuts < 1700 || spaces < 1800 || removals < 1500 || others < 100 {
		t.Fatalf("the fixtures no longer reach every case: %d cuts, %d spaces, %d removals, %d other entries", cuts, spaces, removals, others)
	}
}

// A row carries the version of its stored form. DecodeEntry reads the current
// one and refuses every other, whatever the bytes are. The digest domain names
// the same version: a new form needs both changed.
func TestDecodeEntryReadsOnlyTheCurrentFormVersion(t *testing.T) {
	if EntryFormVersion != 1 {
		t.Fatalf("EntryFormVersion = %d; the first committed form is version 1", EntryFormVersion)
	}
	if want := fmt.Sprintf("budget-entry-v%d\n", EntryFormVersion); entryDigestDomain != want {
		t.Fatalf("the digest domain is %q, form version %d wants %q", entryDigestDomain, EntryFormVersion, want)
	}
	raw := []byte(pinnedStoredForm)
	if got, err := DecodeEntry(EntryFormVersion, raw); err != nil || got != pinnedEntry() {
		t.Fatalf("version %d: DecodeEntry = %+v, %v", EntryFormVersion, got, err)
	}
	for _, version := range []int{0, 2, 3, 10, -1, EntryFormVersion - 1, EntryFormVersion + 1, math.MaxInt32, math.MinInt32} {
		if err := notStoredForm(DecodeEntry(version, raw)); err != nil {
			t.Errorf("version %d: %v", version, err)
		}
	}
	// A version that is not the current one is refused before the bytes are
	// looked at, so the two refusals do not depend on each other.
	if err := notStoredForm(DecodeEntry(EntryFormVersion+1, []byte(step2StoredForm))); err != nil {
		t.Errorf("another version and another form: %v", err)
	}
}

// EncodeEntry judges an entry the way a ledger judges it on sight, before any
// state: an entry no ledger would ever hold gets no stored form and no
// digest. What depends on the ledger is not asked.
func TestEncodeEntryRefusesWhatALedgerRefusesOnSight(t *testing.T) {
	foreign := reserveEntry("reserve-1", "r1", 10)
	foreign.Usage.ScopeID = "scope-1"
	unknownReason := holdEntry("hold-1", "r1", "bored")
	sizedAndUnsized := reserveEntry("reserve-1", "r1", 10)
	sizedAndUnsized.Unsized = true
	negativeTokens := delta("d1", "r1", "scope-1", reported(1))
	negativeTokens.Usage.Tokens.Output = -1

	for name, tc := range map[string]struct {
		entry Entry
		want  error
	}{
		"no ID":                         {reserveEntry("", "r1", 10), ErrInvalidEntry},
		"an unknown kind":               {Entry{ID: "x", Kind: "refund"}, ErrInvalidEntry},
		"no kind":                       {Entry{ID: "x"}, ErrInvalidEntry},
		"a reserve of no reservation":   {reserveEntry("reserve-1", "", 10), ErrInvalidEntry},
		"an unsized reserve of 10":      {sizedAndUnsized, ErrInvalidEntry},
		"a reserve with usage fields":   {foreign, ErrInvalidEntry},
		"a hold for an unknown reason":  {unknownReason, ErrInvalidEntry},
		"a gap with a lifecycle reason": {Entry{ID: "gap-1", Kind: KindGap, GapReason: "timeout"}, ErrInvalidEntry},
		"an exclusion without a name":   {Entry{ID: "x-1", Kind: KindExclusion}, ErrInvalidEntry},
		"usage without a scope":         {delta("d1", "r1", "", reported(1)), ErrInvalidEntry},
		"usage without a mode":          {Entry{ID: "d1", Kind: KindUsage, Usage: Usage{ScopeID: "s", InvocationID: "i"}}, ErrInvalidEntry},
		"an ID that is not UTF-8":       {reserveEntry("reserve-\xff", "r1", 10), ErrInvalidEntry},
		"a detail that is not UTF-8":    {Entry{ID: "gap-1", Kind: KindGap, GapReason: ReasonCoverageGap, Detail: "a\xfe"}, ErrInvalidEntry},
		"a negative reservation":        {reserveEntry("reserve-1", "r1", -1), ErrNegativeAmount},
		"a negative token count":        {negativeTokens, ErrNegativeAmount},
	} {
		raw, digest, err := EncodeEntry(tc.entry)
		if !errors.Is(err, tc.want) || raw != nil || digest != "" {
			t.Errorf("%s: EncodeEntry = %s, %q, %v; want nothing and %v", name, raw, digest, err, tc.want)
			continue
		}
		// It is the ledger's own refusal, word for word.
		if _, _, ledgerErr := NewLedger().With(tc.entry); ledgerErr == nil || ledgerErr.Error() != err.Error() {
			t.Errorf("%s: EncodeEntry says %q, a ledger says %v", name, err, ledgerErr)
		}
	}

	// Why the text has to be judged before it is encoded: JSON cannot carry a
	// byte that is not UTF-8, and encoding/json puts the replacement character
	// in its place. Encoded all the same, these two IDs would share one form
	// and one digest, and a decoder would read a third ID out of it.
	one := encodedForm(t, reserveEntry("reserve-\xff", "r1", 10))
	other := encodedForm(t, reserveEntry("reserve-\xfe", "r1", 10))
	third := reserveEntry("reserve-"+string(utf8.RuneError), "r1", 10)
	var read Entry
	if err := json.Unmarshal(one, &read); err != nil || !bytes.Equal(one, other) || read != third {
		t.Fatalf("encoding/json writes the two IDs as\n%s\n%s\nand reads %+v back, %v", one, other, read, err)
	}
	// Whether those bytes are a stored form depends on the Go release. Go 1.26
	// writes the byte as an escape, which no stored form has. Go 1.27 writes
	// the replacement character itself, and that is the stored form of the
	// entry with the third ID. Neither of the two IDs ever comes back.
	if got, err := DecodeEntry(EntryFormVersion, one); err != nil {
		if err := notStoredForm(got, err); err != nil {
			t.Errorf("the JSON encoding of an ID that is not UTF-8: %v", err)
		}
	} else if form, _ := encoded(t, third); got != third || !bytes.Equal(form, one) {
		t.Errorf("the JSON encoding of an ID that is not UTF-8 is read as %+v", got)
	}

	// An entry that only some ledgers refuse has a stored form: whether its
	// reservation exists, whether its ID is taken and what its scope has seen
	// are for With to say.
	for _, e := range []Entry{
		holdEntry("hold-1", "no-such-reservation", HoldTimeout),
		delta("d1", "no-such-reservation", "scope-1", reported(1)),
		{ID: "settle-1", Kind: KindSettle, ReservationID: "r1"},
	} {
		raw, digest := encoded(t, e)
		if back, err := DecodeEntry(EntryFormVersion, raw); err != nil || back != e || len(digest) != 64 {
			t.Errorf("%s: DecodeEntry = %+v, %v; digest %q", e.ID, back, err, digest)
		}
		if _, _, err := NewLedger().With(e); !errors.Is(err, ErrUnknownReservation) {
			t.Errorf("%s: an empty ledger says %v, want ErrUnknownReservation", e.ID, err)
		}
	}
}

// The table that keeps the entries takes an ID of at most 200 bytes and a
// stored form of at most 65,536. EncodeEntry refuses what would not fit, at
// exactly those sizes, counted in bytes of the ID and of the stored form.
func TestEncodeEntryKeepsToTheLimitsOfTheEntryTable(t *testing.T) {
	if MaxEntryIDBytes != 200 || MaxStoredEntryBytes != 65536 {
		t.Fatalf("the limits are %d and %d; the entry table has 200 and 65536", MaxEntryIDBytes, MaxStoredEntryBytes)
	}
	check := func(name string, e Entry, idBytes, formBytes int, fits bool) {
		t.Helper()
		plain, err := json.Marshal(e)
		if err != nil || len(e.ID) != idBytes || len(plain) != formBytes {
			t.Fatalf("%s: the fixture has an ID of %d bytes and a form of %d (%v), not %d and %d", name, len(e.ID), len(plain), err, idBytes, formBytes)
		}
		raw, digest, err := EncodeEntry(e)
		if !fits {
			if !errors.Is(err, ErrInvalidEntry) || raw != nil || digest != "" {
				t.Errorf("%s: EncodeEntry = %d bytes, %q, %v; want nothing and ErrInvalidEntry", name, len(raw), digest, err)
			}
			return
		}
		if err != nil || !bytes.Equal(raw, plain) {
			t.Errorf("%s: EncodeEntry = %d bytes, %v", name, len(raw), err)
			return
		}
		if back, err := DecodeEntry(EntryFormVersion, raw); err != nil || back != e {
			t.Errorf("%s: DecodeEntry: %v", name, err)
		}
	}

	// The ID. The longest one the service builds names an attempt, an
	// invocation and a model of 120 bytes: usage/<uuid>/<uuid>/<model>.
	const uuid = "0f8fad5b-d9cb-469f-a165-70867728950e"
	longest := "usage/" + uuid + "/" + uuid + "/" + strings.Repeat("m", 120)
	usage := func(id string) Entry { return delta(id, "", "scope-1", reported(5)) }
	shortForm := len(encodedForm(t, usage("")))
	for _, tc := range []struct {
		name string
		id   string
		size int
		fits bool
	}{
		{"an ID of one byte", "x", 1, true},
		{"the longest ID the service builds", longest, 200, true},
		{"the longest ID and one byte", longest + "m", 201, false},
		{"an ID of 199 bytes", longest[:199], 199, true},
		{"100 runes of two bytes", strings.Repeat("é", 100), 200, true},
		{"66 runes of three bytes and two letters", strings.Repeat("预", 66) + "ab", 200, true},
		{"67 runes of three bytes", strings.Repeat("预", 67), 201, false},
		{"an ID of a thousand bytes", strings.Repeat("x", 1000), 1000, false},
	} {
		check(tc.name, usage(tc.id), tc.size, shortForm+tc.size, tc.fits)
	}

	// The stored form. A gap entry takes any detail.
	gap := func(detail string) Entry {
		return Entry{ID: "gap-1", Kind: KindGap, GapReason: ReasonCoverageGap, Detail: detail}
	}
	room := MaxStoredEntryBytes - len(encodedForm(t, gap("")))
	// The stored form writes "<" in six bytes, so this detail of some 11,000
	// bytes fills the form to the last byte.
	escaped := strings.Repeat("<", room/6) + strings.Repeat("d", room%6)
	for _, tc := range []struct {
		name   string
		detail string
		size   int
		fits   bool
	}{
		{"a detail that leaves one byte free", strings.Repeat("d", room-1), 65535, true},
		{"a detail that fills the form", strings.Repeat("d", room), 65536, true},
		{"a detail one byte too long", strings.Repeat("d", room+1), 65537, false},
		{"a detail of escaped characters that fills the form", escaped, 65536, true},
		{"that detail and one byte", escaped + "d", 65537, false},
		{"that detail and one escaped character", escaped + "<", 65542, false},
		{"runes of three bytes that fill the form", strings.Repeat("预", room/3) + strings.Repeat("d", room%3), 65536, true},
		{"those runes and one byte", strings.Repeat("预", room/3) + strings.Repeat("d", room%3+1), 65537, false},
	} {
		check(tc.name, gap(tc.detail), len("gap-1"), tc.size, tc.fits)
	}
	if len(escaped) > 11_000 {
		t.Fatalf("the escaped detail has %d bytes", len(escaped))
	}

	// An ID that is too long is refused for its length before anything else is
	// said of the entry, and the refusal does not repeat it.
	long := strings.Repeat("x", 100_000)
	for name, e := range map[string]Entry{
		"a negative reservation": reserveEntry(long, "r1", -1),
		"an unknown kind":        {ID: long, Kind: "refund"},
		"a reservation":          reserveEntry(long, "r1", 10),
	} {
		raw, digest, err := EncodeEntry(e)
		if !errors.Is(err, ErrInvalidEntry) || errors.Is(err, ErrNegativeAmount) || raw != nil || digest != "" {
			t.Errorf("%s with an ID of %d bytes: EncodeEntry = %d bytes, %q, %v; want nothing and ErrInvalidEntry", name, len(long), len(raw), digest, err)
		} else if strings.Contains(err.Error(), long[:8]) {
			t.Errorf("%s with an ID of %d bytes: the refusal of %d bytes repeats the ID", name, len(long), len(err.Error()))
		}
	}
}

// DecodeEntry proves the form and nothing else. The form of an entry that no
// ledger accepts, or that is longer than the entry table takes, is read as
// that entry, and the digest of the bytes is the digest of the entry.
// EncodeEntry would not have written such a row, and With refuses an entry no
// ledger accepts when the row is replayed.
func TestDecodeEntryProvesTheFormAndNothingElse(t *testing.T) {
	sizedAndUnsized := reserveEntry("reserve-1", "r1", 10)
	sizedAndUnsized.Unsized = true
	foreign := reserveEntry("reserve-1", "r1", 10)
	foreign.Usage.ScopeID = "scope-1"
	tooLong := Entry{ID: "gap-1", Kind: KindGap, GapReason: ReasonCoverageGap, Detail: strings.Repeat("d", MaxStoredEntryBytes)}

	for name, tc := range map[string]struct {
		entry Entry
		// invalid: no ledger accepts the entry. The others are refused by
		// EncodeEntry for their size alone.
		invalid bool
	}{
		"the zero entry":               {Entry{}, true},
		"no ID":                        {reserveEntry("", "r1", 10), true},
		"an unknown kind":              {Entry{ID: "x", Kind: "refund"}, true},
		"an unsized reserve of 10":     {sizedAndUnsized, true},
		"a reserve with usage fields":  {foreign, true},
		"a hold for an unknown reason": {holdEntry("hold-1", "r1", "bored"), true},
		"an exclusion without a name":  {Entry{ID: "x-1", Kind: KindExclusion}, true},
		"an ID of 201 bytes":           {reserveEntry(strings.Repeat("x", MaxEntryIDBytes+1), "r1", 10), false},
		"a form beyond the limit":      {tooLong, false},
	} {
		form := encodedForm(t, tc.entry)
		if raw, digest, err := EncodeEntry(tc.entry); !errors.Is(err, ErrInvalidEntry) || raw != nil || digest != "" {
			t.Errorf("%s: EncodeEntry = %d bytes, %q, %v; want nothing and ErrInvalidEntry", name, len(raw), digest, err)
		}
		got, err := DecodeEntry(EntryFormVersion, form)
		if err != nil || got != tc.entry {
			t.Errorf("%s: DecodeEntry of %d bytes = %+v, %v; want the entry", name, len(form), got, err)
			continue
		}
		sum := sha256.Sum256(append([]byte("budget-entry-v1\n"), form...))
		if own, err := got.Digest(); err != nil || own != hex.EncodeToString(sum[:]) {
			t.Errorf("%s: Digest() = %q, %v; the bytes have %x", name, own, err, sum)
		}
		if !tc.invalid {
			continue
		}
		if next, receipts, err := NewLedger().With(got); !errors.Is(err, ErrInvalidEntry) || next != nil || receipts != nil {
			t.Errorf("%s: a ledger takes the decoded entry: %+v, %v", name, receipts, err)
		}
	}
}

// spelled is every character encoding/json does not write as its own bytes,
// in the order of the code points: the 32 controls, the quote, the three it
// escapes for HTML, the backslash and the two line separators.
const spelled = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
	`"&<>\` + "\u2028\u2029"

// spelledForm is how the stored form writes them.
const spelledForm = `\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\b\t\n\u000b\f\r\u000e\u000f` +
	`\u0010\u0011\u0012\u0013\u0014\u0015\u0016\u0017\u0018\u0019\u001a\u001b\u001c\u001d\u001e\u001f` +
	`\"\u0026\u003c\u003e\\\u2028\u2029`

// unspelled is a sample of what is written as itself, as every other
// character is: the delete character, a control of two bytes, runes of two,
// three and four bytes, the replacement character and the last code point.
const unspelled = "\x7f\u0080é预\ufffd🙂\U0010ffff"

// The stored form is what encoding/json writes, so a row is read back only
// while the encoder writes the bytes the row has. Go 1.26 and Go 1.27 write
// the same bytes for every Unicode scalar value; they differ for a byte that
// is not UTF-8, which no entry carries. The spelling of every character that
// has one is pinned here, so that a toolchain which writes one of them in
// another way fails this test before it refuses a row written earlier.
func TestStoredFormSpellsTextTheSameInEveryToolchain(t *testing.T) {
	gap := func(detail string) Entry {
		return Entry{ID: "gap-1", Kind: KindGap, GapReason: ReasonCoverageGap, Detail: detail}
	}
	form := func(spelling string) string {
		return strings.Replace(string(encodedForm(t, gap(""))), `"detail":""`, `"detail":"`+spelling+`"`, 1)
	}
	if n := utf8.RuneCountInString(spelled); n != 39 {
		t.Fatalf("the fixture has %d characters", n)
	}

	e := gap(spelled + unspelled)
	raw, digest := encoded(t, e)
	if want := form(spelledForm + unspelled); string(raw) != want {
		t.Fatalf("stored form\n%q\nwant\n%q", raw, want)
	}
	const wantDigest = "1335d7e77cb6a80bf49c913e30974de720f75978cb3abcda6be93a958f04a82d"
	if digest != wantDigest {
		t.Fatalf("digest = %q, want %q", digest, wantDigest)
	}
	if back, err := DecodeEntry(EntryFormVersion, raw); err != nil || back != e {
		t.Fatalf("DecodeEntry = %+v, %v", back, err)
	}

	// A character has one spelling in a stored form. Each of these is another
	// way to write the same text, which encoding/json reads as the same entry.
	for _, tc := range []struct {
		name, text, stored, other string
	}{
		{"a backspace as a number", "\b", `\b`, `\u0008`},
		{"a tab as a number", "\t", `\t`, `\u0009`},
		{"a control in capital hex digits", "\x1f", `\u001f`, `\u001F`},
		{"a quote as a number", `"`, `\"`, `\u0022`},
		{"a backslash as a number", `\`, `\\`, `\u005c`},
		{"a slash with a backslash", "/", `/`, `\/`},
		{"a less-than sign as itself", "<", `\u003c`, `<`},
		{"a greater-than sign in capital hex digits", ">", `\u003e`, `\u003E`},
		{"a line separator as itself", "\u2028", `\u2028`, "\u2028"},
		{"a letter of two bytes as a number", "é", `é`, `\u00e9`},
		{"a rune of four bytes as two numbers", "🙂", `🙂`, `\ud83d\ude42`},
		{"the replacement character as a number", "\ufffd", "\ufffd", `\ufffd`},
	} {
		want := gap(tc.text)
		if raw, _ := encoded(t, want); string(raw) != form(tc.stored) {
			t.Errorf("%s: the stored form is %q, want %q", tc.name, raw, form(tc.stored))
			continue
		}
		if tc.other == tc.stored {
			t.Errorf("%s: the other spelling is the stored one", tc.name)
			continue
		}
		var plain Entry
		if err := json.Unmarshal([]byte(form(tc.other)), &plain); err != nil || plain != want {
			t.Errorf("%s: encoding/json alone reads %q as %+v, %v", tc.name, form(tc.other), plain, err)
		}
		if err := notStoredForm(DecodeEntry(EntryFormVersion, []byte(form(tc.other)))); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// encodedForm is the JSON encoding of an entry, valid or not, for the tests
// that need to know how long a form would be.
func encodedForm(t *testing.T, e Entry) []byte {
	t.Helper()
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
