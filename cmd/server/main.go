package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"agent-timeline/internal/ingest"
	"agent-timeline/internal/pricing"
	"agent-timeline/internal/timeline"
)

func main() {
	args := os.Args[1:]
	if len(args) != 0 && (len(args) != 1 || args[0] != "rotate-token") {
		log.Fatal("usage: agent-timeline [rotate-token]")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dataDir, err := dataDirectory(home)
	if err != nil {
		log.Fatal(err)
	}
	tokenPath := filepath.Join(dataDir, "ingest.token")
	if len(args) == 1 {
		token, err := ingest.RotateToken(tokenPath)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(token)
		return
	}
	store, err := index.Open(filepath.Join(dataDir, "agent-timeline.sqlite"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	auth, err := ingest.LoadAuth(tokenPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	go store.Watch(ctx, home)
	go store.WatchAccountUsage(ctx, dataDir)
	rates := pricing.Load(filepath.Join(home, ".t3/userdata/usage-model-rates.json"))
	go rates.Watch(ctx)
	server := http.Server{Addr: "127.0.0.1:4180", Handler: routes(store, rates, auth, os.DirFS("dist")), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("listening on http://%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func routes(store *index.Store, rates *pricing.Source, auth ingest.Auth, dist fs.FS) http.Handler {
	mux := http.NewServeMux()
	registerIngest(mux, store, auth)
	rangeRoute := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		allTime := r.URL.Path == "/api/summary" && !query.Has("start") && !query.Has("end")
		var start, end int64
		if allTime {
			var err error
			start, end, err = store.HistoryBounds()
			if err != nil {
				respond(w, struct{}{}, err)
				return
			}
		} else {
			var startErr, endErr error
			start, startErr = strconv.ParseInt(query.Get("start"), 10, 64)
			end, endErr = strconv.ParseInt(query.Get("end"), 10, 64)
			if startErr != nil || endErr != nil || start < -62135596800000 || end > 253402300799999 || end <= start || end-start > 366*timeline.Day {
				http.Error(w, "invalid date range", http.StatusBadRequest)
				return
			}
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
			catalog.Price(b)
		}
		blocks = timeline.UnionSessions(blocks)
		summary := timeline.Summarize(blocks, start, end)
		usage, err := store.AccountUsage(start, end)
		if err != nil {
			respond(w, struct{}{}, err)
			return
		}
		daily, err := store.DailyUsage(start, end)
		if err != nil {
			respond(w, struct{}{}, err)
			return
		}
		if r.URL.Path == "/api/summary" {
			var history *timeline.History
			if allTime {
				account := make(map[string]int64, len(usage.Days))
				for _, day := range usage.Days {
					account[day.StartDate] = day.Tokens
				}
				trend := timeline.SummarizeHistory(start, blocks, summary.Days, account)
				history = &trend
			}
			respond(w, struct {
				timeline.Summary
				AccountUsage index.AccountUsage  `json:"accountUsage"`
				History      *timeline.History   `json:"history,omitempty"`
				DailyUsage   []ingest.DailyUsage `json:"dailyUsage"`
			}{summary, usage, history, daily}, nil)
			return
		}
		respond(w, struct {
			Blocks       []timeline.CalendarBlock `json:"blocks"`
			Days         []timeline.DayTotal      `json:"days"`
			AccountUsage index.AccountUsage       `json:"accountUsage"`
			DailyUsage   []ingest.DailyUsage      `json:"dailyUsage"`
		}{timeline.CalendarBlocks(blocks, start, end), summary.Days, usage, daily}, nil)
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
