#!/usr/bin/env python3
"""Client-side history ingest script for Agent Timeline (POST /api/v1/ingest).

Collects local AI coding agent session histories and upserts them into an
Agent Timeline server. Uses only the Python standard library (plus the `zstd`
CLI when decompressing `.jsonl.zst` Codex rollouts).

Supported providers:
  - cx  : Codex (~/.codex/sessions/**/*.{jsonl,jsonl.zst}, ~/.codex/state_5.sqlite)
  - cc  : Claude Code (~/.claude/projects, ~/.claude/.claude-backup/projects, ~/.claude/history.jsonl)
  - glm : GLM / Z.ai (GLM-model sessions stored in Claude Code projects)
  - gm  : Gemini CLI (~/.gemini/tmp/**/chats/*.json)
  - oc  : OpenCode (~/.local/share/opencode/opencode.db)
  - ag  : Antigravity via T3 Code (~/.t3/userdata/statev2.sqlite)
"""

from __future__ import annotations

import argparse
import collections
import concurrent.futures
import datetime
import glob
import hashlib
import json
import os
import re
import shutil
import sqlite3
import subprocess
import sys
import urllib.error
import urllib.request

MIN_TS = 946684800000  # 2000-01-01T00:00:00Z
MAX_TS = 4102444800000  # 2100-01-01T00:00:00Z
MAX_SPAN_MS = 366 * 86400 * 1000 - 1000
DEFAULT_GAP_MS = 5 * 60 * 1000
HISTORY_SESSION_GAP_MS = 30 * 60 * 1000
MAX_BLOCKS_PER_RECORD = 1000
MAX_RECORDS_PER_BATCH = 200  # server max is 500
MAX_BATCH_BYTES = 700 * 1024  # server max is 1 MiB

ALL_PROVIDERS = ("cx", "cc", "glm", "gm", "oc", "ag")

UUID_RE = re.compile(
    r"([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})", re.I
)
CTRL_RE = re.compile(r"[\x00-\x1f\x7f]")
AGENT_RE = re.compile(r"^[a-z0-9][a-z0-9_-]{0,31}$")
VALID_SERVICE_TIERS = {"default", "standard", "fast", "priority"}


def normalize_service_tier(raw: str | None) -> str:
    if not raw or not isinstance(raw, str):
        return "default"
    tier = raw.strip().lower()
    if tier in ("fast", "priority"):
        return tier
    return "default"


def finalize_usage_by_model(
    buckets: dict[tuple[str, str], list[int]],
    fallback_model: str | None = None,
) -> list[dict]:
    merged: dict[tuple[str, str], list[int]] = {}
    for (model, tier), vals in buckets.items():
        m = clean_meta(model) or clean_meta(fallback_model)
        if not m:
            continue
        t = normalize_service_tier(tier)
        inp, cac, out, rea = vals
        if inp <= 0 and out <= 0 and cac <= 0 and rea <= 0:
            continue
        key = (m, t)
        if key not in merged:
            merged[key] = [0, 0, 0, 0]
        merged[key][0] += max(0, inp)
        merged[key][1] += max(0, cac)
        merged[key][2] += max(0, out)
        merged[key][3] += max(0, rea)

    rows: list[dict] = []
    for (m, t), (inp, cac, out, rea) in sorted(merged.items()):
        inp = max(inp, cac)
        out = max(out, rea)
        row: dict = {
            "model": m,
            "serviceTier": t,
            "inputTokens": inp,
            "cachedInputTokens": cac,
            "outputTokens": out,
            "reasoningTokens": rea,
        }
        rows.append(row)
    return rows[:1000]


def truncate_utf8(s: str, max_bytes: int) -> str:
    b = s.encode("utf-8")
    if len(b) <= max_bytes:
        return s
    return b[:max_bytes].decode("utf-8", errors="ignore")


def clean_session_key(s: str) -> str:
    return truncate_utf8(CTRL_RE.sub("", s).strip(), 160)


def clean_meta(s: str | None) -> str | None:
    if not s:
        return None
    s = s.strip()
    return truncate_utf8(s, 2048) if s else None


def clean_title(s: str | None) -> str | None:
    if not s:
        return None
    s = s.strip()
    return truncate_utf8(s, 8192) if s else None


def normalize_endpoint(url: str) -> str:
    url = url.strip().rstrip("/")
    if url.endswith("/api/v1/ingest"):
        return url
    return f"{url}/api/v1/ingest"


def parse_iso_ms(ts_str: str | None) -> int | None:
    if not ts_str or not isinstance(ts_str, str):
        return None
    try:
        s = ts_str.strip()
        if s.endswith("Z"):
            s = s[:-1] + "+00:00"
        dt = datetime.datetime.fromisoformat(s)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=datetime.timezone.utc)
        ms = int(dt.timestamp() * 1000)
        if MIN_TS <= ms < MAX_TS:
            return ms
    except Exception:
        pass
    return None


def parse_since_ms(since_str: str) -> int:
    s = since_str.strip()
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", s):
        s = f"{s}T00:00:00+09:00"
    ms = parse_iso_ms(s)
    if ms is None:
        raise ValueError(f"Invalid --since value: {since_str!r} (expected YYYY-MM-DD or ISO-8601 timestamp)")
    return ms


