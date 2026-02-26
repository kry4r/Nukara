#!/bin/bash
set -o pipefail

BASE_URL="${1:-http://localhost:8080}"
REPORT="/tmp/nukara_memory_test_$(date +%Y%m%d_%H%M%S).md"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
PASS_COUNT=0; FAIL_COUNT=0; WARN_COUNT=0

pass() { ((PASS_COUNT++)) || true; echo -e "${GREEN}[PASS]${NC} $1"; echo "- [PASS] $1" >> "$REPORT"; }
fail() { ((FAIL_COUNT++)) || true; echo -e "${RED}[FAIL]${NC} $1"; echo "- [FAIL] $1" >> "$REPORT"; }
warn() { ((WARN_COUNT++)) || true; echo -e "${YELLOW}[WARN]${NC} $1"; echo "- [WARN] $1" >> "$REPORT"; }
info() { echo -e "  ${CYAN}→${NC} $1"; }
section() { echo -e "\n=== $1 ==="; echo -e "\n## $1\n" >> "$REPORT"; }

jval() { echo "$2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('$1',''))" 2>/dev/null; }

curl_json() {
  local method="$1" url="$2" body="${3:-}" auth="${4:-}"
  local args=(-sS -X "$method" "$url" -H "Content-Type: application/json")
  [[ -n "$auth" ]] && args+=(-H "Authorization: Bearer $auth")
  [[ -n "$body" ]] && args+=(-d "$body")
  curl "${args[@]}" 2>/dev/null
}

generate_phone() { printf "139%08d" "$((RANDOM * RANDOM % 100000000))"; }

