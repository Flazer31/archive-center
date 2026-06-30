package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.BindAddr != "127.0.0.1:28080" {
		t.Errorf("BindAddr = %q, want %q", cfg.BindAddr, "127.0.0.1:28080")
	}
	if cfg.Mode != ModeShadow {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeShadow)
	}
	if cfg.StoreMode != StoreModeNoop {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeNoop)
	}
	if cfg.RuntimeProfile != RuntimeProfileCoreLite {
		t.Errorf("RuntimeProfile = %q, want %q", cfg.RuntimeProfile, RuntimeProfileCoreLite)
	}
	if cfg.VectorMode != VectorModeFallback {
		t.Errorf("VectorMode = %q, want %q", cfg.VectorMode, VectorModeFallback)
	}
	if cfg.BuildVersion != "2.0.0-dev" {
		t.Errorf("BuildVersion = %q, want %q", cfg.BuildVersion, "2.0.0-dev")
	}
	if cfg.Readiness.MariaDBConfigured {
		t.Error("MariaDBConfigured should be false by default")
	}
	if cfg.Readiness.MilvusConfigured {
		t.Error("MilvusConfigured should be false by default")
	}
	if cfg.MariaDBProductReadEnabled {
		t.Error("MariaDBProductReadEnabled should be false by default")
	}
	if cfg.ChromaEnabled {
		t.Error("ChromaEnabled should be false by default in core_lite fallback mode")
	}
	if cfg.ChromaEndpoint != "" {
		t.Errorf("ChromaEndpoint should be empty by default, got %q", cfg.ChromaEndpoint)
	}
	if cfg.ChromaCollection != "archive_center_vectors" {
		t.Errorf("ChromaCollection = %q, want archive_center_vectors", cfg.ChromaCollection)
	}
	if cfg.ChromaAPIPath != "/api/v2" {
		t.Errorf("ChromaAPIPath = %q, want /api/v2", cfg.ChromaAPIPath)
	}
	if cfg.MilvusStubEnabled {
		t.Error("MilvusStubEnabled should be false by default")
	}
	if cfg.MilvusLitePath != "" {
		t.Errorf("MilvusLitePath should be empty by default, got %q", cfg.MilvusLitePath)
	}
	if cfg.MilvusSDKEnabled {
		t.Error("MilvusSDKEnabled should be false by default")
	}
	if cfg.MilvusRecallReadEnabled {
		t.Error("MilvusRecallReadEnabled should be false by default")
	}
	if cfg.MilvusProductReadEnabled {
		t.Error("MilvusProductReadEnabled should be false by default")
	}
	if cfg.MilvusEndpoint != "" {
		t.Errorf("MilvusEndpoint should be empty by default, got %q", cfg.MilvusEndpoint)
	}
	if cfg.PromptDir != "" {
		t.Errorf("PromptDir should be empty by default, got %q", cfg.PromptDir)
	}
	if cfg.Auth.Enforce {
		t.Error("Auth.Enforce should be false by default")
	}
	if cfg.Auth.BearerToken != "" {
		t.Error("Auth.BearerToken should be empty by default")
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("AllowedOrigins = %#v, want [*]", cfg.AllowedOrigins)
	}
	if cfg.PrunePolicy != "soft" {
		t.Errorf("PrunePolicy = %q, want soft", cfg.PrunePolicy)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("AC_BIND_ADDR", "0.0.0.0:8080")
	t.Setenv("AC_MODE", "live")
	t.Setenv("AC_STORE_MODE", "mariadb_shadow")
	t.Setenv("AC_BUILD_VERSION", "2.0.0-test")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac")
	t.Setenv("AC_BEARER_TOKEN", "secret-token")
	t.Setenv("AC_ENFORCE_AUTH", "true")
	t.Setenv("AC_PROMPT_DIR", "/tmp/prompts")
	t.Setenv("AC_ALLOWED_ORIGINS", "https://a.example, http://localhost:3000")
	t.Setenv("AC_PRUNE_POLICY", "off")
	t.Setenv("AC_RUNTIME_PROFILE", "vector_external")
	t.Setenv("AC_VECTOR_MODE", "external")
	t.Setenv("AC_CHROMA_ENDPOINT", "http://127.0.0.1:8000")
	t.Setenv("AC_CHROMA_COLLECTION", "archive_center_test_vectors")
	t.Setenv("AC_CHROMA_API_PATH", "/api/v1")

	cfg := Load()

	if cfg.BindAddr != "0.0.0.0:8080" {
		t.Errorf("BindAddr = %q, want %q", cfg.BindAddr, "0.0.0.0:8080")
	}
	if cfg.Mode != ModeLive {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeLive)
	}
	if cfg.StoreMode != StoreModeMariaDBShadow {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeMariaDBShadow)
	}
	if cfg.BuildVersion != "2.0.0-test" {
		t.Errorf("BuildVersion = %q, want %q", cfg.BuildVersion, "2.0.0-test")
	}
	if !cfg.Readiness.MariaDBConfigured {
		t.Error("MariaDBConfigured should be true when AC_MARIADB_DSN is set")
	}
	if cfg.Auth.BearerToken != "secret-token" {
		t.Errorf("Auth.BearerToken = %q, want %q", cfg.Auth.BearerToken, "secret-token")
	}
	if !cfg.Auth.Enforce {
		t.Error("Auth.Enforce should be true when AC_ENFORCE_AUTH=true")
	}
	if cfg.PromptDir != "/tmp/prompts" {
		t.Errorf("PromptDir = %q, want %q", cfg.PromptDir, "/tmp/prompts")
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://a.example" || cfg.AllowedOrigins[1] != "http://localhost:3000" {
		t.Errorf("AllowedOrigins = %#v", cfg.AllowedOrigins)
	}
	if cfg.PrunePolicy != "off" {
		t.Errorf("PrunePolicy = %q, want off", cfg.PrunePolicy)
	}
	if cfg.RuntimeProfile != RuntimeProfileVectorExternal {
		t.Errorf("RuntimeProfile = %q, want %q", cfg.RuntimeProfile, RuntimeProfileVectorExternal)
	}
	if cfg.VectorMode != VectorModeExternal {
		t.Errorf("VectorMode = %q, want %q", cfg.VectorMode, VectorModeExternal)
	}
	if !cfg.ChromaEnabled {
		t.Error("ChromaEnabled should be true when vector_external is selected")
	}
	if cfg.ChromaEndpoint != "http://127.0.0.1:8000" {
		t.Errorf("ChromaEndpoint = %q", cfg.ChromaEndpoint)
	}
	if cfg.ChromaCollection != "archive_center_test_vectors" {
		t.Errorf("ChromaCollection = %q", cfg.ChromaCollection)
	}
	if cfg.ChromaAPIPath != "/api/v1" {
		t.Errorf("ChromaAPIPath = %q", cfg.ChromaAPIPath)
	}
	if !cfg.Readiness.ChromaConfigured {
		t.Error("Readiness.ChromaConfigured should be true when AC_CHROMA_ENDPOINT is set")
	}
}

func TestLoadFixtureShadowStoreMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AC_STORE_MODE", "fixture_shadow")
	t.Setenv("AC_STORE_FIXTURE_DIR", dir)

	cfg := Load()
	if cfg.StoreMode != StoreModeFixtureShadow {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeFixtureShadow)
	}
	if cfg.StoreFixtureDir != dir {
		t.Errorf("StoreFixtureDir = %q, want %q", cfg.StoreFixtureDir, dir)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("fixture shadow config should validate: %v", err)
	}
}

func TestLoadMilvusStubEnabledOverride(t *testing.T) {
	t.Setenv("AC_MILVUS_STUB_ENABLED", "true")
	t.Setenv("AC_MILVUS_LITE_PATH", "/tmp/milvus_test.db")

	cfg := Load()
	if !cfg.MilvusStubEnabled {
		t.Error("MilvusStubEnabled should be true when AC_MILVUS_STUB_ENABLED=true")
	}
	if cfg.MilvusLitePath != "/tmp/milvus_test.db" {
		t.Errorf("MilvusLitePath = %q, want %q", cfg.MilvusLitePath, "/tmp/milvus_test.db")
	}
}

func TestLoadMariaDBProductReadEnabledOverride(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "mariadb_read_shadow")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac?parseTime=true")
	t.Setenv("AC_MARIADB_PRODUCT_READ_ENABLED", "true")

	cfg := Load()
	if !cfg.MariaDBProductReadEnabled {
		t.Error("MariaDBProductReadEnabled should be true when AC_MARIADB_PRODUCT_READ_ENABLED=true")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow MariaDB product read proof with mariadb_read_shadow and DSN: %v", err)
	}
}

