package httpapi

const referenceInjectionBudgetContractVersion = "reference_injection_budget.v1"

type referencePrimaryCanonBaseBudget struct {
	Scope              string `json:"scope"`
	Configured         bool   `json:"configured"`
	ConfiguredCapChars int    `json:"configured_cap_chars"`
	EffectiveCapChars  int    `json:"effective_cap_chars"`
	UsedChars          int    `json:"used_chars"`
}

type referenceInjectionBudgetPolicy struct {
	ContractVersion       string                          `json:"contract_version"`
	Status                string                          `json:"status"`
	Mode                  string                          `json:"mode"`
	Source                string                          `json:"source"`
	ResolutionRule        string                          `json:"resolution_rule"`
	Rounding              string                          `json:"rounding"`
	RelationshipToMain    string                          `json:"relationship_to_main"`
	MainInjectionCapChars int                             `json:"main_injection_cap_chars"`
	BudgetBasisChars      int                             `json:"budget_basis_chars"`
	TotalCapChars         int                             `json:"total_cap_chars"`
	UsedChars             int                             `json:"used_chars"`
	RemainingChars        int                             `json:"remaining_chars"`
	Truncated             bool                            `json:"truncated"`
	RatioNumerator        int                             `json:"ratio_numerator"`
	RatioDenominator      int                             `json:"ratio_denominator"`
	PrimaryCanonBase      referencePrimaryCanonBaseBudget `json:"primary_canon_base"`
}

func resolveReferenceInjectionBudget(maxInjectionChars, budgetBasisChars int, injectionEnabled bool, bindingCount int, referenceModes map[string]int, primaryCanonBaseConfiguredCap *int) referenceInjectionBudgetPolicy {
	mainCap := maxInjectionChars
	if mainCap < 0 {
		mainCap = 0
	}
	policy := referenceInjectionBudgetPolicy{
		ContractVersion:       referenceInjectionBudgetContractVersion,
		Status:                "off",
		Mode:                  "off",
		Source:                "no_active_reference_mode",
		ResolutionRule:        "primary_if_any",
		Rounding:              "floor",
		RelationshipToMain:    "additive_non_displacing",
		MainInjectionCapChars: mainCap,
		BudgetBasisChars:      maxReferenceBudget(budgetBasisChars, 0),
		RatioNumerator:        0,
		RatioDenominator:      1,
		PrimaryCanonBase: referencePrimaryCanonBaseBudget{
			Scope: "within_reference_total",
		},
	}
	if primaryCanonBaseConfiguredCap != nil {
		policy.PrimaryCanonBase.Configured = true
		policy.PrimaryCanonBase.ConfiguredCapChars = *primaryCanonBaseConfiguredCap
		if policy.PrimaryCanonBase.ConfiguredCapChars < 0 {
			policy.PrimaryCanonBase.ConfiguredCapChars = 0
		}
	}
	switch {
	case policy.BudgetBasisChars <= 0:
		policy.Source = "reference_budget_basis_non_positive"
	case !injectionEnabled:
		policy.Source = "reference_injection_disabled"
	case bindingCount <= 0:
		policy.Source = "no_reference_binding"
	case referenceModes[referenceModePrimary] > 0:
		policy.Status = "resolved"
		policy.Mode = referenceModePrimary
		policy.Source = "primary_reference_mode"
		policy.TotalCapChars = policy.BudgetBasisChars
		policy.RatioNumerator = 1
		policy.PrimaryCanonBase.EffectiveCapChars = minReferenceBudget(policy.PrimaryCanonBase.ConfiguredCapChars, policy.TotalCapChars)
	case referenceModes[referenceModeSupplement] > 0:
		policy.Status = "resolved"
		policy.Mode = referenceModeSupplement
		policy.Source = "supplement_reference_mode"
		policy.TotalCapChars = policy.BudgetBasisChars / 2
		policy.RatioNumerator = 1
		policy.RatioDenominator = 2
	}
	return policy
}

func maxReferenceBudget(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minReferenceBudget(left, right int) int {
	if left < right {
		return left
	}
	return right
}
