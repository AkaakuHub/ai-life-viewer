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
