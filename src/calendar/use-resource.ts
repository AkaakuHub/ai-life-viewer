import { useEffect, useState } from "react";

export function useResource<T>(url: string | null) {
  const [result, setResult] = useState<{ url: string; data?: T; error?: boolean }>();
  useEffect(() => {
    if (!url) return;
    const controller = new AbortController();
    async function refresh() {
      if (!url) return;
      try {
        const response = await fetch(url, { signal: controller.signal });
        if (!response.ok) throw new Error(String(response.status));
        const data: T = await response.json();
        if (!controller.signal.aborted) setResult({ url, data });
      } catch {
        if (!controller.signal.aborted) setResult({ url, error: true });
      }
    }
    void refresh();
    const interval = window.setInterval(() => void refresh(), 10_000);
    return () => {
      controller.abort();
      window.clearInterval(interval);
    };
  }, [url]);
  return result?.url === url ? result : undefined;
}
