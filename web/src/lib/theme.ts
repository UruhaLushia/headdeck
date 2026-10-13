import { useSyncExternalStore } from "react";

export type Theme = "light" | "dark" | "auto";
const storageKey = "headdeck.theme";
const system = window.matchMedia("(prefers-color-scheme: dark)");
const listeners = new Set<() => void>();

function parse(value: string | null): Theme {
  return value === "light" || value === "dark" ? value : "auto";
}

function initial(): Theme {
  try {
    return parse(localStorage.getItem(storageKey));
  } catch {
    return "auto";
  }
}

let theme = initial();
function apply() {
  document.documentElement.dataset.theme =
    theme === "auto" ? (system.matches ? "dark" : "light") : theme;
}
apply();

function update(next: Theme) {
  theme = next;
  apply();
  for (const listener of listeners) listener();
}

function storage(event: StorageEvent) {
  if (event.key === storageKey || event.key === null) update(parse(event.newValue));
}

function subscribe(listener: () => void) {
  if (!listeners.size) {
    system.addEventListener("change", apply);
    window.addEventListener("storage", storage);
    apply();
  }
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (!listeners.size) {
      system.removeEventListener("change", apply);
      window.removeEventListener("storage", storage);
    }
  };
}

export function setTheme(next: Theme) {
  try {
    localStorage.setItem(storageKey, next);
  } catch {}
  update(next);
}

export function useTheme() {
  return useSyncExternalStore(subscribe, () => theme);
}
