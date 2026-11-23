package timeline

import (
	"testing"
	"time"
)

func TestDailyCalendarExcludesSessionGapsAndSplitsAtTokyoMidnight(t *testing.T) {
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, JST).UnixMilli()
	hour := Day / 24
	intervals := UnionSessions([]Block{
		{Agent: "cx", Session: "one", Source: "rollout", Start: day - hour, End: day + hour},
		{Agent: "cx", Session: "one", Source: "t3", Start: day, End: day + hour/2},
		{Agent: "cx", Session: "one", Source: "rollout", Start: day + 10*hour, End: day + 11*hour},
		{Agent: "cl", Session: "one", Source: "claude", Start: day + hour, End: day + 2*hour},
		{Agent: "cx", Session: "one", Source: "rollout", Start: day + 2*Day, End: day + 2*Day + hour},
	})
	start, end := day-hour/2, day+3*Day
	calendar := CalendarBlocks(intervals, start, end)
	if len(calendar) != 4 {
		t.Fatalf("daily blocks = %+v", calendar)
	}
	previous, current := calendar[0], calendar[1]
	if previous.Start != start || previous.End != day || previous.ActiveMS != hour/2 ||
		current.Start != day || current.End != day+11*hour || current.ActiveMS != 2*hour || current.WallMS != 11*hour ||
		calendar[2].Agent != "cl" || calendar[3].Start != day+2*Day {
		t.Fatalf("unexpected daily blocks: %+v", calendar)
	}
	summary := Summarize(intervals, start, end)
	if summary.Total != 4*hour+hour/2 || summary.Days[1].MS != 3*hour || summary.Days[2].MS != 0 || summary.SessionCount != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
