package store

import (
	"reflect"
	"testing"
)

func TestWorldRuleScopeChainIncludesSessionGlobalRules(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  []string
	}{
		{name: "root", scope: "root", want: []string{"root", "session"}},
		{name: "world alias", scope: "world", want: []string{"root", "session"}},
		{name: "global alias", scope: "global", want: []string{"root", "session"}},
		{name: "empty", scope: "", want: []string{"root", "session"}},
		{name: "location", scope: "location", want: []string{"location", "region", "root", "session"}},
		{name: "region", scope: "region", want: []string{"region", "root", "session"}},
		{name: "faction", scope: "faction", want: []string{"faction", "root", "session"}},
		{name: "system", scope: "system", want: []string{"system", "root", "session"}},
		{name: "session", scope: "session", want: []string{"session"}},
		{name: "country alias", scope: "country", want: []string{"region", "root", "session"}},
		{name: "city alias", scope: "city", want: []string{"region", "root", "session"}},
		{name: "dungeon alias", scope: "dungeon", want: []string{"location", "region", "root", "session"}},
		{name: "church alias", scope: "church", want: []string{"faction", "root", "session"}},
		{name: "progression alias", scope: "progression", want: []string{"system", "root", "session"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WorldRuleScopeChain(tt.scope); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("WorldRuleScopeChain(%q) = %#v, want %#v", tt.scope, got, tt.want)
			}
		})
	}
}
