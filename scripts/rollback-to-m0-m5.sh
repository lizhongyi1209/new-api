#!/usr/bin/env bash
# 紧急回滚:M0-M9 (dcf63b9ce) -> M0-M5 (77b740deb),即 2026-08-19 已稳定运行的生产版本。
#
# 用法:  ./scripts/rollback-to-m0-m5.sh
#
# 原理:M6-M9 零数据库 schema 变更(已验证:无 gorm tag 改动、无 AutoMigrate 新增),
#      因此回滚只需把容器换回旧镜像,不需要恢复数据库。
set -Eeuo pipefail

readonly rollback_image="new-api-new-api:rollback-77b740deb5f9-M0-M5"
readonly rollback_revision="77b740deb5f960065c7c0179ba1deae28da2531e"
readonly container="new-api"
readonly service="new-api"

fail() { echo "ERROR: $*" >&2; exit 1; }

cd "$(dirname "$0")/.."

echo "==> 校验回滚镜像存在"
docker image inspect "${rollback_image}" >/dev/null 2>&1 \
  || fail "回滚镜像 ${rollback_image} 不存在。改用源码回滚:git checkout 77b740deb && docker compose up --build -d ${service}"

actual_revision=$(docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "${rollback_image}")
[[ "${actual_revision}" == "${rollback_revision}" ]] \
  || fail "镜像 revision 不符:期望 ${rollback_revision},实际 ${actual_revision}"

echo "==> 把 :latest 指回 M0-M5 镜像"
docker tag "${rollback_image}" new-api-new-api:latest

echo "==> 重启生产容器(不重新构建)"
docker compose up -d --no-build --force-recreate "${service}"

echo "==> 等待健康检查"
for _ in $(seq 1 30); do
  status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}")
  [[ "${status}" == "healthy" ]] && break
  sleep 2
done
[[ "${status}" == "healthy" ]] || fail "容器未恢复健康,当前状态 ${status}。查看日志:docker compose logs --tail=100 ${service}"

echo "==> 校验运行中的版本"
body=$(curl --fail --silent --show-error "${PRODUCTION_BASE_URL:-http://127.0.0.1:3000}/api/status")
[[ "${body}" == *"\"git_commit\":\"${rollback_revision}\""* ]] \
  || fail "/api/status 未报告 ${rollback_revision},回滚可能未生效"

echo
echo "回滚完成,已恢复到 M0-M5 (${rollback_revision})。"
echo "注意:git 工作树仍在 dcf63b9ce(M0-M9)。如需让源码也回到 M0-M5:"
echo "  git reset --hard 77b740deb && git push origin custom --force-with-lease"
