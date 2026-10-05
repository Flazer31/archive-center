package httpapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func criticIdentityRowsForTest(t *testing.T, ledger map[string]any) []map[string]any {
	t.Helper()
	index := mapFromAny(ledger["existing_identity_index"])
	fields := stringsFromAny(index["fields"])
	out := []map[string]any{}
	for _, raw := range sliceFromAny(index["groups"]) {
		group := mapFromAny(raw)
		reader := csv.NewReader(strings.NewReader(stringFromMap(group, "rows")))
		reader.Comma, reader.FieldsPerRecord = '|', -1
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, values := range rows {
			for i, cell := range values {
				values[i] = strings.NewReplacer("\\\\", "\\", "\\r", "\r", "\\n", "\n").Replace(cell)
			}
			if len(values) != 2 && len(values) != 4 {
				t.Fatal("identity table has inconsistent columns")
			}
			turn, err := strconv.Atoi(values[1])
			if err != nil {
				t.Fatal(err)
			}
			row := map[string]any{"subject": group["subject"], "source_turn": turn, "lifecycle_key": values[0]}
			if len(values) == 2 {
				row["lifecycle_key"] = stringFromMap(group, "key_prefix") + values[0]
			} else {
				row["state_slot"], row["value"] = values[2], values[3]
			}
			for _, field := range []string{"claim_scope", "perspective_owner"} {
				if value, exists := group[field]; exists {
					row[field] = value
				}
			}
			out = append(out, row)
		}
	}
	for _, raw := range sliceFromAny(index["rows"]) {
		values := sliceFromAny(raw)
		if len(values) != len(fields) {
			t.Fatal("identity index has inconsistent columns")
		}
		row := map[string]any{}
		for i, field := range fields {
			row[field] = values[i]
		}
		out = append(out, row)
	}
	return out
}

