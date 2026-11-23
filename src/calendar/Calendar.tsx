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
import { MINUTE, dayKey, type Block, type DayTotal, type UsageDay } from "../../shared/timeline.ts";
import {
  blockColor,
  blockTime,
  duration,
  clock,
  date,
  dayNumber,
  hours,
  usd,
  weekday,
} from "./format.ts";
import { plainText } from "./markdown.ts";
import { UsageBadge } from "./Usage.tsx";
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
  timeline,
}: {
  block: Block;
  title: string;
  hover: boolean;
  timeline: boolean;
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
          <Text size="xs">時間帯 {blockTime(block)}</Text>
          <Text size="xs">実稼働 {duration(block.activeMs)}</Text>
          {block.model && <Text size="xs">{block.model}</Text>}
          <Text size="xs">概算API料金（セッション） {usd(block.estimatedCostUsd)}</Text>
        </Stack>
      }
    >
      {timeline ? (
        <Box
          component="span"
          display="block"
          style={{ whiteSpace: "normal", overflowWrap: "anywhere", overflow: "hidden" }}
        >
          <Text component="span" display="block" size="xs" fw={650} lh={1.2}>
            {block.agent} {clock.format(block.start)}
          </Text>
          <Text component="span" display="block" size="xs" lh={1.25}>
            {title}
          </Text>
        </Box>
      ) : (
        <Text component="span" size="xs" truncate display="block">
          {title}
        </Text>
      )}
    </Tooltip>
  );
}

export function Calendar({
  view,
  start,
  blocks,
  totals,
  usageDays,
  onSelect,
  onUsageSelect,
  drillDown,
}: {
  view: View;
  start: number;
  blocks: Block[];
  totals: DayTotal[];
  usageDays: UsageDay[];
  onSelect?: (block: Block) => void;
  onUsageSelect?: (usage: UsageDay) => void;
  drillDown: (target: View, day: number) => void;
}) {
  const ref = useRef<CalendarRef>(null);
  const hover = useMediaQuery("(hover: hover) and (pointer: fine)");
  const timeline = view === "week" || view === "day";
  const yearly = view === "year";
  // The library owns the current date inside a view. The app owns navigation between views.
  useEffect(() => {
    ref.current?.getApi().gotoDate(new Date(start));
  }, [start]);
  const entries = useMemo(
    () =>
      new Map(
        blocks.map((block) => {
          const color = blockColor(block.agent, block.project);
          const id = `${block.id}:${block.start}`;
          return [
            id,
            {
              block,
              event: {
                id: id,
                title: plainText(block.title) || block.project,
                start: new Date(block.start),
                end: new Date(block.end),
                color: `${color}80`,
                contrastColor: "#f3f6ff",
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
  const dailyUsage = useMemo(
    () => new Map(usageDays.map((day) => [day.startDate, day])),
    [usageDays],
  );
  const clickDate = (day: Date) => drillDown(view === "year" ? "month" : "day", day.getTime());
  return (
    <Box
      component="section"
      aria-label="稼働時間カレンダー"
      h={yearly ? undefined : "100%"}
      style={{ overflow: yearly ? undefined : "auto" }}
    >
      <Box
        h={yearly ? undefined : "100%"}
        mih={view === "month" ? 560 : undefined}
        miw={view === "week" ? 840 : undefined}
        className="calendar-display"
      >
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
          height={yearly ? "auto" : "100%"}
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
          dayMaxEvents={timeline ? undefined : 1}
          showNonCurrentDates={yearly}
          fixedWeekCount={!timeline}
          expandRows={!timeline}
          moreLinkContent={({ num }) => `+${num}件`}
          dayCellTopInnerClass={
            !timeline
              ? ({ isToday }) =>
                  isToday ? "day-summary-link day-summary-today" : "day-summary-link"
              : undefined
          }
          eventClass={timeline ? "timeline-block" : undefined}
          singleMonthMinWidth={310}
          aspectRatio={yearly ? 1.1 : undefined}
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
              ? ({ date: day }) => {
                  const usage = view === "day" ? dailyUsage.get(dayKey(day.getTime())) : undefined;
                  return (
                    <Group gap={5} justify="center" wrap="wrap">
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
                      {usage && <UsageBadge usage={usage} onClick={() => onUsageSelect?.(usage)} />}
                    </Group>
                  );
                }
              : undefined
          }
          dayCellTopContent={
            !timeline
              ? ({ date: day, inPopover, isOther }) => {
                  if (inPopover) return true;
                  const ms = isOther ? 0 : (activity.get(day.getTime()) ?? 0);
                  const usage = isOther ? undefined : dailyUsage.get(dayKey(day.getTime()));
                  return (
                    <Box
                      component="span"
                      display="block"
                      w="100%"
                      mih={42}
                      ta="start"
                      px={3}
                      className="day-summary"
                      style={{
                        borderRadius: 6,
                        background: ms
                          ? `color-mix(in srgb, var(--mantine-color-blue-7) ${15 + (ms / max) * 55}%, transparent)`
                          : undefined,
                      }}
                    >
                      <Text component="span" display="block" size="xs" lh={1.15} fw={700}>
                        {dayNumber.format(day)}
                      </Text>
                      <Text component="span" display="block" size="xs" lh={1.15} truncate>
                        {isOther || ms === 0
                          ? "–"
                          : ms < 3 * MINUTE
                            ? `${Math.max(1, Math.round(ms / MINUTE))}分`
                            : hours(ms)}
                      </Text>
                      {usage && <UsageBadge usage={usage} compact />}
                    </Box>
                  );
                }
              : undefined
          }
          events={events}
          eventClick={({ event }) => {
            const entry = entries.get(event.id);
            if (entry) onSelect?.(entry.block);
          }}
          eventContent={({ event }) => {
            const entry = entries.get(event.id);
            return entry ? (
              <EventLabel
                block={entry.block}
                title={event.title}
                hover={hover}
                timeline={timeline}
              />
            ) : null;
          }}
        />
      </Box>
    </Box>
  );
}
