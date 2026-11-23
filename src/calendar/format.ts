import { DAY, HOUR, MINUTE, dayStart, type Block } from "../../shared/timeline.ts";

import type { View } from "./period.ts";

const timeZone = "Asia/Tokyo";
export const date = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  month: "numeric",
  day: "numeric",
});
export const dayNumber = new Intl.DateTimeFormat("en", { timeZone, day: "numeric" });
export const monthNumber = new Intl.DateTimeFormat("ja-JP", { timeZone, month: "long" });
export const monthName = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  year: "numeric",
  month: "long",
});
export const yearName = new Intl.DateTimeFormat("ja-JP", { timeZone, year: "numeric" });

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

export function duration(ms: number) {
  if (ms > 0 && ms < MINUTE) return "<1m";
  const minutes = Math.floor(ms / MINUTE);
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h ? `${h}h${m ? `${m}m` : ""}` : `${m}m`;
}

export function blockTime(block: Block) {
  const from = `${date.format(block.start)} ${clock.format(block.start)}`;
  const to = block.end === dayStart(block.start) + DAY ? "24:00" : clock.format(block.end);
  return `${from}–${to}（経過 ${duration(block.wallMs)}）`;
}

const colors = [
  "#478659",
  "#a95370",
  "#87913e",
  "#a7703b",
  "#398e86",
  "#8168b5",
  "#66798f",
  "#b3644d",
  "#4b78a7",
  "#9466a0",
];
export function blockColor(agent: string, project: string) {
  let hash = 2166136261;
  for (const character of project || agent) {
    hash = Math.imul(hash ^ character.charCodeAt(0), 16777619) >>> 0;
  }
  return colors[hash % colors.length];
}

const dollars = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" });
export const usd = (value?: number) => (value === undefined ? "—" : dollars.format(value));
