// archive-center-go is the entry point for the Archive Center 2.0 shadow service.
// It starts an HTTP server on a non-conflicting port with shadow-only defaults.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/diagnostics"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
)

func main() {
	// Export works before configuration validation or database initialization.
	if len(os.Args) > 1 && os.Args[1] == "diagnostics" {
		report := diagnostics.Collect(diagnostics.Directory(), config.Load().BuildVersion)
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	cfg := config.Load()
	logWriter := &diagnostics.Writer{Dir: diagnostics.Directory(), Console: os.Stderr, Secrets: []string{cfg.MariaDBDSN}}
	levelErr := logWriter.SetLevel(os.Getenv("AC_LOG_LEVEL"))
	logger := slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: logWriter}))
	slog.SetDefault(logger)
	if levelErr != nil {
		logger.Warn("invalid AC_LOG_LEVEL; using info", "error", levelErr)
	}
	if err := diagnostics.EnableCrashLog(logWriter.Dir); err != nil {
		logger.Warn("crash log unavailable", "error", err)
	}
	appCtx, cancelApp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelApp()
	if httpapi.ConfigureOutboundDNSServers(os.Getenv("AC_DNS_SERVERS")) {
		logger.Info("configured outbound dns override")
	}

	logger.Info("loaded config", "config", cfg.String())
	logger.Info("diagnostic log enabled", "directory", logWriter.Dir, "logging_level", logWriter.LoggingState().Level)

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
	server.DiagnosticWriter = logWriter
	var requestedExitCode atomic.Int32
	if managedUpdateLauncherAuthorized(cfg) {
		server.RequestShutdown = func(exitCode int) {
			if exitCode == httpapi.UpdateApplyExitCode && requestedExitCode.CompareAndSwap(0, int32(exitCode)) {
				cancelApp()
			}
		}
	} else if strings.TrimSpace(os.Getenv("AC_UPDATE_APPLY_MODE")) == httpapi.UpdateApplyManagedLauncherMode {
		logger.Warn("managed update mode ignored because launcher session authorization was not present")
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
	httpServer := &http.Server{Addr: cfg.BindAddr, Handler: mux, ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}
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

const updateLauncherSessionContract = "archive-center.update-launcher-session.v1"

type updateLauncherSession struct {
	ContractVersion string `json:"contract_version"`
	Token           string `json:"token"`
}

func managedUpdateLauncherAuthorized(cfg config.Config) bool {
	if strings.TrimSpace(os.Getenv("AC_UPDATE_APPLY_MODE")) != httpapi.UpdateApplyManagedLauncherMode {
		return false
	}
	token := strings.TrimSpace(os.Getenv("AC_UPDATE_LAUNCHER_TOKEN"))
	if len(token) < 32 || strings.TrimSpace(cfg.UpdateStagingDir) == "" {
		return false
	}
	path := filepath.Join(cfg.UpdateStagingDir, "launcher-session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var session updateLauncherSession
	if json.Unmarshal(data, &session) != nil || session.ContractVersion != updateLauncherSessionContract || session.Token != token {
		return false
	}
	if err := os.Remove(path); err != nil {
		return false
	}
	_ = os.Unsetenv("AC_UPDATE_LAUNCHER_TOKEN")
	return true
}
