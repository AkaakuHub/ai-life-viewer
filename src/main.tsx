import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { ChartColumn, ChevronLeft, ChevronRight, SlidersHorizontal } from "lucide-react";
import {
  DAY,
  dayStart,
  type Block,
  type BlockResult,
  type FilterOptions,
  type Summary,
} from "../shared/timeline.ts";
import { date, weekday } from "./calendar/format.ts";
import { Grid } from "./calendar/Grid.tsx";
import { Markdown } from "./calendar/Markdown.tsx";
import { weekStart } from "./calendar/layout.ts";
import {
  BlockDetails,
  Filters,
  Panel,
  SummaryContent,
  type Selection,
} from "./calendar/Panels.tsx";
import { useResource } from "./calendar/use-resource.ts";
import "./style.css";

type View = "week" | "day";

function App() {
  const [now, setNow] = useState(Date.now);
  const [anchor, setAnchor] = useState(Date.now);
  const [view, setView] = useState<View>(() =>
    window.matchMedia("(max-width: 600px)").matches ? "day" : "week",
  );
  const [panel, setPanel] = useState<"filters" | "summary" | Block>();
  const [selection, setSelection] = useState<Selection>({ agents: [], projects: [], q: "" });
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 15_000);
    return () => window.clearInterval(timer);
  }, []);
  const step = (view === "week" ? 7 : 1) * DAY;
  const start = view === "week" ? weekStart(anchor) : dayStart(anchor);
  const end = start + step;
  const query = new URLSearchParams({ start: String(start), end: String(end), q: selection.q });
  for (const agent of selection.agents) query.append("agents", agent);
  for (const project of selection.projects) query.append("projects", project);
  const blocks = useResource<BlockResult>(`/api/blocks?${query}`);
  const options = useResource<FilterOptions>(panel === "filters" ? "/api/filters" : null);
  const summary = useResource<Summary>(panel === "summary" ? `/api/summary?${query}` : null);
  const activeFilters = Boolean(
    selection.q || selection.agents.length || selection.projects.length,
  );
  const move = (direction: number) => setAnchor(anchor + direction * step);
  const changeView = (target: View) => {
    if (
      target === "day" &&
      view === "week" &&
      dayStart(anchor) === start &&
      now >= start &&
      now < end
    ) {
      setAnchor(now);
    }
    setView(target);
  };
  return (
    <main>
      <header className="toolbar">
        <h1>エージェント稼働時間</h1>
        <div className="navigation">
          <div className="arrows">
            <button
              className="icon-button"
              type="button"
              aria-label={view === "week" ? "前の週" : "前の日"}
              onClick={() => move(-1)}
            >
              <ChevronLeft size={19} />
            </button>
            <button
              className="icon-button"
              type="button"
              aria-label={view === "week" ? "次の週" : "次の日"}
              onClick={() => move(1)}
            >
              <ChevronRight size={19} />
            </button>
          </div>
          <button className="toolbar-button" type="button" onClick={() => setAnchor(Date.now())}>
            最新
          </button>
          <span className="range" aria-live="polite">
            {view === "week"
              ? `${date.format(start)} - ${date.format(end - DAY)}`
              : `${date.format(start)} (${weekday.format(start)})`}
          </span>
        </div>
        <div className="toolbar-actions">
          <div className="view-switch" aria-label="表示範囲" role="group">
            <button type="button" aria-pressed={view === "week"} onClick={() => changeView("week")}>
              週
            </button>
            <button type="button" aria-pressed={view === "day"} onClick={() => changeView("day")}>
              日
            </button>
          </div>
          <button
            className="toolbar-button"
            type="button"
            data-active={activeFilters}
            onClick={() => setPanel("filters")}
          >
            <SlidersHorizontal size={16} />
            絞り込み
          </button>
          <button className="toolbar-button" type="button" onClick={() => setPanel("summary")}>
            <ChartColumn size={16} />
            集計
          </button>
        </div>
      </header>
      {blocks?.error && (
        <p className="notice" role="alert">
          稼働時間を取得できませんでした。
        </p>
      )}
      <Grid
        start={start}
        end={end}
        now={now}
        blocks={blocks?.data?.blocks ?? []}
        totals={blocks?.data?.days ?? []}
        onSelect={setPanel}
      />
      {panel === "filters" && (
        <Panel title="絞り込み" close={() => setPanel(undefined)}>
          {options?.error && <p role="alert">絞り込みの項目を取得できませんでした。</p>}
          <Filters
            options={options?.data ?? { agents: [], projects: [] }}
            value={selection}
            change={setSelection}
          />
        </Panel>
      )}
      {panel === "summary" && (
        <Panel title="集計" close={() => setPanel(undefined)} wide>
          {summary?.data ? (
            <SummaryContent summary={summary.data} />
          ) : (
            <p role="status">{summary?.error ? "集計を取得できませんでした。" : "読み込み中"}</p>
          )}
        </Panel>
      )}
      {typeof panel === "object" && (
        <Panel title={<Markdown>{panel.title}</Markdown>} close={() => setPanel(undefined)} wide>
          <BlockDetails block={panel} />
        </Panel>
      )}
    </main>
  );
}

const root = document.getElementById("root");
if (root) createRoot(root).render(<App />);
