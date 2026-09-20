package store

import "testing"

func TestStateRepairObservationOrderSeparatesCauseFromRecording46(t *testing.T) {
	for _, tt := range []struct {
		name, evidence string
		cause, want    int
	}{
		{"ordinary", `{"source_revision":"source"}`, 3, 3},
		{"explicit repair", `{"repair_recorded_turn":9}`, 3, 9},
		{"unknown recording", `{"repair_recorded_turn":0}`, 3, 3},
		{"negative recording", `{"repair_recorded_turn":-1}`, 3, 3},
		{"unknown legacy cause", `{}`, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			current := StatusCurrentValue{SourceTurn: tt.cause, EvidenceJSON: tt.evidence}
			event := StatusChangeEvent{SourceTurn: tt.cause, EvidenceJSON: tt.evidence}
			if got := StatusCurrentObservationTurn(current); got != tt.want {
				t.Fatalf("current observation = %d, want %d", got, tt.want)
			}
			if got := StatusChangeEventObservationTurn(event); got != tt.want {
				t.Fatalf("event observation = %d, want %d", got, tt.want)
			}
			if current.SourceTurn != tt.cause || event.SourceTurn != tt.cause {
				t.Fatal("reading observation changed historical cause")
			}
		})
	}
}
