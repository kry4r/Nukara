package api

import (
	"fmt"
	"strings"
)

// buildMemoryExtractPrompt builds the per-turn memory extraction prompt.
// botName is the bot's display name (e.g. "米米").
// userRole is how the bot refers to the user (e.g. "我的恋人", "朋友").
func buildMemoryExtractPrompt(userText, botText, existingMemory, botName, userRole string) string {
	if strings.TrimSpace(existingMemory) == "" {
		existingMemory = "（暂无）"
	}
	roleCtx := buildRoleContext(botName, userRole)
	return fmt.Sprintf(`[system:memory_extract_json]
你现在负责判断"这轮对话里有没有值得长期保留的记忆事实"，并决定是新建记忆还是更新已有记忆。

%s

规则：
1. 只有在确实值得长期保留时才写入 items；如果没有，必须返回 {"items":[]}。
2. 重点提取以下类型的信息：
   - bot 的稳定身份信息，例如专业方向、职业定位、自我认知
   - bot 的生活背景，例如宠物、家庭关系、居住情况
   - 用户稳定偏好、边界、长期习惯
   - 对后续关系重要的承诺、计划、里程碑事件
3. 明确关注这些场景：专业、宠物、家庭关系、父母称谓、长期偏好、稳定生活背景。
4. 不要记住以下内容：
   - 寒暄、口头禅、一次性闲聊
   - 临时任务指令或测试语句（例如"只回复这句话"）
   - 没有长期价值的短暂情绪波动
   - 对当前回复格式的即时要求
   - 明显重复、泛化、无信息量的句子
5. 先对照"已有长期记忆"：
   - 如果已有记忆已经完整覆盖这次信息，返回 {"items":[]}。
   - 如果这次信息是在修正/补充已有记忆，复用对应 memory_id，并输出修正后的稳定表述。
   - 只有确实全新时才创建新记忆（memory_id 留空）。
   - 如果多条已有记忆描述的是同一场景（相同地点、人物、话题），优先合并成一条更完整的记忆，复用其中一个 memory_id。
6. kind 可取：event | promise | self_fact | user_fact | habit | state_basis | fact。
7. semantic_category 可取：identity | personality | life_context | expression_style | taboos_and_preferences | relationship。
8. stability 只允许 stable 或 temporary；出现"最近/这周/暂时/今天"等时间限定词时应优先标为 temporary。
9. 如果文本里提到人物、宠物、地点等实体，请在 entities 里列出；如果能识别关系，请在 relations 里列出。
   - entity.type 必须是以下之一：person（人物）、location（地点）、time（时间）、organization（组织）
   - 示例：{"id":"pet-doufu","type":"person","name":"豆腐","role":"user"}
10. content 要写成简洁、可长期复用的稳定事实，topics 只保留真正可检索的关键词。

已有长期记忆：
%s

严格输出 JSON：
{"items":[{"memory_id":"已有记忆ID或空字符串","kind":"self_fact","owner":"user|bot|shared","content":"...","importance":0-100,"status":"active","topics":["..."],"semantic_category":"identity","stability":"stable","entities":[{"id":"pet-xiaomi","type":"pet","name":"小蜜","role":"bot"}],"relations":[{"source_entity_id":"bot-self","target_entity_id":"pet-xiaomi","relation_type":"owns","role_hint":"first_person"}]}]}
用户：%s
机器人：%s`, roleCtx, existingMemory, userText, botText)
}

