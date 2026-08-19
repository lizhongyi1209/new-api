#!/usr/bin/env bash
set -Eeuo pipefail

readonly source_container="${UPSTREAM_TEST_CONTAINER:-new-api-upstream-test-api}"
readonly test_port="${UPSTREAM_TEST_SECURITY_PORT:-3303}"
readonly test_origin="${UPSTREAM_TEST_SECURITY_ORIGIN:-https://key.o1key.com}"
readonly container_name="new-api-upstream-test-security-$$"
readonly work_dir="$(mktemp -d /tmp/new-api-auth-security.XXXXXX)"
image_name="${UPSTREAM_TEST_IMAGE:-$(docker inspect --format '{{.Config.Image}}' "${source_container}")}"

cleanup() {
  docker stop "${container_name}" >/dev/null 2>&1 || true
  rm -rf -- "${work_dir}"
}
trap cleanup EXIT

docker run --rm -d \
  --name "${container_name}" \
  -p "127.0.0.1:${test_port}:3000" \
  -e GIN_MODE=release \
  -e SESSION_SECRET=upstream-test-security-session-secret \
  -e CRYPTO_SECRET=upstream-test-security-crypto-secret \
  -e SESSION_COOKIE_SECURE=true \
  -e "SESSION_COOKIE_TRUSTED_URL=${test_origin}" \
  -e TRUSTED_PROXIES=none \
  -e CRITICAL_RATE_LIMIT_ENABLE=true \
  -e CRITICAL_RATE_LIMIT=20 \
  -e CRITICAL_RATE_LIMIT_DURATION=1200 \
  "${image_name}" >/dev/null

base_url="http://127.0.0.1:${test_port}"
for _ in $(seq 1 30); do
  if curl -fsS "${base_url}/api/status" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "${base_url}/api/status" >/dev/null

request_refresh() {
  local name=$1
  shift
  curl -sS -D "${work_dir}/${name}.headers" -o "${work_dir}/${name}.json" \
    -w '%{http_code}' -X POST "$@" "${base_url}/api/user/auth/refresh"
}

status=$(request_refresh missing-origin)
[[ "${status}" == "403" ]]

status=$(request_refresh foreign-origin -H 'Origin: https://evil.example.test')
[[ "${status}" == "403" ]]

status=$(request_refresh multiple-origin -H "Origin: ${test_origin}, https://evil.example.test")
[[ "${status}" == "403" ]]

status=$(request_refresh trusted-origin -H "Origin: ${test_origin}")
[[ "${status}" == "401" ]]

status=$(request_refresh trusted-referer -H "Referer: ${test_origin}/profile")
[[ "${status}" == "401" ]]

status=$(request_refresh forwarded-proto \
  -H "Origin: https://127.0.0.1:${test_port}" \
  -H 'X-Forwarded-Proto: https')
[[ "${status}" == "403" ]]

for header_file in \
  "${work_dir}/missing-origin.headers" \
  "${work_dir}/foreign-origin.headers" \
  "${work_dir}/multiple-origin.headers" \
  "${work_dir}/forwarded-proto.headers"; do
  if grep -qi '^Access-Control-Allow-Origin:' "${header_file}"; then
    echo "A rejected origin unexpectedly received a CORS allow-origin header" >&2
    exit 1
  fi
done

grep -qi "^Access-Control-Allow-Origin: ${test_origin}" "${work_dir}/trusted-origin.headers"

# Two trusted-origin refresh requests above consumed two entries in the same
# critical IP window. Eighteen spoofed X-Forwarded-For values must still share
# the direct peer's remaining quota when TRUSTED_PROXIES=none.
for request_number in $(seq 1 18); do
  status=$(curl -sS -o /dev/null -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H "X-Forwarded-For: 203.0.113.${request_number}" \
    --data-binary '{"username":"missing-user","password":"invalid-password"}' \
    "${base_url}/api/user/login")
  [[ "${status}" != "429" ]]
done

status=$(curl -sS -o /dev/null -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -H 'X-Forwarded-For: 198.51.100.200' \
  --data-binary '{"username":"missing-user","password":"invalid-password"}' \
  "${base_url}/api/user/login")
[[ "${status}" == "429" ]]

echo "Auth security E2E passed: strict Origin Guard, no untrusted CORS reflection, and forwarded IP spoofing cannot evade rate limits."
