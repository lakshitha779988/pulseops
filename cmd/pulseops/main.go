package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type application struct {
	metrics *metricStore
	logger  *log.Logger
}

func newApplication(out io.Writer) *application {
	return &application{metrics: newMetricStore(), logger: log.New(out, "", 0)}
}

func (a *application) logEvent(level, event string, status int, elapsed time.Duration) {
	record := map[string]any{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"level":      level,
		"service":    "pulseops",
		"event":      event,
		"status":     status,
		"latency_ms": elapsed.Milliseconds(),
	}
	b, _ := json.Marshal(record)
	a.logger.Print(string(b))
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, map[string]any{"service": "pulseops", "endpoints": []string{"/healthz", "/readyz", "/work", "/metrics"}})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); fmt.Fprint(w, "healthy\n") })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); fmt.Fprint(w, "ready\n") })
	mux.Handle("GET /metrics", a.metrics)
	mux.HandleFunc("GET /work", a.work)
	return mux
}

func (a *application) work(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	rawDelay := r.URL.Query().Get("delay_ms")
	delay := 0
	var err error
	if rawDelay != "" {
		delay, err = strconv.Atoi(rawDelay)
	}
	if err != nil || delay < 0 || delay > 2500 {
		sendJSON(w, 400, map[string]string{"error": "delay_ms must be between 0 and 2500"})
		a.metrics.observe("/work", 400, time.Since(started).Seconds())
		a.logEvent("warn", "invalid_request", 400, time.Since(started))
		return
	}
	if delay > 0 {
		select {
		case <-time.After(time.Duration(delay) * time.Millisecond):
		case <-r.Context().Done():
			return
		}
	}
	failure := strings.EqualFold(r.URL.Query().Get("fail"), "true") || r.URL.Query().Get("fail") == "1"
	status, level, event := 200, "info", "request_completed"
	if failure {
		status, level, event = 500, "error", "simulated_failure"
	}
	sendJSON(w, status, map[string]any{"success": !failure, "simulated_delay_ms": delay})
	elapsed := time.Since(started)
	a.metrics.observe("/work", status, elapsed.Seconds())
	a.logEvent(level, event, status, elapsed)
}

func sendJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func outputWriter() io.Writer {
	path := os.Getenv("LOG_FILE")
	if path == "" {
		return os.Stdout
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		log.Printf("cannot create log directory, using stdout: %v", err)
		return os.Stdout
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		log.Printf("cannot create log file, using stdout: %v", err)
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, f)
}

func main() {
	app := newApplication(outputWriter())
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: app.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("PulseOps listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}
