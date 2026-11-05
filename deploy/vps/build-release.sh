#!/usr/bin/env bash
set -euo pipefail
revision="${1:?Pass the commit SHA}"
output="${2:?Pass an output directory outside the checkout}"
[[ "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo 'Expected a full commit SHA.' >&2; exit 1; }
[[ "$(git rev-parse HEAD)" == "$revision" ]] || { echo 'Checkout does not match the release SHA.' >&2; exit 1; }
[[ ! -e "$output" ]] || { echo 'Release output must be a new directory.' >&2; exit 1; }
mkdir -p "$output"
output="$(cd "$output" && pwd)"
build_context="$(mktemp -d)"
trap 'rm -rf "$build_context"' EXIT
git archive --format=tar.gz --output="$output/source.tar.gz" "$revision"
tar -xzf "$output/source.tar.gz" -C "$build_context"
export WEB_DOMAIN="${WEB_DOMAIN:-kingsgambit2700.in}"
export API_DOMAIN="${API_DOMAIN:-api.kingsgambit2700.in}"

images=()
for service in api worker realtime river-migrate bootstrap-admin; do
  image="kg2700-$service:$revision"
  docker build --platform linux/amd64 --file "$build_context/backend/Dockerfile" --target "$service" \
    --label "org.opencontainers.image.revision=$revision" --tag "$image" "$build_context/backend"
  images+=("$image")
done
image="kg2700-frontend:$revision"
docker build --platform linux/amd64 --file "$build_context/Dockerfile" --target frontend \
  --build-arg "WEB_DOMAIN=$WEB_DOMAIN" --build-arg "API_DOMAIN=$API_DOMAIN" \
  --label "org.opencontainers.image.revision=$revision" --tag "$image" "$build_context"
images+=("$image")
docker save "${images[@]}" | gzip -1 > "$output/images.tar.gz"
python3 - "$output" "$revision" <<'PY'
from pathlib import Path
import hashlib, json, os, sys
directory, revision = Path(sys.argv[1]), sys.argv[2]
manifest = {"version": 1, "revision": revision,
            "web_domain": os.environ["WEB_DOMAIN"], "api_domain": os.environ["API_DOMAIN"]}
for name in ["images.tar.gz", "source.tar.gz"]:
    with (directory / name).open("rb") as source:
        manifest[name] = hashlib.file_digest(source, "sha256").hexdigest()
(directory / "manifest.json").write_text(json.dumps(manifest) + "\n")
PY
tar -C "$output" -cf "$output/release.tar" manifest.json source.tar.gz images.tar.gz
rm "$output/source.tar.gz" "$output/images.tar.gz" "$output/manifest.json"
echo "Packaged release $revision. Production secrets are not included."
