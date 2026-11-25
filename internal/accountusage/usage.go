package accountusage

type Bucket struct {
	StartDate string `json:"startDate"`
	Tokens    int64  `json:"tokens"`
}

type Summary struct {
	CurrentStreakDays     int64 `json:"currentStreakDays"`
	LifetimeTokens        int64 `json:"lifetimeTokens"`
	LongestRunningTurnSec int64 `json:"longestRunningTurnSec"`
	LongestStreakDays     int64 `json:"longestStreakDays"`
	PeakDailyTokens       int64 `json:"peakDailyTokens"`
}
