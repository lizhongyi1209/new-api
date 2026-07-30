#!/usr/bin/env bash
set -Eeuo pipefail

readonly production_branch="custom"
readonly production_container="new-api"
readonly production_service="new-api"
readonly production_image="new-api-new-api:latest"
readonly production_tag_prefix="production-"
readonly deploy_confirmation="DEPLOY_NEW_API_PRODUCTION"

usage() {
  echo "Usage: $0 check|build|deploy"
  echo "  check   Validate repository and production ancestry without changing runtime state."
  echo "  build   Run checks and build an immutable, revision-labelled image; does not deploy."
  echo "  deploy  Build and deploy. Requires DEPLOY_CONFIRM=${deploy_confirmation}."
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

current_branch=$(git branch --show-current)
head_sha=$(git rev-parse HEAD)
short_sha=$(git rev-parse --short=12 HEAD)
build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
release_tag="${production_tag_prefix}$(date -u +%Y%m%d-%H%M)-${short_sha}"

check_release() {
  [[ "${current_branch}" == "${production_branch}" ]] || fail "branch must be ${production_branch}, got ${current_branch:-detached HEAD}"
  [[ -z "$(git status --porcelain)" ]] || fail "working tree is not clean"
  git show-ref --verify --quiet "refs/remotes/origin/${production_branch}" || fail "origin/${production_branch} is missing; run git fetch first"
  [[ "${head_sha}" == "$(git rev-parse "origin/${production_branch}")" ]] || fail "HEAD must exactly match origin/${production_branch}"

  local latest_production_tag
  latest_production_tag=$(git for-each-ref --count=1 --sort=-creatordate --format='%(refname:short)' "refs/tags/${production_tag_prefix}*")
  if [[ -n "${latest_production_tag}" ]]; then
    git merge-base --is-ancestor "${latest_production_tag}" HEAD || fail "latest production tag ${latest_production_tag} is not an ancestor of HEAD"
    echo "Previous production: ${latest_production_tag} ($(git rev-parse --short=12 "${latest_production_tag}"))"
  else
    echo "Previous production: none"
  fi

  echo "Candidate revision: ${head_sha}"
  echo "Candidate tag: ${release_tag}"
}

build_release() {
  check_release
  local go_command=${GO_COMMAND:-go}
  if [[ -x /usr/local/go/bin/go && -z "${GO_COMMAND:-}" ]]; then
    go_command=/usr/local/go/bin/go
  fi
  "${go_command}" test ./middleware ./relay/common ./relay ./router ./relay/channel/task/kling ./relay/channel/task/xai

  export GIT_COMMIT="${head_sha}"
  export BUILD_TIME="${build_time}"
  local build_status=0
  docker compose build "${production_service}" || build_status=$?
  docker builder prune -af
  [[ "${build_status}" -eq 0 ]] || fail "Docker build failed with status ${build_status}"

  local image_id image_revision
  image_id=$(docker image inspect --format '{{.Id}}' "${production_image}")
  image_revision=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "${production_image}")
  [[ "${image_revision}" == "${head_sha}" ]] || fail "image revision ${image_revision} does not match ${head_sha}"
  echo "Built image: ${image_id} (${image_revision})"
}

smoke_test() {
  local base_url=${PRODUCTION_BASE_URL:-http://127.0.0.1:3000}
  local status_body route_body
  status_body=$(curl --fail --silent --show-error "${base_url}/api/status")
  [[ "${status_body}" == *"\"git_commit\":\"${head_sha}\""* ]] || fail "status endpoint does not report ${head_sha}"

  for route in /kling/omni-video/kling-3.0-omni /kling/motion-control/kling-3.0 /grok/v1/videos/generations /grok/v1/videos/edits /grok/v1/videos/extensions; do
    route_body=$(curl --silent --show-error --request POST --header 'Content-Type: application/json' --data '{}' "${base_url}${route}")
    [[ "${route_body}" != *"<html"* && "${route_body}" != *"<!DOCTYPE html"* ]] || fail "${route} was handled by an HTML fallback"
  done
}

command=${1:-check}
case "${command}" in
  check)
    check_release
    ;;
  build)
    build_release
    ;;
  deploy)
    [[ "${DEPLOY_CONFIRM:-}" == "${deploy_confirmation}" ]] || fail "set DEPLOY_CONFIRM=${deploy_confirmation} to deploy"
    build_release
    docker compose up -d --no-build "${production_service}"
    for _ in $(seq 1 30); do
      [[ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${production_container}")" == "healthy" ]] && break
      sleep 2
    done
    [[ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${production_container}")" == "healthy" ]] || fail "deployed container did not become healthy"
    smoke_test
    git tag -a "${release_tag}" -m "Production deployment ${head_sha}"
    git push origin "refs/tags/${release_tag}"
    echo "Deployed and tagged ${release_tag}"
    ;;
  *)
    usage
    exit 2
    ;;
esac
