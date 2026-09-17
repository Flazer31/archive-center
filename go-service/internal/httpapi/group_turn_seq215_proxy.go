package httpapi

func buildSeq215P730ProxyPluginMainModelSeparated() map[string]any {
	return map[string]any{
		"version":            "s215-p730.v1",
		"role":               "seq215_proxy_plugin_main_model_separated",
		"truth_authority":    false,
		"sub_step":           "21.5-proxy-config-split",
		"route":              "/proxy/plugin-main",
		"ownership":          "go_httpapi_group_proxy",
		"monolith_separated": true,
		"note":               "/proxy/plugin-main request model and provider/upstream auth helpers separated from main.py monolith into Go httpapi group_proxy.",
		"policy_version":     "s215-sc.v1",
		"mode":               "seq215_proxy_plugin_main_model_separated_definition",
	}
}

// buildSeq215P731ProviderOwnershipSplit exposes the evidence that OpenAI-like,
// Claude, Gemini, Vertex, Copilot proxy call ownership is split to a dedicated
// Go service for SEQ-21.5-P731.
func buildSeq215P731ProviderOwnershipSplit() map[string]any {
	return map[string]any{
		"version":         "s215-p731.v1",
		"role":            "seq215_provider_ownership_split",
		"truth_authority": false,
		"sub_step":        "21.5-proxy-config-split",
		"providers":       []string{"openai", "claude", "gemini", "vertex", "copilot"},
		"ownership":       "go_httpapi_group_proxy",
		"note":            "OpenAI-like, Claude, Gemini, Vertex, Copilot proxy call ownership split to dedicated Go service (group_proxy).",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_provider_ownership_split_definition",
	}
}

// buildSeq215P732ThinProxyRoute exposes the evidence that /proxy/plugin-main
// is a thin route delegating to the proxy payload handler for SEQ-21.5-P732.
func buildSeq215P732ThinProxyRoute() map[string]any {
	return map[string]any{
		"version":         "s215-p732.v1",
		"role":            "seq215_thin_proxy_route",
		"truth_authority": false,
		"sub_step":        "21.5-proxy-config-split",
		"route":           "/proxy/plugin-main",
		"route_type":      "thin_delegating",
		"handler":         "handleProxyPluginMain",
		"delegate":        "performProxyPluginMain",
		"note":            "/proxy/plugin-main is a thin route delegating from handleProxyPluginMain to performProxyPluginMain in Go.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_thin_proxy_route_definition",
	}
}

// buildSeq215P733ConfigServiceSplit exposes the evidence that /config/update
// key mapping and type normalization are owned by the Go runtime config service
// while persistence remains explicitly runtime-only for SEQ-21.5-P733.
func buildSeq215P733ConfigServiceSplit() map[string]any {
	return map[string]any{
		"version":          "s215-p733.v1",
		"role":             "seq215_config_service_split",
		"truth_authority":  false,
		"sub_step":         "21.5-proxy-config-split",
		"route":            "/config/update",
		"ownership":        "go_httpapi_runtime_config",
		"persistence_mode": "runtime_only",
		"persisted":        false,
		"implemented_features": []string{
			"key_mapping",
			"type_normalization",
			"runtime_config_trace",
			"secret_response_masking",
		},
		"deferred_features": []string{
			"env_file_persistence",
			"encrypted_api_key_persistence",
		},
		"env_file_persistence":          "not_enabled_in_2_0",
		"encrypted_api_key_persistence": "not_enabled_in_2_0",
		"boundary":                      "runtime_config_owner_split_without_secret_persistence",
		"note":                          "/config/update key mapping and type normalization are moved to Go runtime_config; responses stay runtime_only and do not persist or echo secrets.",
		"policy_version":                "s215-sc.v1",
		"mode":                          "seq215_config_service_split_definition",
	}
}

// buildSeq215P734ThinConfigRoute exposes the evidence that /config/update is a
// thin route delegating to the runtime config update handler for SEQ-21.5-P734.
func buildSeq215P734ThinConfigRoute() map[string]any {
	return map[string]any{
		"version":         "s215-p734.v1",
		"role":            "seq215_thin_config_route",
		"truth_authority": false,
		"sub_step":        "21.5-proxy-config-split",
		"route":           "/config/update",
		"route_type":      "thin_delegating",
		"handler":         "handleConfigUpdate",
		"note":            "/config/update is a thin route delegating to update_runtime_config handler in Go.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_thin_config_route_definition",
	}
}

// buildSeq215P735RoutesExplicitlyWired exposes the evidence that both
// /proxy/plugin-main and /config/update are explicitly wired in the Go route
// registry for SEQ-21.5-P735.
func buildSeq215P735RoutesExplicitlyWired() map[string]any {
	return map[string]any{
		"version":          "s215-p735.v1",
		"role":             "seq215_routes_explicitly_wired",
		"truth_authority":  false,
		"sub_step":         "21.5-proxy-config-split",
		"routes":           []string{"/proxy/plugin-main", "/config/update"},
		"wiring_location":  "registerProxyRoutes + registerConfigRoutes",
		"explicit_binding": true,
		"note":             "Both /proxy/plugin-main and /config/update are explicitly wired in Go route registry with explicit dependency binding.",
		"policy_version":   "s215-sc.v1",
		"mode":             "seq215_routes_explicitly_wired_definition",
	}
}

