package index

import (
	"database/sql"
	"errors"
	"os"
	"time"
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
	mark, err := watermark(s.db, "codex")
	if err != nil {
		return err
	}
	if mark == "" {
		mark = "0"
	}
	rows, err := db.Query(`SELECT id,title,cwd,COALESCE(model,''),COALESCE(git_branch,''),tokens_used,updated_at_ms
		FROM threads WHERE updated_at_ms >= CAST(? AS INTEGER) ORDER BY updated_at_ms`, mark)
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
		var tokens int64
		if err := rows.Scan(&m.Session, &m.Title, &m.CWD, &m.Model, &m.Branch, &tokens, &mark); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO codex VALUES(?,?,?,?,?,?,?) ON CONFLICT(session) DO UPDATE SET
			title=excluded.title,cwd=excluded.cwd,model=excluded.model,branch=excluded.branch,
			tokens=excluded.tokens,project=excluded.project`, m.Session, m.Title, m.CWD, m.Model, m.Branch, tokens, projectName(m.CWD))
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := writeWatermark(tx, "codex", mark); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) syncT3(path string) error {
	db, err := readOnly(path)
	if err != nil || db == nil {
		return err
	}
	defer db.Close()
	mark, err := watermark(s.db, "t3")
	if err != nil {
		return err
	}
	rows, err := db.Query(`
		SELECT r.run_id,r.provider,t.title,
			COALESCE(NULLIF(json_extract(t.payload_json,'$.worktreePath'),''),p.workspace_root,''),
			COALESCE(p.title,''),COALESCE(json_extract(t.payload_json,'$.branch'),''),
			COALESCE(json_extract(r.payload_json,'$.modelSelection.model'),''),
			COALESCE(json_extract(pt.payload_json,'$.nativeThreadRef.nativeId'),''),
			COALESCE(json_extract(r.payload_json,'$.workStartedAt'),json_extract(r.payload_json,'$.startedAt'),''),
			COALESCE(r.completed_at,json_extract(r.payload_json,'$.completedAt'),''),
			r.status,MAX(t.updated_at,r.requested_at,COALESCE(r.completed_at,''))
		FROM orchestration_v2_projection_runs r
		JOIN orchestration_v2_projection_threads t ON t.thread_id=r.thread_id
		LEFT JOIN projection_projects p ON p.project_id=t.project_id
		LEFT JOIN orchestration_v2_projection_provider_threads pt
			ON pt.provider_thread_id=COALESCE(r.provider_thread_id,t.active_provider_thread_id)
		WHERE r.provider IN ('codex','claude','antigravity')
			AND (r.status='running' OR t.updated_at>=? OR r.requested_at>=? OR r.completed_at>=?)`, mark, mark, mark)
	if err != nil {
		return err
	}
	defer rows.Close()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	agents := map[string]string{"codex": "cx", "claude": "cl", "antigravity": "ag"}
	for rows.Next() {
		var run, provider, start, end, status, updated string
		var m source
		if err := rows.Scan(&run, &provider, &m.Title, &m.CWD, &m.Project, &m.Branch, &m.Model, &m.Session,
			&start, &end, &status, &updated); err != nil {
			return err
		}
		mark = max(mark, updated)
		m.Agent = agents[provider]
		if m.Project == "" {
			m.Project = projectName(m.CWD)
		}
		key := "t3:" + run
		from, fromErr := time.Parse(time.RFC3339Nano, start)
		to, toErr := time.Parse(time.RFC3339Nano, end)
		if end == "" && status == "running" {
			to, toErr = now, nil
		}
		if fromErr != nil || toErr != nil || to.Before(from) || status == "rolled_back" {
			if _, err := tx.Exec("DELETE FROM sources WHERE key=?", key); err != nil {
				return err
			}
			continue
		}
		if err := saveSource(tx, key, m); err != nil {
			return err
		}
		if err := saveIntervals(tx, key, []interval{{from.UnixMilli(), to.UnixMilli()}}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := writeWatermark(tx, "t3", mark); err != nil {
		return err
	}
	return tx.Commit()
}
