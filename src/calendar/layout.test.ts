import assert from "node:assert/strict";
import { test } from "node:test";
import { DAY, MINUTE, type Block } from "../../shared/timeline.ts";
import { partition, weekStart } from "./layout.ts";

function block(id: number, start: number, end: number): Block {
  return {
    id,
    start: start * MINUTE,
    end: end * MINUTE,
    agent: "cx",
    title: "",
    project: "",
    cwd: "",
    model: "",
    branch: "",
    tokens: null,
  };
}

test("clips at midnight and reuses a lane when the previous block ends", () => {
  const segments = partition(
    [
      block(1, -5, 25),
      block(2, 5, 10),
      block(3, 10, 15),
      block(4, 30, 40),
      block(5, 1435, 1455),
      block(6, 1440, 1450),
    ],
    0,
  );
  assert.deepEqual(
    segments.map(({ block, lane, lanes }) => [block.id, lane, lanes]),
    [
      [1, 0, 2],
      [2, 1, 2],
      [3, 1, 2],
      [4, 0, 1],
      [5, 0, 1],
    ],
  );
  assert.equal(segments[0].start, 0);
  assert.equal(segments.at(-1)?.end, DAY);
});

test("starts the week at Sunday midnight in Tokyo", () => {
  const midnight = Date.parse("2026-11-21T15:00:00Z");
  assert.equal(weekStart(midnight), midnight);
  assert.equal(weekStart(midnight - 1), midnight - 7 * DAY);
});
