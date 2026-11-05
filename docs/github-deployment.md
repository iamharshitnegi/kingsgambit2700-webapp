# GitHub Actions deployment to the Hostinger VPS

Push to `main` to run CI and deploy after every required job passes. Pull requests
run checks without production access. To redeploy the current `main` commit, open
Actions → CI → Run workflow and select `main`; this runs the checks again first.

GitHub builds six Docker images for Linux amd64 and tags them with the full commit
SHA. A source archive and the compressed images travel directly over SSH to the
existing VPS. No container registry, self-hosted runner, or staging server is used.
The Actions SSH key only permits verified release uploads and deployments.
Re-running an already-active commit retains its deployed image artifacts while
reapplying configuration. Retrying a failed candidate can load rebuilt images;
its source archive and build domains must still match the original commit.

## One-time setup

The existing production Compose project and environment must already be configured
using [the VPS runbook](hostinger-vps.md). Both the web and API DNS records must
point to this VPS and serve HTTPS before the first automated deployment.

On the VPS, run the installer from the complete application checkout:

```bash
cd /opt/kg2700/app
bash deploy/vps/install-actions.sh
```

It preserves the existing private settings in
`/opt/kg2700/shared/production.env`, links the original checkout to those settings,
installs the deployment helpers, and generates a dedicated SSH key. Existing human
SSH access is retained. Re-running the installer preserves the key and settings.
Re-run it after deliberately updating the installed deployment-helper protocol.

In this repository's **Settings → Secrets and variables → Actions → New repository
secret**, add:

| Secret | Value |
| --- | --- |
| `VPS_SSH_KEY` | Complete private key from `/opt/kg2700/ci/github-actions-key` |

Transfer the key privately or use an authenticated GitHub CLI, without putting it
in chat, a Git commit, or a workflow variable. For example, from the local machine:

```bash
ssh -o StrictHostKeyChecking=yes root@187.126.116.75 \
  'cat /opt/kg2700/ci/github-actions-key' \
  | gh secret set VPS_SSH_KEY --repo iamharshitnegi/kingsgambit2700-webapp
```

GitHub CLI must already be signed in with permission to manage repository Actions
secrets. This pipeline does not require a GitHub token on the VPS. Gmail, OAuth,
database, storage, and MFA credentials stay in the VPS's private environment.

The SSH host public key is pinned in `deploy/vps/known_hosts`. When replacing the
VPS or rotating host keys, verify the new key through a trusted connection before
updating that file. Do not replace host verification with runtime `ssh-keyscan`.

The domains default to `kingsgambit2700.in` and `api.kingsgambit2700.in`. If they
change, set repository variables `KG_WEB_DOMAIN` and `KG_API_DOMAIN` to match the
private VPS environment. The deployer refuses mismatched build domains.

## What a release does

1. Validates the uploaded checksum, manifest, source paths, and image revisions.
2. Extracts source into `/opt/kg2700/releases/<commit-sha>` and links shared settings.
3. Loads prebuilt images; it never builds application images on the VPS.
4. Ensures infrastructure is running without recreating existing infrastructure.
5. Creates and validates a PostgreSQL custom-format dump before migrations.
6. Applies application and River migrations, then validates River's schema.
7. Starts the versioned application containers and Caddy, waits for health, and
   checks API readiness plus public HTTPS for the site and API.
8. Atomically changes `/opt/kg2700/current` only after those checks pass.

Production deployments are serialized in GitHub and with a VPS file lock. New
pushes do not cancel a running production deployment. On one VPS, application
replacement can briefly interrupt requests and WebSocket sessions.
The server-side operation continues if SSH disconnects; inspect the private logs
in `/opt/kg2700/ci/logs/` before retrying an interrupted runner job.

If backup or migration fails, application replacement does not start. If rollout
or its health checks fail, the deployer attempts to restore previous application
images. It never runs down migrations or deletes database/storage volumes.
Migration changes must remain compatible with the prior application for automatic
rollback to work; otherwise use a forward fix or a deliberate recovery procedure.

Three versioned releases are retained, always preserving the current and previous
active releases, along with seven database dumps. Only obsolete application image tags are removed;
there is no global Docker/volume prune. PostgreSQL dumps stay private under
`/opt/kg2700/backups/`. Copy backups off the VPS and back up uploaded files and
shared settings separately, as described in the VPS runbook.

## Inspect or roll back

```bash
cd /opt/kg2700/current
bash deploy/vps/compose.sh ps
bash deploy/vps/compose.sh logs --tail=100 api worker realtime caddy
```

Each release's `deploy/vps/.release` selects its immutable image tag. The shared
environment's old `RELEASE` value does not override that marker. Explicit shell
`RELEASE=...` overrides remain available for deliberate maintenance.

For a manually reviewed, schema-compatible rollback:

```bash
flock /opt/kg2700/.deployment.lock bash <<'SH'
set -euo pipefail
cd "$(readlink -f /opt/kg2700/previous)"
bash deploy/vps/compose.sh up -d --no-deps --no-build --pull never --wait api worker realtime frontend caddy
ln -s "$PWD" /opt/kg2700/current.rollback
mv -Tf /opt/kg2700/current.rollback /opt/kg2700/current
SH
```

This restores application containers, not the database. Investigate a failed
migration before retrying, and never automatically clear a dirty migration flag.

## References

- [GitHub: reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
- [GitHub: workflow concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
