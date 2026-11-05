# Approved portal design integration

The calendar/classes design is applied to the authenticated academy workspace.
Live pages use the existing authorized academy/learning APIs.

## Available views

Each authorized role keeps its own base route (`/admin`, `/coach`, `/guardian`,
`/student`). The shared shell supplies grouped navigation, distinct outline
icons, the academy palette, readable headings, cards, forms, tables, dialogs,
and mobile navigation. This covers home, calendar, classes, learners/batches,
homework, the teaching board, resources, game reviews, progress, messages,
notifications, operations, settings, and support. Sidebar destinations remain
role-dependent. Multi-role workspace switching and explicit sign-out remain
available. Classroom controls keep their room-specific layout.

Calendar and Classes are separate routes over the same saved class records:

- `/calendar`: Monday-based week grid, day, month, and agenda; phones initially
  use Agenda. Date arrows, Today, direct date selection, timezone, coach/type
  filters, and class search are available. Overlaps get separate columns;
  classes crossing midnight appear on both dates in the selected timezone.
- `/classes`: chronological list with upcoming/completed/cancelled/all filters
  and incremental loading. Search includes topics, batches, coaches and learners.
- Selecting a class opens Overview, Learners and Notes. The saved meeting URL is
  visible, copyable and openable before class time; the shared classroom's join
  window is still enforced. Missing links show an explanation and a staff edit
  action when authorized. Mobile selection moves focus to the details panel.
- Scheduling presents details, then the existing server preview and confirmation.
  Returning to details retains entered fields. Successful creation or editing
  selects the saved class and its date. Attendance, cancellation previews,
  recurrence, make-ups, revisions and credit checks retain their existing API
  workflows. Notes are saved by the existing attendance/completion form.
- Home shows upcoming lessons and recent completed lesson notes separately.

## Integration boundaries

`PortalShell.tsx`, `portal-design.css`, and the shared `Icon.tsx` own navigation
and common styling. `CalendarView.tsx` / `calendar.module.css` render saved class
records; `CalendarPanel.tsx` owns authorized scheduling/completion/cancellation
forms. `app/lib/calendar.ts` supplies timezone display, date ranges, overlap
layout, and join-window helpers. No preview fixtures or mutations are imported
into production components.

The UI only exposes actions allowed by workspace capabilities; the API remains
the authorization authority. This change adds no database migration, API schema,
credit grant, authentication exception, or background service. Existing
production API/Realtime configuration and migrations through 000017 still apply.

## Local review

1. Run the normal frontend and local API stack and sign in at
   `http://localhost:3000/sign-in`.
2. Visit Calendar and Classes from your sidebar. Select an existing class and
   inspect its meeting URL, roster, and notes. A class without a URL will say so.
3. As an authorized coach/admin, schedule a class with available credits and a
   Zoom/Meet URL. Preview, go back to check retained fields, then confirm.
4. Verify the saved details after reloading. Classroom joining opens 15 minutes
   before start and closes 15 minutes after the end, as before.
5. Open the other workspace views, then repeat on a narrow/mobile window. Open
   the navigation drawer, tab through it, and dismiss with Escape.

## Verification

- `npm run check`: lint, TypeScript, and 190 unit tests.
- Production build uses `.next-workspace`, leaving `.next` development untouched.
- Real production browser journey uses an isolated PostgreSQL schema/Redis and
  exercises provisioning, consent, enrollment, credits, schedules, classroom
  synchronization, homework, reports, session restoration and logout.
- Demo journeys cover the same shared forms, calendar selection and persistent
  meeting URLs. The portal design tests check every workspace section at desktop
  and phone sizes, calendar modes, keyboard navigation and axe accessibility.
  The final calendar journey also passes at 320px without page overflow.

Browser screenshots/traces are written to `test-results/demo` and
`test-results-workspace`; they are local test artifacts, not deployment assets.
