package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"

	"agent-timeline/internal/timeline"
)

func TestIngestPricesModelBreakdownAndPreservesItOnPartialUpdates(t *testing.T) {
	_, _, handler := ingestTestHandler(t)
	const start = 1795400400000
	submit := func(extra string, want int) {
		t.Helper()
		body := fmt.Sprintf(`{"records":[{"agent":"cx","session":"mac","title":"External","blocks":[{"start":%d,"end":%d}]%s}]}`, start, start+60000, extra)
		w := request(handler, http.MethodPost, "/api/v1/ingest", body, testToken)
		if w.Code != want {
			t.Fatalf("ingest status=%d: %s", w.Code, w.Body.String())
		}
	}
	submit(`,"serviceTier":"turbo"`, 400)
	submit(`,"usageByModel":[{"model":"gpt-6.2-sol","inputTokens":10}]`, 400)
	submit(`,"usageByModel":[
		{"model":"gpt-6.2-sol","inputTokens":1000000,"outputTokens":100000},
		{"model":"zai/glm-4.5-air","serviceTier":"fast","inputTokens":1000000,"outputTokens":100000}
	]`, 200)
	submit("", 200)
	w := request(handler, http.MethodGet, fmt.Sprintf("/api/blocks?start=%d&end=%d", start, start+60000), "", "")
	var result struct{ Blocks []timeline.CalendarBlock }
	if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil || len(result.Blocks) != 1 {
		t.Fatalf("read status=%d: %v %s", w.Code, err, w.Body.String())
	}
	block := result.Blocks[0]
	if block.ID >= 0 || len(block.UsageByModel) != 2 || block.InputTokens == nil || *block.InputTokens != 2_000_000 ||
		block.EstimatedCostUSD == nil || math.Abs(*block.EstimatedCostUSD-3.62) > 1e-12 ||
		block.UsageByModel[1].EstimatedCostUSD == nil || math.Abs(*block.UsageByModel[1].EstimatedCostUSD-.62) > 1e-12 {
		t.Fatalf("unexpected block: %+v", block)
	}
}
