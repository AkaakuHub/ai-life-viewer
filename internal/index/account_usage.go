package index

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"agent-timeline/internal/accountusage"
	"agent-timeline/internal/timeline"
)

const usageRefreshInterval = 6 * time.Hour

type AccountUsage struct {
	Days    []accountusage.Bucket `json:"days"`
	Summary *accountusage.Summary `json:"summary,omitempty"`
}

func (s *Store) WatchAccountUsage(ctx context.Context, dir string) {
	for ctx.Err() == nil {
		delay := s.refreshAccountUsage(ctx, dir, time.Now())
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Store) refreshAccountUsage(ctx context.Context, dir string, now time.Time) time.Duration {
	claimed, err := s.claimUsageRefresh(now)
	if err != nil {
		return usageRefreshInterval
	}
	if claimed {
		snapshot, err := accountusage.Read(ctx, dir)
		if err == nil {
			_ = s.saveAccountUsage(snapshot, now)
		}
		return usageRefreshInterval
	}
	var last int64
	if err := s.db.QueryRow("SELECT value FROM watermarks WHERE key='account_usage_attempt'").Scan(&last); err != nil {
		return usageRefreshInterval
	}
	return max(time.Second, time.UnixMilli(last).Add(usageRefreshInterval).Sub(now))
}

func (s *Store) claimUsageRefresh(now time.Time) (bool, error) {
	result, err := s.db.Exec(`INSERT INTO watermarks VALUES('account_usage_attempt',?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
		WHERE CAST(watermarks.value AS INTEGER) <= ?`, now.UnixMilli(), now.Add(-usageRefreshInterval).UnixMilli())
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) saveAccountUsage(snapshot accountusage.Snapshot, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, day := range snapshot.DailyUsageBuckets {
		if _, err := time.Parse(time.DateOnly, day.StartDate); err != nil || day.Tokens < 0 {
			continue
		}
		_, err := tx.Exec(`INSERT INTO daily_usage(day,tokens,updated_at) VALUES(?,?,?)
			ON CONFLICT(day) DO UPDATE SET tokens=excluded.tokens,updated_at=excluded.updated_at`,
			day.StartDate, day.Tokens, now.UnixMilli())
		if err != nil {
			return err
		}
	}
	if m := snapshot.Summary; m != nil {
		_, err := tx.Exec(`INSERT INTO usage_summary VALUES(1,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET current_streak_days=excluded.current_streak_days,
			lifetime_tokens=excluded.lifetime_tokens,longest_running_turn_sec=excluded.longest_running_turn_sec,
			longest_streak_days=excluded.longest_streak_days,peak_daily_tokens=excluded.peak_daily_tokens,
			updated_at=excluded.updated_at`, m.CurrentStreakDays, m.LifetimeTokens, m.LongestRunningTurnSec,
			m.LongestStreakDays, m.PeakDailyTokens, now.UnixMilli())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
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
	rows, err := s.db.Query("SELECT day,tokens FROM daily_usage WHERE day BETWEEN ? AND ? AND tokens>0 ORDER BY day",
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
