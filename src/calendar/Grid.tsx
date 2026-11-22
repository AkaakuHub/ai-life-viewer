import { useLayoutEffect, useMemo, useRef, type CSSProperties } from "react";
import { DAY, HOUR, dayStart, daysIn, type Block, type DayTotal } from "../../shared/timeline.ts";
import { blockColor, clock, date, hours, weekday } from "./format.ts";
import { partition } from "./layout.ts";
import { plainText } from "./markdown.ts";

const timeLabels = Array.from({ length: 24 }, (_, hour) => `${String(hour).padStart(2, "0")}:00`);

export function Grid({
  start,
  end,
  blocks,
  totals,
  now,
  onSelect,
}: {
  start: number;
  end: number;
  blocks: Block[];
  totals: DayTotal[];
  now: number;
  onSelect: (block: Block) => void;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const body = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (scroller.current && body.current) {
      scroller.current.scrollTop = (body.current.offsetHeight * 8) / 24;
    }
  }, []);
  const days = daysIn(start, end);
  const today = dayStart(now);
  const titles = useMemo(
    () =>
      new Map(
        [...new Set(blocks.map(({ title }) => title))].map((title) => [title, plainText(title)]),
      ),
    [blocks],
  );
  return (
    <div className="calendar-scroll" ref={scroller}>
      <section
        aria-label="稼働時間カレンダー"
        className="calendar"
        style={{ "--days": days.length } as CSSProperties}
      >
        <div className="calendar-head">
          <div className="axis-head">JST</div>
          {days.map((day) => {
            const label = weekday.format(day);
            return (
              <div className="day-head" key={day} data-today={day === today}>
                <span className="day-date">
                  <span className="weekday" data-day={label}>
                    {label}
                  </span>
                  <span className="date">{date.format(day)}</span>
                </span>
                <span className="day-hours">
                  {hours(totals.find((total) => total.day === day)?.ms ?? 0)}
                </span>
              </div>
            );
          })}
        </div>
        <div className="calendar-body" ref={body}>
          <div className="axis">
            {timeLabels.map((label) => (
              <div key={label}>{label}</div>
            ))}
          </div>
          {days.map((day) => (
            <div className="day-column" key={day} data-today={day === today}>
              {partition(blocks, day).map(({ block, start: from, end: to, lane, lanes }) => {
                const title = titles.get(block.title) || block.project;
                return (
                  <button
                    key={block.id}
                    type="button"
                    className="block"
                    aria-label={`${block.agent} ${clock.format(block.start)} ${title}`}
                    onClick={() => onSelect(block)}
                    style={
                      {
                        "--accent": blockColor(block.agent, block.project),
                        top: `${((from - day) / DAY) * 100}%`,
                        height: `${((to - from) / DAY) * 100}%`,
                        left: `${(lane / lanes) * 100}%`,
                        width: `${100 / lanes}%`,
                      } as CSSProperties
                    }
                  >
                    <span className="block-label">
                      <span className="block-time">
                        {block.agent} · {clock.format(block.start)}
                      </span>{" "}
                      {title}
                    </span>
                  </button>
                );
              })}
              {today === day && (
                <div
                  className="now-line"
                  style={{ top: `${((now - day) / (24 * HOUR)) * 100}%` }}
                />
              )}
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