func TestLoadChromaEndpointOverride(t *testing.T) {
	t.Setenv("AC_CHROMA_ENDPOINT", "http://127.0.0.1:8000")
	t.Setenv("AC_CHROMA_COLLECTION", "archive_center_vectors_live")

	cfg := Load()
	if !cfg.ChromaEnabled {
		t.Error("ChromaEnabled should be true for backward-compatible envs with AC_CHROMA_ENDPOINT")
	}
	if cfg.RuntimeProfile != RuntimeProfileFullLocal {
		t.Errorf("RuntimeProfile = %q, want %q", cfg.RuntimeProfile, RuntimeProfileFullLocal)
	}
	if cfg.VectorMode != VectorModeBundled {
		t.Errorf("VectorMode = %q, want %q", cfg.VectorMode, VectorModeBundled)
	}
	if cfg.ChromaEndpoint != "http://127.0.0.1:8000" {
		t.Errorf("ChromaEndpoint = %q", cfg.ChromaEndpoint)
	}
	if cfg.ChromaCollection != "archive_center_vectors_live" {
		t.Errorf("ChromaCollection = %q", cfg.ChromaCollection)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() should allow configured Chroma vector store: %v", err)
	}
}

func TestValidateAllowsShadowWithoutChromaEndpoint(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() should allow shadow mode to degrade without Chroma endpoint: %v", err)
	}
}

func TestValidateAllowsCoreLiteLiveWithoutChromaEndpoint(t *testing.T) {
	cfg := Default()
	cfg.Mode = ModeLive
	cfg.StoreMode = StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow core_lite live mode without Chroma endpoint: %v", err)
	}
}

func TestValidateBlocksVectorExternalWithoutChromaEndpoint(t *testing.T) {
	cfg := Default()
	cfg.Mode = ModeLive
	cfg.StoreMode = StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac"
	cfg.RuntimeProfile = RuntimeProfileVectorExternal
	cfg.VectorMode = VectorModeExternal
	cfg.ChromaEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject vector_external without Chroma endpoint")
	}
}

func TestValidateBlocksMismatchedRuntimeProfileAndVectorMode(t *testing.T) {
	cfg := Default()
	cfg.RuntimeProfile = RuntimeProfileCoreLite
	cfg.VectorMode = VectorModeExternal
	cfg.ChromaEnabled = true
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject core_lite with external vector mode")
	}
}

func TestLoadMariaDBAuthorityStoreMode(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "mariadb_authority")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac?parseTime=true")

	cfg := Load()
	if cfg.StoreMode != StoreModeMariaDBAuthority {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeMariaDBAuthority)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow MariaDB authority mode with DSN: %v", err)
	}
}

func TestValidateBlocksMariaDBAuthorityWithoutDSN(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeMariaDBAuthority
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject MariaDB authority mode without DSN")
	}
}

func TestValidateBlocksMariaDBProductReadWithoutReadShadow(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeMariaDBShadow
	cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac?parseTime=true"
	cfg.MariaDBProductReadEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject MariaDB product read proof outside mariadb_read_shadow")
	}
}

func TestValidateBlocksMariaDBProductReadWithoutDSN(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeMariaDBReadShadow
	cfg.MariaDBProductReadEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject MariaDB product read proof without DSN")
	}
}

func TestLoadMilvusSDKEnabledOverride(t *testing.T) {
	t.Setenv("AC_MILVUS_SDK_ENABLED", "true")
	t.Setenv("AC_MILVUS_ENDPOINT", "localhost:19530")

	cfg := Load()
	if !cfg.MilvusSDKEnabled {
		t.Error("MilvusSDKEnabled should be true when AC_MILVUS_SDK_ENABLED=true")
	}
	if cfg.MilvusEndpoint != "localhost:19530" {
		t.Errorf("MilvusEndpoint = %q, want %q", cfg.MilvusEndpoint, "localhost:19530")
	}
	if !cfg.Readiness.MilvusConfigured {
		t.Error("Readiness.MilvusConfigured should be true when endpoint is set")
	}
}

