package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexesModelAndTierAcrossAppendedRollout(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "rollout.jsonl")
	appendLog := func(log string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString(log)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("write=%v close=%v", err, closeErr)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.indexFile(path, "cx", info); err != nil {
			t.Fatal(err)
		}
	}
	appendLog(`{"timestamp":"2026-11-22T00:00:00Z","type":"session_meta","payload":{"id":"sample"}}
{"timestamp":"2026-11-22T00:00:01Z","type":"turn_context","payload":{"model":"first"}}
{"timestamp":"2026-11-22T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":10,"reasoning_output_tokens":2}}}}
`)
	appendLog(`{"timestamp":"2026-11-22T00:05:00Z","type":"event_msg","payload":{"type":"thread_settings_applied","thread_settings":{"model":"second","service_tier":"priority"}}}
{"timestamp":"2026-11-22T00:05:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":10,"reasoning_output_tokens":2},"last_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":10,"reasoning_output_tokens":2}}}}
{"timestamp":"2026-11-22T00:05:02Z","type":"token_usage_record","payload":{"thread_token_usage":{"input_tokens":125,"cached_input_tokens":55,"output_tokens":15,"reasoning_output_tokens":3}}}
{"timestamp":"2026-11-22T00:05:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":52,"output_tokens":13,"reasoning_output_tokens":2},"last_token_usage":{"input_tokens":20,"cached_input_tokens":2,"output_tokens":3}}}}
`)
	appendLog(`{"timestamp":"2026-11-22T00:05:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":25,"cached_input_tokens":5,"output_tokens":5,"reasoning_output_tokens":1}}}}
{"timestamp":"2026-11-22T00:05:04Z","type":"event_msg","payload":{"type":"task_complete","duration_ms":4000}}
`)
	blocks, err := store.Blocks(0, 2_000_000_000_000, Options{}, "")
	if err != nil || len(blocks) != 1 {
		t.Fatalf("blocks=%v err=%v", blocks, err)
	}
	block := blocks[0]
	if block.Model != "second" || block.ServiceTier != "priority" || block.InputTokens == nil ||
		*block.InputTokens != 150 || len(block.UsageByModel) != 2 {
		t.Fatalf("unexpected usage: %+v", block)
	}
	first, second := block.UsageByModel[0], block.UsageByModel[1]
	if first.Model != "first" || first.ServiceTier != "default" ||
		first.InputTokens == nil || *first.InputTokens != 100 ||
		second.Model != "second" || second.ServiceTier != "priority" ||
		second.InputTokens == nil || *second.InputTokens != 50 ||
		second.OutputTokens == nil || *second.OutputTokens != 10 {
		t.Fatalf("unexpected breakdown: %+v", block.UsageByModel)
	}
}
