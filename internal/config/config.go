// Package config charge et valide la configuration du serveur MCP Légifrance.
//
// La configuration est lue depuis les variables d'environnement, puis
// éventuellement surchargée par les options de ligne de commande.
package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Environnements PISTE connus : chaque environnement possède sa propre URL
// d'API et son propre serveur OAuth.
const (
	EnvProduction = "production"
	EnvSandbox    = "sandbox"
)

// Modes de transport MCP supportés.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// minAuthTokenLength est la longueur minimale du jeton Bearer (ex. « openssl rand -hex 32 »).
const minAuthTokenLength = 32

var environments = map[string]struct{ apiURL, oauthURL string }{
	EnvProduction: {
		apiURL:   "https://api.piste.gouv.fr/dila/legifrance/lf-engine-app",
		oauthURL: "https://oauth.piste.gouv.fr/api/oauth/token",
	},
	EnvSandbox: {
		apiURL:   "https://sandbox-api.piste.gouv.fr/dila/legifrance/lf-engine-app",
		oauthURL: "https://sandbox-oauth.piste.gouv.fr/api/oauth/token",
	},
}

// Config regroupe l'ensemble des paramètres du serveur.
type Config struct {
	// Identifiants de l'application PISTE (flux OAuth2 client credentials).
	ClientID     string
	ClientSecret string

	// Environment vaut "production" ou "sandbox" ; il détermine les URL par défaut.
	Environment string
	APIBaseURL  string
	TokenURL    string

	// HTTPTimeout borne la durée de chaque appel à l'API Légifrance.
	HTTPTimeout time.Duration

	// Transport vaut "stdio" (usage local) ou "http" (Streamable HTTP, usage conteneurisé).
	Transport string
	HTTPAddr  string
	// AuthToken est exigé en jeton Bearer sur le transport HTTP.
	AuthToken string
	// AllowUnauthenticated autorise explicitement le transport HTTP sans jeton
	// (par exemple derrière une passerelle qui authentifie déjà les appels).
	AllowUnauthenticated bool
	// MaxConcurrency borne le nombre de requêtes MCP traitées simultanément en HTTP.
	MaxConcurrency int

	// MaxContentLength est la taille par défaut (en caractères) des contenus renvoyés au client.
	MaxContentLength int

	LogLevel slog.Level
}

// Load construit la configuration à partir de l'environnement (via getenv)
// et des arguments de ligne de commande (sans le nom du programme).
func Load(args []string, getenv func(string) string) (Config, error) {
	env := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}

	timeout, err := parseDuration(env("LEGIFRANCE_TIMEOUT", "30s"))
	if err != nil {
		return Config{}, fmt.Errorf("LEGIFRANCE_TIMEOUT: %w", err)
	}
	maxLen, err := strconv.Atoi(env("MCP_MAX_CONTENT_LENGTH", "20000"))
	if err != nil {
		return Config{}, fmt.Errorf("MCP_MAX_CONTENT_LENGTH: %w", err)
	}
	maxConcurrency, err := strconv.Atoi(env("MCP_MAX_CONCURRENCY", "8"))
	if err != nil {
		return Config{}, fmt.Errorf("MCP_MAX_CONCURRENCY: %w", err)
	}
	allowUnauth, err := strconv.ParseBool(env("MCP_ALLOW_UNAUTHENTICATED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("MCP_ALLOW_UNAUTHENTICATED: %w", err)
	}

	cfg := Config{
		ClientID:             env("LEGIFRANCE_CLIENT_ID", ""),
		ClientSecret:         env("LEGIFRANCE_CLIENT_SECRET", ""),
		Environment:          strings.ToLower(env("LEGIFRANCE_ENV", EnvProduction)),
		APIBaseURL:           env("LEGIFRANCE_API_URL", ""),
		TokenURL:             env("LEGIFRANCE_TOKEN_URL", ""),
		HTTPTimeout:          timeout,
		Transport:            strings.ToLower(env("MCP_TRANSPORT", TransportStdio)),
		HTTPAddr:             env("MCP_HTTP_ADDR", ":8080"),
		AuthToken:            env("MCP_AUTH_TOKEN", ""),
		AllowUnauthenticated: allowUnauth,
		MaxConcurrency:       maxConcurrency,
		MaxContentLength:     maxLen,
	}
	logLevel := env("LOG_LEVEL", "info")

	// Les secrets ne sont volontairement pas exposés en flags : ils
	// apparaîtraient dans la liste des processus.
	fs := flag.NewFlagSet("legifrance-mcp", flag.ContinueOnError)
	fs.StringVar(&cfg.Environment, "env", cfg.Environment, "environnement PISTE : production ou sandbox")
	fs.StringVar(&cfg.Transport, "transport", cfg.Transport, "transport MCP : stdio ou http")
	fs.StringVar(&cfg.HTTPAddr, "addr", cfg.HTTPAddr, "adresse d'écoute du transport http")
	fs.DurationVar(&cfg.HTTPTimeout, "timeout", cfg.HTTPTimeout, "délai maximal d'un appel à l'API Légifrance")
	fs.IntVar(&cfg.MaxContentLength, "max-content-length", cfg.MaxContentLength, "taille maximale par défaut des contenus renvoyés")
	fs.StringVar(&logLevel, "log-level", logLevel, "niveau de log : debug, info, warn ou error")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(logLevel)); err != nil {
		return Config{}, fmt.Errorf("log level: %w", err)
	}
	cfg.Environment = strings.ToLower(cfg.Environment)
	cfg.Transport = strings.ToLower(cfg.Transport)

	if err := cfg.resolveURLs(); err != nil {
		return Config{}, err
	}
	return cfg, cfg.validate()
}

