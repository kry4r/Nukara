#!/bin/bash
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
EMAIL_INPUT="${2:-}"
EMAIL_DOMAIN="${NUKARA_TEST_EMAIL_DOMAIN:-nukara.local}"
NICKNAME="ios_contract_user"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

generate_email() {
  local epoch rand
  epoch="$(date +%s)"
  rand="$(printf "%04d" $((RANDOM % 10000)))"
  printf "ios_contract_%s_%s@%s" "$epoch" "$rand" "$EMAIL_DOMAIN"
}

EMAIL="$EMAIL_INPUT"
if [[ -z "$EMAIL" ]]; then
  EMAIL="$(generate_email)"
fi

extract_json_value() {
  local key="$1"
  local json="$2"
  echo "$json" | sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n 1
}

request_json() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth="${4:-}"
  local out_file="$TMP_DIR/resp.json"
  local code

  if [[ -n "$body" ]]; then
    if [[ -n "$auth" ]]; then
      code=$(curl -sS -o "$out_file" -w "%{http_code}" -X "$method" "$url" -H "Content-Type: application/json" -H "Authorization: Bearer $auth" -d "$body")
    else
      code=$(curl -sS -o "$out_file" -w "%{http_code}" -X "$method" "$url" -H "Content-Type: application/json" -d "$body")
    fi
  else
    if [[ -n "$auth" ]]; then
      code=$(curl -sS -o "$out_file" -w "%{http_code}" -X "$method" "$url" -H "Authorization: Bearer $auth")
    else
      code=$(curl -sS -o "$out_file" -w "%{http_code}" -X "$method" "$url")
    fi
  fi

  local response
  response="$(cat "$out_file")"
  if [[ "$code" -lt 200 || "$code" -ge 300 ]]; then
    echo "request failed: $method $url status=$code body=$response"
    exit 1
  fi
  echo "$response"
}

read_email_code() {
  local purpose="$1"
  docker logs configs-gateway-1 --tail 50 2>&1 | grep "\[EMAIL\].*email=$EMAIL.*purpose=$purpose" | tail -1 | sed 's/.*code=//'
}

echo "[1/19] auth email register"
request_json POST "$BASE_URL/api/v1/auth/email/send" "{\"email\":\"$EMAIL\",\"purpose\":\"register\"}" >/dev/null
sleep 0.5
REGISTER_CODE="$(read_email_code register)"
if [[ -z "$REGISTER_CODE" ]]; then
  echo "register email code missing from logs; make sure SMTP is configured and gateway logs are reachable"
  exit 1
fi

echo "[2/19] auth register"
register_resp=$(request_json POST "$BASE_URL/api/v1/auth/register" "{\"email\":\"$EMAIL\",\"email_code\":\"$REGISTER_CODE\",\"nickname\":\"$NICKNAME\"}")
token=$(extract_json_value "access_token" "$register_resp")
if [[ -z "$token" ]]; then
  echo "register token missing: $register_resp"
  exit 1
fi

echo "[3/19] auth email login"
request_json POST "$BASE_URL/api/v1/auth/email/send" "{\"email\":\"$EMAIL\",\"purpose\":\"login\"}" >/dev/null
sleep 0.5
LOGIN_CODE="$(read_email_code login)"
if [[ -z "$LOGIN_CODE" ]]; then
  echo "login email code missing from logs; make sure SMTP is configured and gateway logs are reachable"
  exit 1
fi

echo "[4/19] auth login"
request_json POST "$BASE_URL/api/v1/auth/login" "{\"email\":\"$EMAIL\",\"email_code\":\"$LOGIN_CODE\"}" >/dev/null

echo "[5/19] bots list"
request_json GET "$BASE_URL/api/v1/bots" "" "$token" >/dev/null

echo "[6/19] bots create"
bot_resp=$(request_json POST "$BASE_URL/api/v1/bots" "{\"name\":\"合约测试角色\",\"summary\":\"测试摘要\",\"speaking_style\":\"温柔\",\"background\":\"城市\",\"traits\":[\"细心\"],\"gender\":\"female\"}" "$token")
bot_id=$(extract_json_value "id" "$bot_resp")
if [[ -z "$bot_id" ]]; then
  echo "bot id missing: $bot_resp"
  exit 1
