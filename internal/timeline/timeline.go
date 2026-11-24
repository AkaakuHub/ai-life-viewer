package timeline

import (
	"sort"
	"time"
)

const Day = 24 * int64(time.Hour/time.Millisecond)

var JST = time.FixedZone("JST", 9*60*60)

type ModelUsage struct {
	Model             string   `json:"model"`
	ServiceTier       string   `json:"serviceTier,omitempty"`
	InputTokens       *int64   `json:"inputTokens,omitempty"`
	CachedInputTokens *int64   `json:"cachedInputTokens,omitempty"`
	OutputTokens      *int64   `json:"outputTokens,omitempty"`
	ReasoningTokens   *int64   `json:"reasoningTokens,omitempty"`
	EstimatedCostUSD  *float64 `json:"estimatedCostUsd,omitempty"`
}

type Block struct {
	Metrics
	PCIDs            []string     `json:"pcIds"`
	UsageByModel     []ModelUsage `json:"usageByModel,omitempty"`
	ServiceTier      string       `json:"serviceTier,omitempty"`
	IncompleteCost   bool         `json:"incompleteCost,omitempty"`
	EstimatedCostUSD *float64     `json:"estimatedCostUsd,omitempty"`
	Source           string       `json:"-"`
	Session          string       `json:"session,omitempty"`
	ReasoningEffort  string       `json:"reasoningEffort,omitempty"`
	GitSHA           string       `json:"gitSha,omitempty"`
	GitOriginURL     string       `json:"gitOriginUrl,omitempty"`
	Provenance       string       `json:"provenance,omitempty"`
	Preview          string       `json:"preview,omitempty"`
	ID               int64        `json:"id"`
	Agent            string       `json:"agent"`
	Title            string       `json:"title"`
	Project          string       `json:"project"`
	CWD              string       `json:"cwd"`
	Model            string       `json:"model"`
	Branch           string       `json:"branch"`
	Tokens           *int64       `json:"tokens"`
	Start            int64        `json:"start"`
	End              int64        `json:"end"`
}

type DayTotal struct {
	Day int64 `json:"day"`
	MS  int64 `json:"ms"`
}

type Cost struct {
	EstimatedCostUSD *float64 `json:"estimatedCostUsd,omitempty"`
	UnpricedSessions int      `json:"unpricedSessions,omitempty"`
}

func (c *Cost) add(cost *float64, incomplete bool) {
	if cost == nil || incomplete {
		c.UnpricedSessions++
	}
	if cost == nil {
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
	Total        int64      `json:"totalMs"`
	SessionCount int        `json:"sessionCount"`
	Agents       []Group    `json:"agents"`
	Projects     []Group    `json:"projects"`
	Days         []DayTotal `json:"days"`
}

func Summarize(blocks []Block, start, end int64) Summary {
	summary := Summary{Days: []DayTotal{}}
	agents := make(map[string]*Group)
	projects := make(map[string]*Group)
	seen := make(map[sessionKey]bool)
	first := midnight(start)
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
		key := block.sessionKey()
		if !seen[key] {
			summary.Metrics.add(block.Metrics)
			summary.Cost.add(block.EstimatedCostUSD, block.IncompleteCost)
			agent.Cost.add(block.EstimatedCostUSD, block.IncompleteCost)
			project.Cost.add(block.EstimatedCostUSD, block.IncompleteCost)
			seen[key] = true
		}
		for i := int((from - first) / Day); i < len(summary.Days); i++ {
			day := &summary.Days[i]
			if day.Day >= to {
				break
			}
			day.MS += max(0, min(day.Day+Day, to)-max(day.Day, from))
		}
	}
	summary.Agents, summary.Projects = groups(agents), groups(projects)
	summary.SessionCount = len(seen)
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
