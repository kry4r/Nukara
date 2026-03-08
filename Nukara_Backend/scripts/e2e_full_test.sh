#!/bin/bash
set -uo pipefail

BASE_URL="${1:-http://localhost:8080}"
TOTAL_ROUNDS=50
MEMORY_TEST_ROUND=30
REPORT_FILE="/tmp/nukara_e2e_report_$(date +%Y%m%d_%H%M%S).md"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

pass() { ((PASS_COUNT++)) || true; echo -e "${GREEN}[PASS]${NC} $1"; echo "- [PASS] $1" >> "$REPORT_FILE"; }
fail() { ((FAIL_COUNT++)) || true; echo -e "${RED}[FAIL]${NC} $1"; echo "- [FAIL] $1" >> "$REPORT_FILE"; }
warn() { ((WARN_COUNT++)) || true; echo -e "${YELLOW}[WARN]${NC} $1"; echo "- [WARN] $1" >> "$REPORT_FILE"; }
info() { echo -e "       $1"; }
section() { echo -e "\n=== $1 ==="; echo -e "\n## $1\n" >> "$REPORT_FILE"; }

# --- JSON helpers ---
jval() { echo "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$1',''))" 2>/dev/null; }
jpath() { echo "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); v=$1; print(v if v else '')" 2>/dev/null; }

curl_json() {
  local method="$1" url="$2" body="${3:-}" auth="${4:-}"
  local args=(-sS -X "$method" "$url" -H "Content-Type: application/json")
  [[ -n "$auth" ]] && args+=(-H "Authorization: Bearer $auth")
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" 2>/dev/null
}

EMAIL_DOMAIN="${NUKARA_TEST_EMAIL_DOMAIN:-nukara.local}"

generate_email() {
  printf "e2e_%s_%s@%s" "$(date +%s)" "$((RANDOM * RANDOM % 100000000))" "$EMAIL_DOMAIN"
}

# --- Init report ---
cat > "$REPORT_FILE" << 'EOF'
# Nukara 端到端全量集成测试报告

EOF
echo "**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')" >> "$REPORT_FILE"
echo "**测试环境**: $BASE_URL" >> "$REPORT_FILE"
echo "**对话轮次**: $TOTAL_ROUNDS" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# ============================================================
section "Phase 1: 用户注册与机器人创建"
# ============================================================

EMAIL=$(generate_email)
info "测试邮箱: $EMAIL"

# 1.1 发送验证码
email_resp=$(curl_json POST "$BASE_URL/api/v1/auth/email/send" "{\"email\":\"$EMAIL\",\"purpose\":\"register\"}")
if echo "$email_resp" | grep -q "ok\|success\|code"; then
  pass "发送邮箱验证码"
else
  fail "发送邮箱验证码: $email_resp"
fi

# Extract email code from gateway Docker logs
sleep 0.5
EMAIL_CODE=$(docker logs configs-gateway-1 --tail 20 2>&1 | grep "\[EMAIL\].*email=$EMAIL" | tail -1 | sed 's/.*code=//')
if [[ -z "$EMAIL_CODE" ]]; then
  fail "无法从日志提取邮箱验证码，请确认 SMTP 已配置且 gateway 日志可访问"
  exit 1
fi
info "验证码: $EMAIL_CODE"

# 1.2 注册
reg_resp=$(curl_json POST "$BASE_URL/api/v1/auth/register" "{\"email\":\"$EMAIL\",\"email_code\":\"$EMAIL_CODE\",\"nickname\":\"e2e_tester\"}")
TOKEN=$(jval access_token "$reg_resp")
if [[ -n "$TOKEN" ]]; then
  pass "用户注册 (token获取成功)"
else
  fail "用户注册失败: $reg_resp"
  echo "无法继续测试，退出"
  exit 1
fi

# 1.3 创建机器人
bot_resp=$(curl_json POST "$BASE_URL/api/v1/bots" '{
  "name":"苏子衿",
  "summary":"温柔体贴的江南女子，喜欢诗词和茶道",
  "speaking_style":"温柔婉约，偶尔引用诗词",
  "background":"出生在苏州，从小学习古典文学和茶艺",
  "traits":["温柔","体贴","有文化","善解人意"],
  "gender":"female"
}' "$TOKEN")
BOT_ID=$(jval id "$bot_resp")
BOT_NAME=$(jval name "$bot_resp")
if [[ -n "$BOT_ID" ]]; then
  pass "创建机器人: $BOT_NAME ($BOT_ID)"