def build_blocks(
    timestamps: list[int],
    fallback_start: int | None = None,
    fallback_end: int | None = None,
    gap_ms: int = DEFAULT_GAP_MS,
) -> list[dict[str, int]]:
    ts = sorted(t for t in timestamps if MIN_TS <= t < MAX_TS)
    raw_blocks: list[list[int]] = []
    if ts:
        cur_start = ts[0]
        cur_end = ts[0]
        for t in ts[1:]:
            if t - cur_end <= gap_ms:
                cur_end = t
            else:
                raw_blocks.append([cur_start, cur_end])
                cur_start = t
                cur_end = t
        raw_blocks.append([cur_start, cur_end])
    elif fallback_start is not None and MIN_TS <= fallback_start < MAX_TS:
        end = (
            fallback_end
            if (fallback_end is not None and fallback_end > fallback_start)
            else fallback_start + 1000
        )
        raw_blocks.append([fallback_start, min(end, MAX_TS)])
    else:
        return []

    blocks: list[list[int]] = []
    for s, e in raw_blocks:
        if e <= s:
            e = min(s + 1000, MAX_TS)
        if e <= s:
            continue
        while e - s > MAX_SPAN_MS:
            blocks.append([s, s + MAX_SPAN_MS])
            s = s + MAX_SPAN_MS
        if e > s:
            blocks.append([s, e])

    blocks.sort(key=lambda b: (b[0], b[1]))
    merged: list[list[int]] = []
    for s, e in blocks:
        if merged and s <= merged[-1][1]:
            if e > merged[-1][1]:
                merged[-1][1] = e
        else:
            merged.append([s, e])

    while len(merged) > MAX_BLOCKS_PER_RECORD:
        best_i = 0
        best_gap = merged[1][0] - merged[0][1]
        for i in range(1, len(merged) - 1):
            g = merged[i + 1][0] - merged[i][1]
            if g < best_gap:
                best_gap = g
                best_i = i
        merged[best_i][1] = merged[best_i + 1][1]
        del merged[best_i + 1]

    final: list[dict[str, int]] = []
    for s, e in merged:
        if e - s > MAX_SPAN_MS:
            e = s + MAX_SPAN_MS
        if MIN_TS <= s < e <= MAX_TS:
            final.append({"start": s, "end": e})
    return final


# ---------------------------------------------------------------------------
# Codex (cx)
# ---------------------------------------------------------------------------

def _extract_codex_user_text(payload: dict) -> str | None:
    content = payload.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = [
            c["text"].strip()
            for c in content
            if isinstance(c, dict) and isinstance(c.get("text"), str) and c["text"].strip()
        ]
        if parts:
            return "\n".join(parts)
    msg = payload.get("message")
    if isinstance(msg, str):
        return msg
    return None


def _usage_delta(snapshot: dict[str, int | None], key: str, total_val, last_val) -> int:
    if isinstance(total_val, (int, float)) and total_val >= 0:
        t = int(total_val)
        prev = snapshot[key]
        if prev is not None:
            if t <= prev:
                return 0
            delta = t - prev
        else:
            delta = t
        snapshot[key] = t
        return delta
    if isinstance(last_val, (int, float)) and last_val >= 0:
        lv = int(last_val)
        prev = snapshot[key]
        snapshot[key] = (prev + lv) if prev is not None else lv
        return lv
    return 0


def _parse_codex_rollout(path: str, zstd_bin: str | None) -> dict | None:
    timestamps: list[int] = []
    session_id: str | None = None
    cwd: str | None = None
    branch: str | None = None
    model: str | None = None
    service_tier: str = "default"
    reasoning_effort: str | None = None
    first_user_msg: str | None = None

    snapshot: dict[str, int | None] = {
        "total": None,
        "input": None,
        "cached": None,
        "output": None,
        "reasoning": None,
    }
    totals = {"total": 0, "input": 0, "cached": 0, "output": 0, "reasoning": 0}
    usage_buckets: dict[tuple[str, str], list[int]] = {}
    has_token_breakdown = False

    task_started_count = 0
    turn_context_count = 0
    user_msg_count = 0
    tool_call_count = 0

    def apply_settings(settings: dict) -> None:
        nonlocal model, service_tier
        if not isinstance(settings, dict):
            return
        m = settings.get("model")
        if isinstance(m, str) and m.strip():
            model = m.strip()
        if "service_tier" in settings and settings["service_tier"] is not None:
            service_tier = normalize_service_tier(settings["service_tier"])

    def observe_usage(total_obj: dict | None, last_obj: dict | None) -> None:
        nonlocal has_token_breakdown
        t = total_obj if isinstance(total_obj, dict) else {}
        l = last_obj if isinstance(last_obj, dict) else {}
        d_tot = _usage_delta(snapshot, "total", t.get("total_tokens"), l.get("total_tokens"))
        d_inp = _usage_delta(snapshot, "input", t.get("input_tokens"), l.get("input_tokens"))
        d_cac = _usage_delta(snapshot, "cached", t.get("cached_input_tokens"), l.get("cached_input_tokens"))
        d_out = _usage_delta(snapshot, "output", t.get("output_tokens"), l.get("output_tokens"))
        d_rea = _usage_delta(snapshot, "reasoning", t.get("reasoning_output_tokens"), l.get("reasoning_output_tokens"))

        if d_tot or d_inp or d_cac or d_out or d_rea:
            has_token_breakdown = True
            totals["total"] += d_tot
            totals["input"] += d_inp
            totals["cached"] += d_cac
            totals["output"] += d_out
            totals["reasoning"] += d_rea

        if d_inp > 0 or d_cac > 0 or d_out > 0 or d_rea > 0:
            key = (model or "", service_tier)
            b = usage_buckets.get(key)
            if b is None:
                b = [0, 0, 0, 0]
                usage_buckets[key] = b
            b[0] += d_inp
            b[1] += d_cac
            b[2] += d_out
            b[3] += d_rea

    proc = None
    if path.endswith(".zst"):
        if not zstd_bin:
            return None
        proc = subprocess.Popen(
            [zstd_bin, "-dc", path],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            errors="replace",
        )
        stream = proc.stdout
    else:
        stream = open(path, "r", encoding="utf-8", errors="replace")

    assert stream is not None
    try:
        for line in stream:
            if not line or line[0] != "{":
                continue
            try:
                obj = json.loads(line)
            except Exception:
                continue

            ts = parse_iso_ms(obj.get("timestamp"))
            if ts is not None:
                timestamps.append(ts)

            t = obj.get("type")
            p = obj.get("payload")
            if not isinstance(p, dict):
                continue

            if t == "session_meta":
                sid = p.get("id") or p.get("session_id")
                if isinstance(sid, str) and sid:
                    session_id = sid
                if isinstance(p.get("cwd"), str) and p["cwd"]:
                    cwd = p["cwd"]
                git = p.get("git")
                if isinstance(git, dict) and isinstance(git.get("branch"), str) and git["branch"]:
                    branch = git["branch"]
                ts_meta = parse_iso_ms(p.get("timestamp"))
                if ts_meta is not None:
                    timestamps.append(ts_meta)

            elif t == "turn_context":
                turn_context_count += 1
                apply_settings(p)
                eff = p.get("effort") or p.get("reasoning_effort")
                if isinstance(eff, str) and eff:
                    reasoning_effort = eff
                if not cwd and isinstance(p.get("cwd"), str) and p["cwd"]:
                    cwd = p["cwd"]

            elif t == "token_usage_record":
                observe_usage(p.get("thread_token_usage"), None)

            elif t == "event_msg":
                pt = p.get("type")
                if pt == "thread_settings_applied":
                    apply_settings(p.get("thread_settings"))
                elif pt == "task_started":
                    task_started_count += 1
                elif pt == "user_message":
                    user_msg_count += 1
                    if not first_user_msg:
                        txt = _extract_codex_user_text(p)
                        if txt and not txt.startswith(("<environment_context>", "# AGENTS.md")):
                            first_user_msg = txt
                elif pt == "token_count":
                    apply_settings(p)
                    info = p.get("info")
                    if isinstance(info, dict):
                        observe_usage(info.get("total_token_usage"), info.get("last_token_usage"))

            elif t == "response_item":
                pt = p.get("type")
                if pt in ("function_call", "custom_tool_call", "local_shell_call", "web_search_call"):
                    tool_call_count += 1
                elif pt == "message" and p.get("role") == "user":
                    if not first_user_msg:
                        txt = _extract_codex_user_text(p)
                        if txt and not txt.startswith(
                            ("<environment_context>", "# AGENTS.md", "<INSTRUCTIONS>", "<permissions")
                        ):
                            first_user_msg = txt
    finally:
        stream.close()
        if proc is not None:
            proc.wait()

    if not session_id:
        m = UUID_RE.search(os.path.basename(path))
        if m:
            session_id = m.group(1)

    tot_in = max(totals["input"], totals["cached"])
    tot_out = max(totals["output"], totals["reasoning"])
    tot_all = max(totals["total"], tot_in + tot_out)

    return {
        "session_id": session_id,
        "timestamps": timestamps,
        "cwd": cwd,
        "branch": branch,
        "model": model,
        "service_tier": service_tier,
        "reasoning_effort": reasoning_effort,
        "first_user_msg": first_user_msg,
        "has_token_breakdown": has_token_breakdown,
        "tokens_total": tot_all,
        "tokens_input": tot_in,
        "tokens_cached": min(totals["cached"], tot_in),
        "tokens_output": tot_out,
        "tokens_reasoning": min(totals["reasoning"], tot_out),
        "usage_buckets": usage_buckets,
        "turn_count": task_started_count or turn_context_count or user_msg_count,
        "tool_call_count": tool_call_count,
    }


