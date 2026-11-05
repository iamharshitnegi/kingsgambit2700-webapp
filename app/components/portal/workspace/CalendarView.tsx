"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
} from "react";
import { Icon } from "@/app/components/Icon";
import type {
  AcademyClass,
  AcademyWorkspaceData,
} from "@/app/lib/academy-types";
import {
  addDays,
  calendarDates,
  canJoinClass,
  dateLabel,
  dayInZone,
  calendarSegments,
  eventLanes,
  isActiveClass,
  safeMeetingUrl,
  timeLabel,
  type CalendarMode,
} from "@/app/lib/calendar";
import { label, time } from "./WorkspaceUI";
import s from "./calendar.module.css";

function subscribePhone(callback: () => void) {
  const query = window.matchMedia("(max-width: 540px)");
  query.addEventListener("change", callback);
  return () => query.removeEventListener("change", callback);
}
const phoneSnapshot = () => window.matchMedia("(max-width: 540px)").matches;
const serverSnapshot = () => false;
const classFilters = ["upcoming", "completed", "cancelled", "all"] as const;
type ClassFilter = (typeof classFilters)[number];
function matchesStatus(item: AcademyClass, filter: ClassFilter, now: number) {
  return (
    filter === "all" ||
    (filter === "upcoming"
      ? isActiveClass(item) && Date.parse(item.endsAt) >= now
      : filter === "cancelled"
        ? item.status.startsWith("cancelled") || item.status === "rescheduled"
        : item.status === "completed")
  );
}
function Status({ lesson }: { lesson: AcademyClass }) {
  return (
    <span
      className={`${s.status} ${lesson.status === "completed" ? s.complete : isActiveClass(lesson) ? s.scheduled : s.cancelled}`}
    >
      <Icon
        name={lesson.status === "completed" ? "checkCircle" : "clock"}
        size={12}
      />
      {label(lesson.status)}
    </span>
  );
}

