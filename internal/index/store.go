package index

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"agent-timeline/internal/timeline"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", uri(path, "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"))
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS sources (
			key TEXT PRIMARY KEY, agent TEXT NOT NULL, session TEXT NOT NULL,
			title TEXT NOT NULL, project TEXT NOT NULL, cwd TEXT NOT NULL,
			model TEXT NOT NULL, branch TEXT NOT NULL, tokens INTEGER
		);
		CREATE INDEX IF NOT EXISTS sources_session ON sources(agent,session);
		CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY REFERENCES sources(key) ON DELETE CASCADE,
			mtime_ns INTEGER NOT NULL, size INTEGER NOT NULL, byte_offset INTEGER NOT NULL,
			device INTEGER NOT NULL, inode INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS blocks (
			id INTEGER PRIMARY KEY, source TEXT NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
			start INTEGER NOT NULL, last INTEGER NOT NULL, end INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS blocks_end ON blocks(end);
		CREATE UNIQUE INDEX IF NOT EXISTS blocks_source_start ON blocks(source,start);
		CREATE TABLE IF NOT EXISTS watermarks (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS codex (
			session TEXT PRIMARY KEY, title TEXT NOT NULL, cwd TEXT NOT NULL,
			model TEXT NOT NULL, branch TEXT NOT NULL, tokens INTEGER NOT NULL, project TEXT NOT NULL
		);
		CREATE VIEW IF NOT EXISTS timeline AS
		SELECT b.id, s.agent, COALESCE(NULLIF(c.title,''),NULLIF(s.title,''),s.project) title,
		       CASE WHEN c.cwd IS NOT NULL AND c.cwd <> '' THEN c.cwd ELSE s.cwd END cwd,
		       CASE WHEN c.cwd IS NOT NULL AND c.cwd <> '' THEN c.project ELSE s.project END project, COALESCE(NULLIF(c.model,''),s.model) model,
		       COALESCE(NULLIF(c.branch,''),s.branch) branch,
		       COALESCE(NULLIF(c.tokens,0),s.tokens) tokens, b.start,b.end
		FROM blocks b JOIN sources s ON s.key=b.source
		LEFT JOIN files own ON own.path=s.key
		LEFT JOIN codex c ON s.agent='cx' AND c.session=s.session
		WHERE s.session='' OR NOT EXISTS (
			SELECT 1 FROM sources f JOIN files i ON i.path=f.key
			WHERE f.agent=s.agent AND f.session=s.session AND
			      (own.path IS NULL OR i.byte_offset > own.byte_offset OR
			       (i.byte_offset = own.byte_offset AND f.key > s.key))
			      AND EXISTS (SELECT 1 FROM blocks WHERE source=f.key)
		);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func uri(path, query string) string {
	return (&url.URL{Scheme: "file", Path: path, RawQuery: query}).String()
}

type Status struct {
	Files    int   `json:"indexedFiles"`
	Blocks   int   `json:"blockCount"`
	LastSync int64 `json:"lastSync"`
}

func (s *Store) Status() (Status, error) {
	var out Status
	err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM files),
		(SELECT COUNT(*) FROM timeline),
		COALESCE((SELECT value FROM watermarks WHERE key='sync'),0)`).Scan(&out.Files, &out.Blocks, &out.LastSync)
	return out, err
}

type Options struct {
	Agents   []string `json:"agents"`
	Projects []string `json:"projects"`
}

func (s *Store) Options() (Options, error) {
	out := Options{Agents: []string{}, Projects: []string{}}
	for column, list := range map[string]*[]string{"agent": &out.Agents, "project": &out.Projects} {
		rows, err := s.db.Query("SELECT DISTINCT " + column + " FROM timeline ORDER BY " + column)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return out, err
			}
			*list = append(*list, value)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *Store) Blocks(start, end int64, options Options, search string) ([]timeline.Block, error) {
	agents, err := json.Marshal(options.Agents)
	if err != nil {
		return nil, err
	}
	projects, err := json.Marshal(options.Projects)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id,agent,title,project,cwd,model,branch,tokens,start,end FROM timeline
		WHERE end>? AND start<?
		  AND (?=0 OR agent IN (SELECT value FROM json_each(?)))
		  AND (?=0 OR project IN (SELECT value FROM json_each(?)))
		  AND (?='' OR instr(lower(title || ' ' || project || ' ' || cwd),lower(?))>0)
	    ORDER BY start,end,id`, start, end, len(options.Agents), string(agents), len(options.Projects), string(projects), search, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []timeline.Block{}
	for rows.Next() {
		var b timeline.Block
		if err := rows.Scan(&b.ID, &b.Agent, &b.Title, &b.Project, &b.CWD, &b.Model, &b.Branch, &b.Tokens, &b.Start, &b.End); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
