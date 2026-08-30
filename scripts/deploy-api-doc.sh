#!/usr/bin/env bash
set -Eeuo pipefail

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly source_page="${project_root}/docs/api-doc.html"
readonly source_nginx="${project_root}/deploy/nginx/api-doc-locations.conf"
readonly target_dir="/var/www/new-api-docs"
readonly target_page="${target_dir}/api-doc.html"
readonly target_nginx="/etc/nginx/snippets/new-api-docs.conf"

[[ -f "${source_page}" ]] || { echo "ERROR: ${source_page} is missing" >&2; exit 1; }
[[ -f "${source_nginx}" ]] || { echo "ERROR: ${source_nginx} is missing" >&2; exit 1; }

sudo install -d -m 0755 "${target_dir}"
sudo install -m 0644 "${source_page}" "${target_page}"
sudo install -m 0644 "${source_nginx}" "${target_nginx}"
sudo nginx -t
sudo systemctl reload nginx

echo "Published ${source_page} to ${target_page}"
