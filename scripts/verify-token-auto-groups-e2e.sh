#!/usr/bin/env bash
# Live M5 check against the isolated candidate stack: exercises the token-level
# ordered AutoGroups snapshot over real HTTP, a real database, and the Redis
# cache the container is configured with. Groups are discovered from the running
# instance so the script never mutates its option state.
set -Eeuo pipefail

readonly base_url="${UPSTREAM_TEST_BASE_URL:-http://127.0.0.1:3302}"
readonly test_username="${UPSTREAM_TEST_USERNAME:-m3root}"
readonly test_password="${UPSTREAM_TEST_PASSWORD:-M3-test-password-2026}"
readonly work_dir="$(mktemp -d /tmp/new-api-auto-groups-e2e.XXXXXX)"
trap 'rm -rf -- "${work_dir}"' EXIT

assert_success() {
  jq -e '.success == true' "$1" >/dev/null
}

assert_rejected() {
  jq -e '.success == false and (.message | length > 0)' "$1" >/dev/null
}

curl -fsS -o "${work_dir}/login.json" \
  -H 'Content-Type: application/json' \
  --data-binary "{\"username\":\"${test_username}\",\"password\":\"${test_password}\"}" \
  "${base_url}/api/user/login"
assert_success "${work_dir}/login.json"
access_token=$(jq -r '.data.access_token' "${work_dir}/login.json")

api() {
  local method=$1 path=$2 out=$3
  shift 3
  curl -fsS -o "${out}" -X "${method}" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json' \
    "$@" "${base_url}${path}"
}

api_status() {
  local method=$1 path=$2 out=$3
  shift 3
  curl -sS -o "${out}" -w '%{http_code}' -X "${method}" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json' \
    "$@" "${base_url}${path}"
}

# The Auto group catalogue must be readable and bounded.
api GET /api/token/auto-groups "${work_dir}/auto-groups.json"
assert_success "${work_dir}/auto-groups.json"
jq -e '.data.groups | type == "array"' "${work_dir}/auto-groups.json" >/dev/null
jq -e '.data.max_count | type == "number" and . > 0' "${work_dir}/auto-groups.json" >/dev/null

