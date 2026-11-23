package daemon

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"time"
)

// The wire format of a usage batch (budget usage protocol v1). The server
// decodes strictly and refuses unknown fields, so these types carry exactly
// the fields of the protocol, declared in the order of its frozen example,
// and nothing of the private record.

type budgetUsageWireBatch struct {
	ProtocolVersion   int                         `json:"protocol_version"`
	BatchKey          string                      `json:"batch_key"`
	ProducerEpoch     string                      `json:"producer_epoch"`
	Adapter           string                      `json:"adapter"`
	AdapterVersion    string                      `json:"adapter_version,omitempty"`
	NormalizerVersion string                      `json:"normalizer_version"`
	ReportedAt        string                      `json:"reported_at"`
	Invocations       []budgetUsageWireInvocation `json:"invocations"`
}

type budgetUsageWireInvocation struct {
	InvocationID string                 `json:"invocation_id"`
	Resumed      bool                   `json:"resumed"`
	Ended        string                 `json:"ended"`
	ObservedAt   string                 `json:"observed_at"`
	Usage        []budgetUsageWireUsage `json:"usage"`
}

type budgetUsageWireUsage struct {
	Model     string                `json:"model"`
	Tokens    budgetUsageWireTokens `json:"tokens"`
	CostTicks string                `json:"cost_ticks,omitempty"`
}

type budgetUsageWireTokens struct {
	Input      string `json:"input"`
	Output     string `json:"output"`
	CacheRead  string `json:"cache_read"`
	CacheWrite string `json:"cache_write"`
}

// budgetUsageWireTime renders a time the way the wire format carries it: UTC,
// whole seconds.
func budgetUsageWireTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// budgetUsageWireItems lists an invocation's usage, one item per model,
// sorted by model. Each item carries all four token counts as canonical
// decimal strings, and the cost only when it is greater than zero. An item is
// left out only when it says nothing: four zero token counts and no cost.
// Nothing else is checked on the server's behalf.
func budgetUsageWireItems(usage map[string]budgetUsageCounts) []budgetUsageWireUsage {
	items := make([]budgetUsageWireUsage, 0, len(usage))
	for _, model := range slices.Sorted(maps.Keys(usage)) {
		counts := usage[model]
		noTokens := counts.InputTokens == 0 && counts.OutputTokens == 0 && counts.CacheReadTokens == 0 && counts.CacheWriteTokens == 0
		if noTokens && counts.CostTicks <= 0 {
			continue
		}
		item := budgetUsageWireUsage{
			Model: model,
			Tokens: budgetUsageWireTokens{
				Input:      strconv.FormatInt(counts.InputTokens, 10),
				Output:     strconv.FormatInt(counts.OutputTokens, 10),
				CacheRead:  strconv.FormatInt(counts.CacheReadTokens, 10),
				CacheWrite: strconv.FormatInt(counts.CacheWriteTokens, 10),
			},
		}
		if counts.CostTicks > 0 {
			item.CostTicks = strconv.FormatInt(counts.CostTicks, 10)
		}
		items = append(items, item)
	}
	return items
}

// buildBudgetUsageBatch produces the exact bytes of the batch of a record.
// producerEpoch is the start identity of the process that builds the batch
// and reportedAt the build time. An invocation the record holds only a launch
// intent for is reported as unknown, with no usage and its launch time.
func buildBudgetUsageBatch(record budgetUsageRecord, producerEpoch string, reportedAt time.Time) ([]byte, error) {
	batch := budgetUsageWireBatch{
		ProtocolVersion:   budgetUsageProtocolVersion,
		BatchKey:          record.BatchKey,
		ProducerEpoch:     producerEpoch,
		Adapter:           record.Adapter,
		AdapterVersion:    record.AdapterVersion,
		NormalizerVersion: record.NormalizerVersion,
		ReportedAt:        budgetUsageWireTime(reportedAt),
		// Never null and never omitted: an attempt that launched nothing
		// reports an empty array.
		Invocations: make([]budgetUsageWireInvocation, 0, len(record.Invocations)),
	}
	for _, invocation := range record.Invocations {
		wire := budgetUsageWireInvocation{
			InvocationID: invocation.InvocationID,
			Resumed:      invocation.Resumed,
			Ended:        invocation.Ended,
			ObservedAt:   budgetUsageWireTime(invocation.ObservedAt),
			Usage:        budgetUsageWireItems(invocation.Usage),
		}
		if invocation.Ended == "" {
			wire.Ended = budgetUsageEndedUnknown
			wire.ObservedAt = budgetUsageWireTime(invocation.LaunchedAt)
			wire.Usage = []budgetUsageWireUsage{}
		}
		batch.Invocations = append(batch.Invocations, wire)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	// A model name is sent as the provider gave it; claude reports one called
	// "<synthetic>".
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(batch); err != nil {
		return nil, err
	}
	// Encode ends the value with a newline that is not part of it.
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
