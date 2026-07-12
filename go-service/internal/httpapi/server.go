// Package httpapi provides HTTP handlers for the Archive Center Go shadow service.
// Route groups are split by domain (health, turn, memory, proxy, admin, narrative)
// to keep files small and focused.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Server holds the HTTP handler dependencies.
type Server struct {
	Cfg               config.Config
	Started           time.Time
	Store             store.Store
	StoreOpenError    error
	Vector            vector.VectorStore
	VectorOpenError   error
	RuntimeConfig     RuntimeConfig
	RuntimeConfigMu   sync.RWMutex
	AdminJobs         *adminJobManager
	CompleteTurns     *completeTurnRequestLedger
	RollbackDecisions *rollbackDecisionLedger
}

// ValidateRuntimeDependencies verifies live dependencies before the HTTP
// service advertises readiness. Chroma Health checks both the configured API
// heartbeat and collection endpoint without deleting existing data.
func (s *Server) ValidateRuntimeDependencies(ctx context.Context) error {
	if s.StoreOpenError != nil && s.Cfg.StoreMode == config.StoreModeMariaDBAuthority {
		return fmt.Errorf("mariadb startup preflight failed: %w", s.StoreOpenError)
	}
	if !s.Cfg.ChromaEnabled || strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		return nil
	}
	if s.VectorOpenError != nil {
		return fmt.Errorf("chromadb startup preflight failed (api_path=%s): %w", s.Cfg.ChromaAPIPath, s.VectorOpenError)
	}
	health, err := s.Vector.Health(ctx)
	if err != nil {
		return fmt.Errorf("chromadb startup preflight failed (api_path=%s): %w", s.Cfg.ChromaAPIPath, err)
	}
	if strings.TrimSpace(health.Status) != "ok" || !health.ModelReady {
		return fmt.Errorf("chromadb startup preflight failed (api_path=%s): status=%s model_ready=%t", s.Cfg.ChromaAPIPath, health.Status, health.ModelReady)
	}
	return nil
}

// NewServer creates a Server with the given configuration.
func NewServer(cfg config.Config) *Server {
	st, storeErr := newStoreForConfig(cfg)
	var vs vector.VectorStore
	var vectorErr error
	switch {
	case cfg.ChromaEnabled && strings.TrimSpace(cfg.ChromaEndpoint) != "":
		vs, vectorErr = vector.NewChromaStore(cfg.ChromaEndpoint, cfg.ChromaCollection, cfg.ChromaAPIPath)
	default:
		vs = vector.NewFakeVectorStore()
	}
	if vectorErr != nil {
		vs = vector.NewFakeVectorStore()
	}
	return &Server{
		Cfg:               cfg,
		Started:           time.Now().UTC(),
		Store:             st,
		StoreOpenError:    storeErr,
		Vector:            vs,
		VectorOpenError:   vectorErr,
		AdminJobs:         newAdminJobManager(),
		CompleteTurns:     newCompleteTurnRequestLedger(),
		RollbackDecisions: newRollbackDecisionLedger(),
	}
}

// newStoreForConfig picks the store implementation based on the config store mode.
func newStoreForConfig(cfg config.Config) (store.Store, error) {
	switch cfg.StoreMode {
	case config.StoreModeDualShadow:
		return store.NewDualWriteStore(store.NewNoopStore(), store.NewNoopStore()), nil
	case config.StoreModeMariaDBShadow:
		maria, err := store.OpenMariaDB(cfg.MariaDBDSN)
		if err != nil {
			return store.NewNoopStore(), err
		}
		return store.NewDualWriteStore(store.NewNoopStore(), maria), nil
	case config.StoreModeMariaDBReadShadow:
		maria, err := store.OpenMariaDB(cfg.MariaDBDSN)
		if err != nil {
			return store.NewNoopStore(), err
		}
		return store.NewReadOnlyStore(maria), nil
	case config.StoreModeMariaDBAuthority:
		maria, err := store.OpenMariaDB(cfg.MariaDBDSN)
		if err != nil {
			return store.NewNoopStore(), err
		}
		return maria, nil
	case config.StoreModeFixtureShadow:
		fixture, err := store.NewFixtureStoreFromExportDir(cfg.StoreFixtureDir)
		if err != nil {
			return store.NewNoopStore(), err
		}
		return fixture, nil
	default:
		return store.NewNoopStore(), nil
	}
}

func (s *Server) usesShadowWriteStore() bool {
	if errors.Is(s.StoreOpenError, store.ErrNotEnabled) {
		return false
	}
	return s.Cfg.StoreMode == config.StoreModeDualShadow ||
		s.Cfg.StoreMode == config.StoreModeMariaDBShadow ||
		s.Cfg.StoreMode == config.StoreModeMariaDBAuthority
}

