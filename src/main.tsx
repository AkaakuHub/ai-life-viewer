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
  type FilterOptions,
  type Summary,
} from "../shared/timeline.ts";
import { CalendarRegion } from "./calendar/CalendarRegion.tsx";
import { isInputTarget } from "./calendar/gestures.ts";
import { rangeLabel } from "./calendar/format.ts";
import { Calendar } from "./calendar/Calendar.tsx";
import { movePeriod, period, views, type View } from "./calendar/period.ts";
import { Filters, type Selection } from "./calendar/Filters.tsx";
import { BlockPanel, SummaryPanel } from "./calendar/Panels.tsx";
import { useResource } from "./calendar/use-resource.ts";
import "./style.css";

function App() {
  const [anchor, setAnchor] = useState(Date.now);
  const [view, setView] = useState<View>(() =>
    window.matchMedia("(max-width: 600px)").matches ? "day" : "week",
  );
  const [panel, setPanel] = useState<"filters" | "summary" | Block>();
  const [selection, setSelection] = useState<Selection>({ agents: [], projects: [], q: "" });
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
  const close = () => setPanel(undefined);
  return (
    <Box component="main" h="100dvh" display="flex" style={{ flexDirection: "column" }}>
      <Group component="header" p="sm" bg="dark.7" gap="xs" wrap="wrap">
        <Title order={1} size="h5" w={{ base: "100%", md: "auto" }}>
          エージェント稼働時間
        </Title>
        <Group gap={6} wrap="wrap">
          <ActionIcon
            variant="default"
            size={42}
            aria-label={`前の${views[view]}`}
            aria-keyshortcuts="ArrowLeft"
            onClick={() => move(-1)}
          >
            <ChevronLeft size={19} />
          </ActionIcon>
          <ActionIcon
            variant="default"
            size={42}
            aria-label={`次の${views[view]}`}
            aria-keyshortcuts="ArrowRight"
            onClick={() => move(1)}
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
        <Group gap={6} wrap="wrap" ml={{ base: 0, md: "auto" }}>
          <SegmentedControl<View>
            aria-label="表示範囲"
            value={view}
            onChange={setView}
            data={[
              { value: "year", label: "年" },
              { value: "month", label: "月" },
              { value: "week", label: "週" },
              { value: "day", label: "日" },
            ]}
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
          <Button
            variant="default"
            mih={42}
            leftSection={<ChartColumn size={16} />}
            onClick={() => setPanel("summary")}
          >
            集計
          </Button>
        </Group>
      </Group>
      {(view === "year" ? summary?.error : blocks?.error) && (
        <Alert color="red" role="alert">
          稼働時間を取得できませんでした。
        </Alert>
      )}
      <CalendarRegion week={view === "week"} move={move}>
        <Calendar
          key={view}
          view={view}
          start={start}
          blocks={blocks?.data?.blocks ?? []}
          totals={(view === "year" ? summary?.data?.days : blocks?.data?.days) ?? []}
          onSelect={setPanel}
          drillDown={drillDown}
        />
      </CalendarRegion>
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
      <BlockPanel block={typeof panel === "object" ? panel : undefined} close={close} />
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
