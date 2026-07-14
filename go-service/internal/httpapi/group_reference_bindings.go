package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const referenceBindingContractVersion = "reference_binding.v1"

type referenceBindingRequest struct {
	WorkID              string `json:"work_id"`
	ContinuityID        string `json:"continuity_id"`
	BindingRole         string `json:"binding_role"`
	Enabled             *bool  `json:"enabled,omitempty"`
	InjectionEnabled    *bool  `json:"injection_enabled,omitempty"`
	AnchorMode          string `json:"anchor_mode"`
	CurrentNodeID       string `json:"current_node_id"`
	RevealCeilingNodeID string `json:"reveal_ceiling_node_id"`
	DivergenceNodeID    string `json:"divergence_node_id"`
	FuturePolicy        string `json:"future_policy"`
	Priority            int    `json:"priority"`
	ExpectedRevision    int64  `json:"expected_revision"`
}

type referenceBindingValidation struct {
	Candidate      store.SessionReferenceBinding
	Existing       *store.SessionReferenceBinding
	Work           *store.ReferenceWork
	Continuity     *store.ReferenceContinuity
	CurrentNode    *store.ReferenceTimelineNode
	RevealNode     *store.ReferenceTimelineNode
	DivergenceNode *store.ReferenceTimelineNode
	Action         string
	BlockedReasons []string
	Warnings       []string
}

func (s *Server) handleSessionReferenceBindingsList(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	bindings, err := ref.ListSessionReferenceBindings(r.Context(), sid, false)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	runtimes := map[string]store.SessionReferenceRuntime{}
	for _, binding := range bindings {
		runtime, runtimeErr := ref.GetSessionReferenceRuntime(r.Context(), binding.BindingID)
		if errors.Is(runtimeErr, store.ErrNotFound) {
			continue
		}
		if runtimeErr != nil {
			writeReferenceStoreError(w, runtimeErr)
			return
		}
		if runtime != nil {
			runtimes[binding.BindingID] = *runtime
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": referenceBindingContractVersion,
		"chat_session_id":  sid,
		"bindings":         bindings,
		"runtimes":         runtimes,
	})
}

func (s *Server) handleSessionReferenceBindingPreview(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceBindingRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	validation, err := validateReferenceBinding(r.Context(), ref, strings.TrimSpace(r.PathValue("chat_session_id")), "", req)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, referenceBindingPreviewResponse(validation))
}

func (s *Server) handleSessionReferenceBindingApply(w http.ResponseWriter, r *http.Request) {
	s.handleSessionReferenceBindingMutation(w, r, "")
}

func (s *Server) handleSessionReferenceBindingUpdate(w http.ResponseWriter, r *http.Request) {
	s.handleSessionReferenceBindingMutation(w, r, strings.TrimSpace(r.PathValue("binding_id")))
}

func (s *Server) handleSessionReferenceBindingMutation(w http.ResponseWriter, r *http.Request, bindingID string) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	var req referenceBindingRequest
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	validation, err := validateReferenceBinding(r.Context(), ref, sid, bindingID, req)
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	preview := referenceBindingPreviewResponse(validation)
	if len(validation.BlockedReasons) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":  "blocked",
			"code":    "reference_binding_blocked",
			"preview": preview,
		})
		return
	}
	expectedRevision := int64(0)
	if validation.Existing != nil {
		expectedRevision = req.ExpectedRevision
		if expectedRevision < 1 || expectedRevision != validation.Existing.Revision {
			writeJSON(w, http.StatusConflict, map[string]any{
				"status":            "failed",
				"code":              "reference_binding_revision_conflict",
				"expected_revision": validation.Existing.Revision,
				"preview":           preview,
			})
			return
		}
	} else if req.ExpectedRevision != 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":  "failed",
			"code":    "reference_binding_revision_conflict",
			"preview": preview,
		})
		return
	}
	if err := ref.UpsertSessionReferenceBinding(r.Context(), &validation.Candidate, expectedRevision); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	persisted, err := findSessionReferenceBinding(r.Context(), ref, sid, validation.Candidate.BindingID, "", "")
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": referenceBindingContractVersion,
		"action":           validation.Action,
		"binding":          persisted,
		"preview":          preview,
	})
}