func (s *Server) storeWriteSource() string {
	switch s.Cfg.StoreMode {
	case config.StoreModeMariaDBAuthority:
		return "mariadb_authority"
	case config.StoreModeMariaDBShadow:
		return "mariadb_shadow"
	case config.StoreModeDualShadow:
		return "dual_shadow"
	default:
		return "shadow"
	}
}

// RegisterRoutes mounts all handler routes on the provided mux.
// Tier mapping (mirrors contracts/go-route-group-design.md):
//
//	R0: health, config static, chroma-shadow probes
//	R1: search, retrieval-index read, explorer read, narrative read, metrics read, audit
//	R2: turn write, memory write, proxy config write, admin ops, narrative write, import
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	sub := http.NewServeMux()
	s.registerHealthRoutes(sub)    // R0 + R1 read
	s.registerConfigRoutes(sub)    // R1 read + R2 write
	s.registerTurnRoutes(sub)      // R2 write
	s.registerMemoryRoutes(sub)    // R1 read + R2 write
	s.registerCanonicalRoutes(sub) // R1 store-backed canonical shadow
	// Deprecated vector shadow/proof routes are intentionally not mounted in
	// the active 2.0 runtime. ChromaDB is the selected vector accelerator.
	s.registerProxyRoutes(sub)        // R2 write
	s.registerAdminRoutes(sub)        // R2 write
	s.registerTimelineRoutes(sub)     // R1 read
	s.registerDashboardRoutes(sub)    // R1 presentation model
	s.registerPresentationRoutes(sub) // R1 timeline/explorer presentation model
	s.registerSessionMigrationRoutes(sub)
	s.registerNarrativeRoutes(sub) // R1 read + R2 write
	s.registerPersonaRoutes(sub)   // R1 read + R2 write
	s.registerTableReadRoutes(sub) // R1 read-only planning
	s.registerCriticLedgerRoutes(sub)
	s.registerStep22ValidationRoutes(sub)
	s.registerStep23ConsequenceRoutes(sub)
	s.registerStep23PsychologyRoutes(sub)
	s.registerStep23ForkLineageRoutes(sub)
	s.registerStep23ThemeOffscreenRoutes(sub)
	s.registerStep23CaptureVerificationRoutes(sub)
	s.registerStatusSchemaRoutes(sub)
	s.registerReferenceLibraryRoutes(sub)
	s.registerUpdateRoutes(sub)
	mux.Handle("/", s.corsMiddleware(s.authMiddleware(s.reverseProxyBasePathMiddleware(sub))))
}

func (s *Server) reverseProxyBasePathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		strippedPath, ok := stripUnknownReverseProxyBasePath(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		clone := r.Clone(r.Context())
		urlCopy := *clone.URL
		urlCopy.Path = strippedPath
		urlCopy.RawPath = ""
		clone.URL = &urlCopy
		next.ServeHTTP(w, clone)
	})
}

func stripUnknownReverseProxyBasePath(requestPath string) (string, bool) {
	path := strings.TrimSpace(requestPath)
	if path == "" || path == "/" || !strings.HasPrefix(path, "/") {
		return requestPath, false
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) < 2 {
		return requestPath, false
	}
	first := strings.TrimSpace(parts[0])
	second := strings.TrimSpace(parts[1])
	if first == "" || second == "" || isArchiveRouteRoot(first) || !isArchiveRouteRoot(second) {
		return requestPath, false
	}
	if len(parts) == 2 {
		return "/" + second, true
	}
	return "/" + second + "/" + parts[2], true
}

func isArchiveRouteRoot(segment string) bool {
	switch strings.ToLower(strings.TrimSpace(segment)) {
	case "active-states",
		"admin",
		"arc",
		"arcs",
		"canonical",
		"characters",
		"chapter",
		"chapters",
		"chroma-shadow",
		"complete-turn",
		"config",
		"continuity-pack",
		"effective-inputs",
		"episodes",
		"explorer",
		"feedback",
		"health",
		"import",
		"intent-routing",
		"kg",
		"long-session-health",
		"maintenance",
		"maintenance-pass",
		"metrics",
		"momentum-packet",
		"narrative-control",
		"pending-threads",
		"persona-capsules",
		"prepare-turn",
		"prompts",
		"proxy",
		"ready",
		"retrieval-index",
		"rollback",
		"saga",
		"sagas",
		"search",
		"session",
		"session-state",
		"sessions",
		"stats",
		"status-schema",
		"step22",
		"step23",
		"storylines",
		"subjective-entity-memories",
		"supervisor",
		"table-read",
		"timeline",
		"turns",
		"update",
		"validation",
		"version",
		"wakeup",
		"world-rules":
		return true
	default:
		return false
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		allowed := s.resolveAllowedOrigin(origin)
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) resolveAllowedOrigin(origin string) string {
	origins := s.Cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	for _, item := range origins {
		item = strings.TrimSpace(item)
		switch {
		case item == "*":
			return "*"
		case origin != "" && strings.EqualFold(item, origin):
			return origin
		}
	}
	return ""
}
