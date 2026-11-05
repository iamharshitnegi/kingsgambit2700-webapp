# KingsGambit2700

A chess academy platform with a public website, guardian/student/coach/admin
portals, and a Go backend for the API, background jobs, and realtime classroom.

**Start with the local demo to explore the project.** It includes sample
accounts and data, and needs no backend services or API keys.

| Run mode | Use it for | Requirements |
| --- | --- | --- |
| Local demo | Explore the UI and test sample academy workflows | Node.js 24 and npm |
| Full development stack | Develop and test the real API and database | Node.js 24, npm, Go 1.26, Docker Compose v2, and Make |

Run commands from the project root: the folder containing `package.json`,
`Makefile`, and `compose.yaml`. The full-stack shell examples use Bash or a
compatible shell; on Windows, use WSL2 with Docker integration for those steps.

## 1. Run the local demo

Check that Node.js and npm are installed:

```bash
node --version
npm --version
```

Install the project dependencies and start the demo:

```bash
npm ci
npm run dev:demo
```

Open:

- **Website:** [http://localhost:3000](http://localhost:3000)
- **Demo accounts:** [http://localhost:3000/demo](http://localhost:3000/demo)

No `.env` file, Go installation, Docker, Sanity account, or provider credentials
are required for this mode. After the first setup, run only `npm run dev:demo`
to start again. Stop the server with **Ctrl+C**.

### Choose a sample account

Click an account on `/demo` to sign in immediately. You can also enter these
emails on `/sign-in`; the demo offers a local continuation instead of sending
an email. There are no demo passwords.

| Role | Email | What to try |
| --- | --- | --- |
| Guardian | `guardian@example.test` | Add a child, save consent, view classes/reports, simulate payment |
| Student | `student@example.test` | Join the classroom and submit homework |
| Coach | `coach@example.test` | Teach on the board, review homework, save attendance and notes |
| Operations admin | `operations@example.test` | Follow up enquiries, view trial bookings, manage support |
| Academic manager | `academics@example.test` | Review learning and publish a draft report |
| Finance admin | `finance@example.test` | View invoices and simulate payment outcomes |
| Super admin | `admin@example.test` | Explore combined staff views and activity history |

The student access code at `/student-access` is **`DEMO2700`**.

### Test a workflow

1. Book a trial through `/free-trial`, then open Operations to see the booking.
2. Submit homework as Student, review it as Coach, and return to Student to
   read the feedback.
3. Open Coach and Student in separate tabs, join their classroom, and enable
   **Allow student moves** from the coach tab. Board moves synchronize.
4. As Guardian, open **Review payment** and try both failure and success.

Use the same hostname and port in every tab: `localhost` and `127.0.0.1` have
separate browser storage. Changes persist in localStorage; each tab keeps its
own selected account in sessionStorage. Use **Reset demo data** on `/demo` to
restore the samples and create a fresh join window for the current class.

Demo people, prices, policies, and payments are fictional. No money is charged,
no email is sent, and video/audio are not connected. The shared board works
between tabs in one browser, not between different devices. Demo data is not
written to PostgreSQL. See the [complete demo walkthrough](docs/demo-walkthrough.md)
for more scenarios and limitations.

## 2. Run the full development stack

Stop any running demo server first. This mode uses PostgreSQL and the real Go
services; demo accounts and browser-stored records do not carry over.

### Prerequisites

Install Node.js 24, Go 1.26, Docker with Compose v2, and Make. Start the Docker
daemon or Docker Desktop, then check:

```bash
node --version
npm --version
go version
docker compose version
make --version
```

### Configure the environment

For a new checkout, copy the **root** example file:

```bash
cp .env.example .env
```

If `.env` already exists, update it instead of overwriting your settings. The
root `.env.example` contains the values matching the local Compose services.
These are the main connection settings:

```dotenv
NEXT_PUBLIC_DEMO_MODE=false
NEXT_PUBLIC_SITE_URL=http://localhost:3000
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
NEXT_PUBLIC_REALTIME_URL=ws://localhost:8081/ws/v1
API_BASE_URL=http://localhost:8080
KG_PUBLIC_WEB_URL=http://localhost:3000
KG_PUBLIC_API_URL=http://localhost:8080
KG_ALLOWED_ORIGINS=http://localhost:3000
KG_DATABASE_URL=postgres://kg2700:kg2700@localhost:5432/kg2700?sslmode=disable
KG_DATABASE_DIRECT_URL=postgres://kg2700:kg2700@localhost:5432/kg2700?sslmode=disable
KG_REDIS_URL=redis://localhost:6379/0
```

Keep API/site origins free of trailing slashes. Leave
`EDITORIAL_CONTENT_ENABLED=false` to run without Sanity. Keep the other local
settings from `.env.example`, including SMTP, storage, and authentication
configuration. Example credentials are for local development only.

### Install dependencies, start infrastructure, and migrate

```bash
make bootstrap
make infra-up
docker compose ps
```

Wait for the infrastructure services to become healthy. The first startup
downloads container images; ClamAV also needs time to initialize. Then run:

```bash
make migrate
```

This applies both the application database migrations and the River background
job migrations. It may build the migration utility image on its first run.

### Start the application

Open **four terminals**, each in the project root, and leave these commands
running:

| Terminal | Command | Service |
| --- | --- | --- |
| 1 | `make backend-api` | REST API |
| 2 | `make backend-worker` | Background worker |
| 3 | `make backend-realtime` | Classroom WebSocket gateway |
| 4 | `make frontend` | Next.js frontend |

The Make targets load the root `.env` for the Go processes. Next.js loads it
for the frontend. `make frontend` is equivalent to `npm run dev`.

| Service | Local address |
| --- | --- |
| Website and sign-in | [http://localhost:3000](http://localhost:3000) |
| REST API base | `http://localhost:8080/api/v1` |
| API readiness check | [http://localhost:8080/health/ready](http://localhost:8080/health/ready) |
| Classroom WebSocket | `ws://localhost:8081/ws/v1` |
| Captured email (Mailpit) | [http://localhost:8025](http://localhost:8025) |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| Local S3 endpoint | `http://localhost:9090` |

### Create the first admin account

The real backend has **no default login credentials**. After migrations, use a
separate terminal in the project root to check the first-admin setup:

```bash
(
  set -a
  . ./.env
  set +a
  cd backend
  go run ./cmd/bootstrap-admin \
    -email admin@example.test \
    -display-name 'Local Administrator' \
    -dry-run
)
```

After the dry run succeeds, run the same command **without `-dry-run`** to
create the account. This initializer works only before any super-admin grant
exists and will not elevate an existing user.

For the bootstrapped administrator, choose **Email me a sign-in link** on
`/sign-in`, enter `admin@example.test`, and open the captured email in Mailpit.
Follow the link and complete the staff authenticator setup. New guardian accounts
can register with a username and password after email verification. Google
sign-in is available once `KG_GOOGLE_CLIENT_ID` is configured.

See [authentication setup](docs/authentication-setup.md) for Google setup and
password provisioning for existing local test accounts. Other real staff accounts
can be provisioned in **Settings & access** by the super administrator; browser demo accounts do not automatically become database
accounts. See [backend setup](backend/README.md) for further details.

### Alternative: run the Go services in Docker

After configuring the root `.env` and installing the frontend dependencies,
you can run the backend services, infrastructure, and migrations together:

```bash
make app-up
```

Run `npm run dev` in another terminal for the frontend. Choose this option
**instead of** the three host `make backend-*` commands to avoid port conflicts.
Go is built inside the containers, though the first-admin command above still
requires a host Go installation.

The container environment is defined in `compose.yaml`; it is not a copy of
every root `.env` value. If you change ports or provider settings, check the
Compose configuration as well as `.env`.

### Stop the stack

Press **Ctrl+C** in each running application terminal, then stop infrastructure:

```bash
make infra-down
```

If you used the containerized application option, run `make app-down` instead.
Both commands preserve named volumes, so database records remain after restart.

## 3. Run checks and tests

Run frontend lint, TypeScript checks, and unit tests without backend services:

```bash
npm run check
```

Install the browser used by the end-to-end tests once:

```bash
npx playwright install chromium
```

Then run the two browser suites sequentially:

```bash
npm run test:demo
npm run test:e2e
```

Stop your existing Next.js development server before running either suite.
`test:demo` starts a demo server on port **3001**; `test:e2e` starts a regular
frontend on **3000** and uses HTTP fixtures. Keep `NEXT_PUBLIC_DEMO_MODE=false`
in your environment for the regular suite; the demo suite enables it itself.

For backend and Compose checks:

```bash
make go-race
make compose-check
```

Database integration tests need a disposable PostgreSQL database with migrations
applied. Follow the [backend integration instructions](backend/README.md), which
include the required database environment variables.

Useful additional commands:

| Command | Purpose |
| --- | --- |
| `npm test` | Run frontend unit tests |
| `npm run test:watch` | Rerun unit tests as files change |
| `npm run lint` | Run ESLint |
| `npm run typecheck` | Check TypeScript |
| `make go-test` | Run Go unit tests |
| `make generate` | Regenerate API and database code |
| `make help` | List Make commands |

## 4. Run a production frontend build locally

Configure the real API environment first, then run:

```bash
npm run build
npm start
```

Open [http://localhost:3000](http://localhost:3000). The Go services are separate
processes and must also be running for connected features. Demo mode is always
disabled in a production build, so `/demo` returns 404 even if its environment
flag is set. Use `npm run dev:demo` to explore sample workflows.

## Troubleshooting

| Problem | What to check |
| --- | --- |
| Port 3000 is already in use | Stop the existing frontend, or run `npm run dev:demo -- --port 3002` and open `http://localhost:3002/demo`. Use only one development server per checkout. |
| `/demo` returns 404 | Start with `npm run dev:demo`, not `npm start` or plain `npm run dev`. |
| Demo class cannot be joined | Reset demo data to refresh the class window. Join as the assigned Student or Coach; the coach must enable student moves. |
| Different tabs show different records | Use the same browser profile, hostname, and port in both tabs. |
| Academy service unavailable / `ECONNREFUSED` | For sample workflows use demo mode. For the full stack, start the API and verify both API origin variables match its actual port. Restart services after changing `.env`. |
| Sign-in fails after switching hostname | Use `http://localhost:3000` consistently with the supplied full-stack configuration. If changing origins, update `KG_ALLOWED_ORIGINS` and public URLs too. |
| Dashboard returns 403 from `/learning/workspace` for an existing account | Restart the API after updating. Learning accepts existing UUIDv4 IDs as well as generated UUIDv7 IDs; do not recreate accounts or change roles to work around identifier validation. |
| Existing coach is rejected when creating a batch | Restart the API after updating. Existing UUIDv4 references are supported in batches, enrollment, credit grants and permissions. Restart realtime too for classroom access. |
| Enrollment requires guardian consent | Save academy legal/contact details and publish privacy + child-safety policies in Settings & access. The linked guardian must sign in and select Review consent under Learners & batches. Refresh the staff workspace before enrolling. The form displays these prerequisites before submission. |
| MFA verification expires during sign-in | The portal returns to sign-in with an expiry message and keeps your intended destination. Sign in again to start a new MFA challenge. Restart the API after updating to avoid old session-discovery behavior. |
| Switching accounts leads to an unavailable admin view | Sign-in now checks saved return destinations against the new account’s roles and falls back to its authorized dashboard. Refresh the frontend after updating; account permissions do not need to change. |
| No sign-in email arrives | The real backend needs an existing account. Check Mailpit at port 8025 and the API logs. Demo mode never sends email. |
| Docker cannot connect | Start Docker Desktop/the Docker daemon and confirm your user can access it. |
| Infrastructure remains unhealthy | Run `docker compose ps` and `docker compose logs --tail=100 postgres clamav`; wait for first-run initialization before starting the app. |
| Database connection or missing-table errors | Ensure PostgreSQL is healthy and run `make migrate`. If changing `KG_POSTGRES_PORT`, also update both host database URLs. |
| Playwright reports a missing browser | Run `npx playwright install chromium`. On Linux, missing system libraries can be installed with `npx playwright install --with-deps chromium`. |
| Browser tests use the wrong app or hit a development lock | Stop existing Next.js servers and run the two suites one at a time. |

For backend container status and logs:

```bash
docker compose --profile app ps
docker compose --profile app logs -f api worker realtime
```

## Repository map

The approved brown **Open Play** design is used throughout the application.
Run the local demo above to review it with working sample accounts. The original
logo is preserved in a circular crop across public pages, sign-in, workspaces,
and the classroom.

For future UI changes, start with [the brand guide](design/BRAND.md) and
[brand.config.json](design/brand.config.json). The root layout maps these tokens
to CSS variables; fonts and artwork are self-hosted in `public/brand/`.
Regression tests and their fixtures are versioned alongside the application.
Generated builds, test reports, screenshots, local exports, and private settings
are excluded from Git.

```text
app/                     Public website and role portals
app/lib/demo/            Sample accounts, mock transport, and persistent demo data
app/lib/brand.ts          Shared brand config to CSS variable mapping
design/                  Approved brand configuration, schema, and design guide
public/brand/            Self-hosted artwork and licensed fonts
backend/cmd/api/         Go REST API
backend/cmd/worker/      Background jobs
backend/cmd/realtime/    Classroom WebSocket gateway
backend/migrations/      PostgreSQL application migrations
backend/api/             REST and WebSocket contracts
sanity/                  Optional blog/resource CMS schemas
tests/                   Frontend unit and browser tests
docs/                    Walkthroughs, architecture, security, and deployment
compose.yaml             Local database, cache, storage, email, and malware scanner
```

Sanity manages editorial content only. The real Go backend uses PostgreSQL for
academy records; the demo uses separate browser storage.

## Deployment

The frontend and backend are intentionally independent deployments:

- Deploy the Next.js root at `kingsgambit2700.in` and
  `www.kingsgambit2700.in`.
- Build `backend/Dockerfile` targets `api`, `worker`, and `realtime` from the
  same revision. Use the `river-migrate` utility target before the worker, and
  route REST/WebSocket traffic through
  `api.kingsgambit2700.in`.
- Use a direct or session-pooled PostgreSQL connection for migrations and
  workers. PostgreSQL remains authoritative; Redis is never the durable
  classroom event source.

See [architecture](docs/architecture.md), [deployment](docs/deployment.md), and
[security](docs/security.md).

## Release boundary

The connected portals include learner/staff provisioning, consent, batches and
enrollment, scheduling/attendance, enquiries/trials, homework/resources, game
review, approved progress reports, guardian-visible communication and durable
notifications. Home/logo navigation preserves sign-in through revocable
remembered-browser restoration and explicit logout. Scheduling uses the real
credit ledger; classes support external HTTPS Zoom/Meet links.

See the [portal delivery and verification record](docs/portal-workspace-implementation.md)
for roles, the empty-academy journey, migrations 11–16 and precise test evidence.
Run API, worker and realtime together for the full workflow. The local teaching
engine's GPL license, corresponding source archive and build notice are served
from `public/engine/` and must remain available in deployments.

Checkout/subscription/refund automation and embedded video are separate work.
Recording, transcription, Google Calendar OAuth,
waitlists, advanced substitution, AI features, tournaments, schools,
marketplace, native apps, and multi-tenancy are deliberately excluded.

Checkout and child onboarding remain launch-gated until approved legal entity,
contact, policy, safeguarding, backup, and provider configuration is active.
See the [scope and launch gates](docs/release-scope.md).
For a route-by-route delivery summary, see
[implementation status](docs/implementation-status.md).
