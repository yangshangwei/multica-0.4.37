package budget_test

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/pkg/agent"
)

// The adapters own the provider-reported cost unit. Budget amounts must use
// the same scale, or every reported cost would be misread by a constant factor.
func TestScaleMatchesAdapterCostUnit(t *testing.T) {
	if budget.TicksPerUSD != agent.CostUSDTicksPerUSD {
		t.Fatalf("budget.TicksPerUSD = %d, agent.CostUSDTicksPerUSD = %d; they must be equal", budget.TicksPerUSD, int64(agent.CostUSDTicksPerUSD))
	}
}
