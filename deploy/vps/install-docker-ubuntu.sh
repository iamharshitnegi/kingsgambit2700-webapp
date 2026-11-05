#!/usr/bin/env bash
# Official Docker apt repository; run as root on the intended Ubuntu VPS.
set -euo pipefail
if [[ "$(id -u)" != 0 ]]; then
  echo 'Run this installer as root on the VPS.' >&2
  exit 1
fi
if command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  docker --version
  docker compose version
  exit 0
fi
. /etc/os-release
if [[ "$ID" != ubuntu ]]; then
  echo 'This installer supports Ubuntu only.' >&2
  exit 1
fi
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
kg_vps_arch="$(dpkg --print-architecture)"
cat > /etc/apt/sources.list.d/docker.sources <<SOURCES
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${UBUNTU_CODENAME:-$VERSION_CODENAME}
Components: stable
Architectures: $kg_vps_arch
Signed-By: /etc/apt/keyrings/docker.asc
SOURCES
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker
docker --version
docker compose version
