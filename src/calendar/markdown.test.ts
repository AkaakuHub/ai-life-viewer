import assert from "node:assert/strict";
import { test } from "vite-plus/test";
import { supplementaryPreview } from "./markdown.ts";

test("a preview only appears when it supplements the title", () => {
  assert.equal(
    Boolean(supplementaryPreview("# **作業**\n\n`Codex`の更新", " 作業 Codexの更新 ")),
    false,
  );
  assert.equal(
    Boolean(supplementaryPreview("続き\n\n[設定](https://example.com)を確認", "設定を確認")),
    false,
  );
  assert.equal(
    Boolean(
      supplementaryPreview("続き\n\n[設定](https://example.com)を確認", "[設定](https://exam"),
    ),
    false,
  );
  assert.equal(Boolean(supplementaryPreview("設定を確認", "テストが1件失敗")), true);
});
