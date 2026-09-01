#!/usr/bin/env bash
set -Eeuo pipefail

readonly compose_project="new-api-upstream-test"
readonly compose_file="docker-compose.upstream-test.yml"
readonly api_container="new-api-upstream-test-api"

usage() {
  echo "Usage: $0 build|up|smoke|crossdb|verify|down|clean"
  echo "  build   Build the isolated candidate image and prune Docker build cache."
  echo "  up      Start the isolated PostgreSQL, MySQL, Redis, and API stack."
  echo "  smoke   Wait for the isolated API container to become healthy."
  echo "  crossdb Run quota, locking, settlement, and auth migration tests against isolated PostgreSQL and MySQL."
  echo "  verify  Build, start, smoke-test, and run cross-database tests."
  echo "  down    Stop containers but retain isolated test volumes."
  echo "  clean   Stop containers and delete only this test stack's volumes."
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

source_revision=$(git rev-parse HEAD)
source_tag=$(git rev-parse --short=12 HEAD)
# Merge bookkeeping documents are excluded from the candidate digest: they are
# not product code, and recording a build result into them must not invalidate
# the tag that was just recorded.
readonly merge_docs=("UPSTREAM_SELECTIVE_MERGE_PLAN.md" "docs/archive/upstream-merge/UPSTREAM_SELECTIVE_MERGE_ROADMAP.md")
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  worktree_digest=$(
    {
      git diff --binary HEAD -- . ':!UPSTREAM_SELECTIVE_MERGE_PLAN.md' ':!docs/archive/upstream-merge/UPSTREAM_SELECTIVE_MERGE_ROADMAP.md'
      while IFS= read -r -d '' file; do
        skip=0
        for merge_doc in "${merge_docs[@]}"; do
          [[ "${file}" == "${merge_doc}" ]] && skip=1 && break
        done
        [[ "${skip}" -eq 1 ]] && continue
        printf '%s\0' "${file}"
        sha256sum -- "${file}"
      done < <(git ls-files --others --exclude-standard -z | sort -z)
    } | sha256sum | cut -c1-12
  )
  source_revision="${source_revision}-dirty.${worktree_digest}"
  source_tag="${source_tag}-dirty.${worktree_digest}"
fi
export UPSTREAM_TEST_GIT_COMMIT="${UPSTREAM_TEST_GIT_COMMIT:-${source_revision}}"
export UPSTREAM_TEST_BUILD_TIME="${UPSTREAM_TEST_BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
export TEST_IMAGE_TAG="${TEST_IMAGE_TAG:-${source_tag}-candidate}"

compose() {
  docker compose --project-name "${compose_project}" --file "${compose_file}" "$@"
}

build_image() {
  local build_status=0
  compose build upstream-test-api || build_status=$?
  docker builder prune -af
  [[ "${build_status}" -eq 0 ]] || fail "candidate image build failed with status ${build_status}"
}

start_stack() {
  compose up -d --no-build
}

smoke_test() {
  local status=""
  for _ in $(seq 1 60); do
    status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${api_container}" 2>/dev/null || true)
    [[ "${status}" == "healthy" ]] && break
    sleep 2
  done
  [[ "${status}" == "healthy" ]] || {
    compose ps
    compose logs --tail=200 upstream-test-api
    fail "isolated API did not become healthy; final status=${status:-missing}"
  }

  local image_revision
  image_revision=$(docker inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "${api_container}")
  [[ "${image_revision}" == "${UPSTREAM_TEST_GIT_COMMIT}" ]] || fail "container revision ${image_revision} does not match ${UPSTREAM_TEST_GIT_COMMIT}"
  echo "Isolated test stack is healthy at revision ${image_revision}."
}

wait_for_healthy_container() {
  local container_name=$1
  local status=""
  for _ in $(seq 1 60); do
    status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_name}" 2>/dev/null || true)
    [[ "${status}" == "healthy" ]] && return 0
    sleep 2
  done
  fail "isolated dependency ${container_name} did not become healthy; final status=${status:-missing}"
}

cross_database_test() {
  wait_for_healthy_container "new-api-upstream-test-postgres"
  wait_for_healthy_container "new-api-upstream-test-mysql"
  UPSTREAM_TEST_POSTGRES_DSN="host=127.0.0.1 port=${UPSTREAM_TEST_POSTGRES_PORT:-35432} user=newapi_test password=newapi_test_password dbname=newapi_test search_path=crossdb sslmode=disable" \
    UPSTREAM_TEST_MYSQL_DSN="newapi_test:newapi_test_password@tcp(127.0.0.1:${UPSTREAM_TEST_MYSQL_PORT:-33306})/newapi_crossdb?charset=utf8mb4&parseTime=True&loc=Local" \
    /usr/local/go/bin/go test ./model -run '^TestQuotaAndSettlementCrossDatabase$' -count=1
  TEST_POSTGRES_DSN="host=127.0.0.1 port=${UPSTREAM_TEST_POSTGRES_PORT:-35432} user=newapi_test password=newapi_test_password dbname=newapi_test search_path=crossdb sslmode=disable" \
    TEST_MYSQL_DSN="newapi_test:newapi_test_password@tcp(127.0.0.1:${UPSTREAM_TEST_MYSQL_PORT:-33306})/newapi_crossdb?charset=utf8mb4&parseTime=True&loc=Local" \
    /usr/local/go/bin/go test ./model -run '^TestUserSessionPreviousRefreshHashMigrationConfiguredDatabases$' -count=1
}

command=${1:-verify}
case "${command}" in
  build)
    build_image
    ;;
  up)
    start_stack
    ;;
  smoke)
    smoke_test
    ;;
  crossdb)
    cross_database_test
    ;;
  verify)
    build_image
    start_stack
    smoke_test
    cross_database_test
    ;;
  down)
    compose down
    ;;
  clean)
    compose down --volumes
    ;;
  *)
    usage
    exit 2
    ;;
esac
