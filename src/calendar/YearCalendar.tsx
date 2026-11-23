import { Box, Stack, Title } from "@mantine/core";
import { useReducedMotion } from "@mantine/hooks";
import { useLayoutEffect, useMemo, useRef } from "react";
import { type DashboardSummary, type DayTotal, type UsageDay } from "../../shared/timeline.ts";
import { Calendar } from "./Calendar.tsx";
import { yearName } from "./format.ts";
import { movePeriod, period, type View } from "./period.ts";

export type YearJump = { start: number };

function splitYears(summary: DashboardSummary, earliest: number, latest: number) {
  const years = new Map<number, { totals: DayTotal[]; usageDays: UsageDay[] }>();
  for (let year = earliest; year <= latest; year = movePeriod("year", year, 1)) {
    years.set(year, { totals: [], usageDays: [] });
  }
  for (const day of summary.days) {
    years.get(period("year", day.day).start)?.totals.push(day);
  }
  for (const usage of summary.accountUsage.days) {
    const day = Date.parse(`${usage.startDate}T00:00:00+09:00`);
    years.get(period("year", day).start)?.usageDays.push(usage);
  }
  return Array.from(years);
}

export function YearCalendar({
  summary,
  anchor,
  jump,
  latest,
  visible,
  drillDown,
}: {
  summary: DashboardSummary;
  anchor: number;
  jump?: YearJump;
  latest: number;
  visible: (year: number) => void;
  drillDown: (target: View, day: number) => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const sections = useRef(new Map<number, HTMLElement>());
  const initialAnchor = useRef(period("year", anchor).start).current;
  const mounted = useRef(false);
  const reducedMotion = useReducedMotion();
  const earliest = period("year", summary.history.start).start;
  const years = useMemo(() => splitYears(summary, earliest, latest), [summary, earliest, latest]);
  useLayoutEffect(() => {
    const viewport = root.current;
    const start = Math.max(earliest, Math.min(latest, jump?.start ?? initialAnchor));
    const target = sections.current.get(start);
    if (!viewport || !target) return;
    const first = !mounted.current;
    const place = () => {
      viewport.scrollTo({
        top:
          viewport.scrollTop +
          target.getBoundingClientRect().top -
          viewport.getBoundingClientRect().top,
        behavior: first || reducedMotion ? "instant" : "smooth",
      });
      mounted.current = true;
    };
    const frame = requestAnimationFrame(place);
    const stack = viewport.firstElementChild;
    if (!first || start === earliest || !stack) return () => cancelAnimationFrame(frame);
    const observer = new ResizeObserver(place);
    observer.observe(stack);
    const timer = window.setTimeout(() => observer.disconnect(), 250);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(timer);
      observer.disconnect();
    };
  }, [earliest, latest, initialAnchor, jump, reducedMotion]);
  return (
    <Box
      ref={root}
      h="100%"
      style={{ overflow: "auto" }}
      aria-label="年別カレンダー"
      onScroll={(event) => {
        const edge = event.currentTarget.getBoundingClientRect().top + 12;
        let year = earliest;
        for (const [start] of years) {
          const section = sections.current.get(start);
          if (section && section.getBoundingClientRect().top <= edge) year = start;
        }
        visible(year);
      }}
    >
      <Stack gap="lg">
        {years.map(([year, data]) => (
          <Box
            key={year}
            component="section"
            aria-label={yearName.format(year)}
            ref={(node) => {
              if (node) sections.current.set(year, node);
              else sections.current.delete(year);
            }}
          >
            <Title order={2} size="h5" p="sm">
              {yearName.format(year)}
            </Title>
            <Calendar
              view="year"
              start={year}
              blocks={[]}
              totals={data.totals}
              usageDays={data.usageDays}
              drillDown={drillDown}
            />
          </Box>
        ))}
      </Stack>
    </Box>
  );
}
