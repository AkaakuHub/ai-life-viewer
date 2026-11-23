package timeline

import (
	"testing"
	"time"
)

func TestHistorySplitsTimeAndCountsSessionTotalsOnlyAtFirstActivity(t *testing.T) {
	jan := time.Date(2027, 1, 1, 0, 0, 0, 0, JST).UnixMilli()
	dec := time.Date(2026, 12, 1, 0, 0, 0, 0, JST).UnixMilli()
	hour := Day / 24
	price, tokens := 2.5, int64(40)
	blocks := []Block{
		{Source: "one", Session: "shared", Agent: "cx", Start: jan + hour, End: jan + 2*hour, EstimatedCostUSD: &price, Tokens: &tokens},
		{Source: "one", Session: "shared", Agent: "cx", Start: jan - hour, End: jan + hour, EstimatedCostUSD: &price, Tokens: &tokens},
		{Source: "two", Session: "shared", Agent: "cx", Start: jan + 2*hour, End: jan + 3*hour},
	}
	start, end := jan-hour, jan+3*hour
	summary := Summarize(blocks, start, end)
	if summary.SessionCount != 1 || summary.Total != 4*hour {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	history := SummarizeHistory(start, blocks, summary.Days, map[string]int64{"2027-01-01": 100})
	if len(history.Months) != 2 || len(history.Years) != 2 {
		t.Fatalf("unexpected periods: %+v", history)
	}
	before, after := history.Months[0], history.Months[1]
	if before.Start != dec || before.MS != hour || before.Tokens == nil || *before.Tokens != 40 ||
		before.EstimatedCostUSD == nil || *before.EstimatedCostUSD != 2.5 || before.AccountTokens != nil {
		t.Fatalf("unexpected December totals: %+v", before)
	}
	if after.Start != jan || after.MS != 3*hour || after.Tokens != nil ||
		after.EstimatedCostUSD != nil || after.AccountTokens == nil || *after.AccountTokens != 100 {
		t.Fatalf("unexpected January totals: %+v", after)
	}
}
