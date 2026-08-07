// archive-center-go is the entry point for the Archive Center 2.0 shadow service.
// It starts an HTTP server on a non-conflicting port with shadow-only defaults.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	appCtx, cancelApp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelApp()
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
	var requestedExitCode atomic.Int32
	if strings.TrimSpace(os.Getenv("AC_UPDATE_APPLY_MODE")) == httpapi.UpdateApplyManagedLauncherMode {
		server.RequestShutdown = func(exitCode int) {
			if exitCode == httpapi.UpdateApplyExitCode && requestedExitCode.CompareAndSwap(0, int32(exitCode)) {
				cancelApp()
			}
		}
	}
	if err := server.ValidateRuntimeDependencies(appCtx); err != nil {
		logger.Error("runtime dependency preflight failed", "error", err)
		os.Exit(1)
	}
	if server.StartMemoryWorkers(appCtx) {
		logger.Info("memory reprocessing worker enabled")
	}
	server.RegisterRoutes(mux)

	logger.Info("starting server", "bind", cfg.BindAddr, "mode", cfg.Mode)
	httpServer := &http.Server{Addr: cfg.BindAddr, Handler: mux}
	go func() {
		<-appCtx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful server shutdown failed", "error", err)
			_ = httpServer.Close()
		}
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
	if exitCode := requestedExitCode.Load(); exitCode != 0 {
		os.Exit(int(exitCode))
	}
}
