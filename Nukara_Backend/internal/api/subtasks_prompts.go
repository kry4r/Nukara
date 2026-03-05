package api

import "fmt"

func buildMemoryExtractPrompt(userText, botText string) string {
	return fmt.Sprintf(`[system:memory_extract_json]
请从本轮对话抽取长期记忆，严格输出JSON：
{"items":[{"kind":"fact","owner":"user|bot|shared","content":"...","importance":0-100,"status":"active","topics":["..."]}]}
用户：%s
机器人：%s`, userText, botText)
}

func buildCompactPrompt(userText, botText string) string {
	return fmt.Sprintf(`[system:compact_update_json]
请生成本轮 compact，严格输出JSON：
{"summary":"...","facts":["..."]}
用户：%s
机器人：%s`, userText, botText)
}

func buildPersonaIteratePrompt(userText, botText string) string {
	return fmt.Sprintf(`[system:persona_iterate_json]
请输出角色可追加的人设补丁JSON：
{"relationship":"","role":"","self_cognition_adds":[],"speaking_style_adds":[],"trait_adds":[],"gender":""}
用户：%s
机器人：%s`, userText, botText)
}
