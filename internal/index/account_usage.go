package index

import (
	"database/sql"
	"errors"
	"time"

	"agent-timeline/internal/accountusage"
	"agent-timeline/internal/timeline"
)

type AccountUsage struct {
	Days    []accountusage.Bucket `json:"days"`
	Summary *accountusage.Summary `json:"summary,omitempty"`
}

func (s *Store) AccountUsage(start, end int64) (AccountUsage, error) {
	out := AccountUsage{Days: []accountusage.Bucket{}}
	var summary accountusage.Summary
	err := s.db.QueryRow(`SELECT current_streak_days,lifetime_tokens,longest_running_turn_sec,longest_streak_days,peak_daily_tokens
		FROM usage_summary WHERE id=1`).Scan(&summary.CurrentStreakDays, &summary.LifetimeTokens,
		&summary.LongestRunningTurnSec, &summary.LongestStreakDays, &summary.PeakDailyTokens)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err == nil {
		out.Summary = &summary
	}
	rows, err := s.db.Query("SELECT day,tokens FROM daily_usage WHERE agent='cx' AND day BETWEEN ? AND ? AND tokens>0 ORDER BY day",
		time.UnixMilli(start).In(timeline.JST).Format(time.DateOnly), time.UnixMilli(end-1).In(timeline.JST).Format(time.DateOnly))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var bucket accountusage.Bucket
		if err := rows.Scan(&bucket.StartDate, &bucket.Tokens); err != nil {
			return out, err
		}
		out.Days = append(out.Days, bucket)
	}
	return out, rows.Err()
}
