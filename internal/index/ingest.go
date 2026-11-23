package index

import (
	"database/sql"
	"time"

	"agent-timeline/internal/ingest"
	"agent-timeline/internal/timeline"
)

func (s *Store) Ingest(batch ingest.Batch) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, record := range batch.Records {
		if err := mergeRecord(tx, record); err != nil {
			return err
		}
	}
	now := time.Now().UnixMilli()
	for _, d := range batch.DailyUsage {
		_, err := tx.Exec(`INSERT INTO daily_usage(agent,day,tokens,updated_at) VALUES(?,?,?,?)
			ON CONFLICT(agent,day) DO UPDATE SET tokens=excluded.tokens,updated_at=excluded.updated_at`,
			d.Agent, d.Day, d.Tokens, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func mergeRecord(tx *sql.Tx, r ingest.Record) error {
	session := r.SessionID()
	key := "session:" + r.Agent + ":" + session
	_, err := tx.Exec(`INSERT INTO sources (key,agent,session,title,project,cwd,model,branch,tokens,
		input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,turn_count,tool_call_count,reasoning_effort)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET
			title=excluded.title, project=COALESCE(NULLIF(excluded.project,''),project),
			cwd=COALESCE(NULLIF(excluded.cwd,''),cwd), model=COALESCE(NULLIF(excluded.model,''),model),
			branch=COALESCE(NULLIF(excluded.branch,''),branch), tokens=COALESCE(excluded.tokens,tokens),
			input_tokens=COALESCE(excluded.input_tokens,input_tokens),
			cached_input_tokens=COALESCE(excluded.cached_input_tokens,cached_input_tokens),
			output_tokens=COALESCE(excluded.output_tokens,output_tokens),
			reasoning_tokens=COALESCE(excluded.reasoning_tokens,reasoning_tokens),
			turn_count=COALESCE(excluded.turn_count,turn_count), tool_call_count=COALESCE(excluded.tool_call_count,tool_call_count),
			reasoning_effort=COALESCE(NULLIF(excluded.reasoning_effort,''),reasoning_effort)`,
		key, r.Agent, session, r.Title, r.Project, r.CWD, r.Model, r.Branch, r.Tokens.Total,
		r.Tokens.Input, r.Tokens.CachedInput, r.Tokens.Output, r.Tokens.Reasoning,
		r.TurnCount, r.ToolCallCount, r.ReasoningEffort)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO session_intervals(source,start,end) VALUES(?,?,?)
		ON CONFLICT(source,start) DO UPDATE SET end=excluded.end`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, b := range r.Blocks {
		if _, err := stmt.Exec(key, b.Start, b.End); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DailyUsage(start, end int64) ([]ingest.DailyUsage, error) {
	rows, err := s.db.Query(`SELECT agent,day,tokens FROM daily_usage WHERE day BETWEEN ? AND ? ORDER BY day,agent`,
		time.UnixMilli(start).In(timeline.JST).Format(time.DateOnly), time.UnixMilli(end-1).In(timeline.JST).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ingest.DailyUsage{}
	for rows.Next() {
		var item ingest.DailyUsage
		if err := rows.Scan(&item.Agent, &item.Day, &item.Tokens); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