func TestCriticIdentityIndexBoundedOrderingAndExactValues(t *testing.T) {
	ref := func(subject, key, value string) any {
		return map[string]any{"subject": subject, "state_slot": "rank", "lifecycle_key": key, "value": value, "claim_scope": "objective"}
	}
	inputs := []map[string]any{
		{"id": 1, "turn_index": 1, "recorded_state_claims": []any{ref("Mira", "mira_reused", "old")}},
		{"id": 2, "turn_index": 2, "recorded_state_claims": []any{ref("Mira", "mira_reused", "6 (azure / gold)")}},
		{"id": 3, "turn_index": 3, "recorded_state_claims": []any{ref("Mira", "mira_newer", "雪\r\n\"\\n|<>&")}},
		{"id": 4, "turn_index": 4, "recorded_state_claims": []any{ref("Mira", "mira_latest", "unchanged")}},
		{"id": 5, "turn_index": 5, "recorded_state_claims": []any{ref("Harin", "harin_previous_only", "cobalt guardian northern oath")}},
	}
	before := mustCompactJSON(inputs)
	run := func(budget int) (map[string]any, map[string]any) {
		latest := map[string]map[string]any{}
		turns := map[string]map[int]bool{}
		for _, memory := range inputs {
			claim := cloneMapAny(mapFromAny(sliceFromAny(memory["recorded_state_claims"])[0]))
			key, turn := stringFromMap(claim, "lifecycle_key"), intFromAny(memory["turn_index"], 0)
			if turns[key] == nil {
				turns[key] = map[int]bool{}
			}
			turns[key][turn] = true
			if prior := latest[key]; prior == nil || turn > intFromAny(prior["source_turn"], 0) {
				claim["source_turn"] = turn
				latest[key] = claim
			}
		}
		index, trace := buildCompleteTurnCriticIdentityIndex(latest, turns, nil, "Mira sits.", "Mira sits. Harin sits.", nil, budget)
		if index == nil {
			return nil, trace
		}
		return map[string]any{"existing_identity_index": index}, trace
	}
	full, _ := run(4000)
	rows := criticIdentityRowsForTest(t, full)
	want := []string{"mira_latest", "mira_newer", "mira_reused", "harin_previous_only"}
	if len(rows) != len(want) {
		t.Fatalf("missing structural identity: %s", mustCompactJSON(full))
	}
	for i, key := range want {
		if rows[i]["lifecycle_key"] != key {
			t.Fatalf("current subjects then recent sources: row %d = %v, want %s", i, rows[i], key)
		}
	}
	if rows[2]["value"] != "6 (azure / gold)" || rows[1]["value"] != "雪\r\n\"\\n|<>&" {
		t.Fatal("value was shortened or escaped lossily")
	}
	fields := stringsFromAny(mapFromAny(full["existing_identity_index"])["fields"])
	for _, dropped := range []string{"memory_id", "claim_scope", "perspective_owner"} {
		if slices.Contains(fields, dropped) {
			t.Fatalf("unneeded default column retained: %s", dropped)
		}
	}
	for _, budget := range []int{-1, 0, 1, 137, 200, 800, 4000} {
		ledger, trace := run(budget)
		base := cloneMapAny(ledger)
		delete(base, "existing_identity_index")
		if len(base) == 0 {
			base = nil
		}
		cost := len([]rune(mustCompactJSON(ledger))) - len([]rune(mustCompactJSON(base)))
		if cost > maxInt(budget, 0) || intFromAny(trace["selected_rows"], -1) != len(criticIdentityRowsForTest(t, ledger)) {
			t.Fatalf("actual index exceeds configured budget %d: cost %d, trace %#v", budget, cost, trace)
		}
		for _, row := range criticIdentityRowsForTest(t, ledger) {
			for _, original := range rows {
				if row["lifecycle_key"] == original["lifecycle_key"] && row["value"] != nil && row["value"] != original["value"] {
					t.Fatal("a value must remain whole or be omitted")
				}
			}
		}
		slices.Reverse(inputs)
		reordered, _ := run(budget)
		slices.Reverse(inputs)
		if mustCompactJSON(reordered) != mustCompactJSON(ledger) {
			t.Fatal("input order changed the bounded directory")
		}
	}
	if before != mustCompactJSON(inputs) {
		t.Fatal("public support mutated")
	}
}

func TestCriticIdentityIndexCountsSourceTurnsOnce(t *testing.T) {
	claim := map[string]any{"subject": "Mira", "state_slot": "rank", "lifecycle_key": "mira_rank", "value": "silver"}
	inputs := []map[string]any{{"id": 1, "turn_index": 1, "recorded_state_claims": []any{claim}}}
	run := func() string {
		_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira sits.", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 800})
		return mustCompactJSON([]any{ledger["existing_identity_index"], trace["identity_index_selection"]})
	}
	before := run()
	inputs[0]["recorded_pending_threads"] = []any{claim}
	inputs = append(inputs, inputs[0])
	if after := run(); before != after {
		t.Fatal("same-turn state/thread or duplicate memory inflated identity priority")
	}
}

