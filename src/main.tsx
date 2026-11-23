import { useCallback, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { ChartColumn, ChevronLeft, ChevronRight, SlidersHorizontal } from "lucide-react";
import {
  type Block,
  type BlockResult,
  type FilterOptions,
  type Summary,
} from "../shared/timeline.ts";
import { CalendarRegion } from "./calendar/CalendarRegion.tsx";
import { isInputTarget } from "./calendar/gestures.ts";
import { rangeLabel } from "./calendar/format.ts";
import { Grid } from "./calendar/Grid.tsx";
import { Markdown } from "./calendar/Markdown.tsx";
import { MonthGrid, YearGrid } from "./calendar/Overview.tsx";
import { movePeriod, period, views, type View } from "./calendar/period.ts";
import {
  BlockDetails,
  Filters,
  Panel,
  SummaryContent,
  type Selection,
} from "./calendar/Panels.tsx";
import { useResource } from "./calendar/use-resource.ts";
import "./style.css";

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
  const { start, end } = period(view, anchor);
  const query = new URLSearchParams({ start: String(start), end: String(end), q: selection.q });
  for (const agent of selection.agents) query.append("agents", agent);
  for (const project of selection.projects) query.append("projects", project);
  const blocks = useResource<BlockResult>(view === "year" ? null : `/api/blocks?${query}`);
  const options = useResource<FilterOptions>(panel === "filters" ? "/api/filters" : null);
  const summary = useResource<Summary>(
    view === "year" || panel === "summary" ? `/api/summary?${query}` : null,
  );
  const activeFilters = Boolean(
    selection.q || selection.agents.length || selection.projects.length,
  );
  const move = useCallback(
    (direction: number) => setAnchor((current) => movePeriod(view, current, direction)),
    [view],
  );
  const goLatest = useCallback(() => setAnchor(Date.now()), []);
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (
        panel ||
        event.defaultPrevented ||
        event.isComposing ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        isInputTarget(event.target)
      )
        return;
      switch (event.key) {
        case "ArrowLeft":
          move(-1);
          break;
        case "ArrowRight":
          move(1);
          break;
        case "t":
          goLatest();
          break;
        default:
          return;
      }
      event.preventDefault();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [goLatest, move, panel]);
  const drillDown = (target: View, day: number) => {
    setAnchor(day);
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
              aria-label={`前の${views[view]}`}
              aria-keyshortcuts="ArrowLeft"
              onClick={() => move(-1)}
            >
              <ChevronLeft size={19} />
            </button>
            <button
              className="icon-button"
              type="button"
              aria-label={`次の${views[view]}`}
              aria-keyshortcuts="ArrowRight"
              onClick={() => move(1)}
            >
              <ChevronRight size={19} />
            </button>
          </div>
          <button className="toolbar-button" type="button" aria-keyshortcuts="T" onClick={goLatest}>
            最新
          </button>
          <span className="range" aria-live="polite">
            {rangeLabel(view, start, end)}
          </span>
        </div>
        <div className="toolbar-actions">
          <div className="view-switch" aria-label="表示範囲" role="group">
            {(["year", "month", "week", "day"] as const).map((option) => (
              <button
                key={option}
                type="button"
                aria-pressed={view === option}
                onClick={() => setView(option)}
              >
                {views[option]}
              </button>
            ))}
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
      {(view === "year" ? summary?.error : blocks?.error) && (
        <p className="notice" role="alert">
          稼働時間を取得できませんでした。
        </p>
      )}
      <CalendarRegion week={view === "week"} move={move}>
        {view === "year" ? (
          <YearGrid
            start={start}
            now={now}
            days={summary?.data?.days ?? []}
            selectMonth={(day) => drillDown("month", day)}
          />
        ) : view === "month" ? (
          <MonthGrid
            start={start}
            end={end}
            now={now}
            days={blocks?.data?.days ?? []}
            blocks={blocks?.data?.blocks ?? []}
            selectDay={(day) => drillDown("day", day)}
          />
        ) : (
          <Grid
            start={start}
            end={end}
            now={now}
            blocks={blocks?.data?.blocks ?? []}
            totals={blocks?.data?.days ?? []}
            onSelect={setPanel}
          />
        )}
      </CalendarRegion>
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
            <SummaryContent
              summary={summary.data}
              yearStart={view === "year" ? start : undefined}
            />
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