func (s *Server) handleSessionReferenceBindingDelete(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.referenceLibraryStore(w)
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	bindingID := strings.TrimSpace(r.PathValue("binding_id"))
	if sid == "" || bindingID == "" {
		writeBadRequest(w, "chat_session_id and binding_id are required")
		return
	}
	existing, err := findSessionReferenceBinding(r.Context(), ref, sid, bindingID, "", "")
	if err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("expected_revision")); raw != "" {
		expectedRevision, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || expectedRevision != existing.Revision {
			writeJSON(w, http.StatusConflict, map[string]any{
				"status":            "failed",
				"code":              "reference_binding_revision_conflict",
				"expected_revision": existing.Revision,
			})
			return
		}
	}
	if err := ref.DeleteSessionReferenceBinding(r.Context(), sid, bindingID); err != nil {
		writeReferenceStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": referenceBindingContractVersion,
		"action":           "unlinked",
		"binding_id":       bindingID,
		"chat_session_id":  sid,
	})
}

func validateReferenceBinding(ctx context.Context, ref store.ReferenceLibraryStore, sid, bindingID string, req referenceBindingRequest) (referenceBindingValidation, error) {
	result := referenceBindingValidation{Action: "create", BlockedReasons: []string{}, Warnings: []string{}}
	sid = strings.TrimSpace(sid)
	bindingID = strings.TrimSpace(bindingID)
	workID := strings.TrimSpace(req.WorkID)
	continuityID := strings.TrimSpace(req.ContinuityID)
	if sid == "" {
		result.BlockedReasons = append(result.BlockedReasons, "chat_session_id_required")
	}
	if workID == "" {
		result.BlockedReasons = append(result.BlockedReasons, "work_id_required")
	}
	if continuityID == "" {
		result.BlockedReasons = append(result.BlockedReasons, "continuity_id_required")
	}

	bindingRole := referenceBindingEnum(req.BindingRole, "primary", map[string]bool{"primary": true, "crossover": true, "reference_only": true}, "binding_role_invalid", &result.BlockedReasons)
	anchorMode := referenceBindingEnum(req.AnchorMode, "manual", map[string]bool{"manual": true, "assisted": true}, "anchor_mode_invalid", &result.BlockedReasons)
	futurePolicy := referenceBindingEnum(req.FuturePolicy, "block", map[string]bool{"block": true, "preview_only": true}, "future_policy_invalid", &result.BlockedReasons)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	injectionEnabled := false
	if req.InjectionEnabled != nil {
		injectionEnabled = *req.InjectionEnabled
	}

	if workID != "" {
		work, err := ref.GetReferenceWork(ctx, workID)
		if errors.Is(err, store.ErrNotFound) {
			result.BlockedReasons = append(result.BlockedReasons, "work_not_found")
		} else if err != nil {
			return result, err
		} else {
			result.Work = work
			if strings.EqualFold(strings.TrimSpace(work.Status), "disabled") {
				result.BlockedReasons = append(result.BlockedReasons, "work_disabled")
			}
		}
	}

	if workID != "" && continuityID != "" {
		continuities, err := ref.ListReferenceContinuities(ctx, workID)
		if err != nil {
			return result, err
		}
		for i := range continuities {
			if continuities[i].ContinuityID == continuityID {
				copy := continuities[i]
				result.Continuity = &copy
				break
			}
		}
		if result.Continuity == nil {
			result.BlockedReasons = append(result.BlockedReasons, "continuity_not_in_work")
		} else if !strings.EqualFold(strings.TrimSpace(result.Continuity.Status), "active") {
			result.BlockedReasons = append(result.BlockedReasons, "continuity_not_active")
		}
	}

	if sid != "" {
		existing, err := findSessionReferenceBinding(ctx, ref, sid, bindingID, workID, continuityID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return result, err
		}
		if err == nil {
			result.Existing = existing
			result.Action = "update"
			bindingID = existing.BindingID
			if req.InjectionEnabled == nil {
				injectionEnabled = existing.InjectionEnabled
			}
		} else if bindingID != "" {
			result.BlockedReasons = append(result.BlockedReasons, "binding_not_found")
		}
	}
	if bindingID == "" && sid != "" && workID != "" && continuityID != "" {
		bindingID = referenceStableID("binding", sid, workID, continuityID)
	}

	if workID != "" && continuityID != "" && result.Continuity != nil {
		timeline, err := ref.ListReferenceTimelineNodes(ctx, workID, continuityID, "")
		if err != nil {
			return result, err
		}
		nodes := map[string]*store.ReferenceTimelineNode{}
		for i := range timeline {
			if !strings.EqualFold(strings.TrimSpace(timeline[i].ReviewStatus), "approved") {
				continue
			}
			copy := timeline[i]
			nodes[copy.NodeID] = &copy
		}
		result.CurrentNode = referenceBindingNode(nodes, strings.TrimSpace(req.CurrentNodeID), "current_node_not_approved", &result.BlockedReasons)
		result.RevealNode = referenceBindingNode(nodes, strings.TrimSpace(req.RevealCeilingNodeID), "reveal_ceiling_node_not_approved", &result.BlockedReasons)
		result.DivergenceNode = referenceBindingNode(nodes, strings.TrimSpace(req.DivergenceNodeID), "divergence_node_not_approved", &result.BlockedReasons)
		referenceBindingValidateNodeOrder(&result)
	}

	if strings.TrimSpace(req.CurrentNodeID) == "" {
		result.Warnings = append(result.Warnings, "current_node_unknown")
	}
	if strings.TrimSpace(req.RevealCeilingNodeID) == "" {
		result.Warnings = append(result.Warnings, "reveal_ceiling_unknown")
	}
	if strings.TrimSpace(req.DivergenceNodeID) != "" && strings.TrimSpace(req.CurrentNodeID) == "" {
		result.BlockedReasons = append(result.BlockedReasons, "divergence_requires_current_node")
	}

	result.Candidate = store.SessionReferenceBinding{
		BindingID:           bindingID,
		ChatSessionID:       sid,
		WorkID:              workID,
		ContinuityID:        continuityID,
		BindingRole:         bindingRole,
		Enabled:             enabled,
		InjectionEnabled:    injectionEnabled,
		AnchorMode:          anchorMode,
		CurrentNodeID:       strings.TrimSpace(req.CurrentNodeID),
		RevealCeilingNodeID: strings.TrimSpace(req.RevealCeilingNodeID),
		DivergenceNodeID:    strings.TrimSpace(req.DivergenceNodeID),
		FuturePolicy:        futurePolicy,
		Priority:            req.Priority,
	}
	if result.Existing != nil {
		result.Candidate.Revision = result.Existing.Revision
	}
	return result, nil
}

