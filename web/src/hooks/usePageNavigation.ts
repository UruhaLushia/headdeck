import { useEffect, useState } from "react";
import { pages } from "../components/Layout";
import { getPage, loadPage } from "../lib/pages";
import { usePagePreload } from "./usePagePreload";

function resolvePage(permissions: string[]) {
  if (
    window.location.hash === "#/personal" ||
    new URLSearchParams(location.search).has("account_security")
  )
    return { page: "personal" as const, nodeID: undefined };
  const path = window.location.hash.slice(2);
  const requested = path.startsWith("nodes/") ? "nodes" : path;
  const page =
    pages.find((item) => item.id === requested && permissions.includes(item.permission))?.id ??
    pages.find((item) => permissions.includes(item.permission))?.id ??
    "overview";
  return {
    page,
    nodeID: page === "nodes" && path.startsWith("nodes/") ? path.slice(6) : undefined,
  };
}

export function usePageNavigation(permissions: string[]) {
  const preload = usePagePreload(permissions);
  const permissionKey = JSON.stringify(permissions);
  const [view, setView] = useState(() => resolvePage(permissions));
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const allowed: string[] = JSON.parse(permissionKey);
    let generation = 0;
    async function change() {
      const request = ++generation;
      const next = resolvePage(allowed);
      setError(undefined);
      try {
        if (!getPage(next.page)) await loadPage(next.page);
        if (request !== generation) return;
        setView(next);
      } catch (reason) {
        if (request === generation)
          setError(reason instanceof Error ? reason.message : "Request failed");
      }
    }
    void change();
    window.addEventListener("hashchange", change);
    return () => {
      generation++;
      window.removeEventListener("hashchange", change);
    };
  }, [permissionKey, attempt]);
  return {
    page: view.page,
    nodeID: view.nodeID,
    Component: getPage(view.page),
    error,
    retry: () => setAttempt((value) => value + 1),
    preload,
  };
}