func TestCriticIdentityIndexCompressionPreservesEveryKeyAndRecency(t *testing.T) {
	latest := map[string]map[string]any{}
	turns := map[string]map[int]bool{}
	for i := 1; i <= 24; i++ {
		key := fmt.Sprintf("synthetic_character_identity_%02d", i)
		latest[key] = map[string]any{"subject": "Mira", "lifecycle_key": key, "state_slot": "rank", "source_turn": i, "value": strings.Repeat("long exact historical context ", 8)}
		turns[key] = map[int]bool{i: true}
	}
	key := "synthetic_character_identity_01"
	latest[key]["value"] = "6 (azure / gold)"
	turns[key][0] = true
	index, trace := buildCompleteTurnCriticIdentityIndex(latest, turns, nil, "Mira", "Mira", nil, 800)
	rows := criticIdentityRowsForTest(t, map[string]any{"existing_identity_index": index})
	if len(rows) != len(latest) || len([]rune(mustCompactJSON(map[string]any{"existing_identity_index": index})))-len("null") > 800 {
		t.Fatalf("compression should retain the complete short key directory: %s", mustCompactJSON(index))
	}
	for i, row := range rows {
		expectedKey := fmt.Sprintf("synthetic_character_identity_%02d", len(latest)-i)
		if row["lifecycle_key"] != expectedKey || row["source_turn"] != latest[expectedKey]["source_turn"] {
			t.Fatal("lossless prefix sharing changed a key/source or replaced strict recent-source ordering")
		}
		if expectedKey == key && row["value"] != latest[key]["value"] {
			t.Fatal("reused key lost its exact prior context")
		}
		if expectedKey != key && row["value"] != nil {
			t.Fatal("single-observation context was not compressed before trimming")
		}
	}
	if !strings.Contains(mustCompactJSON(index), key+"|") || intFromAny(trace["omitted_values"], 0) != len(latest)-1 {
		t.Fatal("reused key must stay literal and context omission must be measured")
	}
	for _, field := range []string{"original_chars", "without_memory_id_chars", "without_defaults_chars", "compact_chars", "optional_context_chars", "oversize_context_chars"} {
		if intFromAny(trace[field], 0) == 0 {
			t.Fatalf("missing compression measurement: %s", field)
		}
	}
}

func TestCriticIdentityIndexLatestPublicSupport(t *testing.T) {
	old := map[string]any{"subject": "Mira", "state_slot": "rank", "lifecycle_key": "mira_rank", "value": "bronze knight northern oath"}
	newer := cloneMapAny(old)
	newer["state_slot"], newer["value"] = "knight_rank", "azure knight northern oath"
	inputs := []map[string]any{
		{"id": 9, "turn_index": 3, "recorded_state_claims": []any{newer}},
		{"id": 7, "turn_index": 1, "recorded_state_claims": []any{old}},
	}
	before := mustCompactJSON(inputs)
	for _, budget := range []int{800, 4000} {
		_, ledger, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "NAME: Mira | RESIDENCE: Harbor | AFFILIATION: Guild | rank: 9", completeTurnCriticInputPolicy{AuxiliaryMaxChars: budget})
		rows := criticIdentityRowsForTest(t, ledger)
		if len(rows) != 1 {
			t.Fatalf("lost existing identity at budget %d: %s", budget, mustCompactJSON(ledger))
		}
		row := rows[0]
		for field, expected := range newer {
			if row[field] != expected {
				t.Fatalf("latest %s = %v, want %v", field, row[field], expected)
			}
		}
		if _, exists := row["memory_id"]; exists {
			t.Fatal("unneeded memory ID retained")
		}
		if row["source_turn"] != 3 || len(criticReferenceCardsForTest(t, ledger)) != 0 {
			t.Fatal("source changed or structural supply became a full reference")
		}
		base := cloneMapAny(ledger)
		delete(base, "existing_identity_index")
		if len(base) == 0 {
			base = nil
		}
		chars := len([]rune(mustCompactJSON(ledger))) - len([]rune("null"))
		baseChars := len([]rune(mustCompactJSON(base))) - len([]rune("null"))
		if intFromAny(trace["identity_index_chars"], 0) != chars-baseChars || intFromAny(trace["total_support_chars"], 0) != chars || intFromAny(trace["auxiliary_selected_chars"], 0) != baseChars || baseChars > budget {
			t.Fatalf("structural cost hidden or charged to reference budget: %#v", trace)
		}
	}
	if mustCompactJSON(inputs) != before {
		t.Fatal("canonical support mutated")
	}
}

