package index

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogResumesAtIncompleteLine(t *testing.T) {
	meta := source{Agent: "cx"}
	first := `{"timestamp":"2026-11-22T00:00:00Z","type":"session_meta","payload":{"id":"abc","cwd":"/projects/example"}}` + "\n"
	event := `{"timestamp":"2026-11-22T00:01:00Z","type":"response_item","payload":{}}` + "\n"
	next := `{"timestamp":"2026-11-22T00:04:00Z","type":"turn_context","payload":{"model":"test-model"}}` + "\n"
	initial := first + event + next[:30]
	spans, offset, err := parseLog(strings.NewReader(initial), &meta)
	if err != nil || offset != int64(len(first)+len(event)) || len(spans) != 1 || meta.Session != "abc" {
		t.Fatalf("initial parse: spans=%v offset=%d metadata=%+v err=%v", spans, offset, meta, err)
	}
	full := first + event + next
	events, consumed, err := parseLog(strings.NewReader(full[offset:]), &meta)
	if err != nil || offset+consumed != int64(len(full)) || len(events) != 1 || meta.Model != "test-model" {
		t.Fatalf("incremental parse: spans=%v consumed=%d metadata=%+v err=%v", events, consumed, meta, err)
	}
	merged := mergeIntervals(append(spans, events...))
	if len(merged) != 1 || merged[0].end()-merged[0].Start != 3*minute {
		t.Fatalf("append should extend the current block: %+v", merged)
	}
}

func TestMergeIntervalsUsesRawGapAndOneMinuteMinimum(t *testing.T) {
	base := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC).UnixMilli()
	out := mergeIntervals([]interval{
		{base + 11*minute, base + 11*minute}, {base, base}, {base + 5*minute, base + 5*minute},
	})
	if len(out) != 2 || out[0].end() != base+5*minute || out[1].end() != base+12*minute {
		t.Fatalf("unexpected work intervals: %+v", out)
	}
}
