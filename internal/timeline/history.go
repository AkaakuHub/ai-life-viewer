package timeline

import (
	"sort"
	"time"
)

type PeriodTotal struct {
	Cost
	Start         int64  `json:"start"`
	MS            int64  `json:"ms"`
	Tokens        *int64 `json:"tokens,omitempty"`
	AccountTokens *int64 `json:"accountTokens,omitempty"`
}

type History struct {
	Start  int64         `json:"start"`
	Months []PeriodTotal `json:"months"`
	Years  []PeriodTotal `json:"years"`
}

func SummarizeHistory(start int64, blocks []Block, days []DayTotal, account map[string]int64) History {
	months := make(map[int64]*PeriodTotal)
	years := make(map[int64]*PeriodTotal)
	for _, day := range days {
		d := time.UnixMilli(day.Day).In(JST)
		month, year := periodTotals(months, years, d)
		month.MS += day.MS
		year.MS += day.MS
		if tokens, ok := account[d.Format(time.DateOnly)]; ok {
			addTokens(&month.AccountTokens, tokens)
			addTokens(&year.AccountTokens, tokens)
		}
	}
	first := make(map[sessionKey]Block)
	for _, block := range blocks {
		key := block.sessionKey()
		if earlier, exists := first[key]; !exists || block.Start < earlier.Start {
			first[key] = block
		}
	}
	for _, block := range first {
		month, year := periodTotals(months, years, time.UnixMilli(block.Start).In(JST))
		tokens := block.Tokens
		if tokens == nil && block.InputTokens != nil && block.OutputTokens != nil {
			sum := *block.InputTokens + *block.OutputTokens
			tokens = &sum
		}
		for _, period := range []*PeriodTotal{month, year} {
			period.Cost.add(block.EstimatedCostUSD, block.IncompleteCost)
			if tokens != nil {
				addTokens(&period.Tokens, *tokens)
			}
		}
	}
	return History{Start: start, Months: orderedPeriods(months), Years: orderedPeriods(years)}
}

func periodTotals(months, years map[int64]*PeriodTotal, d time.Time) (*PeriodTotal, *PeriodTotal) {
	entry := func(totals map[int64]*PeriodTotal, month time.Month) *PeriodTotal {
		key := time.Date(d.Year(), month, 1, 0, 0, 0, 0, JST).UnixMilli()
		if totals[key] == nil {
			totals[key] = &PeriodTotal{Start: key}
		}
		return totals[key]
	}
	return entry(months, d.Month()), entry(years, time.January)
}

func addTokens(total **int64, tokens int64) {
	if *total == nil {
		*total = new(int64)
	}
	**total += tokens
}

func orderedPeriods(totals map[int64]*PeriodTotal) []PeriodTotal {
	out := make([]PeriodTotal, 0, len(totals))
	for _, p := range totals {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}
