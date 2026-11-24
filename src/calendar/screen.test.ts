import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { restoreScreen, screenParams } from "./screen.ts";

const now = Date.parse("2026-11-23T13:00:00+09:00");
const today = Date.parse("2026-11-23T00:00:00+09:00");

test("restores the stored view, date and filters when the URL has no screen state", () => {
  const screen = {
    view: "week" as const,
    anchor: today,
    selection: {
      agents: ["cx", "cl"],
      projects: ["研究,開発", "T3 Code"],
      pcs: ["work-pc"],
      q: "a & b",
    },
  };
  assert.deepEqual(
    restoreScreen(new URLSearchParams("other=1"), screenParams(screen).toString(), now),
    screen,
  );
});

test("the URL controls the entire screen even when its filters are empty", () => {
  const params = new URLSearchParams("view=month&date=2026-08-04");
  const saved = "view=year&date=2025-01-01&agents=cx&q=search";
  assert.deepEqual(restoreScreen(params, saved, now), {
    view: "month",
    anchor: Date.parse("2026-08-04T00:00:00+09:00"),
    selection: { agents: [], projects: [], pcs: [], q: "" },
  });
});

test("ignores invalid dates and view names in shared links", () => {
  const params = new URLSearchParams("view=constructor&date=2026-02-30");
  assert.deepEqual(restoreScreen(params, null, now), {
    view: "top",
    anchor: today,
    selection: { agents: [], projects: [], pcs: [], q: "" },
  });
});
