import { ArrowRight, Pencil } from "lucide-react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { t } from "../../lib/i18n";
import type { PolicyObjects } from "../../lib/types";
import { policyTitle } from "./PolicySections";
import type { PolicyOption } from "./policyOptions";
import { aclDestination, optionLabel } from "./policyOptions";

type Entry = PolicyObjects["items"][number];

function Values({ entry, name, options }: { entry: Entry; name: string; options: PolicyOption[] }) {
  const values = entry.fields.find((field) => field.name === name)?.values.filter(Boolean) ?? [];
  if (!values.length) return <span className="text-muted">—</span>;
  return (
    <span className="flex flex-wrap gap-x-2 gap-y-1 font-medium">
      {values.map((value, index) => (
        <span key={`${value}:${index}`} className="min-w-0 wrap-anywhere">
          {displayValue(entry, name, value, options)}
          {index < values.length - 1 && <span className="ml-2 text-muted">·</span>}
        </span>
      ))}
    </span>
  );
}

function displayValue(entry: Entry, name: string, value: string, options: PolicyOption[]) {
  if (name === "dst" && entry.item.section === "acls") {
    const { target, ports } = aclDestination(value);
    return `${optionLabel(options, target)} · ${ports === "*" ? t("All ports") : t("Ports: {0}", ports)}`;
  }
  if (name === "ip") {
    const [protocol, ports] = value.includes(":") ? value.split(":") : ["", value];
    return `${protocol ? protocol.toUpperCase() : t("Any protocol")} · ${ports === "*" ? t("All ports") : ports}`;
  }
  if (name === "users" && value === "autogroup:nonroot") return t("Any non-root user");
  return optionLabel(options, value);
}

export function PolicyList({
  section,
  entries,
  writable,
  edit,
  remove,
  options,
}: {
  section: string;
  entries: Entry[];
  writable: boolean;
  edit: (entry: Entry) => void;
  remove: (entry: Entry) => void;
  options: PolicyOption[];
}) {
  const rules = ["acls", "grants", "ssh"].includes(section);
  const labels: Record<string, string> = {
    groups: "Members",
    tagOwners: "Owners",
    hosts: "IP address or prefix",
  };
  if (!entries.length)
    return <p className="px-3 py-8 text-center text-sm text-muted">{t("No entries yet.")}</p>;
  return (
    <div className="space-y-1">
      {entries.map((entry) => {
        const title = policyTitle(entry.item);
        const detail = entry.fields.find(
          (field) => field.name === (section === "ssh" ? "action" : "proto"),
        )?.values[0];
        const extra = section === "grants" ? "ip" : section === "ssh" ? "users" : undefined;
        const content = (
          <>
            <span className="mb-2 flex items-center gap-2 font-medium">
              {title}
              {detail && <span className="text-xs font-normal text-muted">{t(detail)}</span>}
              {writable && (
                <Pencil size={14} className="ml-auto shrink-0 text-muted group-hover:text-accent" />
              )}
            </span>
            {rules ? (
              <span className="grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_16px_minmax(0,1fr)]">
                <span className="min-w-0">
                  <span className="mb-1 block text-xs text-muted">{t("Sources")}</span>
                  <Values entry={entry} name="src" options={options} />
                </span>
                <ArrowRight size={16} className="mt-6 hidden text-muted sm:block" />
                <span className="min-w-0">
                  <span className="mb-1 block text-xs text-muted">{t("Destinations")}</span>
                  <Values entry={entry} name="dst" options={options} />
                </span>
              </span>
            ) : (
              <span className="grid grid-cols-[minmax(0,0.85fr)_minmax(0,1fr)] gap-3">
                <span className="text-muted">{t(labels[section] ?? "Approvers")}</span>
                <Values entry={entry} name="value" options={options} />
              </span>
            )}
            {extra && (
              <span className="mt-3 block">
                <span className="mb-1 block text-xs text-muted">
                  {t(extra === "ip" ? "IP permissions" : "SSH users")}
                </span>
                <Values entry={entry} name={extra} options={options} />
              </span>
            )}
          </>
        );
        const style =
          "group block min-w-0 flex-1 rounded-lg px-3 py-3 text-left text-sm transition-colors hover:bg-surface-secondary";
        return (
          <article
            key={`${entry.item.key}:${entry.item.index}`}
            aria-label={title}
            className="flex items-start gap-1"
          >
            {writable ? (
              <button
                type="button"
                className={`${style} focus-visible:outline-2 focus-visible:outline-accent`}
                onClick={() => edit(entry)}
                aria-label={t("Edit {0}", title)}
              >
                {content}
              </button>
            ) : (
              <div className={style}>{content}</div>
            )}
            {writable && (
              <div className="pt-1.5">
                <ActionsMenu
                  label={t("Manage {0}", title)}
                  actions={[
                    { label: "Edit", onSelect: () => edit(entry) },
                    { label: "Delete", danger: true, onSelect: () => remove(entry) },
                  ]}
                />
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}
