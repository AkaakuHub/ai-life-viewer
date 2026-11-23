import { useRef, useState, type PointerEvent, type ReactNode } from "react";
import { swipeDirection } from "./gestures.ts";

type Start = { id: number; x: number; y: number };

export function CalendarRegion({
  week,
  move,
  children,
}: {
  week: boolean;
  move: (direction: number) => void;
  children: ReactNode;
}) {
  const start = useRef<Start>(null);
  const [direction, setDirection] = useState(0);

  function reset() {
    start.current = null;
    setDirection(0);
  }

  function begin(event: PointerEvent<HTMLDivElement>) {
    if (event.pointerType !== "touch" || !event.isPrimary) {
      if (event.pointerType === "touch") reset();
      return;
    }
    if (week && !(event.target instanceof Element && event.target.closest(".calendar-head")))
      return;
    start.current = { id: event.pointerId, x: event.clientX, y: event.clientY };
  }

  function drag(event: PointerEvent<HTMLDivElement>) {
    const origin = start.current;
    if (!origin || event.pointerId !== origin.id) return;
    const next = swipeDirection(event.clientX - origin.x, event.clientY - origin.y, 16);
    if (next && !event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
    setDirection(next);
  }

  function finish(event: PointerEvent<HTMLDivElement>) {
    const origin = start.current;
    if (!origin || event.pointerId !== origin.id) return;
    const next = swipeDirection(event.clientX - origin.x, event.clientY - origin.y, 55);
    reset();
    if (next) move(next);
  }

  return (
    <div
      className="calendar-region"
      data-week={week || undefined}
      data-swipe={direction === -1 ? "previous" : direction === 1 ? "next" : undefined}
      onPointerDown={begin}
      onPointerMove={drag}
      onPointerUp={finish}
      onPointerCancel={reset}
      onLostPointerCapture={(event) => {
        if (event.target === event.currentTarget) reset();
      }}
    >
      {children}
    </div>
  );
}