def _load_codex_sqlite(db_path: str) -> dict[str, dict]:
    res: dict[str, dict] = {}
    if not os.path.exists(db_path):
        return res
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    try:
        cols = {r["name"] for r in conn.execute("PRAGMA table_info(threads)")}
        want = [
            "id", "created_at", "updated_at", "created_at_ms", "updated_at_ms",
            "cwd", "title", "tokens_used", "git_branch", "first_user_message",
            "model", "reasoning_effort", "preview",
        ]
        sel = ", ".join(c if c in cols else f"NULL AS {c}" for c in want)
        for row in conn.execute(f"SELECT {sel} FROM threads"):
            d = dict(row)
            if d["id"]:
                res[d["id"]] = d
    except sqlite3.Error:
        pass
    finally:
        conn.close()
    return res


def collect_codex(
    codex_dir: str,
    gap_ms: int = DEFAULT_GAP_MS,
    workers: int = 8,
    verbose: bool = False,
) -> list[dict]:
    if not os.path.isdir(codex_dir):
        return []

    db_threads = _load_codex_sqlite(os.path.join(codex_dir, "state_5.sqlite"))
    files = sorted(
        glob.glob(os.path.join(codex_dir, "sessions", "**", "*.jsonl"), recursive=True)
        + glob.glob(os.path.join(codex_dir, "sessions", "**", "*.jsonl.zst"), recursive=True)
    )
    zstd_bin = shutil.which("zstd")
    if any(f.endswith(".zst") for f in files) and not zstd_bin:
        print("Warning: `zstd` not found in PATH; skipping .jsonl.zst Codex rollouts.", file=sys.stderr)

    if verbose:
        print(f"[cx] Found {len(files)} rollout files and {len(db_threads)} sqlite threads.", file=sys.stderr)

    by_session: dict[str, dict] = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as ex:
        futs = [ex.submit(_parse_codex_rollout, f, zstd_bin) for f in files]
        for i, fut in enumerate(futs, 1):
            p = fut.result()
            if not p or not p["session_id"]:
                continue
            sid = p["session_id"]
            if sid not in by_session:
                by_session[sid] = p
            else:
                ex_p = by_session[sid]
                ex_p["timestamps"].extend(p["timestamps"])
                if p["tokens_total"] > ex_p["tokens_total"]:
                    for k in (
                        "has_token_breakdown", "tokens_total", "tokens_input",
                        "tokens_cached", "tokens_output", "tokens_reasoning",
                        "usage_buckets", "service_tier",
                    ):
                        ex_p[k] = p[k]
                ex_p["turn_count"] = max(ex_p["turn_count"], p["turn_count"])
                ex_p["tool_call_count"] = max(ex_p["tool_call_count"], p["tool_call_count"])
                for k in ("cwd", "branch", "model", "reasoning_effort", "first_user_msg"):
                    if not ex_p[k] and p[k]:
                        ex_p[k] = p[k]
            if verbose and (i % 400 == 0 or i == len(files)):
                print(f"[cx] Parsed {i}/{len(files)} rollout files...", file=sys.stderr)

    records: list[dict] = []
    for sid in sorted(set(by_session.keys()) | set(db_threads.keys())):
        session_key = clean_session_key(sid)
        if not session_key:
            continue
        p = by_session.get(sid)
        db = db_threads.get(sid)

        fb_start = None
        fb_end = None
        if db:
            fb_start = db.get("created_at_ms") or (db["created_at"] * 1000 if db.get("created_at") else None)
            fb_end = db.get("updated_at_ms") or (db["updated_at"] * 1000 if db.get("updated_at") else None)

        blocks = build_blocks(p["timestamps"] if p else [], fb_start, fb_end, gap_ms=gap_ms)
        if not blocks:
            continue

        cwd = clean_meta((p["cwd"] if p else None) or (db.get("cwd") if db else None))
        project = clean_meta(os.path.basename(cwd.rstrip("/"))) if cwd else None
        title = (
            clean_title(db.get("title") if db else None)
            or clean_title(db.get("preview") if db else None)
            or clean_title(db.get("first_user_message") if db else None)
            or clean_title(p["first_user_msg"] if p else None)
            or clean_title(project)
            or f"Codex session {session_key}"
        )
        model = clean_meta((p["model"] if p else None) or (db.get("model") if db else None))
        branch = clean_meta((p["branch"] if p else None) or (db.get("git_branch") if db else None))
        effort = clean_meta(
            (p["reasoning_effort"] if p else None) or (db.get("reasoning_effort") if db else None)
        )

        rec: dict = {
            "session": session_key,
            "agent": "cx",
            "title": title,
            "blocks": blocks,
        }
        if project:
            rec["project"] = project
        if cwd:
            rec["cwd"] = cwd
        if model:
            rec["model"] = model
        if p and p["service_tier"] != "default":
            rec["serviceTier"] = p["service_tier"]
        if branch:
            rec["branch"] = branch
        if effort:
            rec["reasoningEffort"] = effort

        if p and p["usage_buckets"]:
            ubm = finalize_usage_by_model(p["usage_buckets"], fallback_model=model)
            if ubm:
                rec["usageByModel"] = ubm

        if p and p["has_token_breakdown"] and p["tokens_total"] > 0:
            rec["tokens"] = {
                "total": p["tokens_total"],
                "input": p["tokens_input"],
                "cachedInput": p["tokens_cached"],
                "output": p["tokens_output"],
                "reasoning": p["tokens_reasoning"],
            }
        elif db and (db.get("tokens_used") or 0) > 0:
            rec["tokens"] = {"total": int(db["tokens_used"])}

        if p and p["turn_count"] > 0:
            rec["turnCount"] = p["turn_count"]
        if p and p["tool_call_count"] > 0:
            rec["toolCallCount"] = p["tool_call_count"]

        records.append(rec)

    return records