// buildSeq215P736PublicPathsPreserved exposes the evidence that public paths
// /proxy/plugin-main and /config/update are preserved unchanged for SEQ-21.5-P736.
func buildSeq215P736PublicPathsPreserved() map[string]any {
	return map[string]any{
		"version":         "s215-p736.v1",
		"role":            "seq215_public_paths_preserved",
		"truth_authority": false,
		"sub_step":        "21.5-proxy-config-split",
		"paths":           []string{"/proxy/plugin-main", "/config/update"},
		"unchanged":       true,
		"note":            "Public paths /proxy/plugin-main and /config/update are preserved unchanged in 2.0.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_public_paths_preserved_definition",
	}
}

// buildSeq215P737CompatibilityWrapper exposes the evidence that a compatibility
// wrapper/direct caller compatibility is maintained for SEQ-21.5-P737.
func buildSeq215P737CompatibilityWrapper() map[string]any {
	return map[string]any{
		"version":                       "s215-p737.v1",
		"role":                          "seq215_compatibility_wrapper",
		"truth_authority":               false,
		"sub_step":                      "21.5-proxy-config-split",
		"wrapper_present":               true,
		"backward_compatible":           true,
		"compatibility_mode":            "stable_route_and_handler_compatibility",
		"python_wrapper_not_applicable": true,
		"note":                          "2.0 preserves direct caller compatibility through the stable /config/update route and handleConfigUpdate/updateRuntimeConfig handler path; the literal backend.main.config_update Python wrapper is not applicable in Go.",
		"policy_version":                "s215-sc.v1",
		"mode":                          "seq215_compatibility_wrapper_definition",
	}
}

// buildSeq215P738RouteLevelTests exposes the evidence that route-level tests
// for /config/update and /proxy/plugin-main no-upstream rejection exist for
// SEQ-21.5-P738.
func buildSeq215P738RouteLevelTests() map[string]any {
	return map[string]any{
		"version":         "s215-p738.v1",
		"role":            "seq215_route_level_tests",
		"truth_authority": false,
		"sub_step":        "21.5-proxy-config-split",
		"test_routes":     []string{"/config/update", "/proxy/plugin-main"},
		"test_focus":      "no_upstream_rejection",
		"direct_route_assertions": []string{
			"proxy_missing_endpoint_returns_400_without_upstream",
			"config_update_masks_secret_and_reports_runtime_only",
		},
		"test_names": []string{
			"TestSeq215P738RouteLevelTests",
			"TestHandleProxyPluginMainMissingEndpointReturns400",
			"TestConfigUpdateProjectGUISettingsTraceMasksSecrets",
		},
		"note":           "Route-level tests for /config/update and /proxy/plugin-main no-upstream rejection are directly exercised in Go test suite.",
		"policy_version": "s215-sc.v1",
		"mode":           "seq215_route_level_tests_definition",
	}
}

// buildSeq215P739JSRouteUsage exposes the evidence that Archive Center.js sends
// secret-bearing config through /config/update and provider calls through
// /proxy/plugin-main for SEQ-21.5-P739.
func buildSeq215P739JSRouteUsage() map[string]any {
	return map[string]any{
		"version":          "s215-p739.v1",
		"role":             "seq215_js_route_usage",
		"truth_authority":  false,
		"sub_step":         "21.5-proxy-config-split",
		"config_route":     "/config/update",
		"proxy_route":      "/proxy/plugin-main",
		"js_config_sender": "syncConfigToBackend",
		"js_proxy_sender":  "bridgeFetch",
		"note":             "Archive Center.js sends secret-bearing config through /config/update and provider calls through /proxy/plugin-main.",
		"policy_version":   "s215-sc.v1",
		"mode":             "seq215_js_route_usage_definition",
	}
}

// buildSeq215P740MonolithNotApplicable exposes the evidence that main.py line
// count reduction is recorded as "monolith-not-applicable" in 2.0 Go context
// for SEQ-21.5-P740.
func buildSeq215P740MonolithNotApplicable() map[string]any {
	return map[string]any{
		"version":                "s215-p740.v1",
		"role":                   "seq215_monolith_not_applicable",
		"truth_authority":        false,
		"sub_step":               "21.5-proxy-config-split",
		"original_claim":         "main.py_line_count_reduced",
		"go_interpretation":      "monolith_not_applicable",
		"go_route_owner_split":   true,
		"beta_reference_mutated": false,
		"reason":                 "2.0 Go backend has no single main.py monolith; route/service ownership split makes line-count reduction concept inapplicable.",
		"note":                   "main.py line count reduction recorded as 'monolith-not-applicable' in 2.0 Go context; original Beta 0.8 intent preserved without modification.",
		"policy_version":         "s215-sc.v1",
		"mode":                   "seq215_monolith_not_applicable_definition",
	}
}

// ===========================================================================
// SEQ-21.5 JS ownership boundary and function preservation evidence (P776 ~ P789)
// ===========================================================================

// buildSeq215P776PrepareTurnBundleNormalUse exposes the evidence that backend
// /prepare-turn bundle fields are treated as normal-use data sources when
// present for SEQ-21.5-P776.
