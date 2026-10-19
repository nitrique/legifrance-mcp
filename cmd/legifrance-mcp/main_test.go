package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := requireBearer("s3cret", next)

	cases := map[string]int{
		"":                http.StatusUnauthorized,
		"Bearer wrong":    http.StatusUnauthorized,
		"Basic s3cret":    http.StatusUnauthorized,
		"Bearer s3cret":   http.StatusNoContent,
		"Bearer  s3cret ": http.StatusNoContent,
	}
	for header, want := range cases {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: got %d, want %d", header, rec.Code, want)
		}
	}
}

func TestLimitConcurrencyReleasesSlots(t *testing.T) {
	calls := 0
	h := limitConcurrency(1, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	for range 3 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("got %d", rec.Code)
		}
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}