// buildMemoryExtractPromptAggregated is like buildMemoryExtractPrompt but
// accepts pre-formatted multi-turn conversation text (e.g. from the memory
// buffer) instead of separate userText/botText strings.
func buildMemoryExtractPromptAggregated(aggregatedText, existingMemory, botName, userRole string) string {
	if strings.TrimSpace(existingMemory) == "" {
		existingMemory = "（暂无）"
	}
	roleCtx := buildRoleContext(botName, userRole)
	return fmt.Sprintf(`[system:memory_extract_json]
你现在负责从一段多轮对话中提取值得长期保留的记忆事实，并决定是新建记忆还是更新已有记忆。

%s

规则：
1. 只有在确实值得长期保留时才写入 items；如果没有，必须返回 {"items":[]}。
2. 重点提取以下类型的信息：
   - bot 的稳定身份信息，例如专业方向、职业定位、自我认知
   - bot 的生活背景，例如宠物、家庭关系、居住情况
   - 用户稳定偏好、边界、长期习惯
   - 对后续关系重要的承诺、计划、里程碑事件
3. 明确关注这些场景：专业、宠物、家庭关系、父母称谓、长期偏好、稳定生活背景。
4. 不要记住以下内容：
   - 寒暄、口头禅、一次性闲聊
   - 临时任务指令或测试语句
   - 没有长期价值的短暂情绪波动
   - 明显重复、泛化、无信息量的句子
5. 先对照"已有长期记忆"：
   - 如果已有记忆已经完整覆盖这次信息，返回 {"items":[]}。
   - 如果这次信息是在修正/补充已有记忆，复用对应 memory_id，并输出修正后的稳定表述。
   - 只有确实全新时才创建新记忆（memory_id 留空）。
   - 如果多条已有记忆描述的是同一场景（相同地点、人物、话题），优先合并成一条更完整的记忆，复用其中一个 memory_id。
6. kind 可取：event | promise | self_fact | user_fact | habit | state_basis | fact。
7. semantic_category 可取：identity | personality | life_context | expression_style | taboos_and_preferences | relationship。
8. stability 只允许 stable 或 temporary；出现"最近/这周/暂时/今天"等时间限定词时应优先标为 temporary。
9. 如果文本里提到人物、宠物、地点等实体，请在 entities 里列出；如果能识别关系，请在 relations 里列出。
   - entity.type 必须是以下之一：person（人物）、location（地点）、time（时间）、organization（组织）
10. content 要写成简洁、可长期复用的稳定事实，topics 只保留真正可检索的关键词。

已有长期记忆：
%s

严格输出 JSON：
{"items":[{"memory_id":"已有记忆ID或空字符串","kind":"self_fact","owner":"user|bot|shared","content":"...","importance":0-100,"status":"active","topics":["..."],"semantic_category":"identity","stability":"stable","entities":[],"relations":[]}]}
对话内容：
%s`, roleCtx, existingMemory, aggregatedText)
}

// buildRoleContext returns the persona context block injected into memory prompts.
func buildRoleContext(botName, userRole string) string {
	botName = strings.TrimSpace(botName)
	userRole = strings.TrimSpace(userRole)
	if botName == "" {
		botName = "我"
	}
	if userRole == "" {
		userRole = "用户"
	}
	return fmt.Sprintf(`角色说明（写入 content 时必须遵守）：
- 对话中的"机器人"即"%s"，在 content 里一律写成"我"（第一人称）。
- 对话中的"用户"即"%s"，在 content 里一律使用这个称谓。
- 示例：不要写"bot在咖啡厅打工"，要写"我在咖啡厅打工"。`, botName, userRole)
}

func buildCompactPrompt(userText, botText string) string {
	return fmt.Sprintf(`[system:compact_update_json]
请生成本轮 compact，严格输出JSON：
{"summary":"...","facts":["..."]}
用户：%s
机器人：%s`, userText, botText)
}

// buildPersonaIteratePrompt generates persona patch suggestions.
// botName is used so the LLM writes self-facts in first person.
func buildPersonaIteratePrompt(userText, botText, botName string) string {
	if strings.TrimSpace(botName) == "" {
		botName = "我"
	}
	return fmt.Sprintf(`[system:persona_iterate_json]
你正在为 AI 角色"%s"分析人设变化。
重要：所有 adds 里的文字必须用第一人称描述（用"我"，不要用"bot"或角色名字）。
示例：不要写"米米在咖啡厅打工"，要写"我在咖啡厅打工"。
请输出角色可追加的人设补丁JSON：
{"identity_adds":[],"personality_adds":[],"expression_style_adds":[],"life_context_adds":[],"taboos_and_preferences_adds":[]}
用户：%s
机器人：%s`, botName, userText, botText)
}

func buildSelfCognitionSummaryPrompt(current []string, changes []string) string {
	currentText := strings.TrimSpace(strings.Join(current, "；"))
	if currentText == "" {
		currentText = "（暂无）"
	}
	changeText := strings.TrimSpace(strings.Join(changes, "\n- "))
	if changeText == "" {
		changeText = "（暂无）"
	} else {
		changeText = "- " + changeText
	}
	return fmt.Sprintf(`[system:self_cognition_summary_json]
请根据最近已经确认的人设变化，输出一条简短的"自我认知"总结。
要求：
1. 严格输出 JSON：{"self_cognition":"..."}
2. 只输出一个短句，18~50字。
3. 用第一人称"我"来描述，不要出现"bot"或角色名字。
4. 优先总结最近形成的习惯、近期状态、表达倾向，不要改写核心身份设定。
5. 如果没有足够新变化，就沿用当前自我认知并自然压缩。

当前自我认知：%s
最近确认的人设变化：
%s`, currentText, changeText)
}
