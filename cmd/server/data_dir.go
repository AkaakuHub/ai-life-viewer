package main

import (
	"os"
	"path/filepath"
)

func dataDirectory(home string) (string, error) {
	if dir := os.Getenv("AGENT_TIMELINE_DATA_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Abs(filepath.Join(base, "ai-life-viewer"))
}
