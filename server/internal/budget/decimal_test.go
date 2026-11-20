package budget

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestScaleIsTenBillionTicksPerUSD(t *testing.T) {
	if TicksPerUSD != 10_000_000_000 {
		t.Fatalf("TicksPerUSD = %d, want 10000000000", TicksPerUSD)
	}
}

// B24a: the wire codec accepts only canonical non-negative decimal strings.
func TestB24aDecimalCodecAcceptsCanonicalStrings(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"7", 7},
		{"30000030000", 30_000_030_000},
		{"9223372036854775807", 9_223_372_036_854_775_807},
	} {
		got, err := ParseDecimal(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseDecimal(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
		back, err := FormatDecimal(tc.want)
		if err != nil || back != tc.in {
			t.Errorf("FormatDecimal(%d) = %q, %v; want %q", tc.want, back, err, tc.in)
		}
	}
}

func TestB24aDecimalCodecRejectsNonCanonicalStrings(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want error
	}{
		{"1e3", ErrInvalidDecimal},
		{"1E3", ErrInvalidDecimal},
		{"1.5", ErrInvalidDecimal},
		{"1.0", ErrInvalidDecimal},
		{"-1", ErrInvalidDecimal},
		{"+1", ErrInvalidDecimal},
		{"-0", ErrInvalidDecimal},
		{"", ErrInvalidDecimal},
		{"00", ErrInvalidDecimal},
		{"01", ErrInvalidDecimal},
		{" 1", ErrInvalidDecimal},
		{"1 ", ErrInvalidDecimal},
		{"1\n", ErrInvalidDecimal},
		{"1_000", ErrInvalidDecimal},
		{"1,000", ErrInvalidDecimal},
		{"0x10", ErrInvalidDecimal},
		{"\uff11\uff12", ErrInvalidDecimal}, // fullwidth digits
		{"9223372036854775808", ErrOverflow},
		{"18446744073709551616", ErrOverflow},
		{"100000000000000000000000000000", ErrOverflow},
	} {
		got, err := ParseDecimal(tc.in)
		if !errors.Is(err, tc.want) || got != 0 {
			t.Errorf("ParseDecimal(%q) = %d, %v; want 0, %v", tc.in, got, err, tc.want)
		}
	}
	if s, err := FormatDecimal(-1); !errors.Is(err, ErrNegativeAmount) || s != "" {
		t.Errorf("FormatDecimal(-1) = %q, %v; want ErrNegativeAmount", s, err)
	}
}

func TestB24aTicksJSONIsADecimalStringNeverANumber(t *testing.T) {
	raw, err := json.Marshal(struct {
		Amount Ticks `json:"amount"`
		Tokens Count `json:"tokens"`
	}{Amount: 9_223_372_036_854_775_807, Tokens: 1_000_001})
	if err != nil || string(raw) != `{"amount":"9223372036854775807","tokens":"1000001"}` {
		t.Fatalf("marshal = %s, %v", raw, err)
	}
	var got struct {
		Amount Ticks `json:"amount"`
		Tokens Count `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &got); err != nil || got.Amount != 9_223_372_036_854_775_807 || got.Tokens != 1_000_001 {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
	for _, in := range []string{
		`9007199254740993`, // JSON number beyond the exactly representable integer range
		`1`,
		`0`,
		`-1`,
		`"-1"`,
		`"1e3"`,
		`"1.5"`,
		`1.5`,
		`1e3`,
		`null`,
		`true`,
		`""`,
		`"01"`,
		`" 1"`,
		`"\u0031"`, // an escaped digit is not canonical
		`"9223372036854775808"`,
		`["1"]`,
		`{"v":"1"}`,
	} {
		ticks, count := Ticks(41), Count(41)
		if err := json.Unmarshal([]byte(in), &ticks); err == nil || ticks != 41 {
			t.Errorf("Ticks accepted %s as %d (err %v)", in, ticks, err)
		}
		if err := json.Unmarshal([]byte(in), &count); err == nil || count != 41 {
			t.Errorf("Count accepted %s as %d (err %v)", in, count, err)
		}
	}
	var wrapped struct {
		Amount Ticks `json:"amount"`
	}
	if err := json.Unmarshal([]byte(`{"amount":9007199254740993}`), &wrapped); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("number field error = %v, want ErrInvalidDecimal", err)
	}
	if _, err := json.Marshal(Ticks(-1)); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("marshal negative error = %v, want ErrNegativeAmount", err)
	}
	// An absent optional amount is null, which is distinct from "0".
	var optional struct {
		Remaining *Ticks `json:"remaining"`
	}
	if err := json.Unmarshal([]byte(`{"remaining":null}`), &optional); err != nil || optional.Remaining != nil {
		t.Fatalf("optional = %+v, %v", optional, err)
	}
}