func TestCriticIdentityIndexDoesNotBindPropertiesOrSelectCards(t *testing.T) {
	ref := func(subject, key string) any {
		return map[string]any{"subject": subject, "state_slot": "rank", "lifecycle_key": key, "value": "azure knight northern oath"}
	}
	for _, query := range []string{
		"Mira eats supper. Harin rank improved.",
		"Mira eats supper while Harin rank improved.",
		"NAME: Mira | action: supper | NAME: Harin | rank: 9",
		"Mira rank improved. Harin rank improved. Harin rank improved. Harin rank improved.",
	} {
		for _, knownHarin := range []bool{false, true} {
			refs := []any{ref("Mira", "mira_rank")}
			if knownHarin {
				refs = append(refs, ref("Harin", "harin_rank"))
			}
			inputs := []map[string]any{{"id": 1, "turn_index": 1, "recorded_state_claims": refs}}
			_, ledger, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, query, completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4000})
			if len(criticReferenceCardsForTest(t, ledger)) != 0 {
				t.Fatal("another person's property newly selected a full reference card")
			}
			if len(criticIdentityRowsForTest(t, ledger)) != len(refs) {
				t.Fatal("name presence must supply identities without deciding the touched attribute")
			}
		}
	}
}

func TestCriticIdentityIndexNamePresenceAndPreviousTurn(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		want        int
	}{
		{"Mira", "Mirabel rank improved.", 0},
		{"Mira Vale", "Mira eats while Vale reads.", 0},
		{"Mira Vale", "NAME: Mira Vale | rank: 9", 1},
		{"최서준", "최서준는 책을 읽는다.", 1},
		{"최서준", "최서준성은 다른 사람이다.", 0},
		{"Mira", "", 0},
	} {
		latest := map[string]map[string]any{"rank_key": {"subject": tc.name, "state_slot": "rank", "lifecycle_key": "rank_key", "value": "azure knight northern oath", "source_turn": 1}}
		for _, previous := range []bool{false, true} {
			query := tc.query
			selectionQuery := query
			if previous {
				query = "Continues."
				selectionQuery = query + "\n" + tc.query
			}
			index, _ := buildCompleteTurnCriticIdentityIndex(latest, map[string]map[int]bool{"rank_key": {1: true}}, nil, query, selectionQuery, nil, 800)
			ledger := map[string]any{"existing_identity_index": index}
			if got := len(criticIdentityRowsForTest(t, ledger)); got != tc.want {
				t.Fatalf("subject %q query %q previous %v: got %d want %d", tc.name, tc.query, previous, got, tc.want)
			}
		}
	}
}

func TestCriticIdentityIndexPreservesScopesAndAvoidsDuplicateCards(t *testing.T) {
	public := map[string]any{"subject": "Mira", "state_slot": "rank", "lifecycle_key": "mira_rank", "value": "azure knight northern oath", "claim_scope": "objective"}
	belief := cloneMapAny(public)
	belief["claim_scope"], belief["perspective_owner"], belief["value"] = "belief", "Harin", "bronze knight northern oath"
	inputs := []map[string]any{{"id": 1, "turn_index": 1, "recorded_state_claims": []any{public, belief}}}
	_, small, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira rank", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 800})
	rows := criticIdentityRowsForTest(t, small)
	if len(rows) != 2 || rows[0]["value"] == rows[1]["value"] {
		t.Fatal("distinct perspectives collapsed")
	}
	_, full, trace := applyCompleteTurnCriticAuxiliaryBudget(nil, inputs, nil, nil, "Mira azure bronze knight northern oath", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4000})
	if len(criticReferenceCardsForTest(t, full)) != 2 || len(criticIdentityRowsForTest(t, full)) != 0 || intFromAny(trace["identity_index_chars"], 0) != 0 {
		t.Fatal("already supplied cards repeated in index")
	}
	if strings.Contains(mustCompactJSON(full), "existing_identity_index") {
		t.Fatal("empty index must not add input overhead")
	}
}

