#!/usr/bin/env bash
set -Eeuo pipefail

readonly base_url="${UPSTREAM_TEST_BASE_URL:-http://127.0.0.1:3302}"
readonly test_username="${UPSTREAM_TEST_USERNAME:-m3root}"
readonly test_password="${UPSTREAM_TEST_PASSWORD:-M3-test-password-2026}"
readonly replay_wait_seconds="${UPSTREAM_TEST_REPLAY_WAIT_SECONDS:-31}"
readonly work_dir="$(mktemp -d /tmp/new-api-auth-e2e.XXXXXX)"
trap 'rm -rf -- "${work_dir}"' EXIT

assert_success() {
  local file=$1
  jq -e '.success == true' "${file}" >/dev/null
}

setup_status=$(curl -fsS "${base_url}/api/setup" | jq -r '.data.status')
if [[ "${setup_status}" != "true" ]]; then
  curl -fsS -o "${work_dir}/setup.json" \
    -H 'Content-Type: application/json' \
    --data-binary "{\"username\":\"${test_username}\",\"password\":\"${test_password}\",\"confirmPassword\":\"${test_password}\",\"SelfUseModeEnabled\":true,\"DemoSiteEnabled\":false}" \
    "${base_url}/api/setup"
  assert_success "${work_dir}/setup.json"
fi

login() {
  local cookie_jar=$1
  local response_file=$2
  curl -fsS -c "${cookie_jar}" -o "${response_file}" \
    -H 'Content-Type: application/json' \
    --data-binary "{\"username\":\"${test_username}\",\"password\":\"${test_password}\"}" \
    "${base_url}/api/user/login"
  assert_success "${response_file}"
  jq -e '.data.access_token | type == "string" and length > 20' "${response_file}" >/dev/null
  jq -e '.data.session.sid | type == "string" and length > 10' "${response_file}" >/dev/null
}

login "${work_dir}/current.cookies" "${work_dir}/login.json"
cp "${work_dir}/current.cookies" "${work_dir}/old.cookies"
access_token=$(jq -r '.data.access_token' "${work_dir}/login.json")
session_id=$(jq -r '.data.session.sid' "${work_dir}/login.json")

curl -fsS -o "${work_dir}/sessions.json" \
  -H "Authorization: Bearer ${access_token}" \
  "${base_url}/api/user/sessions"
assert_success "${work_dir}/sessions.json"
jq -e --arg sid "${session_id}" '.data | any(.sid == $sid and .current == true)' "${work_dir}/sessions.json" >/dev/null

curl -fsS -b "${work_dir}/current.cookies" -c "${work_dir}/current.cookies" \
  -o "${work_dir}/refresh.json" \
  -H "X-Auth-Session: ${session_id}" \
  -X POST "${base_url}/api/user/auth/refresh"
assert_success "${work_dir}/refresh.json"
rotated_token=$(jq -r '.data.access_token' "${work_dir}/refresh.json")

curl -fsS -b "${work_dir}/old.cookies" -c "${work_dir}/old-replay.cookies" \
  -o "${work_dir}/grace-replay.json" \
  -H "X-Auth-Session: ${session_id}" \
  -X POST "${base_url}/api/user/auth/refresh"
assert_success "${work_dir}/grace-replay.json"
jq -e --arg sid "${session_id}" '.data.session.sid == $sid' "${work_dir}/grace-replay.json" >/dev/null

sleep "${replay_wait_seconds}"
reuse_status=$(curl -sS -b "${work_dir}/old.cookies" -o "${work_dir}/reuse.json" -w '%{http_code}' \
  -H "X-Auth-Session: ${session_id}" \
  -X POST "${base_url}/api/user/auth/refresh")
[[ "${reuse_status}" == "401" ]]
jq -e '.success == false' "${work_dir}/reuse.json" >/dev/null

revoked_status=$(curl -sS -o "${work_dir}/revoked.json" -w '%{http_code}' \
  -H "Authorization: Bearer ${rotated_token}" \
  "${base_url}/api/user/sessions")
[[ "${revoked_status}" == "401" ]]

login "${work_dir}/session-a.cookies" "${work_dir}/session-a.json"
login "${work_dir}/session-b.cookies" "${work_dir}/session-b.json"
token_a=$(jq -r '.data.access_token' "${work_dir}/session-a.json")
token_b=$(jq -r '.data.access_token' "${work_dir}/session-b.json")
sid_b=$(jq -r '.data.session.sid' "${work_dir}/session-b.json")

curl -fsS -o "${work_dir}/revoke-others.json" \
  -H "Authorization: Bearer ${token_b}" \
  -X POST "${base_url}/api/user/sessions/revoke-others"
assert_success "${work_dir}/revoke-others.json"
jq -e '.data.revoked_count >= 1' "${work_dir}/revoke-others.json" >/dev/null

session_a_status=$(curl -sS -o "${work_dir}/session-a-revoked.json" -w '%{http_code}' \
  -H "Authorization: Bearer ${token_a}" \
  "${base_url}/api/user/sessions")
[[ "${session_a_status}" == "401" ]]

curl -fsS -b "${work_dir}/session-b.cookies" -o "${work_dir}/logout.json" \
  -H "Authorization: Bearer ${token_b}" \
  -H "X-Auth-Session: ${sid_b}" \
  -X POST "${base_url}/api/user/auth/logout"
assert_success "${work_dir}/logout.json"

logout_refresh_status=$(curl -sS -b "${work_dir}/session-b.cookies" -o "${work_dir}/logout-refresh.json" -w '%{http_code}' \
  -X POST "${base_url}/api/user/auth/refresh")
[[ "${logout_refresh_status}" == "401" ]]

echo "Auth session E2E passed: login, refresh rotation, replay revoke, revoke-others, and logout."
