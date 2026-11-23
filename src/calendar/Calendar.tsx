import { useEffect, useMemo, useRef } from "react";
import FullCalendar, { type CalendarRef } from "@fullcalendar/react";
import dayGrid from "@fullcalendar/react/daygrid";
import interaction from "@fullcalendar/react/interaction";
import ja from "@fullcalendar/react/locales/ja";
import multiMonth from "@fullcalendar/react/multimonth";
import monarch from "@fullcalendar/react/themes/monarch";
import timeGrid from "@fullcalendar/react/timegrid";
import { Box, Group, Stack, Text, Tooltip } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { MINUTE, type Block, type DayTotal } from "../../shared/timeline.ts";
import { blockColor, clock, date, dayNumber, hours, usd, weekday } from "./format.ts";
import { plainText } from "./markdown.ts";
import type { View } from "./period.ts";

const plugins = [monarch, timeGrid, dayGrid, multiMonth, interaction];
const calendarViews = {
  year: "multiMonthYear",
  month: "dayGridMonth",
  week: "timeGridWeek",
  day: "timeGridDay",
} as const;

function EventLabel({
  block,
  title,
  hover,
  timeText,
}: {
  block: Block;
  title: string;
  hover: boolean;
  timeText: string;
}) {
  return (
    <Tooltip
      disabled={!hover}
      events={{ hover: true, focus: false, touch: false }}
      openDelay={120}
      multiline
      withArrow
      maw="min(320px, calc(100vw - 24px))"
      label={
        <Stack gap={3} style={{ overflowWrap: "anywhere" }}>
          <Text size="sm" fw={700}>
            {title}
          </Text>
          <Text size="xs">
            {date.format(block.start)} {clock.format(block.start)} – {date.format(block.end)}{" "}
            {clock.format(block.end)}
            {" · "}
            {Math.round((block.end - block.start) / MINUTE)}分
          </Text>
          {block.model && <Text size="xs">{block.model}</Text>}
          <Text size="xs">概算API料金（セッション） {usd(block.estimatedCostUsd)}</Text>
        </Stack>
      }
    >
      <Text component="span" size="xs" truncate display="block">
        {block.agent} {timeText} {title}
      </Text>
    </Tooltip>
  );
}

export function Calendar({
  view,
  start,
  blocks,
  totals,
  onSelect,
  drillDown,
}: {
  view: View;
  start: number;
  blocks: Block[];
  totals: DayTotal[];
  onSelect: (block: Block) => void;
  drillDown: (target: View, day: number) => void;
}) {
  const ref = useRef<CalendarRef>(null);
  const hover = useMediaQuery("(hover: hover) and (pointer: fine)");
  const timeline = view === "week" || view === "day";
  // The library owns the current date inside a view. The app owns navigation between views.
  useEffect(() => {
    ref.current?.getApi().gotoDate(new Date(start));
  }, [start]);
  const entries = useMemo(
    () =>
      new Map(
        blocks.map((block) => {
          const color = blockColor(block.agent, block.project);
          return [
            String(block.id),
            {
              block,
              event: {
                id: String(block.id),
                title: plainText(block.title) || block.project,
                start: new Date(block.start),
                end: new Date(block.end),
                backgroundColor: `color-mix(in srgb, ${color} 24%, #131923)`,
                borderColor: color,
                textColor: "#f3f6ff",
              },
            },
          ] as const;
        }),
      ),
    [blocks],
  );
  const events = useMemo(() => Array.from(entries.values(), ({ event }) => event), [entries]);
  const activity = useMemo(() => new Map(totals.map(({ day, ms }) => [day, ms])), [totals]);
  const max = Math.max(1, ...activity.values());
  const clickDate = (day: Date) => drillDown(view === "year" ? "month" : "day", day.getTime());
  return (
    <Box component="section" aria-label="稼働時間カレンダー" h="100%" style={{ overflow: "auto" }}>
      <Box h="100%" miw={view === "week" ? 840 : undefined} className="calendar-display">
        <FullCalendar
          ref={ref}
          plugins={plugins}
          locale={ja}
          initialView={calendarViews[view]}
          initialDate={new Date(start)}
          timeZone="Asia/Tokyo"
          colorScheme="dark"
          firstDay={0}
          headerToolbar={false}
          height="100%"
          nowIndicator={timeline}
          allDaySlot={false}
          scrollTime="08:00:00"
          scrollTimeReset={false}
          slotDuration="01:00:00"
          slotHeaderFormat={{ hour: "2-digit", minute: "2-digit", hour12: false }}
          slotEventOverlap={false}
          eventMinHeight={12}
          eventInteractive
          eventOrder={timeline ? "start,-duration,title" : "-duration,start,title"}
          dayMaxEvents={view === "month" ? 1 : undefined}
          showNonCurrentDates={false}
          fixedWeekCount={false}
          singleMonthMinWidth={290}
          navLinks={!timeline}
          navLinkDayClick={clickDate}
          datesSet={({ view: current }) => {
            if (view === "year" && current.type === "dayGridMonth") clickDate(current.currentStart);
          }}
          dateClick={({ date }) => {
            if (!timeline) clickDate(date);
          }}
          dayHeaderRowClass={view === "week" ? "week-swipe-header" : undefined}
          dayHeaderContent={
            timeline
              ? ({ date: day }) => (
                  <Group gap={5} justify="center" wrap="nowrap">
                    <Text
                      size="sm"
                      c={
                        weekday.format(day) === "日"
                          ? "red.3"
                          : weekday.format(day) === "土"
                            ? "blue.3"
                            : undefined
                      }
                    >
                      {weekday.format(day)} {date.format(day)}
                    </Text>
                    <Text size="xs" c="dimmed">
                      {hours(activity.get(day.getTime()) ?? 0)}
                    </Text>
                  </Group>
                )
              : undefined
          }
          dayCellTopContent={
            !timeline
              ? ({ date: day, inPopover }) => {
                  if (inPopover) return true;
                  const ms = activity.get(day.getTime()) ?? 0;
                  return (
                    <Box
                      component="span"
                      display="block"
                      px={3}
                      style={{
                        borderRadius: 4,
                        background: ms
                          ? `color-mix(in srgb, var(--mantine-color-blue-7) ${15 + (ms / max) * 55}%, transparent)`
                          : undefined,
                      }}
                    >
                      <Text component="span" display="block" size="xs" fw={700}>
                        {dayNumber.format(day)}
                      </Text>
                      {(view === "month" || ms > 0) && (
                        <Text component="span" display="block" size="xs">
                          {hours(ms)}
                        </Text>
                      )}
                    </Box>
                  );
                }
              : undefined
          }
          events={events}
          eventClick={({ event }) => {
            const entry = entries.get(event.id);
            if (entry) onSelect(entry.block);
          }}
          eventContent={({ event, timeText }) => {
            const entry = entries.get(event.id);
            return entry ? (
              <EventLabel
                block={entry.block}
                title={event.title}
                hover={hover}
                timeText={timeline ? timeText : ""}
              />
            ) : null;
          }}
        />
      </Box>
    </Box>
  );
}
