package timeline

import (
	"sort"
	"time"
)

const Day = 24 * int64(time.Hour/time.Millisecond)

var JST = time.FixedZone("JST", 9*60*60)

type Block struct {
	Metrics
	EstimatedCostUSD *float64 `json:"estimatedCostUsd,omitempty"`
	Source           string   `json:"-"`
	ReasoningEffort  string   `json:"reasoningEffort,omitempty"`
	GitSHA           string   `json:"gitSha,omitempty"`
	GitOriginURL     string   `json:"gitOriginUrl,omitempty"`
	Provenance       string   `json:"provenance,omitempty"`
	Preview          string   `json:"preview,omitempty"`
	ID               int64    `json:"id"`
	Agent            string   `json:"agent"`
	Title            string   `json:"title"`
	Project          string   `json:"project"`
	CWD              string   `json:"cwd"`
	Model            string   `json:"model"`
	Branch           string   `json:"branch"`
	Tokens           *int64   `json:"tokens"`
	Start            int64    `json:"start"`
	End              int64    `json:"end"`
}

type DayTotal struct {
	Day int64 `json:"day"`
	MS  int64 `json:"ms"`
}

type Cost struct {
	EstimatedCostUSD *float64 `json:"estimatedCostUsd,omitempty"`
	UnpricedSessions int      `json:"unpricedSessions,omitempty"`
}

func (c *Cost) add(cost *float64) {
	if cost == nil {
		c.UnpricedSessions++
		return
	}
	if c.EstimatedCostUSD == nil {
		c.EstimatedCostUSD = new(float64)
	}
	*c.EstimatedCostUSD += *cost
}

type Group struct {
	Cost
	Name string `json:"name"`
	MS   int64  `json:"ms"`
}

type Summary struct {
	Metrics
	Cost
	Total    int64      `json:"totalMs"`
	Agents   []Group    `json:"agents"`
	Projects []Group    `json:"projects"`
	Days     []DayTotal `json:"days"`
}

func Summarize(blocks []Block, start, end int64) Summary {
	summary := Summary{Days: []DayTotal{}}
	agents := make(map[string]*Group)
	projects := make(map[string]*Group)
	seen := make(map[string]bool)
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
		agent, project := group(agents, block.Agent), group(projects, block.Project)
		agent.MS += ms
		project.MS += ms
		if !seen[block.Source] {
			summary.Metrics.add(block.Metrics)
			summary.Cost.add(block.EstimatedCostUSD)
			agent.Cost.add(block.EstimatedCostUSD)
			project.Cost.add(block.EstimatedCostUSD)
			seen[block.Source] = true
		}
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

func group(totals map[string]*Group, name string) *Group {
	if totals[name] == nil {
		totals[name] = &Group{Name: name}
	}
	return totals[name]
}

func groups(totals map[string]*Group) []Group {
	out := make([]Group, 0, len(totals))
	for _, total := range totals {
		out = append(out, *total)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MS == out[j].MS {
			return out[i].Name < out[j].Name
		}
		return out[i].MS > out[j].MS
	})
	return out
}