send_msg() {
  local conv_id="$1" text="$2" token="$3" tag="$4"
  local resp
  resp=$(curl -sS --max-time 60 -X POST "$BASE_URL/api/v1/conversations/$conv_id/send" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $token" \
    -d "{\"client_msg_id\":\"mem-$tag\",\"content\":{\"type\":\"text\",\"text\":\"$text\"}}" 2>/dev/null) || true
  local bot_text
  bot_text=$(echo "$resp" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    print(d.get('bot_message',{}).get('content',{}).get('text',''))
except: print('')
" 2>/dev/null) || true
  echo "${bot_text:-}"
}

# --- Init report ---
cat > "$REPORT" << 'EOF'
# Nukara 记忆增强功能测试报告

EOF
echo "**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')" >> "$REPORT"
echo "**测试环境**: $BASE_URL" >> "$REPORT"
echo "" >> "$REPORT"

# ============================================================
section "Phase 1: 环境准备"
# ============================================================

PHONE=$(generate_phone)
info "测试手机号: $PHONE"

sms_resp=$(curl_json POST "$BASE_URL/api/v1/auth/sms/send" "{\"phone\":\"$PHONE\",\"purpose\":\"register\"}")
sleep 0.5
SMS_CODE=$(docker logs configs-gateway-1 --tail 20 2>&1 | grep "\[SMS\].*phone=$PHONE" | tail -1 | sed 's/.*code=//')
[[ -z "$SMS_CODE" ]] && SMS_CODE="123456"
info "验证码: $SMS_CODE"

reg_resp=$(curl_json POST "$BASE_URL/api/v1/auth/register" "{\"phone\":\"$PHONE\",\"sms_code\":\"$SMS_CODE\",\"nickname\":\"mem_tester\"}")
TOKEN=$(jval access_token "$reg_resp")
if [[ -n "$TOKEN" ]]; then
  pass "用户注册成功"
else
  fail "用户注册失败: $reg_resp"; exit 1
fi

bot_resp=$(curl_json POST "$BASE_URL/api/v1/bots" '{
  "name":"苏子衿","summary":"温柔体贴的江南女子","speaking_style":"温柔婉约",
  "background":"苏州人，学习古典文学","traits":["温柔","体贴"],"gender":"female"
}' "$TOKEN")
BOT_ID=$(jval id "$bot_resp")
if [[ -n "$BOT_ID" ]]; then
  pass "创建机器人: $BOT_ID"
else
  fail "创建机器人失败"; exit 1
fi

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
  fail "获取会话失败"; exit 1
fi

# ============================================================
section "Phase 2: 记忆植入 (100轮对话)"
# ============================================================

FACTS=(
  # --- 基本信息 (1-10) ---
  "你好，我叫张三，很高兴认识你"
  "我今年25岁，在北京工作"
  "我养了一只猫，叫小橘，是橘猫"
  "我的生日是3月15号"
  "我最好的朋友叫李四，我们从小一起长大"
  "我喜欢吃火锅，尤其是四川麻辣火锅"
  "我最近在学Python编程"
  "我喜欢听周杰伦的歌，最喜欢《晴天》"
  "我的梦想是开一家咖啡店"
  "我不喜欢吃香菜，闻到就想跑"
  # --- 家庭 (11-20) ---
  "我爸爸是个数学老师，妈妈是护士"
  "我有一个妹妹叫张小美，比我小3岁"
  "我妹妹在上海读研究生，学的是心理学"
  "我爷爷以前是个木匠，手艺特别好"
  "我奶奶做的红烧肉是我吃过最好吃的"
  "我们家过年一定会包饺子，这是传统"
  "我爸特别喜欢下象棋，经常在公园跟人对弈"
  "我妈最近迷上了广场舞，每天晚上都去跳"
  "我小时候是在河北老家长大的，后来才搬到北京"
  "我家老房子门前有棵大槐树，夏天特别凉快"
  # --- 工作与学习 (21-30) ---
  "我在一家互联网公司做产品经理"
  "我们公司在中关村，离地铁站很近"
  "我的领导姓王，人挺好的，就是开会太多"
  "我最近在考虑要不要跳槽去大厂"
  "我大学学的是计算机科学，在武汉大学"
  "大学时候我参加了辩论队，拿过校级冠军"
  "我的毕业论文写的是推荐系统相关的"
  "我觉得AI会改变很多行业，所以想多学点"
  "我每天通勤要一个半小时，地铁转公交"
  "我工位旁边有个同事叫小王，我们经常一起吃午饭"
  # --- 兴趣爱好 (31-40) ---
  "我周末喜欢去爬山，最喜欢香山"
  "我最近开始学吉他了，买了一把雅马哈的"
  "我喜欢看科幻电影，最喜欢《星际穿越》"
  "我也喜欢看动漫，最喜欢《进击的巨人》"
  "我喜欢摄影，用的是索尼A7M4"
  "我最近在读《三体》，觉得刘慈欣太厉害了"
  "我喜欢打篮球，虽然打得不太好"
  "我是湖人队的球迷，最喜欢科比"
  "我偶尔会玩游戏，最近在玩原神"
  "我喜欢骑自行车，周末有时候会骑行去郊区"
  # --- 饮食偏好 (41-50) ---
  "除了火锅，我也很喜欢吃烧烤"
  "我早餐一般吃豆浆油条，很传统"
  "我喜欢喝咖啡，每天至少一杯美式"
  "我不太能吃辣，但是火锅例外"
  "我最喜欢的水果是芒果，但是吃多了会过敏"
  "我喜欢吃日料，尤其是三文鱼刺身"
  "我不喝酒，一喝就脸红"
  "我最近在尝试自己做饭，学会了番茄炒蛋"
  "我特别喜欢吃我奶奶做的糖醋排骨"
  "我觉得北京烤鸭还是全聚德的最正宗"
  # --- 旅行经历 (51-60) ---
  "我去年去了一趟云南，丽江古城特别美"
  "我最想去的国家是日本，想去看樱花"
  "我去过一次三亚，在那里学了浮潜"
  "我大学的时候和李四一起骑行去了西藏"
  "我去过成都，那里的大熊猫太可爱了"
  "我计划明年去一趟欧洲，想看看巴黎铁塔"
  "我喜欢坐火车旅行，觉得沿途风景很美"
  "我在厦门的时候住了一家很棒的民宿"
  "我去过长白山看天池，可惜那天有雾没看到"
  "我最难忘的旅行是和大学室友去青海湖"
  # --- 生活习惯 (61-70) ---
  "我是个夜猫子，经常熬夜到一两点"
  "我每天早上会跑步半小时，坚持了三个月了"
  "我有记日记的习惯，用的是一个叫Day One的app"
  "我睡觉的时候喜欢听白噪音，比如下雨声"
  "我周末喜欢睡懒觉，经常睡到中午"
  "我有轻微的强迫症，东西一定要摆整齐"
  "我洗澡的时候喜欢唱歌"
  "我最近在尝试冥想，每天十分钟"
  "我喜欢在下雨天窝在家里看书喝茶"
  "我有收集冰箱贴的习惯，每去一个地方都会买"
  # --- 情感与性格 (71-80) ---
  "我觉得自己是个比较内向的人，但跟熟人话很多"
  "我有时候会莫名其妙地感到焦虑"
  "我很重视朋友之间的信任"
  "我不太擅长拒绝别人，有时候会为难自己"
  "我觉得真诚是最重要的品质"
  "我小时候很怕黑，现在好多了"
  "我是个很念旧的人，手机里存了很多老照片"
  "我遇到压力大的时候喜欢一个人去散步"
  "我觉得人生最重要的是做自己喜欢的事"
  "我最近在思考要不要养一只狗陪小橘"
  # --- 童年回忆 (81-85) ---
  "我小时候最喜欢看《西游记》，孙悟空是我的偶像"
  "我小学的时候参加过书法比赛，拿了二等奖"
  "我记得小时候夏天最喜欢吃冰棍，五毛钱一根"
  "我和李四小时候经常一起去河边抓鱼"
  "我初中的时候开始近视，现在400度"
  # --- 未来规划 (86-90) ---
  "除了咖啡店，我也想过以后去乡下开个民宿"
  "我打算明年考个PMP证书"
  "我希望30岁之前能攒够首付买房"
  "我想学日语，这样去日本旅游就方便了"
  "我以后想养两只猫，给小橘找个伴"
  # --- 近期动态 (91-95) ---
  "我昨天加班到很晚，有点累"
  "我最近在追一部韩剧叫《鱿鱼游戏》"
  "我上周末和李四去了一家新开的密室逃脱"
  "我最近体检发现有点缺维生素D，医生让我多晒太阳"
  "我刚给小橘买了一个新的猫爬架，它特别喜欢"
  # --- 随机细节 (96-100) ---
  "我的手机是iPhone 15 Pro，拍照确实不错"
  "我最喜欢的颜色是蓝色，衣柜里一半都是蓝色的"
  "我有一个幸运数字是7，从小就觉得7很特别"
  "我最怕的动物是蛇，看到图片都会起鸡皮疙瘩"
  "我觉得和你聊天很开心，希望你能记住这些关于我的事"
)

for i in "${!FACTS[@]}"; do
  idx=$((i + 1))
  reply=$(send_msg "$CONV_ID" "${FACTS[$i]}" "$TOKEN" "fact-$idx")
  truncated="${reply:0:60}"
  total=${#FACTS[@]}
  info "[$idx/$total] → ${truncated}..."
  sleep 1
done
pass "${#FACTS[@]}轮记忆植入对话完成"

# ============================================================
section "Phase 3: 等待记忆提取 (60s)"
# ============================================================

info "等待异步记忆提取完成 (100轮需要更长时间)..."
sleep 60
pass "等待完成"

# ============================================================
section "Phase 4: 记忆回忆测试"
# ============================================================

# 4.1 名字
reply=$(send_msg "$CONV_ID" "你还记得我叫什么名字吗？" "$TOKEN" "recall-name")
info "名字回忆: ${reply:0:80}..."
if echo "$reply" | grep -q "张三"; then
  pass "记忆-名字: 正确记住'张三'"
else
  warn "记忆-名字: 未提及'张三'"
fi
sleep 1

# 4.2 宠物
reply=$(send_msg "$CONV_ID" "你记得我养了什么宠物吗？叫什么？" "$TOKEN" "recall-pet")
info "宠物回忆: ${reply:0:80}..."
if echo "$reply" | grep -qE "猫|小橘"; then
  pass "记忆-宠物: 正确记住宠物信息"
else
  warn "记忆-宠物: 未提及猫/小橘"
fi
sleep 1

# 4.3 生日
reply=$(send_msg "$CONV_ID" "你记得我的生日是哪天吗？" "$TOKEN" "recall-bday")
info "生日回忆: ${reply:0:80}..."
if echo "$reply" | grep -qE "3月15|三月十五"; then
  pass "记忆-生日: 正确记住3月15日"
else
  warn "记忆-生日: 未提及3月15日"
fi
sleep 1

# 4.4 朋友
reply=$(send_msg "$CONV_ID" "我之前提过我最好的朋友，你还记得叫什么吗？" "$TOKEN" "recall-friend")
info "朋友回忆: ${reply:0:80}..."
if echo "$reply" | grep -q "李四"; then
  pass "记忆-朋友: 正确记住'李四'"
else
  warn "记忆-朋友: 未提及'李四'"
fi
sleep 1

# 4.5 兴趣综合
reply=$(send_msg "$CONV_ID" "帮我总结一下你记得的关于我的兴趣爱好" "$TOKEN" "recall-hobby")
info "兴趣回忆: ${reply:0:120}..."
hobby_hits=0
for kw in "火锅" "Python" "编程" "周杰伦" "咖啡" "晴天"; do
  if echo "$reply" | grep -q "$kw"; then
    ((hobby_hits++)) || true
  fi
done
if (( hobby_hits >= 2 )); then
  pass "记忆-兴趣: 提及 $hobby_hits 个关键词"
else
  warn "记忆-兴趣: 仅提及 $hobby_hits 个关键词"
fi
sleep 1

# 4.6 负面偏好
reply=$(send_msg "$CONV_ID" "你记得我不喜欢吃什么吗？" "$TOKEN" "recall-dislike")
info "负面偏好: ${reply:0:80}..."
if echo "$reply" | grep -q "香菜"; then
  pass "记忆-负面偏好: 正确记住不喜欢香菜"
else
  warn "记忆-负面偏好: 未提及香菜"
fi

# ============================================================
section "Phase 5: 记忆更新与矛盾测试"
# ============================================================

# 5.1 更新信息
reply=$(send_msg "$CONV_ID" "对了，我最近换工作了，现在在上海工作" "$TOKEN" "update-1")
info "更新回复: ${reply:0:60}..."
sleep 2

# 5.2 验证更新
reply=$(send_msg "$CONV_ID" "你记得我现在在哪个城市工作吗？" "$TOKEN" "verify-update")
info "更新验证: ${reply:0:80}..."
if echo "$reply" | grep -q "上海"; then
  pass "记忆更新: 正确记住新工作地点'上海'"
else
  warn "记忆更新: 未提及'上海'"
fi
sleep 1

# 5.3 矛盾信息
reply=$(send_msg "$CONV_ID" "其实我现在不学Python了，改学Go了" "$TOKEN" "contradict-1")
info "矛盾回复: ${reply:0:60}..."
sleep 2

reply=$(send_msg "$CONV_ID" "你记得我在学什么编程语言吗？" "$TOKEN" "verify-contradict")
info "矛盾验证: ${reply:0:80}..."
if echo "$reply" | grep -q "Go"; then
  pass "记忆矛盾处理: 正确更新为'Go'"
elif echo "$reply" | grep -q "Python"; then
  warn "记忆矛盾处理: 仍记为Python（旧信息未更新）"
else
  warn "记忆矛盾处理: 未明确提及编程语言"
fi

# ============================================================
section "Phase 6: 印象系统测试"
# ============================================================

# 发送5轮触发印象更新
info "发送5轮对话触发印象更新..."
IMPRESSION_MSGS=(
  "我最近心情不太好，工作压力很大"
  "谢谢你的安慰，和你聊天让我开心很多"
  "你觉得我是什么样的人？"
  "我觉得你真的很温柔"
  "帮我总结一下你对我的整体印象吧"
)
for i in "${!IMPRESSION_MSGS[@]}"; do
  reply=$(send_msg "$CONV_ID" "${IMPRESSION_MSGS[$i]}" "$TOKEN" "impr-$((i+1))")
  if (( i == 4 )); then
    info "印象总结: ${reply:0:120}..."
    impr_hits=0
    for kw in "张三" "猫" "火锅" "编程" "咖啡" "周杰伦" "北京" "上海"; do
      if echo "$reply" | grep -q "$kw"; then
        ((impr_hits++)) || true
      fi
    done
    if (( impr_hits >= 2 )); then
      pass "印象系统: 总结中包含 $impr_hits 个用户特征"
    else
      warn "印象系统: 总结中仅包含 $impr_hits 个用户特征"
    fi
  fi
  sleep 1
done

# ============================================================
section "Phase 7: 直接记忆图谱验证"
# ============================================================

# Check nanobot memory DB directly
info "检查nanobot记忆图谱..."
MEM_STATS=$(docker exec configs-nanobot-1 python3 -c "
import sqlite3, json, sys, glob
dbs = glob.glob('/data/workspace/robots/default/memory/*_$BOT_ID/graph.db')
if not dbs:
    dbs = glob.glob('/data/workspace/robots/*/memory/*/graph.db')
if not dbs:
    print(json.dumps({'error': 'no graph.db found'}))
    sys.exit(0)
db = sorted(dbs)[-1]
conn = sqlite3.connect(db)
conn.row_factory = sqlite3.Row
nodes = conn.execute('SELECT COUNT(*) as c FROM nodes').fetchone()['c']
edges = conn.execute('SELECT COUNT(*) as c FROM edges').fetchone()['c']
events = conn.execute(\"SELECT COUNT(*) as c FROM nodes WHERE type='event'\").fetchone()['c']
concepts = conn.execute(\"SELECT COUNT(*) as c FROM nodes WHERE type='concept'\").fetchone()['c']
entities = conn.execute(\"SELECT COUNT(*) as c FROM nodes WHERE type='entity'\").fetchone()['c']
with_body = conn.execute(\"SELECT COUNT(*) as c FROM nodes WHERE body != ''\").fetchone()['c']
print(json.dumps({
    'nodes': nodes, 'edges': edges,
    'events': events, 'concepts': concepts, 'entities': entities,
    'with_body': with_body, 'db_path': db
}))
conn.close()
" 2>&1)

info "记忆图谱: $MEM_STATS"

node_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('nodes',0))" 2>/dev/null)
edge_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('edges',0))" 2>/dev/null)
body_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('with_body',0))" 2>/dev/null)

if [[ -n "$node_count" ]] && (( node_count > 0 )); then
  pass "记忆图谱: $node_count 个节点, $edge_count 条边"
else
  fail "记忆图谱: 无节点 (记忆提取可能未工作)"
fi

if [[ -n "$body_count" ]] && (( body_count > 0 )); then
  pass "Body字段: $body_count 个节点包含详细内容"
else
  warn "Body字段: 无节点使用body字段"
fi

# Check node types
events_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('events',0))" 2>/dev/null)
concepts_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('concepts',0))" 2>/dev/null)
entities_count=$(echo "$MEM_STATS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('entities',0))" 2>/dev/null)
info "节点类型: event=$events_count, concept=$concepts_count, entity=$entities_count"

if (( concepts_count > 0 )); then
  pass "概念提升: $concepts_count 个关键词被提升为concept节点"
else
  warn "概念提升: 无concept节点 (关键词复用不足或提取未触发)"
fi

# ============================================================
section "测试总结"
# ============================================================

TOTAL=$((PASS_COUNT + FAIL_COUNT + WARN_COUNT))
echo "" >> "$REPORT"
echo "| 类别 | 数量 |" >> "$REPORT"
echo "|------|------|" >> "$REPORT"
echo "| PASS | $PASS_COUNT |" >> "$REPORT"
echo "| FAIL | $FAIL_COUNT |" >> "$REPORT"
echo "| WARN | $WARN_COUNT |" >> "$REPORT"
echo "| 总计 | $TOTAL |" >> "$REPORT"

echo ""
echo "========================================"
echo -e "  PASS: ${GREEN}$PASS_COUNT${NC}  FAIL: ${RED}$FAIL_COUNT${NC}  WARN: ${YELLOW}$WARN_COUNT${NC}"
echo "========================================"
echo "报告: $REPORT"

if (( FAIL_COUNT > 0 )); then exit 1; fi
