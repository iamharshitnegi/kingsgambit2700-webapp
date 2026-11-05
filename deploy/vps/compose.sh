#!/usr/bin/env bash
set -euo pipefail
kg_vps_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "$kg_vps_dir/.env" ]]; then
  echo 'First run: bash deploy/vps/init-env.sh' >&2
  exit 1
fi
exec docker compose --env-file "$kg_vps_dir/.env" -f "$kg_vps_dir/compose.yaml" "$@"
