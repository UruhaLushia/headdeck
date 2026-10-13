import { CategoryPanel } from "../../components/Categories";
import type { Setting } from "../../lib/types";
import { SettingRow } from "./SettingRow";

export function SettingsList({
  items,
  group,
  writable,
  save,
}: {
  items: Setting[];
  group: string;
  writable: boolean;
  save: (item: Setting, value: string) => Promise<void>;
}) {
  const sections =
    group === "DERP"
      ? [
          {
            title: "DERP map",
            items: items.filter((item) => !item.key.startsWith("hs.derp.server.")),
          },
          {
            title: "Embedded DERP",
            items: items.filter((item) => item.key.startsWith("hs.derp.server.")),
          },
        ]
      : [{ title: group, items }];
  return (
    <div className="min-w-0 space-y-4">
      {sections
        .filter((section) => section.items.length)
        .map((section) => (
          <CategoryPanel key={section.title} title={section.title}>
            <div className="divide-y divide-border">
              {section.items.map((item) => (
                <SettingRow
                  key={item.key}
                  item={item}
                  group={group}
                  writable={writable}
                  save={save}
                />
              ))}
            </div>
          </CategoryPanel>
        ))}
    </div>
  );
}
