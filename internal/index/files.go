package index

import (
	"database/sql"
	"errors"
	"io"
	"os"
	"syscall"
)

type fileState struct {
	mtime, size, offset, device, inode int64
	version                            int
}

func (s *Store) indexFile(path, agent string, info os.FileInfo) error {
	stat := info.Sys().(*syscall.Stat_t)
	next := fileState{mtime: info.ModTime().UnixNano(), size: info.Size(), device: int64(stat.Dev), inode: int64(stat.Ino)}
	var last fileState
	err := s.db.QueryRow("SELECT mtime_ns,size,byte_offset,device,inode,metadata_version FROM files WHERE path=?", path).
		Scan(&last.mtime, &last.size, &last.offset, &last.device, &last.inode, &last.version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sameFile := err == nil && last.version == metadataVersion && last.device == next.device && last.inode == next.inode
	if sameFile && next.mtime == last.mtime && next.size == last.size {
		return nil
	}
	appendOnly := sameFile && next.size > last.size && next.mtime >= last.mtime
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	meta := source{Agent: agent}
	var spans []interval
	if appendOnly {
		next.offset = last.offset
		err := s.db.QueryRow(`SELECT session,title,cwd,model,branch,tokens,input_tokens,cached_input_tokens,output_tokens,
			reasoning_tokens,turn_count,tool_call_count,reasoning_effort,git_sha,git_origin_url,preview FROM sources WHERE key=?`, path).
			Scan(&meta.Session, &meta.Title, &meta.CWD, &meta.Model, &meta.Branch, &meta.Tokens,
				&meta.InputTokens, &meta.CachedInputTokens, &meta.OutputTokens, &meta.ReasoningTokens,
				&meta.TurnCount, &meta.ToolCallCount, &meta.ReasoningEffort, &meta.GitSHA, &meta.GitOriginURL, &meta.Preview)
		if err != nil {
			return err
		}
		rows, err := s.db.Query("SELECT start,last FROM blocks WHERE source=?", path)
		if err != nil {
			return err
		}
		for rows.Next() {
			var span interval
			if err := rows.Scan(&span.Start, &span.Last); err != nil {
				rows.Close()
				return err
			}
			spans = append(spans, span)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	reader := io.NewSectionReader(f, next.offset, next.size-next.offset)
	events, consumed, err := parseLog(reader, &meta)
	if err != nil {
		return err
	}
	next.offset += consumed
	meta.Project = projectName(meta.CWD)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveSource(tx, path, meta); err != nil {
		return err
	}
	if agent == "cl" {
		if err := saveClaudeUsage(tx, path, appendOnly, meta.claudeMessages); err != nil {
			return err
		}
	}
	if err := saveIntervals(tx, path, mergeTurnSpans(append(spans, events...))); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO files(path,mtime_ns,size,byte_offset,device,inode,metadata_version) VALUES(?,?,?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET
		mtime_ns=excluded.mtime_ns,size=excluded.size,byte_offset=excluded.byte_offset,
		device=excluded.device,inode=excluded.inode,metadata_version=excluded.metadata_version`, path, next.mtime, next.size, next.offset, next.device, next.inode, metadataVersion)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func saveSource(tx *sql.Tx, key string, m source) error {
	_, err := tx.Exec(`INSERT INTO sources(key,agent,session,title,project,cwd,model,branch,tokens,input_tokens,
		cached_input_tokens,output_tokens,reasoning_tokens,turn_count,tool_call_count,reasoning_effort,
		git_sha,git_origin_url,preview) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(key) DO UPDATE SET
		agent=excluded.agent,session=excluded.session,title=excluded.title,project=excluded.project,
		cwd=excluded.cwd,model=excluded.model,branch=excluded.branch,tokens=excluded.tokens,
		input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,
		output_tokens=excluded.output_tokens,reasoning_tokens=excluded.reasoning_tokens,
		turn_count=excluded.turn_count,tool_call_count=excluded.tool_call_count,
		reasoning_effort=excluded.reasoning_effort,git_sha=excluded.git_sha,
		git_origin_url=excluded.git_origin_url,preview=excluded.preview`,
		key, m.Agent, m.Session, m.Title, m.Project, m.CWD, m.Model, m.Branch, m.Tokens,
		m.InputTokens, m.CachedInputTokens, m.OutputTokens, m.ReasoningTokens, m.TurnCount,
		m.ToolCallCount, m.ReasoningEffort, m.GitSHA, m.GitOriginURL, m.Preview)
	return err
}

func saveIntervals(tx *sql.Tx, key string, spans []interval) error {
	if _, err := tx.Exec("DELETE FROM blocks WHERE source=?", key); err != nil {
		return err
	}
	if len(spans) == 0 {
		return nil
	}
	stmt, err := tx.Prepare("INSERT INTO blocks(source,start,last,end) VALUES(?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, span := range spans {
		if _, err := stmt.Exec(key, span.Start, span.Last, span.Last); err != nil {
			return err
		}
	}
	return nil
}
