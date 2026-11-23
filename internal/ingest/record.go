package ingest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"agent-timeline/internal/timeline"
)

const (
	MaxBodyBytes = 1 << 20
	MaxRecords   = 500
	MaxBlocks    = 1000
	MinTimestamp = 946684800000
	MaxTimestamp = 4102444800000
	maxSpan      = 366 * 24 * 60 * 60 * 1000
)

type Batch struct {
	Records    []Record     `json:"records,omitempty"`
	DailyUsage []DailyUsage `json:"dailyUsage,omitempty"`
}

type Record struct {
	ServiceTier     string                `json:"serviceTier,omitempty"`
	UsageByModel    []timeline.ModelUsage `json:"usageByModel,omitempty"`
	SourceKey       string                `json:"sourceKey,omitempty"`
	Session         string                `json:"session,omitempty"`
	Agent           string                `json:"agent"`
	Title           string                `json:"title"`
	Project         string                `json:"project,omitempty"`
	CWD             string                `json:"cwd,omitempty"`
	Model           string                `json:"model,omitempty"`
	Branch          string                `json:"branch,omitempty"`
	ReasoningEffort string                `json:"reasoningEffort,omitempty"`
	Tokens          Tokens                `json:"tokens,omitempty"`
	TurnCount       *int64                `json:"turnCount,omitempty"`
	ToolCallCount   *int64                `json:"toolCallCount,omitempty"`
	Blocks          []Block               `json:"blocks"`
}

type Tokens struct {
	Total       *int64 `json:"total,omitempty"`
	Input       *int64 `json:"input,omitempty"`
	CachedInput *int64 `json:"cachedInput,omitempty"`
	Output      *int64 `json:"output,omitempty"`
	Reasoning   *int64 `json:"reasoning,omitempty"`
}

type Block struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type DailyUsage struct {
	Agent  string `json:"agent"`
	Day    string `json:"day"`
	Tokens *int64 `json:"tokens"`
}

type Issue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

var agentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func (r Record) SessionID() string {
	if r.SourceKey != "" {
		return r.SourceKey
	}
	return r.Session
}

func (b Batch) Validate() *Issue {
	if len(b.Records) == 0 && len(b.DailyUsage) == 0 {
		return &Issue{"body", "provide at least one record or daily usage entry"}
	}
	if len(b.Records) > MaxRecords || len(b.DailyUsage) > MaxRecords {
		return &Issue{"body", "records and dailyUsage must each contain at most 500 entries"}
	}
	keys := make(map[[2]string]bool)
	for i, r := range b.Records {
		path := fmt.Sprintf("records[%d]", i)
		if issue := r.validate(path); issue != nil {
			return issue
		}
		identity := [2]string{r.Agent, r.SessionID()}
		if keys[identity] {
			return &Issue{path, "duplicate record key"}
		}
		keys[identity] = true
	}
	usage := make(map[[2]string]bool)
	for i, d := range b.DailyUsage {
		path := fmt.Sprintf("dailyUsage[%d]", i)
		if !agentPattern.MatchString(d.Agent) {
			return &Issue{path + ".agent", "use a lowercase agent identifier (1–32 characters)"}
		}
		if day, err := time.Parse(time.DateOnly, d.Day); err != nil || day.Year() < 2000 || day.Year() >= 2100 {
			return &Issue{path + ".day", "use a date from 2000-01-01 through 2099-12-31 (YYYY-MM-DD in JST)"}
		}
		if d.Tokens == nil || *d.Tokens < 0 {
			return &Issue{path + ".tokens", "provide a nonnegative integer"}
		}
		key := [2]string{d.Agent, d.Day}
		if usage[key] {
			return &Issue{path, "duplicate agent and day"}
		}
		usage[key] = true
	}
	return nil
}

func (r Record) validate(path string) *Issue {
	if (r.SourceKey == "") == (r.Session == "") {
		return &Issue{path, "provide exactly one of sourceKey or session"}
	}
	key := r.SessionID()
	if len(key) > 160 || strings.TrimSpace(key) != key || strings.IndexFunc(key, unicode.IsControl) != -1 {
		return &Issue{path, "record key must be 1–160 bytes without surrounding whitespace or control characters"}
	}
	if !agentPattern.MatchString(r.Agent) {
		return &Issue{path + ".agent", "use a lowercase agent identifier (1–32 characters)"}
	}
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 8192 {
		return &Issue{path + ".title", "provide a nonblank title of at most 8192 bytes"}
	}
	for _, v := range []struct{ name, text string }{
		{"project", r.Project}, {"cwd", r.CWD}, {"model", r.Model}, {"branch", r.Branch}, {"reasoningEffort", r.ReasoningEffort},
	} {
		if len(v.text) > 2048 {
			return &Issue{path + "." + v.name, "must be at most 2048 bytes"}
		}
	}
	for _, v := range []struct {
		name  string
		count *int64
	}{
		{"tokens.total", r.Tokens.Total}, {"tokens.input", r.Tokens.Input},
		{"tokens.cachedInput", r.Tokens.CachedInput}, {"tokens.output", r.Tokens.Output},
		{"tokens.reasoning", r.Tokens.Reasoning}, {"turnCount", r.TurnCount}, {"toolCallCount", r.ToolCallCount},
	} {
		if v.count != nil && *v.count < 0 {
			return &Issue{path + "." + v.name, "must be a nonnegative integer"}
		}
	}
	if issue := r.validateModelUsage(path); issue != nil {
		return issue
	}
	if len(r.Blocks) == 0 || len(r.Blocks) > MaxBlocks {
		return &Issue{path + ".blocks", "provide 1–1000 blocks"}
	}
	starts := make([]int64, 0, len(r.Blocks))
	for i, b := range r.Blocks {
		at := fmt.Sprintf("%s.blocks[%d]", path, i)
		if b.Start < MinTimestamp || b.Start >= MaxTimestamp {
			return &Issue{at + ".start", "use Unix milliseconds from 2000-01-01 through 2099-12-31 UTC"}
		}
		if b.End <= b.Start || b.End > MaxTimestamp || b.End-b.Start > maxSpan {
			return &Issue{at + ".end", "must be Unix milliseconds after start and within 366 days (at most 2100-01-01 UTC)"}
		}
		starts = append(starts, b.Start)
	}
	slices.Sort(starts)
	for i := 1; i < len(starts); i++ {
		if starts[i] == starts[i-1] {
			return &Issue{path + ".blocks", "duplicate block start timestamp"}
		}
	}
	return nil
}
