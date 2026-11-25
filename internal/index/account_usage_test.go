package index

import (
	"database/sql"
	"testing"
	"time"

	"agent-timeline/internal/timeline"
)

func TestStoredAccountUsageCoexistsWithDetailedBlocks(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := initialize(db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, timeline.JST)
	if _, err := db.Exec(`
		INSERT INTO daily_usage VALUES('cx','2026-06-01',123,1);
		INSERT INTO usage_summary VALUES(1,0,999,0,0,0,1);
	`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSource(tx, "test", source{Agent: "cx", Session: "p", Project: "test"}); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := saveIntervals(tx, "test", []interval{{now.UnixMilli(), now.Add(time.Minute).UnixMilli()}}); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	restarted := Store{db}
	end := now.Add(24 * time.Hour).UnixMilli()
	got, err := restarted.AccountUsage(now.UnixMilli(), end)
	if err != nil || len(got.Days) != 1 || got.Days[0].Tokens != 123 || got.Summary == nil || got.Summary.LifetimeTokens != 999 {
		t.Fatalf("unexpected stored usage: %+v %v", got, err)
	}
	blocks, err := restarted.Blocks(now.UnixMilli(), end, Options{}, "")
	if err != nil || len(blocks) != 1 {
		t.Fatalf("unexpected blocks: %d %v", len(blocks), err)
	}
}
