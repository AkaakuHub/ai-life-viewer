package index

import (
	"database/sql"
	"testing"
	"time"

	"agent-timeline/internal/accountusage"
	"agent-timeline/internal/timeline"
)

func TestUsageCacheCoexistsWithDetailedBlocksAndSurvivesThrottle(t *testing.T) {
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
	store := Store{db}
	if ok, err := store.claimUsageRefresh(now); err != nil || !ok {
		t.Fatalf("initial refresh: %v %v", ok, err)
	}
	snapshot := accountusage.Snapshot{
		DailyUsageBuckets: []accountusage.Bucket{{StartDate: "2026-06-01", Tokens: 123}},
		Summary:           &accountusage.Summary{LifetimeTokens: 999},
	}
	if err := store.saveAccountUsage(snapshot, now); err != nil {
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
	if err := saveIntervals(tx, "test", []interval{{now.UnixMilli(), now.Add(time.Minute).UnixMilli()}}, 0); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	restarted := Store{db}
	if ok, err := restarted.claimUsageRefresh(now.Add(6*time.Hour - time.Millisecond)); err != nil || ok {
		t.Fatalf("premature refresh on restart: %v %v", ok, err)
	}
	if ok, err := restarted.claimUsageRefresh(now.Add(6 * time.Hour)); err != nil || !ok {
		t.Fatalf("scheduled refresh: %v %v", ok, err)
	}
	end := now.Add(24 * time.Hour).UnixMilli()
	got, err := restarted.AccountUsage(now.UnixMilli(), end)
	if err != nil || len(got.Days) != 1 || got.Days[0].Tokens != 123 || got.Summary == nil || got.Summary.LifetimeTokens != 999 {
		t.Fatalf("unexpected cache after refresh attempt: %+v %v", got, err)
	}
	blocks, err := restarted.Blocks(now.UnixMilli(), end, Options{}, "")
	if err != nil || len(blocks) != 1 {
		t.Fatalf("unexpected blocks: %d %v", len(blocks), err)
	}
}
