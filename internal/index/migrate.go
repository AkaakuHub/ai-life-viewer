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
	case metadataVersion:
		return nil
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
		DROP VIEW IF EXISTS timeline;
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
		       COALESCE(NULLIF(c.preview,''),s.preview) preview
		FROM blocks b JOIN sources s ON s.key=b.source
		LEFT JOIN files own ON own.path=s.key
		LEFT JOIN codex c ON s.agent='cx' AND c.session=s.session
		WHERE s.session='' OR NOT EXISTS (
			SELECT 1 FROM sources f JOIN files i ON i.path=f.key
			WHERE f.agent=s.agent AND f.session=s.session AND
			      (own.path IS NULL OR i.byte_offset > own.byte_offset OR
			       (i.byte_offset = own.byte_offset AND f.key > s.key))
			      AND EXISTS (SELECT 1 FROM blocks WHERE source=f.key)
		);
		PRAGMA user_version=1;
	`); err != nil {
		return err
	}
	return tx.Commit()
}
