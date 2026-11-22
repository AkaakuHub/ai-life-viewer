package index

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"time"
)

type source struct {
	Agent, Session, Title, Project, CWD, Model, Branch string
	Tokens                                             *int64
}

type interval struct{ Start, Last int64 }

const minute = int64(time.Minute / time.Millisecond)

func (i interval) end() int64 { return max(i.Last, i.Start+minute) }

func mergeIntervals(spans []interval) []interval {
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	merged := spans[:0]
	for _, span := range spans {
		if len(merged) == 0 || span.Start-merged[len(merged)-1].Last > 5*minute {
			merged = append(merged, span)
			continue
		}
		last := &merged[len(merged)-1]
		last.Last = max(last.Last, span.Last)
	}
	return merged
}

type usage struct {
	Total *int64 `json:"total_tokens"`
}

type entry struct {
	Timestamp time.Time
	Type      string
	SessionID string
	CWD       string
	GitBranch string
	Slug      string
	Message   struct{ Model string }
	Payload   struct {
		ID          string
		CWD         string
		Model       string
		Type        string
		Git         struct{ Branch string }
		ThreadUsage usage `json:"thread_token_usage"`
		Info        struct {
			Total usage `json:"total_token_usage"`
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
		if stamp, ok := codexEventTime(line, meta.Agent); ok {
			if stamp != 0 {
				spans = append(spans, interval{stamp, stamp})
			}
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
				continue
			case "turn_context":
				if e.Payload.Model != "" {
					meta.Model = e.Payload.Model
				}
			case "token_usage_record":
				if e.Payload.ThreadUsage.Total != nil {
					meta.Tokens = e.Payload.ThreadUsage.Total
				}
			case "event_msg":
				if e.Payload.Type == "token_count" && e.Payload.Info.Total.Total != nil {
					meta.Tokens = e.Payload.Info.Total.Total
				}
			case "response_item":
			default:
				continue
			}
		} else {
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
			if e.Message.Model != "" {
				meta.Model = e.Message.Model
			}
		}
		stamp := e.Timestamp.UnixMilli()
		spans = append(spans, interval{stamp, stamp})
	}
}

// Codex writes large response bodies and snapshots after the timestamp and type.
func codexEventTime(line []byte, agent string) (int64, bool) {
	if agent != "cx" {
		return 0, false
	}
	rest, ok := bytes.CutPrefix(line, []byte(`{"timestamp":"`))
	if !ok {
		return 0, false
	}
	stamp, rest, ok := bytes.Cut(rest, []byte(`","type":"`))
	if !ok {
		return 0, false
	}
	if bytes.HasPrefix(rest, []byte(`world_state"`)) ||
		bytes.HasPrefix(rest, []byte(`compacted"`)) ||
		bytes.HasPrefix(rest, []byte(`thread_name"`)) {
		return 0, true
	}
	if !bytes.HasPrefix(rest, []byte(`response_item"`)) {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, string(stamp))
	return t.UnixMilli(), err == nil
}

func projectName(cwd string) string {
	if cwd == "" {
		return "プロジェクト未設定"
	}
	return filepath.Base(cwd)
}
