#!/bin/bash
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
NICKNAME="smoke_user"

generate_phone() {
  local epoch rand
  epoch="$(date +%s)"
  rand="$(printf "%04d" $((RANDOM % 10000)))"
  printf "13%09d" "$(((epoch + 10#$rand) % 1000000000))"
}

PHONE="${2:-$(generate_phone)}"

extract_json_value() {
  local key="$1"
  local json="$2"
  echo "$json" | sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n 1
}

curl_json() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth="${4:-}"

  if [[ -n "$body" ]]; then
    if [[ -n "$auth" ]]; then
      curl -sS -X "$method" "$url" -H "Content-Type: application/json" -H "Authorization: Bearer $auth" -d "$body"
    else
      curl -sS -X "$method" "$url" -H "Content-Type: application/json" -d "$body"
    fi
  else
    if [[ -n "$auth" ]]; then
      curl -sS -X "$method" "$url" -H "Authorization: Bearer $auth"
    else
      curl -sS -X "$method" "$url"
    fi
  fi
}

echo "[1/8] send sms"
_=$(curl_json POST "$BASE_URL/api/v1/auth/sms/send" "{\"phone\":\"$PHONE\",\"purpose\":\"register\"}")

echo "[2/8] register"
register_resp=$(curl_json POST "$BASE_URL/api/v1/auth/register" "{\"phone\":\"$PHONE\",\"sms_code\":\"123456\",\"nickname\":\"$NICKNAME\"}")
token=$(extract_json_value "access_token" "$register_resp")
if [[ -z "$token" ]]; then
  echo "register failed: $register_resp"
  exit 1
fi

echo "[3/8] create bot"
bot_resp=$(curl_json POST "$BASE_URL/api/v1/bots" "{\"name\":\"测试角色\",\"summary\":\"温柔陪伴\",\"speaking_style\":\"温柔\",\"background\":\"江南\",\"traits\":[\"体贴\"],\"gender\":\"female\"}" "$token")
bot_id=$(extract_json_value "id" "$bot_resp")
if [[ -z "$bot_id" ]]; then
  echo "create bot failed: $bot_resp"
  exit 1
fi

echo "[4/8] list conversations"
conv_resp=$(curl_json GET "$BASE_URL/api/v1/conversations" "" "$token")
conv_id=$(extract_json_value "id" "$conv_resp")
if [[ -z "$conv_id" ]]; then
  echo "list conversations failed: $conv_resp"
  exit 1
fi

echo "[5/8] send message"
send_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$conv_id/send" "{\"client_msg_id\":\"smoke-msg-1\",\"content\":{\"type\":\"text\",\"text\":\"你好\"}}" "$token")
if ! echo "$send_resp" | grep -q '"ack"'; then
  echo "send message failed: $send_resp"
  exit 1
fi

echo "[6/8] proactive trigger"
pro_resp=$(curl_json POST "$BASE_URL/api/v1/gateway/test/proactive" "{\"bot_id\":\"$bot_id\",\"conversation_id\":\"$conv_id\",\"trigger_type\":\"manual\"}" "$token")
if ! echo "$pro_resp" | grep -q '"should_send"'; then
  echo "proactive failed: $pro_resp"
  exit 1
fi

echo "[7/8] health"
health_resp=$(curl_json GET "$BASE_URL/api/v1/gateway/health")
if ! echo "$health_resp" | grep -q '"status"'; then
  echo "health failed: $health_resp"
  exit 1
fi

echo "[8/8] metrics"
metrics_resp=$(curl_json GET "$BASE_URL/api/v1/gateway/metrics")
if ! echo "$metrics_resp" | grep -q '"requests_total"'; then
  echo "metrics failed: $metrics_resp"
  exit 1
fi

echo "smoke test passed"
