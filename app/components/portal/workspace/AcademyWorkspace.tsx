"use client";

import Link from "next/link";
import { Icon } from "@/app/components/Icon";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { completeMFAChallenge } from "@/app/lib/auth-client";
import { APIError, apiFetch } from "@/app/lib/api/client";
import type { SessionUser } from "@/app/lib/api/types";
import {
  academyLevels,
  sectionLabels,
  type AcademyMutation,
  type AcademyWorkspaceData,
  type LearningAction,
  type LearningWorkspace,
  type WorkspaceSection,
} from "@/app/lib/academy-types";
import { requestID } from "@/app/lib/request-id";
import { workspaceHome } from "@/app/lib/role-routing";
import CalendarPanel from "./CalendarPanel";
import LearnersPanel from "./LearnersPanel";
import HomeworkPanel from "./HomeworkPanel";
import TeachingBoard from "./TeachingBoard";
import LibraryPanel from "./LibraryPanel";
import GamesPanel from "./GamesPanel";
import ProgressPanel from "./ProgressPanel";
import MessagesPanel from "./MessagesPanel";
import SettingsPanel from "./SettingsPanel";
import OperationsPanel from "./OperationsPanel";
import {
  Badge,
  Empty,
  Field,
  Modal,
  Panel,
  WorkspaceFeedback,
  formValues,
  time,
  value,
} from "./WorkspaceUI";
import "./workspace.css";

