package httpapi

import "testing"

func TestResolveReferenceInjectionBudget(t *testing.T) {
	mainCap := 101
	primaryCap := 80
	tests := []struct {
		name             string
		maxChars         int
		enabled          bool
		bindingCount     int
		modes            map[string]int
		wantMode         string
		wantSource       string
		wantTotal        int
		wantNumerator    int
		wantDenominator  int
		wantPrimaryLimit int
	}{
		{name: "no binding", maxChars: mainCap, enabled: true, wantMode: "off", wantSource: "no_reference_binding", wantDenominator: 1},
		{name: "reference injection disabled", maxChars: mainCap, bindingCount: 1, modes: map[string]int{referenceModePrimary: 1}, wantMode: "off", wantSource: "reference_injection_disabled", wantDenominator: 1},
		{name: "non-positive budget basis", maxChars: 0, enabled: true, bindingCount: 1, modes: map[string]int{referenceModePrimary: 1}, wantMode: "off", wantSource: "reference_budget_basis_non_positive", wantDenominator: 1},
		{name: "unknown mode only", maxChars: mainCap, enabled: true, bindingCount: 1, modes: map[string]int{referenceModeUnknown: 1}, wantMode: "off", wantSource: "no_active_reference_mode", wantDenominator: 1},
		{name: "supplement uses floor half", maxChars: mainCap, enabled: true, bindingCount: 1, modes: map[string]int{referenceModeSupplement: 1}, wantMode: referenceModeSupplement, wantSource: "supplement_reference_mode", wantTotal: mainCap / 2, wantNumerator: 1, wantDenominator: 2},
		{name: "primary uses full main cap", maxChars: mainCap, enabled: true, bindingCount: 1, modes: map[string]int{referenceModePrimary: 1}, wantMode: referenceModePrimary, wantSource: "primary_reference_mode", wantTotal: mainCap, wantNumerator: 1, wantDenominator: 1, wantPrimaryLimit: primaryCap},
		{name: "primary dominates mixed modes", maxChars: mainCap, enabled: true, bindingCount: 2, modes: map[string]int{referenceModeSupplement: 1, referenceModePrimary: 1}, wantMode: referenceModePrimary, wantSource: "primary_reference_mode", wantTotal: mainCap, wantNumerator: 1, wantDenominator: 1, wantPrimaryLimit: primaryCap},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := resolveReferenceInjectionBudget(test.maxChars, test.maxChars, test.enabled, test.bindingCount, test.modes, &primaryCap)
			if policy.Mode != test.wantMode || policy.Source != test.wantSource || policy.TotalCapChars != test.wantTotal {
				t.Fatalf("policy = %#v", policy)
			}
			if policy.RatioNumerator != test.wantNumerator || policy.RatioDenominator != test.wantDenominator {
				t.Fatalf("ratio = %d/%d, want %d/%d", policy.RatioNumerator, policy.RatioDenominator, test.wantNumerator, test.wantDenominator)
			}
			if policy.PrimaryCanonBase.Scope != "within_reference_total" || policy.PrimaryCanonBase.EffectiveCapChars != test.wantPrimaryLimit {
				t.Fatalf("primary subbudget = %#v", policy.PrimaryCanonBase)
			}
		})
	}
}

func TestResolveReferenceInjectionBudgetBoundsPrimarySubbudget(t *testing.T) {
	mainCap := 120
	configuredPrimaryCap := mainCap * 10
	policy := resolveReferenceInjectionBudget(mainCap, mainCap, true, 1, map[string]int{referenceModePrimary: 1}, &configuredPrimaryCap)
	if policy.PrimaryCanonBase.ConfiguredCapChars != configuredPrimaryCap || policy.PrimaryCanonBase.EffectiveCapChars != policy.TotalCapChars {
		t.Fatalf("primary cap was not bounded by reference total: %#v", policy)
	}
}

func TestResolveReferenceInjectionBudgetUsesConfiguredBasisWhenMainTurnCapIsZero(t *testing.T) {
	basis := 1200
	policy := resolveReferenceInjectionBudget(0, basis, true, 1, map[string]int{referenceModePrimary: 1}, intPointer(300))
	if policy.MainInjectionCapChars != 0 || policy.BudgetBasisChars != basis || policy.TotalCapChars != basis {
		t.Fatalf("independent first-turn reference basis = %#v", policy)
	}
}
