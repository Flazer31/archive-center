package main

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func deriveSourceMode(sqliteDB, exportDir string) string {
	hasSQLite := strings.TrimSpace(sqliteDB) != ""
	hasExport := strings.TrimSpace(exportDir) != ""
	switch {
	case hasSQLite && hasExport:
		return "ambiguous"
	case hasSQLite:
		return "sqlite-db"
	case hasExport:
		return "export-dir"
	default:
		return "none"
	}
}

func summarizeVectorRuntime() map[string]any {
	endpoint := strings.TrimSpace(os.Getenv("AC_CHROMA_ENDPOINT"))
	collection := strings.TrimSpace(os.Getenv("AC_CHROMA_COLLECTION"))
	if collection == "" {
		collection = "archive_center_vectors"
	}
	apiPath := strings.TrimSpace(os.Getenv("AC_CHROMA_API_PATH"))
	if apiPath == "" {
		apiPath = "/api/v1"
	}
	return map[string]any{
		"accelerator":                             "chromadb",
		"chromadb_required":                       true,
		"chromadb_endpoint_configured":            endpoint != "",
		"chromadb_endpoint_host":                  routeSmokeEndpointHost(endpoint),
		"chromadb_collection":                     collection,
		"chromadb_api_path":                       apiPath,
		"live_cutover_requires_chromadb_endpoint": true,
	}
}

func summarizeRollbackProof(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested":   requested,
		"rolled_back": false,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name == "rollback-ready-check" {
			out["status"] = step.Status
			out["rolled_back"] = step.Status == "ok"
			out["note"] = step.Note
			if step.Error != "" {
				out["error"] = step.Error
			}
			return out
		}
	}
	out["status"] = "missing"
	return out
}

func summarizeRouteWriteSmoke(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "route-write-smoke" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}

func summarizeSessionIsolationSmoke(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "session-isolation-smoke" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}
func summarizeBackupRestore(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	for _, step := range steps {
		if step.Name != "backup-restore-drill" {
			continue
		}
		if step.Details != nil {
			for k, v := range step.Details {
				out[k] = v
			}
		}
		out["status"] = step.Status
		if step.Error != "" {
			out["error"] = step.Error
		}
		return out
	}
	out["status"] = "missing"
	return out
}

func summarizeAuthorityCutover(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var summary map[string]any
	readyOK := false
	routeOK := false
	replayOK := false
	rollbackOK := false
	fallbackOK := false
	for _, step := range steps {
		switch step.Name {
		case "authority-cutover-summary":
			summary = map[string]any{"status": step.Status}
			if step.Details != nil {
				for k, v := range step.Details {
					summary[k] = v
				}
			}
			if step.Error != "" {
				summary["error"] = step.Error
			}
		case "authority-ready-check":
			readyOK = step.Status == "ok"
		case "authority-route-write-smoke":
			routeOK = step.Status == "ok"
			if step.Details != nil {
				out["route_write_smoke"] = step.Details
			}
		case "authority-post-cutover-replay":
			replayOK = step.Status == "ok"
		case "rollback-ready-check":
			rollbackOK = step.Status == "ok"
		case "python-fallback-replay":
			fallbackOK = step.Status == "ok"
		}
	}
	if summary != nil {
		for k, v := range summary {
			out[k] = v
		}
	}
	out["store_mode"] = "mariadb_authority"
	out["authority_switch"] = true
	out["persistent_switch"] = false
	out["go_default_switch"] = false
	out["python_runtime_retired"] = false
	out["ready_check"] = readyOK
	out["route_write_smoke_ok"] = routeOK
	out["post_cutover_replay"] = replayOK
	out["rollback_available"] = rollbackOK
	out["fallback_available"] = fallbackOK
	if readyOK && routeOK && replayOK && rollbackOK {
		out["status"] = "ok"
		return out
	}
	out["status"] = "missing"
	if summary != nil || readyOK || routeOK || replayOK || rollbackOK || fallbackOK {
		out["status"] = "failed"
	}
	return out
}

