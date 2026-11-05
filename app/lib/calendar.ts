import type { AcademyClass } from "./academy-types";

export type CalendarMode = "day" | "week" | "month" | "agenda";
export function dayInZone(value: string | number, timeZone: string) {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(value));
}
export function addDays(day: string, count: number) {
  const date = new Date(`${day}T12:00:00Z`);
  date.setUTCDate(date.getUTCDate() + count);
  return date.toISOString().slice(0, 10);
}
export function calendarDates(day: string, mode: CalendarMode) {
  let start = day;
  let count = mode === "day" ? 1 : 7;
  if (mode === "month") {
    start = `${day.slice(0, 7)}-01`;
    count = 42;
  }
  if (mode === "month" || mode === "week")
    start = addDays(
      start,
      -((new Date(`${start}T12:00:00Z`).getUTCDay() + 6) % 7),
    );
  return Array.from({ length: count }, (_, i) => addDays(start, i));
}
export function dateLabel(
  day: string,
  options: Intl.DateTimeFormatOptions = {
    weekday: "short",
    day: "numeric",
    month: "short",
  },
) {
  return new Intl.DateTimeFormat(undefined, {
    ...options,
    timeZone: "UTC",
  }).format(new Date(`${day}T12:00:00Z`));
}
export function timeLabel(value: string | number, timeZone: string) {
  return new Intl.DateTimeFormat(undefined, {
    timeZone,
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}
export function minuteInZone(value: string | number, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(value));
  return (
    Number(parts.find((p) => p.type === "hour")?.value) * 60 +
    Number(parts.find((p) => p.type === "minute")?.value)
  );
}
export function isActiveClass(item: AcademyClass) {
  return ![
    "completed",
    "cancelled_family",
    "cancelled_academy",
    "rescheduled",
  ].includes(item.status);
}
export function canJoinClass(item: AcademyClass, now: number) {
  return (
    isActiveClass(item) &&
    now >= Date.parse(item.startsAt) - 900000 &&
    now < Date.parse(item.endsAt) + 900000
  );
}
export function safeMeetingUrl(value: string) {
  try {
    const url = new URL(value);
    return ["https:", "http:"].includes(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
}

// Split classes at local midnight so every occurrence remains visible in its display timezone.
export function daySegments(
  classes: AcademyClass[],
  day: string,
  zone: string,
) {
  return calendarSegments(classes, [day], zone)[0];
}

export function calendarSegments(
  classes: AcademyClass[],
  days: string[],
  zone: string,
) {
  // Resolve each saved instant once per range, including the 42-cell month view.
  const projected = classes.map((lesson) => ({
    lesson,
    first: dayInZone(lesson.startsAt, zone),
    last: dayInZone(Date.parse(lesson.endsAt) - 1, zone),
    endDay: dayInZone(lesson.endsAt, zone),
    startMinute: minuteInZone(lesson.startsAt, zone),
    endMinute: minuteInZone(lesson.endsAt, zone),
  }));
  return days.map((day) =>
    projected.flatMap((item) => {
      if (day < item.first || day > item.last) return [];
      const start = day === item.first ? item.startMinute : 0;
      const end = day === item.endDay ? item.endMinute : 1440;
      return [
        {
          lesson: item.lesson,
          start,
          end: Math.min(1440, Math.max(start + 15, end)),
        },
      ];
    }),
  );
}

export function eventLanes(segments: ReturnType<typeof daySegments>) {
  const sorted = [...segments].sort(
    (a, b) => a.start - b.start || b.end - a.end,
  );
  const result: ((typeof sorted)[number] & { lane: number; lanes: number })[] =
    [];
  let group: typeof result = [],
    ends: number[] = [],
    groupEnd = -1;
  function flush() {
    for (const item of group) item.lanes = ends.length;
    result.push(...group);
    group = [];
    ends = [];
  }
  for (const item of sorted) {
    if (item.start >= groupEnd) flush();
    let lane = ends.findIndex((end) => end <= item.start);
    if (lane < 0) lane = ends.length;
    ends[lane] = item.end;
    group.push({ ...item, lane, lanes: 1 });
    groupEnd = Math.max(...ends);
  }
  flush();
  return result;
}
