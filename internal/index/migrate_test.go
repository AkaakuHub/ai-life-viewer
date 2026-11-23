package index

import (
	"database/sql"
	"testing"
)

func TestMigrationRetainsHistoricalBlocks(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE sources(key TEXT PRIMARY KEY, agent TEXT, session TEXT, title TEXT,
		  project TEXT, cwd TEXT, model TEXT, branch TEXT, tokens INTEGER);
		CREATE TABLE files(path TEXT PRIMARY KEY, mtime_ns INTEGER, size INTEGER,
		  byte_offset INTEGER, device INTEGER, inode INTEGER);
		CREATE TABLE blocks(id INTEGER PRIMARY KEY, source TEXT, start INTEGER, last INTEGER, end INTEGER);
		INSERT INTO sources VALUES('missing.jsonl','cx','example','Archived','project','','','',42);
		INSERT INTO files VALUES('missing.jsonl',1,1000,1000,1,1);
		INSERT INTO blocks VALUES(9,'missing.jsonl',1,60,60);
		CREATE VIEW timeline AS SELECT id FROM blocks;
	`)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := initialize(db); err != nil {
			t.Fatal(err)
		}
	}
	var offset int64
	var version int
	var tokens int
	var input *int
	if err := db.QueryRow(`SELECT f.byte_offset,f.metadata_version,t.tokens,t.input_tokens
		FROM files f JOIN timeline t ON t.source=f.path WHERE t.id=9`).
		Scan(&offset, &version, &tokens, &input); err != nil {
		t.Fatal(err)
	}
	if offset != 1000 || version != 0 || tokens != 42 || input != nil {
		t.Fatalf("offset=%d version=%d tokens=%d input=%v", offset, version, tokens, input)
	}
}