# ---------------------------------------------------------------------------
# Claude Code (cc) & GLM / Z.ai (glm)
# ---------------------------------------------------------------------------

def _extract_claude_text(content) -> str | None:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = [
            item["text"].strip()
            for item in content
            if isinstance(item, dict) and item.get("type") == "text"
            and isinstance(item.get("text"), str) and item["text"].strip()
        ]
        if parts:
            return "\n".join(parts)
    return None


def collect_claude_and_glm(
    claude_dir: str,
    include_backup: bool = True,
    want_cc: bool = True,
    want_glm: bool = True,
    gap_ms: int = DEFAULT_GAP_MS,
) -> list[dict]:
    if not os.path.isdir(claude_dir) or (not want_cc and not want_glm):
        return []

    patterns = [os.path.join(claude_dir, "projects", "**", "*.jsonl")]
    if include_backup:
        patterns.append(os.path.join(claude_dir, ".claude-backup", "projects", "**", "*.jsonl"))

    files = sorted(f for pat in patterns for f in glob.glob(pat, recursive=True))
    sessions: dict[str, dict] = {}

    for path in files:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            for line in fh:
                if not line or line[0] != "{":
                    continue
                try:
                    obj = json.loads(line)
                except Exception:
                    continue

                sid = obj.get("sessionId")
                if not sid:
                    continue

                st = sessions.get(sid)
                if st is None:
                    st = {
                        "timestamps": [],
                        "cwd": None,
                        "branch": None,
                        "models": collections.Counter(),
                        "title": None,
                        "msg_usage": {},
                        "turns": 0,
                        "tool_calls": 0,
                    }
                    sessions[sid] = st

                ts = parse_iso_ms(obj.get("timestamp"))
                if ts is not None:
                    st["timestamps"].append(ts)

                if not st["cwd"] and isinstance(obj.get("cwd"), str) and obj["cwd"]:
                    st["cwd"] = obj["cwd"]
                if not st["branch"] and isinstance(obj.get("gitBranch"), str) and obj["gitBranch"]:
                    st["branch"] = obj["gitBranch"]

                if obj.get("type") == "summary" and isinstance(obj.get("summary"), str) and obj["summary"].strip():
                    st["title"] = obj["summary"].strip()

                msg = obj.get("message")
                if not isinstance(msg, dict):
                    continue

                role = msg.get("role")
                if role == "user" and not obj.get("isMeta"):
                    content = msg.get("content")
                    is_tool_result = (
                        isinstance(content, list)
                        and len(content) > 0
                        and all(isinstance(c, dict) and c.get("type") == "tool_result" for c in content)
                    )
                    if not is_tool_result:
                        st["turns"] += 1
                        if not st["title"]:
                            txt = _extract_claude_text(content)
                            if txt and not txt.startswith("<command-"):
                                st["title"] = txt

                elif role == "assistant":
                    model = msg.get("model")
                    if isinstance(model, str) and model and model != "<synthetic>":
                        st["models"][model] += 1

                    content = msg.get("content")
                    if isinstance(content, list):
                        for item in content:
                            if isinstance(item, dict) and item.get("type") == "tool_use":
                                st["tool_calls"] += 1

                    usage = msg.get("usage")
                    if isinstance(usage, dict):
                        mid = msg.get("id") or f"anon-{len(st['msg_usage'])}"
                        m_name = model if (isinstance(model, str) and model and model != "<synthetic>") else ""
                        m_tier = normalize_service_tier(usage.get("service_tier"))
                        inp = int(usage.get("input_tokens") or 0)
                        cr = int(usage.get("cache_read_input_tokens") or 0)
                        cc = int(usage.get("cache_creation_input_tokens") or 0)
                        out = int(usage.get("output_tokens") or 0)
                        prev = st["msg_usage"].get(mid)
                        if prev is None or (inp + cr + cc + out) >= sum(prev[2:]):
                            st["msg_usage"][mid] = (m_name, m_tier, inp, cr, cc, out)

    records: list[dict] = []
    covered_intervals: list[tuple[int, int, str | None]] = []

    for sid, st in sorted(sessions.items()):
        session_key = clean_session_key(sid)
        if not session_key:
            continue
        blocks = build_blocks(st["timestamps"], gap_ms=gap_ms)
        if not blocks:
            continue

        top_model = st["models"].most_common(1)[0][0] if st["models"] else None
        is_glm = bool(top_model and top_model.lower().startswith("glm"))
        agent = "glm" if is_glm else "cc"

        cwd = clean_meta(st["cwd"])
        for b in blocks:
            covered_intervals.append((b["start"] - 60000, b["end"] + 60000, cwd))

        if (agent == "glm" and not want_glm) or (agent == "cc" and not want_cc):
            continue

        project = clean_meta(os.path.basename(cwd.rstrip("/"))) if cwd else None
        title = clean_title(st["title"]) or clean_title(project) or f"{agent} session {session_key}"
        branch = clean_meta(st["branch"])
        model = clean_meta(top_model)

        tot_inp = 0
        tot_cr = 0
        tot_out = 0
        cc_buckets: dict[tuple[str, str], list[int]] = {}
        tiers_seen: set[str] = set()
        for m_name, m_tier, inp, cr, cc, out in st["msg_usage"].values():
            full_inp = inp + cr + cc
            tot_inp += full_inp
            tot_cr += cr
            tot_out += out
            tiers_seen.add(m_tier)
            bkey = (m_name or (top_model or ""), m_tier)
            if bkey not in cc_buckets:
                cc_buckets[bkey] = [0, 0, 0, 0]
            cc_buckets[bkey][0] += full_inp
            cc_buckets[bkey][1] += cr
            cc_buckets[bkey][2] += out
        tot_all = tot_inp + tot_out

        rec: dict = {
            "session": session_key,
            "agent": agent,
            "title": title,
            "blocks": blocks,
        }
        if project:
            rec["project"] = project
        if cwd:
            rec["cwd"] = cwd
        if model:
            rec["model"] = model
        if "fast" in tiers_seen:
            rec["serviceTier"] = "fast"
        elif "priority" in tiers_seen:
            rec["serviceTier"] = "priority"
        if branch:
            rec["branch"] = branch
        ubm = finalize_usage_by_model(cc_buckets, fallback_model=model)
        if ubm:
            rec["usageByModel"] = ubm
        if tot_all > 0:
            rec["tokens"] = {
                "total": tot_all,
                "input": tot_inp,
                "cachedInput": min(tot_cr, tot_inp),
                "output": tot_out,
                "reasoning": 0,
            }
        if st["turns"] > 0:
            rec["turnCount"] = st["turns"]
        if st["tool_calls"] > 0:
            rec["toolCallCount"] = st["tool_calls"]

        records.append(rec)

    hist_path = os.path.join(claude_dir, "history.jsonl")
    if want_cc and os.path.exists(hist_path):
        by_sid: dict[str, list[dict]] = collections.defaultdict(list)
        no_sid_by_proj: dict[str, list[dict]] = collections.defaultdict(list)
        with open(hist_path, "r", encoding="utf-8", errors="replace") as fh:
            for line in fh:
                if not line.strip():
                    continue
                try:
                    e = json.loads(line)
                except Exception:
                    continue
                ts = e.get("timestamp")
                if not isinstance(ts, int) or not (MIN_TS <= ts < MAX_TS):
                    continue
                sid = e.get("sessionId")
                if sid:
                    if sid not in sessions:
                        by_sid[sid].append(e)
                else:
                    proj = e.get("project") or ""
                    if any(s <= ts <= en and (c == proj or not proj) for s, en, c in covered_intervals):
                        continue
                    no_sid_by_proj[proj].append(e)

        for sid, entries in sorted(by_sid.items()):
            entries.sort(key=lambda x: x["timestamp"])
            blocks = build_blocks([x["timestamp"] for x in entries], gap_ms=gap_ms)
            if not blocks:
                continue
            cwd = clean_meta(entries[0].get("project"))
            project = clean_meta(os.path.basename(cwd.rstrip("/"))) if cwd else None
            first_disp = next((x.get("display") for x in entries if x.get("display") and x["display"].strip()), None)
            rec = {
                "session": clean_session_key(sid),
                "agent": "cc",
                "title": clean_title(first_disp) or clean_title(project) or f"cc session {sid}",
                "blocks": blocks,
                "turnCount": len(entries),
            }
            if project:
                rec["project"] = project
            if cwd:
                rec["cwd"] = cwd
            records.append(rec)

        for proj, entries in sorted(no_sid_by_proj.items()):
            entries.sort(key=lambda x: x["timestamp"])
            groups: list[list[dict]] = []
            for e in entries:
                if groups and e["timestamp"] - groups[-1][-1]["timestamp"] <= HISTORY_SESSION_GAP_MS:
                    groups[-1].append(e)
                else:
                    groups.append([e])
            for g in groups:
                blocks = build_blocks([x["timestamp"] for x in g], gap_ms=gap_ms)
                if not blocks:
                    continue
                h = hashlib.sha1(f"{proj}:{g[0]['timestamp']}".encode("utf-8")).hexdigest()[:16]
                cwd = clean_meta(proj)
                project = clean_meta(os.path.basename(cwd.rstrip("/"))) if cwd else None
                first_disp = next((x.get("display") for x in g if x.get("display") and x["display"].strip()), None)
                rec = {
                    "session": f"cc-history-{h}",
                    "agent": "cc",
                    "title": clean_title(first_disp) or clean_title(project) or f"cc history {h}",
                    "blocks": blocks,
                    "turnCount": len(g),
                }
                if project:
                    rec["project"] = project
                if cwd:
                    rec["cwd"] = cwd
                records.append(rec)

    return records


