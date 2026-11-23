package pricing

import (
	"agent-timeline/internal/timeline"
	"strings"
)

type rates struct {
	Input  *float64 `json:"input_cost_per_token"`
	Cached *float64 `json:"cache_read_input_token_cost"`
	Output *float64 `json:"output_cost_per_token"`
}

type Catalog struct {
	exact      map[string]rates
	normalized map[string]rates
}

var fallback = map[string][3]float64{
	"claude-sonnet-4-20250514": {3, .30, 15},
	"claude-opus-4-20250514":   {15, 1.50, 75},
	"claude-opus-4-6":          {5, .50, 25},
	"glm-4.5-air":              {.20, .20, 1.10},
	"glm-4.7":                  {.60, .12, 2.20},
	"gpt-5-codex":              {1.25, .125, 10},
	"gpt-5.1-codex":            {1.25, .125, 10},
	"gpt-5.1-codex-max":        {1.25, .125, 10},
	"gpt-5.2-codex":            {1.75, .175, 14},
	"gpt-5.3-codex":            {1.75, .175, 14},
	"gpt-5.2":                  {1.75, .175, 14},
	"gpt-5.4":                  {2.50, .25, 15},
	"gpt-5.5":                  {5, .50, 30},
	"gpt-6.1-terra":            {1, .05, 5},
	"gemini-2.5-pro":           {1.25, .125, 10},
	"gpt-6.2-sol":              {2, .10, 10},
	"gpt-6.1-sol":              {2, .10, 10},
	"gpt-6-sol":                {2, .20, 10},
	"gpt-6-luna":               {.10, .01, .50},
	"gpt-6-astra":              {10, 1, 50},
	"gpt-6.2-astra":            {8, .40, 40},
	"gpt-5.6-sol":              {4, .40, 20},
	"gpt-5.6-luna":             {.20, .02, 1.20},
	"gpt-5.6-terra":            {2, .20, 12},
	"gemini-4-neon":            {3, .15, 15},
	"claude-opus-5-5":          {4, .20, 20},
	"claude-sonnet-5-5":        {2, .10, 10},
	"claude-haiku-5-5":         {.10, .01, .50},
}

func normalize(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	for {
		prefixed := model
		for _, prefix := range []string{
			"openai/", "openai.", "azure_ai/", "azure/", "perplexity/", "gemini/",
			"anthropic.", "anthropic/", "google/", "vertex_ai/", "openrouter/",
			"zai/", "zai.", "zai-org/", "baseten/", "vercel_ai_gateway/", "gmi/", "cloudflare/",
			"apac.", "eu.", "us.", "global.", "chatgpt/",
		} {
			if suffix, ok := strings.CutPrefix(model, prefix); ok {
				model = suffix
				break
			}
		}
		if model == prefixed {
			return strings.TrimSuffix(strings.TrimSuffix(model, "-v1:0"), "-v1")
		}
	}
}

func (c *Catalog) Estimate(model, tier string, input, cached, output *int64) *float64 {
	key := normalize(model)
	if strings.HasSuffix(key, "-free") {
		return new(float64)
	}
	if input == nil || output == nil || *input < 0 || *output < 0 {
		return nil
	}
	var cachedCount int64
	if cached != nil {
		if *cached < 0 {
			return nil
		}
		cachedCount = *cached
	}
	current := c.exact[strings.ToLower(strings.TrimSpace(model))]
	if current.Input == nil || current.Output == nil {
		current = c.normalized[key]
	}

	base, hasFallback := fallback[key]
	if current.Input == nil || current.Output == nil {
		if !hasFallback {
			return nil
		}
		in, cache, out := base[0]/1e6, base[1]/1e6, base[2]/1e6
		current = rates{&in, &cache, &out}
	}
	cacheRate := current.Input
	if current.Cached != nil {
		cacheRate = current.Cached
	}
	cost := float64(max(0, *input-cachedCount))**current.Input +
		float64(cachedCount)**cacheRate + float64(*output)**current.Output
	if tier == "fast" || tier == "priority" {
		cost *= 2
	}
	return &cost
}

func (c *Catalog) Price(block *timeline.Block) {
	if len(block.UsageByModel) == 0 {
		block.EstimatedCostUSD = c.Estimate(block.Model, block.ServiceTier, block.InputTokens, block.CachedInputTokens, block.OutputTokens)
		return
	}
	var total *float64
	for i := range block.UsageByModel {
		row := &block.UsageByModel[i]
		row.EstimatedCostUSD = c.Estimate(row.Model, row.ServiceTier, row.InputTokens, row.CachedInputTokens, row.OutputTokens)
		if row.EstimatedCostUSD == nil {
			block.IncompleteCost = true
			continue
		}
		if total == nil {
			total = new(float64)
		}
		*total += *row.EstimatedCostUSD
	}
	block.EstimatedCostUSD = total
}
