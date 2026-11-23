import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { swipeDirection } from "./gestures.ts";

test("period swipes require horizontal travel and reject scrolling gestures", () => {
  assert.equal(swipeDirection(70, 15, 55), -1);
  assert.equal(swipeDirection(-70, -15, 55), 1);
  assert.equal(swipeDirection(30, 5, 55), 0);
  assert.equal(swipeDirection(65, 65, 55), 0);
  assert.equal(swipeDirection(-65, 85, 55), 0);
});
