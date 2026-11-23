package index

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type logFile struct {
	path, agent string
	info        fs.FileInfo
}

func (s *Store) Watch(ctx context.Context, home string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher: %v", err)
	}
	var events <-chan fsnotify.Event
	if watcher != nil {
		defer watcher.Close()
		// Drain events while the initial index streams large files.
		invalidate := make(chan fsnotify.Event, 1)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if !event.Has(fsnotify.Chmod) {
						select {
						case invalidate <- event:
						default:
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Printf("watcher: %v", err)
				}
			}
		}()
		events = invalidate
	}
	s.sync(ctx, home, watcher, true)
	poll := time.NewTicker(30 * time.Second)
	defer poll.Stop()
	var timer *time.Timer
	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-events:
			if timer == nil {
				timer = time.NewTimer(time.Second)
			} else {
				timer.Reset(time.Second)
			}
			debounce = timer.C
		case <-debounce:
			debounce = nil
			s.sync(ctx, home, watcher, false)
		case <-poll.C:
			s.sync(ctx, home, watcher, true)
		}
	}
}

func (s *Store) sync(ctx context.Context, home string, w *fsnotify.Watcher, metadata bool) {
	ok := true
	report := func(err error) {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			ok = false
			log.Printf("index: %v", err)
		}
	}
	if metadata {
		report(s.syncCodex(filepath.Join(home, ".codex/state_5.sqlite")))
		report(s.syncCodexTurns(filepath.Join(home, ".codex/thread_history_1.sqlite")))
		report(s.syncT3(filepath.Join(home, ".t3/userdata/statev2.sqlite")))
	}
	var files []logFile
	watched := map[string]bool{}
	if w != nil {
		for _, dir := range w.WatchList() {
			watched[dir] = true
		}
	}
	for _, root := range []struct{ dir, agent string }{
		{".codex/sessions", "cx"}, {".codex/archived_sessions", "cx"}, {".claude/projects", "cl"},
	} {
		report(filepath.WalkDir(filepath.Join(home, root.dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				report(err)
				return nil
			}
			if entry.IsDir() {
				if w != nil && !watched[path] {
					report(w.Add(path))
				}
				return nil
			}
			if !entry.Type().IsRegular() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				report(err)
				return nil
			}
			files = append(files, logFile{path, root.agent, info})
			return nil
		}))
	}
	slices.SortFunc(files, func(a, b logFile) int { return b.info.ModTime().Compare(a.info.ModTime()) })
	for _, f := range files {
		if ctx.Err() != nil {
			return
		}
		report(s.indexFile(f.path, f.agent, f.info))
	}
	if ok {
		_, err := s.db.Exec("INSERT INTO watermarks VALUES('sync',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", time.Now().UnixMilli())
		report(err)
	}
}