// resolveURLs complète les URL non surchargées à partir de l'environnement choisi.
func (c *Config) resolveURLs() error {
	defaults, ok := environments[c.Environment]
	if !ok {
		return fmt.Errorf("unknown environment %q (expected %q or %q)", c.Environment, EnvProduction, EnvSandbox)
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = defaults.apiURL
	}
	if c.TokenURL == "" {
		c.TokenURL = defaults.oauthURL
	}
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	return nil
}

func (c *Config) validate() error {
	var errs []error
	if c.ClientID == "" || c.ClientSecret == "" {
		errs = append(errs, errors.New("LEGIFRANCE_CLIENT_ID and LEGIFRANCE_CLIENT_SECRET are required"))
	}
	for name, raw := range map[string]string{"API URL": c.APIBaseURL, "token URL": c.TokenURL} {
		if err := validateHTTPSURL(raw); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	if c.Transport != TransportStdio && c.Transport != TransportHTTP {
		errs = append(errs, fmt.Errorf("unknown transport %q (expected %q or %q)", c.Transport, TransportStdio, TransportHTTP))
	}
	if c.Transport == TransportHTTP {
		switch {
		case c.AuthToken == "" && !c.AllowUnauthenticated:
			errs = append(errs, errors.New("MCP_AUTH_TOKEN is required with the http transport (or set MCP_ALLOW_UNAUTHENTICATED=true)"))
		case c.AuthToken != "" && len(c.AuthToken) < minAuthTokenLength:
			errs = append(errs, fmt.Errorf("MCP_AUTH_TOKEN must be at least %d characters long", minAuthTokenLength))
		}
	}
	if c.MaxConcurrency < 1 {
		errs = append(errs, errors.New("max concurrency must be at least 1"))
	}
	if c.HTTPTimeout <= 0 {
		errs = append(errs, errors.New("timeout must be positive"))
	}
	if c.MaxContentLength < 1000 || c.MaxContentLength > 200000 {
		errs = append(errs, errors.New("max content length must be between 1000 and 200000"))
	}
	return errors.Join(errs...)
}

// validateHTTPSURL refuse toute URL non HTTPS afin de ne jamais envoyer les
// identifiants PISTE en clair.
func validateHTTPSURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%q must be an absolute https URL", raw)
	}
	return nil
}

func parseDuration(raw string) (time.Duration, error) {
	// Un entier seul est interprété comme un nombre de secondes.
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	return time.ParseDuration(raw)
}
