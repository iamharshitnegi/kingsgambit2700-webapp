# Deploy on one Hostinger VPS

One VPS is the intended initial production setup. Keep development and automated
verification on the local machine. A separate staging server can be added later.
This does not promise high availability: releases can have short interruptions,
and a failed VPS needs recovery from an off-server backup.

## What runs where

```text
Internet -> Caddy :443
            |-- academy domain -> Next.js
            `-- API domain
                 |-- /api/v1/* -> Go API
                 `-- /ws/v1    -> Go realtime

Private Docker network on the same VPS:
  API / realtime / worker -> PostgreSQL
  API / realtime          -> Redis
  API                     -> SeaweedFS S3 + ClamAV
  API / worker            -> configured delivery SMTP
```

Only Caddy publishes ports 80 and 443. PostgreSQL, Redis, file storage, its admin
interfaces, the scanner and application service ports have no host port mappings.
The API trusts forwarded client addresses only from Caddy's fixed private IP,
so visitors have separate rate-limit buckets and other peers cannot forge them.
If the chosen Docker subnet conflicts with another local network, change both
`DOCKER_SUBNET` and `CADDY_PRIVATE_IP` together before creating the project.
Docker named volumes hold durable data. SeaweedFS replaces the development
S3Mock; no storage subscription or second server is required. Mailpit is omitted:
use a real delivery SMTP account so guardians and staff receive sign-in emails.

Size the stack against the purchased VPS before first launch. ClamAV and builds
are significant memory consumers. Build images on the developer machine or CI
and transfer them if server memory is limited. The frontend image uses Next.js
standalone output; the VPS runtime does not need the full frontend dependencies.

## 1. Prepare the server and DNS

Confirm the VPS OS, CPU/RAM, available disk, existing workloads and SSH access.
Install Docker Engine and the Compose plugin using the official instructions for
that OS, or use Hostinger's Docker template if this is still an empty server.
Do not reinstall an OS with existing data just to obtain Docker.

Allow SSH and HTTP/HTTPS in the Hostinger/host firewall. Check SSH remains working
before closing the current session. Set A records for the academy domain,
`www`, and API domain to the VPS. Remove or correct stale AAAA records when IPv6
does not reach this VPS. Ports 80 and 443 must be available for Caddy.

Copy a complete, reviewed release checkout onto the VPS. Use the committed
application source or a source archive that excludes private settings,
dependencies, build outputs, and local test data. Do not copy the local database
as production. For subsequent automatic releases, configure
[GitHub Actions deployment](github-deployment.md).

## 2. Configure this deployment

From the repository root on the VPS:

```bash
bash deploy/vps/init-env.sh
nano deploy/vps/.env
```

The initializer refuses to overwrite an existing environment and generates
unique database, object-storage and authenticator encryption credentials without
printing them. Save an encrypted copy outside the VPS. In particular, losing
`KG_TOTP_ENCRYPTION_KEY` prevents decrypting existing staff MFA secrets.

Fill `WEB_DOMAIN`, `API_DOMAIN`, `ACME_EMAIL`, `KG_SMTP_HOST`, SMTP credentials
and verified `KG_SMTP_FROM`. The database password is generated as URL-safe hex;
preserve that format when replacing it. SMTP values live in Compose's dotenv
file; quote values containing spaces, `#` or `$` according to dotenv rules.
Optional Google sign-in can be configured later.

For this deployment, Google SMTP is selected: `smtp.gmail.com`, port 587,
STARTTLS. Enable Google 2-Step Verification and create an app password if the
account's policy permits it. Enter it privately on the VPS, rather than in chat:

```bash
ssh -t root@187.126.116.75 \
  'python3 /opt/kg2700/app/deploy/vps/configure-google.py'
```

The helper records sender/admin emails, the app password, and an optional public
Google Web OAuth client ID in the mode-0600 environment. This is direct Google
Identity Services sign-in, already supported by the application. Authorize the
HTTPS academy origin in the Google Web client's JavaScript origins. No Google
client secret is used for this ID-token flow. Firebase Authentication issues
different tokens and requires its own integration if selected.

Webhook signing values are initially random inactive-provider placeholders to
satisfy the backend production configuration contract. Do not configure provider
webhooks until their actual signing secrets have replaced those values. Payment
checkout and embedded video are not activated by this setup.

The production environment is `deploy/vps/.env`, separate from the root local
`.env`. All commands below use the wrapper to select the correct project.

```bash
bash deploy/vps/compose.sh config --quiet
COMPOSE_PARALLEL_LIMIT=1 bash deploy/vps/compose.sh \
  --profile tools build frontend api worker realtime river-migrate bootstrap-admin
```

Public URLs are compiled into the frontend image; rebuild it when changing
domains. Server-side public API reads use the internal API URL at runtime.
At build time, the public API may be unavailable on a fresh server; public pages
use their existing fallbacks and refresh through the configured revalidation.
Verify the real academy information before opening the site to customers.

## 3. Start infrastructure, then migrate

```bash
bash deploy/vps/compose.sh up -d --wait postgres redis clamav storage
bash deploy/vps/compose.sh --profile tools run --rm migrate
bash deploy/vps/compose.sh --profile tools run --rm river-migrate up
bash deploy/vps/compose.sh --profile tools run --rm river-migrate validate
bash deploy/vps/compose.sh up -d --wait api realtime worker frontend caddy
bash deploy/vps/compose.sh ps
```

ClamAV's first signature download can take time. Caddy obtains and renews TLS
certificates after DNS points here and the challenge ports are reachable. Keep
its data volume to retain certificate/account state. Check its logs if HTTPS is
not ready. Worker startup must follow successful River migrations.

This uses one production project named `kg2700-production`; do not run the root
development Compose file on the VPS. Never use `down -v` on this production
project: it deletes the persistent volumes.

## 4. Create the real first administrator

Replace the example mailbox with the actual administrator's email:

```bash
bash deploy/vps/compose.sh --profile tools run --rm bootstrap-admin \
  -email owner@your-domain.example -display-name 'Academy Owner' -dry-run
```

Review the target, then repeat without `-dry-run`. This works once on a fresh
academy and refuses to elevate an existing account. Sign in through the delivered
email and complete staff MFA; save recovery codes securely. No demo accounts or
shared default passwords are installed.

Save real academy contact information, publish approved policies, provision a
coach and family, and complete guardian consent. Grant initial enrollment credits
through the audited admin flow. External Zoom/Meet links remain the class meeting
mechanism while the shared board runs through this VPS.

## 5. Check the live flow before inviting families

Use your own test accounts on the production host, clearly distinguishable from
customer records. Check sign-in mail and MFA, guardian consent, enrollment and
credit grant, scheduling, one shared-board lesson with reload/replay, scanned
file upload/download, homework submission/review and report visibility. Verify
that one family cannot access another family's records. Confirm logout and device
revocation. Check that the worker delivers a notification to a real mailbox.

Run the destructive/isolated integration and browser suites locally, not against
the customer database. There is no requirement to pay for staging to run those
checks. Keep unfinished payment flows disabled; manual credit grants do not
constitute automated payment processing.

## 6. Backups and small releases

Before the first customer and before a release, make a database backup:

```bash
umask 077
mkdir -p backups
bash deploy/vps/compose.sh exec -T postgres \
  pg_dump -U kg2700 -d kg2700 -Fc > backups/academy.dump
```

This example writes a local file; copy it to an encrypted location outside the
VPS and keep dated versions. A backup on the VPS does not protect against VPS
loss. Also back up file storage and the private deployment environment. For a
simple consistent file-store backup, briefly stop API and worker writes, stop
storage, archive its named volume using a trusted backup tool, then restart the
services. Preserve the database backup from the same write-free interval.
Verify restoration into a disposable local database and storage volume before
relying on the backups. Hostinger snapshots may supplement this; verify the
purchased plan's retention and restore behavior.

For updates, run checks locally, retain the previous release images, set a new
`RELEASE` in the production environment, build images, back up, run forward
migrations, then recreate the application services. Recreate Caddy after replacing
backend containers so existing WebSockets reconnect. Check login and a lesson
afterward. Roll back application images if compatible with the active schema;
do not automatically reverse database migrations. Plan a short maintenance window
for initial releases rather than promising zero downtime on one host.

Review `compose.sh logs --tail=100 api worker realtime caddy`, failed queue jobs,
disk space, container health, real email delivery and backup completion. An
external uptime check against the HTTPS site is useful even when everything
else lives on the VPS.

## References

- [Docker: Compose in production](https://docs.docker.com/compose/how-tos/production/)
- [Caddy: automatic HTTPS](https://caddyserver.com/docs/automatic-https)
- [Caddy: reverse proxy and WebSockets](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [SeaweedFS: single-node mini setup](https://github.com/seaweedfs/seaweedfs/wiki/Quick-Start-with-weed-mini)
- [Hostinger: Docker Manager](https://www.hostinger.com/support/12040789-hostinger-docker-manager-for-vps-simplify-your-container-deployments/)
- [Google: Gmail SMTP and STARTTLS](https://developers.google.com/workspace/gmail/imap/imap-smtp)
- [Google: app passwords](https://support.google.com/accounts/answer/185833)
- [Google: Web sign-in client configuration](https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid)
