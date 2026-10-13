import { useSyncExternalStore } from "react";
import zhCN from "../locales/zh-CN.json";

export type Locale = "en" | "zh-CN";
const listeners = new Set<() => void>();
function initialLocale(): Locale {
  try {
    const saved = localStorage.getItem("headdeck.locale");
    if (saved === "en" || saved === "zh-CN") return saved;
  } catch {}
  return navigator.language.startsWith("zh") ? "zh-CN" : "en";
}
let locale = initialLocale();
document.documentElement.lang = locale;

export function setLocale(next: Locale) {
  if (next === locale) return;
  locale = next;
  document.documentElement.lang = next;
  try {
    localStorage.setItem("headdeck.locale", next);
  } catch {}
  for (const listener of listeners) listener();
}

export function useLocale() {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    () => locale,
  );
}

export function t(key: string, ...values: (string | number)[]) {
  const messages: Record<string, string> = zhCN;
  const message = locale === "zh-CN" ? (messages[key] ?? key) : key;
  return message.replace(/\{(\d+)\}/g, (match, index) => String(values[Number(index)] ?? match));
}