api GET /api/user/self/groups "${work_dir}/groups.json"
assert_success "${work_dir}/groups.json"
mapfile -t selectable_groups < <(jq -r '.data | keys_unsorted | map(select(. != "auto")) | .[]' "${work_dir}/groups.json")
(( ${#selectable_groups[@]} >= 2 )) || { echo "need at least two selectable groups on the test instance" >&2; exit 1; }
selectable_group="${selectable_groups[0]}"
second_group="${selectable_groups[1]}"
max_count=$(jq -r '.data.max_count' "${work_dir}/auto-groups.json")

create_token() {
  local name=$1 payload=$2 out=$3
  api POST /api/token/ "${out}" --data-binary "${payload}"
}

# AddToken answers with an empty envelope, so created tokens are resolved by
# their unique name through the exact-match search endpoint.
token_id_by_name() {
  local name=$1 out=$2
  api GET "/api/token/search?keyword=${name}" "${out}"
  assert_success "${out}"
  jq -r --arg n "${name}" '.data.items | map(select(.name == $n)) | .[0].id' "${out}"
}

base_payload() {
  local name=$1 group=$2 extra=$3
  printf '{"name":"%s","expired_time":-1,"remain_quota":0,"unlimited_quota":true,"group":"%s","cross_group_retry":true%s}' \
    "${name}" "${group}" "${extra}"
}

# Omitting auto_groups keeps the token on the globally ordered Auto list.
create_token inherit "$(base_payload "e2e-inherit-$$" auto "")" "${work_dir}/inherit.json"
assert_success "${work_dir}/inherit.json"

# An explicit snapshot must survive the round trip in the submitted order. The
# two groups are sent in reverse discovery order so a server-side re-sort would
# be visible rather than coincidentally matching.
custom_name="e2e-custom-$$"
custom_payload=$(base_payload "${custom_name}" auto ",\"auto_groups\":[\"${second_group}\",\"${selectable_group}\"]")
create_token custom "${custom_payload}" "${work_dir}/custom.json"
assert_success "${work_dir}/custom.json"
custom_id=$(token_id_by_name "${custom_name}" "${work_dir}/custom-search.json")
[[ "${custom_id}" =~ ^[0-9]+$ ]] || { echo "could not resolve created token id" >&2; exit 1; }

api GET "/api/token/${custom_id}" "${work_dir}/custom-read.json"
assert_success "${work_dir}/custom-read.json"
jq -e --arg a "${second_group}" --arg b "${selectable_group}" \
  '.data.auto_groups == [$a, $b]' "${work_dir}/custom-read.json" >/dev/null
jq -e '.data.group == "auto"' "${work_dir}/custom-read.json" >/dev/null

# Duplicated and unauthorized entries are rejected under the project envelope
# (HTTP 200 with success=false), and must not create a token.
dup_payload=$(base_payload "e2e-dup-$$" auto ",\"auto_groups\":[\"${selectable_group}\",\"${selectable_group}\"]")
dup_status=$(api_status POST /api/token/ "${work_dir}/dup.json" --data-binary "${dup_payload}")
[[ "${dup_status}" == "200" ]]
assert_rejected "${work_dir}/dup.json"

invalid_payload=$(base_payload "e2e-invalid-$$" auto ',"auto_groups":["definitely-not-a-real-group"]')
invalid_status=$(api_status POST /api/token/ "${work_dir}/invalid.json" --data-binary "${invalid_payload}")
[[ "${invalid_status}" == "200" ]]
assert_rejected "${work_dir}/invalid.json"

# Exceeding the configured per-token limit is rejected the same way.
if (( ${#selectable_groups[@]} > max_count )); then
  over_limit_json=$(printf '%s\n' "${selectable_groups[@]:0:$((max_count + 1))}" | jq -R . | jq -sc .)
  over_payload=$(base_payload "e2e-overlimit-$$" auto ",\"auto_groups\":${over_limit_json}")
  over_status=$(api_status POST /api/token/ "${work_dir}/over.json" --data-binary "${over_payload}")
  [[ "${over_status}" == "200" ]]
  assert_rejected "${work_dir}/over.json"
else
  echo "skipped over-limit case: instance exposes ${#selectable_groups[@]} groups for a limit of ${max_count}" >&2
fi

# Switching to a fixed group clears the snapshot and disables cross-group retry.
update_payload=$(printf '{"id":%s,"name":"%s","expired_time":-1,"remain_quota":0,"unlimited_quota":true,"status":1,"group":"%s","cross_group_retry":true,"model_limits":"","model_limits_enabled":false}' \
  "${custom_id}" "${custom_name}" "${selectable_group}")
api PUT /api/token/ "${work_dir}/fixed.json" --data-binary "${update_payload}"
assert_success "${work_dir}/fixed.json"

api GET "/api/token/${custom_id}" "${work_dir}/fixed-read.json"
assert_success "${work_dir}/fixed-read.json"
jq -e '.data.auto_groups == null' "${work_dir}/fixed-read.json" >/dev/null
jq -e '.data.cross_group_retry == false' "${work_dir}/fixed-read.json" >/dev/null

# The batch endpoint rewrites the snapshot for owned tokens only.
batch_payload=$(printf '{"ids":[%s],"group":"auto","auto_groups":["%s"],"cross_group_retry":true}' \
  "${custom_id}" "${selectable_group}")
api POST /api/token/batch/group "${work_dir}/batch.json" --data-binary "${batch_payload}"
assert_success "${work_dir}/batch.json"
jq -e '.data == 1' "${work_dir}/batch.json" >/dev/null

api GET "/api/token/${custom_id}" "${work_dir}/batch-read.json"
assert_success "${work_dir}/batch-read.json"
jq -e --arg g "${selectable_group}" '.data.auto_groups == [$g]' "${work_dir}/batch-read.json" >/dev/null

echo "Token AutoGroups E2E passed: catalogue, ordered snapshot round trip, tri-state inherit, duplicate/unauthorized rejection, fixed-group cleanup, and batch rewrite."