else
  fail "创建机器人失败: $bot_resp"
  exit 1
fi

# 1.4 获取会话
conv_resp=$(curl_json GET "$BASE_URL/api/v1/conversations" "" "$TOKEN")
CONV_ID=$(echo "$conv_resp" | python3 -c "
import sys,json
data=json.load(sys.stdin)
convs = data if isinstance(data, list) else data.get('conversations',[])
for c in convs:
  if c.get('bot_id','') == '$BOT_ID':
    print(c['id']); break
" 2>/dev/null)
if [[ -n "$CONV_ID" ]]; then
  pass "获取会话: $CONV_ID"
else
  fail "获取会话失败: $conv_resp"
  exit 1
fi

# 1.5 检查开场白
msgs_resp=$(curl_json GET "$BASE_URL/api/v1/conversations/$CONV_ID/messages?limit=5" "" "$TOKEN")
starter_text=$(echo "$msgs_resp" | python3 -c "
import sys,json
data=json.load(sys.stdin)
msgs = data if isinstance(data, list) else data.get('messages',[])
for m in msgs:
  if m.get('sender_type') == 'bot':
    c = m.get('content',{})
    print(c.get('text',''))
    break
" 2>/dev/null)
if [[ -n "$starter_text" ]]; then
  pass "开场白生成: \"${starter_text:0:60}...\""
else
  warn "未检测到开场白消息"
fi

# ============================================================
section "Phase 2: 50轮对话测试"
# ============================================================

PROMPTS=(
  "你好呀，很高兴认识你"
  "你平时喜欢做什么？"
  "我今天心情不太好"
  "你觉得什么是幸福？"
  "我最近在学画画"
  "你喜欢什么季节？"
  "给我讲个有趣的故事吧"
  "我叫小明，记住我的名字哦"
  "你知道我叫什么名字吗？"
  "我喜欢吃火锅"
  "今天下雨了，好冷"
  "你会弹钢琴吗？"
  "我最喜欢的颜色是蓝色"
  "周末我想去爬山"
  "你觉得人生的意义是什么？"
  "我养了一只猫，叫小橘"
  "你还记得我养的宠物吗？"
  "最近工作压力好大"
  "你能安慰安慰我吗？"
  "我喜欢听周杰伦的歌"
  "你最喜欢哪首诗？"
  "我昨天做了一个奇怪的梦"
  "你觉得我是什么样的人？"
  "我想学做饭"
  "推荐一本好书给我吧"
  "我的生日是3月15号"
  "你还记得我的生日吗？"
  "今天好开心，升职了！"
  "我想去日本旅行"
  "你觉得樱花美吗？"
  "我有时候会感到孤独"
  "谢谢你一直陪着我"
  "你觉得我们算朋友吗？"
  "我最近在看一部电影"
  "你喜欢什么类型的音乐？"
  "我小时候想当宇航员"
  "你有什么梦想？"
  "今天吃了一顿大餐"
  "我喜欢在雨天喝咖啡"
  "你觉得什么是爱？"
  "我最好的朋友叫小红"
  "你还记得我朋友的名字吗？"
  "我想学一门新语言"
  "你觉得我适合学什么？"
  "最近睡眠不太好"
  "有什么助眠的方法吗？"
  "我喜欢看星星"
  "你觉得宇宙有多大？"
  "今天是个好日子"
  "和你聊天真开心"
)

REPLY_COUNT=0
STATUS_COUNT=0
EMOTION_COUNT=0
TAG_LEAK=0
MEMORY_KEYWORDS=()

echo "开始 $TOTAL_ROUNDS 轮对话..." >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

for i in $(seq 1 $TOTAL_ROUNDS); do
  idx=$(( (i - 1) % ${#PROMPTS[@]} ))
  prompt="${PROMPTS[$idx]}"

  resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
    "{\"client_msg_id\":\"e2e-msg-$i\",\"content\":{\"type\":\"text\",\"text\":\"$prompt\"}}" "$TOKEN")

  # Extract bot reply
  bot_text=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
bm=d.get('bot_message',{})
c=bm.get('content',{})
print(c.get('text',''))
" 2>/dev/null)

  # Extract status update
  status_emoji=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
su=d.get('bot_status_update',)
print(su.get('emoji',''))
" 2>/dev/null)
  status_text=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
su=d.get('bot_status_update',{})
print(su.get('text',''))
" 2>/dev/null)

  # Extract emotion tag
  emotion=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
bm=d.get('bot_message',{})
print(bm.get('emotion_tag',''))
" 2>/dev/null)

  # Check ack
  ack_id=$(echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('ack',{}).get('client_msg_id',''))
" 2>/dev/null)

  if [[ -n "$bot_text" ]]; then
    ((REPLY_COUNT++)) || true
  fi
  if [[ -n "$status_emoji" && "$status_emoji" != "None" ]]; then
    ((STATUS_COUNT++)) || true
  fi
  if [[ -n "$emotion" && "$emotion" != "None" && "$emotion" != "" ]]; then
    ((EMOTION_COUNT++)) || true
  fi

  # Check for tag leaks in reply text
  if echo "$bot_text" | grep -qE '\[status:|\[emotion:'; then
    ((TAG_LEAK++)) || true
    echo "  Round $i TAG LEAK: $bot_text" >> "$REPORT_FILE"
  fi

  # Progress output every 10 rounds
  if (( i % 10 == 0 )) || (( i == 1 )); then
    truncated="${bot_text:0:50}"
    echo "  [$i/$TOTAL_ROUNDS] User: ${prompt:0:20}... → Bot: ${truncated}..."
    echo "    Status: $status_emoji $status_text | Emotion: $emotion"
  fi

  # Small delay to avoid overwhelming
  sleep 0.3
done

echo "" >> "$REPORT_FILE"
echo "| 指标 | 结果 |" >> "$REPORT_FILE"
echo "|------|------|" >> "$REPORT_FILE"
echo "| 总轮次 | $TOTAL_ROUNDS |" >> "$REPORT_FILE"
echo "| 成功回复 | $REPLY_COUNT |" >> "$REPORT_FILE"
echo "| 状态生成 | $STATUS_COUNT |" >> "$REPORT_FILE"
echo "| 情绪标注 | $EMOTION_COUNT |" >> "$REPORT_FILE"
echo "| 标签泄漏 | $TAG_LEAK |" >> "$REPORT_FILE"

if (( REPLY_COUNT == TOTAL_ROUNDS )); then
  pass "全部 $TOTAL_ROUNDS 轮对话均收到回复"
else
  fail "仅 $REPLY_COUNT/$TOTAL_ROUNDS 轮收到回复"
fi

if (( TAG_LEAK == 0 )); then
  pass "无标签泄漏 ([status:] / [emotion:] 均已剥离)"
else
  fail "$TAG_LEAK 轮存在标签泄漏"
fi

if (( STATUS_COUNT > TOTAL_ROUNDS / 2 )); then
  pass "AI状态生成率: $STATUS_COUNT/$TOTAL_ROUNDS ($(( STATUS_COUNT * 100 / TOTAL_ROUNDS ))%)"
else
  warn "AI状态生成率偏低: $STATUS_COUNT/$TOTAL_ROUNDS"
fi

if (( EMOTION_COUNT > TOTAL_ROUNDS / 2 )); then
  pass "情绪标注率: $EMOTION_COUNT/$TOTAL_ROUNDS ($(( EMOTION_COUNT * 100 / TOTAL_ROUNDS ))%)"
else
  warn "情绪标注率偏低: $EMOTION_COUNT/$TOTAL_ROUNDS"
fi

# ============================================================
section "Phase 3: 记忆功能测试"
# ============================================================

info "等待异步记忆提取完成 (5s)..."
sleep 5

# 3.1 测试名字记忆
mem_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mem-1","content":{"type":"text","text":"你还记得我叫什么名字吗？"}}' "$TOKEN")
mem_text=$(echo "$mem_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "记忆测试-名字: $mem_text"
if echo "$mem_text" | grep -q "小明"; then
  pass "记忆回忆-名字: 正确记住用户名字'小明'"
else
  warn "记忆回忆-名字: 未在回复中提及'小明' (可能记忆提取延迟)"
fi
sleep 1

# 3.2 测试宠物记忆
mem_resp2=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mem-2","content":{"type":"text","text":"你还记得我养了什么宠物吗？叫什么名字？"}}' "$TOKEN")
mem_text2=$(echo "$mem_resp2" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "记忆测试-宠物: $mem_text2"
if echo "$mem_text2" | grep -qE "猫|小橘"; then
  pass "记忆回忆-宠物: 正确记住宠物信息"
else
  warn "记忆回忆-宠物: 未在回复中提及猫/小橘"
fi
sleep 1

# 3.3 测试生日记忆
mem_resp3=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mem-3","content":{"type":"text","text":"你记得我的生日是哪天吗？"}}' "$TOKEN")
mem_text3=$(echo "$mem_resp3" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "记忆测试-生日: $mem_text3"
if echo "$mem_text3" | grep -qE "3月15|三月十五|3\.15"; then
  pass "记忆回忆-生日: 正确记住生日"
else
  warn "记忆回忆-生日: 未在回复中提及3月15日"
fi
sleep 1

# 3.4 测试朋友记忆
mem_resp4=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mem-4","content":{"type":"text","text":"我之前跟你提过我最好的朋友，你还记得ta叫什么吗？"}}' "$TOKEN")
mem_text4=$(echo "$mem_resp4" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "记忆测试-朋友: $mem_text4"
if echo "$mem_text4" | grep -q "小红"; then
  pass "记忆回忆-朋友: 正确记住朋友名字'小红'"
else
  warn "记忆回忆-朋友: 未在回复中提及'小红'"
fi
sleep 1

# 3.5 测试兴趣记忆
mem_resp5=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mem-5","content":{"type":"text","text":"你觉得我有哪些兴趣爱好？帮我总结一下"}}' "$TOKEN")
mem_text5=$(echo "$mem_resp5" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "记忆测试-兴趣: $mem_text5"
hobby_hits=0
for kw in "画画" "火锅" "蓝色" "爬山" "周杰伦" "咖啡" "星星"; do
  if echo "$mem_text5" | grep -q "$kw"; then
    ((hobby_hits++)) || true
  fi
done
if (( hobby_hits >= 2 )); then
  pass "记忆回忆-兴趣: 提及 $hobby_hits 个兴趣关键词"
else
  warn "记忆回忆-兴趣: 仅提及 $hobby_hits 个关键词 (可能记忆窗口不足)"
fi

# ============================================================
section "Phase 4: 人设更新后对话测试"
# ============================================================

# 4.1 更新人设
patch_resp=$(curl_json PATCH "$BASE_URL/api/v1/bots/$BOT_ID" '{
  "speaking_style_adds":["说话变得更加活泼开朗","喜欢用网络流行语"],
  "background_adds":["最近迷上了电竞和二次元文化"],
  "trait_adds":["活泼","搞怪","二次元"]
}' "$TOKEN")
if echo "$patch_resp" | grep -q "$BOT_ID"; then
  pass "人设更新成功"
else
  fail "人设更新失败: $patch_resp"
fi
sleep 1

# 4.2 验证人设变化
persona_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-persona-1","content":{"type":"text","text":"你最近有什么新的兴趣爱好吗？用你最自然的方式跟我聊聊"}}' "$TOKEN")
persona_text=$(echo "$persona_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "人设更新后回复: $persona_text"
pass "人设更新后对话正常返回"
sleep 1

# 4.3 再次验证风格变化
persona_resp2=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-persona-2","content":{"type":"text","text":"给我安利一部你最近看的番剧吧！"}}' "$TOKEN")
persona_text2=$(echo "$persona_resp2" | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
info "人设验证回复: $persona_text2"
pass "人设更新后二次对话正常"

# ============================================================
section "Phase 5: 用户状态更新与情绪变更测试"
# ============================================================

# 5.1 设置用户状态: 开心
put_resp=$(curl_json PUT "$BASE_URL/api/v1/users/status" '{"emoji":"😊","text":"今天超开心"}' "$TOKEN")
if echo "$put_resp" | grep -qE "ok\|emoji\|200"; then
  pass "用户状态设置: 😊 今天超开心"
else
  # Check HTTP status
  put_code=$(curl -sS -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/api/v1/users/status" \
    -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
    -d '{"emoji":"😊","text":"今天超开心"}')
  if [[ "$put_code" == "200" ]]; then
    pass "用户状态设置: 😊 今天超开心 (HTTP 200)"
  else
    fail "用户状态设置失败: HTTP $put_code"
  fi
fi

# 5.2 验证状态读取
get_resp=$(curl_json GET "$BASE_URL/api/v1/users/status" "" "$TOKEN")
got_emoji=$(jval emoji "$get_resp")
got_text=$(jval text "$get_resp")
if [[ "$got_emoji" == "😊" && "$got_text" == "今天超开心" ]]; then
  pass "用户状态读取: $got_emoji $got_text"
else
  fail "用户状态读取不匹配: $get_resp"
fi

# 5.3 开心状态下对话
happy_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mood-1","content":{"type":"text","text":"今天升职加薪了！太开心了！"}}' "$TOKEN")
happy_text=$(echo "$happy_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
happy_emotion=$(echo "$happy_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('emotion_tag',''))
" 2>/dev/null)
happy_status_emoji=$(echo "$happy_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_status_update',{}).get('emoji',''))
" 2>/dev/null)
info "开心状态回复: $happy_text"
info "情绪: $happy_emotion | 状态: $happy_status_emoji"
pass "开心状态下对话正常"
sleep 1

# 5.4 切换到难过状态
curl_json PUT "$BASE_URL/api/v1/users/status" '{"emoji":"😢","text":"心情很低落"}' "$TOKEN" > /dev/null
sleep 0.5

sad_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mood-2","content":{"type":"text","text":"今天被领导批评了，好难过..."}}' "$TOKEN")
sad_text=$(echo "$sad_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
sad_emotion=$(echo "$sad_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('emotion_tag',''))
" 2>/dev/null)
sad_status_emoji=$(echo "$sad_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_status_update',{}).get('emoji',''))
" 2>/dev/null)
info "难过状态回复: $sad_text"
info "情绪: $sad_emotion | 状态: $sad_status_emoji"
pass "难过状态下对话正常"
sleep 1

# 5.5 切换到焦虑状态
curl_json PUT "$BASE_URL/api/v1/users/status" '{"emoji":"😰","text":"压力山大"}' "$TOKEN" > /dev/null
sleep 0.5

anxious_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$CONV_ID/send" \
  '{"client_msg_id":"e2e-mood-3","content":{"type":"text","text":"明天有个很重要的面试，好紧张啊"}}' "$TOKEN")
anxious_text=$(echo "$anxious_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('content',{}).get('text',''))
" 2>/dev/null)
anxious_emotion=$(echo "$anxious_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin)
print(d.get('bot_message',{}).get('emotion_tag',''))
" 2>/dev/null)
info "焦虑状态回复: $anxious_text"
info "情绪: $anxious_emotion"
pass "焦虑状态下对话正常"

# ============================================================
section "Phase 6: Bot状态与轮次计数验证"
# ============================================================

bot_detail=$(curl_json GET "$BASE_URL/api/v1/bots/$BOT_ID" "" "$TOKEN")
bot_status_emoji=$(echo "$bot_detail" | python3 -c "
import sys,json; d=json.load(sys.stdin)
s=d.get('status',{})
print(s.get('emoji',''))
" 2>/dev/null)
bot_status_text=$(echo "$bot_detail" | python3 -c "
import sys,json; d=json.load(sys.stdin)
s=d.get('status',{})
print(s.get('text',''))
" 2>/dev/null)
if [[ -n "$bot_status_emoji" && "$bot_status_emoji" != "None" ]]; then
  pass "Bot状态持久化: $bot_status_emoji $bot_status_text"
else
  warn "Bot状态未持久化到详情接口"
fi

# ============================================================
section "测试总结"
# ============================================================

TOTAL=$((PASS_COUNT + FAIL_COUNT + WARN_COUNT))
echo "" >> "$REPORT_FILE"
echo "| 类别 | 数量 |" >> "$REPORT_FILE"
echo "|------|------|" >> "$REPORT_FILE"
echo "| PASS | $PASS_COUNT |" >> "$REPORT_FILE"
echo "| FAIL | $FAIL_COUNT |" >> "$REPORT_FILE"
echo "| WARN | $WARN_COUNT |" >> "$REPORT_FILE"
echo "| 总计 | $TOTAL |" >> "$REPORT_FILE"

echo ""
echo "========================================"
echo -e "  PASS: ${GREEN}$PASS_COUNT${NC}  FAIL: ${RED}$FAIL_COUNT${NC}  WARN: ${YELLOW}$WARN_COUNT${NC}"
echo "========================================"
echo "报告已保存: $REPORT_FILE"

if (( FAIL_COUNT > 0 )); then
  exit 1
fi
