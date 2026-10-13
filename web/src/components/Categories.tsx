import { useRef } from "react";
import type { KeyboardEvent, ReactNode } from "react";
import { t } from "../lib/i18n";

interface Category {
  id: string;
  label: string;
  count?: number;
}

export function CategoryLayout({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-w-0 max-w-275 items-start gap-5 md:grid-cols-[180px_minmax(0,1fr)]">
      {children}
    </div>
  );
}

export function CategoryNav({
  label,
  groups,
  selected,
  onSelect,
}: {
  label: string;
  groups: { label?: string; items: Category[] }[];
  selected: string;
  onSelect: (id: string) => void;
}) {
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const items = groups.flatMap((group) => group.items);
  function move(event: KeyboardEvent<HTMLButtonElement>, id: string) {
    const index = items.findIndex((item) => item.id === id);
    let next: number;
    switch (event.key) {
      case "ArrowDown":
      case "ArrowRight":
        next = (index + 1) % items.length;
        break;
      case "ArrowUp":
      case "ArrowLeft":
        next = (index + items.length - 1) % items.length;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = items.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    onSelect(items[next].id);
    const button = buttons.current.get(items[next].id);
    button?.focus();
    button?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  return (
    <nav className="flex min-w-0 gap-1 overflow-x-auto md:block md:space-y-4" aria-label={t(label)}>
      {groups
        .filter((group) => group.items.length)
        .map((group, index) => (
          <div key={group.label ?? index} className="contents md:block">
            {group.label && (
              <p className="mb-1 hidden px-3 text-xs text-muted md:block">{t(group.label)}</p>
            )}
            <div className="contents md:block md:space-y-1">
              {group.items.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  ref={(button) => {
                    if (button) buttons.current.set(item.id, button);
                    else buttons.current.delete(item.id);
                  }}
                  aria-pressed={selected === item.id}
                  onClick={() => onSelect(item.id)}
                  onKeyDown={(event) => move(event, item.id)}
                  className={`flex shrink-0 items-center justify-between gap-2 rounded-lg px-3 py-2.5 text-left text-sm transition-colors motion-reduce:transition-none focus-visible:outline-2 focus-visible:outline-accent md:w-full ${selected === item.id ? "bg-accent-soft font-medium text-accent" : "text-muted hover:bg-default hover:text-foreground"}`}
                >
                  <span className="whitespace-nowrap">{t(item.label)}</span>
                  {!!item.count && (
                    <span className="text-xs tabular-nums opacity-70">{item.count}</span>
                  )}
                </button>
              ))}
            </div>
          </div>
        ))}
    </nav>
  );
}

export function CategoryPanel({
  title,
  actions,
  children,
}: {
  title: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={t(title)}
      className="min-w-0 rounded-2xl border border-border bg-surface p-2 sm:p-3"
    >
      <div className="flex items-center justify-between gap-3 px-3 pt-2 pb-3">
        <h2 className="text-base font-semibold">{t(title)}</h2>
        {actions}
      </div>
      {children}
    </section>
  );
}
