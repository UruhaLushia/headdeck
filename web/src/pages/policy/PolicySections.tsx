import { CategoryNav } from "../../components/Categories";
import { t } from "../../lib/i18n";
import type { PolicyItem, PolicyObjects } from "../../lib/types";
import { PolicySelect } from "./PolicyControls";

const sections = [
  {
    label: "Rules",
    items: [
      { id: "acls", label: "Access rules" },
      { id: "grants", label: "Grants" },
      { id: "ssh", label: "SSH rules" },
    ],
  },
  {
    label: "Resources",
    items: [
      { id: "groups", label: "Groups" },
      { id: "tagOwners", label: "Tag owners" },
      { id: "hosts", label: "Host aliases" },
    ],
  },
  {
    label: "Automatic approvals",
    items: [
      { id: "autoApprovers.routes", label: "Route approvers" },
      { id: "autoApprovers.exitNode", label: "Exit node approvers" },
    ],
  },
];

export function sectionLabel(section: string) {
  return (
    sections.flatMap((group) => group.items).find((item) => item.id === section)?.label ?? section
  );
}

export function policyTitle(item: PolicyItem) {
  if (["acls", "grants", "ssh"].includes(item.section)) return t("Rule {0}", item.index + 1);
  return item.key.replace(/^(group|tag):/, "") || t(sectionLabel(item.section));
}

export function PolicySections({
  selected,
  items,
  onSelect,
}: {
  selected: string;
  items: PolicyObjects["items"];
  onSelect: (section: string) => void;
}) {
  const available = new Set(items.map(({ item }) => item.section));
  const groups = sections.map((group) => ({
    ...group,
    items: group.items
      .filter((item) => available.has(item.id))
      .map((item) => ({
        ...item,
        count: items.filter((entry) => entry.item.section === item.id && !entry.item.add).length,
      })),
  }));
  return (
    <>
      <div className="md:hidden">
        <PolicySelect
          label="Policy categories"
          value={selected}
          options={groups.flatMap((group) =>
            group.items.map((item) => ({
              value: item.id,
              label: `${t(item.label)}${item.count ? ` · ${item.count}` : ""}`,
            })),
          )}
          onChange={onSelect}
          disabled={false}
        />
      </div>
      <div className="hidden md:block">
        <CategoryNav
          label="Policy categories"
          groups={groups}
          selected={selected}
          onSelect={onSelect}
        />
      </div>
    </>
  );
}
