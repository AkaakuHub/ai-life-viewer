import { useCallback, useEffect, useRef, useState } from "react";
import { dayStart } from "../../shared/timeline.ts";
import { restoreScreen, screenParams, type Screen } from "./screen.ts";

const storageKey = "ai-life-viewer.screen";

function read() {
  let saved: string | null = null;
  try {
    saved = window.localStorage.getItem(storageKey);
  } catch {
    // The URL remains available when storage is blocked.
  }
  return restoreScreen(new URLSearchParams(window.location.search), saved, Date.now());
}

function write(screen: Screen, mode: "push" | "replace") {
  const url = new URL(window.location.href);
  url.search = screenParams(screen, url.searchParams).toString();
  if (mode === "push") window.history.pushState(null, "", url);
  else window.history.replaceState(null, "", url);
  try {
    window.localStorage.setItem(storageKey, screenParams(screen).toString());
  } catch {
    // The URL remains available when storage is blocked.
  }
}

export function useScreen(onPop: (screen: Screen) => void) {
  const [screen, setScreen] = useState(read);
  const current = useRef(screen);
  useEffect(() => {
    write(current.current, "replace");
    const pop = () => {
      const next = read();
      current.current = next;
      setScreen(next);
      write(next, "replace");
      onPop(next);
    };
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, [onPop]);

  const navigate = useCallback((update: Partial<Screen>, mode: "push" | "replace" = "push") => {
    const next = { ...current.current, ...update };
    next.anchor = dayStart(next.anchor);
    if (screenParams(next).toString() === screenParams(current.current).toString()) return;
    write(next, mode);
    current.current = next;
    setScreen(next);
  }, []);
  return [screen, navigate] as const;
}
