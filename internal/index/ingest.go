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
		if err := mergeRecord(tx, record, batch.PCID); err != nil {
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
	if _, err := tx.Exec(`INSERT INTO ingest_pcs(pc_id,last_ingest) VALUES(?,?)
 ON CONFLICT(pc_id) DO UPDATE SET last_ingest=max(last_ingest,excluded.last_ingest)`, batch.PCID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func mergeRecord(tx *sql.Tx, r ingest.Record, pcID string) error {
	session := r.SessionID()
	key := "session:" + r.Agent + ":" + session
	breakdown, err := modelUsageJSON(r.UsageByModel)
	if err != nil {
		return err
	}
	tokens := r.Tokens
	if totals, ok := r.ModelUsageTotals(); len(r.UsageByModel) != 0 && ok {
		tokens = totals
	}
	_, err = tx.Exec(`INSERT INTO sources (key,agent,session,title,project,cwd,model,branch,tokens,
		input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,turn_count,tool_call_count,reasoning_effort,service_tier,usage_by_model)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET
			title=excluded.title, project=COALESCE(NULLIF(excluded.project,''),project),
			cwd=COALESCE(NULLIF(excluded.cwd,''),cwd), model=COALESCE(NULLIF(excluded.model,''),model),
			branch=COALESCE(NULLIF(excluded.branch,''),branch), tokens=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.tokens
				WHEN usage_by_model<>'[]' THEN tokens ELSE COALESCE(excluded.tokens,tokens) END,
			input_tokens=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.input_tokens
				WHEN usage_by_model<>'[]' THEN input_tokens ELSE COALESCE(excluded.input_tokens,input_tokens) END,
			cached_input_tokens=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.cached_input_tokens
				WHEN usage_by_model<>'[]' THEN cached_input_tokens ELSE COALESCE(excluded.cached_input_tokens,cached_input_tokens) END,
			output_tokens=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.output_tokens
				WHEN usage_by_model<>'[]' THEN output_tokens ELSE COALESCE(excluded.output_tokens,output_tokens) END,
			reasoning_tokens=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.reasoning_tokens
				WHEN usage_by_model<>'[]' THEN reasoning_tokens ELSE COALESCE(excluded.reasoning_tokens,reasoning_tokens) END,
			turn_count=COALESCE(excluded.turn_count,turn_count), tool_call_count=COALESCE(excluded.tool_call_count,tool_call_count),
			reasoning_effort=COALESCE(NULLIF(excluded.reasoning_effort,''),reasoning_effort),
			service_tier=COALESCE(NULLIF(excluded.service_tier,''),service_tier),
			usage_by_model=CASE WHEN excluded.usage_by_model<>'[]' THEN excluded.usage_by_model ELSE usage_by_model END`,
		key, r.Agent, session, r.Title, r.Project, r.CWD, r.Model, r.Branch, tokens.Total,
		tokens.Input, tokens.CachedInput, tokens.Output, tokens.Reasoning,
		r.TurnCount, r.ToolCallCount, r.ReasoningEffort, r.ServiceTier, breakdown)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO session_pcs(source,pc_id) VALUES(?,?)
		ON CONFLICT(source,pc_id) DO NOTHING`, key, pcID); err != nil {
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
