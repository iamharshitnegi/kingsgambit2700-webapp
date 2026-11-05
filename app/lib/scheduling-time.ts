/** Resolves a wall-clock time without silently normalizing a daylight-saving gap/fold. */
export function localTeachingInstant(local: string, timeZone: string): string {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(local))
    throw new Error("Enter a complete local date and time.");
  const format = new Intl.DateTimeFormat("sv-SE", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });
  const base = Date.parse(`${local}:00Z`);
  if (!Number.isFinite(base)) throw new Error("Enter a valid date.");
  const wall = (instant: number) =>
    format.format(new Date(instant)).replace(" ", "T");
  const offsets = new Set(
    [-36, -12, 0, 12, 36].map((hours) => {
      const instant = base + hours * 3600000;
      return Date.parse(`${wall(instant)}:00Z`) - instant;
    }),
  );
  const candidates = [...offsets]
    .map((offset) => base - offset)
    .filter((instant) => wall(instant) === local);
  if (candidates.length !== 1)
    throw new Error(
      candidates.length
        ? "This local time occurs twice during daylight saving. Choose an unambiguous time."
        : "This local time does not exist during daylight saving. Choose a different time.",
    );
  return new Date(candidates[0]).toISOString();
}
