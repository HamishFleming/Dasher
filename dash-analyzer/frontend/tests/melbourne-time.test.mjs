import { test } from "node:test";
import assert from "node:assert/strict";
import { localDateTime } from "../src/lib/time.ts";
test("ledger dates are interpreted in Melbourne, independent of browser zone", () => {
  assert.equal(localDateTime("2026-07-06T19:00"), "2026-07-06T09:00:00.000Z");
  assert.equal(localDateTime("2026-01-05T19:00"), "2026-01-05T08:00:00.000Z");
});
test("rejects the missing spring DST hour and invalid dates", () => {
  assert.throws(() => localDateTime("2026-10-04T02:30"), /daylight saving/);
  assert.throws(() => localDateTime("2026-02-30T12:00"));
  assert.throws(() => localDateTime("invalid"));
});
test("repeated autumn hour chooses the earlier instant", () => {
  assert.equal(localDateTime("2026-04-05T02:30"), "2026-04-04T15:30:00.000Z");
});

test("map calendar selections use Melbourne calendar arithmetic across DST", async () => {
  const { melbourneTime, nextMelbourneDate, validCalendarDate } = await import(
    "../src/lib/time.ts"
  );
  assert.deepEqual(melbourneTime(new Date("2026-10-03T15:30:00Z")), {
    date: "2026-10-04",
    day: 0,
    hour: 1,
  });
  assert.equal(
    nextMelbourneDate(0, new Date("2026-10-03T02:00:00Z")),
    "2026-10-04",
  );
  assert.equal(
    nextMelbourneDate(1, new Date("2026-10-03T02:00:00Z")),
    "2026-10-05",
  );
  assert.ok(validCalendarDate("2028-02-29"));
  assert.equal(validCalendarDate("2026-02-30"), false);
});
