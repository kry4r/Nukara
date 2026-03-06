package api

import (
	"fmt"
	"strings"
)

func buildMemoryExtractPrompt(userText, botText, existingMemory string) string {
	if strings.TrimSpace(existingMemory) == "" {
		existingMemory = "（暂无）"
	}
	return fmt.Sprintf(`[system:memory_extract_json]
你现在负责判断“这轮对话里有没有值得长期记住的内容”，并决定是新建记忆还是更新已有记忆。

规则：
1. 只有在确实值得长期保留时才写入 items；如果没有，必须返回 {"items":[]}。
2. 适合长期记忆的内容包括：
   - 用户稳定偏好/厌恶（喜欢什么、讨厌什么、长期习惯）
   - 用户身份信息/关系信息/重要背景
   - 明确希望你以后记住的要求或边界
   - 对后续关系重要的承诺、计划、里程碑事件
   - 可沉淀为 bot 自我认知或双方共享事实的关键信息
3. 不要记住以下内容：
   - 寒暄、口头禅、一次性闲聊
   - 临时任务指令或测试语句（例如“只回复这句话”）
   - 没有长期价值的短暂情绪波动
   - 对当前回复格式的即时要求
   - 明显重复、泛化、无信息量的句子
4. 先对照“已有长期记忆”：
   - 如果已有记忆已经完整覆盖这次信息，返回 {"items":[]}。
   - 如果这次信息是在修正/补充已有记忆，复用对应 memory_id，并输出修正后的稳定表述。
   - 只有确实全新时才创建新记忆（memory_id 留空）。
5. owner 含义：
   - user：关于用户的长期信息
   - bot：关于 bot 自我认知/长期承诺的信息
   - shared：双方共享、会影响后续关系的事实
6. content 要写成简洁、可长期复用的稳定事实，topics 只保留真正可检索的关键词。
7. 宁缺毋滥；不确定时就不要记。

已有长期记忆：
%s

严格输出 JSON：
{"items":[{"memory_id":"已有记忆ID或空字符串","kind":"fact","owner":"user|bot|shared","content":"...","importance":0-100,"status":"active","topics":["..."]}]}
用户：%s
机器人：%s`, existingMemory, userText, botText)
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
