// Package config provides application configuration for the Archive Center Go shadow service.
// All values have safe defaults suitable for local development and shadow mode.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mode describes the runtime role of this service instance.
type Mode string

const (
	ModeShadow  Mode = "shadow"
	ModeLive    Mode = "live"
	ModeCutover Mode = "cutover"
)

// StoreMode selects the concrete store implementation.
type StoreMode string

const (
	StoreModeNoop              StoreMode = "noop"
	StoreModeDualShadow        StoreMode = "dual_shadow"
	StoreModeMariaDBShadow     StoreMode = "mariadb_shadow"
	StoreModeFixtureShadow     StoreMode = "fixture_shadow"
	StoreModeMariaDBReadShadow StoreMode = "mariadb_read_shadow"
	StoreModeMariaDBAuthority  StoreMode = "mariadb_authority"
)

// RuntimeProfile selects the deployment shape for 2.0.1 packages.
type RuntimeProfile string

const (
	RuntimeProfileClientOnly        RuntimeProfile = "client_only"
	RuntimeProfileCoreLite          RuntimeProfile = "core_lite"
	RuntimeProfileVectorExternal    RuntimeProfile = "vector_external"
	RuntimeProfileVectorLocalNative RuntimeProfile = "vector_local_native"
	RuntimeProfileFullLocal         RuntimeProfile = "full_local"
)

// VectorMode selects whether the vector accelerator is disabled, degraded, or
// backed by a ChromaDB endpoint.
type VectorMode string

const (
	VectorModeOff         VectorMode = "off"
	VectorModeFallback    VectorMode = "fallback"
	VectorModeExternal    VectorMode = "external"
	VectorModeLocalNative VectorMode = "local_native"
	VectorModeLocalProot  VectorMode = "local_proot"
	VectorModeBundled     VectorMode = "bundled"
)

// Config holds the entire service configuration.
// It contains no secrets; secrets must be injected via the environment
// or a future secret management integration.
type Config struct {
	// HTTP bind address. Default is 127.0.0.1:28080 to avoid conflict with the 0.8 backend.
	BindAddr string

	// AllowedOrigins controls CORS response headers. "*" is the local default.
	AllowedOrigins []string

	// Service mode. Default is shadow. Live and cutover are disabled by default.
	Mode Mode

	// StoreMode selects the store backend. Default is noop.
	// dual_shadow enables the R1 dual-write wrapper over two no-op stores.
	// mariadb_shadow enables the R1 dual-write wrapper with noop primary and
	// MariaDB shadow target. It is not an authority switch.
	StoreMode StoreMode

	// RuntimeProfile selects the 2.0.1 service bundle shape.
	RuntimeProfile RuntimeProfile

	// VectorMode selects how vector recall is provided for this profile.
	VectorMode VectorMode

	// MariaDBDSN is used only when StoreMode is a MariaDB-backed mode.
	MariaDBDSN string

	// MariaDBProductReadEnabled marks an explicit R2 product-read proof where
	// read-only HTTP surfaces are allowed to use MariaDB as their selected
	// relational read source. Default is false and it is accepted only in
	// mariadb_read_shadow mode with a configured DSN.
	MariaDBProductReadEnabled bool

	// StoreFixtureDir is used only by fixture_shadow R1 evidence mode.
	// It points at sqlite-export NDJSON and is not a product authority path.
	StoreFixtureDir string

	// ChromaEnabled records whether this runtime profile should wire the
	// ChromaDB-compatible vector accelerator. MariaDB remains canonical truth.
	ChromaEnabled bool

	// ChromaEndpoint is the ChromaDB HTTP base URL, for example http://127.0.0.1:8000.
	ChromaEndpoint string

	// ChromaCollection is the collection used for Archive Center vectors.
	ChromaCollection string

	// ReferenceChromaCollection is a separate collection for reusable original-
	// work material. It must never share the session-memory collection.
	ReferenceChromaCollection string

	// ChromaAPIPath is the Chroma HTTP API prefix. Default is /api/v2.
	ChromaAPIPath string

	// PromptDir is the editable prompt directory used by the migrated prompt
	// editor. Only known prompt files are exposed by the HTTP API.
	PromptDir string

	// Readiness probe settings.
	Readiness ReadinessConfig

	// Build metadata injected at compile time.
	BuildVersion string
	BuildCommit  string
	BuildTime    string

	// Auth holds the bearer-token envelope. Enforce is off by default.
	Auth AuthConfig
	// ChromaShadowPersistDir is the Chroma shadow persist directory for preflight parity.
	ChromaShadowPersistDir string

	// Embedder config for preflight parity.
	EmbedderProvider string
	EmbedderModel    string
	EmbedderEndpoint string

	// PrunePolicy controls critic prune target handling: "soft" or "off".
	PrunePolicy string

	// CriticLedgerPreviewEnabled exposes the read-only 2.1 critic ledger preview route.
	CriticLedgerPreviewEnabled bool

	// CriticLedgerEnabled allows the ledger to be wired into live Critic calls.
	// It stays false by default; 2.1-7 live wiring requires explicit opt-in.
	CriticLedgerEnabled bool

	// Update settings power the GitHub Releases update check/download flow.
	// The updater only stages verified packages; applying and rollback are
	// intentionally handled by a later helper process.
	UpdateEnabled          bool
	UpdateGitHubRepo       string
	UpdateChannel          string
	UpdateStagingDir       string
	UpdateMaxDownloadBytes int64
}

