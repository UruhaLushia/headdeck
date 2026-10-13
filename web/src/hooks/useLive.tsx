import { createContext, useContext, useEffect, useEffectEvent, useState } from "react";
import type { ReactNode } from "react";
import type { LiveEvent } from "../lib/types";
import type { Session } from "../lib/types";
import { api } from "../lib/api";
import { appURL } from "../lib/url";

type Listener = (event: LiveEvent) => void;
const LiveContext = createContext<Set<Listener> | null>(null);

export function LiveProvider({
  children,
  enabled = true,
}: {
  children: ReactNode;
  enabled?: boolean;
}) {
  const [listeners] = useState(() => new Set<Listener>());
  useEffect(() => {
    if (!enabled) {
      return;
    }
    const source = new EventSource(appURL("/api/events"));
    const controller = new AbortController();
    let checking = false;
    let connected = false;
    source.onerror = async () => {
      if (checking || controller.signal.aborted) return;
      checking = true;
      try {
        const session = await api.get<Session>("/api/session", controller.signal);
        if (!controller.signal.aborted) {
          if (!session.authenticated) source.close();
          window.dispatchEvent(new CustomEvent("headdeck:session", { detail: session }));
        }
      } catch {
        // EventSource retries transient connection failures.
      } finally {
        checking = false;
      }
    };
    const receive = (message: MessageEvent) => {
      try {
        if (message.type === "hello" && !connected) {
          connected = true;
          return;
        }
        const event = {
          ...JSON.parse(message.data),
          type: message.type,
        } as LiveEvent;
        for (const listener of listeners) listener(event);
      } catch {
        for (const listener of listeners) listener({ id: 0, type: "resync", revision: 0 });
      }
    };
    for (const type of [
      "hello",
      "resync",
      "nodes.changed",
      "users.changed",
      "routes.changed",
      "keys.changed",
      "operations.changed",
      "dns.changed",
      "policy.changed",
      "config.changed",
      "accounts.changed",
      "providers.changed",
      "headscale.recovered",
    ])
      source.addEventListener(type, receive);
    return () => {
      controller.abort();
      source.close();
    };
  }, [enabled, listeners]);
  return <LiveContext value={listeners}>{children}</LiveContext>;
}

export function useLive(listener: Listener) {
  const listeners = useContext(LiveContext);
  const callback = useEffectEvent(listener);
  useEffect(() => {
    const receive: Listener = (event) => callback(event);
    listeners?.add(receive);
    return () => {
      listeners?.delete(receive);
    };
  }, [listeners]);
}
