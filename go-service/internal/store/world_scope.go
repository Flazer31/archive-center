package store

import "strings"

// NormalizeWorldRuleScope maps legacy/model aliases onto the canonical
// world-rule scope vocabulary. "world" has historically been emitted by LLM
// extraction for setting-global rules; Archive Center treats that as root.
func NormalizeWorldRuleScope(scope string) string {
	normalized := strings.ToLower(strings.TrimSpace(scope))
	switch normalized {
	case "world", "global", "universal":
		return "root"
	case "area", "city", "country", "nation", "kingdom", "province", "territory", "district", "zone":
		return "region"
	case "place", "site", "base", "building", "room", "facility", "dungeon", "landmark":
		return "location"
	case "organization", "organisation", "org", "group", "guild", "church", "cult", "clan", "gang", "government", "party", "team", "order":
		return "faction"
	case "mechanic", "mechanics", "progression", "economy", "magic", "technology", "tech", "combat", "reward", "upgrade":
		return "system"
	default:
		return normalized
	}
}

// WorldRuleScopeChain returns the active world-rule scope followed by ancestors.
// It mirrors the Python 0.8 World Graph Lite parent contract.
func WorldRuleScopeChain(scope string) []string {
	normalized := NormalizeWorldRuleScope(scope)
	switch normalized {
	case "location":
		return []string{"location", "region", "root", "session"}
	case "region":
		return []string{"region", "root", "session"}
	case "faction":
		return []string{"faction", "root", "session"}
	case "system":
		return []string{"system", "root", "session"}
	case "session":
		return []string{"session"}
	case "root", "":
		return []string{"root", "session"}
	default:
		return []string{normalized}
	}
}
