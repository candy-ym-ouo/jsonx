package main
import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
	jerrors "jsonx/errors"
	"jsonx/jsonx"
	"jsonx/parser"
)
//go:embed static/*
var assets embed.FS
type request struct {
	JSON     string `json:"json"`
	Schema   string `json:"schema"`
	Path     string `json:"path"`
	Indent   string `json:"indent"`
	SortKeys bool   `json:"sortKeys"`
	Strict   bool   `json:"strict"`
}
type apiError struct {
	Code    string `json:"code"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Offset  int    `json:"offset"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Snippet string `json:"snippet"`
}
func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := assets.ReadFile("static/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]any{"status": "ok", "version": jsonx.Version()})
	})
	mux.HandleFunc("/api/parse", post(parseAPI))
	mux.HandleFunc("/api/format", post(formatAPI))
	mux.HandleFunc("/api/validate", post(validateAPI))
	mux.HandleFunc("/api/path", post(pathAPI))
	server := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	displayAddr := *addr
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "127.0.0.1" + displayAddr
	}
	log.Printf("jsonx web UI listening on http://%s", displayAddr)
	log.Fatal(server.ListenAndServe())
}
func post(next func(http.ResponseWriter, request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			respondError(w, err)
			return
		}
		var req request
		if err := jsonx.Decode(data, &req); err != nil {
			respondError(w, err)
			return
		}
		next(w, req)
	}
}
func parseAPI(w http.ResponseWriter, req request) {
	start := time.Now()
	v, err := jsonx.Parse([]byte(req.JSON))
	if err != nil {
		respondError(w, err)
		return
	}
	nodes, depth := parser.Stats(v)
	respond(w, map[string]any{"ok": true, "stats": map[string]any{"nodes": nodes, "depth": depth, "bytes": len(req.JSON), "durationMs": float64(time.Since(start).Microseconds()) / 1000}})
}
func formatAPI(w http.ResponseWriter, req request) {
	out, err := jsonx.Format([]byte(req.JSON), req.Indent, jsonx.SortKeys(req.SortKeys))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, map[string]any{"ok": true, "result": string(out)})
}
func validateAPI(w http.ResponseWriter, req request) {
	var err error
	if req.Schema != "" {
		err = jsonx.ValidateSchema([]byte(req.JSON), []byte(req.Schema))
	} else {
		err = jsonx.Validate([]byte(req.JSON))
	}
	if err != nil {
		respond(w, map[string]any{"ok": true, "valid": false, "errors": errorsOf(err)})
		return
	}
	respond(w, map[string]any{"ok": true, "valid": true})
}
func pathAPI(w http.ResponseWriter, req request) {
	v, err := jsonx.PathGet([]byte(req.JSON), req.Path)
	if err != nil {
		respondError(w, err)
		return
	}
	raw, err := jsonx.Marshal(v)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, map[string]any{"ok": true, "value": v.Any(true), "raw": string(raw)})
}
func respond(w http.ResponseWriter, value any) {
	data, err := jsonx.Marshal(value, jsonx.SortKeys(true))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
func respondError(w http.ResponseWriter, err error) {
	respond(w, map[string]any{"ok": false, "errors": errorsOf(err)})
}
func errorsOf(err error) []apiError {
	list := []error{err}
	if many, ok := err.(jerrors.ErrorList); ok {
		list = []error(many)
	}
	out := make([]apiError, 0, len(list))
	for _, item := range list {
		var row apiError
		if e, ok := item.(*jerrors.Error); ok {
			row = apiError{Code: e.Code(), Line: e.Position().Line, Column: e.Position().Column, Offset: e.Position().Offset, Path: e.Path().String(), Message: e.Message, Snippet: e.Snippet()}
		} else {
			row = apiError{Message: item.Error(), Path: "$"}
		}
		out = append(out, row)
	}
	return out
}
var _ = fmt.Sprintf
