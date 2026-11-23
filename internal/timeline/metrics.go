package timeline

type Metrics struct {
	InputTokens       *int64 `json:"inputTokens,omitempty"`
	CachedInputTokens *int64 `json:"cachedInputTokens,omitempty"`
	OutputTokens      *int64 `json:"outputTokens,omitempty"`
	ReasoningTokens   *int64 `json:"reasoningTokens,omitempty"`
	TurnCount         *int64 `json:"turnCount,omitempty"`
	ToolCallCount     *int64 `json:"toolCallCount,omitempty"`
}

func (m *Metrics) add(other Metrics) {
	add := func(total **int64, value *int64) {
		if value != nil {
			sum := *value
			if *total != nil {
				sum += **total
			}
			*total = &sum
		}
	}
	add(&m.InputTokens, other.InputTokens)
	add(&m.CachedInputTokens, other.CachedInputTokens)
	add(&m.OutputTokens, other.OutputTokens)
	add(&m.ReasoningTokens, other.ReasoningTokens)
	add(&m.TurnCount, other.TurnCount)
	add(&m.ToolCallCount, other.ToolCallCount)
}
