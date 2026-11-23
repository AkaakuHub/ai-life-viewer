package index

import (
	"database/sql"
	"time"
)

func (s *Store) syncT3(path string) error {
	db, err := readOnly(path)
	if err != nil || db == nil {
		return err
	}
	defer db.Close()
	const watermarkKey = "t3_threads_v1"
	mark, err := watermark(s.db, watermarkKey)
	if err != nil {
		return err
	}
	rebuild := mark == ""
	rows, err := db.Query(`
		WITH changed_threads AS (
			SELECT DISTINCT r.thread_id
			FROM orchestration_v2_projection_runs r
			JOIN orchestration_v2_projection_threads t ON t.thread_id=r.thread_id
			WHERE r.status='running' OR t.updated_at>=? OR r.requested_at>=? OR r.completed_at>=?
		)
		SELECT r.thread_id,r.run_id,r.provider,t.title,
			COALESCE(NULLIF(json_extract(t.payload_json,'$.worktreePath'),''),p.workspace_root,''),
			COALESCE(p.title,''),COALESCE(json_extract(t.payload_json,'$.branch'),''),
			COALESCE(json_extract(r.payload_json,'$.modelSelection.model'),''),
			COALESCE(json_extract(pt.payload_json,'$.nativeThreadRef.nativeId'),''),
			COALESCE(json_extract(r.payload_json,'$.workStartedAt'),json_extract(r.payload_json,'$.startedAt'),''),
			COALESCE(r.completed_at,json_extract(r.payload_json,'$.completedAt'),''),
			r.status,MAX(t.updated_at,r.requested_at,COALESCE(r.completed_at,'')),
			(SELECT COUNT(*) FROM orchestration_v2_projection_turn_items i
			 WHERE i.run_id=r.run_id AND i.type IN ('command_execution','dynamic_tool','file_change','file_search','web_search','subagent')),
			COALESCE((SELECT json_extract(option.value,'$.value')
				FROM json_each(r.payload_json,'$.modelSelection.options') option
				WHERE json_extract(option.value,'$.id') IN ('reasoningEffort','thought_level') LIMIT 1),'')
		FROM orchestration_v2_projection_runs r
		JOIN orchestration_v2_projection_threads t ON t.thread_id=r.thread_id
		LEFT JOIN projection_projects p ON p.project_id=t.project_id
		LEFT JOIN orchestration_v2_projection_provider_threads pt
			ON pt.provider_thread_id=COALESCE(r.provider_thread_id,t.active_provider_thread_id)
		WHERE r.provider IN ('codex','claude','antigravity')
			AND r.thread_id IN (SELECT thread_id FROM changed_threads)
		ORDER BY r.thread_id,r.ordinal`, mark, mark, mark)
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
	threads := make(map[string]*t3Thread)
	for rows.Next() {
		var threadID, runID, provider, start, end, status, updated string
		var m source
		if err := rows.Scan(&threadID, &runID, &provider, &m.Title, &m.CWD, &m.Project, &m.Branch, &m.Model, &m.Session,
			&start, &end, &status, &updated, &m.ToolCallCount, &m.ReasoningEffort); err != nil {
			return err
		}
		mark = max(mark, updated)
		if rebuild {
			if _, err := tx.Exec("DELETE FROM sources WHERE key=?", "t3:"+runID); err != nil {
				return err
			}
		}
		thread := threads[threadID]
		if thread == nil {
			thread = &t3Thread{}
			threads[threadID] = thread
		}
		from, fromErr := time.Parse(time.RFC3339Nano, start)
		to, toErr := time.Parse(time.RFC3339Nano, end)
		if end == "" && status == "running" {
			to, toErr = now, nil
		}
		if fromErr != nil || toErr != nil || to.Before(from) || status == "rolled_back" {
			continue
		}
		m.Agent = agents[provider]
		if m.Project == "" {
			m.Project = projectName(m.CWD)
		}
		thread.add(m, interval{from.UnixMilli(), to.UnixMilli()})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for threadID, thread := range threads {
		if err := thread.save(tx, threadID); err != nil {
			return err
		}
	}
	if err := writeWatermark(tx, watermarkKey, mark); err != nil {
		return err
	}
	return tx.Commit()
}

type t3Thread struct {
	metadata source
	spans    []interval
}

func (t *t3Thread) add(m source, span interval) {
	if m.Session == "" && t.metadata.Agent == m.Agent {
		m.Session = t.metadata.Session
	}
	m.TurnCount = t.metadata.TurnCount
	increment(&m.TurnCount)
	if t.metadata.ToolCallCount != nil {
		*m.ToolCallCount += *t.metadata.ToolCallCount
	}
	t.metadata = m
	t.spans = append(t.spans, span)
}

func (t *t3Thread) save(tx *sql.Tx, id string) error {
	key := "t3:" + id
	if len(t.spans) == 0 {
		_, err := tx.Exec("DELETE FROM sources WHERE key=?", key)
		return err
	}
	if t.metadata.Session == "" {
		t.metadata.Session = id
	}
	if err := saveSource(tx, key, t.metadata); err != nil {
		return err
	}
	return saveIntervals(tx, key, mergeIntervals(t.spans), minute)
}
