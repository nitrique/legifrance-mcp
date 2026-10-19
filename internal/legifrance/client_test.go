package legifrance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakePiste simule le serveur OAuth et l'API Légifrance.
type fakePiste struct {
	tokenCalls atomic.Int32
	apiCalls   atomic.Int32
	handler    func(w http.ResponseWriter, r *http.Request)
}

func newTestClient(t *testing.T, f *fakePiste) *Client {
	t.Helper()
	return newTestClientWith(t, f, nil)
}

func newTestClientWith(t *testing.T, f *fakePiste, tweak func(*Options)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse token form: %v", err)
		}
		if r.PostForm.Get("grant_type") != "client_credentials" ||
			r.PostForm.Get("client_id") != "id" || r.PostForm.Get("client_secret") != "secret" {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		f.apiCalls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		f.handler(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	opts := Options{
		ClientID:      "id",
		ClientSecret:  "secret",
		TokenURL:      srv.URL + "/oauth/token",
		BaseURL:       srv.URL + "/api",
		BaseTransport: srv.Client().Transport,
		RetryDelay:    time.Millisecond,
	}
	if tweak != nil {
		tweak(&opts)
	}
	c, err := NewClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGetArticleSendsAuthenticatedRequest(t *testing.T) {
	f := &fakePiste{handler: func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/consult/getArticle" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["id"] != "LEGIARTI000006417749" {
			t.Errorf("body = %v (%v)", body, err)
		}
		_, _ = io.WriteString(w, `{"article":{"id":"LEGIARTI000006417749","num":"1240","etat":"VIGUEUR",
			"dateDebut":1475280000000,"dateFin":32472144000000,"texteHtml":"<p>Tout fait quelconque</p>"}}`)
	}}
	c := newTestClient(t, f)

	for range 2 {
		art, err := c.GetArticle(context.Background(), "LEGIARTI000006417749")
		if err != nil {
			t.Fatal(err)
		}
		if art.Num != "1240" || art.DateDebut.String() != "2016-10-01" || !art.DateFin.OpenEnded() {
			t.Errorf("unexpected article: %+v", art)
		}
	}
	if n := f.tokenCalls.Load(); n != 1 {
		t.Errorf("token fetched %d times, want 1 (cached)", n)
	}
}

func TestRetryOnServiceUnavailable(t *testing.T) {
	f := &fakePiste{}
	f.handler = func(w http.ResponseWriter, _ *http.Request) {
		if f.apiCalls.Load() < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"totalResultNumber":1,"results":[]}`)
	}
	c := newTestClient(t, f)

	resp, err := c.Search(context.Background(), SearchRequest{Fond: "ALL"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalResultNumber != 1 || f.apiCalls.Load() != 3 {
		t.Errorf("total=%d calls=%d", resp.TotalResultNumber, f.apiCalls.Load())
	}
}

func TestNotFoundIsNotRetried(t *testing.T) {
	f := &fakePiste{handler: func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}}
	c := newTestClient(t, f)

	_, err := c.GetJuri(context.Background(), "JURITEXT000000000001")
	if !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
	if f.apiCalls.Load() != 1 {
		t.Errorf("calls = %d, want 1", f.apiCalls.Load())
	}
}

func TestEmptyDocumentIsNotFound(t *testing.T) {
	f := &fakePiste{handler: func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"article":null}`)
	}}
	c := newTestClient(t, f)

	if _, err := c.GetArticle(context.Background(), "LEGIARTI000000000001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAuthenticationFailure(t *testing.T) {
	f := &fakePiste{handler: func(http.ResponseWriter, *http.Request) {}}
	c := newTestClientWith(t, f, func(o *Options) { o.ClientSecret = "wrong" })

	_, err := c.Search(context.Background(), SearchRequest{})
	if err == nil || !strings.Contains(err.Error(), "authentication failed (HTTP 401)") {
		t.Fatalf("err = %v, want authentication failure", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Errorf("error leaks the secret: %v", err)
	}
	if f.apiCalls.Load() != 0 || f.tokenCalls.Load() != 1 {
		t.Errorf("api calls = %d, token calls = %d", f.apiCalls.Load(), f.tokenCalls.Load())
	}
}

func TestDateUnmarshal(t *testing.T) {
	cases := map[string]string{
		`1475280000000`:                  "2016-10-01",
		`"1475280000000"`:                "2016-10-01",
		`"2021-04-15"`:                   "2021-04-15",
		`"2021-04-15T16:49:47.707+0000"`: "2021-04-15",
		`"2021-04-15T00:00:00Z"`:         "2021-04-15",
		`null`:                           "",
		`""`:                             "",
		`"n/a"`:                          "",
	}
	for raw, want := range cases {
		var d Date
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if d.String() != want {
			t.Errorf("%s: got %q, want %q", raw, d.String(), want)
		}
	}
}

func TestSuggestValuesWalksNestedMaps(t *testing.T) {
	raw := `{"results":{"LODA":{"LEGITEXT000006072050":{"id":"LEGITEXT000006072050","label":"Code du travail","origin":"LEGI"}},
		"JURI":{"x":[{"id":"JURITEXT000000000001","label":"Arrêt"}]}}}`
	var resp SuggestResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if got := len(resp.Values()); got != 2 {
		t.Errorf("got %d suggestions, want 2", got)
	}
}

func TestRetryAfterIsHonoredAndContextCancels(t *testing.T) {
	f := &fakePiste{handler: func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		http.Error(w, "quota", http.StatusTooManyRequests)
	}}
	c := newTestClient(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Search(ctx, SearchRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded while waiting for Retry-After", err)
	}
	if time.Since(start) > 2*time.Second || f.apiCalls.Load() != 1 {
		t.Errorf("waited %s with %d calls", time.Since(start), f.apiCalls.Load())
	}
}

func TestFlexString(t *testing.T) {
	var v struct {
		A, B, C, D FlexString
	}
	if err := json.Unmarshal([]byte(`{"A":"L1234-5","B":1240,"C":null,"D":{"x":1}}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != "L1234-5" || v.B != "1240" || v.C != "" || v.D != "" {
		t.Errorf("unexpected values: %+v", v)
	}
}

func TestForbiddenGivesSubscriptionHint(t *testing.T) {
	f := &fakePiste{handler: func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.Copy(w, r.Body) // la passerelle PISTE renvoie la requête
	}}
	c := newTestClient(t, f)

	_, err := c.Search(context.Background(), SearchRequest{Fond: "ALL"})
	if err == nil || !strings.Contains(err.Error(), "subscribed to the Légifrance API") || strings.Contains(err.Error(), `"fond"`) {
		t.Fatalf("err = %v", err)
	}
}
