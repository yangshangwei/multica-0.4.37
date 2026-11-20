package budget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"unicode/utf8"
)

// Rate is the price of one token dimension as an integer fraction: Ticks per
// PerTokens tokens.
type Rate struct {
	Ticks     Ticks `json:"ticks"`
	PerTokens Count `json:"per_tokens"`
}

// USDRate builds the rate "usdNumerator/usdDenominator USD per perTokens
// tokens" without dividing, so a price such as 1/3 USD stays exact.
func USDRate(usdNumerator, usdDenominator, perTokens int64) (Rate, error) {
	if usdNumerator < 0 || usdDenominator <= 0 || perTokens <= 0 {
		return Rate{}, fmt.Errorf("%w: rate %d/%d USD per %d tokens", ErrInvalidPrice, usdNumerator, usdDenominator, perTokens)
	}
	ticks, err := mulChecked(usdNumerator, TicksPerUSD)
	if err != nil {
		return Rate{}, err
	}
	per, err := mulChecked(usdDenominator, perTokens)
	if err != nil {
		return Rate{}, err
	}
	return Rate{Ticks: Ticks(ticks), PerTokens: Count(per)}, nil
}

// Rounding versions decide how a fractional estimate is shown. A reservation
// always rounds up, whatever the version.
const (
	RoundingHalfUpV1 = "half-up-v1"
	RoundingCeilV1   = "ceil-v1"
)

// PriceSnapshot is the price table frozen by the server for one execution.
// Its stored form is its JSON encoding; Digest binds every field of it.
type PriceSnapshot struct {
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	NativeModel       string `json:"native_model"`
	AdapterVersion    string `json:"adapter_version"`
	NormalizerVersion string `json:"normalizer_version"`
	PriceVersion      string `json:"price_version"`
	Source            string `json:"source"`
	EffectiveFrom     string `json:"effective_from"`
	EffectiveUntil    string `json:"effective_until"`
	Currency          string `json:"currency"`
	RoundingVersion   string `json:"rounding_version"`
	Input             Rate   `json:"input"`
	Output            Rate   `json:"output"`
	CacheRead         Rate   `json:"cache_read"`
	CacheWrite        Rate   `json:"cache_write"`
}

// Estimate is the result of pricing token counts with a frozen snapshot.
// Ticks follows the snapshot's rounding version; ReserveTicks is the ceiling.
// ExactNumerator/ExactDenominator hold the unrounded value in lowest terms.
type Estimate struct {
	Ticks            Ticks  `json:"ticks"`
	ReserveTicks     Ticks  `json:"reserve_ticks"`
	Exact            bool   `json:"exact"`
	ExactNumerator   string `json:"exact_numerator"`
	ExactDenominator string `json:"exact_denominator"`
	PriceVersion     string `json:"price_version"`
	PriceDigest      string `json:"price_digest"`
	RoundingVersion  string `json:"rounding_version"`
	// ClaimedPriceVersionIgnored reports that the caller named a different
	// price version. The claim changes nothing in the computation.
	ClaimedPriceVersionIgnored bool `json:"claimed_price_version_ignored"`
}

func (s PriceSnapshot) rates() [4]Rate {
	return [4]Rate{s.Input, s.Output, s.CacheRead, s.CacheWrite}
}

func (s PriceSnapshot) Validate() error {
	if s.Currency != CurrencyUSD {
		return fmt.Errorf("%w: %q", ErrUnsupportedCurrency, s.Currency)
	}
	if s.Provider == "" || s.Model == "" || s.AdapterVersion == "" || s.NormalizerVersion == "" || s.PriceVersion == "" {
		return fmt.Errorf("%w: provider, model, adapter version, normalizer version and price version are required", ErrInvalidPrice)
	}
	if s.RoundingVersion != RoundingHalfUpV1 && s.RoundingVersion != RoundingCeilV1 {
		return fmt.Errorf("%w: rounding version %q", ErrInvalidPrice, s.RoundingVersion)
	}
	// The stored form and the digest are JSON, which replaces invalid UTF-8:
	// two different snapshots would share one digest.
	for _, text := range [...]string{
		s.Provider, s.Model, s.NativeModel, s.AdapterVersion, s.NormalizerVersion,
		s.PriceVersion, s.Source, s.EffectiveFrom, s.EffectiveUntil,
	} {
		if !utf8.ValidString(text) {
			return fmt.Errorf("%w: text is not valid UTF-8", ErrInvalidPrice)
		}
	}
	for _, r := range s.rates() {
		if r.Ticks < 0 {
			return fmt.Errorf("%w: rate", ErrNegativeAmount)
		}
		if r.PerTokens <= 0 {
			return fmt.Errorf("%w: rate denominator %d", ErrInvalidPrice, r.PerTokens)
		}
	}
	return nil
}

// Digest is the SHA-256 of the snapshot's stored form under a versioned
// domain prefix, in lowercase hex.
func (s PriceSnapshot) Digest() (string, error) {
	return digest("budget-price-snapshot-v1\n", s)
}

func digest(domain string, v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Estimate prices tokens with this snapshot and nothing else. The four
// dimensions are summed as exact fractions and rounded once.
// claimedPriceVersion is what the caller says the price version is; it is
// reported when it differs and never used.
func (s PriceSnapshot) Estimate(tokens TokenCounts, claimedPriceVersion string) (Estimate, error) {
	if err := s.Validate(); err != nil {
		return Estimate{}, err
	}
	counts := [4]Count{tokens.Input, tokens.Output, tokens.CacheRead, tokens.CacheWrite}
	total := new(big.Rat)
	for i, r := range s.rates() {
		if counts[i] < 0 {
			return Estimate{}, fmt.Errorf("%w: token count", ErrNegativeAmount)
		}
		cost := new(big.Int).Mul(big.NewInt(int64(counts[i])), big.NewInt(int64(r.Ticks)))
		total.Add(total, new(big.Rat).SetFrac(cost, big.NewInt(int64(r.PerTokens))))
	}

	// total is non-negative, so truncating division is the floor.
	floor, remainder := new(big.Int).QuoRem(total.Num(), total.Denom(), new(big.Int))
	exact := remainder.Sign() == 0
	ceiling := new(big.Int).Set(floor)
	if !exact {
		ceiling.Add(ceiling, big.NewInt(1))
	}
	shown := ceiling
	if s.RoundingVersion == RoundingHalfUpV1 && remainder.Lsh(remainder, 1).Cmp(total.Denom()) < 0 {
		shown = floor
	}
	// The ceiling is the largest of the three, so it bounds them all.
	if !ceiling.IsInt64() {
		return Estimate{}, ErrOverflow
	}
	priceDigest, err := s.Digest()
	if err != nil {
		return Estimate{}, err
	}
	return Estimate{
		Ticks:                      Ticks(shown.Int64()),
		ReserveTicks:               Ticks(ceiling.Int64()),
		Exact:                      exact,
		ExactNumerator:             total.Num().String(),
		ExactDenominator:           total.Denom().String(),
		PriceVersion:               s.PriceVersion,
		PriceDigest:                priceDigest,
		RoundingVersion:            s.RoundingVersion,
		ClaimedPriceVersionIgnored: claimedPriceVersion != "" && claimedPriceVersion != s.PriceVersion,
	}, nil
}
