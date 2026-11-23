package index

import (
	"database/sql"
	"fmt"
)

const metadataVersion = 1

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch version {
	case 3:
		return nil
	case 2:
		return migrateUsage(tx)
	case 1:
		if err := migrateHistory(tx); err != nil {
			return err
		}
		return migrateUsage(tx)
	case 0:
	default:
		return fmt.Errorf("unsupported index database version %d", version)
	}

	for _, table := range []string{"sources", "codex"} {
		for _, column := range []string{
			"input_tokens INTEGER", "cached_input_tokens INTEGER", "output_tokens INTEGER",
			"reasoning_tokens INTEGER", "turn_count INTEGER", "tool_call_count INTEGER",
			"reasoning_effort TEXT NOT NULL DEFAULT ''", "git_sha TEXT NOT NULL DEFAULT ''",
			"git_origin_url TEXT NOT NULL DEFAULT ''", "preview TEXT NOT NULL DEFAULT ''",
		} {
			if _, err := tx.Exec("ALTER TABLE " + table + " ADD COLUMN " + column); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`ALTER TABLE files ADD COLUMN metadata_version INTEGER NOT NULL DEFAULT 0;
		CREATE TABLE claude_messages (
			source TEXT NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
			message TEXT NOT NULL, input_tokens INTEGER, cached_input_tokens INTEGER,
			output_tokens INTEGER,
			PRIMARY KEY(source,message)
		) WITHOUT ROWID;
		CREATE TABLE claude_tools (
			source TEXT NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
			tool TEXT NOT NULL, PRIMARY KEY(source,tool)
		) WITHOUT ROWID;
	`); err != nil {
		return err
	}
	if err := migrateHistory(tx); err != nil {
		return err
	}
	return migrateUsage(tx)
}

func migrateHistory(tx *sql.Tx) error {
	_, err := tx.Exec(`
		DROP VIEW IF EXISTS timeline;
		CREATE VIEW ranked_sources AS
		SELECT s.*, own.byte_offset,
			CASE WHEN own.path IS NOT NULL THEN 3
			     WHEN s.key='codex:turns:' || s.session THEN 2
			     WHEN s.key='codex:metadata:' || s.session THEN 1
			     ELSE 0 END AS precedence
		FROM sources s LEFT JOIN files own ON own.path=s.key;
		CREATE VIEW timeline AS
		SELECT b.id, s.agent, COALESCE(NULLIF(c.title,''),NULLIF(s.title,''),s.project) title,
		       CASE WHEN c.cwd IS NOT NULL AND c.cwd <> '' THEN c.cwd ELSE s.cwd END cwd,
		       CASE WHEN c.cwd IS NOT NULL AND c.cwd <> '' THEN c.project ELSE s.project END project, COALESCE(NULLIF(c.model,''),s.model) model,
		       COALESCE(NULLIF(c.branch,''),s.branch) branch,
		       COALESCE(NULLIF(c.tokens,0),s.tokens) tokens, b.start,b.end,
		       s.key source, s.input_tokens, s.cached_input_tokens, s.output_tokens,
		       s.reasoning_tokens, s.turn_count, s.tool_call_count,
		       COALESCE(NULLIF(c.reasoning_effort,''),s.reasoning_effort) reasoning_effort,
		       COALESCE(NULLIF(c.git_sha,''),s.git_sha) git_sha,
		       COALESCE(NULLIF(c.git_origin_url,''),s.git_origin_url) git_origin_url,
		       COALESCE(NULLIF(c.preview,''),s.preview) preview,
		       CASE s.precedence WHEN 3 THEN 'rollout' WHEN 2 THEN 'turns'
		           WHEN 1 THEN 'metadata' ELSE 't3' END AS provenance
		FROM blocks b JOIN ranked_sources s ON s.key=b.source
		LEFT JOIN codex c ON s.agent='cx' AND c.session=s.session
		WHERE s.session='' OR NOT EXISTS (
			SELECT 1 FROM ranked_sources f
			WHERE f.agent=s.agent AND f.session=s.session
			      AND (f.precedence>s.precedence OR
			          (f.precedence=3 AND s.precedence=3 AND
			           (f.byte_offset>s.byte_offset OR (f.byte_offset=s.byte_offset AND f.key>s.key))))
			      AND EXISTS (SELECT 1 FROM blocks WHERE source=f.key)
		);
		PRAGMA user_version=2;
	`)
	return err
}

func migrateUsage(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE daily_usage (
			day TEXT PRIMARY KEY, tokens INTEGER NOT NULL, updated_at INTEGER NOT NULL
		) WITHOUT ROWID;
		CREATE TABLE usage_summary (
			id INTEGER PRIMARY KEY CHECK(id=1), current_streak_days INTEGER NOT NULL,
			lifetime_tokens INTEGER NOT NULL, longest_running_turn_sec INTEGER NOT NULL,
			longest_streak_days INTEGER NOT NULL, peak_daily_tokens INTEGER NOT NULL, updated_at INTEGER NOT NULL
		);
		PRAGMA user_version=3;
	`)
	if err != nil {
		return err
	}
	return tx.Commit()
}
