package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
)

type IssueBudgetUsageEvidenceKey struct{ Adapter, NormalizerVersion string }
type IssueBudgetUsageEvidence struct {
	FreshOwnUsage   bool `json:"fresh_own_usage"`
	ResumedOwnUsage bool `json:"resumed_own_usage"`
	ReportedCost    bool `json:"reported_cost"`
}
type IssueBudgetUsageEvidenceTable map[IssueBudgetUsageEvidenceKey]IssueBudgetUsageEvidence
type IssueBudgetUsagePriceSource func(string) (metrics.ModelPrice, bool)

func productionIssueBudgetUsageEvidence() IssueBudgetUsageEvidenceTable {
	return IssueBudgetUsageEvidenceTable{
		// Claude cost-tracking documentation, retrieved 2026-11-18:
		// https://code.claude.com/docs/en/agent-sdk/cost-tracking.md
		// Independent calls cover only their own usage (L222). Resumed modelUsage
		// includes history since 2.1.277 (L61/L223). claude.go never supplies cost.
		// Interrupted assistant-message sums may be imprecise; estimates never
		// become actual cost. Changing this row requires renewed evidence/spec.
		{Adapter: "claude", NormalizerVersion: "1"}: {FreshOwnUsage: true},
	}
}

type issueBudgetUsageEvidenceResult struct {
	Matched           bool   `json:"matched"`
	Adapter           string `json:"adapter"`
	NormalizerVersion string `json:"normalizer_version"`
	IssueBudgetUsageEvidence
}

type issueBudgetUsageInvocationResult struct {
	InvocationID string                         `json:"invocation_id"`
	Resumed      bool                           `json:"resumed"`
	Ended        string                         `json:"ended"`
	Evidence     issueBudgetUsageEvidenceResult `json:"evidence"`
}

type issueBudgetNormalizedUsage struct {
	Entries     []issueBudgetLedgerEntry
	Invocations []issueBudgetUsageInvocationResult
}

func normalizeIssueBudgetUsage(batch issueBudgetUsageBatch, attemptID pgtype.UUID, hasReservation bool, evidence IssueBudgetUsageEvidenceTable, prices IssueBudgetUsagePriceSource) issueBudgetNormalizedUsage {
	attempt := util.UUIDToString(attemptID)
	reservation := ""
	if hasReservation {
		reservation = attempt
	}
	facts, matched := evidence[IssueBudgetUsageEvidenceKey{batch.Adapter, batch.NormalizerVersion}]
	version := batch.AdapterVersion
	if version == "" {
		version = "unknown"
	}
	adapterVersion := batch.Adapter + "/" + version
	reportedAt, _ := time.Parse(time.RFC3339, batch.ReportedAt)
	out := issueBudgetNormalizedUsage{Entries: []issueBudgetLedgerEntry{}, Invocations: []issueBudgetUsageInvocationResult{}}
	for _, inv := range batch.Invocations {
		out.Invocations = append(out.Invocations, issueBudgetUsageInvocationResult{
			InvocationID: inv.InvocationID, Resumed: inv.Resumed, Ended: inv.Ended,
			Evidence: issueBudgetUsageEvidenceResult{Matched: matched, Adapter: batch.Adapter, NormalizerVersion: batch.NormalizerVersion, IssueBudgetUsageEvidence: facts},
		})
		observedAt, _ := time.Parse(time.RFC3339, inv.ObservedAt)
		row := issueBudgetLedgerEntry{Source: "daemon_report", AttemptID: attemptID, AdapterVersion: adapterVersion,
			NormalizerVersion: batch.NormalizerVersion, ProducerEpoch: batch.ProducerEpoch, PayloadDigest: batch.PayloadDigest,
			ReportedAt: reportedAt, ObservedAt: observedAt,
		}
		if len(inv.Usage) == 0 {
			detail := "no_usage:"
			if inv.Ended == "unknown" {
				detail = "outcome_unknown:"
			}
			row.Entry = budget.Entry{ID: "gap/" + attempt + "/" + inv.InvocationID, Kind: budget.KindGap, ReservationID: reservation, GapReason: budget.ReasonCoverageGap, Detail: detail + inv.InvocationID}
			out.Entries = append(out.Entries, row)
			continue
		}
		overlap := budget.OverlapUnproven
		if (!inv.Resumed && facts.FreshOwnUsage) || (inv.Resumed && facts.ResumedOwnUsage) {
			overlap = budget.OverlapNone
		}
		row.Completeness = "final"
		if inv.Ended == "interrupted" {
			row.Completeness = "partial"
		}
		for _, item := range inv.Usage {
			cost, snapshot, digest := issueBudgetUsageCost(batch, item, facts, prices, adapterVersion)
			row.PriceSnapshot, row.PriceDigest = snapshot, digest
			row.Entry = budget.Entry{ID: "usage/" + attempt + "/" + inv.InvocationID + "/" + item.Model, Kind: budget.KindUsage, ReservationID: reservation,
				Usage: budget.Usage{ScopeID: attempt + "/" + inv.InvocationID, InvocationID: inv.InvocationID, Mode: budget.ModeDelta, Overlap: overlap, Tokens: item.Tokens, Cost: cost},
			}
			out.Entries = append(out.Entries, row)
		}
	}
	return out
}

