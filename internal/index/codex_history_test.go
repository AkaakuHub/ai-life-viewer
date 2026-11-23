package index

import (
	"database/sql"
	"slices"
	"testing"
)

func TestMetadataSpanCapsUnobservedActivity(t *testing.T) {
	const start = 1_000_000
	for _, c := range []struct {
		delta, expected int64
	}{
		{0, minute},
		{2150, 2150},
		{5 * minute, 5 * minute},
		{3 * 24 * 60 * minute, minute},
	} {
		got, ok := metadataSpan(start, start+c.delta)
		if !ok || got != (interval{start, start + c.expected}) {
			t.Errorf("created=%d delta=%d got=%+v", start, c.delta, got)
		}
	}
}

func TestMeasuredTurnsMergeOverlapsWithoutFillingIdleTime(t *testing.T) {
	got := mergeTurnSpans([]interval{{500, 700}, {100, 200}, {150, 202}})
	if !slices.Equal(got, []interval{{100, 202}, {500, 700}}) {
		t.Fatalf("unexpected ranges: %v", got)
	}
}

func TestTimelineChoosesBestAvailableCodexSource(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := initialize(db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, key := range []string{"codex:metadata:a", "codex:turns:a", "t3:run", "rollout.jsonl", "archive.jsonl"} {
		if err := saveSource(tx, key, source{Agent: "cx", Session: "a", Project: "project"}); err != nil {
			t.Fatal(err)
		}
		if err := saveIntervals(tx, key, []interval{{1, 2}}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec("INSERT INTO files VALUES('rollout.jsonl',1,9,9,1,1,1),('archive.jsonl',1,8,8,1,2,1)"); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		var key string
		var count int
		if err := tx.QueryRow("SELECT min(source), count(*) FROM timeline").Scan(&key, &count); err != nil {
			t.Fatal(err)
		}
		if count != 1 || key != want {
			t.Fatalf("got count=%d source=%q, want %q", count, key, want)
		}
	}
	check("rollout.jsonl")
	if _, err := tx.Exec("DELETE FROM blocks WHERE source IN ('rollout.jsonl','archive.jsonl')"); err != nil {
		t.Fatal(err)
	}
	check("codex:turns:a")
	if _, err := tx.Exec("DELETE FROM blocks WHERE source='codex:turns:a'"); err != nil {
		t.Fatal(err)
	}
	check("codex:metadata:a")
}
