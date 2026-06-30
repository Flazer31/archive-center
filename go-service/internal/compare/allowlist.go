package compare

import "strings"

// readOnlyAllowlist defines the endpoints that the compare harness is permitted
// to call. Any method/path not in this list (or matching a prefix rule) is
// rejected with ErrUnsafeRoute.
var readOnlyAllowlist = map[string]struct{}{
	"GET /health":                         {},
	"GET /ready":                          {},
	"GET /version":                        {},
	"GET /stats":                          {},
	"GET /wakeup":                         {},
	"GET /sessions":                       {},
	"GET /audit":                          {},
	"GET /chroma-shadow/preflight":        {},
	"POST /search":                        {},
	"GET /kg/recall":                      {},
	"POST /kg/recall":                     {},
	"GET /sessions/compare":               {},
	"POST /chapters/dry-run":              {},
	"POST /chapters/search":               {},
	"POST /episodes/search":               {},
	"GET /retrieval-index/runtime-config": {},
	"GET /intent-routing/runtime-config":  {},
}

// prefixAllowlist matches parameterized read-only paths.
var prefixAllowlist = []string{
	"GET /active-states/",
	"GET /retrieval-index/",
	"GET /sessions/",
	"GET /explorer/",
	"GET /metrics/",
	"GET /world-rules/",
	"GET /characters/",
	"GET /kg/recall?",
	"GET /storylines/",
	"GET /episodes/",
	"GET /pending-threads/",
	"GET /narrative-control/",
	"GET /session-state/",
	"GET /canonical-state-layer/",
	"GET /continuity-pack/",
	"GET /momentum-packet/",
	"GET /session/",
	"GET /long-session-health/",
	"GET /prompts",
	"GET /prompts/",
}

func isAllowed(method, path string) bool {
	endpoint := method + " " + path
	if _, ok := readOnlyAllowlist[endpoint]; ok {
		return true
	}
	for _, prefix := range prefixAllowlist {
		if strings.HasPrefix(endpoint, prefix) {
			return true
		}
	}
	return false
}
