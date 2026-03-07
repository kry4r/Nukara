package api

import (
	"strings"
	"time"
)

type LocaleContext struct {
	Timezone  string
	Region    string
	LocalTime string
	DayPhase  string
	localNow  time.Time
}

type localeRule struct {
	Timezone string
	Region   string
	Keywords []string
}

var defaultLocaleRule = localeRule{
	Timezone: "Asia/Shanghai",
	Region:   "中国",
}

var localeRules = []localeRule{
	{
		Timezone: "Asia/Tokyo",
		Region:   "日本",
		Keywords: []string{"东京", "日本", "tokyo", "japan"},
	},
	{
		Timezone: "America/New_York",
		Region:   "美国东部",
		Keywords: []string{"纽约", "美国", "new york", "nyc", "usa", "us"},
	},
}

func InferLocaleContext(lifeContext string, now time.Time) LocaleContext {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rule := defaultLocaleRule
	normalized := strings.ToLower(strings.TrimSpace(lifeContext))
	for _, candidate := range localeRules {
		if containsLocaleKeyword(normalized, candidate.Keywords) {
			rule = candidate
			break
		}
	}

	loc, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	return LocaleContext{
		Timezone:  rule.Timezone,
		Region:    rule.Region,
		LocalTime: localNow.Format("2006-01-02 15:04"),
		DayPhase:  inferDayPhase(localNow),
		localNow:  localNow,
	}
}

func (lc LocaleContext) LocalNow() time.Time {
	if lc.localNow.IsZero() {
		return time.Time{}
	}
	return lc.localNow
}

func enrichLocaleSystemContext(systemContext map[string]any, now time.Time) map[string]any {
	out := cloneSystemContext(systemContext)
	locale := InferLocaleContext(stringifySystemContextValue(out["life_context"]), now)
	out["local_timezone"] = locale.Timezone
	out["local_region"] = locale.Region
	out["local_time"] = locale.LocalTime
	out["day_phase"] = locale.DayPhase
	return out
}

func formatLocaleContext(systemContext map[string]any) string {
	parts := make([]string, 0, 4)
	if localTime := stringifySystemContextValue(systemContext["local_time"]); localTime != "" {
		parts = append(parts, "当地时间："+localTime)
	}
	if timezone := stringifySystemContextValue(systemContext["local_timezone"]); timezone != "" {
		parts = append(parts, "时区："+timezone)
	}
	if dayPhase := stringifySystemContextValue(systemContext["day_phase"]); dayPhase != "" {
		parts = append(parts, "时间段："+dayPhase)
	}
	if region := stringifySystemContextValue(systemContext["local_region"]); region != "" {
		parts = append(parts, "地域："+region)
	}
	return strings.Join(parts, "；")
}

func cloneSystemContext(systemContext map[string]any) map[string]any {
	if len(systemContext) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(systemContext)+4)
	for key, value := range systemContext {
		switch typed := value.(type) {
		case []string:
			out[key] = append([]string(nil), typed...)
		case []any:
			out[key] = append([]any(nil), typed...)
		default:
			out[key] = typed
		}
	}
	return out
}

func containsLocaleKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func inferDayPhase(now time.Time) string {
	hour := now.Hour()
	switch {
	case hour < 5:
		return "深夜"
	case hour < 7:
		return "清晨"
	case hour < 11:
		return "早上"
	case hour < 13:
		return "中午"
	case hour < 18:
		return "下午"
	case hour < 23:
		return "晚上"
	default:
		return "深夜"
	}
}
