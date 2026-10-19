// Commande legifrance-mcp : serveur MCP donnant accès à l'API Légifrance.
//
// Deux transports sont disponibles :
//   - stdio (par défaut) pour un usage local depuis un client MCP ;
//   - http (Streamable HTTP) pour un déploiement conteneurisé.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // fuseau Europe/Paris disponible même dans une image minimale

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"legifrance-mcp/internal/config"
	"legifrance-mcp/internal/legifrance"
	"legifrance-mcp/internal/tools"
)

// version est renseignée à la compilation via -ldflags "-X main.version=…".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "legifrance-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}

	// Les logs partent sur stderr : en mode stdio, stdout est réservé au protocole MCP.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	// Le SDK MCP journalise chaque connexion de session au niveau INFO : on ne
	// conserve ses messages qu'à partir de WARN, sauf en mode debug.
	sdkLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: max(cfg.LogLevel, slog.LevelWarn)}))
	if cfg.LogLevel <= slog.LevelDebug {
		sdkLogger = logger
	}

	client, err := legifrance.NewClient(legifrance.Options{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		BaseURL:      cfg.APIBaseURL,
		Timeout:      cfg.HTTPTimeout,
		UserAgent:    "legifrance-mcp/" + version,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	server := tools.NewServer(client, tools.Options{
		Version:          version,
		MaxContentLength: cfg.MaxContentLength,
		Logger:           sdkLogger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting legifrance-mcp",
		slog.String("version", version),
		slog.String("transport", cfg.Transport),
		slog.String("environment", cfg.Environment))

	if cfg.Transport == config.TransportHTTP {
		return serveHTTP(ctx, cfg, server, logger, sdkLogger)
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// serveHTTP expose le serveur MCP en Streamable HTTP sur /mcp, avec une sonde /healthz.
func serveHTTP(ctx context.Context, cfg config.Config, server *mcp.Server, logger, sdkLogger *slog.Logger) error {
	// Mode sans état : aucun outil n'émet de requête vers le client, ce qui
	// permet de répartir la charge sur plusieurs instances sans affinité de session.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:           true,
			Logger:              sdkLogger,
			MaxRequestBodyBytes: maxRequestBodyBytes,
		})

	protected := limitConcurrency(cfg.MaxConcurrency, http.NewCrossOriginProtection().Handler(mcpHandler))
	if cfg.AuthToken != "" {
		protected = requireBearer(cfg.AuthToken, protected)
	} else {
		// Cas autorisé explicitement par MCP_ALLOW_UNAUTHENTICATED (validé dans config).
		logger.Warn("the /mcp endpoint is unauthenticated (MCP_ALLOW_UNAUTHENTICATED=true)")
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", protected)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		// Pas de WriteTimeout : les réponses en flux SSE peuvent durer.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// maxRequestBodyBytes borne la taille d'un message JSON-RPC entrant : les
// arguments des outils sont courts.
const maxRequestBodyBytes = 256 << 10

// requireBearer exige l'en-tête « Authorization: Bearer <token> ».
// Les empreintes SHA-256 sont comparées en temps constant, ce qui ne divulgue
// ni le contenu ni la longueur du jeton.
func requireBearer(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		sum := sha256.Sum256([]byte(strings.TrimSpace(got)))
		if !ok || subtle.ConstantTimeCompare(sum[:], expected[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="legifrance-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitConcurrency borne le nombre de requêtes traitées simultanément, afin
// de protéger la mémoire du conteneur et le quota PISTE. Une requête en
// attente est abandonnée si le client se déconnecte.
func limitConcurrency(n int, next http.Handler) http.Handler {
	slots := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		case <-r.Context().Done():
			http.Error(w, "request cancelled while waiting for a slot", http.StatusServiceUnavailable)
		}
	})
}