// ReadinessConfig describes the state of external dependencies for readiness checks.
type ReadinessConfig struct {
	// MariaDBConfigured is true when a MariaDB DSN is present in the environment.
	MariaDBConfigured bool

	// ChromaConfigured is true when a Chroma endpoint is present in the environment.
	ChromaConfigured bool
}

// AuthConfig holds bearer-token authentication settings.
// Enforce is false by default in R0/R1; live enforcement requires explicit approval.
type AuthConfig struct {
	BearerToken string
	Enforce     bool
}

// Default returns a Config populated with safe defaults.
func Default() Config {
	return Config{
		BindAddr:                  "127.0.0.1:28080",
		AllowedOrigins:            []string{"*"},
		Mode:                      ModeShadow,
		StoreMode:                 StoreModeNoop,
		RuntimeProfile:            RuntimeProfileCoreLite,
		VectorMode:                VectorModeFallback,
		MariaDBProductReadEnabled: false,
		ChromaEnabled:             false,
		ChromaEndpoint:            "",
		ChromaCollection:          "archive_center_vectors",
		ReferenceChromaCollection: "archive_center_reference_vectors",
		ChromaAPIPath:             "/api/v2",
		PromptDir:                 "",
		BuildVersion:              "2.0.0-dev",
		BuildCommit:               "unknown",
		BuildTime:                 time.Now().UTC().Format(time.RFC3339),
		Readiness: ReadinessConfig{
			MariaDBConfigured: false,
			ChromaConfigured:  false,
		},
		Auth: AuthConfig{
			BearerToken: "",
			Enforce:     false,
		},
		PrunePolicy:                "soft",
		CriticLedgerPreviewEnabled: true,
		CriticLedgerEnabled:        false,
		UpdateEnabled:              true,
		UpdateGitHubRepo:           "Flazer31/archive-center",
		UpdateChannel:              "stable",
		UpdateStagingDir:           ".updates",
		UpdateMaxDownloadBytes:     1024 * 1024 * 1024,
	}
}

