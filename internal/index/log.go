package index

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"

	"agent-timeline/internal/timeline"
	"path/filepath"
	"time"
)

type source struct {
	timeline.Metrics
	usageSnapshot                                      usage
	usageByModel                                       []timeline.ModelUsage
	serviceTier                                        string
	ReasoningEffort, GitSHA, GitOriginURL, Preview     string
	claudeMessages                                     map[string]claudeUsage
	Agent, Session, Title, Project, CWD, Model, Branch string
	Tokens                                             *int64
}

type interval struct{ Start, Last int64 }

const minute = int64(time.Minute / time.Millisecond)

type usage struct {
	Total     *int64 `json:"total_tokens,omitempty"`
	Input     *int64 `json:"input_tokens,omitempty"`
	Cached    *int64 `json:"cached_input_tokens,omitempty"`
	Output    *int64 `json:"output_tokens,omitempty"`
	Reasoning *int64 `json:"reasoning_output_tokens,omitempty"`
}

type entry struct {
	Timestamp  time.Time
	Type       string
	SessionID  string
	CWD        string
	GitBranch  string
	Slug       string
	Message    claudeMessage
	IsMeta     bool
	Subtype    string
	DurationMS int64 `json:"durationMs"`
	Payload    struct {
		StartedAt   int64 `json:"started_at"`
		CompletedAt int64 `json:"completed_at"`
		DurationMS  int64 `json:"duration_ms"`
		ID          string
		CWD         string
		modelSettings
		ThreadSettings modelSettings `json:"thread_settings"`
		Type           string
		Effort         string
		Message        string
		Git            struct {
			Branch string
			Commit string `json:"commit_hash"`
			Remote string `json:"repository_url"`
		}
		ThreadUsage usage `json:"thread_token_usage"`
		Info        struct {
			Total usage `json:"total_token_usage"`
			Last  usage `json:"last_token_usage"`
		}
	}
}

func parseLog(reader io.Reader, meta *source) ([]interval, int64, error) {
	lines := bufio.NewReaderSize(reader, 64*1024)
	var spans []interval
	var offset int64
	for {
		line, err := lines.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return spans, offset, nil
		}
		if err != nil {
			return nil, 0, err
		}
		offset += int64(len(line))
		var e entry
		if skipCodexBody(line, meta) {
			continue
		}
		if json.Unmarshal(line, &e) != nil || e.Timestamp.IsZero() {
			continue
		}
		if meta.Agent == "cx" {
			switch e.Type {
			case "session_meta":
				meta.Session = e.Payload.ID
				meta.CWD = e.Payload.CWD
				meta.Branch = e.Payload.Git.Branch
				meta.GitSHA = e.Payload.Git.Commit
				meta.GitOriginURL = cleanRemote(e.Payload.Git.Remote)
				continue
			case "turn_context":
				meta.applySettings(e.Payload.modelSettings)
				if e.Payload.Effort != "" {
					meta.ReasoningEffort = e.Payload.Effort
				}
			case "token_usage_record":
				meta.observeModelUsage(e.Payload.ThreadUsage, usage{})
			case "event_msg":
				switch e.Payload.Type {
				case "token_count":
					meta.applySettings(e.Payload.modelSettings)
					meta.observeModelUsage(e.Payload.Info.Total, e.Payload.Info.Last)
				case "thread_settings_applied":
					meta.applySettings(e.Payload.ThreadSettings)
				case "task_started", "turn_started":
					increment(&meta.TurnCount)
				case "task_complete", "turn_complete", "turn_aborted":
					if span, ok := e.codexSpan(); ok {
						spans = append(spans, span)
					}
				case "user_message":
					if meta.Preview == "" {
						meta.Preview = shortPreview(e.Payload.Message)
					}
				}
			case "response_item":
				if isToolCall(e.Payload.Type) {
					increment(&meta.ToolCallCount)
				}
			default:
				continue
			}
		} else {
			if e.Type == "system" && e.Subtype == "turn_duration" && e.DurationMS > 0 {
				end := e.Timestamp.UnixMilli()
				spans = append(spans, interval{end - e.DurationMS, end})
				continue
			}
			if e.Type != "user" && e.Type != "assistant" && e.Type != "tool_result" {
				continue
			}
			if e.SessionID != "" {
				meta.Session = e.SessionID
			}
			if e.CWD != "" {
				meta.CWD = e.CWD
			}
			if e.GitBranch != "" {
				meta.Branch = e.GitBranch
			}
			if e.Slug != "" {
				meta.Title = e.Slug
			}
			meta.observeClaude(e)
		}
	}
}

func (e entry) codexSpan() (interval, bool) {
	end := e.Timestamp.UnixMilli()
	if e.Payload.CompletedAt > 0 && e.Payload.CompletedAt != e.Timestamp.Unix() {
		end = e.Payload.CompletedAt * 1000
	}
	if e.Payload.DurationMS > 0 {
		return interval{end - e.Payload.DurationMS, end}, true
	}
	start := e.Payload.StartedAt * 1000
	return interval{start, end}, start > 0 && start < end
}

// Codex writes large response bodies and snapshots after the timestamp and type.
func skipCodexBody(line []byte, meta *source) bool {
	if meta.Agent != "cx" {
		return false
	}
	rest, ok := bytes.CutPrefix(line, []byte(`{"timestamp":"`))
	if !ok {
		return false
	}
	_, rest, ok = bytes.Cut(rest, []byte(`","type":"`))
	if !ok {
		return false
	}
	if bytes.HasPrefix(rest, []byte(`world_state"`)) ||
		bytes.HasPrefix(rest, []byte(`compacted"`)) ||
		bytes.HasPrefix(rest, []byte(`thread_name"`)) {
		return true
	}
	if payload, ok := bytes.CutPrefix(rest, []byte(`response_item","payload":{"type":"`)); ok {
		kind, _, ok := bytes.Cut(payload, []byte(`"`))
		if !ok {
			return false
		}
		if isToolCall(string(kind)) {
			increment(&meta.ToolCallCount)
		}
		return true
	}
	return false
}

func projectName(cwd string) string {
	if cwd == "" {
		return "プロジェクト未設定"
	}
	return filepath.Base(cwd)
}

func increment(counter **int64) {
	if *counter == nil {
		*counter = new(int64)
	}
	**counter++
}

func isToolCall(kind string) bool {
	switch kind {
	case "function_call", "custom_tool_call", "local_shell_call", "web_search_call", "file_search_call", "computer_call", "mcp_call":
		return true
	default:
		return false
	}
}

func shortPreview(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 240 {
		return text
	}
	count := 0
	for offset := range text {
		if count == 240 {
			return strings.TrimSpace(text[:offset])
		}
		count++
	}
	return text
}

func cleanRemote(raw string) string {
	if !strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	u.User, u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = nil, "", "", "", false
	return u.String()
}