func TestLoadMilvusRecallReadEnabledOverride(t *testing.T) {
	t.Setenv("AC_MILVUS_SDK_ENABLED", "true")
	t.Setenv("AC_MILVUS_ENDPOINT", "localhost:19530")
	t.Setenv("AC_MILVUS_RECALL_READ_ENABLED", "true")

	cfg := Load()
	if !cfg.MilvusRecallReadEnabled {
		t.Error("MilvusRecallReadEnabled should be true when AC_MILVUS_RECALL_READ_ENABLED=true")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow bounded recall read drill with SDK endpoint: %v", err)
	}
}

func TestLoadMilvusProductReadEnabledOverride(t *testing.T) {
	t.Setenv("AC_MILVUS_SDK_ENABLED", "true")
	t.Setenv("AC_MILVUS_ENDPOINT", "localhost:19530")
	t.Setenv("AC_MILVUS_RECALL_READ_ENABLED", "true")
	t.Setenv("AC_MILVUS_PRODUCT_READ_ENABLED", "true")

	cfg := Load()
	if !cfg.MilvusProductReadEnabled {
		t.Error("MilvusProductReadEnabled should be true when AC_MILVUS_PRODUCT_READ_ENABLED=true")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow product read proof with SDK endpoint and recall read enabled: %v", err)
	}
}

func TestLoadIgnoresMilvusLiveEnabledEnvInR1(t *testing.T) {
	t.Setenv("AC_MILVUS_LIVE_ENABLED", "true")

	cfg := Load()
	if cfg.MilvusStubEnabled {
		t.Error("AC_MILVUS_LIVE_ENABLED must not enable the R1 stub or live retrieval")
	}
	if cfg.MilvusSDKEnabled {
		t.Error("AC_MILVUS_LIVE_ENABLED must not enable the SDK store")
	}
}

func TestValidateBlocksLiveAndCutoverWithoutMariaDBAuthority(t *testing.T) {
	for _, mode := range []Mode{ModeLive, ModeCutover} {
		cfg := Default()
		cfg.Mode = mode
		if err := cfg.Validate(); err == nil {
			t.Errorf("Validate() should reject mode %q without MariaDB authority", mode)
		}
	}
}

func TestValidateAllowsLiveAndCutoverWithMariaDBAuthority(t *testing.T) {
	for _, mode := range []Mode{ModeLive, ModeCutover} {
		cfg := Default()
		cfg.Mode = mode
		cfg.StoreMode = StoreModeMariaDBAuthority
		cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac"
		cfg.ChromaEndpoint = "http://127.0.0.1:8000"
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() should allow mode %q with MariaDB authority: %v", mode, err)
		}
	}
}

func TestValidateAllowsShadow(t *testing.T) {
	cfg := Default()
	cfg.Mode = ModeShadow
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow shadow mode: %v", err)
	}
}

func TestValidateBlocksMilvusRecallReadWithoutSDKEndpoint(t *testing.T) {
	cfg := Default()
	cfg.MilvusRecallReadEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject Milvus recall read drill without SDK endpoint")
	}
}

func TestValidateBlocksMilvusProductReadWithoutRecallRead(t *testing.T) {
	cfg := Default()
	cfg.MilvusSDKEnabled = true
	cfg.MilvusEndpoint = "localhost:19530"
	cfg.MilvusProductReadEnabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject Milvus product read proof without recall read enabled")
	}
}

func TestValidateBlocksInvalidStoreMode(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = "live"
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject invalid store mode")
	}
}

func TestValidateBlocksMariaDBShadowWithoutDSN(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeMariaDBShadow
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject mariadb_shadow without DSN")
	}
}

func TestValidateBlocksFixtureShadowWithoutExportDir(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeFixtureShadow
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject fixture_shadow without AC_STORE_FIXTURE_DIR")
	}
}

func TestValidateAllowsNoopDualShadowMariaDBShadowAndFixtureShadow(t *testing.T) {
	for _, sm := range []StoreMode{StoreModeNoop, StoreModeDualShadow, StoreModeMariaDBShadow, StoreModeFixtureShadow, StoreModeMariaDBReadShadow} {
		cfg := Default()
		cfg.StoreMode = sm
		if sm == StoreModeMariaDBShadow || sm == StoreModeMariaDBReadShadow {
			cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac"
		}
		if sm == StoreModeFixtureShadow {
			cfg.StoreFixtureDir = t.TempDir()
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() should allow store mode %q: %v", sm, err)
		}
	}
}