// Load builds a Config from defaults, then overrides with environment variables.
func Load() Config {
	cfg := Default()

	if v := os.Getenv("AC_BIND_ADDR"); v != "" {
		cfg.BindAddr = v
	}
	if v := os.Getenv("AC_ALLOWED_ORIGINS"); v != "" {
		cfg.AllowedOrigins = splitCSV(v)
		if len(cfg.AllowedOrigins) == 0 {
			cfg.AllowedOrigins = []string{"*"}
		}
	}

	if v := os.Getenv("AC_MODE"); v != "" {
		switch strings.ToLower(v) {
		case string(ModeLive):
			cfg.Mode = ModeLive
		case string(ModeCutover):
			cfg.Mode = ModeCutover
		default:
			cfg.Mode = ModeShadow
		}
	}

	if v := os.Getenv("AC_STORE_MODE"); v != "" {
		switch strings.ToLower(v) {
		case string(StoreModeNoop):
			cfg.StoreMode = StoreModeNoop
		case string(StoreModeDualShadow):
			cfg.StoreMode = StoreModeDualShadow
		case string(StoreModeMariaDBShadow):
			cfg.StoreMode = StoreModeMariaDBShadow
		case string(StoreModeFixtureShadow):
			cfg.StoreMode = StoreModeFixtureShadow
		case string(StoreModeMariaDBReadShadow):
			cfg.StoreMode = StoreModeMariaDBReadShadow
		case string(StoreModeMariaDBAuthority):
			cfg.StoreMode = StoreModeMariaDBAuthority
		default:
			cfg.StoreMode = StoreMode(strings.ToLower(v))
		}
	}

	runtimeProfileExplicit := false
	if v := os.Getenv("AC_RUNTIME_PROFILE"); v != "" {
		runtimeProfileExplicit = true
		cfg.RuntimeProfile = parseRuntimeProfile(v)
	}

	if v := os.Getenv("AC_BUILD_VERSION"); v != "" {
		cfg.BuildVersion = v
	}

	if v := os.Getenv("AC_BUILD_COMMIT"); v != "" {
		cfg.BuildCommit = v
	}

	if v := os.Getenv("AC_BUILD_TIME"); v != "" {
		cfg.BuildTime = v
	}

	cfg.MariaDBDSN = os.Getenv("AC_MARIADB_DSN")
	cfg.StoreFixtureDir = os.Getenv("AC_STORE_FIXTURE_DIR")
	cfg.Readiness.MariaDBConfigured = cfg.MariaDBDSN != ""
	cfg.ChromaEndpoint = os.Getenv("AC_CHROMA_ENDPOINT")
	cfg.Readiness.ChromaConfigured = cfg.ChromaEndpoint != ""
	vectorModeExplicit := false
	if v := os.Getenv("AC_VECTOR_MODE"); v != "" {
		vectorModeExplicit = true
		cfg.VectorMode = parseVectorMode(v)
	} else if !runtimeProfileExplicit && strings.TrimSpace(cfg.ChromaEndpoint) != "" {
		cfg.RuntimeProfile = RuntimeProfileFullLocal
		cfg.VectorMode = VectorModeBundled
	} else {
		cfg.VectorMode = defaultVectorMode(cfg.RuntimeProfile)
	}
	cfg.ChromaEnabled = cfg.VectorRequiresEndpoint()
	if vectorModeExplicit && (cfg.VectorMode == VectorModeOff || cfg.VectorMode == VectorModeFallback) {
		cfg.ChromaEnabled = false
	}
	if v := os.Getenv("AC_CHROMA_COLLECTION"); v != "" {
		cfg.ChromaCollection = v
	}
	if v := os.Getenv("AC_REFERENCE_CHROMA_COLLECTION"); v != "" {
		cfg.ReferenceChromaCollection = v
	}
	if v := os.Getenv("AC_CHROMA_API_PATH"); v != "" {
		cfg.ChromaAPIPath = v
	}
	cfg.PromptDir = os.Getenv("AC_PROMPT_DIR")
	cfg.ChromaShadowPersistDir = os.Getenv("AC_CHROMA_SHADOW_PERSIST_DIR")
	cfg.EmbedderProvider = os.Getenv("AC_EMBEDDER_PROVIDER")
	cfg.EmbedderModel = os.Getenv("AC_EMBEDDER_MODEL")
	cfg.EmbedderEndpoint = os.Getenv("AC_EMBEDDER_ENDPOINT")
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("AC_PRUNE_POLICY"))); v != "" {
		switch v {
		case "off", "soft":
			cfg.PrunePolicy = v
		default:
			cfg.PrunePolicy = "soft"
		}
	}
	if os.Getenv("AC_MARIADB_PRODUCT_READ_ENABLED") == "true" {
		cfg.MariaDBProductReadEnabled = true
	}
	if v := strings.TrimSpace(os.Getenv("AC_CRITIC_LEDGER_PREVIEW_ENABLED")); v != "" {
		cfg.CriticLedgerPreviewEnabled = strings.EqualFold(v, "true")
	}
	if os.Getenv("AC_CRITIC_LEDGER_ENABLED") == "true" {
		cfg.CriticLedgerEnabled = true
	}
	if v := strings.TrimSpace(os.Getenv("AC_UPDATE_ENABLED")); v != "" {
		cfg.UpdateEnabled = strings.EqualFold(v, "true")
	}
	if v := strings.TrimSpace(os.Getenv("AC_UPDATE_GITHUB_REPO")); v != "" {
		cfg.UpdateGitHubRepo = v
	}
	if v := strings.TrimSpace(os.Getenv("AC_UPDATE_CHANNEL")); v != "" {
		cfg.UpdateChannel = v
	}
	if v := strings.TrimSpace(os.Getenv("AC_UPDATE_STAGING_DIR")); v != "" {
		cfg.UpdateStagingDir = v
	}
	if v := strings.TrimSpace(os.Getenv("AC_UPDATE_MAX_DOWNLOAD_MB")); v != "" {
		if mb, err := strconv.ParseInt(v, 10, 64); err == nil && mb > 0 {
			cfg.UpdateMaxDownloadBytes = mb * 1024 * 1024
		}
	}

	if v := os.Getenv("AC_BEARER_TOKEN"); v != "" {
		cfg.Auth.BearerToken = v
	}
	if os.Getenv("AC_ENFORCE_AUTH") == "true" {
		cfg.Auth.Enforce = true
	}

	return cfg
}

