package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(nil, env(map[string]string{
		"LEGIFRANCE_CLIENT_ID":     "id",
		"LEGIFRANCE_CLIENT_SECRET": "secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != EnvProduction || cfg.Transport != TransportStdio {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if !strings.HasPrefix(cfg.APIBaseURL, "https://api.piste.gouv.fr/") || cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("unexpected URLs or timeout: %s %s", cfg.APIBaseURL, cfg.HTTPTimeout)
	}
}

func TestLoadFlagsOverrideEnv(t *testing.T) {
	cfg, err := Load([]string{"-env", "sandbox", "-transport", "http", "-addr", ":9000"}, env(map[string]string{
		"LEGIFRANCE_CLIENT_ID":     "id",
		"LEGIFRANCE_CLIENT_SECRET": "secret",
		"MCP_TRANSPORT":            "stdio",
		"MCP_AUTH_TOKEN":           strings.Repeat("t", 32),
		"LEGIFRANCE_TIMEOUT":       "10",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != TransportHTTP || cfg.HTTPAddr != ":9000" || cfg.HTTPTimeout != 10*time.Second {
		t.Errorf("flags not applied: %+v", cfg)
	}
	if !strings.Contains(cfg.TokenURL, "sandbox-oauth") {
		t.Errorf("sandbox token URL expected, got %s", cfg.TokenURL)
	}
}

func TestLoadValidation(t *testing.T) {
	cases := map[string]map[string]string{
		"missing credentials": {},
		"plain http API":      {"LEGIFRANCE_CLIENT_ID": "id", "LEGIFRANCE_CLIENT_SECRET": "s", "LEGIFRANCE_API_URL": "http://example.com"},
		"unknown transport":   {"LEGIFRANCE_CLIENT_ID": "id", "LEGIFRANCE_CLIENT_SECRET": "s", "MCP_TRANSPORT": "grpc"},
		"unknown environment": {"LEGIFRANCE_CLIENT_ID": "id", "LEGIFRANCE_CLIENT_SECRET": "s", "LEGIFRANCE_ENV": "staging"},
		"http without token":  {"LEGIFRANCE_CLIENT_ID": "id", "LEGIFRANCE_CLIENT_SECRET": "s", "MCP_TRANSPORT": "http"},
		"http short token":    {"LEGIFRANCE_CLIENT_ID": "id", "LEGIFRANCE_CLIENT_SECRET": "s", "MCP_TRANSPORT": "http", "MCP_AUTH_TOKEN": "short"},
	}
	for name, values := range cases {
		if _, err := Load(nil, env(values)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadHTTPWithoutTokenWhenExplicitlyAllowed(t *testing.T) {
	cfg, err := Load(nil, env(map[string]string{
		"LEGIFRANCE_CLIENT_ID":      "id",
		"LEGIFRANCE_CLIENT_SECRET":  "secret",
		"MCP_TRANSPORT":             "http",
		"MCP_ALLOW_UNAUTHENTICATED": "true",
	}))
	if err != nil || !cfg.AllowUnauthenticated {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
