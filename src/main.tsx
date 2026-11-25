import "@mantine/core/styles.css";
import "@fullcalendar/react/skeleton.css";
import "@fullcalendar/react/themes/monarch/theme.css";
import "@fullcalendar/react/themes/monarch/palettes/blue.css";
import { Alert, AppShell, createTheme, Drawer, MantineProvider, Modal } from "@mantine/core";
import { useWindowEvent } from "@mantine/hooks";
import { lazy, Suspense, useCallback, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  type Block,
  type BlockResult,
  type DashboardSummary,
  type FilterOptions,
  type Summary,
  type UsageDay,
} from "../shared/timeline.ts";
import { DashboardSkeleton } from "./calendar/DashboardSkeleton.tsx";
import { CalendarRegion } from "./calendar/CalendarRegion.tsx";
import { isInputTarget } from "./calendar/gestures.ts";
import { Calendar } from "./calendar/Calendar.tsx";
import { YearCalendar, type YearJump } from "./calendar/YearCalendar.tsx";
import { movePeriod, period, type View } from "./calendar/period.ts";
import { Filters } from "./calendar/Filters.tsx";
import { BlockPanel, SummaryPanel } from "./calendar/Panels.tsx";
import { UsageDayPanel } from "./calendar/Usage.tsx";
import { filterParams, type AppView } from "./calendar/screen.ts";
import { useScreen } from "./calendar/use-screen.ts";
import { useResource } from "./calendar/use-resource.ts";
import { AppHeader } from "./layout/AppHeader.tsx";
import { CalendarToolbar } from "./calendar/CalendarToolbar.tsx";
import "./style.css";

const Dashboard = lazy(() => import("./calendar/Dashboard.tsx"));

function App() {
  const [lastCalendarView, setLastCalendarView] = useState<View>("week");
  const [jump, setJump] = useState<YearJump>();
  const [panel, setPanel] = useState<"filters" | "summary" | Block | UsageDay>();
  const restore = useCallback(({ view, anchor }: { view: AppView; anchor: number }) => {
    if (view !== "top") setLastCalendarView(view);
    setJump(view === "year" ? { start: period("year", anchor).start } : undefined);
    setPanel(undefined);
  }, []);
  const [screen, navigate] = useScreen(restore);
  const { view, anchor, selection } = screen;
  const { start, end } = period(view === "top" ? "day" : view, anchor);
  const filters = filterParams(selection);
  const query = new URLSearchParams(filters);
  query.set("start", String(start));
  query.set("end", String(end));
  const blocks = useResource<BlockResult>(
    view === "year" || view === "top" ? null : `/api/blocks?${query}`,
  );
  const options = useResource<FilterOptions>(panel === "filters" ? "/api/filters" : null);
  const dashboard = useResource<DashboardSummary>(
    view === "year" || view === "top" ? `/api/summary?${filters}` : null,
  );
  const summary = useResource<Summary>(
    panel === "summary" && view !== "top" ? `/api/summary?${query}` : null,
  );
  const latest = period("year", Date.now()).start;
  const earliest = dashboard.data ? period("year", dashboard.data.history.start).start : latest;
  const jumpToYear = useCallback(
    (target: number) => {
      setJump({ start: target });
      navigate({ anchor: target });
    },
    [navigate],
  );
  const move = useCallback(
    (direction: number) => {
      if (view === "top") return;
      if (view === "year") {
        jumpToYear(Math.max(earliest, Math.min(latest, movePeriod("year", start, direction))));
      } else {
        navigate({ anchor: movePeriod(view, anchor, direction) });
      }
    },
    [anchor, earliest, jumpToYear, latest, navigate, start, view],
  );
  const goLatest = useCallback(() => {
    if (view === "year") jumpToYear(latest);
    else if (view !== "top") navigate({ anchor: Date.now() });
  }, [jumpToYear, latest, navigate, view]);
  useWindowEvent("keydown", (event) => {
    if (
      view === "top" ||
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
  });
  const drillDown = (target: View, day: number) => {
    setJump(undefined);
    navigate({ view: target, anchor: day });
  };
  const changeView = (target: AppView) => {
    if (target === view) return;
    if (target === "top" && view !== "top") setLastCalendarView(view);
    setJump(undefined);
    navigate({ view: target });
  };
  const close = () => setPanel(undefined);
  return (
    <AppShell h="100dvh" display="flex" style={{ flexDirection: "column" }}>
      <AppHeader
        screen={screen}
        calendarView={view === "top" ? lastCalendarView : view}
        changeView={changeView}
        openFilters={() => setPanel("filters")}
        openSummary={() => setPanel("summary")}
      />
      <AppShell.Main flex={1} mih={0} display="flex" style={{ flexDirection: "column" }}>
        {view === "top" ? (
          <Suspense fallback={<DashboardSkeleton />}>
            <Dashboard
              summary={dashboard.data}
              failed={dashboard.error}
              pcs={selection.pcs}
              openMonth={(day) => drillDown("month", day)}
            />
          </Suspense>
        ) : (
          <>
            <CalendarToolbar
              view={view}
              start={start}
              end={end}
              earliest={earliest}
              latest={latest}
              move={move}
              goLatest={goLatest}
              changeView={changeView}
            />
            {(view === "year" ? dashboard.error : blocks.error) && (
              <Alert color="red" role="alert">
                稼働時間を取得できませんでした。
              </Alert>
            )}
            <CalendarRegion
              week={view === "week"}
              move={move}
              loading={view === "year" ? dashboard.loading : blocks.loading}
            >
              {view === "year" ? (
                dashboard.data && (
                  <YearCalendar
                    summary={dashboard.data}
                    anchor={anchor}
                    jump={jump}
                    latest={latest}
                    visible={(year) => {
                      if (year !== start) navigate({ anchor: year }, "replace");
                    }}
                    drillDown={drillDown}
                  />
                )
              ) : (
                <Calendar
                  key={view}
                  view={view}
                  start={start}
                  blocks={blocks.data?.blocks ?? []}
                  totals={blocks.data?.days ?? []}
                  usageDays={blocks.data?.accountUsage.days ?? []}
                  onSelect={setPanel}
                  onUsageSelect={setPanel}
                  drillDown={drillDown}
                />
              )}
            </CalendarRegion>
          </>
        )}
      </AppShell.Main>
      <Filters
        opened={panel === "filters"}
        close={close}
        options={options.data}
        failed={options.error}
        loading={options.loading}
        value={selection}
        change={(selection, mode) => navigate({ selection }, mode)}
      />
      <SummaryPanel
        opened={panel === "summary"}
        close={close}
        summary={summary.data}
        failed={summary.error}
        loading={summary.loading}
        yearStart={view === "year" ? start : undefined}
      />
      <BlockPanel
        block={typeof panel === "object" && "id" in panel ? panel : undefined}
        close={close}
      />
      <UsageDayPanel
        usage={typeof panel === "object" && "startDate" in panel ? panel : undefined}
        close={close}
      />
    </AppShell>
  );
}

const theme = createTheme({
  fontFamily: 'Inter, "Noto Sans JP", system-ui, sans-serif',
  defaultRadius: "md",
  components: {
    Drawer: Drawer.extend({ defaultProps: { removeScrollProps: { allowPinchZoom: true } } }),
    Modal: Modal.extend({ defaultProps: { removeScrollProps: { allowPinchZoom: true } } }),
  },
});

const root = document.getElementById("root");
if (root)
  createRoot(root).render(
    <MantineProvider forceColorScheme="dark" theme={theme}>
      <App />
    </MantineProvider>,
  );
