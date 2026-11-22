import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import type { Block, FilterOptions, GroupTotal, Summary } from "../../shared/timeline.ts";
import { clock, date, hours, weekday } from "./format.ts";

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
        close();
      }}
    >
      <div className="panel-heading">
        <div id={titleId} className="panel-title">
          {typeof title === "string" ? <h2>{title}</h2> : title}
        </div>
        <button className="icon-button" type="button" aria-label="閉じる" onClick={close}>
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

export function SummaryContent({ summary }: { summary: Summary }) {
  const max = Math.max(1, ...summary.days.map(({ ms }) => ms));
  return (
    <div className="summary">
      <div className="total">
        <span>合計稼働時間</span>
        <strong>{hours(summary.totalMs)}</strong>
      </div>
      <section>
        <h3>日別</h3>
        <ul className="daily-chart">
          {summary.days.map(({ day, ms }) => (
            <li key={day}>
              <span>
                {date.format(day)} ({weekday.format(day)})
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

export function BlockDetails({ block }: { block: Block }) {
  const fields = [
    ["エージェント", block.agent],
    ["プロジェクト", block.project],
    ["作業ディレクトリ", block.cwd],
    ["開始", `${date.format(block.start)} ${clock.format(block.start)}`],
    ["終了", `${date.format(block.end)} ${clock.format(block.end)}`],
    ["稼働時間", `${Math.round((block.end - block.start) / 60_000)}分`],
    ["モデル", block.model],
    ["Gitブランチ", block.branch],
    ["トークン", block.tokens?.toLocaleString("ja-JP")],
  ];
  return (
    <dl className="details">
      {fields.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{value || "—"}</dd>
        </div>
      ))}
    </dl>
  );
}
