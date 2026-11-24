package timeline

import (
	"slices"
	"testing"
	"time"
)

func TestSummarizeClipsAndSplitsConcurrentWorkAtMidnight(t *testing.T) {
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, JST).UnixMilli()
	hour := Day / 24
	result := Summarize([]Block{
		{Agent: "cx", Project: "a", PCIDs: []string{"work-pc"}, Start: day - 3*hour, End: day + hour},
		{Agent: "cl", Project: "a", PCIDs: []string{"home-pc", "work-pc"}, Start: day - hour, End: day + 3*hour},
	}, day-2*hour, day+2*hour)
	if result.Total != 6*hour || len(result.Days) != 2 ||
		result.Days[0] != (DayTotal{Day: day - Day, MS: 3 * hour}) ||
		result.Days[1] != (DayTotal{Day: day, MS: 3 * hour}) ||
		len(result.Agents) != 2 || result.Agents[0].MS != 3*hour ||
		len(result.Projects) != 1 || (result.Projects[0].Name != "a" || result.Projects[0].MS != 6*hour) {
		t.Fatalf("unexpected summary: %+v", result)
	}
	if !slices.Equal(result.Projects[0].PCIDs, []string{"home-pc", "work-pc"}) {
		t.Fatalf("unexpected project PCs: %v", result.Projects[0].PCIDs)
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

func TestSummaryCountsKnownAndUnknownPricesOncePerSession(t *testing.T) {
	cost := 1.25
	free := 0.0
	out := Summarize([]Block{
		{Source: "a", Agent: "cx", Project: "one", Start: 1, End: 4, EstimatedCostUSD: &cost},
		{Source: "a", Agent: "cx", Project: "one", Start: 5, End: 8, EstimatedCostUSD: &cost},
		{Source: "b", Agent: "cl", Project: "two", Start: 6, End: 9},
		{Source: "c", Agent: "ag", Project: "two", Start: 7, End: 9, EstimatedCostUSD: &free},
	}, 0, 10)
	if out.EstimatedCostUSD == nil || *out.EstimatedCostUSD != cost || out.UnpricedSessions != 1 ||
		out.Agents[0].EstimatedCostUSD == nil || *out.Agents[0].EstimatedCostUSD != cost ||
		out.Projects[1].EstimatedCostUSD == nil || *out.Projects[1].EstimatedCostUSD != 0 || out.Projects[1].UnpricedSessions != 1 {
		t.Fatalf("unexpected costs: %+v", out)
	}
}