# ---------------------------------------------------------------------------
# Gemini CLI (gm)
# ---------------------------------------------------------------------------

def collect_gemini_cli(gemini_dir: str, gap_ms: int = DEFAULT_GAP_MS) -> list[dict]:
    tmp_dir = os.path.join(gemini_dir, "tmp")
    if not os.path.isdir(tmp_dir):
        return []

    files = sorted(glob.glob(os.path.join(tmp_dir, "**", "chats", "*.json"), recursive=True))
    records: list[dict] = []
    seen: set[str] = set()

    for path in files:
        try:
            with open(path, "r", encoding="utf-8", errors="replace") as fh:
                d = json.load(fh)
        except Exception:
            continue
        if not isinstance(d, dict):
            continue
        sid = d.get("sessionId")
        if not sid or sid in seen:
            continue
        seen.add(sid)

        timestamps: list[int] = []
        s_ms = parse_iso_ms(d.get("startTime"))
        e_ms = parse_iso_ms(d.get("lastUpdated"))
        if s_ms is not None:
            timestamps.append(s_ms)
        if e_ms is not None:
            timestamps.append(e_ms)

        title = None
        models = collections.Counter()
        turns = 0
        tool_calls = 0
        tot_in = 0
        tot_cached = 0
        tot_out = 0
        tot_reason = 0
        gm_buckets: dict[tuple[str, str], list[int]] = {}

        for m in d.get("messages", []):
            if not isinstance(m, dict):
                continue
            mts = parse_iso_ms(m.get("timestamp"))
            if mts is not None:
                timestamps.append(mts)
            if m.get("type") == "user":
                turns += 1
                if not title:
                    c = m.get("content")
                    if isinstance(c, str) and c.strip():
                        title = c.strip()
                    elif isinstance(c, list):
                        parts = [x["text"].strip() for x in c if isinstance(x, dict) and isinstance(x.get("text"), str) and x["text"].strip()]
                        if parts:
                            title = "\n".join(parts)
            m_name = m.get("model") if isinstance(m.get("model"), str) and m["model"] else ""
            if m_name:
                models[m_name] += 1
            if isinstance(m.get("toolCalls"), list):
                tool_calls += len(m["toolCalls"])
            toks = m.get("tokens")
            if isinstance(toks, dict):
                inp = int(toks.get("input") or 0)
                cac = int(toks.get("cached") or 0)
                out = int(toks.get("output") or 0)
                tho = int(toks.get("thoughts") or 0)
                tot_in += inp
                tot_cached += min(cac, inp)
                tot_out += out + tho
                tot_reason += tho
                bkey = (m_name, "default")
                if bkey not in gm_buckets:
                    gm_buckets[bkey] = [0, 0, 0, 0]
                gm_buckets[bkey][0] += inp
                gm_buckets[bkey][1] += min(cac, inp)
                gm_buckets[bkey][2] += out + tho
                gm_buckets[bkey][3] += tho

        blocks = build_blocks(timestamps, s_ms, e_ms, gap_ms=gap_ms)
        if not blocks:
            continue

        top_model = clean_meta(models.most_common(1)[0][0]) if models else None
        rec: dict = {
            "session": clean_session_key(sid),
            "agent": "gm",
            "title": clean_title(title) or f"Gemini CLI session {sid[:8]}",
            "blocks": blocks,
        }
        if top_model:
            rec["model"] = top_model
        ubm = finalize_usage_by_model(gm_buckets, fallback_model=top_model)
        if ubm:
            rec["usageByModel"] = ubm
        if tot_in + tot_out > 0:
            rec["tokens"] = {
                "total": tot_in + tot_out,
                "input": tot_in,
                "cachedInput": tot_cached,
                "output": tot_out,
                "reasoning": tot_reason,
            }
        if turns > 0:
            rec["turnCount"] = turns
        if tool_calls > 0:
            rec["toolCallCount"] = tool_calls

        records.append(rec)

    return records


