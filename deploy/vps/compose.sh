#!/usr/bin/env bash
set -euo pipefail
kg_vps_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "$kg_vps_dir/.env" ]]; then
  echo 'First run: bash deploy/vps/init-env.sh' >&2
  exit 1
fi
# A CI release carries its own immutable image tag; secrets remain shared.
if [[ -z "${RELEASE:-}" && -f "$kg_vps_dir/.release" ]]; then
  IFS= read -r RELEASE < "$kg_vps_dir/.release"
  [[ "$RELEASE" =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$ ]] || { echo 'Invalid release image tag.' >&2; exit 1; }
  export RELEASE
fi
exec docker compose --env-file "$kg_vps_dir/.env" -f "$kg_vps_dir/compose.yaml" "$@"
