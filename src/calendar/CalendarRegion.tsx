import { Box, Loader, LoadingOverlay } from "@mantine/core";
import { useRef, useState, type PointerEvent, type ReactNode } from "react";
import { swipeDirection } from "./gestures.ts";

type Start = { id: number; x: number; y: number };

export function CalendarRegion({
  week,
  move,
  loading,
  children,
}: {
  week: boolean;
  move: (direction: number) => void;
  loading: boolean;
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
    if (week && !(event.target instanceof Element && event.target.closest(".week-swipe-header")))
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
      <Box h="100%" aria-busy={loading} inert={loading}>
        {children}
      </Box>
      <LoadingOverlay
        visible={loading}
        zIndex={5}
        overlayProps={{ backgroundOpacity: 0.24 }}
        loaderProps={{
          children: <Loader size="sm" role="status" aria-label="カレンダーを読み込み中" />,
        }}
      />
    </div>
  );
}
