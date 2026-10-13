import { Button } from "@heroui/react";
import { Plus, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { CategoryPanel } from "../../components/Categories";
import { t } from "../../lib/i18n";
import type { DNSConfig } from "../../lib/types";

export function DNSLists({
  config,
  section,
  writable,
  editButton,
  split,
  record,
  remove,
}: {
  config: DNSConfig;
  section: "split" | "records";
  writable: boolean;
  editButton: (label: string, action: () => void) => ReactNode;
  split: (domain?: string) => void;
  record: (index?: number) => void;
  remove: (title: string, edit: (draft: DNSConfig) => void) => void;
}) {
  const isSplit = section === "split";
  const entries = isSplit
    ? Object.entries(config.nameservers.split).map(([domain, servers]) => ({
        name: domain,
        value: servers.join(" · "),
      }))
    : config.extra_records.map((entry) => ({
        name: entry.name,
        value: `${entry.type} · ${entry.value}`,
      }));
  const edit = (index: number) => (isSplit ? split(entries[index].name) : record(index));
  return (
    <CategoryPanel
      title={isSplit ? "Split DNS" : "DNS records"}
      actions={
        writable && (
          <Button size="sm" variant="secondary" onPress={() => (isSplit ? split() : record())}>
            <Plus size={15} />
            {t(isSplit ? "Add" : "Add record")}
          </Button>
        )
      }
    >
      <div className="space-y-1">
        {entries.map((entry, index) => (
          <div
            key={`${entry.name}:${entry.value}`}
            className="flex items-center gap-3 rounded-lg px-3 py-3 transition-colors hover:bg-surface-secondary"
          >
            <button
              type="button"
              disabled={!writable}
              onClick={() => edit(index)}
              className="min-w-0 flex-1 rounded text-left focus-visible:outline-2 focus-visible:outline-accent disabled:cursor-default"
            >
              <span className="block wrap-break-word text-sm font-medium">{entry.name}</span>
              <span className="mt-1 block wrap-break-word text-xs text-muted">{entry.value}</span>
            </button>
            {writable && (
              <div className="flex shrink-0">
                {editButton(t("Edit {0}", entry.name), () => edit(index))}
                <Button
                  size="sm"
                  variant="tertiary"
                  isIconOnly
                  aria-label={t("Delete {0}", entry.name)}
                  onPress={() =>
                    remove(entry.name, (draft) => {
                      if (isSplit) delete draft.nameservers.split[entry.name];
                      else draft.extra_records.splice(index, 1);
                    })
                  }
                >
                  <Trash2 size={15} />
                </Button>
              </div>
            )}
          </div>
        ))}
        {!entries.length && (
          <p className="px-3 py-8 text-center text-sm text-muted">
            {t(isSplit ? "No split DNS rules." : "No custom records.")}
          </p>
        )}
      </div>
    </CategoryPanel>
  );
}
