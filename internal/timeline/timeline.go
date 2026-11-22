package timeline

import (
	"sort"
	"time"
)

const Day = 24 * int64(time.Hour/time.Millisecond)

var JST = time.FixedZone("JST", 9*60*60)

type Block struct {
	ID      int64  `json:"id"`
	Agent   string `json:"agent"`
	Title   string `json:"title"`
	Project string `json:"project"`
	CWD     string `json:"cwd"`
	Model   string `json:"model"`
	Branch  string `json:"branch"`
	Tokens  *int64 `json:"tokens"`
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
}

type DayTotal struct {
	Day int64 `json:"day"`
	MS  int64 `json:"ms"`
}

type Group struct {
	Name string `json:"name"`
	MS   int64  `json:"ms"`
}

type Summary struct {
	Total    int64      `json:"totalMs"`
	Agents   []Group    `json:"agents"`
	Projects []Group    `json:"projects"`
	Days     []DayTotal `json:"days"`
}

func Summarize(blocks []Block, start, end int64) Summary {
	summary := Summary{Days: []DayTotal{}}
	agents := make(map[string]int64)
	projects := make(map[string]int64)
	d := time.UnixMilli(start).In(JST)
	first := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, JST).UnixMilli()
	for day := first; day < end; day += Day {
		summary.Days = append(summary.Days, DayTotal{Day: day})
	}
	for _, block := range blocks {
		from, to := max(start, block.Start), min(end, block.End)
		if from >= to {
			continue
		}
		ms := to - from
		summary.Total += ms
		agents[block.Agent] += ms
		projects[block.Project] += ms
		for i := range summary.Days {
			day := &summary.Days[i]
			if day.Day >= to {
				break
			}
			day.MS += max(0, min(day.Day+Day, to)-max(day.Day, from))
		}
	}
	summary.Agents, summary.Projects = groups(agents), groups(projects)
	return summary
}

func groups(totals map[string]int64) []Group {
	out := make([]Group, 0, len(totals))
	for name, ms := range totals {
		out = append(out, Group{name, ms})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MS == out[j].MS {
			return out[i].Name < out[j].Name
		}
		return out[i].MS > out[j].MS
	})
	return out
}
