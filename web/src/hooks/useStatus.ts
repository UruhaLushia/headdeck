import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { useResourceCache } from "./ResourceProvider";
import { api } from "../lib/api";
import { t } from "../lib/i18n";
import { appURL } from "../lib/url";
import { advanceUptime } from "../lib/uptime";
import type { Session, Status } from "../lib/types";

function onOverview() {
  return window.location.hash === "#/overview" || window.location.hash === "";
}

export function useStatus() {
  const cache = useResourceCache();
  if (!cache) throw new Error("Status requires a resource cache");
  const data = useSyncExternalStore(cache.subscribe, () => cache.get<Status>("/api/status"));
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => {
    let source: EventSource | undefined;
    let checking: AbortController | undefined;
    function disconnect() {
      checking?.abort();
      checking = undefined;
      source?.close();
      source = undefined;
    }
    const receive = (event: MessageEvent) => {
      try {
        const next = JSON.parse(event.data) as Status | Partial<Status>;
        if (event.type === "snapshot") cache!.set("/api/status", next as Status);
        else {
          const previous = cache!.get<Status>("/api/status");
          if (previous) cache!.set("/api/status", { ...previous, ...next });
        }
        setError(undefined);
      } catch {
        setError(t("Invalid status update"));
      }
    };
    function connect() {
      disconnect();
      if (!onOverview() || document.hidden) return;
      const current = new EventSource(appURL("/api/status/stream"));
      source = current;
      current.addEventListener("snapshot", receive);
      current.addEventListener("diff", receive);
      current.onerror = async () => {
        if (checking || source !== current) return;
        setError(t("Connection lost. Reconnecting…"));
        const controller = new AbortController();
        checking = controller;
        try {
          const session = await api.get<Session>("/api/session", controller.signal);
          if (!controller.signal.aborted && source === current) {
            if (!session.authenticated || !session.permissions?.includes("system.read"))
              disconnect();
            window.dispatchEvent(new CustomEvent("headdeck:session", { detail: session }));
          }
        } catch {
        } finally {
          if (checking === controller) checking = undefined;
        }
      };
    }
    connect();
    document.addEventListener("visibilitychange", connect);
    window.addEventListener("hashchange", connect);
    return () => {
      disconnect();
      document.removeEventListener("visibilitychange", connect);
      window.removeEventListener("hashchange", connect);
    };
  }, [cache, attempt]);
  const refresh = useCallback(async () => setAttempt((value) => value + 1), []);
  const elapsed = (now - cache.updatedAt("/api/status")) / 1000;
  const state = data?.service_control.state;
  const status = data && {
    ...data,
    server: { ...data.server, uptime: advanceUptime(data.server.uptime, elapsed)! },
    service_control: {
      ...data.service_control,
      state: state && {
        ...state,
        uptime:
          state.active_state === "active" ? advanceUptime(state.uptime, elapsed) : state.uptime,
      },
    },
  };
  return { data: status, error, refresh };
}
