package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type effectiveInputErrorStore struct {
	*turnRecordingStore
	inputErr, auditErr     error
	inputCalls, auditCalls int
}

func (f *effectiveInputErrorStore) SaveEffectiveInput(ctx context.Context, v *store.EffectiveInput) error {
	f.inputCalls++
	if f.inputErr != nil {
		return f.inputErr
	}
	return f.turnRecordingStore.SaveEffectiveInput(ctx, v)
}

func (f *effectiveInputErrorStore) SaveAuditLog(ctx context.Context, v *store.AuditLog) error {
	f.auditCalls++
	if f.auditErr != nil {
		return f.auditErr
	}
	return f.turnRecordingStore.SaveAuditLog(ctx, v)
}

func TestEffectiveInputFailurePreservesSafeCause(t *testing.T) {
	const secret = "fixture-private-credential"
	for _, stage := range []string{"input", "audit", "success"} {
		t.Run(stage, func(t *testing.T) {
			f := &effectiveInputErrorStore{turnRecordingStore: &turnRecordingStore{}}
			cause := errors.New("fixture database unavailable " + secret)
			if stage == "input" {
				f.inputErr = cause
			}
			if stage == "audit" {
				f.auditErr = cause
			}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeDualShadow
			s := NewServer(cfg)
			s.Store = f
			s.RuntimeConfig.MainAPIKey = secret
			mux := http.NewServeMux()
			s.RegisterRoutes(mux)
			var logs bytes.Buffer
			prior := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(prior)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/effective-inputs", strings.NewReader(`{"chat_session_id":"fixture-error-session","turn_index":5,"effective_input":"fixture assembled text"}`))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(rec, req)
			var result map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 200 || f.inputCalls != 1 {
				t.Fatalf("write flow changed: %d %#v", rec.Code, result)
			}
			if strings.Contains(rec.Body.String()+logs.String(), secret) {
				t.Fatal("credential leaked")
			}
			if stage == "success" {
				if result["save_ok"] != true || result["save_error"] != "" || logs.Len() != 0 {
					t.Fatalf("success changed: %#v %s", result, &logs)
				}
				return
			}
			for _, text := range []string{rec.Body.String(), logs.String()} {
				if !strings.Contains(text, "fixture database unavailable") || strings.Contains(text, "shadow_mode: save disabled") {
					t.Fatalf("cause lost/mislabeled: %s", text)
				}
			}
			details, ok := result["store_write_error_details"].([]any)
			if !ok || len(details) != 1 {
				t.Fatalf("missing failure details: %#v", result)
			}
			wantSaved := stage == "audit"
			if result["save_ok"] != wantSaved {
				t.Fatalf("effective input success changed: %#v", result)
			}
			if stage == "input" && f.auditCalls != 0 {
				t.Fatal("audit attempted after input failed")
			}
			if stage == "audit" && f.auditCalls != 1 {
				t.Fatal("audit call changed")
			}
		})
	}
}

type workerReadErrorStore struct {
	*memoryReprocessingDrainStore
	cause                           error
	claims, vectorClaims, schedules int
}

func (f *workerReadErrorStore) ClaimMemoryReprocessingJob(context.Context, string, time.Time, time.Duration) (*store.MemoryReprocessingJob, error) {
	f.claims++
	if f.cause != nil {
		return nil, f.cause
	}
	return nil, store.ErrNotFound
}

func (f *workerReadErrorStore) ClaimMemoryVectorOperations(context.Context, string, time.Time, time.Duration) ([]*store.MemoryVectorOutboxItem, error) {
	f.vectorClaims++
	if f.cause != nil {
		return nil, f.cause
	}
	return nil, store.ErrNotFound
}

func (f *workerReadErrorStore) NextMemoryReprocessingWakeAt(context.Context) (time.Time, error) {
	f.schedules++
	return time.Time{}, f.cause
}

func TestMemoryWorkerReadFailurePreservesSafeCause(t *testing.T) {
	const secret = "fixture-worker-credential"
	for _, failing := range []bool{true, false} {
		f := &workerReadErrorStore{memoryReprocessingDrainStore: &memoryReprocessingDrainStore{memoryAdmissionWorkerStore: &memoryAdmissionWorkerStore{Store: store.NewNoopStore()}}}
		if failing {
			f.cause = errors.New("fixture DB read failure " + secret)
		}
		s := &Server{Store: f, RuntimeConfig: RuntimeConfig{Synced: true, CriticTimeoutSec: 2, MainAPIKey: secret}}
		var logs bytes.Buffer
		prior := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
		next := s.processMemoryWorkerWake(context.Background(), "fixture-worker", time.Now())
		slog.SetDefault(prior)
		if !next.IsZero() || f.claims != 1 || f.schedules != 1 {
			t.Fatalf("worker scheduling changed: %v %+v", next, f)
		}
		if strings.Contains(logs.String(), secret) {
			t.Fatal("worker credential leaked")
		}
		if !failing {
			if logs.Len() != 0 || f.vectorClaims != 2 {
				t.Fatalf("idle worker treated as failure: %s", &logs)
			}
			continue
		}
		if f.vectorClaims != 1 {
			t.Fatal("extra vector attempt added")
		}
		for _, operation := range []string{"memory_reprocessing", "vector_outbox", "wake_schedule"} {
			if !strings.Contains(logs.String(), `"operation":"`+operation+`"`) {
				t.Fatalf("operation failure lost: %s", &logs)
			}
		}
		if strings.Count(logs.String(), "fixture DB read failure") != 3 {
			t.Fatalf("original errors lost: %s", &logs)
		}
	}
}