func parseRuntimeProfile(raw string) RuntimeProfile {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RuntimeProfileClientOnly):
		return RuntimeProfileClientOnly
	case string(RuntimeProfileVectorExternal):
		return RuntimeProfileVectorExternal
	case string(RuntimeProfileVectorLocalNative):
		return RuntimeProfileVectorLocalNative
	case string(RuntimeProfileFullLocal):
		return RuntimeProfileFullLocal
	case string(RuntimeProfileCoreLite):
		return RuntimeProfileCoreLite
	default:
		return RuntimeProfile(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func parseVectorMode(raw string) VectorMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(VectorModeOff):
		return VectorModeOff
	case string(VectorModeExternal):
		return VectorModeExternal
	case string(VectorModeLocalNative):
		return VectorModeLocalNative
	case string(VectorModeLocalProot):
		return VectorModeLocalProot
	case string(VectorModeBundled):
		return VectorModeBundled
	case string(VectorModeFallback):
		return VectorModeFallback
	default:
		return VectorMode(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func defaultVectorMode(profile RuntimeProfile) VectorMode {
	switch profile {
	case RuntimeProfileClientOnly:
		return VectorModeOff
	case RuntimeProfileVectorExternal:
		return VectorModeExternal
	case RuntimeProfileVectorLocalNative:
		return VectorModeLocalNative
	case RuntimeProfileFullLocal:
		return VectorModeBundled
	default:
		return VectorModeFallback
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// Validate returns an error if the configuration is inconsistent or unsafe.
func (c Config) Validate() error {
	if c.Mode != ModeShadow && c.Mode != ModeLive && c.Mode != ModeCutover {
		return fmt.Errorf("config: mode %q is not allowed", c.Mode)
	}
	if !isAllowedRuntimeProfile(c.RuntimeProfile) {
		return fmt.Errorf("config: runtime_profile %q is not allowed", c.RuntimeProfile)
	}
	if !isAllowedVectorMode(c.VectorMode) {
		return fmt.Errorf("config: vector_mode %q is not allowed", c.VectorMode)
	}
	if err := c.validateProfileVectorPair(); err != nil {
		return err
	}
	if (c.Mode == ModeLive || c.Mode == ModeCutover) && (c.StoreMode != StoreModeMariaDBAuthority || strings.TrimSpace(c.MariaDBDSN) == "") {
		return fmt.Errorf("config: mode %q requires AC_STORE_MODE=%q and AC_MARIADB_DSN", c.Mode, StoreModeMariaDBAuthority)
	}
	if c.StoreMode != StoreModeNoop && c.StoreMode != StoreModeDualShadow && c.StoreMode != StoreModeMariaDBShadow && c.StoreMode != StoreModeFixtureShadow && c.StoreMode != StoreModeMariaDBReadShadow && c.StoreMode != StoreModeMariaDBAuthority {
		return fmt.Errorf("config: store_mode %q is not allowed in this slice; only %q, %q, %q, %q, %q, and %q are allowed", c.StoreMode, StoreModeNoop, StoreModeDualShadow, StoreModeMariaDBShadow, StoreModeFixtureShadow, StoreModeMariaDBReadShadow, StoreModeMariaDBAuthority)
	}
	if (c.StoreMode == StoreModeMariaDBShadow || c.StoreMode == StoreModeMariaDBReadShadow || c.StoreMode == StoreModeMariaDBAuthority) && strings.TrimSpace(c.MariaDBDSN) == "" {
		return fmt.Errorf("config: store_mode %q requires AC_MARIADB_DSN", c.StoreMode)
	}
	if c.MariaDBProductReadEnabled && (c.StoreMode != StoreModeMariaDBReadShadow || strings.TrimSpace(c.MariaDBDSN) == "") {
		return fmt.Errorf("config: mariadb product read proof requires AC_STORE_MODE=%q and AC_MARIADB_DSN", StoreModeMariaDBReadShadow)
	}
	if c.StoreMode == StoreModeFixtureShadow && strings.TrimSpace(c.StoreFixtureDir) == "" {
		return fmt.Errorf("config: store_mode %q requires AC_STORE_FIXTURE_DIR", c.StoreMode)
	}
	if c.VectorRequiresEndpoint() && strings.TrimSpace(c.ChromaEndpoint) == "" {
		return fmt.Errorf("config: vector_mode %q requires AC_CHROMA_ENDPOINT", c.VectorMode)
	}
	if strings.TrimSpace(c.ChromaEndpoint) != "" && strings.TrimSpace(c.ChromaCollection) == "" {
		return fmt.Errorf("config: chroma vector store requires AC_CHROMA_COLLECTION")
	}
	if strings.TrimSpace(c.ChromaEndpoint) != "" && strings.TrimSpace(c.ReferenceChromaCollection) == "" {
		return fmt.Errorf("config: reference vector store requires AC_REFERENCE_CHROMA_COLLECTION")
	}
	if strings.TrimSpace(c.ChromaEndpoint) != "" && strings.EqualFold(strings.TrimSpace(c.ChromaCollection), strings.TrimSpace(c.ReferenceChromaCollection)) {
		return fmt.Errorf("config: AC_REFERENCE_CHROMA_COLLECTION must differ from AC_CHROMA_COLLECTION")
	}
	return nil
}

func isAllowedRuntimeProfile(profile RuntimeProfile) bool {
	switch profile {
	case RuntimeProfileClientOnly, RuntimeProfileCoreLite, RuntimeProfileVectorExternal, RuntimeProfileVectorLocalNative, RuntimeProfileFullLocal:
		return true
	default:
		return false
	}
}

func isAllowedVectorMode(mode VectorMode) bool {
	switch mode {
	case VectorModeOff, VectorModeFallback, VectorModeExternal, VectorModeLocalNative, VectorModeLocalProot, VectorModeBundled:
		return true
	default:
		return false
	}
}

func (c Config) validateProfileVectorPair() error {
	switch c.RuntimeProfile {
	case RuntimeProfileClientOnly:
		if c.VectorMode != VectorModeOff {
			return fmt.Errorf("config: runtime_profile %q requires vector_mode %q", c.RuntimeProfile, VectorModeOff)
		}
	case RuntimeProfileCoreLite:
		if c.VectorMode != VectorModeFallback && c.VectorMode != VectorModeOff {
			return fmt.Errorf("config: runtime_profile %q supports only vector_mode %q or %q", c.RuntimeProfile, VectorModeFallback, VectorModeOff)
		}
	case RuntimeProfileVectorExternal:
		if c.VectorMode != VectorModeExternal {
			return fmt.Errorf("config: runtime_profile %q requires vector_mode %q", c.RuntimeProfile, VectorModeExternal)
		}
	case RuntimeProfileVectorLocalNative:
		if c.VectorMode != VectorModeLocalNative && c.VectorMode != VectorModeBundled {
			return fmt.Errorf("config: runtime_profile %q supports only vector_mode %q or %q", c.RuntimeProfile, VectorModeLocalNative, VectorModeBundled)
		}
	case RuntimeProfileFullLocal:
		if c.VectorMode != VectorModeLocalNative && c.VectorMode != VectorModeLocalProot && c.VectorMode != VectorModeBundled {
			return fmt.Errorf("config: runtime_profile %q requires a local vector mode", c.RuntimeProfile)
		}
	}
	return nil
}

// VectorRequiresEndpoint reports whether this profile must have a ChromaDB
// endpoint to satisfy readiness.
func (c Config) VectorRequiresEndpoint() bool {
	switch c.VectorMode {
	case VectorModeExternal, VectorModeLocalNative, VectorModeLocalProot, VectorModeBundled:
		return true
	default:
		return false
	}
}

// VectorPolicySatisfied reports whether the selected vector policy can be
// considered configured. In core_lite fallback/off modes, missing ChromaDB is a
// degraded feature state rather than a service blocker.
func (c Config) VectorPolicySatisfied() bool {
	return !c.VectorRequiresEndpoint() || strings.TrimSpace(c.ChromaEndpoint) != ""
}

// IsLiveCutoverAllowed is the runtime guard for product-mode execution.
func (c Config) IsLiveCutoverAllowed() bool {
	return (c.Mode == ModeLive || c.Mode == ModeCutover) &&
		c.StoreMode == StoreModeMariaDBAuthority &&
		strings.TrimSpace(c.MariaDBDSN) != "" &&
		c.VectorPolicySatisfied()
}

// String returns a redacted string representation safe for logs.
func (c Config) String() string {
	return fmt.Sprintf("Config{BindAddr=%s Mode=%s StoreMode=%s RuntimeProfile=%s VectorMode=%s MariaDBProductReadEnabled=%t ChromaEnabled=%t Version=%s}", c.BindAddr, c.Mode, c.StoreMode, c.RuntimeProfile, c.VectorMode, c.MariaDBProductReadEnabled, c.ChromaEnabled, c.BuildVersion)
}
