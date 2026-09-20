package store

import "encoding/json"

// StateRepairContract identifies an explicit source-grounded correction. A
// correction retains its actual cause turn while separately recording when the
// correction entered the current projection.
const StateRepairContract = "state_repair.v1"

func statusObservationTurn(evidenceJSON string, sourceTurn int) int {
	var evidence struct {
		RepairRecordedTurn int `json:"repair_recorded_turn"`
	}
	if json.Unmarshal([]byte(evidenceJSON), &evidence) == nil && evidence.RepairRecordedTurn > 0 {
		return evidence.RepairRecordedTurn
	}
	return sourceTurn
}

// StatusCurrentObservationTurn is the existing current projection order, with
// explicit corrections ordered by their recording turn instead of backdating
// their authority to the historical cause turn.
func StatusCurrentObservationTurn(value StatusCurrentValue) int {
	return statusObservationTurn(value.EvidenceJSON, value.SourceTurn)
}

func StatusChangeEventObservationTurn(event StatusChangeEvent) int {
	return statusObservationTurn(event.EvidenceJSON, event.SourceTurn)
}

func statusObservationTurnSQL(alias string) string {
	return "COALESCE(NULLIF(GREATEST(CAST(JSON_UNQUOTE(JSON_EXTRACT(" + alias + ".evidence_json, '$.repair_recorded_turn')) AS SIGNED), 0), 0), " + alias + ".source_turn, 0)"
}

// The reversible owner continues to read accepted sources. Explicit repairs of
// older imports also participate without inventing an accepted source revision.
func statusProjectionSourceSQL(alias, revisionAlias string) string {
	return "(" + revisionAlias + ".lifecycle_state = 'active' OR (COALESCE(JSON_UNQUOTE(JSON_EXTRACT(" + alias + ".evidence_json, '$.source_revision')), '') = '' AND JSON_UNQUOTE(JSON_EXTRACT(" + alias + ".evidence_json, '$.source_contract')) = '" + StateRepairContract + "'))"
}
