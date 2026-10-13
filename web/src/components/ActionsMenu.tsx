import { Dropdown } from "@heroui/react";
import { Ellipsis } from "lucide-react";
import { t } from "../lib/i18n";
import type { ReactNode } from "react";

export function ActionsMenu({
  label,
  actions,
  children,
}: {
  label: string;
  actions: { label: string; onSelect: () => void; danger?: boolean }[];
  children?: ReactNode;
}) {
  if (!actions.length) return null;
  return (
    <Dropdown>
      <Dropdown.Trigger
        className={
          children
            ? "inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-xl bg-default px-3 text-sm font-medium hover:bg-surface-secondary focus-visible:outline-2 focus-visible:outline-accent"
            : "rounded-xl p-2 text-muted hover:bg-default"
        }
        aria-label={label}
      >
        {children ?? <Ellipsis size={18} />}
      </Dropdown.Trigger>
      <Dropdown.Popover className="min-w-40 rounded-xl">
        <Dropdown.Menu onAction={(key) => actions[Number(key)]?.onSelect()}>
          {actions.map((action, index) => (
            <Dropdown.Item
              id={String(index)}
              key={action.label}
              textValue={t(action.label)}
              className={action.danger ? "text-danger" : ""}
            >
              {t(action.label)}
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}