func referenceBindingEnum(value, fallback string, allowed map[string]bool, blockedCode string, blocked *[]string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	if !allowed[value] {
		*blocked = append(*blocked, blockedCode)
		return fallback
	}
	return value
}

func referenceBindingNode(nodes map[string]*store.ReferenceTimelineNode, nodeID, blockedCode string, blocked *[]string) *store.ReferenceTimelineNode {
	if nodeID == "" {
		return nil
	}
	if node := nodes[nodeID]; node != nil {
		return node
	}
	*blocked = append(*blocked, blockedCode)
	return nil
}

func referenceBindingValidateNodeOrder(result *referenceBindingValidation) {
	selected := []*store.ReferenceTimelineNode{result.CurrentNode, result.RevealNode, result.DivergenceNode}
	branch := ""
	for _, node := range selected {
		if node == nil {
			continue
		}
		nodeBranch := strings.TrimSpace(node.BranchKey)
		if nodeBranch == "" {
			nodeBranch = "main"
		}
		if branch == "" {
			branch = nodeBranch
			continue
		}
		if branch != nodeBranch {
			result.BlockedReasons = append(result.BlockedReasons, "selected_nodes_cross_branch")
			break
		}
	}
	if result.CurrentNode != nil && result.RevealNode != nil && result.RevealNode.Ordinal < result.CurrentNode.Ordinal {
		result.BlockedReasons = append(result.BlockedReasons, "reveal_ceiling_before_current")
	}
	if result.CurrentNode != nil && result.DivergenceNode != nil && result.DivergenceNode.Ordinal > result.CurrentNode.Ordinal {
		result.BlockedReasons = append(result.BlockedReasons, "divergence_after_current")
	}
}

func findSessionReferenceBinding(ctx context.Context, ref store.ReferenceLibraryStore, sid, bindingID, workID, continuityID string) (*store.SessionReferenceBinding, error) {
	items, err := ref.ListSessionReferenceBindings(ctx, sid, false)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if bindingID != "" && items[i].BindingID == bindingID {
			copy := items[i]
			return &copy, nil
		}
		if bindingID == "" && workID != "" && continuityID != "" && items[i].WorkID == workID && items[i].ContinuityID == continuityID {
			copy := items[i]
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func referenceBindingPreviewResponse(validation referenceBindingValidation) map[string]any {
	return map[string]any{
		"status":           "ok",
		"contract_version": referenceBindingContractVersion,
		"valid":            len(validation.BlockedReasons) == 0,
		"action":           validation.Action,
		"blocked_reasons":  validation.BlockedReasons,
		"warnings":         validation.Warnings,
		"binding":          validation.Candidate,
		"existing_binding": validation.Existing,
		"selection": map[string]any{
			"work":                validation.Work,
			"continuity":          validation.Continuity,
			"current_node":        validation.CurrentNode,
			"reveal_ceiling_node": validation.RevealNode,
			"divergence_node":     validation.DivergenceNode,
		},
	}
}
