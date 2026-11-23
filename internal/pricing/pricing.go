package pricing

import "strings"

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
	"gpt-6.2-sol":       {2, .10, 10},
	"gpt-6.1-sol":       {2, .10, 10},
	"gpt-6-sol":         {2, .20, 10},
	"gpt-6-luna":        {.10, .01, .50},
	"gpt-6-astra":       {10, 1, 50},
	"gpt-6.2-astra":     {8, .40, 40},
	"gpt-5.6-sol":       {4, .40, 20},
	"gpt-5.6-luna":      {.20, .02, 1.20},
	"gpt-5.6-terra":     {2, .20, 12},
	"gemini-4-neon":     {3, .15, 15},
	"claude-opus-5-5":   {4, .20, 20},
	"claude-sonnet-5-5": {2, .10, 10},
	"claude-haiku-5-5":  {.10, .01, .50},
}

func normalize(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	for {
		prefixed := model
		for _, prefix := range []string{
			"openai/", "openai.", "azure_ai/", "azure/", "perplexity/", "gemini/",
			"anthropic.", "anthropic/", "google/", "vertex_ai/", "openrouter/",
		} {
			if suffix, ok := strings.CutPrefix(model, prefix); ok {
				model = suffix
				break
			}
		}
		if model == prefixed {
			return model
		}
	}
}

func (c *Catalog) Estimate(model string, input, cached, output *int64) *float64 {
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
	key := normalize(model)
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
	if current.Cached == nil && cachedCount > 0 {
		if !hasFallback {
			return nil
		}
		cache := base[1] / 1e6
		current.Cached = &cache
	}
	cost := float64(max(0, *input-cachedCount))**current.Input + float64(*output)**current.Output
	if current.Cached != nil {
		cost += float64(cachedCount) * *current.Cached
	}
	return &cost
}
