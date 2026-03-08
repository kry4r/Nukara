package persona

import "testing"

func TestClassifyPatchRiskTreatsIdentityChangeAsHighRisk(t *testing.T) {
	result := ClassifyPatchRisk(Patch{IdentityAdds: []string{"其实我是医生"}})
	if result.Risk != RiskHigh {
		t.Fatalf("risk = %q, want %q", result.Risk, RiskHigh)
	}
	if result.Route != RoutePendingConfirm {
		t.Fatalf("route = %q, want %q", result.Route, RoutePendingConfirm)
	}
}

func TestClassifyPatchRiskTreatsExpressionStyleChangeAsLowRisk(t *testing.T) {
	result := ClassifyPatchRisk(Patch{ExpressionStyleAdds: []string{"最近更喜欢黏一点的口气"}})
	if result.Risk != RiskLow {
		t.Fatalf("risk = %q, want %q", result.Risk, RiskLow)
	}
	if result.Route != RouteAutoApply {
		t.Fatalf("route = %q, want %q", result.Route, RouteAutoApply)
	}
}

func TestClassifyPatchRiskTreatsHabitLifeContextChangeAsLowRisk(t *testing.T) {
	result := ClassifyPatchRisk(Patch{LifeContextAdds: []string{"最近开始把凌晨散步当作下班后的固定习惯"}})
	if result.Risk != RiskLow {
		t.Fatalf("risk = %q, want %q", result.Risk, RiskLow)
	}
	if result.Route != RouteAutoApply {
		t.Fatalf("route = %q, want %q", result.Route, RouteAutoApply)
	}
}

func TestClassifyPatchRiskTreatsCoreRoleLifeContextChangeAsHighRisk(t *testing.T) {
	result := ClassifyPatchRisk(Patch{LifeContextAdds: []string{"其实我是医院急诊科医生，最近一直上夜班"}})
	if result.Risk != RiskHigh {
		t.Fatalf("risk = %q, want %q", result.Risk, RiskHigh)
	}
	if result.Route != RoutePendingConfirm {
		t.Fatalf("route = %q, want %q", result.Route, RoutePendingConfirm)
	}
}
