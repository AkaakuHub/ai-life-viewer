import { DAY, dayStart, daysIn, type Block, type DayTotal } from "../../shared/timeline.ts";
import { date, dayNumber, hours, monthNumber, weekday } from "./format.ts";
import { plainText } from "./markdown.ts";
import { monthTotals, movePeriod, weekStart } from "./period.ts";
import "./overview.css";

export function MonthGrid({
  start,
  end,
  now,
  days,
  blocks,
  selectDay,
}: {
  start: number;
  end: number;
  now: number;
  days: DayTotal[];
  blocks: Block[];
  selectDay: (day: number) => void;
}) {
  const first = weekStart(start);
  const calendarDays = daysIn(first, weekStart(end - 1) + 7 * DAY);
  const totals = new Map(days.map(({ day, ms }) => [day, ms]));
  const max = Math.max(1, ...totals.values());
  return (
    <section className="overview" aria-label="月間稼働時間">
      <div className="month-grid">
        {calendarDays.slice(0, 7).map((day) => (
          <div key={day} className="month-weekday" data-weekday={weekday.format(day)}>
            {weekday.format(day)}
          </div>
        ))}
        {calendarDays.map((day) => {
          if (day < start || day >= end) return <div className="empty-day" key={day} />;
          const ms = totals.get(day) ?? 0;
          let primary: Block | undefined;
          let longest = 0;
          for (const block of blocks) {
            const overlap = Math.min(day + DAY, block.end) - Math.max(day, block.start);
            if (overlap <= longest) continue;
            primary = block;
            longest = overlap;
          }
          const title = primary && (plainText(primary.title) || primary.project);
          const today = day === dayStart(now);
          return (
            <button
              key={day}
              type="button"
              className="month-day"
              aria-current={today ? "date" : undefined}
              aria-label={`${date.format(day)} ${hours(ms)}${title ? ` ${title}` : ""}`}
              onClick={() => selectDay(day)}
              style={{
                backgroundColor: `color-mix(in srgb, #4d69aa ${(ms / max) * 65}%, var(--surface))`,
              }}
            >
              <span className="month-date">{dayNumber.format(day)}</span>
              <span className="overview-hours">{hours(ms)}</span>
              {title && <span className="month-session">{title}</span>}
            </button>
          );
        })}
      </div>
    </section>
  );
}

export function YearGrid({
  start,
  now,
  days,
  selectMonth,
}: {
  start: number;
  now: number;
  days: DayTotal[];
  selectMonth: (day: number) => void;
}) {
  const totals = monthTotals(start, days);
  const max = Math.max(1, ...totals.map(({ ms }) => ms));
  return (
    <section className="overview" aria-label="年間稼働時間">
      <div className="year-grid">
        {totals.map(({ day, ms }) => (
          <button
            key={day}
            type="button"
            className="year-month"
            onClick={() => selectMonth(day)}
            aria-current={day === movePeriod("month", now, 0) ? "date" : undefined}
          >
            <span>{monthNumber.format(day)}</span>
            <strong className="overview-hours">{hours(ms)}</strong>
            <span className="bar-track" aria-hidden="true">
              <span style={{ width: `${(ms / max) * 100}%` }} />
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}
