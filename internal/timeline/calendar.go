package timeline

import (
	"cmp"
	"slices"
	"time"
)

type CalendarBlock struct {
	Block
	ActiveMS int64 `json:"activeMs"`
	WallMS   int64 `json:"wallMs"`
}

type sessionKey struct {
	agent, session, source string
}

func (b Block) sessionKey() sessionKey {
	if b.Session != "" {
		return sessionKey{agent: b.Agent, session: b.Session}
	}
	return sessionKey{agent: b.Agent, source: b.Source}
}

func midnight(ms int64) int64 {
	d := time.UnixMilli(ms).In(JST)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, JST).UnixMilli()
}

func UnionSessions(blocks []Block) []Block {
	slices.SortStableFunc(blocks, func(a, b Block) int { return cmp.Compare(a.Start, b.Start) })
	last := make(map[sessionKey]int)
	out := blocks[:0]
	for _, block := range blocks {
		key := block.sessionKey()
		if i, ok := last[key]; ok && block.Start <= out[i].End {
			out[i].End = max(out[i].End, block.End)
			continue
		}
		last[key] = len(out)
		out = append(out, block)
	}
	return out
}

func CalendarBlocks(intervals []Block, start, end int64) []CalendarBlock {
	type sessionDay struct {
		sessionKey
		day int64
	}
	groups := make(map[sessionDay]int)
	out := []CalendarBlock{}
	for _, block := range intervals {
		from, to := max(block.Start, start), min(block.End, end)
		if from >= to {
			continue
		}
		for day := midnight(from); day < to; day += Day {
			first, last := max(day, from), min(day+Day, to)
			key := sessionDay{block.sessionKey(), day}
			i, exists := groups[key]
			if !exists {
				groups[key] = len(out)
				part := CalendarBlock{Block: block, ActiveMS: last - first, WallMS: last - first}
				part.Start, part.End = first, last
				out = append(out, part)
				continue
			}
			group := &out[i]
			group.Start = min(group.Start, first)
			group.End = max(group.End, last)
			group.ActiveMS += last - first
			group.WallMS = group.End - group.Start
		}
	}
	slices.SortStableFunc(out, func(a, b CalendarBlock) int { return cmp.Compare(a.Start, b.Start) })
	return out
}
