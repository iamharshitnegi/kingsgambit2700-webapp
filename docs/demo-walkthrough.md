# Local demo walkthrough

Run `npm run dev:demo`, then open `http://localhost:3000/demo`. No backend,
Docker, account invitation, provider credentials, or email delivery is needed.
Use the same hostname and port in every tab; browser storage is origin-scoped.

The cream, charcoal and gold public landing page follows the last committed
site, with restored centred hero typography, working program links, responsive
navigation and concise parent-facing copy. UI decisions follow
[Vercel’s interface guidelines](https://vercel.com/design/guidelines): keyboard
access, visible focus, clear hierarchy, readable contrast, useful feedback and
responsive layouts.

## Accounts

The demo launcher signs in directly. The normal sign-in form also accepts these
sample emails and offers a local continuation instead of sending a real email.

| Role | Sample email | Main workflow |
| --- | --- | --- |
| Guardian | guardian@example.test | Children, consent, schedule, reports, invoices, support |
| Student | student@example.test | Classroom and homework; student code `DEMO2700` |
| Coach | coach@example.test | Shared board, student control, feedback, attendance and notes |
| Operations | operations@example.test | Enquiry stages, trial queue, cancellation and support |
| Academics | academics@example.test | Review assignments and publish draft reports |
| Finance | finance@example.test | Invoice payment simulation and credit balances |
| Super admin | admin@example.test | Combined staff views and activity timeline |

## Try a complete learning journey

1. From `/free-trial`, fill in a fictional guardian and student age, choose a
   slot, provide a contact address and consent, then confirm. Switch to
   Operations; the trial and enquiry appear together. Change its stage and
   refresh to confirm it persists.
2. Open Guardian, expand **Add a child**, create a learner and save their policy
   consent. New learners start without consent; the sample Aarav already has
   consent and an assigned class.
3. Open **Review payment**. Simulate a failed payment, then a successful one.
   Failure leaves the invoice and credits unchanged; success marks the invoice
   paid and adds eight credits once. Refresh to verify persistence.
4. Open Student, explain the homework answer and submit practice. Switch to
   Coach, review the response and share feedback. Reopen Student to see it.
5. Open Coach in one tab and Student in another. Join the scheduled classroom
   and click **Join classroom board**. Coach can make moves, flip the board,
   load FEN/PGN, undo and download PGN. Enable **Allow student moves**; the
   student can now move. Both tabs receive position changes. Notation input is
   the keyboard alternative to drag-and-drop.
6. After teaching, Coach resolves every attendance entry and saves lesson
   notes using **Complete class**. Guardian sees the completed class and notes.
   One class credit is consumed; repeat completion is rejected.
7. Academics publishes **Building a thinking routine**. Before publication,
   guardian/student accounts cannot see this draft. Guardian can mark the
   published report as read.
8. Guardian opens Support and creates a case. Operations can reply, add an
   internal note, assign it and change status. Reopen Guardian: internal notes
   stay hidden. Conflicting edits require reloading the latest case.
9. Guardian or Operations can cancel a future class, with a confirmation and
   the sample 24-hour cutoff. Cancelling within that window returns a useful
   error and leaves the schedule unchanged.
10. Try `/admin` while signed in as Student; access is denied. Sign out, and
    protected routes return to sign-in. Reset the demo to repeat the scenarios.

## Persistence and boundaries

- Demo records live in `kingsgambit.demo.v1` in localStorage. Each tab’s role is
  stored in sessionStorage. Reset affects sample records in all same-origin
  tabs. This is a local simulator, not a security boundary against someone who
  can edit browser storage.
- Demo mode requires an explicit flag and a non-production environment. It
  cannot grant access to real accounts or call real payment/email providers.
- The initial class join window is relative to seed/reset time. Reset if the
  class has expired. All timestamps are ISO UTC; UI times use the viewer’s IANA
  time zone.
- The sample classroom uses one shared board. It synchronizes tabs in one
  browser; separate browsers/devices require the real backend realtime stack.
- New children can be created and consented but do not receive automatic
  placement, class assignments, or generated access codes in this preview.
- Video/audio, real invoices and tax calculations, payment webhooks, MFA
  devices, recurring scheduling, files, recordings and external notifications
  require the real providers/backend. No production readiness is implied by a
  successful simulated workflow. See [implementation status](implementation-status.md).

## Automated checks

```bash
npm run check
npx playwright install chromium
npm run test:demo
npm run test:e2e
npm run build
```

Run browser suites sequentially with existing development servers stopped.
`test:demo` starts its server on 3001; `test:e2e` uses 3000 and checks the real
HTTP-client flow with route fixtures. Unit tests verify the demo’s state
transitions, permissions and failures. Production Go regression tests exercise
the real services independently; use `make go-race` and the PostgreSQL
integration instructions in the backend documentation.

## Homework design and samples

Open `/student/homework` for **My homework**. Students see only their own
published assignments; there is no learner selector. The compact list groups
work into **To do**, **In review**, and **Completed**, with search, activity
filters, a detail pane, and a resume shortcut. On phones, opening an assignment
shows its details with **Back to homework** navigation.

Eight additional samples are added once, including five playable checkmate
positions (two already solved), a returned game review, opposition study with
attached notes, opening reflection, a response awaiting review, completed work
with feedback, practice evidence, and a staff-only draft. Existing demo edits
are preserved. Dates are relative to when these samples are first added.

Coaches use **Create homework** in three steps: choose the activity and content,
select learners or a batch and an optional due date, then review and save a
private draft or assign it. Example starters are available for study, game
review, and puzzles. Puzzle solutions accept legal SAN or UCI lines. Published
library material is attached as a versioned snapshot. Drafts can be edited and
assigned later.

Student puzzle progress and uploaded files persist automatically. Unsubmitted
reflection text stays only on the current page; submit it before leaving if it
is ready for the coach. A returned assignment keeps its previous attempt and
feedback while allowing a fresh attempt.
