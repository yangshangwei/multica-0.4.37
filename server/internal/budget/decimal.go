package budget

import (
	"math"
	"strconv"
)

// maxDecimalDigits is the digit count of the largest signed 64-bit value.
const maxDecimalDigits = 19

// ParseDecimal reads a canonical non-negative decimal string: ASCII digits
// only, no sign, fraction, exponent, separator or whitespace, and no leading
// zero except for "0" itself.
func ParseDecimal(s string) (int64, error) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, ErrInvalidDecimal
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, ErrInvalidDecimal
		}
	}
	if len(s) > maxDecimalDigits {
		return 0, ErrOverflow
	}
	// Nineteen digits always fit an unsigned 64-bit value.
	var v uint64
	for i := 0; i < len(s); i++ {
		v = v*10 + uint64(s[i]-'0')
	}
	if v > math.MaxInt64 {
		return 0, ErrOverflow
	}
	return int64(v), nil
}

// FormatDecimal writes the canonical decimal string ParseDecimal accepts.
func FormatDecimal(v int64) (string, error) {
	if v < 0 {
		return "", ErrNegativeAmount
	}
	return strconv.FormatInt(v, 10), nil
}

func marshalDecimal(v int64) ([]byte, error) {
	s, err := FormatDecimal(v)
	if err != nil {
		return nil, err
	}
	return []byte(`"` + s + `"`), nil
}

// unmarshalDecimal accepts a JSON string holding a canonical decimal and
// nothing else: a JSON number, null, or an escaped digit is rejected.
func unmarshalDecimal(data []byte) (int64, error) {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return 0, ErrInvalidDecimal
	}
	return ParseDecimal(string(data[1 : len(data)-1]))
}

func (t Ticks) MarshalJSON() ([]byte, error) { return marshalDecimal(int64(t)) }

func (t *Ticks) UnmarshalJSON(data []byte) error {
	v, err := unmarshalDecimal(data)
	if err != nil {
		return err
	}
	*t = Ticks(v)
	return nil
}

func (c Count) MarshalJSON() ([]byte, error) { return marshalDecimal(int64(c)) }

func (c *Count) UnmarshalJSON(data []byte) error {
	v, err := unmarshalDecimal(data)
	if err != nil {
		return err
	}
	*c = Count(v)
	return nil
}

// addChecked adds two signed values and reports a result outside the 64-bit
// range instead of wrapping.
func addChecked(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, ErrOverflow
	}
	return a + b, nil
}

// mulChecked multiplies two non-negative values.
func mulChecked(a, b int64) (int64, error) {
	if b != 0 && a > math.MaxInt64/b {
		return 0, ErrOverflow
	}
	return a * b, nil
}
