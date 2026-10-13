import { useCallback, useEffect } from "react";
import { pages } from "../components/Layout";
import type { PageID } from "../components/Layout";
import { loadPage } from "../lib/pages";

export function usePagePreload(permissions: string[]) {
  const permissionKey = JSON.stringify(permissions);
  const preload = useCallback(
    async (page: PageID) => {
      const allowed: string[] = JSON.parse(permissionKey);
      if (
        page !== "personal" &&
        !pages.some((p) => p.id === page && allowed.includes(p.permission))
      )
        return;
      await Promise.allSettled([loadPage(page)]);
    },
    [permissionKey],
  );
  useEffect(() => {
    const allowed: string[] = JSON.parse(permissionKey);
    const queue: PageID[] = [
      ...pages.filter((p) => allowed.includes(p.permission)).map((p) => p.id),
      "personal",
    ];
    let cancelled = false;
    let idle: number | undefined;
    let timer: number | undefined;
    function next() {
      if (cancelled) return;
      const page = queue.shift();
      if (page) void preload(page).finally(schedule);
    }
    function schedule() {
      if (cancelled || !queue.length) return;
      if (window.requestIdleCallback) idle = window.requestIdleCallback(next, { timeout: 1500 });
      else timer = window.setTimeout(next, 100);
    }
    timer = window.setTimeout(schedule, 300);
    return () => {
      cancelled = true;
      if (idle !== undefined) window.cancelIdleCallback(idle);
      window.clearTimeout(timer);
    };
  }, [permissionKey, preload]);
  return preload;
}
