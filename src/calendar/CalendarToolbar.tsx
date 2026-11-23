import { ActionIcon, Button, Group, Paper, SegmentedControl, Text } from "@mantine/core";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { rangeLabel } from "./format.ts";
import { views, type View } from "./period.ts";

export function CalendarToolbar({
  view,
  start,
  end,
  earliest,
  latest,
  move,
  goLatest,
  changeView,
}: {
  view: View;
  start: number;
  end: number;
  earliest: number;
  latest: number;
  move: (direction: number) => void;
  goLatest: () => void;
  changeView: (view: View) => void;
}) {
  return (
    <Paper component="section" aria-label="カレンダー操作" radius={0} bg="dark.8">
      <Group px={{ base: "xs", sm: "lg" }} py="xs" gap="xs" justify="space-between">
        <Group gap={6} wrap="nowrap">
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
          <Text
            component="h1"
            fz="md"
            aria-live="polite"
            ml={4}
            fw={650}
            style={{ whiteSpace: "nowrap" }}
          >
            {rangeLabel(view, start, end)}
          </Text>
        </Group>
        <SegmentedControl<View>
          aria-label="カレンダーの表示単位"
          value={view}
          onChange={changeView}
          data={[
            { value: "year", label: views.year },
            { value: "month", label: views.month },
            { value: "week", label: views.week },
            { value: "day", label: views.day },
          ]}
          styles={{ label: { minHeight: 40, minWidth: 42, display: "grid", placeItems: "center" } }}
        />
      </Group>
    </Paper>
  );
}
