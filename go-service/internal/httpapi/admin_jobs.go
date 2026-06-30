package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type adminJobProgressFunc func(map[string]any)

type adminBackgroundJob struct {
	ID         string
	Kind       string
	SessionID  string
	Status     string
	Request    map[string]any
	Progress   map[string]any
	Result     map[string]any
	Error      string
	StartedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}

type adminJobManager struct {
	mu      sync.RWMutex
	nextID  uint64
	jobs    map[string]*adminBackgroundJob
	order   []string
	maxJobs int
}

func newAdminJobManager() *adminJobManager {
	return &adminJobManager{
		jobs:    map[string]*adminBackgroundJob{},
		order:   []string{},
		maxJobs: 80,
	}
}

func (m *adminJobManager) start(kind, sid string, request map[string]any, work func(context.Context, adminJobProgressFunc) (map[string]any, error)) map[string]any {
	if m == nil {
		m = newAdminJobManager()
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("%s-%d-%06d", strings.ToLower(strings.TrimSpace(kind)), now.UnixNano(), atomic.AddUint64(&m.nextID, 1))
	job := &adminBackgroundJob{
		ID:        id,
		Kind:      strings.TrimSpace(kind),
		SessionID: strings.TrimSpace(sid),
		Status:    "queued",
		Request:   sanitizeAdminJobRequest(request),
		Progress: map[string]any{
			"status":             "queued",
			"processed":          0,
			"candidate_count":    0,
			"failed_count":       0,
			"skipped_count":      0,
			"progress_percent":   0,
			"processed_turns":    []int{},
			"failed_turns":       []map[string]any{},
			"failed_ids":         []int64{},
			"last_processed":     nil,
			"foreground_timeout": false,
		},
		StartedAt: now,
		UpdatedAt: now,
	}
	m.mu.Lock()
	m.jobs[id] = job
	m.order = append(m.order, id)
	m.pruneLocked()
	m.mu.Unlock()

	go func() {
		m.update(id, "running", map[string]any{"status": "running", "started": true})
		result, err := work(context.Background(), func(progress map[string]any) {
			m.update(id, "running", progress)
		})
		if err != nil {
			m.finish(id, "failed", result, err.Error())
			return
		}
		m.finish(id, "completed", result, "")
	}()

	return job.snapshot()
}

func (m *adminJobManager) get(id string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[strings.TrimSpace(id)]
	if !ok || job == nil {
		return nil, false
	}
	return job.snapshot(), true
}

func (m *adminJobManager) list(limit int) []map[string]any {
	if m == nil {
		return []map[string]any{}
	}
	if limit <= 0 {
		limit = 20
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []map[string]any{}
	for i := len(m.order) - 1; i >= 0 && len(out) < limit; i-- {
		if job := m.jobs[m.order[i]]; job != nil {
			out = append(out, job.snapshot())
		}
	}
	return out
}

func (m *adminJobManager) update(id, status string, progress map[string]any) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return
	}
	if strings.TrimSpace(status) != "" {
		job.Status = strings.TrimSpace(status)
	}
	if job.Progress == nil {
		job.Progress = map[string]any{}
	}
	for k, v := range progress {
		job.Progress[k] = v
	}
	job.UpdatedAt = time.Now().UTC()
}

func (m *adminJobManager) finish(id, status string, result map[string]any, errText string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return
	}
	now := time.Now().UTC()
	job.Status = status
	job.UpdatedAt = now
	job.FinishedAt = &now
	job.Result = result
	job.Error = strings.TrimSpace(errText)
	if job.Progress == nil {
		job.Progress = map[string]any{}
	}
	job.Progress["status"] = status
	job.Progress["done"] = status == "completed"
	job.Progress["error"] = nilIfEmpty(job.Error)
	job.Progress["finished_at"] = now.Format(time.RFC3339)
	if _, ok := job.Progress["progress_percent"]; !ok {
		job.Progress["progress_percent"] = 100
	}
	if status == "completed" {
		job.Progress["progress_percent"] = 100
	}
}

func (m *adminJobManager) pruneLocked() {
	if m.maxJobs <= 0 {
		m.maxJobs = 80
	}
	if len(m.order) <= m.maxJobs {
		return
	}
	excess := len(m.order) - m.maxJobs
	for _, id := range m.order[:excess] {
		delete(m.jobs, id)
	}
	m.order = append([]string{}, m.order[excess:]...)
}

func (j *adminBackgroundJob) snapshot() map[string]any {
	if j == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"job_id":          j.ID,
		"kind":            j.Kind,
		"chat_session_id": j.SessionID,
		"status":          j.Status,
		"request":         cloneMapAny(j.Request),
		"progress":        cloneMapAny(j.Progress),
		"result":          cloneMapAny(j.Result),
		"error":           nilIfEmpty(j.Error),
		"started_at":      j.StartedAt.Format(time.RFC3339),
		"updated_at":      j.UpdatedAt.Format(time.RFC3339),
		"background":      true,
	}
	if j.FinishedAt != nil {
		out["finished_at"] = j.FinishedAt.Format(time.RFC3339)
	}
	return out
}

func cloneMapAny(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sanitizeAdminJobRequest(req map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range req {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "key") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") {
			out[k] = "[redacted]"
			continue
		}
		out[k] = v
	}
	return out
}

func adminJobProgressPercent(processed, total int) int {
	if total <= 0 {
		if processed > 0 {
			return 100
		}
		return 0
	}
	pct := processed * 100 / total
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func (s *Server) handleAdminJobs(w http.ResponseWriter, r *http.Request) {
	limit := intFromAny(r.URL.Query().Get("limit"), 20)
	jobs := []map[string]any{}
	if s.AdminJobs != nil {
		jobs = s.AdminJobs.list(limit)
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return strings.Compare(fmt.Sprint(jobs[i]["started_at"]), fmt.Sprint(jobs[j]["started_at"])) > 0
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"jobs":   jobs,
	})
}

func (s *Server) handleAdminJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("job_id"))
	if id == "" {
		writeBadRequest(w, "job_id is required")
		return
	}
	if s.AdminJobs == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "not_found", "job_id": id})
		return
	}
	job, ok := s.AdminJobs.get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "not_found", "job_id": id})
		return
	}
	writeJSON(w, http.StatusOK, job)
}
