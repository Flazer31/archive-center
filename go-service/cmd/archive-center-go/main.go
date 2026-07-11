// archive-center-go is the entry point for the Archive Center 2.0 shadow service.
// It starts an HTTP server on a non-conflicting port with shadow-only defaults.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if httpapi.ConfigureOutboundDNSServers(os.Getenv("AC_DNS_SERVERS")) {
		logger.Info("configured outbound dns override")
	}

	cfg := config.Load()
	logger.Info("loaded config", "config", cfg.String())

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	if cfg.Mode != config.ModeShadow {
		if !cfg.IsLiveCutoverAllowed() {
			logger.Error("live/cutover mode is not allowed with this configuration", "config", cfg.String())
			os.Exit(1)
		}
		logger.Info("product runtime mode enabled", "mode", cfg.Mode, "store_mode", cfg.StoreMode)
	}

	mux := http.NewServeMux()
	server := httpapi.NewServer(cfg)
	preflightCtx, cancelPreflight := context.WithTimeout(context.Background(), 30*time.Second)
	if err := server.ValidateRuntimeDependencies(preflightCtx); err != nil {
		cancelPreflight()
		logger.Error("runtime dependency preflight failed", "error", err)
		os.Exit(1)
	}
	cancelPreflight()
	server.RegisterRoutes(mux)

	logger.Info("starting server", "bind", cfg.BindAddr, "mode", cfg.Mode)
	if err := http.ListenAndServe(cfg.BindAddr, mux); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