type Props = {
  data: AcademyWorkspaceData;
  page: "calendar" | "classes";
  compact: boolean;
  focus?: AcademyClass;
  timeZone: string;
  setTimeZone: (zone: string) => void;
  canJoin: boolean;
  canCancel: boolean;
  onCreate: () => void;
  onEdit: (lesson: AcademyClass) => void;
  onComplete: (lesson: AcademyClass) => void;
  onCancel: (lesson: AcademyClass) => void;
};
export default function CalendarView({
  data,
  page,
  compact,
  focus,
  timeZone: zone,
  setTimeZone,
  canJoin,
  canCancel,
  onCreate,
  onEdit,
  onComplete,
  onCancel,
}: Props) {
  const pathname = usePathname();
  const home = `/${pathname.split("/")[1]}`;
  const phone = useSyncExternalStore(
    subscribePhone,
    phoneSnapshot,
    serverSnapshot,
  );
  const [chosenMode, setMode] = useState<CalendarMode | null>(null);
  const mode = chosenMode || (phone ? "agenda" : "week");
  const [now, setNow] = useState(() => Date.now());
  const [day, setDay] = useState(() =>
    dayInZone(focus?.startsAt || Date.now(), zone),
  );
  const [query, setQuery] = useState("");
  const [coach, setCoach] = useState("all");
  const [format, setFormat] = useState("all");
  const [filter, setFilter] = useState<ClassFilter>("upcoming");
  const [count, setCount] = useState(12);
  const [selectedId, setSelected] = useState<string | null | undefined>(
    focus?.id,
  );
  const [detailTab, setDetailTab] = useState("overview");
  const [copyNotice, setCopyNotice] = useState("");
  const details = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLElement | null>(null);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30000);
    return () => clearInterval(timer);
  }, []);
  const today = dayInZone(now, zone);
  const dates = calendarDates(day, mode);
  const studentName = (id: string) =>
    data.students.find((item) => item.id === id)?.name || "Learner";
  const batchName = (lesson: AcademyClass) =>
    data.batches.find((item) => item.id === lesson.batchId)?.name ||
    "Class roster";
  const filtered = [...data.classes]
    .filter(
      (item) =>
        (coach === "all" || item.coachId === coach) &&
        (format === "all" || item.format === format) &&
        `${item.title} ${item.coach} ${batchName(item)} ${item.studentIds.map(studentName).join(" ")}`
          .toLowerCase()
          .includes(query.toLowerCase()),
    )
    .sort((a, b) => Date.parse(a.startsAt) - Date.parse(b.startsAt));
  const segments = calendarSegments(filtered, dates, zone);
  const visible = compact
    ? filtered
        .filter((item) => isActiveClass(item) && Date.parse(item.endsAt) >= now)
        .slice(0, 5)
    : page === "classes"
      ? filtered.filter((item) => matchesStatus(item, filter, now))
      : filtered.filter((item) =>
          segments.some((items) =>
            items.some((segment) => segment.lesson.id === item.id),
          ),
        );
  const displayed =
    page === "classes" && !compact ? visible.slice(0, count) : visible;
  // Prefer the current class, otherwise the nearest upcoming one. Explicitly closing stays closed.
  const initial =
    visible.find((item) => canJoinClass(item, now)) ||
    visible.find(
      (item) => isActiveClass(item) && Date.parse(item.endsAt) >= now,
    ) ||
    visible[0];
  const selected =
    selectedId === undefined && !phone
      ? initial
      : visible.find((item) => item.id === selectedId);
  const meeting = selected ? safeMeetingUrl(selected.meetingUrl) : null;
  const allSegments = segments.flat();
  const firstHour = allSegments.length
    ? Math.max(
        0,
        Math.min(...allSegments.map((item) => Math.floor(item.start / 60))) - 1,
      )
    : 9;
  const lastHour = Math.min(
    24,
    allSegments.length
      ? Math.max(
          firstHour + 6,
          ...allSegments.map((item) => Math.ceil(item.end / 60) + 1),
        )
      : 17,
  );
  const hours = Array.from(
    { length: lastHour - firstHour + 1 },
    (_, i) => firstHour + i,
  );
  const rowHeight = 76;
  const shortZone =
    new Intl.DateTimeFormat(undefined, {
      timeZone: zone,
      timeZoneName: "short",
    })
      .formatToParts(new Date(`${day}T12:00:00Z`))
      .find((p) => p.type === "timeZoneName")?.value || zone;
  function select(item: AcademyClass) {
    trigger.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    setSelected(item.id);
    setDetailTab("overview");
    setCopyNotice("");
    requestAnimationFrame(() => {
      details.current?.focus({ preventScroll: true });
      if (window.matchMedia("(max-width: 1100px)").matches)
        details.current?.scrollIntoView({
          block: "start",
          behavior: window.matchMedia("(prefers-reduced-motion: reduce)")
            .matches
            ? "auto"
            : "smooth",
        });
    });
  }
  function closeDetails() {
    setSelected(null);
    requestAnimationFrame(
      () => trigger.current?.isConnected && trigger.current.focus(),
    );
  }
  function navigate(amount: number) {
    if (mode === "month") {
      const date = new Date(`${day.slice(0, 7)}-01T12:00:00Z`);
      date.setUTCMonth(date.getUTCMonth() + amount);
      setDay(date.toISOString().slice(0, 10));
    } else setDay(addDays(day, amount * (mode === "day" ? 1 : 7)));
  }
  function clearFilters() {
    setQuery("");
    setCoach("all");
    setFormat("all");
    setCount(12);
  }
  function eventButton(
    item: AcademyClass,
    compactEvent = false,
    position?: CSSProperties,
  ) {
    return (
      <button
        key={item.id}
        style={position}
        className={`${s.event} ${item.format === "individual" ? s.individual : s.group} ${item.status === "completed" ? s.pastEvent : !isActiveClass(item) ? s.cancelledEvent : ""} ${selected?.id === item.id ? s.selectedEvent : ""} ${compactEvent ? s.compactEvent : ""}`}
        aria-label={`${item.title}, ${time(item.startsAt, zone)}, ${label(item.status)}. View class details`}
        aria-pressed={selected?.id === item.id}
        onClick={() => select(item)}
      >
        <span className={s.eventTime}>{timeLabel(item.startsAt, zone)}</span>
        <strong>{item.title}</strong>
        {!compactEvent && (
          <span className={s.eventBatch}>{batchName(item)}</span>
        )}
      </button>
    );
  }
  function agenda(items: AcademyClass[]) {
    if (!items.length)
      return (
        <div className={s.empty}>
          <Icon name="calendar" size={36} />
          <h3>A little room on the calendar.</h3>
          <p>
            {query || coach !== "all" || format !== "all"
              ? "No classes match these filters."
              : "No classes scheduled in this view yet."}
          </p>
          {query || coach !== "all" || format !== "all" ? (
            <button className={s.secondaryButton} onClick={clearFilters}>
              Clear filters
            </button>
          ) : data.capabilities.manageScheduling ? (
            <button className={s.secondaryButton} onClick={onCreate}>
              Schedule a class
            </button>
          ) : (
            <p>Your academy will share your next lesson here.</p>
          )}
        </div>
      );
    return Array.from(
      new Set(items.map((item) => dayInZone(item.startsAt, zone))),
    ).map((date) => (
      <section className={s.agendaDay} key={date}>
        <h3>
          {date === today ? "Today · " : ""}
          {dateLabel(date)}
        </h3>
        {items
          .filter((item) => dayInZone(item.startsAt, zone) === date)
          .map((item) => (
            <button
              className={`${s.agendaRow} ${selected?.id === item.id ? s.selectedRow : ""}`}
              key={item.id}
              onClick={() => select(item)}
              aria-label={`View ${item.title}`}
              aria-pressed={selected?.id === item.id}
            >
              <span className={s.agendaTime}>
                {timeLabel(item.startsAt, zone)}
                <small>
                  {(Date.parse(item.endsAt) - Date.parse(item.startsAt)) /
                    60000}{" "}
                  min
                </small>
              </span>
              <span
                className={`${s.typeMark} ${item.format === "individual" ? s.individual : s.group}`}
              >
                <Icon
                  name={item.format === "individual" ? "users" : "board"}
                  size={20}
                />
              </span>
              <span className={s.agendaTitle}>
                <strong>{item.title}</strong>
                <small>
                  {batchName(item)} · {item.coach}
                </small>
              </span>
              <span className={s.agendaStatus}>
                <Status lesson={item} />
                {!safeMeetingUrl(item.meetingUrl) && isActiveClass(item) && (
                  <small className={s.needsLink}>Meeting link needed</small>
                )}
              </span>
              <Icon name="right" size={17} />
            </button>
          ))}
      </section>
    ));
  }
  return (
    <section
      className={s.live}
      aria-label={compact ? "Upcoming classes" : "Class schedule"}
    >
      <div className={s.scheduleHeader}>
        <div>
          <h2>
            {compact
              ? "Your next classes"
              : page === "calendar"
                ? "A clear view of your week."
                : "Every lesson, in one place."}
          </h2>
          <p>
            {compact
              ? "Select a lesson for its meeting link and class details."
              : "Plan your lessons. Find your classroom. Keep learning moving."}
          </p>
        </div>
        {data.capabilities.manageScheduling && (
          <button className={s.primaryButton} onClick={onCreate}>
            <Icon name="plus" size={17} />
            Schedule class
          </button>
        )}
      </div>
      {!compact && (
        <>
          <div className={s.summary}>
            <span>
              <Icon name="calendar" size={15} />
              <strong>
                {
                  data.classes.filter(
                    (item) =>
                      dayInZone(item.startsAt, zone) === today &&
                      isActiveClass(item),
                  ).length
                }
              </strong>{" "}
              classes today
            </span>
            <span>
              <Icon name="clock" size={15} />
              <strong>
                {
                  data.classes.filter(
                    (item) =>
                      isActiveClass(item) && Date.parse(item.endsAt) >= now,
                  ).length
                }
              </strong>{" "}
              upcoming
            </span>
            <span>
              <Icon name="users" size={15} />
              <strong>{data.batches.length}</strong> batches
            </span>
          </div>
          <div className={s.workspaceTabs}>
            <nav className={s.pageTabs} aria-label="Schedule pages">
              <Link
                className={page === "calendar" ? s.activePageTab : ""}
                aria-current={page === "calendar" ? "page" : undefined}
                href={`${home}/calendar`}
              >
                <Icon name="calendar" size={17} />
                Calendar
              </Link>
              <Link
                className={page === "classes" ? s.activePageTab : ""}
                aria-current={page === "classes" ? "page" : undefined}
                href={`${home}/classes`}
              >
                <Icon name="classes" size={17} />
                Classes
              </Link>
            </nav>
            <label className={s.zoneSelect}>
              <Icon name="globe" size={15} />
              <select
                aria-label="Display timezone"
                value={zone}
                onChange={(e) => setTimeZone(e.target.value)}
              >
                {Array.from(
                  new Set([
                    zone,
                    "Asia/Kolkata",
                    "Asia/Dubai",
                    "Europe/London",
                    "America/New_York",
                    "UTC",
                  ]),
                ).map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </label>
          </div>
          <div className={s.filters}>
            <label className={s.search}>
              <Icon name="search" size={17} />
              <input
                aria-label="Search classes"
                type="search"
                placeholder="Search classes, learners, topics…"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setCount(12);
                }}
              />
            </label>
            <label className={s.selectFilter}>
              <Icon name="users" size={16} />
              <select
                aria-label="Filter by coach"
                value={coach}
                onChange={(e) => {
                  setCoach(e.target.value);
                  setCount(12);
                }}
              >
                <option value="all">All coaches</option>
                {data.coaches.map((item) => (
                  <option value={item.id} key={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={s.selectFilter}>
              <select
                aria-label="Class type"
                value={format}
                onChange={(e) => {
                  setFormat(e.target.value);
                  setCount(12);
                }}
              >
                <option value="all">All class types</option>
                {Array.from(
                  new Set(data.classes.map((item) => item.format)),
                ).map((item) => (
                  <option value={item} key={item}>
                    {label(item)}
                  </option>
                ))}
              </select>
            </label>
          </div>
        </>
      )}
      <div className={`${s.workspace} ${selected ? s.withDetails : ""}`}>
        <div className={s.calendarPanel}>
          {!compact && page === "classes" ? (
            <div
              className={s.listFilters}
              role="group"
              aria-label="Class status"
            >
              {classFilters.map((item) => (
                <button
                  key={item}
                  className={filter === item ? s.activeFilter : ""}
                  aria-pressed={filter === item}
                  onClick={() => {
                    setFilter(item);
                    setCount(12);
                  }}
                >
                  {item === "all"
                    ? "All classes"
                    : item[0].toUpperCase() + item.slice(1)}{" "}
                  <span>
                    {
                      filtered.filter((lesson) =>
                        matchesStatus(lesson, item, now),
                      ).length
                    }
                  </span>
                </button>
              ))}
            </div>
          ) : (
            !compact && (
              <div className={s.calendarToolbar}>
                <div className={s.dateNavigation}>
                  <button
                    className={s.iconButton}
                    aria-label="Previous period"
                    onClick={() => navigate(-1)}
                  >
                    <Icon name="left" size={16} />
                  </button>
                  <h2>
                    {mode === "month"
                      ? dateLabel(day, { month: "long", year: "numeric" })
                      : mode === "day"
                        ? dateLabel(day)
                        : `${dateLabel(dates[0], { day: "numeric", month: "short" })} – ${dateLabel(dates.at(-1)!, { day: "numeric", month: "short", year: "numeric" })}`}
                  </h2>
                  <button
                    className={s.iconButton}
                    aria-label="Next period"
                    onClick={() => navigate(1)}
                  >
                    <Icon name="right" size={16} />
                  </button>
                  <button
                    className={s.todayButton}
                    onClick={() => setDay(today)}
                  >
                    Today
                  </button>
                </div>
                <div
                  className={s.viewToggle}
                  role="group"
                  aria-label="Calendar view"
                >
                  {(["day", "week", "month", "agenda"] as const).map((item) => (
                    <button
                      key={item}
                      className={mode === item ? s.activeView : ""}
                      aria-pressed={mode === item}
                      onClick={() => setMode(item)}
                    >
                      {item[0].toUpperCase() + item.slice(1)}
                    </button>
                  ))}
                </div>
                <label className={s.jumpDate}>
                  Go to date
                  <input
                    type="date"
                    aria-label="Date"
                    value={day}
                    required
                    onChange={(e) => {
                      if (e.target.value) setDay(e.target.value);
                    }}
                  />
                </label>
              </div>
            )
          )}
          {compact || page === "classes" || mode === "agenda" ? (
            <div className={s.agenda}>{agenda(displayed)}</div>
          ) : mode === "month" ? (
            <div className={s.monthScroll}>
              <div className={s.monthGrid}>
                {["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"].map(
                  (name) => (
                    <div className={s.monthWeekday} key={name}>
                      {name}
                    </div>
                  ),
                )}
                {dates.map((date, i) => (
                  <section
                    key={date}
                    className={`${s.monthCell} ${date.slice(0, 7) !== day.slice(0, 7) ? s.otherMonth : ""}`}
                    aria-label={dateLabel(date)}
                  >
                    <button
                      className={date === today ? s.monthToday : ""}
                      aria-label={`Show ${dateLabel(date)}`}
                      onClick={() => {
                        setDay(date);
                        setMode("day");
                      }}
                    >
                      {Number(date.slice(-2))}
                    </button>
                    {segments[i]
                      .slice(0, 3)
                      .map((item) => eventButton(item.lesson, true))}
                    {segments[i].length > 3 && (
                      <button
                        className={s.moreEvents}
                        onClick={() => {
                          setDay(date);
                          setMode("day");
                        }}
                      >
                        +{segments[i].length - 3} more
                      </button>
                    )}
                  </section>
                ))}
              </div>
            </div>
          ) : (
            <div className={s.gridScroll}>
              <div
                className={`${s.timeGrid} ${mode === "day" ? s.singleDay : ""}`}
                style={{ "--days": dates.length } as CSSProperties}
              >
                <div className={s.timezoneLabel}>{shortZone}</div>
                {dates.map((date) => (
                  <button
                    key={date}
                    className={`${s.dayHeading} ${date === today ? s.todayHeading : ""}`}
                    onClick={() => {
                      setDay(date);
                      setMode("day");
                    }}
                    aria-label={`Show ${dateLabel(date)}`}
                  >
                    <span>{dateLabel(date, { weekday: "short" })}</span>
                    <b>{Number(date.slice(-2))}</b>
                  </button>
                ))}
                <div
                  className={s.hourLabels}
                  style={{ height: (lastHour - firstHour) * rowHeight }}
                >
                  {hours.map((hour) => (
                    <span
                      key={hour}
                      style={{ top: (hour - firstHour) * rowHeight }}
                    >{`${hour.toString().padStart(2, "0")}:00`}</span>
                  ))}
                </div>
                {dates.map((date, i) => (
                  <div
                    className={`${s.dayColumn} ${date === today ? s.todayColumn : ""}`}
                    key={date}
                    style={{
                      height: (lastHour - firstHour) * rowHeight,
                      backgroundSize: `100% ${rowHeight}px`,
                    }}
                  >
                    {eventLanes(segments[i]).map((item) =>
                      eventButton(item.lesson, false, {
                        top: (item.start / 60 - firstHour) * rowHeight,
                        height: Math.max(
                          24,
                          ((item.end - item.start) / 60) * rowHeight - 3,
                        ),
                        left: `calc(${(item.lane / item.lanes) * 100}% + 3px)`,
                        width: `calc(${100 / item.lanes}% - 6px)`,
                        right: "auto",
                      }),
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          {!compact &&
            page === "calendar" &&
            mode !== "agenda" &&
            !visible.length && (
              <p className={s.calendarEmptyNotice}>
                <Icon name="info" size={16} />
                No classes in this period
                {query || coach !== "all" || format !== "all"
                  ? " match your filters"
                  : ""}
                . Try another date or switch to Classes.
              </p>
            )}
          {page === "classes" && !compact && (
            <div className={s.listFooter}>
              <span>
                Showing {displayed.length} of {visible.length} classes
              </span>
              {visible.length > count && (
                <button
                  className={s.secondaryButton}
                  onClick={() => setCount((value) => value + 12)}
                >
                  Show more classes
                </button>
              )}
            </div>
          )}
          <footer className={s.calendarFooter}>
            <div className={s.legend}>
              <span>
                <i className={s.groupDot} />
                Group
              </span>
              <span>
                <i className={s.individualDot} />
                Individual
              </span>
            </div>
            {compact ? (
              <Link href={`${home}/calendar`}>
                View full calendar <Icon name="arrow" size={14} />
              </Link>
            ) : (
              <span>
                <Icon name="globe" size={13} />
                Times in {zone}
              </span>
            )}
          </footer>
        </div>
        {selected && (
          <div
            className={s.details}
            ref={details}
            tabIndex={-1}
            aria-label={`Class details: ${selected.title}`}
          >
            <header className={s.detailHero}>
              <div className={s.detailTop}>
                <span className={s.kindLabel}>
                  <Icon name="board" size={15} />
                  {label(selected.format)} class
                </span>
                <button
                  className={s.iconButton}
                  onClick={closeDetails}
                  aria-label="Close class details"
                >
                  <Icon name="close" size={18} />
                </button>
              </div>
              <h2>{selected.title}</h2>
              <p>{batchName(selected)}</p>
              <Status lesson={selected} />
            </header>
            <div
              className={s.detailTabs}
              role="group"
              aria-label="Class detail views"
            >
              {["overview", "learners", "notes"].map((tab) => (
                <button
                  key={tab}
                  className={detailTab === tab ? s.activeDetailTab : ""}
                  aria-pressed={detailTab === tab}
                  onClick={() => setDetailTab(tab)}
                >
                  {tab[0].toUpperCase() + tab.slice(1)}
                  {tab === "learners" && (
                    <span>{selected.studentIds.length}</span>
                  )}
                </button>
              ))}
            </div>
            <div className={s.detailContent}>
              {detailTab === "overview" && (
                <>
                  <div className={s.detailFact}>
                    <Icon name="calendar" size={19} />
                    <div>
                      <strong>
                        {dateLabel(dayInZone(selected.startsAt, zone), {
                          weekday: "long",
                          day: "numeric",
                          month: "long",
                        })}
                      </strong>
                      <span>
                        {timeLabel(selected.startsAt, zone)} –{" "}
                        {timeLabel(selected.endsAt, zone)}
                      </span>
                      <small>
                        {(Date.parse(selected.endsAt) -
                          Date.parse(selected.startsAt)) /
                          60000}{" "}
                        minutes · {zone}
                      </small>
                    </div>
                  </div>
                  <div
                    className={`${s.meetingCard} ${!meeting ? s.missingMeeting : ""}`}
                  >
                    <div className={s.meetingTitle}>
                      <Icon name="video" size={18} />
                      <strong>Class meeting</strong>
                      <span>
                        {meeting?.includes("zoom.us")
                          ? "Zoom"
                          : meeting?.includes("meet.google.com")
                            ? "Google Meet"
                            : "Video call"}
                      </span>
                    </div>
                    {meeting ? (
                      <>
                        <div className={s.meetingLink}>
                          <span>{meeting}</span>
                          <button
                            className={s.iconButton}
                            aria-label="Copy meeting link"
                            onClick={async () => {
                              try {
                                await navigator.clipboard.writeText(meeting);
                                setCopyNotice("Meeting link copied.");
                              } catch {
                                setCopyNotice(
                                  "Copy unavailable. Select and copy the link above.",
                                );
                              }
                            }}
                          >
                            <Icon name="copy" size={16} />
                          </button>
                        </div>
                        <a
                          href={meeting}
                          target="_blank"
                          rel="noopener noreferrer"
                          className={s.meetingButton}
                        >
                          Open meeting <Icon name="external" size={14} />
                        </a>
                        <p>Opens your coach’s video meeting in a new tab.</p>
                      </>
                    ) : (
                      <>
                        <p>The coach has not added a meeting link yet.</p>
                        {data.capabilities.manageScheduling &&
                          isActiveClass(selected) && (
                            <button onClick={() => onEdit(selected)}>
                              <Icon name="plus" size={15} />
                              Add meeting link
                            </button>
                          )}
                      </>
                    )}
                    {copyNotice && <p role="status">{copyNotice}</p>}
                  </div>
                  {canJoin && isActiveClass(selected) && (
                    <div className={s.joinBlock}>
                      {canJoinClass(selected, now) ? (
                        <Link
                          href={`/classroom/${selected.id}`}
                          className={s.primaryButton}
                        >
                          <Icon name="board" size={17} />
                          Join classroom
                        </Link>
                      ) : (
                        <button className={s.primaryButton} disabled>
                          <Icon name="board" size={17} />
                          Join classroom
                        </button>
                      )}
                      <small>
                        <Icon name="clock" size={13} />
                        {now >= Date.parse(selected.endsAt) + 900000
                          ? "This classroom’s join window has ended."
                          : canJoinClass(selected, now)
                            ? "Your shared teaching board is ready."
                            : `Opens ${time(new Date(Date.parse(selected.startsAt) - 900000).toISOString(), zone)}`}
                      </small>
                    </div>
                  )}
                  <div className={s.detailFact}>
                    <Icon name="users" size={19} />
                    <div>
                      <strong>{selected.coach}</strong>
                      <span>Your class coach</span>
                    </div>
                  </div>
                  <div className={s.detailFact}>
                    <Icon name="users" size={19} />
                    <div>
                      <strong>
                        {selected.studentIds.length}{" "}
                        {selected.studentIds.length === 1
                          ? "learner"
                          : "learners"}{" "}
                        enrolled
                      </strong>
                      <div className={s.avatars}>
                        {selected.studentIds.slice(0, 5).map((id, i) => (
                          <span
                            className={`${s.avatar} ${s[`avatar${i % 4}`]}`}
                            key={id}
                            title={studentName(id)}
                          >
                            {studentName(id)
                              .split(" ")
                              .map((word) => word[0])
                              .slice(0, 2)
                              .join("")}
                          </span>
                        ))}
                        <button onClick={() => setDetailTab("learners")}>
                          View roster <Icon name="right" size={12} />
                        </button>
                      </div>
                    </div>
                  </div>
                </>
              )}
              {detailTab === "learners" && (
                <>
                  <div className={s.rosterHeading}>
                    <h3>Class roster</h3>
                    <span>{selected.studentIds.length} enrolled</span>
                  </div>
                  {selected.studentIds.map((id) => (
                    <div className={s.learnerRow} key={id}>
                      <span className={s.avatar}>{studentName(id)[0]}</span>
                      <div>
                        <strong>{studentName(id)}</strong>
                        <small>
                          {selected.attendance[id]
                            ? label(selected.attendance[id])
                            : "Attendance not yet recorded"}
                        </small>
                      </div>
                    </div>
                  ))}
                  <p className={s.smallCopy}>
                    Attendance is confirmed by the coach.
                  </p>
                </>
              )}
              {detailTab === "notes" && (
                <div className={s.notesForm}>
                  <h3>Lesson notes</h3>
                  <p className={s.savedNotes}>
                    {selected.notes ||
                      "No lesson notes saved yet. Notes will appear here after the coach completes the class."}
                  </p>
                </div>
              )}
              {data.capabilities.manageTeaching &&
                isActiveClass(selected) &&
                now >= Date.parse(selected.startsAt) && (
                  <button
                    className={s.secondaryButton}
                    onClick={() => onComplete(selected)}
                  >
                    <Icon name="homework" size={16} />
                    Attendance & notes
                  </button>
                )}
            </div>
            {isActiveClass(selected) &&
              (data.capabilities.manageScheduling || canCancel) && (
                <footer className={s.detailFooter}>
                  {data.capabilities.manageScheduling && (
                    <button onClick={() => onEdit(selected)}>
                      <Icon name="edit" size={15} />
                      Edit class
                    </button>
                  )}
                  {canCancel && (
                    <button onClick={() => onCancel(selected)}>
                      Cancellation options
                    </button>
                  )}
                </footer>
              )}
          </div>
        )}
      </div>
    </section>
  );
}