func TestLoadMariaDBReadShadowStoreMode(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "mariadb_read_shadow")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac")
	cfg := Load()
	if cfg.StoreMode != StoreModeMariaDBReadShadow {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeMariaDBReadShadow)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mariadb_read_shadow config should validate: %v", err)
	}
}

func TestValidateBlocksMariaDBReadShadowWithoutDSN(t *testing.T) {
	cfg := Default()
	cfg.StoreMode = StoreModeMariaDBReadShadow
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject mariadb_read_shadow without DSN")
	}
}

func TestLoadMariaDBReadShadowCaseInsensitive(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "MARIADB_READ_SHADOW")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac")
	cfg := Load()
	if cfg.StoreMode != StoreModeMariaDBReadShadow {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeMariaDBReadShadow)
	}
}

func TestIsLiveCutoverAllowed(t *testing.T) {
	cfg := Default()
	if cfg.IsLiveCutoverAllowed() {
		t.Error("IsLiveCutoverAllowed() should be false without product authority config")
	}
	cfg.Mode = ModeLive
	cfg.StoreMode = StoreModeMariaDBAuthority
	cfg.MariaDBDSN = "user:pass@tcp(localhost:3306)/ac"
	if !cfg.IsLiveCutoverAllowed() {
		t.Error("IsLiveCutoverAllowed() should be true for core_lite MariaDB authority without Chroma endpoint")
	}
	cfg.RuntimeProfile = RuntimeProfileVectorExternal
	cfg.VectorMode = VectorModeExternal
	cfg.ChromaEnabled = true
	if cfg.IsLiveCutoverAllowed() {
		t.Error("IsLiveCutoverAllowed() should be false when vector_external has no Chroma endpoint")
	}
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	if !cfg.IsLiveCutoverAllowed() {
		t.Error("IsLiveCutoverAllowed() should be true with live MariaDB authority and required Chroma config")
	}
}

func TestLoadModeCaseInsensitive(t *testing.T) {
	t.Setenv("AC_MODE", "SHADOW")
	cfg := Load()
	if cfg.Mode != ModeShadow {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeShadow)
	}
}

func TestLoadStoreModeCaseInsensitive(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "MARIADB_SHADOW")
	t.Setenv("AC_MARIADB_DSN", "user:pass@tcp(localhost:3306)/ac")
	cfg := Load()
	if cfg.StoreMode != StoreModeMariaDBShadow {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, StoreModeMariaDBShadow)
	}
}

func TestLoadUnknownStoreModeRejectedByValidate(t *testing.T) {
	t.Setenv("AC_STORE_MODE", "unknown")
	cfg := Load()
	if cfg.StoreMode != "unknown" {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, "unknown")
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should reject unknown store mode")
	}
}

func TestLoadUnknownModeFallsBack(t *testing.T) {
	t.Setenv("AC_MODE", "unknown")
	cfg := Load()
	if cfg.Mode != ModeShadow {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeShadow)
	}
}

func TestStringRedacted(t *testing.T) {
	cfg := Default()
	s := cfg.String()
	if s == "" {
		t.Error("String() should return a non-empty representation")
	}
	if !strings.Contains(s, "StoreMode=noop") {
		t.Errorf("String() should contain StoreMode, got %q", s)
	}
	if !strings.Contains(s, "MilvusStubEnabled=false") {
		t.Errorf("String() should contain MilvusStubEnabled=false, got %q", s)
	}
	if !strings.Contains(s, "RuntimeProfile=core_lite") {
		t.Errorf("String() should contain RuntimeProfile=core_lite, got %q", s)
	}
	if !strings.Contains(s, "VectorMode=fallback") {
		t.Errorf("String() should contain VectorMode=fallback, got %q", s)
	}
	if !strings.Contains(s, "ChromaEnabled=false") {
		t.Errorf("String() should contain ChromaEnabled=false, got %q", s)
	}
	if !strings.Contains(s, "MilvusSDKEnabled=false") {
		t.Errorf("String() should contain MilvusSDKEnabled=false, got %q", s)
	}
}

