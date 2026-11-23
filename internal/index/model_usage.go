package index

import (
	"encoding/json"
	"strings"

	"agent-timeline/internal/timeline"
)

type modelSettings struct {
	Model       string
	ServiceTier *string `json:"service_tier"`
}

func (m *source) applySettings(settings modelSettings) {
	if settings.Model != "" {
		m.Model = settings.Model
	}
	if settings.ServiceTier != nil {
		m.serviceTier = strings.ToLower(strings.TrimSpace(*settings.ServiceTier))
	}
}

func usageDelta(snapshot **int64, total, last *int64) *int64 {
	switch {
	case total != nil && *total >= 0:
		delta := *total
		if *snapshot != nil {
			// The rollout and token_count streams can report overlapping totals out of order.
			if *total <= **snapshot {
				delta = 0
				return &delta
			}
			delta -= **snapshot
		}
		*snapshot = total
		return &delta
	case last != nil && *last >= 0:
		if *snapshot != nil {
			next := **snapshot + *last
			*snapshot = &next
		} else {
			*snapshot = last
		}
		return last
	default:
		return nil
	}
}

func addCounter(dst **int64, delta *int64) {
	if delta != nil {
		var value int64
		if *dst != nil {
			value = **dst
		}
		value += *delta
		*dst = &value
	}
}

func (m *source) observeModelUsage(total, last usage) {
	stored := &m.usageSnapshot
	delta := usage{
		Total:     usageDelta(&stored.Total, total.Total, last.Total),
		Input:     usageDelta(&stored.Input, total.Input, last.Input),
		Cached:    usageDelta(&stored.Cached, total.Cached, last.Cached),
		Output:    usageDelta(&stored.Output, total.Output, last.Output),
		Reasoning: usageDelta(&stored.Reasoning, total.Reasoning, last.Reasoning),
	}
	addCounter(&m.Tokens, delta.Total)
	addCounter(&m.InputTokens, delta.Input)
	addCounter(&m.CachedInputTokens, delta.Cached)
	addCounter(&m.OutputTokens, delta.Output)
	addCounter(&m.ReasoningTokens, delta.Reasoning)
	if !delta.hasUsage() {
		return
	}
	tier := m.serviceTier
	if tier == "" || tier == "standard" {
		tier = "default"
	}
	for i := range m.usageByModel {
		row := &m.usageByModel[i]
		if row.Model == m.Model && row.ServiceTier == tier {
			addBreakdown(row, delta)
			return
		}
	}
	row := timeline.ModelUsage{Model: m.Model, ServiceTier: tier}
	addBreakdown(&row, delta)
	m.usageByModel = append(m.usageByModel, row)
}

func (u usage) hasUsage() bool {
	for _, value := range []*int64{u.Input, u.Cached, u.Output, u.Reasoning} {
		if value != nil && *value > 0 {
			return true
		}
	}
	return false
}

func addBreakdown(row *timeline.ModelUsage, delta usage) {
	addCounter(&row.InputTokens, delta.Input)
	addCounter(&row.CachedInputTokens, delta.Cached)
	addCounter(&row.OutputTokens, delta.Output)
	addCounter(&row.ReasoningTokens, delta.Reasoning)
}

func modelUsageJSON(rows []timeline.ModelUsage) (string, error) {
	if len(rows) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(rows)
	return string(data), err
}
