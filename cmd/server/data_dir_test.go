package main

import (
	"path/filepath"
	"testing"
)

func TestDataDirectoryPrecedence(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	override := filepath.Join(home, "override")
	for _, c := range []struct {
		xdg, override, want string
	}{
		{"", "", filepath.Join(home, ".local", "share", "ai-life-viewer")},
		{xdg, "", filepath.Join(xdg, "ai-life-viewer")},
		{xdg, override, override},
	} {
		t.Setenv("XDG_DATA_HOME", c.xdg)
		t.Setenv("AGENT_TIMELINE_DATA_DIR", c.override)
		got, err := dataDirectory(home)
		if err != nil || got != c.want {
			t.Fatalf("data directory=%q err=%v, want %q", got, err, c.want)
		}
	}
}
