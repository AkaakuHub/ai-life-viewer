package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"agent-timeline/internal/index"
	"agent-timeline/internal/ingest"
)

//go:embed schema.json
var schema []byte

func registerIngest(mux *http.ServeMux, store *index.Store, auth ingest.Auth) {
	mux.HandleFunc("GET /api/schema", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(schema)
	})
	mux.HandleFunc("POST /api/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		if !auth.Valid(r.Header) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			ingestError(w, http.StatusUnauthorized, "unauthorized", ingest.Issue{
				Field: "Authorization", Message: "provide a valid Authorization: Bearer <token> header",
			})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, ingest.MaxBodyBytes)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var batch ingest.Batch
		if err := decoder.Decode(&batch); err != nil {
			ingestError(w, http.StatusBadRequest, "invalid_request", ingest.Issue{Field: "body", Message: fmt.Sprintf("invalid JSON: %v", err)})
			return
		}
		var extra json.RawMessage
		if err := decoder.Decode(&extra); err != io.EOF {
			ingestError(w, http.StatusBadRequest, "invalid_request", ingest.Issue{Field: "body", Message: "send exactly one JSON object (maximum 1 MiB)"})
			return
		}
		if issue := batch.Validate(); issue != nil {
			ingestError(w, http.StatusBadRequest, "invalid_request", *issue)
			return
		}
		if err := store.Ingest(batch); err != nil {
			log.Printf("ingest write: %v", err)
			ingestError(w, http.StatusInternalServerError, "write_failed", ingest.Issue{Message: "the batch was not saved"})
			return
		}
		respond(w, struct {
			Records    int `json:"records"`
			DailyUsage int `json:"dailyUsage"`
		}{len(batch.Records), len(batch.DailyUsage)}, nil)
	})
}

func ingestError(w http.ResponseWriter, status int, code string, issue ingest.Issue) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
		ingest.Issue
	}{code, issue})
}
