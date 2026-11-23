package index

import (
	"database/sql"
	"fmt"
)

func migrateIngest(tx *sql.Tx) error {
	if _, err := tx.Exec(fmt.Sprintf(localTimeline, "local_timeline")); err != nil {
		return err
	}
	_, err := tx.Exec(`
		ALTER TABLE daily_usage RENAME TO account_daily_usage;
		CREATE TABLE daily_usage (
			agent TEXT NOT NULL, day TEXT NOT NULL, tokens INTEGER NOT NULL, updated_at INTEGER NOT NULL,
			PRIMARY KEY(agent,day)
		) WITHOUT ROWID;
		INSERT INTO daily_usage SELECT 'cx',day,tokens,updated_at FROM account_daily_usage;
		DROP TABLE account_daily_usage;
		CREATE INDEX daily_usage_day ON daily_usage(day);
		CREATE TABLE session_intervals (
			id INTEGER PRIMARY KEY,
			source TEXT NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
			start INTEGER NOT NULL, end INTEGER NOT NULL, UNIQUE(source,start)
		);
		DROP VIEW timeline;
		CREATE VIEW timeline AS
		WITH unified AS (
			SELECT * FROM sources WHERE key GLOB 'session:*' AND key='session:'||agent||':'||session
		), matched AS (
			SELECT i.key canonical, l.*
			FROM local_timeline l JOIN sources s ON s.key=l.source
			JOIN unified i ON i.agent=s.agent AND i.session=s.session
		), local_details AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY canonical ORDER BY start,id) AS rank
			FROM matched
		), intervals AS (
			SELECT canonical, start, end, id AS local_id, NULL AS submitted_id FROM matched
			UNION ALL
			SELECT source, start, end, NULL, id FROM session_intervals
		), boundaries AS (
			SELECT *,
				MAX(end) OVER (PARTITION BY canonical ORDER BY start,end,local_id,submitted_id
					ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS last_end
			FROM intervals
		), islands AS (
			SELECT *, SUM(CASE WHEN last_end>=start THEN 0 ELSE 1 END)
				OVER (PARTITION BY canonical ORDER BY start,end,local_id,submitted_id ROWS UNBOUNDED PRECEDING) AS island
			FROM boundaries
		), merged AS (
			SELECT canonical, MIN(start) start, MAX(end) end, COALESCE(MIN(local_id),-MIN(submitted_id)) id
			FROM islands GROUP BY canonical,island
		)
		SELECT l.* FROM local_timeline l WHERE NOT EXISTS (SELECT 1 FROM matched m WHERE m.id=l.id)
		UNION ALL
		SELECT m.id, i.agent, COALESCE(NULLIF(i.title,''),l.title,'') title,
			COALESCE(NULLIF(i.cwd,''),l.cwd,'') cwd,
			COALESCE(NULLIF(i.project,''),l.project,'プロジェクト未設定') project,
			COALESCE(NULLIF(i.model,''),l.model,'') model,
			COALESCE(NULLIF(i.branch,''),l.branch,'') branch,
			COALESCE(i.tokens,l.tokens) tokens, m.start,m.end,i.key source,
			COALESCE(i.input_tokens,l.input_tokens) AS input_tokens, COALESCE(i.cached_input_tokens,l.cached_input_tokens) AS cached_input_tokens,
			COALESCE(i.output_tokens,l.output_tokens) AS output_tokens, COALESCE(i.reasoning_tokens,l.reasoning_tokens) AS reasoning_tokens,
			COALESCE(i.turn_count,l.turn_count) AS turn_count, COALESCE(i.tool_call_count,l.tool_call_count) AS tool_call_count,
			COALESCE(NULLIF(i.reasoning_effort,''),l.reasoning_effort,''),
			COALESCE(l.git_sha,''), COALESCE(l.git_origin_url,''), COALESCE(l.preview,''),
			COALESCE(l.provenance,'session')
		FROM merged m JOIN unified i ON i.key=m.canonical
		LEFT JOIN local_details l ON l.canonical=i.key AND l.rank=1;
		PRAGMA user_version=4;
	`)
	if err != nil {
		return err
	}
	return migrateModelUsage(tx)
}