export default function AcademyWorkspace({
  user,
  section = "home",
  kind,
}: {
  user: SessionUser;
  section?: WorkspaceSection;
  kind: "guardian" | "student" | "coach" | "admin";
}) {
  const [data, setData] = useState<AcademyWorkspaceData | null>(null);
  const [learning, setLearning] = useState<LearningWorkspace | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");
  const [studySave, setStudySave] = useState<string | null>(null);
  const recordLimit = useRef(500);
  const [loadingMore, setLoadingMore] = useState(false);
  const home = workspaceHome(user, `/${kind}`);
  const refresh = useCallback(async () => {
    const results = await Promise.allSettled([
      apiFetch<AcademyWorkspaceData>(
        `/academy/workspace?limit=${recordLimit.current}`,
      ),
      apiFetch<LearningWorkspace>(
        `/learning/workspace?limit=${recordLimit.current}`,
      ),
    ]);
    if (results[0].status === "fulfilled") setData(results[0].value);
    else
      setError(
        results[0].reason instanceof Error
          ? results[0].reason.message
          : "Your academy could not be loaded.",
      );
    if (results[1].status === "fulfilled") setLearning(results[1].value);
    else
      setError(
        results[1].reason instanceof Error
          ? results[1].reason.message
          : "Your learning records could not be loaded.",
      );
    setLoading(false);
  }, []);
  useEffect(() => {
    void refresh();
    queueMicrotask(() =>
      setSearch(new URLSearchParams(window.location.search).get("q") || ""),
    );
  }, [refresh]);
  const pending = useRef(false);
  const requestKeys = useRef(new Map<string, string>());
  function mutationKey(fingerprint: string, supplied?: unknown) {
    const existing = requestKeys.current.get(fingerprint);
    if (existing) return existing;
    const key =
      typeof supplied === "string" && supplied ? supplied : requestID();
    requestKeys.current.set(fingerprint, key);
    return key;
  }
  const [mfa, setMFA] = useState(false);
  const [mfaError, setMFAError] = useState("");
  const [verifying, setVerifying] = useState(false);
  const mfaResolve = useRef<((ok: boolean) => void) | null>(null);
  async function authenticatedMutation<T>(
    path: string,
    options: Parameters<typeof apiFetch>[1],
  ): Promise<T> {
    try {
      return await apiFetch<T>(path, options);
    } catch (failure) {
      if (
        !(failure instanceof APIError) ||
        failure.problem?.code !== "recent_mfa_required"
      )
        throw failure;
      const verified = await new Promise<boolean>((resolve) => {
        mfaResolve.current = resolve;
        setMFAError("");
        setMFA(true);
      });
      if (!verified)
        throw new Error(
          "Verification was cancelled. Your change has not been saved.",
        );
      return apiFetch<T>(path, options);
    }
  }
  const mutate: AcademyMutation = async (
    path,
    input,
    success,
    method = "POST",
  ) => {
    if (pending.current) return false;
    pending.current = true;
    setBusy(true);
    setError("");
    setNotice("");
    const fingerprint = `${method}:${path}:${JSON.stringify(input)}`;
    try {
      await authenticatedMutation(path, {
        method,
        body: JSON.stringify(input),
        headers: {
          "Idempotency-Key": mutationKey(
            fingerprint,
            input.idempotencyKey || input.referenceId,
          ),
        },
      });
      requestKeys.current.delete(fingerprint);
      await refresh();
      setNotice(success);
      return true;
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The change could not be saved.",
      );
      if (cause instanceof APIError && cause.status === 409) await refresh();
      return false;
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };
  const act: LearningAction = async (action, input, success = "Saved.") => {
    if (pending.current) return false;
    pending.current = true;
    setBusy(true);
    setError("");
    if (success) setNotice("");
    const fingerprint = JSON.stringify({ action, ...input });
    try {
      setLearning(
        await apiFetch<LearningWorkspace>("/learning/actions", {
          method: "POST",
          body: fingerprint,
          headers: { "Idempotency-Key": mutationKey(fingerprint) },
        }),
      );
      requestKeys.current.delete(fingerprint);
      await refresh();
      if (success) setNotice(success);
      return true;
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The change could not be saved.",
      );
      if (cause instanceof APIError && cause.status === 409) await refresh();
      return false;
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };
  const results = useMemo(() => {
    if (!search.trim() || !data || !learning) return [];
    const items = [
      ...data.students.map((item) => ({
        id: item.id,
        title: item.name,
        section: "students",
      })),
      ...data.batches.map((item) => ({
        id: item.id,
        title: item.name,
        section: "students",
      })),
      ...data.classes.map((item) => ({
        id: item.id,
        title: item.title,
        section: "calendar",
      })),
      ...learning.assignments.map((item) => ({
        id: item.id,
        title: item.title,
        section: "homework",
      })),
      ...learning.resources.map((item) => ({
        id: item.id,
        title: item.title,
        section: "library",
      })),
      ...learning.games.map((item) => ({
        id: item.id,
        title: item.title,
        section: "games",
      })),
      ...learning.assessments.map((item) => ({
        id: item.id,
        title: item.title,
        section: "progress",
      })),
    ];
    return items
      .filter((item) => item.title.toLowerCase().includes(search.toLowerCase()))
      .slice(0, 20);
  }, [search, data, learning]);
  if (loading)
    return (
      <div className="workspace-loading" role="status">
        <div className="portal-loader" />
        <p>Opening your academy…</p>
      </div>
    );
  if (!data)
    return (
      <Panel title="Your academy could not be loaded">
        <p role="alert">{error}</p>
        <button
          className="button button-dark"
          onClick={() => {
            setError("");
            void refresh();
          }}
        >
          Try again
        </button>
      </Panel>
    );
  const canJoin =
    kind !== "guardian" &&
    !user.roles.every((role) => role === "finance_admin");
  return (
    <WorkspaceFeedback value={{ error, busy }}>
      <div className="academy-workspace-v2" aria-busy={busy}>
        {section !== "homework" && <div className="academy-workspace-toolbar">
          <Field label="Search your academy">
            <input
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Learners, classes, homework, resources…"
            />
          </Field>
          <Link
            className="academy-notification-link"
            href={`${home}/notifications`}
          >
            <Icon name="bell" size={17} /> Notifications{" "}
            <Badge>
              {learning?.notifications.filter((item) => !item.readAt).length ||
                0}
            </Badge>
          </Link>
          <button
            className="workspace-refresh"
            disabled={busy}
            onClick={() => {
              setError("");
              void refresh();
            }}
          >
            <Icon name="refresh" size={16} /> Refresh
          </button>
        </div>}
        {search && section !== "homework" && (
          <section
            className="academy-search-results"
            aria-label="Search results"
          >
            {results.length ? (
              results.map((item) => (
                <Link
                  key={`${item.section}-${item.id}`}
                  href={`${home}/${item.section}`}
                  onClick={() => setSearch("")}
                >
                  <strong>{item.title}</strong>
                  <small>
                    {sectionLabels[item.section as WorkspaceSection]}
                  </small>
                </Link>
              ))
            ) : (
              <p>No matching records in your workspace.</p>
            )}
          </section>
        )}
        {error && (
          <div
            className="form-message form-message-error academy-global-error"
            role="alert"
          >
            {error}
            <button onClick={() => setError("")} aria-label="Dismiss error">
              ×
            </button>
          </div>
        )}
        {notice && (
          <p className="workspace-notice" role="status">
            {notice}
          </p>
        )}
        {(data.pagination.hasMore || learning?.pagination.hasMore) && (
          <div className="academy-toolbar" role="status">
            <p>
              Showing the first {Math.max(data.pagination.limit, learning?.pagination.limit || 0)} records in each
              collection. Load more to include older records in lists and
              search.
            </p>
            <button
              disabled={busy || loadingMore}
              onClick={async () => {
                setLoadingMore(true);
                recordLimit.current = Math.max(data.pagination.nextLimit, learning?.pagination.nextLimit || 0);
                await refresh();
                setLoadingMore(false);
              }}
            >
              {loadingMore ? "Loading records…" : "Load more records"}
            </button>
          </div>
        )}
        {section === "home" && (
          <>
            <div className="academy-welcome">
              <div>
                <span className="eyebrow">
                  Hello, {user.displayName?.split(" ")[0] || "there"}
                </span>
                <h2>A clear plan. A stronger next move.</h2>
                <p>
                  Your lessons, practice, and learning conversations in one
                  place.
                </p>
              </div>
              <div className="academy-actions">
                {data.capabilities.manageScheduling && (
                  <Link
                    className="button button-dark"
                    href={`${home}/calendar`}
                  >
                    Schedule a class
                  </Link>
                )}
                {data.capabilities.manageTeaching && (
                  <Link
                    className="button button-quiet"
                    href={`${home}/homework`}
                  >
                    Create homework
                  </Link>
                )}
              </div>
            </div>
            <div className="academy-stat-grid">
              {[
                [
                  "Scheduled classes",
                  data.workload.scheduledClasses,
                  "calendar",
                ],
                [
                  "Needs completion",
                  data.workload.pendingCompletion,
                  "calendar",
                ],
                [
                  "Practice to review",
                  learning?.assignments.filter(
                    (item) => item.attempts.at(-1)?.status === "submitted",
                  ).length || 0,
                  "homework",
                ],
                [
                  "Feedback awaiting review",
                  learning?.assessments.filter(
                    (item) => item.status === "submitted",
                  ).length || 0,
                  "progress",
                ],
              ].map(([title, count, target]) => (
                <Link key={title} href={`${home}/${target}`}>
                  <small>{title}</small>
                  <strong>{count}</strong>
                </Link>
              ))}
            </div>
            <CalendarPanel
              compact
              data={data}
              mutate={mutate}
              canJoin={canJoin}
              canCancel={
                data.capabilities.manageScheduling ||
                user.roles.includes("guardian")
              }
            />
            {data.classes.some(item => item.status === "completed" && item.notes) && <Panel title="Recent lesson notes" action={<Link className="text-link" href={`${home}/classes`}>View classes <Icon name="arrow" size={15} /></Link>}>
              {[...data.classes].filter(item => item.status === "completed" && item.notes).sort((a, b) => Date.parse(b.endsAt) - Date.parse(a.endsAt)).slice(0, 3).map(item => <article className="academy-list-row" key={item.id}><div><Badge>Completed</Badge><h3>{item.title}</h3><small>{time(item.startsAt, data.profile.timeZone || undefined)} · {item.coach}</small><p className="academy-prose">{item.notes}</p></div></article>)}
            </Panel>}
            {data.capabilities.manageTeaching && (
              <Panel title="Teaching workload">
                <div className="academy-stat-grid">
                  {data.workload.months.map((period) => (
                    <div key={period.month}>
                      <small>{period.month} · completed teaching hours</small>
                      <strong>
                        {(period.completedClassMinutes / 60).toFixed(1)}
                      </strong>
                      <p>
                        {period.completedClasses} classes ·{" "}
                        {(period.completedTrialMinutes / 60).toFixed(1)} trial
                        hours ({period.completedTrials} trials)
                      </p>
                    </div>
                  ))}
                </div>
                <p>
                  UTC month totals use saved completed lessons and trials. Open
                  classes to review attendance and notes.
                </p>
              </Panel>
            )}
            <div className="academy-card-grid">
              {(
                [
                  "board",
                  "library",
                  "homework",
                  "progress",
                  "messages",
                ] as WorkspaceSection[]
              )
                .filter(
                  (item) =>
                    data.capabilities.manageTeaching ||
                    !["board", "library"].includes(item),
                )
                .map((item) => (
                  <Link
                    className="academy-feature-link"
                    key={item}
                    href={`${home}/${item}`}
                  >
                    <span className="eyebrow">Keep learning moving</span>
                    <h3>{sectionLabels[item]}</h3>
                    <span>Open workspace →</span>
                  </Link>
                ))}
            </div>
          </>
        )}
        {(section === "calendar" || section === "classes") && (
          <CalendarPanel
            page={section === "classes" ? "classes" : "calendar"}
            data={data}
            mutate={mutate}
            canJoin={canJoin}
            canCancel={
              data.capabilities.manageScheduling ||
              user.roles.includes("guardian")
            }
          />
        )}
        {section === "students" && (
          <LearnersPanel data={data} user={user} mutate={mutate} />
        )}
        {section === "homework" && learning && (
          <HomeworkPanel
            key={`${user.id}-${kind}`}
            data={data}
            learning={learning}
            user={user}
            kind={kind}
            act={act}
            onUploaded={setLearning}
            onRefresh={() => { setError(""); void refresh(); }}
          />
        )}
        {section === "board" && (
          <TeachingBoard
            onSave={
              learning?.canCreateResources
                ? async (pgn) => {
                    setStudySave(pgn);
                    return false;
                  }
                : undefined
            }
          />
        )}
        {section === "library" && learning && (
          <LibraryPanel
            learning={learning}
            act={act}
            onUploaded={setLearning}
            userId={user.id}
          />
        )}
        {section === "games" && learning && (
          <GamesPanel learning={learning} act={act} data={data} />
        )}
        {section === "progress" && learning && (
          <ProgressPanel
            learning={learning}
            act={act}
            onUploaded={setLearning}
            guardian={user.roles.includes("guardian")}
          />
        )}
        {section === "messages" && learning && (
          <MessagesPanel
            data={data}
            learning={learning}
            act={act}
            refresh={refresh}
          />
        )}
        {section === "operations" && (
          <OperationsPanel data={data} mutate={mutate} />
        )}
        {section === "settings" && (
          <SettingsPanel
            data={data}
            user={user}
            mutate={mutate}
            refresh={refresh}
          />
        )}
        {section === "notifications" && learning && (
          <Panel
            title="Notifications"
            action={
              <button
                onClick={() =>
                  void act(
                    "readAllNotifications",
                    {},
                    "Notifications marked as read.",
                  )
                }
              >
                Mark all as read
              </button>
            }
          >
            {learning.notifications.map((item) => (
              <article key={item.id} className="academy-list-row">
                <div>
                  {!item.readAt && <Badge>Unread</Badge>}
                  <h3>{item.title}</h3>
                  <time dateTime={item.createdAt}>{time(item.createdAt)}</time>
                </div>
                <Link
                  href={
                    item.href.startsWith("/") && !item.href.startsWith("//")
                      ? item.href
                      : home
                  }
                  onClick={() =>
                    void act("readNotification", { id: item.id }, "")
                  }
                >
                  Open
                </Link>
              </article>
            ))}
            {!learning.notifications.length && (
              <Empty>
                You are up to date. New class and learning updates will appear
                here.
              </Empty>
            )}
          </Panel>
        )}
        {!learning &&
          [
            "homework",
            "library",
            "games",
            "progress",
            "messages",
            "notifications",
          ].includes(section) && (
            <Empty>
              Learning records could not be loaded. Use Refresh to try again.
            </Empty>
          )}
        {studySave && (
          <Modal title="Save your study" close={() => setStudySave(null)}>
            <form
              className="academy-form"
              onSubmit={async (event) => {
                const form = formValues(event);
                if (
                  await act(
                    "createResource",
                    {
                      title: value(form, "title"),
                      kind: "pgn",
                      visibility: "personal",
                      status: "draft",
                      levelSlug: value(form, "levelSlug"),
                      tags: ["classwork"],
                      content: studySave,
                    },
                    "Study saved in your personal library.",
                  )
                )
                  setStudySave(null);
              }}
            >
              <Field label="Study title">
                <input name="title" required maxLength={200} />
              </Field>
              <Field label="Level">
                <select name="levelSlug">
                  {academyLevels.map((item) => (
                    <option key={item.slug} value={item.slug}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </Field>
              <button className="button button-dark" disabled={busy}>
                Save study
              </button>
            </form>
          </Modal>
        )}
        {mfa && (
          <Modal
            independent
            title="Verify this sensitive change"
            close={() => {
              mfaResolve.current?.(false);
              setMFA(false);
            }}
          >
            <form
              className="academy-form"
              onSubmit={async (event) => {
                const form = formValues(event);
                setVerifying(true);
                setMFAError("");
                try {
                  await completeMFAChallenge({ code: value(form, "code") });
                  setMFA(false);
                  mfaResolve.current?.(true);
                } catch (cause) {
                  setMFAError(
                    cause instanceof Error
                      ? cause.message
                      : "Verification failed.",
                  );
                } finally {
                  setVerifying(false);
                }
              }}
            >
              <p>
                Enter a fresh code from your authenticator to continue. Your
                form is kept open.
              </p>
              <Field label="Authenticator code">
                <input
                  name="code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  required
                  autoFocus
                />
              </Field>
              {mfaError && <p role="alert">{mfaError}</p>}
              <button className="button button-dark" disabled={verifying}>
                Verify & save
              </button>
            </form>
          </Modal>
        )}
      </div>
    </WorkspaceFeedback>
  );
}
