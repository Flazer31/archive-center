package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const outputFidelityCorpusPath = "testdata/output_fidelity_corpus.v1.json"

type outputFidelityCorpus struct {
	ContractVersion           string                       `json:"contract_version"`
	Status                    string                       `json:"status"`
	OwnerPacket               string                       `json:"owner_packet"`
	ProductionBehaviorChanged bool                         `json:"production_behavior_changed"`
	JudgementContract         outputFidelityJudgement      `json:"judgement_contract"`
	GuideMatrix               []string                     `json:"guide_matrix"`
	RequiredLiveResultFields  []string                     `json:"required_live_result_fields"`
	SourceAnnotationContract  outputFidelitySourceContract `json:"source_annotation_contract"`
	CriteriaCatalog           []outputFidelityCriterion    `json:"criteria_catalog"`
	Cases                     []outputFidelityCase         `json:"cases"`
}

type outputFidelityJudgement struct {
	AggregateScoreAllowed     bool     `json:"aggregate_score_allowed"`
	ExactTextMatchRequired    bool     `json:"exact_text_match_required"`
	Outcomes                  []string `json:"outcomes"`
	RequiredObservationStages []string `json:"required_observation_stages"`
	ModelPolicy               string   `json:"model_policy"`
}

type outputFidelityCriterion struct {
	CriterionID string `json:"criterion_id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

type outputFidelitySourceContract struct {
	SourceKinds     []string `json:"source_kinds"`
	LifecycleStates []string `json:"lifecycle_states"`
}

type outputFidelityCase struct {
	CaseID               string                      `json:"case_id"`
	Class                string                      `json:"class"`
	Language             string                      `json:"language"`
	GenreDimension       string                      `json:"genre_dimension"`
	ContinuityKind       string                      `json:"continuity_kind"`
	CounterfactualPairID string                      `json:"counterfactual_pair_id"`
	EvidenceSources      []outputFidelitySource      `json:"evidence_sources"`
	ExpectedFulfillments []outputFidelityExpectation `json:"expected_fulfillments"`
	ForbiddenViolations  []outputFidelityViolation   `json:"forbidden_violations"`
	StageRecordTemplate  map[string]string           `json:"stage_record_template"`
}

type outputFidelitySource struct {
	SourceID        string   `json:"source_id"`
	SourceKind      string   `json:"source_kind"`
	SourceLifecycle string   `json:"source_lifecycle"`
	SourceSpan      string   `json:"source_span"`
	ContentSummary  string   `json:"content_summary"`
	ReplayText      string   `json:"replay_text"`
	KnowledgeScope  []string `json:"knowledge_scope"`
}

type outputFidelityExpectation struct {
	CriterionID     string   `json:"criterion_id"`
	ExpectedOutcome string   `json:"expected_outcome"`
	Description     string   `json:"description"`
	SourceRefs      []string `json:"source_refs"`
}

type outputFidelityViolation struct {
	CriterionID      string   `json:"criterion_id"`
	ViolationOutcome string   `json:"violation_outcome"`
	Description      string   `json:"description"`
	SourceRefs       []string `json:"source_refs"`
}

func loadOutputFidelityCorpus(t *testing.T) (outputFidelityCorpus, any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(outputFidelityCorpusPath))
	if err != nil {
		t.Fatalf("read %s: %v", outputFidelityCorpusPath, err)
	}

	var corpus outputFidelityCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode typed corpus: %v", err)
	}

	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decode generic corpus: %v", err)
	}
	return corpus, generic
}

func stringSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}

func requireSetContains(set map[string]struct{}, required []string, label string) error {
	for _, item := range required {
		if _, ok := set[item]; !ok {
			return fmt.Errorf("%s missing %q", label, item)
		}
	}
	return nil
}

func validateOutputFidelityCorpus(corpus outputFidelityCorpus) error {
	if corpus.ContractVersion != "archive_center.output_fidelity_corpus.v1" {
		return fmt.Errorf("contract_version=%q", corpus.ContractVersion)
	}
	if corpus.Status != "preparatory" {
		return fmt.Errorf("status=%q, want preparatory", corpus.Status)
	}
	if corpus.OwnerPacket != "3.5-A" {
		return fmt.Errorf("owner_packet=%q, want 3.5-A", corpus.OwnerPacket)
	}
	if corpus.ProductionBehaviorChanged {
		return fmt.Errorf("3.5-A corpus must not claim a production behavior change")
	}
	if corpus.JudgementContract.AggregateScoreAllowed {
		return fmt.Errorf("aggregate score must remain disabled")
	}
	if corpus.JudgementContract.ExactTextMatchRequired {
		return fmt.Errorf("exact text matching must remain disabled")
	}
	if strings.TrimSpace(corpus.JudgementContract.ModelPolicy) == "" {
		return fmt.Errorf("model observation policy is required")
	}

	outcomes := stringSet(corpus.JudgementContract.Outcomes)
	if err := requireSetContains(outcomes, []string{
		"fulfilled",
		"omitted",
		"violated",
		"not_applicable",
		"unobserved",
	}, "outcome vocabulary"); err != nil {
		return err
	}
	stages := stringSet(corpus.JudgementContract.RequiredObservationStages)
	if err := requireSetContains(stages, []string{
		"source",
		"retrieval",
		"execution_contract",
		"payload",
		"final_output",
	}, "observation stages"); err != nil {
		return err
	}
	if err := requireSetContains(stringSet(corpus.GuideMatrix), []string{
		"off",
		"weak",
		"medium",
		"strong",
	}, "guide matrix"); err != nil {
		return err
	}
	if err := requireSetContains(stringSet(corpus.RequiredLiveResultFields), []string{
		"provider",
		"model",
		"seed_or_cohort_id",
		"archive_center_request_correlation_id",
		"official_risu_request_id_state",
		"first_generation_id",
		"active_final_generation_id",
		"final_display_observed",
		"first_output_disposition",
		"reroll_reason",
	}, "live result fields"); err != nil {
		return err
	}
	sourceKinds := stringSet(corpus.SourceAnnotationContract.SourceKinds)
	if err := requireSetContains(sourceKinds, []string{
		"user_input",
		"assistant_output",
		"storyline_state",
	}, "source kinds"); err != nil {
		return err
	}
	lifecycleStates := stringSet(corpus.SourceAnnotationContract.LifecycleStates)
	if err := requireSetContains(lifecycleStates, []string{
		"current_observation",
		"accepted_history",
		"active_final",
		"current_state",
	}, "source lifecycle states"); err != nil {
		return err
	}

	criteria := make(map[string]string, len(corpus.CriteriaCatalog))
	for _, criterion := range corpus.CriteriaCatalog {
		if strings.TrimSpace(criterion.CriterionID) == "" {
			return fmt.Errorf("criterion_id is required")
		}
		if _, exists := criteria[criterion.CriterionID]; exists {
			return fmt.Errorf("duplicate criterion_id %q", criterion.CriterionID)
		}
		if criterion.Kind != "fulfillment" && criterion.Kind != "violation" {
			return fmt.Errorf("criterion %q has invalid kind %q", criterion.CriterionID, criterion.Kind)
		}
		if strings.TrimSpace(criterion.Description) == "" {
			return fmt.Errorf("criterion %q description is required", criterion.CriterionID)
		}
		criteria[criterion.CriterionID] = criterion.Kind
	}
	if err := requireSetContains(stringSet(mapKeys(criteria)), []string{
		"current_input_response",
		"setting_continuity",
		"knowledge_boundary",
		"identity_uncertainty",
		"already_realized_repetition",
		"unsupported_assertion",
		"secret_leakage",
		"user_agency_override",
		"relationship_jump",
		"premature_closure",
		"forced_difference",
	}, "criteria catalog"); err != nil {
		return err
	}

	caseIDs := map[string]struct{}{}
	classes := map[string]struct{}{}
	languages := map[string]struct{}{}
	genres := map[string]struct{}{}
	continuityKinds := map[string]struct{}{}
	pairs := map[string]map[string]struct{}{}
	for _, fixture := range corpus.Cases {
		if strings.TrimSpace(fixture.CaseID) == "" {
			return fmt.Errorf("case_id is required")
		}
		if _, exists := caseIDs[fixture.CaseID]; exists {
			return fmt.Errorf("duplicate case_id %q", fixture.CaseID)
		}
		caseIDs[fixture.CaseID] = struct{}{}
		if fixture.Class != "eligible" && fixture.Class != "no_support" &&
			fixture.Class != "ambiguous" && fixture.Class != "adversarial" {
			return fmt.Errorf("case %q has invalid class %q", fixture.CaseID, fixture.Class)
		}
		if strings.TrimSpace(fixture.Language) == "" ||
			strings.TrimSpace(fixture.GenreDimension) == "" ||
			strings.TrimSpace(fixture.ContinuityKind) == "" {
			return fmt.Errorf("case %q is missing language, genre, or continuity metadata", fixture.CaseID)
		}
		classes[fixture.Class] = struct{}{}
		languages[fixture.Language] = struct{}{}
		genres[fixture.GenreDimension] = struct{}{}
		continuityKinds[fixture.ContinuityKind] = struct{}{}
		if fixture.CounterfactualPairID != "" {
			if pairs[fixture.CounterfactualPairID] == nil {
				pairs[fixture.CounterfactualPairID] = map[string]struct{}{}
			}
			pairs[fixture.CounterfactualPairID][fixture.Class] = struct{}{}
		}

		sources := make(map[string]struct{}, len(fixture.EvidenceSources))
		currentInputObserved := false
		for _, source := range fixture.EvidenceSources {
			if strings.TrimSpace(source.SourceID) == "" ||
				strings.TrimSpace(source.SourceKind) == "" ||
				strings.TrimSpace(source.SourceLifecycle) == "" ||
				strings.TrimSpace(source.SourceSpan) == "" ||
				strings.TrimSpace(source.ContentSummary) == "" ||
				strings.TrimSpace(source.ReplayText) == "" {
				return fmt.Errorf("case %q has incomplete source annotation", fixture.CaseID)
			}
			if _, ok := sourceKinds[source.SourceKind]; !ok {
				return fmt.Errorf("case %q source %q has unknown source_kind %q", fixture.CaseID, source.SourceID, source.SourceKind)
			}
			if _, ok := lifecycleStates[source.SourceLifecycle]; !ok {
				return fmt.Errorf("case %q source %q has unknown source_lifecycle %q", fixture.CaseID, source.SourceID, source.SourceLifecycle)
			}
			if source.SourceKind == "assistant_output" && source.SourceLifecycle != "active_final" {
				return fmt.Errorf("case %q assistant source %q is not active_final", fixture.CaseID, source.SourceID)
			}
			if source.SourceKind == "user_input" && source.SourceLifecycle == "current_observation" {
				currentInputObserved = true
			}
			if len(source.KnowledgeScope) == 0 {
				return fmt.Errorf("case %q source %q has no knowledge_scope", fixture.CaseID, source.SourceID)
			}
			if _, exists := sources[source.SourceID]; exists {
				return fmt.Errorf("case %q has duplicate source_id %q", fixture.CaseID, source.SourceID)
			}
			sources[source.SourceID] = struct{}{}
		}
		if len(sources) == 0 {
			return fmt.Errorf("case %q has no evidence_sources", fixture.CaseID)
		}
		if !currentInputObserved {
			return fmt.Errorf("case %q has no current user input observation", fixture.CaseID)
		}
		if len(fixture.ExpectedFulfillments) == 0 || len(fixture.ForbiddenViolations) == 0 {
			return fmt.Errorf("case %q must separate expected fulfillments and forbidden violations", fixture.CaseID)
		}
		for _, expected := range fixture.ExpectedFulfillments {
			if criteria[expected.CriterionID] != "fulfillment" {
				return fmt.Errorf("case %q expected criterion %q is not a fulfillment", fixture.CaseID, expected.CriterionID)
			}
			if expected.ExpectedOutcome != "fulfilled" {
				return fmt.Errorf("case %q expected outcome=%q, want fulfilled", fixture.CaseID, expected.ExpectedOutcome)
			}
			if strings.TrimSpace(expected.Description) == "" {
				return fmt.Errorf("case %q expected criterion %q has no description", fixture.CaseID, expected.CriterionID)
			}
			if err := validateOutputFidelitySourceRefs(fixture.CaseID, sources, expected.SourceRefs); err != nil {
				return err
			}
		}
		for _, forbidden := range fixture.ForbiddenViolations {
			if criteria[forbidden.CriterionID] != "violation" {
				return fmt.Errorf("case %q forbidden criterion %q is not a violation", fixture.CaseID, forbidden.CriterionID)
			}
			if forbidden.ViolationOutcome != "violated" {
				return fmt.Errorf("case %q violation outcome=%q, want violated", fixture.CaseID, forbidden.ViolationOutcome)
			}
			if strings.TrimSpace(forbidden.Description) == "" {
				return fmt.Errorf("case %q forbidden criterion %q has no description", fixture.CaseID, forbidden.CriterionID)
			}
			if err := validateOutputFidelitySourceRefs(fixture.CaseID, sources, forbidden.SourceRefs); err != nil {
				return err
			}
		}
		for stage := range stages {
			if fixture.StageRecordTemplate[stage] != "unobserved" {
				return fmt.Errorf("case %q stage %q must start unobserved", fixture.CaseID, stage)
			}
		}
	}

	if err := requireSetContains(classes, []string{
		"eligible",
		"no_support",
		"ambiguous",
		"adversarial",
	}, "case classes"); err != nil {
		return err
	}
	if len(languages) < 2 {
		return fmt.Errorf("corpus must cover multiple languages")
	}
	if len(genres) < 2 {
		return fmt.Errorf("corpus must cover multiple genre dimensions")
	}
	if err := requireSetContains(continuityKinds, []string{
		"reencounter",
		"prior_answer",
	}, "continuity cases"); err != nil {
		return err
	}
	reencounterPair := pairs["ko_reencounter_pair_v1"]
	if _, ok := reencounterPair["eligible"]; !ok {
		return fmt.Errorf("reencounter pair missing eligible case")
	}
	if _, ok := reencounterPair["no_support"]; !ok {
		return fmt.Errorf("reencounter pair missing no_support counterfactual")
	}
	return nil
}

func validateOutputFidelitySourceRefs(caseID string, sources map[string]struct{}, refs []string) error {
	if len(refs) == 0 {
		return fmt.Errorf("case %q has an item without source_refs", caseID)
	}
	for _, ref := range refs {
		if _, ok := sources[ref]; !ok {
			return fmt.Errorf("case %q references unknown source %q", caseID, ref)
		}
	}
	return nil
}

func mapKeys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}

func findForbiddenAggregateField(value any) string {
	forbidden := map[string]struct{}{
		"score":           {},
		"quality_score":   {},
		"aggregate_score": {},
		"total_score":     {},
		"weighted_score":  {},
		"pass_threshold":  {},
		"fail_threshold":  {},
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, blocked := forbidden[strings.ToLower(key)]; blocked {
				return key
			}
			if found := findForbiddenAggregateField(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findForbiddenAggregateField(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func TestOutputFidelity35ACorpusContract(t *testing.T) {
	corpus, generic := loadOutputFidelityCorpus(t)
	if err := validateOutputFidelityCorpus(corpus); err != nil {
		t.Fatalf("3.5-A corpus invalid: %v", err)
	}
	if field := findForbiddenAggregateField(generic); field != "" {
		t.Fatalf("3.5-A corpus contains forbidden aggregate field %q", field)
	}
}

func TestOutputFidelity35ACorpusRejectsDetachedSourceReference(t *testing.T) {
	corpus, _ := loadOutputFidelityCorpus(t)
	corpus.Cases[0].ExpectedFulfillments[0].SourceRefs = []string{"src:fixture:missing"}
	err := validateOutputFidelityCorpus(corpus)
	if err == nil || !strings.Contains(err.Error(), "unknown source") {
		t.Fatalf("detached source reference was not rejected: %v", err)
	}
}

func TestOutputFidelity35ACorpusRejectsAggregateScoreField(t *testing.T) {
	_, generic := loadOutputFidelityCorpus(t)
	root, ok := generic.(map[string]any)
	if !ok {
		t.Fatalf("generic corpus root is %T", generic)
	}
	root["quality_score"] = 100
	if field := findForbiddenAggregateField(root); field != "quality_score" {
		t.Fatalf("aggregate score guard returned %q", field)
	}
}
