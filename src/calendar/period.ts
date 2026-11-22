import { DAY, dayStart, type DayTotal } from "../../shared/timeline.ts";

export const views = { year: "年", month: "月", week: "週", day: "日" } as const;
export type View = keyof typeof views;

export function weekStart(time: number) {
  const day = dayStart(time);
  const weekday = new Date(day + DAY / 2).getUTCDay();
  return day - weekday * DAY;
}

function calendarStart(time: number, months: number, year: boolean) {
  const date = new Date(dayStart(time) + DAY / 2);
  date.setUTCMonth((year ? 0 : date.getUTCMonth()) + months, 1);
  return dayStart(date.getTime());
}

export function movePeriod(view: View, anchor: number, direction: number) {
  switch (view) {
    case "year":
      return calendarStart(anchor, direction * 12, true);
    case "month":
      return calendarStart(anchor, direction, false);
    case "week":
      return anchor + direction * 7 * DAY;
    case "day":
      return anchor + direction * DAY;
  }
}

export function period(view: View, anchor: number) {
  const start = view === "week" ? weekStart(anchor) : movePeriod(view, dayStart(anchor), 0);
  return { start, end: movePeriod(view, start, 1) };
}

export function monthTotals(start: number, days: DayTotal[]) {
  const totals = new Map(
    Array.from({ length: 12 }, (_, index) => [movePeriod("month", start, index), 0]),
  );
  for (const { day, ms } of days) {
    const month = movePeriod("month", day, 0);
    totals.set(month, (totals.get(month) ?? 0) + ms);
  }
  return Array.from(totals, ([day, ms]) => ({ day, ms }));
}