func summarizeDefaultSwitchRehearsal(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var goCandidate map[string]any
	var pythonFallback map[string]any
	for _, step := range steps {
		switch step.Name {
		case "go-default-candidate-probe":
			goCandidate = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					goCandidate[k] = v
				}
			}
			if step.Error != "" {
				goCandidate["error"] = step.Error
			}
		case "python-fallback-replay":
			pythonFallback = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					pythonFallback[k] = v
				}
			}
			if step.Error != "" {
				pythonFallback["error"] = step.Error
			}
		}
	}
	out["go_candidate"] = goCandidate
	out["python_fallback"] = pythonFallback
	out["selected_runtime"] = "go_rehearsal"
	out["fallback_runtime"] = "python_fallback"
	out["authority_switch"] = false
	out["go_default_switch"] = false
	out["python_runtime_retired"] = false
	ok := statusOKMap(goCandidate) && statusOKMap(pythonFallback)
	if ok {
		out["status"] = "ok"
		out["fallback_available"] = true
		return out
	}
	out["status"] = "missing"
	out["fallback_available"] = false
	if goCandidate != nil || pythonFallback != nil {
		out["status"] = "failed"
	}
	return out
}

func summarizeDefaultRuntimeSwitch(steps []executedStep, requested bool) map[string]any {
	out := map[string]any{
		"requested": requested,
	}
	if !requested {
		out["status"] = "not_requested"
		return out
	}
	var gate map[string]any
	var pythonFallback map[string]any
	shadowReplayOK := false
	rollbackOK := false
	for _, step := range steps {
		switch step.Name {
		case "go-default-actual-switch-gate":
			gate = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					gate[k] = v
				}
			}
			if step.Error != "" {
				gate["error"] = step.Error
			}
		case "python-fallback-replay":
			pythonFallback = map[string]any{
				"status": step.Status,
			}
			if step.Details != nil {
				for k, v := range step.Details {
					pythonFallback[k] = v
				}
			}
			if step.Error != "" {
				pythonFallback["error"] = step.Error
			}
		case "shadow-value-report":
			shadowReplayOK = step.Status == "ok"
		case "rollback-ready-check":
			rollbackOK = step.Status == "ok"
		}
	}
	out["go_gate"] = gate
	out["python_fallback"] = pythonFallback
	out["selected_runtime"] = "go"
	out["fallback_runtime"] = "python_fallback"
	out["switch_scope"] = "managed_disposable_actual"
	out["authority_switch"] = false
	out["go_default_switch"] = true
	out["persistent_switch"] = false
	out["python_runtime_retired"] = false
	out["post_switch_replay"] = shadowReplayOK
	out["rollback_available"] = rollbackOK
	out["fallback_available"] = statusOKMap(pythonFallback)
	if statusOKMap(gate) && statusOKMap(pythonFallback) && shadowReplayOK && rollbackOK {
		out["status"] = "ok"
		return out
	}
	out["status"] = "missing"
	if gate != nil || pythonFallback != nil || shadowReplayOK || rollbackOK {
		out["status"] = "failed"
	}
	return out
}

func statusOKMap(m map[string]any) bool {
	if m == nil {
		return false
	}
	status, _ := m["status"].(string)
	return status == "ok"
}

func buildPassword(sessionID string) string {
	return safeSessionID(sessionID) + "-pass"
}

func buildInternalDSN(dataDir string, port int, sessionID string) string {
	user := "ac_root"
	password := buildPassword(sessionID)
	dbName := "archive_center_temp"
	return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?parseTime=true&timeout=3s&readTimeout=3s&writeTimeout=3s", user, password, port, dbName)
}

func redactDSN(dsn string) string {
	atIdx := strings.Index(dsn, "@")
	if atIdx == -1 {
		return dsn
	}
	prefix := dsn[:atIdx]
	suffix := dsn[atIdx:]
	colonIdx := strings.Index(prefix, ":")
	if colonIdx == -1 {
		return dsn
	}
	return prefix[:colonIdx+1] + "***" + suffix
}

func quoteStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func escapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func safeSessionID(sessionID string) string {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "session"
	}
	return out
}
