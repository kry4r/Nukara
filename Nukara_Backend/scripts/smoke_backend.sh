#!/bin/bash
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
NICKNAME="smoke_user"
EMAIL_DOMAIN="${NUKARA_TEST_EMAIL_DOMAIN:-nukara.local}"
SMOKE_GATEWAY_CONTAINER="${NUKARA_SMOKE_GATEWAY_CONTAINER:-configs-gateway-1}"
USE_TEST_SESSION="${NUKARA_SMOKE_USE_TEST_SESSION:-0}"

generate_email() {
  local epoch rand
  epoch="$(date +%s)"
  rand="$(printf "%04d" $((RANDOM % 10000)))"
  printf "smoke_%s_%s@%s" "$epoch" "$rand" "$EMAIL_DOMAIN"
}

EMAIL="${2:-$(generate_email)}"

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

issue_test_session() {
  local session_resp token
  session_resp=$(curl_json POST "$BASE_URL/api/v1/gateway/test/session" "{\"email\":\"$EMAIL\",\"nickname\":\"$NICKNAME\"}")
  token=$(extract_json_value "access_token" "$session_resp")
  if [[ -z "$token" ]]; then
    echo "test session failed: $session_resp"
    exit 1
  fi
  printf '%s' "$token"
}

acquire_session_token() {
  local email_resp email_code register_resp token

  if [[ "$USE_TEST_SESSION" = "1" ]]; then
    issue_test_session
    return 0
  fi

  email_resp=$(curl_json POST "$BASE_URL/api/v1/auth/email/send" "{\"email\":\"$EMAIL\",\"purpose\":\"register\"}")
  if echo "$email_resp" | grep -q '"success"'; then
    sleep 0.5
    email_code=$(docker logs "$SMOKE_GATEWAY_CONTAINER" --tail 50 2>&1 | grep "\[EMAIL\].*email=$EMAIL" | tail -1 | sed 's/.*code=//')
    if [[ -z "$email_code" ]]; then
      echo "failed to read email code from logs; make sure SMTP is configured and gateway logs are reachable (container=$SMOKE_GATEWAY_CONTAINER)"
      exit 1
    fi

    register_resp=$(curl_json POST "$BASE_URL/api/v1/auth/register" "{\"email\":\"$EMAIL\",\"email_code\":\"$email_code\",\"nickname\":\"$NICKNAME\"}")
    token=$(extract_json_value "access_token" "$register_resp")
    if [[ -z "$token" ]]; then
      echo "register failed: $register_resp"
      exit 1
    fi
    printf '%s' "$token"
    return 0
  fi

  if echo "$email_resp" | grep -q 'smtp not configured'; then
    echo "smtp not configured; falling back to gateway test session" >&2
    issue_test_session
    return 0
  fi

  echo "send email code failed: $email_resp"
  exit 1
}

echo "[1/7] acquire session"
token="$(acquire_session_token)"

echo "[2/7] create bot"
bot_resp=$(curl_json POST "$BASE_URL/api/v1/bots" "{\"name\":\"测试角色\",\"summary\":\"温柔陪伴\",\"speaking_style\":\"温柔\",\"background\":\"江南\",\"traits\":[\"体贴\"],\"gender\":\"female\"}" "$token")
bot_id=$(extract_json_value "id" "$bot_resp")
if [[ -z "$bot_id" ]]; then
  echo "create bot failed: $bot_resp"
  exit 1
fi

echo "[3/7] list conversations"
conv_resp=$(curl_json GET "$BASE_URL/api/v1/conversations" "" "$token")
conv_id=$(extract_json_value "id" "$conv_resp")
if [[ -z "$conv_id" ]]; then
  echo "list conversations failed: $conv_resp"
  exit 1
fi

echo "[4/7] send message"
send_resp=$(curl_json POST "$BASE_URL/api/v1/conversations/$conv_id/send" "{\"client_msg_id\":\"smoke-msg-1\",\"content\":{\"type\":\"text\",\"text\":\"你好\"}}" "$token")
if ! echo "$send_resp" | grep -q '"ack"'; then
  echo "send message failed: $send_resp"
  exit 1
fi

echo "[5/7] proactive trigger"
pro_resp=$(curl_json POST "$BASE_URL/api/v1/gateway/test/proactive" "{\"bot_id\":\"$bot_id\",\"conversation_id\":\"$conv_id\",\"trigger_type\":\"manual\"}" "$token")
if ! echo "$pro_resp" | grep -q '"should_send"'; then
  echo "proactive failed: $pro_resp"
  exit 1
fi

echo "[6/7] health"
health_resp=$(curl_json GET "$BASE_URL/api/v1/gateway/health")
if ! echo "$health_resp" | grep -q '"status"'; then
  echo "health failed: $health_resp"
  exit 1
fi

echo "[7/7] metrics"
metrics_resp=$(curl_json GET "$BASE_URL/api/v1/gateway/metrics")
if ! echo "$metrics_resp" | grep -q '"requests_total"'; then
  echo "metrics failed: $metrics_resp"
  exit 1
fi

echo "smoke test passed"
