package persona

import "strings"

type RiskLevel string

type ApplyRoute string

const (
	RiskLow  RiskLevel = "low"
	RiskHigh RiskLevel = "high"

	RouteAutoApply      ApplyRoute = "auto_apply"
	RoutePendingConfirm ApplyRoute = "pending_confirm"
)

type RiskResult struct {
	Risk  RiskLevel
	Route ApplyRoute
}

var lowRiskLifeContextCues = []string{
	"习惯", "作息", "固定", "最近开始", "开始把", "最近会", "会把", "下班后", "晨跑", "夜跑", "散步", "慢走",
	"喝咖啡", "做饭", "通勤", "晚饭后", "睡前", "周末常", "最近喜欢", "最近常", "会在",
}

var highRiskLifeContextCues = []string{
	"其实我是", "医生", "护士", "老师", "学生", "程序员", "警察", "军人", "结婚", "离婚", "怀孕", "生孩子",
	"男朋友", "女朋友", "恋人", "老公", "老婆", "母亲", "父亲", "住在", "搬到", "来自", "出生", "故乡", "户口",
}

func ClassifyPatchRisk(patch Patch) RiskResult {
	if len(patch.IdentityAdds) > 0 {
		return highRisk()
	}
	if len(patch.PersonalityAdds) > 0 {
		return highRisk()
	}
	if len(patch.TaboosAndPreferencesAdds) > 0 {
		return highRisk()
	}
	for _, value := range patch.LifeContextAdds {
		if classifyLifeContextAdd(value) == RiskHigh {
			return highRisk()
		}
	}
	return RiskResult{Risk: RiskLow, Route: RouteAutoApply}
}

func classifyLifeContextAdd(value string) RiskLevel {
	text := strings.TrimSpace(value)
	if text == "" {
		return RiskLow
	}
	if containsAny(text, highRiskLifeContextCues) {
		return RiskHigh
	}
	if containsAny(text, lowRiskLifeContextCues) {
		return RiskLow
	}
	return RiskHigh
}

func containsAny(text string, cues []string) bool {
	for _, cue := range cues {
		cue = strings.TrimSpace(cue)
		if cue != "" && strings.Contains(text, cue) {
			return true
		}
	}
	return false
}

func highRisk() RiskResult {
	return RiskResult{Risk: RiskHigh, Route: RoutePendingConfirm}
}
