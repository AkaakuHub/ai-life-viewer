package index

import (
	"cmp"
	"slices"
	"strconv"
)

func metadataSpan(created, updated int64) (interval, bool) {
	if created <= 0 || updated < created {
		return interval{}, false
	}
	if updated == created || updated-created > 5*minute {
		updated = created + minute
	}
	return interval{created, updated}, true
}

func (s *Store) syncCodexTurns(path string) error {
	db, err := readOnly(path)
	if err != nil || db == nil {
		return err
	}
	defer db.Close()
	mark, err := watermark(s.db, "codex_turns")
	if err != nil {
		return err
	}
	rows, err := db.Query(`
		SELECT t.thread_id, t.started_at, t.completed_at, t.duration_ms
		FROM thread_turns t
		WHERE t.started_at > 0 AND t.completed_at >= t.started_at
		  AND t.thread_id IN (
			SELECT thread_id FROM thread_turns
			WHERE started_at > 0 AND completed_at >= started_at AND completed_at >= CAST(? AS INTEGER)
		  )
		ORDER BY t.thread_id, t.started_at`, mark)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var session string
	var spans []interval
	var newest int64
	save := func() error {
		if len(spans) == 0 {
			return nil
		}
		key := "codex:turns:" + session
		count := int64(len(spans))
		meta := source{Agent: "cx", Session: session, Project: projectName("")}
		meta.TurnCount = &count
		if err := saveSource(tx, key, meta); err != nil {
			return err
		}
		return saveIntervals(tx, key, mergeTurnSpans(spans))
	}
	for rows.Next() {
		var id string
		var start, completed int64
		var duration *int64
		if err := rows.Scan(&id, &start, &completed, &duration); err != nil {
			return err
		}
		newest = max(newest, completed)
		if session != id {
			if err := save(); err != nil {
				return err
			}
			session, spans = id, spans[:0]
		}
		end := completed * 1000
		start *= 1000
		if duration != nil && *duration > 0 && end-start-*duration > -1000 && end-start-*duration < 1000 {
			end = start + *duration
		}
		if end > start {
			spans = append(spans, interval{start, end})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := save(); err != nil {
		return err
	}
	if newest > 0 {
		if err := writeWatermark(tx, "codex_turns", strconv.FormatInt(newest, 10)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func mergeTurnSpans(spans []interval) []interval {
	slices.SortFunc(spans, func(a, b interval) int { return cmp.Compare(a.Start, b.Start) })
	out := spans[:0]
	for _, span := range spans {
		if len(out) == 0 || span.Start > out[len(out)-1].Last {
			out = append(out, span)
		} else {
			out[len(out)-1].Last = max(out[len(out)-1].Last, span.Last)
		}
	}
	return out
}
