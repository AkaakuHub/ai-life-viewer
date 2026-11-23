package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

type Source struct {
	path    string
	file    os.FileInfo
	current atomic.Pointer[Catalog]
}

func Load(path string) *Source {
	s := &Source{path: path}
	s.current.Store(&Catalog{})
	if err := s.refresh(); err != nil {
		log.Printf("model rates: %v", err)
	}
	return s
}

func (s *Source) Catalog() *Catalog {
	return s.current.Load()
}

func (s *Source) Watch(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.refresh(); err != nil {
				log.Printf("model rates: %v", err)
			}
		}
	}
}

func (s *Source) refresh() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		if s.file != nil {
			s.file = nil
			s.current.Store(&Catalog{})
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if s.file != nil && os.SameFile(s.file, info) && info.ModTime().Equal(s.file.ModTime()) && info.Size() == s.file.Size() {
		return nil
	}

	c, err := decodeCatalog(f)
	if err != nil {
		return err
	}
	s.current.Store(c)
	s.file = info
	return nil
}

func decodeCatalog(reader io.Reader) (*Catalog, error) {
	var data struct{ Document map[string]json.RawMessage }
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return nil, err
	}
	if data.Document == nil {
		return nil, errors.New("missing document map")
	}
	c := &Catalog{exact: make(map[string]rates), normalized: make(map[string]rates)}
	keys := make([]string, 0, len(data.Document))
	for key := range data.Document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	best := make(map[string]string)
	for _, raw := range keys {
		var r rates
		if json.Unmarshal(data.Document[raw], &r) != nil || r.Input == nil || r.Output == nil ||
			*r.Input < 0 || *r.Output < 0 || (r.Cached != nil && *r.Cached < 0) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(raw))
		c.exact[key] = r
		model := normalize(key)
		if previous, exists := best[model]; !exists || len(key) < len(previous) ||
			(len(key) == len(previous) && c.normalized[model].Cached == nil && r.Cached != nil) {
			best[model] = key
			c.normalized[model] = r
		}
	}
	return c, nil
}
