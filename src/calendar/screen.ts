import { dayKey, dayStart, type FilterOptions } from "../../shared/timeline.ts";
import { views, type View } from "./period.ts";

export type AppView = View | "top";
export type Selection = FilterOptions & { q: string };
export type Screen = { view: AppView; anchor: number; selection: Selection };

const keys = ["view", "date", "agents", "projects", "pcs", "q"] as const;

export function restoreScreen(url: URLSearchParams, saved: string | null, now: number): Screen {
  const params = keys.some((key) => url.has(key)) ? url : new URLSearchParams(saved ?? "");
  const view = params.get("view") ?? "";
  const day = params.get("date") ?? "";
  const timestamp = /^\d{4}-\d{2}-\d{2}$/.test(day) ? Date.parse(`${day}T00:00:00+09:00`) : NaN;
  return {
    view: isAppView(view) ? view : "top",
    anchor: Number.isFinite(timestamp) && dayKey(timestamp) === day ? timestamp : dayStart(now),
    selection: {
      agents: params.getAll("agents").filter(Boolean),
      projects: params.getAll("projects").filter(Boolean),
      pcs: params.getAll("pcs").filter(Boolean),
      q: params.get("q") ?? "",
    },
  };
}

function isAppView(value: string): value is AppView {
  return value === "top" || Object.hasOwn(views, value);
}

export function filterParams(selection: Selection) {
  const params = new URLSearchParams();
  for (const agent of selection.agents) params.append("agents", agent);
  for (const project of selection.projects) params.append("projects", project);
  for (const pc of selection.pcs) params.append("pcs", pc);
  if (selection.q) params.set("q", selection.q);
  return params;
}

export function screenParams(screen: Screen, params = new URLSearchParams()) {
  for (const key of keys) params.delete(key);
  params.set("view", screen.view);
  params.set("date", dayKey(screen.anchor));
  for (const [key, value] of filterParams(screen.selection)) params.append(key, value);
  return params;
}
