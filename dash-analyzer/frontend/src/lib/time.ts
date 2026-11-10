export function localDateTime(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!match) throw Error("Enter valid Melbourne start and end times.");
  const [, y, m, d, h, min] = match;
  const naive = Date.UTC(+y, +m - 1, +d, +h, +min);
  const format = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Australia/Melbourne",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });
  // Test both Melbourne UTC offsets. Nonexistent DST hours have no match;
  // an ambiguous autumn hour chooses the earlier matching instant.
  const matches = [11, 10]
    .map((offset) => new Date(naive - offset * 3600000))
    .filter((instant) => {
      const p = Object.fromEntries(
        format.formatToParts(instant).map((x) => [x.type, x.value]),
      );
      return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}` === value;
    });
  if (!matches.length)
    throw Error(
      "That Melbourne time does not exist because of daylight saving.",
    );
  return matches[0].toISOString();
}

export function validCalendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const instant = new Date(value + "T12:00:00Z");
  return (
    Number.isFinite(instant.getTime()) &&
    instant.toISOString().slice(0, 10) === value
  );
}
export function melbourneTime(now = new Date()) {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-AU", {
      timeZone: "Australia/Melbourne",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      weekday: "short",
      hour: "numeric",
      hourCycle: "h23",
    })
      .formatToParts(now)
      .map((p) => [p.type, p.value]),
  );
  return {
    date: `${parts.year}-${parts.month}-${parts.day}`,
    day: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].indexOf(
      parts.weekday,
    ),
    hour: Number(parts.hour),
  };
}
export function nextMelbourneDate(day: number, now = new Date()): string {
  const local = melbourneTime(now);
  const calendar = new Date(local.date + "T12:00:00Z");
  calendar.setUTCDate(calendar.getUTCDate() + ((day - local.day + 7) % 7));
  return calendar.toISOString().slice(0, 10);
}
