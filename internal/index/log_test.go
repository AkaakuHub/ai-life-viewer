package index

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogResumesAtIncompleteLine(t *testing.T) {
	meta := source{Agent: "cx"}
	first := `{"timestamp":"2026-11-22T00:00:00Z","type":"session_meta","payload":{"id":"abc","cwd":"/projects/example"}}` + "\n"
	event := `{"timestamp":"2026-11-22T00:01:00Z","type":"event_msg","payload":{"type":"task_complete","duration_ms":10000}}` + "\n"
	next := `{"timestamp":"2026-11-22T00:04:00Z","type":"event_msg","payload":{"type":"turn_aborted","duration_ms":20000}}` + "\n"
	initial := first + event + next[:30]
	spans, offset, err := parseLog(strings.NewReader(initial), &meta)
	if err != nil || offset != int64(len(first)+len(event)) || len(spans) != 1 || meta.Session != "abc" {
		t.Fatalf("initial parse: spans=%v offset=%d metadata=%+v err=%v", spans, offset, meta, err)
	}
	full := first + event + next
	events, consumed, err := parseLog(strings.NewReader(full[offset:]), &meta)
	if err != nil || offset+consumed != int64(len(full)) || len(events) != 1 {
		t.Fatalf("incremental parse: spans=%v consumed=%d metadata=%+v err=%v", events, consumed, meta, err)
	}
	merged := mergeTurnSpans(append(spans, events...))
	if len(merged) != 2 || merged[0].Last-merged[0].Start != 10_000 || merged[1].Last-merged[1].Start != 20_000 {
		t.Fatalf("expected measured durations across parse batches: %+v", merged)
	}
}

func TestClaudeOnlyCountsRecordedTurnDurations(t *testing.T) {
	meta := source{Agent: "cl"}
	data := `{"timestamp":"2026-11-22T00:00:00Z","type":"user","sessionId":"example"}` + "\n" +
		`{"timestamp":"2026-11-22T00:10:00Z","type":"system","subtype":"turn_duration","durationMs":1500}` + "\n"
	spans, _, err := parseLog(strings.NewReader(data), &meta)
	end := time.Date(2026, 11, 22, 0, 10, 0, 0, time.UTC).UnixMilli()
	if err != nil || len(spans) != 1 || spans[0] != (interval{end - 1500, end}) {
		t.Fatalf("spans=%v err=%v", spans, err)
	}
}

func TestCodexMetricsResumeWithoutAddingCumulativeSnapshots(t *testing.T) {
	meta := source{Agent: "cx"}
	initial := `{"timestamp":"2026-11-22T00:00:00Z","type":"event_msg","payload":{"type":"task_started"}}` + "\n" +
		`{"timestamp":"2026-11-22T00:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":4}}}}` + "\n" +
		`{"timestamp":"2026-11-22T00:02:00Z","type":"response_item","payload":{"type":"custom_tool_call","input":"content"}}` + "\n"
	appended := `{"timestamp":"2026-11-22T00:03:00Z","type":"response_item","payload":{"type":"custom_tool_call_output","output":"content"}}` + "\n" +
		`{"timestamp":"2026-11-22T00:04:00Z","type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":25,"output_tokens":8,"reasoning_output_tokens":6}}}` + "\n"
	for _, part := range []string{initial, appended} {
		if _, offset, err := parseLog(strings.NewReader(part), &meta); err != nil || offset != int64(len(part)) {
			t.Fatalf("offset=%d err=%v", offset, err)
		}
	}
	if meta.InputTokens == nil || *meta.InputTokens != 25 ||
		meta.CachedInputTokens == nil || *meta.CachedInputTokens != 0 ||
		meta.OutputTokens == nil || *meta.OutputTokens != 8 ||
		meta.ReasoningTokens == nil || *meta.ReasoningTokens != 6 ||
		meta.TurnCount == nil || *meta.TurnCount != 1 ||
		meta.ToolCallCount == nil || *meta.ToolCallCount != 1 {
		t.Fatalf("unexpected metrics: %+v", meta.Metrics)
	}
}

func TestShortPreviewCapsUnicodeCharacters(t *testing.T) {
	if n := len([]rune(shortPreview(strings.Repeat("語", 300)))); n != 240 {
		t.Fatalf("preview length=%d", n)
	}
}
