package pricing

import (
	"math"
	"strings"
	"testing"
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
		cost := c.Estimate(row.model, &input, &cached, &output)
		if cost == nil || math.Abs(*cost-row.want) > 1e-12 {
			t.Errorf("%s: cost=%v want=%f", row.model, cost, row.want)
		}
	}
	input = 100_000
	cost := c.Estimate("gpt-6.2-sol", &input, &cached, &output)
	if cost == nil || math.Abs(*cost-1.025) > 1e-12 {
		t.Fatalf("cached input exceeds total: %v", cost)
	}
}

func TestCatalogUsesDynamicRatesAndLeavesUnknownCostsUnset(t *testing.T) {
	c, err := decodeCatalog(strings.NewReader(`{"document":{
		"azure_ai/custom": {"input_cost_per_token":0.000003,"cache_read_input_token_cost":0.000001,"output_cost_per_token":0.000004},
		"gpt-6.2-sol": {"input_cost_per_token":0.000005,"output_cost_per_token":0.000008},
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
		{"perplexity/openai/custom", 2.9}, {"gpt-6.2-sol", 4.575},
	} {
		cost := c.Estimate(row.model, &input, &cached, &output)
		if cost == nil || math.Abs(*cost-row.want) > 1e-12 {
			t.Errorf("%s: cost=%v want=%f", row.model, cost, row.want)
		}
	}
	for _, model := range []string{"unknown", "no-cache-rate", "unusable"} {
		if c.Estimate(model, &input, &cached, &output) != nil {
			t.Errorf("%s should be unpriced", model)
		}
	}
	if c.Estimate("custom", nil, &cached, &output) != nil {
		t.Error("missing input should be unpriced")
	}
}
