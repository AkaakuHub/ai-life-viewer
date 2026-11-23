package pricing

import (
	"math"
	"strings"
	"testing"

	"agent-timeline/internal/timeline"
)

func TestEstimateAppliesCacheRateAndNeverChargesReasoningSeparately(t *testing.T) {
	input, cached, output := int64(1_000_000), int64(250_000), int64(100_000)
	c := &Catalog{}
	for _, row := range []struct {
		model string
		want  float64
	}{
		{"openai/gpt-6.2-sol", 2.525},
		{"perplexity/openai/gpt-6-sol", 2.55},
		{"gemini/gemini-4-neon", 3.7875},
		{"anthropic.claude-opus-5-5", 5.05},
	} {
		cost := c.Estimate(row.model, "", &input, &cached, &output)
		if cost == nil || math.Abs(*cost-row.want) > 1e-12 {
			t.Errorf("%s: cost=%v want=%f", row.model, cost, row.want)
		}
	}
	input = 100_000
	cost := c.Estimate("gpt-6.2-sol", "", &input, &cached, &output)
	if cost == nil || math.Abs(*cost-1.025) > 1e-12 {
		t.Fatalf("cached input exceeds total: %v", cost)
	}
}

func TestCatalogUsesDynamicRatesAndLeavesUnknownCostsUnset(t *testing.T) {
	c, err := decodeCatalog(strings.NewReader(`{"document":{
		"azure_ai/custom": {"input_cost_per_token":0.000003,"cache_read_input_token_cost":0.000001,"output_cost_per_token":0.000004},
		"gpt-6.2-sol": {"input_cost_per_token":0.000005,"output_cost_per_token":0.000008},
		"apac.anthropic.claude-sonnet-4-20250514-v1:0": {"input_cost_per_token":0.000003,"cache_read_input_token_cost":0.0000003,"output_cost_per_token":0.000015},
		"vercel_ai_gateway/zai/glm-4.5-air": {"input_cost_per_token":0.0000002,"output_cost_per_token":0.0000011},
		"zai/glm-4.7": {"input_cost_per_token":0.0000006,"cache_read_input_token_cost":0.00000011,"output_cost_per_token":0.0000022},
		"no-cache-rate": {"input_cost_per_token":0.000003,"output_cost_per_token":0.000004},
		"unusable": {"input_cost_per_token":-1,"output_cost_per_token":1}
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	input, cached, output := int64(1_000_000), int64(250_000), int64(100_000)
	for _, row := range []struct {
		model string
		want  float64
	}{
		{"perplexity/openai/custom", 2.9}, {"gpt-6.2-sol", 5.8}, {"no-cache-rate", 3.4},
		{"claude-sonnet-4-20250514", 3.825}, {"us.anthropic.claude-sonnet-4-20250514-v1", 3.825},
		{"glm-4.5-air", .31}, {"cloudflare/zai-org/glm-4.7", .6975}, {"zai/glm-5.3-flash-free", 0},
	} {
		cost := c.Estimate(row.model, "", &input, &cached, &output)
		if cost == nil || math.Abs(*cost-row.want) > 1e-12 {
			t.Errorf("%s: cost=%v want=%f", row.model, cost, row.want)
		}
	}
	for _, model := range []string{"unknown", "unusable"} {
		if c.Estimate(model, "", &input, &cached, &output) != nil {
			t.Errorf("%s should be unpriced", model)
		}
	}
	if c.Estimate("custom", "", nil, &cached, &output) != nil {
		t.Error("missing input should be unpriced")
	}
}

func TestPricesModelAndTierBreakdownWithoutChargingSessionTotalsAgain(t *testing.T) {
	input, cached, output := int64(1_000_000), int64(250_000), int64(100_000)
	c := &Catalog{}
	block := timeline.Block{
		Model: "gpt-6.2-sol", ServiceTier: "priority",
		Metrics: timeline.Metrics{InputTokens: &input, OutputTokens: &output},
		UsageByModel: []timeline.ModelUsage{
			{Model: "gpt-6.2-sol", ServiceTier: "fast", InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output},
			{Model: "claude-sonnet-4-20250514", ServiceTier: "priority", InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output},
			{Model: "unavailable", ServiceTier: "default", InputTokens: &input, OutputTokens: &output},
		},
	}
	c.Price(&block)
	if block.EstimatedCostUSD == nil || math.Abs(*block.EstimatedCostUSD-12.7) > 1e-12 || !block.IncompleteCost {
		t.Fatalf("breakdown price = %v, incomplete = %v", block.EstimatedCostUSD, block.IncompleteCost)
	}
	fallback := timeline.Block{Model: block.Model, ServiceTier: "priority", Metrics: block.Metrics}
	c.Price(&fallback)
	if fallback.EstimatedCostUSD == nil || math.Abs(*fallback.EstimatedCostUSD-6) > 1e-12 {
		t.Fatalf("session fallback cost = %v", fallback.EstimatedCostUSD)
	}
}
