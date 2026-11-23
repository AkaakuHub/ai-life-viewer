package accountusage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"time"
)

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

type Snapshot struct {
	DailyUsageBuckets []Bucket `json:"dailyUsageBuckets"`
	Summary           *Summary `json:"summary"`
}

func Read(ctx context.Context, dir string) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	cmd.Dir = dir
	cmd.Stderr = io.Discard
	input, err := cmd.StdinPipe()
	if err != nil {
		return Snapshot{}, err
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		return Snapshot{}, err
	}
	if err := cmd.Start(); err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	return exchange(input, output)
}

func exchange(input io.Writer, output io.Reader) (Snapshot, error) {
	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"agent_timeline","version":"1.0.0"},"capabilities":{"experimentalApi":true}}}`
	const initialized = `{"jsonrpc":"2.0","method":"initialized"}`
	const read = `{"jsonrpc":"2.0","id":2,"method":"account/usage/read","params":{}}`
	decoder := json.NewDecoder(output)
	if _, err := io.WriteString(input, initialize+"\n"); err != nil {
		return Snapshot{}, err
	}
	if _, err := reply(decoder, 1); err != nil {
		return Snapshot{}, err
	}
	if _, err := io.WriteString(input, initialized+"\n"+read+"\n"); err != nil {
		return Snapshot{}, err
	}
	payload, err := reply(decoder, 2)
	if err != nil {
		return Snapshot{}, err
	}
	var result Snapshot
	if err := json.Unmarshal(payload, &result); err != nil {
		return Snapshot{}, err
	}
	if result.DailyUsageBuckets == nil && result.Summary == nil {
		return Snapshot{}, errors.New("empty account usage response")
	}
	return result, nil
}

func reply(decoder *json.Decoder, id int) (json.RawMessage, error) {
	for {
		var message struct {
			ID, Error, Result json.RawMessage
			Method            string
		}
		if err := decoder.Decode(&message); err != nil {
			return nil, err
		}
		if message.Method != "" || string(message.ID) != strconv.Itoa(id) {
			continue
		}
		if len(message.Result) == 0 || (len(message.Error) > 0 && string(message.Error) != "null") {
			return nil, errors.New("account usage RPC failed")
		}
		return message.Result, nil
	}
}
