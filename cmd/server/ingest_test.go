package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-timeline/internal/index"
	"agent-timeline/internal/ingest"
	"agent-timeline/internal/pricing"
	"agent-timeline/internal/timeline"
)

const testToken = "fixture-token"

func ingestTestHandler(t *testing.T) (*index.Store, string, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "index.sqlite")
	store, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path, routes(store, pricing.Load(filepath.Join(dir, "missing.json")),
		ingest.NewAuth(testToken), nil)
}

func request(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestIngestRejectsUnauthorizedAndInvalidBatch(t *testing.T) {
	_, _, handler := ingestTestHandler(t)
	for _, tc := range []struct {
		token, body, field string
		status             int
	}{
		{"wrong", "{}", "Authorization", 401},
		{testToken, "{}", "pcId", 400},
		{testToken, `{"pcId":"test-pc","records":[{"agent":"cx","session":"x","title":"x","blocks":[{"start":1795400400,"end":1795400700}]}]}`, "records[0].blocks[0].start", 400},
		{testToken, `{"pcId":"test-pc","dailyUsage":[{"agent":"cx","day":"2026-11-23"}]}`, "dailyUsage[0].tokens", 400},
	} {
		w := request(handler, http.MethodPost, "/api/v1/ingest", tc.body, tc.token)
		var problem ingest.Issue
		err := json.Unmarshal(w.Body.Bytes(), &problem)
		if w.Code != tc.status || err != nil || problem.Field != tc.field || problem.Message == "" {
			t.Fatalf("status=%d problem=%+v err=%v", w.Code, problem, err)
		}
	}
}

func TestIngestMergesExistingSessionAndUpdatesWithoutDuplicates(t *testing.T) {
	_, path, handler := ingestTestHandler(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const start = 1795400400000
	if _, err := db.Exec(`
		INSERT INTO sources(key,agent,session,title,project,cwd,model,branch,tokens)
			VALUES('/logs/codex.jsonl','cx','shared','Local','local','/local','local-model','main',20);
		INSERT INTO files VALUES('/logs/codex.jsonl',1,10,10,1,1,1);
		INSERT INTO blocks(source,start,last,end) VALUES('/logs/codex.jsonl',?, ?, ?);
		INSERT INTO sources(key,agent,session,title,project,cwd,model,branch,input_tokens)
			VALUES('session:cx:shared','cx','shared','Prior upload','','','','',100);
		INSERT INTO session_intervals(source,start,end) VALUES('session:cx:shared',?,?);
		INSERT INTO daily_usage VALUES('cx','2026-11-23',30,1);
	`, start, start+60000, start+60000, start+30000, start+75000); err != nil {
		t.Fatal(err)
	}
	read := func() ([]timeline.CalendarBlock, []ingest.DailyUsage) {
		t.Helper()
		w := request(handler, http.MethodGet, fmt.Sprintf("/api/blocks?start=%d&end=%d", start, start+500000), "", "")
		var result struct {
			Blocks       []timeline.CalendarBlock
			DailyUsage   []ingest.DailyUsage
			AccountUsage index.AccountUsage
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); w.Code != 200 || err != nil {
			t.Fatalf("GET blocks status=%d: %v (%s)", w.Code, err, w.Body.String())
		}
		if len(result.AccountUsage.Days) != 1 || result.AccountUsage.Days[0].Tokens != 40 {
			t.Fatalf("account usage not merged: %+v", result.AccountUsage)
		}
		return result.Blocks, result.DailyUsage
	}
	submit := func(pcID string, end int64) {
		t.Helper()
		body := fmt.Sprintf(`{"pcId":%q,"records":[{"agent":"cx","session":"shared","title":"Remote",
			"tokens":{"input":150},"blocks":[{"start":%d,"end":%d},{"start":%d,"end":%d}]}],
			"dailyUsage":[{"agent":"cx","day":"2026-11-23","tokens":40}]}`,
			pcID, start+30000, start+90000, start+120000, end)
		w := request(handler, http.MethodPost, "/api/v1/ingest", body, testToken)
		if w.Code != 200 {
			t.Fatalf("ingest status=%d: %s", w.Code, w.Body.String())
		}
	}
	submit("test-pc", start+180000)
	first, daily := read()
	if len(first) != 1 || first[0].Start != start || first[0].End != start+180000 ||
		first[0].ActiveMS != 150000 || first[0].WallMS != 180000 ||
		first[0].Session != "shared" || first[0].Title != "Remote" || first[0].Project != "local" ||
		first[0].InputTokens == nil || *first[0].InputTokens != 150 || len(daily) != 1 || daily[0].Tokens == nil || *daily[0].Tokens != 40 {
		t.Fatalf("unexpected merge: %+v %+v", first, daily)
	}
	submit("second-pc", start+240000)
	statusResponse := request(handler, http.MethodGet, "/api/status", "", "")
	var status index.Status
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if statusResponse.Code != http.StatusOK || status.ServerPCID == "" ||
		len(status.IngestPCs) != 2 || status.IngestPCs[1].ID != "test-pc" || status.IngestPCs[1].LastIngest <= 0 {
		t.Fatalf("unexpected PC status: %+v", status)
	}
	second, _ := read()
	if len(second) != 1 || second[0].ID != first[0].ID || second[0].End != start+240000 || second[0].ActiveMS != 210000 {
		t.Fatalf("unexpected upsert: %+v", second)
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, pcID := range []string{hostname, "second-pc", "test-pc"} {
		if !slices.Contains(second[0].PCIDs, pcID) {
			t.Fatalf("PC %q missing from %v", pcID, second[0].PCIDs)
		}
	}
	for _, filter := range []struct {
		query string
		want  int
	}{
		{"pcs=test-pc&pcs=second-pc", 1},
		{"pcs=unknown-pc", 0},
	} {
		url := fmt.Sprintf("/api/blocks?start=%d&end=%d&%s", start, start+500000, filter.query)
		w := request(handler, http.MethodGet, url, "", "")
		var result struct {
			Blocks       []timeline.CalendarBlock
			DailyUsage   []ingest.DailyUsage
			AccountUsage index.AccountUsage
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != http.StatusOK {
			t.Fatalf("filtered blocks status=%d: %v", w.Code, err)
		}
		if len(result.Blocks) != filter.want || len(result.DailyUsage) != 0 || len(result.AccountUsage.Days) != 0 {
			t.Fatalf("unexpected filtered result for %s: %+v", filter.query, result)
		}
	}
	if _, err := db.Exec("UPDATE blocks SET last=?,end=? WHERE source='/logs/codex.jsonl'", start+100000, start+100000); err != nil {
		t.Fatal(err)
	}
	third, _ := read()
	if len(third) != 1 || third[0].End != start+240000 || third[0].ActiveMS != 220000 {
		t.Fatalf("local refresh dropped imported data: %+v", third)
	}
}

func TestSchemaListsEndpointsAndValidExamples(t *testing.T) {
	_, _, handler := ingestTestHandler(t)
	w := request(handler, http.MethodGet, "/api/schema", "", "")
	var doc struct {
		Endpoints map[string]struct {
			Examples  map[string]json.RawMessage
			Responses map[string]json.RawMessage
		}
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); w.Code != 200 || err != nil {
		t.Fatalf("schema status=%d err=%v", w.Code, err)
	}
	for _, endpoint := range []string{"POST /api/v1/ingest", "GET /api/blocks", "GET /api/summary", "GET /api/filters", "GET /api/status", "GET /api/schema"} {
		if len(doc.Endpoints[endpoint].Responses["200"]) == 0 {
			t.Fatalf("missing %s response schema", endpoint)
		}
	}
	checkRefs(t, w.Body.Bytes(), doc.Defs)
	for _, name := range []string{"minimum", "complete"} {
		example := doc.Endpoints["POST /api/v1/ingest"].Examples[name]
		result := request(handler, http.MethodPost, "/api/v1/ingest", string(example), testToken)
		if result.Code != 200 {
			t.Fatalf("example %s status=%d: %s", name, result.Code, result.Body.String())
		}
	}
}

func checkRefs(t *testing.T, raw json.RawMessage, defs map[string]json.RawMessage) {
	t.Helper()
	switch raw[0] {
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatal(err)
		}
		if ref, ok := obj["$ref"]; ok {
			var path string
			if err := json.Unmarshal(ref, &path); err != nil {
				t.Fatal(err)
			}
			key, ok := strings.CutPrefix(path, "#/$defs/")
			if !ok || len(defs[key]) == 0 {
				t.Fatalf("unresolved reference %s", path)
			}
		}
		for _, child := range obj {
			checkRefs(t, child, defs)
		}
	case '[':
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatal(err)
		}
		for _, child := range values {
			checkRefs(t, child, defs)
		}
	}
}