func TestLoadPreservesDefaultsWhenNoEnv(t *testing.T) {
	// Ensure environment is clean for known keys.
	for _, key := range []string{"AC_BIND_ADDR", "AC_MODE", "AC_STORE_MODE", "AC_RUNTIME_PROFILE", "AC_VECTOR_MODE", "AC_BUILD_VERSION", "AC_BUILD_COMMIT", "AC_BUILD_TIME", "AC_MARIADB_DSN", "AC_STORE_FIXTURE_DIR", "AC_CHROMA_ENDPOINT", "AC_CHROMA_COLLECTION", "AC_CHROMA_API_PATH", "AC_MILVUS_ENDPOINT", "AC_BEARER_TOKEN", "AC_ENFORCE_AUTH", "AC_MILVUS_LIVE_ENABLED", "AC_MILVUS_STUB_ENABLED", "AC_MILVUS_LITE_PATH", "AC_MILVUS_SDK_ENABLED", "AC_PROMPT_DIR"} {
		os.Unsetenv(key)
	}

	cfg := Load()
	want := Default()

	if cfg.BindAddr != want.BindAddr {
		t.Errorf("BindAddr = %q, want %q", cfg.BindAddr, want.BindAddr)
	}
	if cfg.Mode != want.Mode {
		t.Errorf("Mode = %q, want %q", cfg.Mode, want.Mode)
	}
	if cfg.StoreMode != want.StoreMode {
		t.Errorf("StoreMode = %q, want %q", cfg.StoreMode, want.StoreMode)
	}
	if cfg.RuntimeProfile != want.RuntimeProfile {
		t.Errorf("RuntimeProfile = %q, want %q", cfg.RuntimeProfile, want.RuntimeProfile)
	}
	if cfg.VectorMode != want.VectorMode {
		t.Errorf("VectorMode = %q, want %q", cfg.VectorMode, want.VectorMode)
	}
	if cfg.BuildVersion != want.BuildVersion {
		t.Errorf("BuildVersion = %q, want %q", cfg.BuildVersion, want.BuildVersion)
	}
	if cfg.Readiness.MariaDBConfigured != want.Readiness.MariaDBConfigured {
		t.Errorf("MariaDBConfigured = %v, want %v", cfg.Readiness.MariaDBConfigured, want.Readiness.MariaDBConfigured)
	}
	if cfg.Readiness.MilvusConfigured != want.Readiness.MilvusConfigured {
		t.Errorf("MilvusConfigured = %v, want %v", cfg.Readiness.MilvusConfigured, want.Readiness.MilvusConfigured)
	}
	if cfg.Readiness.ChromaConfigured != want.Readiness.ChromaConfigured {
		t.Errorf("ChromaConfigured = %v, want %v", cfg.Readiness.ChromaConfigured, want.Readiness.ChromaConfigured)
	}
	if cfg.ChromaEnabled != want.ChromaEnabled {
		t.Errorf("ChromaEnabled = %v, want %v", cfg.ChromaEnabled, want.ChromaEnabled)
	}
	if cfg.ChromaEndpoint != want.ChromaEndpoint {
		t.Errorf("ChromaEndpoint = %q, want %q", cfg.ChromaEndpoint, want.ChromaEndpoint)
	}
	if cfg.MilvusStubEnabled != want.MilvusStubEnabled {
		t.Errorf("MilvusStubEnabled = %v, want %v", cfg.MilvusStubEnabled, want.MilvusStubEnabled)
	}
	if cfg.MilvusLitePath != want.MilvusLitePath {
		t.Errorf("MilvusLitePath = %q, want %q", cfg.MilvusLitePath, want.MilvusLitePath)
	}
	if cfg.MilvusSDKEnabled != want.MilvusSDKEnabled {
		t.Errorf("MilvusSDKEnabled = %v, want %v", cfg.MilvusSDKEnabled, want.MilvusSDKEnabled)
	}
	if cfg.MilvusEndpoint != want.MilvusEndpoint {
		t.Errorf("MilvusEndpoint = %q, want %q", cfg.MilvusEndpoint, want.MilvusEndpoint)
	}
	if cfg.PromptDir != want.PromptDir {
		t.Errorf("PromptDir = %q, want %q", cfg.PromptDir, want.PromptDir)
	}
	if cfg.Auth.Enforce != want.Auth.Enforce {
		t.Errorf("Auth.Enforce = %v, want %v", cfg.Auth.Enforce, want.Auth.Enforce)
	}
	if cfg.Auth.BearerToken != want.Auth.BearerToken {
		t.Errorf("Auth.BearerToken = %q, want %q", cfg.Auth.BearerToken, want.Auth.BearerToken)
	}
}
