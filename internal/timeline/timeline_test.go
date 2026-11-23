package timeline

import (
	"testing"
	"time"
)

func TestSummarizeClipsAndSplitsConcurrentWorkAtMidnight(t *testing.T) {
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, JST).UnixMilli()
	hour := Day / 24
	result := Summarize([]Block{
		{Agent: "cx", Project: "a", Start: day - 3*hour, End: day + hour},
		{Agent: "cl", Project: "a", Start: day - hour, End: day + 3*hour},
	}, day-2*hour, day+2*hour)
	if result.Total != 6*hour || len(result.Days) != 2 ||
		result.Days[0] != (DayTotal{Day: day - Day, MS: 3 * hour}) ||
		result.Days[1] != (DayTotal{Day: day, MS: 3 * hour}) ||
		len(result.Agents) != 2 || result.Agents[0].MS != 3*hour ||
		len(result.Projects) != 1 || result.Projects[0] != (Group{Name: "a", MS: 6 * hour}) {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestSummarizeCountsEachSessionOnce(t *testing.T) {
	input, turns, tools := int64(12), int64(2), int64(0)
	metrics := Metrics{InputTokens: &input, TurnCount: &turns, ToolCallCount: &tools}
	out := Summarize([]Block{
		{Source: "a", Start: 1, End: 4, Metrics: metrics},
		{Source: "a", Start: 5, End: 8, Metrics: metrics},
		{Source: "b", Start: 7, End: 9, Metrics: metrics},
		{Source: "c", Start: 10, End: 11, Metrics: metrics},
	}, 0, 10)
	if out.Total != 8 || out.InputTokens == nil || *out.InputTokens != 24 ||
		out.TurnCount == nil || *out.TurnCount != 4 ||
		out.ToolCallCount == nil || *out.ToolCallCount != 0 || out.OutputTokens != nil {
		t.Fatalf("unexpected summary: %+v", out)
	}
}
