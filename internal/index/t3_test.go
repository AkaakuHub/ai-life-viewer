package index

import (
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestSyncT3GroupsRunsAndMigratesLegacySources(t *testing.T) {
	dir := t.TempDir()
	projection := filepath.Join(dir, "projection.sqlite")
	upstream, err := sql.Open("sqlite", projection)
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	exec := func(db *sql.DB, query string) {
		t.Helper()
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	exec(upstream, `
		CREATE TABLE orchestration_v2_projection_threads(thread_id,title,project_id,payload_json,updated_at,active_provider_thread_id);
		CREATE TABLE orchestration_v2_projection_runs(run_id,thread_id,ordinal,provider,provider_thread_id,status,requested_at,completed_at,payload_json);
		CREATE TABLE orchestration_v2_projection_turn_items(run_id,type);
		CREATE TABLE orchestration_v2_projection_provider_threads(provider_thread_id,payload_json);
		CREATE TABLE projection_projects(project_id,title,workspace_root);
		INSERT INTO orchestration_v2_projection_threads VALUES
			('thread-a','Thread A',NULL,'{}','2026-11-23T00:00:00Z','native'),
			('thread-b','Thread B',NULL,'{}','2026-11-23T00:00:00Z',NULL);
		INSERT INTO orchestration_v2_projection_provider_threads VALUES ('native','{"nativeThreadRef":{"nativeId":"native-a"}}');
		INSERT INTO orchestration_v2_projection_runs VALUES
			('run-1','thread-a',1,'codex',NULL,'completed','2026-11-23T00:00:00Z','2026-11-23T00:01:00Z','{"workStartedAt":"2026-11-23T00:00:00Z"}'),
			('run-2','thread-a',2,'codex',NULL,'completed','2026-11-23T00:06:00Z','2026-11-23T00:07:00Z','{"workStartedAt":"2026-11-23T00:06:00Z"}'),
			('run-3','thread-a',3,'codex',NULL,'completed','2026-11-23T00:13:00Z','2026-11-23T00:14:00Z','{"workStartedAt":"2026-11-23T00:13:00Z"}'),
			('run-4','thread-b',1,'antigravity',NULL,'completed','2026-11-23T00:02:00Z','2026-11-23T00:03:00Z','{"workStartedAt":"2026-11-23T00:02:00Z"}');
		INSERT INTO orchestration_v2_projection_turn_items VALUES ('run-1','command_execution'),('run-2','file_change'),('run-2','message');
	`)
	store, err := Open(filepath.Join(dir, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	exec(store.db, `
		INSERT INTO watermarks VALUES ('t3_metrics_v1','2026-11-23T00:14:00Z');
		INSERT INTO sources(key,agent,session,title,project,cwd,model,branch) VALUES
			('t3:run-1','cx','native-a','','','','',''),('codex:turns:local','cx','local','','','','','');
		INSERT INTO blocks(source,start,last,end) VALUES ('t3:run-1',1000,2000,2000),('codex:turns:local',1000,2000,2000);
		INSERT INTO daily_usage VALUES ('cx','2026-11-23',123,1);
	`)
	sync := func() {
		t.Helper()
		if err := store.syncT3(projection); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC).UnixMilli()
	checkThread := func(key string, want []interval) {
		t.Helper()
		rows, err := store.db.Query("SELECT start,last FROM blocks WHERE source=? ORDER BY start", key)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []interval
		for rows.Next() {
			var span interval
			if err := rows.Scan(&span.Start, &span.Last); err != nil {
				t.Fatal(err)
			}
			got = append(got, interval{(span.Start - base) / minute, (span.Last - base) / minute})
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s blocks = %v, want %v", key, got, want)
		}
	}
	sync()
	checkThread("t3:thread-a", []interval{{0, 7}, {13, 14}})
	checkThread("t3:thread-b", []interval{{2, 3}})
	checkThread("t3:run-1", nil)

	exec(upstream, `
		UPDATE orchestration_v2_projection_threads SET updated_at='2026-11-23T00:21:00Z' WHERE thread_id='thread-b';
		INSERT INTO orchestration_v2_projection_runs VALUES
			('run-5','thread-b',2,'antigravity',NULL,'completed','2026-11-23T00:20:00Z','2026-11-23T00:21:00Z','{"workStartedAt":"2026-11-23T00:20:00Z"}');
	`)
	sync()
	exec(upstream, `
		INSERT INTO orchestration_v2_projection_runs VALUES
			('run-6','thread-a',4,'codex',NULL,'completed','2026-11-23T00:22:00Z','2026-11-23T00:23:00Z','{"workStartedAt":"2026-11-23T00:18:00Z"}');
		INSERT INTO orchestration_v2_projection_turn_items VALUES ('run-6','dynamic_tool');
	`)
	sync()
	checkThread("t3:thread-a", []interval{{0, 7}, {13, 23}})
	checkThread("t3:thread-b", []interval{{2, 3}, {20, 21}})

	var turns, tools, codexBlocks, usage int
	var session string
	err = store.db.QueryRow(`SELECT turn_count,tool_call_count,session,
		(SELECT count(*) FROM blocks WHERE source='codex:turns:local'),
		(SELECT tokens FROM daily_usage WHERE agent='cx' AND day='2026-11-23')
		FROM sources WHERE key='t3:thread-a'`).Scan(&turns, &tools, &session, &codexBlocks, &usage)
	if err != nil {
		t.Fatal(err)
	}
	if turns != 4 || tools != 3 || session != "native-a" || codexBlocks != 1 || usage != 123 {
		t.Fatalf("got turns=%d, tools=%d, native session=%q, Codex blocks=%d, daily tokens=%d",
			turns, tools, session, codexBlocks, usage)
	}
}
