#!/usr/bin/env bash
set -Eeuo pipefail

# Builds docs/api-site and publishes it to Nginx under /docs/.

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly site_source="${project_root}/docs/api-site"
readonly source_nginx="${project_root}/deploy/nginx/api-doc-locations.conf"
readonly target_dir="/var/www/new-api-docs/public/docs"
readonly target_nginx="/etc/nginx/snippets/new-api-docs.conf"

[[ -f "${site_source}/build.py" ]] || { echo "ERROR: ${site_source}/build.py is missing" >&2; exit 1; }
[[ -f "${source_nginx}" ]] || { echo "ERROR: ${source_nginx} is missing" >&2; exit 1; }

build_dir="$(mktemp -d)"
trap 'rm -rf "${build_dir}"' EXIT

python3 -I "${site_source}/build.py" --out "${build_dir}/docs"

sudo install -d -m 0755 "${target_dir}"
sudo rsync -a --delete --chown=root:root --chmod=D0755,F0644 "${build_dir}/docs/" "${target_dir}/"

backup_nginx=""
if sudo test -f "${target_nginx}"; then
  backup_nginx="$(mktemp)"
  sudo cat "${target_nginx}" > "${backup_nginx}"
fi
sudo install -m 0644 "${source_nginx}" "${target_nginx}"
if ! sudo nginx -t; then
  if [[ -n "${backup_nginx}" ]]; then
    sudo install -m 0644 "${backup_nginx}" "${target_nginx}"
    echo "ERROR: nginx -t failed; restored the previous ${target_nginx}" >&2
  fi
  exit 1
fi
[[ -n "${backup_nginx}" ]] && rm -f "${backup_nginx}"
sudo systemctl reload nginx

echo "Published ${site_source} to ${target_dir}"
