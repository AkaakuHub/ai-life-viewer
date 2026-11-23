package index

import (
	"database/sql"
	"strings"
	"testing"
)

func TestClaudeUsageKeepsOneSnapshotPerMessageAcrossAppends(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := initialize(db); err != nil {
		t.Fatal(err)
	}
	parts := []string{
		`{"timestamp":"2026-11-22T00:00:00Z","type":"assistant","message":{"id":"example","content":[{"type":"tool_use","id":"one"}],"usage":{"input_tokens":5,"cache_read_input_tokens":3,"cache_creation_input_tokens":2,"output_tokens":1}}}`,
		`{"timestamp":"2026-11-22T00:00:01Z","type":"assistant","message":{"id":"example","content":[{"type":"tool_use","id":"one"},{"type":"tool_use","id":"two"}],"usage":{"output_tokens":6}}}`,
	}
	for i, line := range parts {
		meta := source{Agent: "cl"}
		if _, _, err := parseLog(strings.NewReader(line+"\n"), &meta); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if err := saveSource(tx, "example", meta); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err := saveClaudeUsage(tx, "example", i > 0, meta.claudeMessages); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	var input, cached, output, tools int
	if err := db.QueryRow("SELECT input_tokens,cached_input_tokens,output_tokens,tool_call_count FROM sources").
		Scan(&input, &cached, &output, &tools); err != nil {
		t.Fatal(err)
	}
	if input != 10 || cached != 3 || output != 6 || tools != 2 {
		t.Fatalf("input=%d cached=%d output=%d tools=%d", input, cached, output, tools)
	}
}