func issueBudgetUsageCost(batch issueBudgetUsageBatch, item issueBudgetUsageItem, evidence IssueBudgetUsageEvidence, prices IssueBudgetUsagePriceSource, adapterVersion string) (budget.Cost, []byte, string) {
	if item.CostTicks != nil {
		basis := budget.CostEstimated
		if evidence.ReportedCost {
			basis = budget.CostReported
		}
		return budget.Cost{Basis: basis, Ticks: *item.CostTicks, Currency: budget.CurrencyUSD}, nil, ""
	}
	if prices == nil {
		return budget.Cost{}, nil, ""
	}
	price, ok := prices(item.Model)
	if !ok {
		return budget.Cost{}, nil, ""
	}
	snapshot, err := issueBudgetUsagePriceSnapshot(price, item.Model, adapterVersion, batch.NormalizerVersion)
	if err != nil {
		return budget.Cost{}, nil, ""
	}
	estimate, err := snapshot.Estimate(item.Tokens, "")
	if err != nil {
		return budget.Cost{}, nil, ""
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return budget.Cost{}, nil, ""
	}
	return budget.Cost{Basis: budget.CostEstimated, Ticks: estimate.Ticks, Currency: budget.CurrencyUSD, PriceVersion: estimate.PriceVersion, PriceDigest: estimate.PriceDigest}, raw, estimate.PriceDigest
}

func issueBudgetUsagePriceSnapshot(price metrics.ModelPrice, nativeModel, adapterVersion, normalizerVersion string) (budget.PriceSnapshot, error) {
	var rates [4]budget.Rate
	for i, v := range [4]float64{price.InputPerM, price.OutputPerM, price.CacheReadPerM, price.CacheWritePerM} {
		rate, err := issueBudgetUsageRate(v)
		if err != nil {
			return budget.PriceSnapshot{}, err
		}
		rates[i] = rate
	}
	// Version identity deliberately excludes the model/provider labels. Its
	// canonical bytes are this domain followed by [input,output,read,write] JSON.
	raw, err := json.Marshal(rates)
	if err != nil {
		return budget.PriceSnapshot{}, err
	}
	sum := sha256.Sum256(append([]byte("budget-metrics-rates-v1\n"), raw...))
	return budget.PriceSnapshot{Provider: price.Provider, Model: price.Model, NativeModel: nativeModel,
		AdapterVersion: adapterVersion, NormalizerVersion: normalizerVersion, PriceVersion: "metrics:" + hex.EncodeToString(sum[:8]),
		Currency: budget.CurrencyUSD, RoundingVersion: budget.RoundingHalfUpV1,
		Input: rates[0], Output: rates[1], CacheRead: rates[2], CacheWrite: rates[3],
	}, nil
}

func issueBudgetUsageRate(v float64) (budget.Rate, error) {
	// The only floating-point operation in this path: shortest round-trip
	// decimal formatting. Its numerator and power-of-ten denominator stay exact.
	text := strconv.FormatFloat(v, 'f', -1, 64)
	integer, fraction, _ := strings.Cut(text, ".")
	if strings.HasPrefix(integer, "-") {
		return budget.Rate{}, budget.ErrInvalidPrice
	}
	digits := strings.TrimLeft(integer+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	numerator, err := budget.ParseDecimal(digits)
	if err != nil {
		return budget.Rate{}, fmt.Errorf("price numerator: %w", err)
	}
	denominator, err := budget.ParseDecimal("1" + strings.Repeat("0", len(fraction)))
	if err != nil {
		return budget.Rate{}, fmt.Errorf("price denominator: %w", err)
	}
	return budget.USDRate(numerator, denominator, 1_000_000)
}
