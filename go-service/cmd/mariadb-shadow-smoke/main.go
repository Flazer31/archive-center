package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type smokeReport struct {
	Status        string            `json:"status"`
	CheckedAt     string            `json:"checked_at"`
	WriteEnabled  bool              `json:"write_enabled"`
	SessionID     string            `json:"session_id"`
	TurnIndex     int               `json:"turn_index"`
	DSNConfigured bool              `json:"dsn_configured"`
	PingOK        bool              `json:"ping_ok"`
	Writes        map[string]string `json:"writes,omitempty"`
	Reads         map[string]any    `json:"reads,omitempty"`
	Error         string            `json:"error,omitempty"`
	Note          string            `json:"note"`
}

func main() {
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	sessionID := flag.String("session", fmt.Sprintf("mariadb-shadow-smoke-%d", time.Now().UTC().UnixNano()), "Smoke session id.")
	turnIndex := flag.Int("turn", int(time.Now().UTC().Unix()%1000000000), "Smoke turn index.")
	write := flag.Bool("write", false, "Actually insert smoke rows into MariaDB shadow tables.")
	timeout := flag.Duration("timeout", 10*time.Second, "Smoke timeout.")
	flag.Parse()

	report := smokeReport{
		Status:        "failed",
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		WriteEnabled:  *write,
		SessionID:     *sessionID,
		TurnIndex:     *turnIndex,
		DSNConfigured: *dsn != "",
		Writes:        map[string]string{},
		Reads:         map[string]any{},
		Note:          "R1 MariaDB shadow smoke only; not an authority switch.",
	}

	if *dsn == "" {
		report.Error = "missing dsn: provide -dsn or AC_MARIADB_DSN"
		writeReport(report)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	st, err := store.OpenMariaDB(*dsn)
	if err != nil {
		report.Error = err.Error()
		writeReport(report)
		os.Exit(1)
	}

	if pinger, ok := st.(store.Pinger); ok {
		if err := pinger.Ping(ctx); err != nil {
			report.Error = fmt.Sprintf("ping failed: %v", err)
			writeReport(report)
			os.Exit(1)
		}
		report.PingOK = true
	}

	if !*write {
		report.Status = "ok"
		report.Note = "MariaDB shadow target ping passed. Re-run with -write to insert and read smoke rows."
		writeReport(report)
		return
	}

	now := time.Now().UTC()
	if err := st.SaveChatLog(ctx, &store.ChatLog{
		ChatSessionID: *sessionID,
		TurnIndex:     *turnIndex,
		Role:          "user",
		Content:       "mariadb shadow smoke user row",
		CreatedAt:     now,
	}); err != nil {
		report.Error = fmt.Sprintf("save user chat log: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["chat_logs.user"] = "ok"

	if err := st.SaveChatLog(ctx, &store.ChatLog{
		ChatSessionID: *sessionID,
		TurnIndex:     *turnIndex,
		Role:          "assistant",
		Content:       "mariadb shadow smoke assistant row",
		CreatedAt:     now,
	}); err != nil {
		report.Error = fmt.Sprintf("save assistant chat log: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["chat_logs.assistant"] = "ok"

	if err := st.SaveEffectiveInput(ctx, &store.EffectiveInput{
		ChatSessionID:  *sessionID,
		TurnIndex:      *turnIndex,
		EffectiveInput: "mariadb shadow smoke effective input",
		CreatedAt:      now,
	}); err != nil {
		report.Error = fmt.Sprintf("save effective input: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["effective_input_logs"] = "ok"

	if err := st.SaveMemory(ctx, &store.Memory{
		ChatSessionID:         *sessionID,
		TurnIndex:             *turnIndex,
		SummaryJSON:           `{"summary":"mariadb shadow smoke memory"}`,
		Embedding:             `[0.1,0.2,0.3]`,
		EmbeddingModel:        "smoke-model",
		Importance:            0.5,
		EmotionalBoost:        0.1,
		Evidence:              `{"source":"smoke"}`,
		EmotionalIntensity:    0.2,
		NarrativeSignificance: 0.3,
		PlaceWing:             "smoke-wing",
		PlaceRoom:             "smoke-room",
		CreatedAt:             now,
	}); err != nil {
		report.Error = fmt.Sprintf("save memory: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["memories"] = "ok"

	if err := st.SaveEvidence(ctx, &store.DirectEvidence{
		ChatSessionID:        *sessionID,
		EvidenceKind:         "fact_event",
		EvidenceText:         "mariadb shadow smoke direct evidence",
		SourceTurnStart:      *turnIndex,
		SourceTurnEnd:        *turnIndex,
		TurnAnchor:           *turnIndex,
		SourceMessageIDsJSON: `["smoke-message"]`,
		SourceHash:           "smoke-hash",
		ArchiveState:         "committed",
		CaptureStage:         "smoke",
		CaptureVerification:  "verified",
		CommittedGate:        "shadow_smoke",
		LineageJSON:          `{"smoke":true}`,
		CreatedAt:            now,
	}); err != nil {
		report.Error = fmt.Sprintf("save direct evidence: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["direct_evidence_records"] = "ok"

	if err := st.SaveKGTriple(ctx, &store.KGTriple{
		ChatSessionID: *sessionID,
		Subject:       "smoke-subject",
		Predicate:     "smoke-predicate",
		Object:        "smoke-object",
		ValidFrom:     *turnIndex,
		SourceTurn:    *turnIndex,
		CreatedAt:     now,
	}); err != nil {
		report.Error = fmt.Sprintf("save kg triple: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["kg_triples"] = "ok"

	if err := st.SaveAuditLog(ctx, &store.AuditLog{
		CreatedAt:     now,
		EventType:     "shadow_smoke",
		ChatSessionID: *sessionID,
		TargetType:    "session",
		Summary:       "mariadb shadow smoke audit",
		DetailsJSON:   `{"smoke":true}`,
		Source:        "smoke",
	}); err != nil {
		report.Error = fmt.Sprintf("save audit log: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["audit_logs"] = "ok"

	if err := st.SaveCriticFeedback(ctx, &store.CriticFeedback{
		CreatedAt:     now,
		ChatSessionID: *sessionID,
		TargetType:    "memory",
		TargetID:      1,
		FeedbackValue: "accept",
		FeedbackNote:  "mariadb shadow smoke feedback",
		Source:        "smoke",
	}); err != nil {
		report.Error = fmt.Sprintf("save critic feedback: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["critic_feedback"] = "ok"

	if err := st.SaveCharacterEvent(ctx, &store.CharacterEvent{
		ChatSessionID: *sessionID,
		CharacterName: "smoke-character",
		TurnIndex:     *turnIndex,
		EventType:     "shadow_smoke",
		DetailsJSON:   `{"smoke":true}`,
		CreatedAt:     now,
	}); err != nil {
		report.Error = fmt.Sprintf("save character event: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Writes["character_events"] = "ok"

	logs, err := st.ListChatLogs(ctx, *sessionID, *turnIndex, *turnIndex)
	if err != nil {
		report.Error = fmt.Sprintf("list chat logs: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["chat_logs_count"] = len(logs)

	effective, err := st.GetEffectiveInput(ctx, *sessionID, *turnIndex)
	if err != nil {
		report.Error = fmt.Sprintf("get effective input: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["effective_input_found"] = effective != nil

	memories, err := st.ListMemories(ctx, *sessionID, *turnIndex, *turnIndex)
	if err != nil {
		report.Error = fmt.Sprintf("list memories: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["memories_count"] = len(memories)

	evidence, err := st.ListEvidence(ctx, *sessionID)
	if err != nil {
		report.Error = fmt.Sprintf("list direct evidence: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["direct_evidence_count"] = len(evidence)

	triples, err := st.ListKGTriples(ctx, *sessionID)
	if err != nil {
		report.Error = fmt.Sprintf("list kg triples: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["kg_triples_count"] = len(triples)

	audits, err := st.ListAuditLogs(ctx, *sessionID, "shadow_smoke", 10)
	if err != nil {
		report.Error = fmt.Sprintf("list audit logs: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["audit_logs_count"] = len(audits)

	feedback, err := st.ListCriticFeedback(ctx, *sessionID, "memory", 1)
	if err != nil {
		report.Error = fmt.Sprintf("list critic feedback: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["critic_feedback_count"] = len(feedback)

	characters, err := st.ListCharacterEvents(ctx, *sessionID, "smoke-character")
	if err != nil {
		report.Error = fmt.Sprintf("list character events: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["character_events_count"] = len(characters)

	stats, err := st.Stats(ctx)
	if err != nil {
		report.Error = fmt.Sprintf("stats: %v", err)
		writeReport(report)
		os.Exit(1)
	}
	report.Reads["stats"] = stats

	report.Status = "ok"
	writeReport(report)
}

func writeReport(report smokeReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
