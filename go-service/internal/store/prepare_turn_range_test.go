package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaPrepareTurnRangeQueriesBoundHistoryAndKeepExplicitOldRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectQuery(`(?s)SELECT COALESCE\(MAX\(turn_index\), 0\).*FROM chat_logs.*WHERE chat_session_id = \?`).
		WithArgs("range-session").
		WillReturnRows(sqlmock.NewRows([]string{"latest"}).AddRow(450))
	latest, err := st.LatestSessionTurnIndex(ctx, "range-session")
	if err != nil || latest != 450 {
		t.Fatalf("latest=%d err=%v", latest, err)
	}

	memoryRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "turn_index", "summary_json", "embedding", "embedding_model",
		"importance", "emotional_boost", "evidence", "emotional_intensity",
		"narrative_significance", "place_wing", "place_room", "created_at",
	}).AddRow(25, "range-session", 25, `{"turn_summary":"old vector memory"}`, nil, nil, 5.0, 0.0, nil, 0.0, 0.0, nil, nil, time.Now())
	mock.ExpectQuery(`(?s)FROM memories.*turn_index >= \?.*id IN \(\?\)`).
		WithArgs("range-session", 151, 151, 450, 450, int64(25)).
		WillReturnRows(memoryRows)
	memories, err := st.ListMemoriesRange(ctx, "range-session", 151, 450, []int64{25})
	if err != nil || len(memories) != 1 || memories[0].ID != 25 {
		t.Fatalf("memories=%#v err=%v", memories, err)
	}

	evidenceRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "evidence_kind", "evidence_text", "source_turn_start", "source_turn_end",
		"turn_anchor", "source_message_ids_json", "source_hash", "archive_state", "capture_stage",
		"capture_verification", "committed_gate", "lineage_json", "repair_needed", "tombstoned",
		"superseded_by_id", "created_at",
	})
	mock.ExpectQuery(`(?s)FROM direct_evidence_records.*tombstoned = FALSE.*COALESCE\(superseded_by_id, 0\) = 0.*GREATEST\(source_turn_start.*id IN \(\?\)`).
		WithArgs("range-session", 151, 151, 450, 450, int64(31)).
		WillReturnRows(evidenceRows)
	if _, err := st.ListEvidenceRange(ctx, "range-session", 151, 450, []int64{31}); err != nil {
		t.Fatalf("ListEvidenceRange: %v", err)
	}

	kgRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "subject", "predicate", "object", "valid_from", "valid_to", "source_turn", "created_at",
	})
	mock.ExpectQuery(`(?s)FROM kg_triples.*valid_to IS NULL.*source_turn >= \?`).
		WithArgs("range-session", 151, 151, 450, 450).
		WillReturnRows(kgRows)
	if _, err := st.ListKGTriplesRange(ctx, "range-session", 151, 450); err != nil {
		t.Fatalf("ListKGTriplesRange: %v", err)
	}

	characterRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "character_name", "appearance_json", "personality_json",
		"status_json", "relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
	})
	mock.ExpectQuery(`(?s)FROM character_states state.*COALESCE\(state.turn_index, 0\) < \?.*NOT EXISTS.*character_states newer.*COALESCE\(newer.turn_index, 0\) < \?`).
		WithArgs("range-session", 451, 451, 451, 451).
		WillReturnRows(characterRows)
	if _, err := st.ListCharacterStatesCurrentBefore(ctx, "range-session", 451); err != nil {
		t.Fatalf("ListCharacterStatesCurrentBefore: %v", err)
	}

	activeRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "state_type", "content", "turn_index", "created_at",
	})
	mock.ExpectQuery(`(?s)FROM active_states state.*state.turn_index >= \?.*NOT EXISTS.*active_states newer`).
		WithArgs("range-session", 151, 151, 450, 450).
		WillReturnRows(activeRows)
	if _, err := st.ListActiveStatesRange(ctx, "range-session", 151, 450); err != nil {
		t.Fatalf("ListActiveStatesRange: %v", err)
	}

	canonicalRows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "layer_type", "content", "source_state_type", "turn_index",
		"source_turn", "source_record", "last_verified_turn", "confidence", "created_at",
	})
	mock.ExpectQuery(`(?s)FROM canonical_state_layers layer.*layer.turn_index >= \?.*NOT EXISTS.*canonical_state_layers newer`).
		WithArgs("range-session", 151, 151, 450, 450).
		WillReturnRows(canonicalRows)
	if _, err := st.ListCanonicalStateLayersRange(ctx, "range-session", 151, 450); err != nil {
		t.Fatalf("ListCanonicalStateLayersRange: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
