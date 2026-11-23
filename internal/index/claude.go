package index

import (
	"database/sql"
	"encoding/json"
)

type claudeMessage struct {
	ID, Model string
	Content   json.RawMessage
	Usage     struct {
		Input      *int64 `json:"input_tokens"`
		Cached     *int64 `json:"cache_read_input_tokens"`
		CacheWrite *int64 `json:"cache_creation_input_tokens"`
		Output     *int64 `json:"output_tokens"`
	}
}

type claudeUsage struct {
	input, cached, output *int64
	tools                 map[string]bool
}

func (m *source) observeClaude(e entry) {
	msg := e.Message
	if msg.Model != "" {
		m.Model = msg.Model
	}
	var content []struct{ Type, ID, Text string }
	_ = json.Unmarshal(msg.Content, &content)
	if e.Type == "user" && !e.IsMeta {
		var text string
		_ = json.Unmarshal(msg.Content, &text)
		for _, part := range content {
			if part.Type == "text" && text == "" {
				text = part.Text
			}
		}
		if text != "" {
			increment(&m.TurnCount)
			if m.Preview == "" {
				m.Preview = shortPreview(text)
			}
		}
	}
	if e.Type != "assistant" || msg.ID == "" {
		return
	}
	if m.claudeMessages == nil {
		m.claudeMessages = make(map[string]claudeUsage)
	}
	current := m.claudeMessages[msg.ID]
	u := msg.Usage
	if u.Input != nil {
		total := *u.Input
		if u.Cached != nil {
			total += *u.Cached
		}
		if u.CacheWrite != nil {
			total += *u.CacheWrite
		}
		current.input = peak(current.input, &total)
	}
	current.cached = peak(current.cached, u.Cached)
	current.output = peak(current.output, u.Output)
	if current.tools == nil {
		current.tools = make(map[string]bool)
	}
	for _, part := range content {
		if part.Type == "tool_use" && part.ID != "" {
			current.tools[part.ID] = true
		}
	}
	m.claudeMessages[msg.ID] = current
}

func peak(a, b *int64) *int64 {
	if a == nil || (b != nil && *b > *a) {
		return b
	}
	return a
}

func saveClaudeUsage(tx *sql.Tx, path string, appendOnly bool, messages map[string]claudeUsage) error {
	if !appendOnly {
		for _, table := range []string{"claude_messages", "claude_tools"} {
			if _, err := tx.Exec("DELETE FROM "+table+" WHERE source=?", path); err != nil {
				return err
			}
		}
	}
	for id, m := range messages {
		if _, err := tx.Exec(`INSERT INTO claude_messages VALUES(?,?,?,?,?)
			ON CONFLICT(source,message) DO UPDATE SET
			input_tokens=COALESCE(MAX(input_tokens,excluded.input_tokens),input_tokens,excluded.input_tokens),
			cached_input_tokens=COALESCE(MAX(cached_input_tokens,excluded.cached_input_tokens),cached_input_tokens,excluded.cached_input_tokens),
			output_tokens=COALESCE(MAX(output_tokens,excluded.output_tokens),output_tokens,excluded.output_tokens)`,
			path, id, m.input, m.cached, m.output); err != nil {
			return err
		}
		for tool := range m.tools {
			if _, err := tx.Exec("INSERT OR IGNORE INTO claude_tools VALUES(?,?)", path, tool); err != nil {
				return err
			}
		}
	}
	_, err := tx.Exec(`UPDATE sources SET
		(input_tokens,cached_input_tokens,output_tokens)=(SELECT SUM(input_tokens),SUM(cached_input_tokens),SUM(output_tokens)
			FROM claude_messages WHERE source=?),
		tool_call_count=(SELECT COUNT(*) FROM claude_tools WHERE source=?) WHERE key=?`, path, path, path)
	return err
}
