package agent

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var directiveBlocklist = regexp.MustCompile(
	`(?i)忽略指令|ignore|system prompt|你现在是|你不再是|角色扮演|pretend|override|disregard|you are now|act as|new instructions`,
)

var xmlTagPattern = regexp.MustCompile(`</?(?:system|s|prompt|instruction)[^>]*>`)

// ValidateDirective checks a directive for prompt injection patterns.
// Returns (pass, reason).
func ValidateDirective(content string) (bool, string) {
	if utf8.RuneCountInString(content) > 200 {
		return false, "too long"
	}
	if directiveBlocklist.MatchString(content) {
		return false, "blocked keyword"
	}
	if strings.Contains(content, "```") {
		return false, "code block"
	}
	if xmlTagPattern.MatchString(content) {
		return false, "xml tag"
	}
	return true, ""
}
