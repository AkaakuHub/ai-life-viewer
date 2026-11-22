import { HOUR } from "../../shared/timeline.ts";

const timeZone = "Asia/Tokyo";
export const date = new Intl.DateTimeFormat("ja-JP", {
  timeZone,
  month: "numeric",
  day: "numeric",
});
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
