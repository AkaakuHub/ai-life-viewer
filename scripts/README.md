# クライアント側 履歴送信スクリプト (`ingest_client.py`)

別マシン（Mac 等）に残っている各 AI エージェントの過去履歴を収集し、Agent Timeline の `POST /api/v1/ingest` へ冪等に登録するスクリプトです。

- **依存関係**: Python 3.10+ の標準ライブラリのみ（Codex の `.jsonl.zst` を展開する場合のみ `zstd` コマンドを使用）
- **冪等性**: `(agent, session)` 単位の upsert として送信するため、何度再実行しても安全です
- **途中モデル切替・Fast倍率課金対応**: `serviceTier` と `usageByModel`（モデル・ティア別のトークン内訳）を抽出して送信します
- **`dailyUsage` 非送信**: サーバー側にあるアカウント全体の日次集計を上書きしないよう、`records` のみ送信します

## 対応プロバイダー

| 指定キー | エージェント          | 参照パス                                                                                                   |
| :------- | :-------------------- | :--------------------------------------------------------------------------------------------------------- |
| `cx`     | Codex                 | `~/.codex/sessions/**/*.{jsonl,jsonl.zst}`, `~/.codex/state_5.sqlite`                                      |
| `cc`     | Claude Code           | `~/.claude/projects/**/*.jsonl`, `~/.claude/.claude-backup/projects/**/*.jsonl`, `~/.claude/history.jsonl` |
| `glm`    | GLM (Z.ai)            | `~/.claude/projects/**/*.jsonl` 内の `glm-*` モデルセッション                                              |
| `gm`     | Gemini CLI            | `~/.gemini/tmp/**/chats/*.json`                                                                            |
| `oc`     | OpenCode              | `${XDG_DATA_HOME:-~/.local/share}/opencode/opencode.db`                                                    |
| `ag`     | Antigravity (T3 Code) | `~/.t3/userdata/statev2.sqlite`                                                                            |

## 使い方

### 送信用トークンの再発行

サービスの起動に使っているサーバーバイナリ `agent-timeline` に、再発行用のサブコマンド `rotate-token` が組み込まれています。稼働ホストでサービスと同じユーザー（現在は `ubuntu`）として実行します。

```bash
/opt/ai-life-viewer/agent-timeline rotate-token && sudo systemctl restart ai-life-viewer
```

新しいトークンは `ingest.token` に権限 `0600` で保存され、端末にも表示されます。サービスの再起動で旧トークンが失効するため、送信元の端末すべてで新しいトークンに更新してください。

### 1. ドライラン（送信せずに件数・期間を確認）

```bash
python3 scripts/ingest_client.py --dry-run
```

### 2. 全プロバイダーを一括送信（棚卸し・過去分バックフィル）

```bash
export AGENT_TIMELINE_URL="https://vcn-tokyo-main.tail4c0963.ts.net:14443"
export AGENT_TIMELINE_INGEST_TOKEN="<サーバーの ingest.token>"

python3 scripts/ingest_client.py -v
```

トークンをファイルから渡す場合:

```bash
python3 scripts/ingest_client.py \
  --url "https://vcn-tokyo-main.tail4c0963.ts.net:14443" \
  --token-file /path/to/ingest.token \
  -v
```

### 3. 特定プロバイダーのみ / 差分のみ送信

```bash
# Codex と Claude Code / GLM のみ送信
python3 scripts/ingest_client.py --providers cx,cc,glm -v

# 指定日（JST）以降に更新されたセッションのみ送信
python3 scripts/ingest_client.py --since 2026-11-01 -v
```

## テスト

```bash
python3 -m unittest discover -s scripts -p "test_*.py" -v
```
