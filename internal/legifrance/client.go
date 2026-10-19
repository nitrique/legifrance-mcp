// Package legifrance fournit un client HTTP typé pour l'API Légifrance de la
// DILA, exposée via la plateforme PISTE.
//
// L'authentification utilise le flux OAuth2 « client credentials » : le jeton
// est obtenu, mis en cache et renouvelé automatiquement avant expiration.
package legifrance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	// maxResponseBytes protège la mémoire contre des réponses anormalement
	// volumineuses ; un code complet doit être consulté section par section.
	maxResponseBytes = 32 << 20
	// maxErrorBodyBytes limite la portion du corps d'erreur conservée dans les messages.
	maxErrorBodyBytes = 1 << 10
	defaultMaxRetries = 2
	defaultRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 10 * time.Second
)

// Options configure la création d'un [Client].
type Options struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	BaseURL      string
	Timeout      time.Duration
	UserAgent    string
	Logger       *slog.Logger

	// BaseTransport permet d'injecter un transport HTTP (tests, proxy).
	// S'il est nil, http.DefaultTransport est utilisé.
	BaseTransport http.RoundTripper
	// MaxRetries est le nombre de nouvelles tentatives sur erreur transitoire
	// (429, 502, 503, 504). Une valeur négative désactive les nouvelles tentatives.
	MaxRetries int
	// RetryDelay est le délai initial entre deux tentatives (doublé à chaque essai).
	RetryDelay time.Duration
}

// Client appelle l'API Légifrance. Il est sûr pour un usage concurrent.
type Client struct {
	http       *http.Client
	baseURL    string
	userAgent  string
	logger     *slog.Logger
	maxRetries int
	retryDelay time.Duration
}

// NewClient crée un client authentifié auprès de PISTE.
func NewClient(opts Options) (*Client, error) {
	if opts.ClientID == "" || opts.ClientSecret == "" {
		return nil, errors.New("legifrance: client ID and secret are required")
	}
	if opts.TokenURL == "" || opts.BaseURL == "" {
		return nil, errors.New("legifrance: token URL and base URL are required")
	}

	base := opts.BaseTransport
	if base == nil {
		base = http.DefaultTransport
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	oauthCfg := clientcredentials.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		TokenURL:     opts.TokenURL,
		Scopes:       []string{"openid"},
		// PISTE attend les identifiants dans le corps de la requête.
		AuthStyle: oauth2.AuthStyleInParams,
	}
	// Le client HTTP utilisé pour récupérer le jeton est transmis via le contexte.
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Transport: base,
		Timeout:   timeout,
	})

	c := &Client{
		http: &http.Client{
			Transport: &oauth2.Transport{
				Source: oauthCfg.TokenSource(tokenCtx),
				Base:   base,
			},
			Timeout: timeout,
		},
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		userAgent:  opts.UserAgent,
		logger:     opts.Logger,
		maxRetries: opts.MaxRetries,
		retryDelay: opts.RetryDelay,
	}
	if c.userAgent == "" {
		c.userAgent = "legifrance-mcp"
	}
	if c.logger == nil {
		c.logger = slog.New(slog.DiscardHandler)
	}
	if c.maxRetries == 0 {
		c.maxRetries = defaultMaxRetries
	}
	if c.retryDelay <= 0 {
		c.retryDelay = defaultRetryDelay
	}
	return c, nil
}

// APIError représente une réponse HTTP non 2xx de l'API Légifrance.
type APIError struct {
	StatusCode int
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("legifrance: %s returned HTTP %d", e.Path, e.StatusCode)
	switch e.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		// La passerelle PISTE renvoie alors un corps sans intérêt (souvent la
		// requête elle-même) : on donne plutôt la cause la plus probable.
		return msg + ": access denied by PISTE; check that the application is subscribed to the " +
			"Légifrance API and that its terms of use are accepted in this environment (sandbox or production)"
	}
	if e.Body != "" {
		msg += ": " + e.Body
	}
	return msg
}

// IsNotFound indique si l'erreur correspond à un document introuvable.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// post envoie une requête JSON et décode la réponse dans out.
// Tous les endpoints utilisés sont en lecture seule : les nouvelles
// tentatives sont donc sans effet de bord.
func (c *Client) post(ctx context.Context, path string, in, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("legifrance: encode %s request: %w", path, err)
	}
	return c.do(ctx, http.MethodPost, path, payload, out)
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte, out any) error {
	delay := c.retryDelay
	for attempt := 0; ; attempt++ {
		start := time.Now()
		retryAfter, err := c.doOnce(ctx, method, path, payload, out)
		c.logger.DebugContext(ctx, "legifrance call",
			slog.String("path", path), slog.Int("attempt", attempt+1),
			slog.Duration("duration", time.Since(start)), slog.Any("error", err))

		if err == nil || attempt >= c.maxRetries || !isRetryable(err) {
			return err
		}
		wait := max(delay, retryAfter)
		if wait > maxRetryDelay {
			wait = maxRetryDelay
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay *= 2
	}
}

// doOnce effectue une tentative et renvoie, le cas échéant, le délai Retry-After annoncé.
func (c *Client) doOnce(ctx context.Context, method, path string, payload []byte, out any) (time.Duration, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, fmt.Errorf("legifrance: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) {
			// Le corps de la réponse OAuth n'est pas repris : il peut contenir
			// des détails inutiles pour l'utilisateur final.
			status := 0
			if retrieveErr.Response != nil {
				status = retrieveErr.Response.StatusCode
			}
			return 0, fmt.Errorf("legifrance: PISTE authentication failed (HTTP %d): check client ID and secret", status)
		}
		return 0, &transportError{err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return parseRetryAfter(resp.Header.Get("Retry-After")), &APIError{
			StatusCode: resp.StatusCode,
			Path:       path,
			Body:       strings.TrimSpace(string(snippet)),
		}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return 0, nil
	}
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return 0, &transportError{err: err}
	}
	if len(data) > maxResponseBytes {
		return 0, fmt.Errorf("legifrance: %s response exceeds %d bytes", path, maxResponseBytes)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return 0, fmt.Errorf("legifrance: decode %s response: %w", path, err)
	}
	return 0, nil
}

// transportError signale une erreur réseau, considérée comme transitoire.
type transportError struct{ err error }

func (e *transportError) Error() string { return "legifrance: " + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var tErr *transportError
	if errors.As(err, &tErr) {
		// Un dépassement du délai client n'est pas rejoué : il multiplierait
		// l'attente sans bénéfice probable.
		var netErr net.Error
		return !errors.As(err, &netErr) || !netErr.Timeout()
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
	}
	return false
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}