fi

echo "[7/19] bots get by id"
request_json GET "$BASE_URL/api/v1/bots/$bot_id" "" "$token" >/dev/null

echo "[8/19] bots patch + legacy patch"
request_json PATCH "$BASE_URL/api/v1/bots/$bot_id" "{\"speaking_style_adds\":[\"幽默\"],\"background_adds\":[\"旅行\"],\"trait_adds\":[\"耐心\"],\"gender\":\"female\"}" "$token" >/dev/null
request_json PATCH "$BASE_URL/api/v1/bots/$bot_id/persona" "{\"speaking_style_adds\":[\"冷静\"],\"background_adds\":[\"阅读\"],\"trait_adds\":[\"真诚\"],\"gender\":\"female\"}" "$token" >/dev/null

echo "[9/19] conversations list"
conv_resp=$(request_json GET "$BASE_URL/api/v1/conversations" "" "$token")
conv_id=$(extract_json_value "id" "$conv_resp")
if [[ -z "$conv_id" ]]; then
  echo "conversation id missing: $conv_resp"
  exit 1
fi

echo "[10/19] conversations messages"
request_json GET "$BASE_URL/api/v1/conversations/$conv_id/messages?limit=100" "" "$token" >/dev/null

echo "[11/19] conversations send text"
send_text_resp=$(request_json POST "$BASE_URL/api/v1/conversations/$conv_id/send" "{\"client_msg_id\":\"ios-contract-text\",\"content\":{\"type\":\"text\",\"text\":\"你好\"}}" "$token")
if ! echo "$send_text_resp" | grep -q '"ack"'; then
  echo "send text invalid response: $send_text_resp"
  exit 1
fi

echo "[12/19] conversations send image"
request_json POST "$BASE_URL/api/v1/conversations/$conv_id/send" "{\"client_msg_id\":\"ios-contract-image\",\"content\":{\"type\":\"image\",\"image_base64\":\"aGVsbG8=\"}}" "$token" >/dev/null

echo "[13/19] conversations send location"
request_json POST "$BASE_URL/api/v1/conversations/$conv_id/send" "{\"client_msg_id\":\"ios-contract-location\",\"content\":{\"type\":\"location\",\"latitude\":39.9042,\"longitude\":116.4074,\"name\":\"测试位置\"}}" "$token" >/dev/null

echo "[14/19] conversations mark read + legacy read"
request_json POST "$BASE_URL/api/v1/conversations/$conv_id/mark-read" "" "$token" >/dev/null
request_json POST "$BASE_URL/api/v1/conversations/$conv_id/read" "" "$token" >/dev/null

echo "[15/19] users device token"
request_json POST "$BASE_URL/api/v1/users/device-token" "{\"device_token\":\"dummy-apns-token\",\"platform\":\"ios\"}" "$token" >/dev/null

echo "[16/19] users notification settings"
request_json GET "$BASE_URL/api/v1/users/notification-settings" "" "$token" >/dev/null
request_json PUT "$BASE_URL/api/v1/users/notification-settings" "{\"proactive_enabled\":true,\"dnd_start\":\"22:00\",\"dnd_end\":\"08:00\",\"frequency\":\"normal\"}" "$token" >/dev/null

echo "[17/19] proactive logs + trigger"
request_json GET "$BASE_URL/api/v1/proactive/logs?limit=10" "" "$token" >/dev/null
request_json POST "$BASE_URL/api/v1/gateway/test/proactive" "{\"bot_id\":\"$bot_id\",\"conversation_id\":\"$conv_id\",\"trigger_type\":\"manual\"}" "$token" >/dev/null

echo "[18/19] gateway test endpoints"
request_json POST "$BASE_URL/api/v1/gateway/test/chat" "{\"bot_id\":\"$bot_id\",\"message\":\"测试消息\",\"debug\":true}" "$token" >/dev/null
request_json GET "$BASE_URL/api/v1/gateway/metrics" >/dev/null

echo "[19/19] gateway health"
request_json GET "$BASE_URL/api/v1/gateway/health" >/dev/null

echo "ios api contract check passed"
