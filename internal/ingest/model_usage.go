package ingest

import (
	"fmt"
	"math"
	"strings"
)

const maxUsageModels = 1000

func validTier(tier string) bool {
	switch tier {
	case "", "default", "standard", "fast", "priority":
		return true
	}
	return false
}

func (r Record) validateModelUsage(path string) *Issue {
	if !validTier(r.ServiceTier) {
		return &Issue{path + ".serviceTier", "use default, standard, fast or priority"}
	}
	if len(r.UsageByModel) > maxUsageModels {
		return &Issue{path + ".usageByModel", "provide at most 1000 model usage entries"}
	}
	seen := make(map[[2]string]bool)
	for i, row := range r.UsageByModel {
		at := fmt.Sprintf("%s.usageByModel[%d]", path, i)
		if strings.TrimSpace(row.Model) == "" || len(row.Model) > 2048 || strings.TrimSpace(row.Model) != row.Model {
			return &Issue{at + ".model", "provide a nonblank model of at most 2048 bytes without surrounding whitespace"}
		}
		if !validTier(row.ServiceTier) {
			return &Issue{at + ".serviceTier", "use default, standard, fast or priority"}
		}
		tier := row.ServiceTier
		if tier == "" || tier == "standard" {
			tier = "default"
		}
		key := [2]string{strings.ToLower(row.Model), tier}
		if seen[key] {
			return &Issue{at, "duplicate model and service tier"}
		}
		seen[key] = true
		for _, metric := range []struct {
			name     string
			value    *int64
			required bool
		}{
			{"inputTokens", row.InputTokens, true},
			{"cachedInputTokens", row.CachedInputTokens, false},
			{"outputTokens", row.OutputTokens, true},
			{"reasoningTokens", row.ReasoningTokens, false},
		} {
			if (metric.required && metric.value == nil) || (metric.value != nil && *metric.value < 0) {
				return &Issue{at + "." + metric.name, "provide a nonnegative integer"}
			}
		}
		if row.EstimatedCostUSD != nil {
			return &Issue{at + ".estimatedCostUsd", "cost is calculated by the server"}
		}
	}
	if _, ok := r.ModelUsageTotals(); !ok {
		return &Issue{path + ".usageByModel", "session token totals exceed the supported integer range"}
	}
	return nil
}

func (r Record) ModelUsageTotals() (Tokens, bool) {
	var totals Tokens
	for _, row := range r.UsageByModel {
		for _, metric := range []struct {
			dst   **int64
			value *int64
		}{
			{&totals.Input, row.InputTokens}, {&totals.CachedInput, row.CachedInputTokens},
			{&totals.Output, row.OutputTokens}, {&totals.Reasoning, row.ReasoningTokens},
			{&totals.Total, row.InputTokens}, {&totals.Total, row.OutputTokens},
		} {
			if metric.value == nil {
				continue
			}
			if *metric.dst == nil {
				*metric.dst = new(int64)
			}
			if **metric.dst > math.MaxInt64-*metric.value {
				return Tokens{}, false
			}
			**metric.dst += *metric.value
		}
	}
	return totals, true
}
