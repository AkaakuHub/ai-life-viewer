import { DAY, HOUR } from "../../shared/timeline.ts";

import type { View } from "./period.ts";

const timeZone = "Asia/Tokyo";
export const date = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  month: "numeric",
  day: "numeric",
});
export const dayNumber = new Intl.DateTimeFormat("en", { timeZone, day: "numeric" });
export const monthNumber = new Intl.DateTimeFormat("ja-JP", { timeZone, month: "long" });
const monthName = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  year: "numeric",
  month: "long",
});
const yearName = new Intl.DateTimeFormat("ja-JP", { timeZone, year: "numeric" });

export function rangeLabel(view: View, start: number, end: number) {
  switch (view) {
    case "year":
      return yearName.format(start);
    case "month":
      return monthName.format(start);
    case "week":
      return `${date.format(start)} - ${date.format(end - DAY)}`;
    case "day":
      return `${date.format(start)} (${weekday.format(start)})`;
  }
}

export const weekday = new Intl.DateTimeFormat("ja-JP", { timeZone, weekday: "short" });
export const clock = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});
export const hours = (ms: number) => `${(ms / HOUR).toFixed(1)}h`;

const colors = ["#b793f6", "#65c9f1", "#6bdbc1", "#f3b573", "#8aa8ff", "#f28da6", "#b4d77e"];
export function blockColor(agent: string, project: string) {
  let hash = 0;
  for (const letter of agent + project) hash = (Math.imul(hash, 31) + letter.charCodeAt(0)) >>> 0;
  return colors[hash % colors.length];
}

const dollars = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });
export const usd = (value?: number) => (value === undefined ? "—" : dollars.format(value));
