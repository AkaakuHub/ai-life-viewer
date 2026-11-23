import { useEffect, useState } from "react";

export function useResource<T>(url: string | null) {
  const [result, setResult] = useState<{ url: string; data?: T; error?: boolean }>();
  useEffect(() => {
    if (!url) return;
    const requestUrl = url;
    const controller = new AbortController();
    let timer: number | undefined;
    async function refresh() {
      try {
        const response = await fetch(requestUrl, { signal: controller.signal });
        if (!response.ok) throw new Error(String(response.status));
        const data: T = await response.json();
        if (!controller.signal.aborted) setResult({ url: requestUrl, data });
      } catch {
        if (!controller.signal.aborted) setResult({ url: requestUrl, error: true });
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(() => void refresh(), 10_000);
      }
    }
    void refresh();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [url]);
  const current = result?.url === url ? result : undefined;
  const loading = url !== null && current === undefined;
  return {
    data: current?.data,
    error: current?.error,
    loading,
    previousData: loading ? result?.data : undefined,
  };
}
