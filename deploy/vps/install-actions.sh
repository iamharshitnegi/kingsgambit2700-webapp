#!/usr/bin/env bash
set -euo pipefail
[[ "$(id -u)" == 0 ]] || { echo 'Run this installer as root on the VPS.' >&2; exit 1; }
source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root=/opt/kg2700
umask 077
install -d -m 700 "$root/ci" "$root/shared" "$root/incoming" "$root/releases" "$root/backups"
if [[ ! -f "$root/shared/production.env" ]]; then
  [[ -f "$root/app/deploy/vps/.env" ]] || { echo 'Configure the production environment first.' >&2; exit 1; }
  install -m 600 "$root/app/deploy/vps/.env" "$root/shared/production.env"
fi
if [[ -d "$root/app/deploy/vps" && ! -L "$root/app/deploy/vps/.env" ]]; then
  ln -s "$root/shared/production.env" "$root/app/deploy/vps/.env.actions-new"
  mv -Tf "$root/app/deploy/vps/.env.actions-new" "$root/app/deploy/vps/.env"
fi
if [[ ! -e "$root/current" && ! -L "$root/current" ]]; then
  ln -s "$root/app" "$root/current"
fi
[[ -L "$root/current" ]] || { echo '/opt/kg2700/current must be a release symlink.' >&2; exit 1; }
[[ -f "$root/current/deploy/vps/compose.yaml" ]] || { echo 'Current production checkout is missing.' >&2; exit 1; }
install -m 700 "$source_dir/ssh-entrypoint.py" "$root/ci/ssh-entrypoint.py"
install -m 700 "$source_dir/deploy-release.py" "$root/ci/deploy-release.py"
if [[ ! -f "$root/ci/github-actions-key" ]]; then
  ssh-keygen -q -t ed25519 -N '' -C kg2700-github-actions -f "$root/ci/github-actions-key"
fi
install -d -m 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
key="$(cat "$root/ci/github-actions-key.pub")"
entry="restrict,command=\"/usr/bin/python3 $root/ci/ssh-entrypoint.py\" $key"
grep -Fqx "$entry" /root/.ssh/authorized_keys || printf '\n%s\n' "$entry" >> /root/.ssh/authorized_keys
echo 'GitHub deployment transport installed. Existing human SSH keys were preserved.'
echo "Add the private key at $root/ci/github-actions-key as the GitHub Actions secret VPS_SSH_KEY."
echo 'Never paste that private key into chat or commit it.'
