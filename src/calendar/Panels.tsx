import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import {
  MINUTE,
  type Block,
  type FilterOptions,
  type GroupTotal,
  type Metrics,
  type Summary,
} from "../../shared/timeline.ts";
import { clock, date, hours, monthNumber, weekday } from "./format.ts";
import { isInputTarget } from "./gestures.ts";
import { monthTotals } from "./period.ts";

export function Panel({
  title,
  close,
  children,
  wide = false,
}: {
  title: ReactNode;
  close: () => void;
  children: ReactNode;
  wide?: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className={wide ? "panel wide" : "panel"}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        if (!isInputTarget(document.activeElement)) close();
      }}
    >
      <div className="panel-heading">
        <div id={titleId} className="panel-title">
          {typeof title === "string" ? <h2>{title}</h2> : title}
        </div>
        <button
          className="icon-button"
          type="button"
          aria-label="閉じる"
          aria-keyshortcuts="Escape"
          onClick={close}
        >
          <X size={19} />
        </button>
      </div>
      {children}
    </dialog>
  );
}

export type Selection = FilterOptions & { q: string };

export function Filters({
  options,
  value,
  change,
}: {
  options: FilterOptions;
  value: Selection;
  change: (value: Selection) => void;
}) {
  return (
    <div className="filter-fields">
      <label className="search-field">
        検索
        <input
          type="search"
          value={value.q}
          onChange={(e) => change({ ...value, q: e.target.value })}
        />
      </label>
      {(["agents", "projects"] as const).map((key) => (
        <fieldset key={key}>
          <legend>{key === "agents" ? "エージェント" : "プロジェクト"}</legend>
          <div className="choices">
            {options[key].map((item) => (
              <label key={item}>
                <input
                  type="checkbox"
                  checked={value[key].includes(item)}
                  onChange={(e) =>
                    change({
                      ...value,
                      [key]: e.target.checked
                        ? [...value[key], item]
                        : value[key].filter((entry) => entry !== item),
                    })
                  }
                />
                <span>{item}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ))}
      <button
        className="toolbar-button"
        type="button"
        onClick={() => change({ agents: [], projects: [], q: "" })}
      >
        解除
      </button>
    </div>
  );
}

function Breakdown({ title, totals }: { title: string; totals: GroupTotal[] }) {
  return (
    <section>
      <h3>{title}</h3>
      <dl className="breakdown">
        {totals.map(({ name, ms }) => (
          <div key={name}>
            <dt>{name}</dt>
            <dd>{hours(ms)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

export function SummaryContent({ summary, yearStart }: { summary: Summary; yearStart?: number }) {
  const monthly = yearStart !== undefined;
  const totals = monthly ? monthTotals(yearStart, summary.days) : summary.days;
  const max = Math.max(1, ...totals.map(({ ms }) => ms));
  return (
    <div className="summary">
      <div className="total">
        <span>合計稼働時間</span>
        <strong>{hours(summary.totalMs)}</strong>
      </div>
      <WorkMetrics metrics={summary} title="対象セッションの合計" />
      <section>
        <h3>{monthly ? "月別" : "日別"}</h3>
        <ul className="daily-chart">
          {totals.map(({ day, ms }) => (
            <li key={day}>
              <span>
                {monthly ? monthNumber.format(day) : `${date.format(day)} (${weekday.format(day)})`}
              </span>
              <div className="bar-track">
                <div style={{ width: `${(ms / max) * 100}%` }} />
              </div>
              <span>{hours(ms)}</span>
            </li>
          ))}
        </ul>
      </section>
      <div className="summary-groups">
        <Breakdown title="エージェント別" totals={summary.agents} />
        <Breakdown title="プロジェクト別" totals={summary.projects} />
      </div>
    </div>
  );
}

const count = new Intl.NumberFormat("ja-JP");
const metricFields: Array<[keyof Metrics, string]> = [
  ["inputTokens", "入力トークン"],
  ["cachedInputTokens", "キャッシュ済み入力"],
  ["outputTokens", "出力トークン"],
  ["reasoningTokens", "推論トークン"],
  ["turnCount", "ターン数"],
  ["toolCallCount", "ツール呼び出し"],
];

function Details({ fields }: { fields: Array<[string, string | undefined]> }) {
  return (
    <dl className="details">
      {fields.map(
        ([label, value]) =>
          value && (
            <div key={label}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </div>
          ),
      )}
    </dl>
  );
}

function WorkMetrics({ metrics, title }: { metrics: Metrics; title: string }) {
  const fields: Array<[string, string]> = [];
  for (const [key, label] of metricFields) {
    const value = metrics[key];
    if (value !== undefined) fields.push([label, count.format(value)]);
  }
  return (
    fields.length > 0 && (
      <section className="work-metrics">
        <h3>{title}</h3>
        <Details fields={fields} />
      </section>
    )
  );
}

export function BlockDetails({ block }: { block: Block }) {
  return (
    <div className="detail-content">
      <Details
        fields={[
          ["エージェント", block.agent],
          ["プロジェクト", block.project],
          ["作業ディレクトリ", block.cwd],
          ["開始", `${date.format(block.start)} ${clock.format(block.start)}`],
          ["終了", `${date.format(block.end)} ${clock.format(block.end)}`],
          ["稼働時間", `${Math.round((block.end - block.start) / MINUTE)}分`],
          ["モデル", block.model],
          ["推論レベル", block.reasoningEffort],
          ["Gitブランチ", block.branch],
          ["Gitコミット", block.gitSha],
          ["Gitリモート", block.gitOriginUrl],
          ["合計トークン", block.tokens?.toLocaleString("ja-JP")],
        ]}
      />
      <WorkMetrics metrics={block} title="セッションの集計" />
      {block.preview && (
        <section className="detail-preview">
          <h3>概要</h3>
          <p>{block.preview}</p>
        </section>
      )}
    </div>
  );
}
