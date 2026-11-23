import "@mantine/core/styles.css";
import "@fullcalendar/react/skeleton.css";
import "@fullcalendar/react/themes/monarch/theme.css";
import "@fullcalendar/react/themes/monarch/palettes/blue.css";
import {
  ActionIcon,
  Alert,
  Box,
  Button,
  createTheme,
  Group,
  MantineProvider,
  SegmentedControl,
  Text,
  Title,
} from "@mantine/core";
import { useWindowEvent } from "@mantine/hooks";
import { useCallback, useState } from "react";
import { createRoot } from "react-dom/client";
import { ChartColumn, ChevronLeft, ChevronRight, SlidersHorizontal } from "lucide-react";
import {
  type Block,
  type BlockResult,
  type DashboardSummary,
  type FilterOptions,
  type Summary,
  type UsageDay,
} from "../shared/timeline.ts";
import { CalendarRegion } from "./calendar/CalendarRegion.tsx";
import { isInputTarget } from "./calendar/gestures.ts";
import { rangeLabel } from "./calendar/format.ts";
import { Calendar } from "./calendar/Calendar.tsx";
import { Dashboard } from "./calendar/Dashboard.tsx";
import { YearCalendar, type YearJump } from "./calendar/YearCalendar.tsx";
import { movePeriod, period, views, type View } from "./calendar/period.ts";
import { Filters, type Selection } from "./calendar/Filters.tsx";
import { BlockPanel, SummaryPanel } from "./calendar/Panels.tsx";
import { UsageDayPanel } from "./calendar/Usage.tsx";
import { useResource } from "./calendar/use-resource.ts";
import "./style.css";

type AppView = View | "top";
const navigation = [
  { value: "top", label: "トップ" },
  { value: "year", label: "年" },
  { value: "month", label: "月" },
  { value: "week", label: "週" },
  { value: "day", label: "日" },
] satisfies Array<{ value: AppView; label: string }>;

function App() {
  const [anchor, setAnchor] = useState(Date.now);
  const [view, setView] = useState<AppView>("top");
  const [jump, setJump] = useState<YearJump>();
  const [panel, setPanel] = useState<"filters" | "summary" | Block | UsageDay>();
  const [selection, setSelection] = useState<Selection>({ agents: [], projects: [], q: "" });
  const { start, end } = period(view === "top" ? "day" : view, anchor);
  const filters = new URLSearchParams({ q: selection.q });
  for (const agent of selection.agents) filters.append("agents", agent);
  for (const project of selection.projects) filters.append("projects", project);
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
  const activeFilters = Boolean(
    selection.q || selection.agents.length || selection.projects.length,
  );
  const latest = period("year", Date.now()).start;
  const earliest = dashboard?.data ? period("year", dashboard.data.history.start).start : latest;
  const jumpToYear = useCallback((target: number) => {
    setJump({ start: target });
  }, []);
  const move = useCallback(
    (direction: number) => {
      if (view === "top") return;
      if (view === "year") {
        jumpToYear(Math.max(earliest, Math.min(latest, movePeriod("year", start, direction))));
      } else {
        setAnchor((current) => movePeriod(view, current, direction));
      }
    },
    [earliest, jumpToYear, latest, start, view],
  );
  const goLatest = useCallback(() => {
    if (view === "year") jumpToYear(latest);
    else setAnchor(Date.now());
  }, [jumpToYear, latest, view]);
  useWindowEvent("keydown", (event) => {
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
  });
  const drillDown = (target: View, day: number) => {
    setAnchor(day);
    setView(target);
  };
  const changeView = (target: AppView) => {
    setJump(undefined);
    setView(target);
  };
  const close = () => setPanel(undefined);
  return (
    <Box component="main" h="100dvh" display="flex" style={{ flexDirection: "column" }}>
      <Group component="header" p="sm" bg="dark.7" gap="xs" wrap="wrap">
        <Title order={1} size="h5" w={{ base: "100%", md: "auto" }}>
          エージェント稼働時間
        </Title>
        {view !== "top" && (
          <Group gap={6} wrap="wrap">
            <ActionIcon
              variant="default"
              size={42}
              aria-label={`前の${views[view]}`}
              aria-keyshortcuts="ArrowLeft"
              onClick={() => move(-1)}
              disabled={view === "year" && start <= earliest}
            >
              <ChevronLeft size={19} />
            </ActionIcon>
            <ActionIcon
              variant="default"
              size={42}
              aria-label={`次の${views[view]}`}
              aria-keyshortcuts="ArrowRight"
              onClick={() => move(1)}
              disabled={view === "year" && start >= latest}
            >
              <ChevronRight size={19} />
            </ActionIcon>
            <Button variant="default" mih={42} aria-keyshortcuts="T" onClick={goLatest}>
              最新
            </Button>
            <Text aria-live="polite" fw={650} style={{ whiteSpace: "nowrap" }}>
              {rangeLabel(view, start, end)}
            </Text>
          </Group>
        )}
        <Group gap={6} wrap="wrap" ml={{ base: 0, md: "auto" }}>
          <SegmentedControl<AppView>
            aria-label="表示範囲"
            value={view}
            onChange={changeView}
            data={navigation}
            styles={{
              label: { minHeight: 40, minWidth: 34, display: "grid", placeItems: "center" },
            }}
          />
          <Button
            variant={activeFilters ? "light" : "default"}
            mih={42}
            leftSection={<SlidersHorizontal size={16} />}
            onClick={() => setPanel("filters")}
          >
            絞り込み
          </Button>
          {view !== "top" && (
            <Button
              variant="default"
              mih={42}
              leftSection={<ChartColumn size={16} />}
              onClick={() => setPanel("summary")}
            >
              集計
            </Button>
          )}
        </Group>
      </Group>
      {view === "top" ? (
        <Dashboard
          summary={dashboard?.data}
          failed={dashboard?.error}
          openMonth={(day) => drillDown("month", day)}
        />
      ) : (
        <>
          {(view === "year" ? dashboard?.error : blocks?.error) && (
            <Alert color="red" role="alert">
              稼働時間を取得できませんでした。
            </Alert>
          )}
          <CalendarRegion week={view === "week"} move={move}>
            {view === "year" ? (
              dashboard?.data && (
                <YearCalendar
                  summary={dashboard.data}
                  anchor={anchor}
                  jump={jump}
                  latest={latest}
                  visible={setAnchor}
                  drillDown={drillDown}
                />
              )
            ) : (
              <Calendar
                key={view}
                view={view}
                start={start}
                blocks={blocks?.data?.blocks ?? []}
                totals={blocks?.data?.days ?? []}
                usageDays={blocks?.data?.accountUsage.days ?? []}
                onSelect={setPanel}
                onUsageSelect={setPanel}
                drillDown={drillDown}
              />
            )}
          </CalendarRegion>
        </>
      )}
      <Filters
        opened={panel === "filters"}
        close={close}
        options={options?.data}
        failed={options?.error}
        value={selection}
        change={setSelection}
      />
      <SummaryPanel
        opened={panel === "summary"}
        close={close}
        summary={summary?.data}
        failed={summary?.error}
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
    </Box>
  );
}

const theme = createTheme({
  fontFamily: 'Inter, "Noto Sans JP", system-ui, sans-serif',
  defaultRadius: "md",
});

const root = document.getElementById("root");
if (root)
  createRoot(root).render(
    <MantineProvider forceColorScheme="dark" theme={theme}>
      <App />
    </MantineProvider>,
  );
