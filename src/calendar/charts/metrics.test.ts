import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { HOUR } from "../../../shared/timeline.ts";
import { monthSeries, rankProjects } from "./metrics.ts";

test("keeps months chronological and keeps local and account usage separate", () => {
  const previous = Date.parse("2025-12-01T00:00:00+09:00");
  const current = Date.parse("2026-01-01T00:00:00+09:00");
  const result = monthSeries([
    { start: current, ms: 2 * HOUR, tokens: 100, accountTokens: 150 },
    { start: previous, ms: HOUR, estimatedCostUsd: 0 },
  ]);
  assert.deepEqual(result, [
    { start: previous, hours: 1, cost: 0, tokens: null, accountTokens: null },
    { start: current, hours: 2, cost: null, tokens: 100, accountTokens: 150 },
  ]);
});

test("ranks projects by the selected metric and limits the list to eight", () => {
  const projects = Array.from({ length: 9 }, (_, index) => ({
    name: `project ${index}`,
    ms: (index + 1) * HOUR,
    estimatedCostUsd: index === 0 ? undefined : 10 - index,
  }));
  assert.deepEqual(
    rankProjects(projects, "hours").map(({ value }) => value),
    [9, 8, 7, 6, 5, 4, 3, 2],
  );
  assert.deepEqual(
    rankProjects(projects, "cost").map(({ name }) => name),
    projects.slice(1).map(({ name }) => name),
  );
});
