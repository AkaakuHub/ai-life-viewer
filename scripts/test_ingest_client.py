#!/usr/bin/env python3
"""Unit tests for scripts/ingest_client.py."""

import json
import os
import sqlite3
import tempfile
import unittest

import ingest_client as ic


class TestIngestClient(unittest.TestCase):
    def test_normalize_endpoint(self):
        self.assertEqual(
            ic.normalize_endpoint("https://example.ts.net:14443/"),
            "https://example.ts.net:14443/api/v1/ingest",
        )
        self.assertEqual(
            ic.normalize_endpoint("https://example.ts.net:14443/api/v1/ingest"),
            "https://example.ts.net:14443/api/v1/ingest",
        )

    def test_build_blocks_merges_and_splits_on_gap(self):
        base = 1795000000000
        blocks = ic.build_blocks([base, base + 60_000, base + 400_000, base + 400_000])
        self.assertEqual(
            blocks,
            [
                {"start": base, "end": base + 60_000},
                {"start": base + 400_000, "end": base + 401_000},
            ],
        )

    def test_build_blocks_compacts_over_1000(self):
        base = 1795000000000
        # 1050 isolated timestamps separated by 10 minutes
        ts = [base + i * 600_000 for i in range(1050)]
        blocks = ic.build_blocks(ts)
        self.assertEqual(len(blocks), ic.MAX_BLOCKS_PER_RECORD)
        starts = [b["start"] for b in blocks]
        self.assertEqual(len(starts), len(set(starts)))

    def test_collect_claude_and_glm_deduplicates_streaming_usage(self):
        with tempfile.TemporaryDirectory() as td:
            proj = os.path.join(td, "projects", "-tmp-demo")
            os.makedirs(proj)
            lines = [
                {
                    "sessionId": "sess-cc-1",
                    "timestamp": "2026-01-10T01:00:00Z",
                    "cwd": "/tmp/demo",
                    "gitBranch": "main",
                    "type": "user",
                    "message": {"role": "user", "content": "Fix bug"},
                },
                {
                    "sessionId": "sess-cc-1",
                    "timestamp": "2026-01-10T01:00:10Z",
                    "type": "assistant",
                    "message": {
                        "id": "msg_1",
                        "role": "assistant",
                        "model": "claude-sonnet-4-20250514",
                        "content": [{"type": "tool_use", "name": "Read"}],
                        "usage": {"input_tokens": 10, "cache_read_input_tokens": 100, "cache_creation_input_tokens": 50, "output_tokens": 20},
                    },
                },
                # Final streaming update for same msg_1 with higher output_tokens
                {
                    "sessionId": "sess-cc-1",
                    "timestamp": "2026-01-10T01:00:20Z",
                    "type": "assistant",
                    "message": {
                        "id": "msg_1",
                        "role": "assistant",
                        "model": "claude-sonnet-4-20250514",
                        "content": [{"type": "text", "text": "Done"}],
                        "usage": {"input_tokens": 10, "cache_read_input_tokens": 100, "cache_creation_input_tokens": 50, "output_tokens": 40},
                    },
                },
                {
                    "sessionId": "sess-glm-1",
                    "timestamp": "2026-01-11T01:00:00Z",
                    "cwd": "/tmp/demo",
                    "type": "user",
                    "message": {"role": "user", "content": "GLM task"},
                },
                {
                    "sessionId": "sess-glm-1",
                    "timestamp": "2026-01-11T01:00:15Z",
                    "type": "assistant",
                    "message": {
                        "id": "msg_g1",
                        "role": "assistant",
                        "model": "glm-4.7",
                        "content": [{"type": "text", "text": "OK"}],
                        "usage": {"input_tokens": 200, "cache_read_input_tokens": 50, "output_tokens": 30},
                    },
                },
            ]
            with open(os.path.join(proj, "all.jsonl"), "w", encoding="utf-8") as fh:
                for row in lines:
                    fh.write(json.dumps(row) + "\n")

            recs = ic.collect_claude_and_glm(td, include_backup=False)
            by_id = {r["session"]: r for r in recs}
            self.assertEqual(by_id["sess-cc-1"]["agent"], "cc")
            self.assertEqual(by_id["sess-cc-1"]["tokens"], {
                "total": 200, "input": 160, "cachedInput": 100, "output": 40, "reasoning": 0
            })
            self.assertEqual(by_id["sess-cc-1"]["toolCallCount"], 1)
            self.assertEqual(by_id["sess-glm-1"]["agent"], "glm")
            self.assertEqual(by_id["sess-glm-1"]["tokens"]["total"], 280)
            self.assertEqual(by_id["sess-cc-1"]["usageByModel"], [{
                "model": "claude-sonnet-4-20250514",
                "serviceTier": "default",
                "inputTokens": 160,
                "cachedInputTokens": 100,
                "outputTokens": 40,
                "reasoningTokens": 0,
            }])

    def test_collect_codex_model_switch_and_fast_tier(self):
        with tempfile.TemporaryDirectory() as td:
            sess_dir = os.path.join(td, "sessions", "2026", "11", "20")
            os.makedirs(sess_dir)
            rollout = os.path.join(sess_dir, "rollout-2026-11-20T00-00-00-01a1f796-b5a7-7243-b342-fd802057c753.jsonl")
            lines = [
                {
                    "timestamp": "2026-11-20T00:00:00Z",
                    "type": "session_meta",
                    "payload": {"id": "01a1f796-b5a7-7243-b342-fd802057c753", "cwd": "/tmp/demo"},
                },
                {
                    "timestamp": "2026-11-20T00:00:01Z",
                    "type": "event_msg",
                    "payload": {
                        "type": "thread_settings_applied",
                        "thread_settings": {"model": "gpt-6.1-sol", "service_tier": "default"},
                    },
                },
                {
                    "timestamp": "2026-11-20T00:00:05Z",
                    "type": "event_msg",
                    "payload": {
                        "type": "token_count",
                        "info": {
                            "total_token_usage": {
                                "total_tokens": 1100,
                                "input_tokens": 1000,
                                "cached_input_tokens": 600,
                                "output_tokens": 100,
                                "reasoning_output_tokens": 40,
                            }
                        },
                    },
                },
                {
                    "timestamp": "2026-11-20T00:00:10Z",
                    "type": "event_msg",
                    "payload": {
                        "type": "thread_settings_applied",
                        "thread_settings": {"model": "gpt-6.2-sol", "service_tier": "fast"},
                    },
                },
                {
                    "timestamp": "2026-11-20T00:00:20Z",
                    "type": "event_msg",
                    "payload": {
                        "type": "token_count",
                        "info": {
                            "total_token_usage": {
                                "total_tokens": 3400,
                                "input_tokens": 3000,
                                "cached_input_tokens": 2100,
                                "output_tokens": 400,
                                "reasoning_output_tokens": 140,
                            }
                        },
                    },
                },
            ]
            with open(rollout, "w", encoding="utf-8") as fh:
                for row in lines:
                    fh.write(json.dumps(row) + "\n")

            recs = ic.collect_codex(td)
            self.assertEqual(len(recs), 1)
            r = recs[0]
            self.assertEqual(r["serviceTier"], "fast")
            self.assertEqual(r["usageByModel"], [
                {
                    "model": "gpt-6.1-sol",
                    "serviceTier": "default",
                    "inputTokens": 1000,
                    "cachedInputTokens": 600,
                    "outputTokens": 100,
                    "reasoningTokens": 40,
                },
                {
                    "model": "gpt-6.2-sol",
                    "serviceTier": "fast",
                    "inputTokens": 2000,
                    "cachedInputTokens": 1500,
                    "outputTokens": 300,
                    "reasoningTokens": 100,
                },
            ])

    def test_collect_gemini_cli_and_opencode(self):
        with tempfile.TemporaryDirectory() as td:
            chat_dir = os.path.join(td, "tmp", "hash1", "chats")
            os.makedirs(chat_dir)
            with open(os.path.join(chat_dir, "s1.json"), "w", encoding="utf-8") as fh:
                json.dump(
                    {
                        "sessionId": "gm-1",
                        "startTime": "2025-09-17T15:21:23.737Z",
                        "lastUpdated": "2025-09-17T15:22:09.679Z",
                        "messages": [
                            {"type": "user", "timestamp": "2025-09-17T15:21:23.737Z", "content": "Hello"},
                            {
                                "type": "gemini",
                                "timestamp": "2025-09-17T15:22:09.679Z",
                                "model": "gemini-2.5-pro",
                                "tokens": {"input": 1000, "cached": 400, "output": 100, "thoughts": 50, "total": 1150},
                            },
                        ],
                    },
                    fh,
                )
            gm_recs = ic.collect_gemini_cli(td)
            self.assertEqual(len(gm_recs), 1)
            self.assertEqual(
                gm_recs[0]["tokens"],
                {"total": 1150, "input": 1000, "cachedInput": 400, "output": 150, "reasoning": 50},
            )

            # OpenCode sqlite fixture
            oc_db = os.path.join(td, "opencode.db")
            conn = sqlite3.connect(oc_db)
            conn.executescript(
                """
                CREATE TABLE session_v2 (
                    id TEXT, title TEXT, directory TEXT, model TEXT,
                    tokens_input INT, tokens_output INT, tokens_reasoning INT,
                    tokens_cache_read INT, tokens_cache_write INT,
                    time_created INT, time_updated INT
                );
                CREATE TABLE session_message (
                    session_id TEXT, type TEXT, time_created INT, time_updated INT, data TEXT
                );
                """
            )
            conn.execute(
                "INSERT INTO session_v2 VALUES (?,?,?,?,?,?,?,?,?,?,?)",
                (
                    "ses_1", "Test OC", "/tmp/proj",
                    json.dumps({"id": "glm-5.3-flash-free", "variant": "medium"}),
                    100, 30, 40, 500, 50, 1793861367372, 1793861400000,
                ),
            )
            conn.execute(
                "INSERT INTO session_message VALUES (?,?,?,?,?)",
                ("ses_1", "user", 1793861367372, 1793861367372, "{}"),
            )
            conn.commit()
            conn.close()

            oc_recs = ic.collect_opencode(oc_db)
            self.assertEqual(len(oc_recs), 1)
            self.assertEqual(oc_recs[0]["agent"], "oc")
            self.assertEqual(
                oc_recs[0]["tokens"],
                {"total": 720, "input": 650, "cachedInput": 500, "output": 70, "reasoning": 40},
            )

    def test_make_batches_respects_size_and_count(self):
        recs = [
            {"agent": "cx", "session": f"s{i}", "title": "x" * 100, "blocks": [{"start": 1795000000000, "end": 1795000001000}]}
            for i in range(10)
        ]
        batches = ic.make_batches(recs, max_records=3, max_bytes=10000)
        self.assertEqual([len(b) for b in batches], [3, 3, 3, 1])


if __name__ == "__main__":
    unittest.main()