# ---------------------------------------------------------------------------
# OpenCode (oc)
# ---------------------------------------------------------------------------

def collect_opencode(opencode_db: str, gap_ms: int = DEFAULT_GAP_MS) -> list[dict]:
    if not os.path.exists(opencode_db):
        return []
    conn = sqlite3.connect(f"file:{opencode_db}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    records: list[dict] = []

    try:
        for row in conn.execute(
            """
            SELECT id, title, directory, model, tokens_input, tokens_output,
                   tokens_reasoning, tokens_cache_read, tokens_cache_write,
                   time_created, time_updated
            FROM session_v2
            """
        ):
            sid = row["id"]
            timestamps: list[int] = []
            turns = 0
            tool_calls = 0
            for mrow in conn.execute(
                "SELECT type, time_created, time_updated, data FROM session_message WHERE session_id = ?",
                (sid,),
            ):
                if mrow["time_created"]:
                    timestamps.append(int(mrow["time_created"]))
                if mrow["time_updated"]:
                    timestamps.append(int(mrow["time_updated"]))
                if mrow["type"] == "user":
                    turns += 1
                elif mrow["data"]:
                    try:
                        md = json.loads(mrow["data"])
                        for c in md.get("content", []) if isinstance(md, dict) else []:
                            if isinstance(c, dict) and c.get("type") in ("tool-call", "tool_use", "tool"):
                                tool_calls += 1
                    except Exception:
                        pass

            blocks = build_blocks(timestamps, row["time_created"], row["time_updated"], gap_ms=gap_ms)
            if not blocks:
                continue

            cwd = clean_meta(row["directory"])
            project = clean_meta(os.path.basename(cwd.rstrip("/"))) if cwd else None
            model_id = None
            effort = None
            if row["model"]:
                try:
                    mj = json.loads(row["model"])
                    if isinstance(mj, dict):
                        model_id = clean_meta(mj.get("id") or mj.get("modelID"))
                        effort = clean_meta(mj.get("variant"))
                except Exception:
                    model_id = clean_meta(str(row["model"]))

            inp = int(row["tokens_input"] or 0) + int(row["tokens_cache_read"] or 0) + int(row["tokens_cache_write"] or 0)
            cached = int(row["tokens_cache_read"] or 0)
            reason = int(row["tokens_reasoning"] or 0)
            out = int(row["tokens_output"] or 0) + reason
            total = inp + out

            rec: dict = {
                "session": clean_session_key(sid),
                "agent": "oc",
                "title": clean_title(row["title"]) or clean_title(project) or f"OpenCode {sid}",
                "blocks": blocks,
            }
            if project:
                rec["project"] = project
            if cwd:
                rec["cwd"] = cwd
            if model_id:
                rec["model"] = model_id
            if effort:
                rec["reasoningEffort"] = effort
            if total > 0:
                rec["tokens"] = {
                    "total": total,
                    "input": inp,
                    "cachedInput": min(cached, inp),
                    "output": out,
                    "reasoning": min(reason, out),
                }
                if model_id:
                    rec["usageByModel"] = [
                        {
                            "model": model_id,
                            "serviceTier": "default",
                            "inputTokens": inp,
                            "cachedInputTokens": min(cached, inp),
                            "outputTokens": out,
                            "reasoningTokens": min(reason, out),
                        }
                    ]
            if turns > 0:
                rec["turnCount"] = turns
            if tool_calls > 0:
                rec["toolCallCount"] = tool_calls

            records.append(rec)
    except sqlite3.Error:
        pass
    finally:
        conn.close()

    return records


# ---------------------------------------------------------------------------
# Antigravity via T3 Code (ag)
# ---------------------------------------------------------------------------

def collect_t3_antigravity(t3_db: str, gap_ms: int = DEFAULT_GAP_MS) -> list[dict]:
    if not os.path.exists(t3_db):
        return []
    conn = sqlite3.connect(f"file:{t3_db}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    records: list[dict] = []

    try:
        proj_map = {
            r["project_id"]: (r["title"], r["workspace_root"])
            for r in conn.execute("SELECT project_id, title, workspace_root FROM projection_projects")
        }
        for row in conn.execute(
            """
            SELECT thread_id, project_id, title, default_provider, created_at, updated_at, payload_json
            FROM orchestration_v2_projection_threads
            WHERE default_provider = 'antigravity'
            """
        ):
            tid = row["thread_id"]
            timestamps: list[int] = []
            s_ms = parse_iso_ms(row["created_at"])
            e_ms = parse_iso_ms(row["updated_at"])
            if s_ms is not None:
                timestamps.append(s_ms)
            if e_ms is not None:
                timestamps.append(e_ms)

            turns = 0
            for mr in conn.execute(
                "SELECT role, created_at, updated_at FROM orchestration_v2_projection_messages WHERE thread_id = ?",
                (tid,),
            ):
                t1 = parse_iso_ms(mr["created_at"])
                t2 = parse_iso_ms(mr["updated_at"])
                if t1 is not None:
                    timestamps.append(t1)
                if t2 is not None:
                    timestamps.append(t2)
                if mr["role"] == "user":
                    turns += 1

            tool_calls = conn.execute(
                "SELECT count(*) FROM orchestration_v2_projection_turn_items WHERE thread_id = ? AND type LIKE '%tool%'",
                (tid,),
            ).fetchone()[0]

            blocks = build_blocks(timestamps, s_ms, e_ms, gap_ms=gap_ms)
            if not blocks:
                continue

            payload = json.loads(row["payload_json"]) if row["payload_json"] else {}
            msel = payload.get("modelSelection") if isinstance(payload, dict) else None
            model = clean_meta(msel.get("model")) if isinstance(msel, dict) else None
            effort = None
            if isinstance(msel, dict) and isinstance(msel.get("options"), list):
                for opt in msel["options"]:
                    if isinstance(opt, dict) and opt.get("id") in ("thought_level", "effort", "reasoning_effort"):
                        effort = clean_meta(str(opt.get("value")))

            cwd = clean_meta(payload.get("worktreePath"))
            proj_info = proj_map.get(row["project_id"])
            if not cwd and proj_info:
                cwd = clean_meta(proj_info[1])
            project = (
                clean_meta(os.path.basename(cwd.rstrip("/")))
                if cwd
                else (clean_meta(proj_info[0]) if proj_info else None)
            )

            rec: dict = {
                "session": clean_session_key(tid),
                "agent": "ag",
                "title": clean_title(row["title"]) or f"ag session {tid}",
                "blocks": blocks,
            }
            if project:
                rec["project"] = project
            if cwd:
                rec["cwd"] = cwd
            if model:
                rec["model"] = model
            if effort:
                rec["reasoningEffort"] = effort
            if turns > 0:
                rec["turnCount"] = turns
            if tool_calls > 0:
                rec["toolCallCount"] = tool_calls

            records.append(rec)
    except sqlite3.Error:
        pass
    finally:
        conn.close()

    return records


# ---------------------------------------------------------------------------
# Deduplication, batching, and HTTP submission
# ---------------------------------------------------------------------------

def deduplicate_records(records: list[dict], gap_ms: int = DEFAULT_GAP_MS) -> list[dict]:
    dedup: dict[tuple[str, str], dict] = {}
    for r in records:
        if not AGENT_RE.match(r["agent"]) or not r["session"] or not r["blocks"]:
            continue
        key = (r["agent"], r["session"])
        if key not in dedup:
            dedup[key] = r
        else:
            ex = dedup[key]
            pts: list[int] = []
            for b in ex["blocks"] + r["blocks"]:
                pts.extend([b["start"], b["end"]])
            ex["blocks"] = build_blocks(pts, gap_ms=gap_ms)
            if r.get("tokens", {}).get("total", 0) > ex.get("tokens", {}).get("total", 0):
                ex["tokens"] = r["tokens"]
                if r.get("usageByModel"):
                    ex["usageByModel"] = r["usageByModel"]
                if r.get("serviceTier"):
                    ex["serviceTier"] = r["serviceTier"]
            for field in ("turnCount", "toolCallCount"):
                if r.get(field, 0) > ex.get(field, 0):
                    ex[field] = r[field]
            for field in ("project", "cwd", "model", "branch", "reasoningEffort", "serviceTier"):
                if not ex.get(field) and r.get(field):
                    ex[field] = r[field]
    return list(dedup.values())


def make_batches(
    records: list[dict],
    max_records: int = MAX_RECORDS_PER_BATCH,
    max_bytes: int = MAX_BATCH_BYTES,
) -> list[list[dict]]:
    batches: list[list[dict]] = []
    cur: list[dict] = []
    cur_bytes = 20
    for rec in records:
        rec_bytes = len(json.dumps(rec, ensure_ascii=False, separators=(",", ":")).encode("utf-8")) + 1
        if cur and (len(cur) >= max_records or cur_bytes + rec_bytes > max_bytes):
            batches.append(cur)
            cur = []
            cur_bytes = 20
        cur.append(rec)
        cur_bytes += rec_bytes
    if cur:
        batches.append(cur)
    return batches


def post_batch(endpoint: str, batch: list[dict], token: str, pc_id: str, timeout: int = 60) -> dict:
    body = json.dumps({"pcId": pc_id, "records": batch}, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    req = urllib.request.Request(
        endpoint,
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {e.code} from {endpoint}: {err_body}") from e


def summarize_records(records: list[dict]) -> dict:
    by_agent: dict[str, dict] = collections.defaultdict(
        lambda: {"records": 0, "blocks": 0, "tokens": 0, "min": None, "max": None}
    )
    for r in records:
        st = by_agent[r["agent"]]
        st["records"] += 1
        st["blocks"] += len(r["blocks"])
        st["tokens"] += r.get("tokens", {}).get("total", 0)
        s0 = r["blocks"][0]["start"]
        e0 = r["blocks"][-1]["end"]
        st["min"] = s0 if st["min"] is None else min(st["min"], s0)
        st["max"] = e0 if st["max"] is None else max(st["max"], e0)

    out = {}
    for agent, st in sorted(by_agent.items()):
        out[agent] = {
            "records": st["records"],
            "blocks": st["blocks"],
            "tokens": st["tokens"],
            "earliest": datetime.datetime.fromtimestamp(st["min"] / 1000, datetime.timezone.utc).isoformat(),
            "latest": datetime.datetime.fromtimestamp(st["max"] / 1000, datetime.timezone.utc).isoformat(),
        }
    return out


def parse_providers(spec: str) -> set[str]:
    if spec.strip().lower() == "all":
        return set(ALL_PROVIDERS)
    chosen = {p.strip().lower() for p in spec.split(",") if p.strip()}
    unknown = chosen - set(ALL_PROVIDERS)
    if unknown:
        raise ValueError(f"Unknown providers: {', '.join(sorted(unknown))}. Valid: {', '.join(ALL_PROVIDERS)}")
    return chosen


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(
        description="Collect local AI agent histories and ingest them into Agent Timeline."
    )
    ap.add_argument(
        "--url",
        default=os.environ.get("AGENT_TIMELINE_URL", ""),
        help="Agent Timeline base URL or /api/v1/ingest endpoint (or env AGENT_TIMELINE_URL)",
    )
    ap.add_argument(
        "--token",
        default=os.environ.get("AGENT_TIMELINE_INGEST_TOKEN", ""),
        help="Bearer token (or env AGENT_TIMELINE_INGEST_TOKEN)",
    )
    ap.add_argument("--pc-id", default=os.environ.get("AGENT_TIMELINE_PC_ID"), help="Unique PC identifier (or env AGENT_TIMELINE_PC_ID)")
    ap.add_argument("--token-file", help="Path to a file containing the Bearer token")
    ap.add_argument(
        "--providers",
        default="all",
        help=f"Comma-separated list of providers or 'all' (valid: {', '.join(ALL_PROVIDERS)})",
    )
    ap.add_argument(
        "--since",
        help="Only include sessions with activity on/after YYYY-MM-DD (JST) or ISO-8601 timestamp",
    )
    ap.add_argument(
        "--gap-seconds",
        type=int,
        default=300,
        help="Idle gap threshold in seconds for splitting session blocks (default: 300)",
    )
    ap.add_argument(
        "--workers",
        type=int,
        default=8,
        help="Parallel worker count for Codex rollout parsing (default: 8)",
    )
    ap.add_argument(
        "--no-claude-backup",
        action="store_true",
        help="Skip ~/.claude/.claude-backup/projects when scanning Claude Code history",
    )
    ap.add_argument("--dry-run", action="store_true", help="Parse and print summary without sending")
    ap.add_argument("-v", "--verbose", action="store_true", help="Print progress to stderr")
    args = ap.parse_args(argv)
    if not args.dry_run and not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}", args.pc_id or ""):
        ap.error("--pc-id must start with a letter or number and contain 1–64 letters, numbers, dots, underscores or hyphens")

    providers = parse_providers(args.providers)
    gap_ms = max(1, args.gap_seconds) * 1000
    since_ms = parse_since_ms(args.since) if args.since else None

    home = os.path.expanduser("~")
    raw_records: list[dict] = []

    if "cx" in providers:
        raw_records.extend(
            collect_codex(os.path.join(home, ".codex"), gap_ms=gap_ms, workers=args.workers, verbose=args.verbose)
        )
    if "cc" in providers or "glm" in providers:
        raw_records.extend(
            collect_claude_and_glm(
                os.path.join(home, ".claude"),
                include_backup=not args.no_claude_backup,
                want_cc="cc" in providers,
                want_glm="glm" in providers,
                gap_ms=gap_ms,
            )
        )
    if "gm" in providers:
        raw_records.extend(collect_gemini_cli(os.path.join(home, ".gemini"), gap_ms=gap_ms))
    if "oc" in providers:
        xdg_data = os.environ.get("XDG_DATA_HOME") or os.path.join(home, ".local", "share")
        raw_records.extend(collect_opencode(os.path.join(xdg_data, "opencode", "opencode.db"), gap_ms=gap_ms))
    if "ag" in providers:
        raw_records.extend(collect_t3_antigravity(os.path.join(home, ".t3", "userdata", "statev2.sqlite"), gap_ms=gap_ms))

    records = deduplicate_records(raw_records, gap_ms=gap_ms)
    if since_ms is not None:
        records = [r for r in records if r["blocks"][-1]["end"] >= since_ms]

    batches = make_batches(records)
    summary = {
        "totalRecords": len(records),
        "totalBlocks": sum(len(r["blocks"]) for r in records),
        "batches": len(batches),
        "byAgent": summarize_records(records),
    }

    if args.dry_run:
        print(json.dumps(summary, indent=2, ensure_ascii=False))
        return 0

    if not args.url:
        print("Error: --url or AGENT_TIMELINE_URL is required unless --dry-run is set.", file=sys.stderr)
        return 2

    token = args.token.strip()
    if args.token_file:
        with open(args.token_file, "r", encoding="utf-8") as fh:
            token = fh.read().strip()
    if not token:
        print("Error: --token, --token-file, or AGENT_TIMELINE_INGEST_TOKEN is required.", file=sys.stderr)
        return 2

    endpoint = normalize_endpoint(args.url)
    ingested = 0
    for idx, batch in enumerate(batches, 1):
        res = post_batch(endpoint, batch, token, args.pc_id)
        ingested += int(res.get("records", 0))
        if args.verbose:
            print(f"Batch {idx}/{len(batches)}: sent {len(batch)} -> {res}", file=sys.stderr)

    summary["ingestedRecords"] = ingested
    summary["endpoint"] = endpoint
    print(json.dumps(summary, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
