import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { DAY } from "../../shared/timeline.ts";
import { movePeriod, monthTotals, period, weekStart } from "./period.ts";

test("starts the week at Sunday midnight in Tokyo", () => {
  const midnight = Date.parse("2026-11-21T15:00:00Z");
  assert.equal(weekStart(midnight), midnight);
  assert.equal(weekStart(midnight - 1), midnight - 7 * DAY);
});

test("navigates calendar boundaries without skipping short months", () => {
  const lateJanuary = Date.parse("2028-01-31T23:30:00+09:00");
  const february = Date.parse("2028-02-01T00:00:00+09:00");
  const march = Date.parse("2028-03-01T00:00:00+09:00");
  assert.equal(movePeriod("month", lateJanuary, 1), february);
  assert.deepEqual(period("month", february), { start: february, end: march });
  const year = period("year", march);
  assert.equal(year.end - year.start, 366 * DAY);
  assert.equal(movePeriod("year", march, -1), Date.parse("2027-01-01T00:00:00+09:00"));
});

test("assigns year totals to their Tokyo calendar months", () => {
  const { start } = period("year", Date.parse("2028-12-31T15:00:00Z") - 1);
  const december = Date.parse("2028-12-01T00:00:00+09:00");
  const months = monthTotals(start, [
    { day: december, ms: 1200 },
    { day: december + DAY, ms: 300 },
  ]);
  assert.equal(months.length, 12);
  assert.deepEqual(months.at(-1), { day: december, ms: 1500 });
});
