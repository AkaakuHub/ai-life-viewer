package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-timeline/internal/index"
	"agent-timeline/internal/pricing"
	"agent-timeline/internal/timeline"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	store, err := index.Open("data/agent-timeline.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	go store.Watch(ctx, home)
	rates := pricing.Load(filepath.Join(home, ".t3/userdata/usage-model-rates.json"))
	go rates.Watch(ctx)
	server := http.Server{Addr: "127.0.0.1:4180", Handler: routes(store, rates, os.DirFS("dist")), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("listening on http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func routes(store *index.Store, rates *pricing.Source, dist fs.FS) http.Handler {
	mux := http.NewServeMux()
	rangeRoute := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		start, startErr := strconv.ParseInt(query.Get("start"), 10, 64)
		end, endErr := strconv.ParseInt(query.Get("end"), 10, 64)
		if startErr != nil || endErr != nil || start < -62135596800000 || end > 253402300799999 || end <= start || end-start > 366*timeline.Day {
			http.Error(w, "invalid date range", http.StatusBadRequest)
			return
		}
		options := index.Options{Agents: filters(query["agents"]), Projects: query["projects"]}
		blocks, err := store.Blocks(start, end, options, strings.TrimSpace(query.Get("q")))
		if err != nil {
			respond(w, struct{}{}, err)
			return
		}
		catalog := rates.Catalog()
		for i := range blocks {
			b := &blocks[i]
			b.EstimatedCostUSD = catalog.Estimate(b.Model, b.InputTokens, b.CachedInputTokens, b.OutputTokens)
		}
		summary := timeline.Summarize(blocks, start, end)
		if r.URL.Path == "/api/summary" {
			respond(w, summary, nil)
			return
		}
		respond(w, struct {
			Blocks []timeline.Block    `json:"blocks"`
			Days   []timeline.DayTotal `json:"days"`
		}{blocks, summary.Days}, nil)
	}
	mux.HandleFunc("GET /api/blocks", rangeRoute)
	mux.HandleFunc("GET /api/summary", rangeRoute)
	mux.HandleFunc("GET /api/filters", func(w http.ResponseWriter, r *http.Request) {
		out, err := store.Options()
		respond(w, out, err)
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		out, err := store.Status()
		respond(w, out, err)
	})
	mux.HandleFunc("GET /api/", http.NotFound)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); errors.Is(err, fs.ErrNotExist) && path.Ext(name) == "" {
			name = "index.html"
		}
		http.ServeFileFS(w, r, dist, name)
	})
	return mux
}

func respond[T interface{}](w http.ResponseWriter, out T, err error) {
	if err != nil {
		log.Print(err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Print(err)
	}
}

func filters(values []string) []string {
	if len(values) == 1 && values[0] != "" {
		return strings.Split(values[0], ",")
	}
	return values
}
