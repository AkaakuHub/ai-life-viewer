package index

import (
	"database/sql"
	"errors"
	"os"
	"strconv"
)

func readOnly(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return sql.Open("sqlite", uri(path, "mode=ro&_busy_timeout=5000&_pragma=busy_timeout(5000)"))
}

func watermark(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM watermarks WHERE key=?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func writeWatermark(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec("INSERT INTO watermarks VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

func (s *Store) syncCodex(path string) error {
	db, err := readOnly(path)
	if err != nil || db == nil {
		return err
	}
	defer db.Close()
	mark, err := watermark(s.db, "codex_metadata_v2")
	if err != nil {
		return err
	}
	if mark == "" {
		mark = "0"
	}
	rows, err := db.Query(`SELECT id,title,cwd,COALESCE(model,''),COALESCE(git_branch,''),tokens_used,COALESCE(updated_at_ms,updated_at*1000),
		COALESCE(created_at_ms,created_at*1000),
		COALESCE(reasoning_effort,''),COALESCE(git_sha,''),COALESCE(git_origin_url,''),substr(trim(preview),1,240)
		FROM threads WHERE COALESCE(updated_at_ms,updated_at*1000) >= CAST(? AS INTEGER) ORDER BY COALESCE(updated_at_ms,updated_at*1000)`, mark)
	if err != nil {
		return err
	}
	defer rows.Close()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for rows.Next() {
		var m source
		var tokens, created, updated int64
		if err := rows.Scan(&m.Session, &m.Title, &m.CWD, &m.Model, &m.Branch, &tokens, &updated, &created, &m.ReasoningEffort, &m.GitSHA, &m.GitOriginURL, &m.Preview); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO codex(session,title,cwd,model,branch,tokens,project,reasoning_effort,git_sha,git_origin_url,preview)
			VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(session) DO UPDATE SET
			title=excluded.title,cwd=excluded.cwd,model=excluded.model,branch=excluded.branch,
			tokens=excluded.tokens,project=excluded.project,
			reasoning_effort=COALESCE(NULLIF(excluded.reasoning_effort,''),reasoning_effort),
			git_sha=COALESCE(NULLIF(excluded.git_sha,''),git_sha),
			git_origin_url=COALESCE(NULLIF(excluded.git_origin_url,''),git_origin_url),
			preview=COALESCE(NULLIF(excluded.preview,''),preview)`, m.Session, m.Title, m.CWD, m.Model, m.Branch, tokens, projectName(m.CWD), m.ReasoningEffort, m.GitSHA, cleanRemote(m.GitOriginURL), m.Preview)
		if err != nil {
			return err
		}
		mark = strconv.FormatInt(updated, 10)
		if span, ok := metadataSpan(created, updated); ok {
			key := "codex:metadata:" + m.Session
			m.Agent = "cx"
			if err := saveSource(tx, key, source{Agent: m.Agent, Session: m.Session, Project: projectName(m.CWD)}); err != nil {
				return err
			}
			if err := saveIntervals(tx, key, []interval{span}); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := writeWatermark(tx, "codex_metadata_v2", mark); err != nil {
		return err
	}
	return tx.Commit()
}
