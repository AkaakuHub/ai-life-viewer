import { HOUR, type DashboardSummary, type GroupTotal } from "../../../shared/timeline.ts";

export type WorkMetric = "hours" | "cost";
export type ChartMetric = WorkMetric | "tokens";

export const workMetrics = [
  { value: "hours", label: "稼働時間" },
  { value: "cost", label: "概算料金" },
] satisfies Array<{ value: WorkMetric; label: string }>;

export const chartMetrics = [
  ...workMetrics,
  { value: "tokens", label: "トークン" },
] satisfies Array<{ value: ChartMetric; label: string }>;

const precise = new Intl.NumberFormat("ja-JP", { maximumFractionDigits: 4 });
const compact = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 1,
});
const dollars = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  maximumFractionDigits: 4,
});

export function chartValue(metric: ChartMetric, value: number, abbreviated = false) {
  const amount = (abbreviated ? compact : precise).format(value);
  switch (metric) {
    case "hours":
      return `${amount}h`;
    case "cost":
      return abbreviated ? `$${amount}` : dollars.format(value);
    case "tokens":
      return amount;
  }
}

export function rankProjects(projects: GroupTotal[], metric: WorkMetric) {
  return projects
    .flatMap(({ name, ms, estimatedCostUsd }) => {
      const value = metric === "hours" ? ms / HOUR : estimatedCostUsd;
      return value !== undefined && value > 0 ? [{ name, value }] : [];
    })
    .sort((a, b) => b.value - a.value || a.name.localeCompare(b.name))
    .slice(0, 8);
}

export function monthSeries(months: DashboardSummary["history"]["months"]) {
  return months
    .map((month) => ({
      start: month.start,
      hours: month.ms / HOUR,
      cost: month.estimatedCostUsd ?? null,
      tokens: month.tokens ?? null,
      accountTokens: month.accountTokens ?? null,
    }))
    .sort((a, b) => a.start - b.start);
}