func TestCriticIdentityIndexPublicProjectionAndFrozenSnapshot(t *testing.T) {
	for _, marking := range []struct {
		field string
		value any
	}{
		{"visibility", "owner_private"}, {"visibility", "restricted"}, {"visibility", "user_private"},
		{"secret_guard", true}, {"privacy_guard", true}, {"sensitivity", "protected"}, {"sensitivity", "private"},
	} {
		t.Run(marking.field+"/"+extractionStringFromAny(marking.value), func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			const secretKey, secretValue = "private_rank_sentinel", "PRIVATE_VALUE_SENTINEL"
			public := map[string]any{"subject": "Mira", "state_slot": "rank", "lifecycle_key": "mira_rank", "value": "azure knight northern oath"}
			secret := map[string]any{"subject": "Mira", "state_slot": "secret_rank", "lifecycle_key": secretKey, "value": secretValue, marking.field: marking.value}
			raw := mustCompactJSON(map[string]any{"turn_summary": "Mira rank " + secretValue, "state_claims": []any{public, secret}, "protected_secrets": []any{map[string]any{"owner": "Mira", "summary": secretValue}}})
			st := &turnRecordingStore{
				returnMemories: []store.Memory{{ID: 1, TurnIndex: 1, SummaryJSON: raw}},
				returnChatLogs: []store.ChatLog{{ChatSessionID: "identity-index", TurnIndex: 1, Role: "user", Content: "Mira rank"}, {ChatSessionID: "identity-index", TurnIndex: 1, Role: "assistant", Content: "Mira rank azure"}},
			}
			cfg := config.Default()
			cfg.PromptDir = filepath.Join("..", "..", "..", "prompts")
			srv := &Server{Store: st, Cfg: cfg}
			policy := completeTurnCriticInputPolicy{Source: "identity-index-test", AuxiliaryMaxChars: 800}
			_, failure, err := srv.runCompleteTurnCriticWithInputPolicy(context.Background(), "identity-index", 2, "Mira rank", "Mira smiles.", nil, nil, completeTurnLLMConfig{}, true, policy, completeTurnCriticInputReplay{SourceRevision: "index-original"})
			if err == nil || failure["code"] != "CRITIC_CONFIG_MISSING" {
				t.Fatalf("must stop before provider access: %v %v", err, failure)
			}
			saved := st.savedCriticInputSnapshots["index-original"]
			var snapshot completeTurnCriticInputSnapshot
			if err := json.Unmarshal([]byte(saved.JSON), &snapshot); err != nil {
				t.Fatal(err)
			}
			rows := criticIdentityRowsForTest(t, snapshot.ArchiveLedger)
			if len(rows) != 1 || rows[0]["lifecycle_key"] != public["lifecycle_key"] || rows[0]["value"] != public["value"] {
				t.Fatalf("public identity absent or private identity supplied: %s", mustCompactJSON(snapshot.ArchiveLedger))
			}
			prompt := buildCompleteTurnCriticPromptWithLanguageContext("identity-index", 2, snapshot.UserInput, snapshot.AssistantContent, snapshot.ContextMessages, nil, snapshot.LanguageContext, snapshot.ArchiveLedger)
			for _, text := range []string{saved.JSON, prompt, mustCompactJSON(failure)} {
				if strings.Contains(text, secretKey) || strings.Contains(text, secretValue) {
					t.Fatal("private key/value escaped the public projection")
				}
			}
			// Replay must consume the frozen index even if the store now differs.
			st.returnMemories, st.returnChatLogs = nil, nil
			_, replayTrace, err := srv.runCompleteTurnCriticWithInputPolicy(context.Background(), "identity-index", 2, snapshot.UserInput, snapshot.AssistantContent, nil, nil, completeTurnLLMConfig{}, true, policy, completeTurnCriticInputReplay{Required: true, SourceRevision: "index-original", SnapshotJSON: saved.JSON, SnapshotHash: saved.Hash})
			if err == nil || replayTrace["code"] != "CRITIC_CONFIG_MISSING" || stringFromMap(mapFromAny(replayTrace["input_snapshot"]), "status") != "replayed" {
				t.Fatalf("frozen identity input did not replay: %v %v", err, replayTrace)
			}
		})
	}
}
