package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthyAndReady(t *testing.T) {
	app := newApplication(io.Discard)
	for _, endpoint := range []string{"/healthz", "/readyz"} {
		r := httptest.NewRecorder()
		app.routes().ServeHTTP(r, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if r.Code != http.StatusOK {
			t.Fatalf("%s returned %d", endpoint, r.Code)
		}
	}
}

func TestSimulatedFailureAndMetrics(t *testing.T) {
	app := newApplication(io.Discard)
	handler := app.routes()
	fail := httptest.NewRecorder()
	handler.ServeHTTP(fail, httptest.NewRequest("GET", "/work?fail=true", nil))
	if fail.Code != 500 {
		t.Fatalf("wanted 500; got %d", fail.Code)
	}
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `pulseops_http_requests_total{route="/work",status="500"} 1`) {
		t.Fatalf("missing failure metric: %s", metrics.Body.String())
	}
}

func TestDelayValidation(t *testing.T) {
	app := newApplication(io.Discard)
	r := httptest.NewRecorder()
	app.routes().ServeHTTP(r, httptest.NewRequest("GET", "/work?delay_ms=9000", nil))
	if r.Code != 400 {
		t.Fatalf("wanted 400; got %d", r.Code)
	}
}
