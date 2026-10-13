import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import type { SetStateAction } from "react";
import { createResourceCache, useResourceCache } from "./ResourceProvider";
import { useLive } from "./useLive";

const resourceEvents: Record<string, string> = {
  "/api/preauth-keys": "keys.changed",
  "/api/routes": "routes.changed",
  "/api/dns": "dns.changed",
  "/api/policy/objects": "policy.changed",
  "/api/headscale/config": "config.changed",
  "/api/accounts": "accounts.changed",
  "/api/login-providers": "providers.changed",
};

export function useResource<T>(path: string | null) {
  const shared = useResourceCache();
  const [local] = useState(createResourceCache);
  const cache = shared ?? local;
  const data = useSyncExternalStore(cache.subscribe, () => (path ? cache.get<T>(path) : undefined));
  const setData = useCallback(
    (value: SetStateAction<T | undefined>) => {
      if (!path) return;
      cache.set(
        path,
        typeof value === "function"
          ? (value as (current: T | undefined) => T | undefined)(cache.get<T>(path))
          : value,
      );
    },
    [cache, path],
  );
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);
  const mounted = useRef(false);
  const generation = useRef(0);
  const fetch = useCallback(
    async (revalidate = false) => {
      if (!mounted.current) return;
      if (!path) {
        setLoading(false);
        return;
      }
      const current = generation.current;
      setLoading(cache.get(path) === undefined);
      try {
        await cache.fetch(path, revalidate);
        if (mounted.current && generation.current === current) {
          setError(undefined);
        }
      } catch (reason) {
        if (mounted.current && generation.current === current) {
          setError(reason instanceof Error ? reason.message : "Request failed");
        }
      } finally {
        if (mounted.current && generation.current === current) {
          setLoading(false);
        }
      }
    },
    [cache, path],
  );
  const refresh = useCallback(() => fetch(true), [fetch]);
  useLive((event) => {
    if (!path) return;
    if (
      ["hello", "resync", "headscale.recovered"].includes(event.type) ||
      event.type === resourceEvents[path]
    )
      void refresh();
  });
  useEffect(() => {
    mounted.current = true;
    setError(undefined);
    setLoading(false);
    if (path) void fetch();
    window.addEventListener("headdeck:refresh", refresh);
    return () => {
      mounted.current = false;
      generation.current = generation.current + 1;
      window.removeEventListener("headdeck:refresh", refresh);
    };
  }, [fetch, path, refresh]);
  useEffect(() => () => local.cancel(), [local]);
  return { data, setData, error, loading, refresh };
}
