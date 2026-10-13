import type { DNSConfig, Setting } from "../../lib/types";
import { SettingsList } from "../settings/SettingsList";

export function DNSGeneral({
  config,
  section,
  writable,
  save,
}: {
  config: DNSConfig;
  section: "magic" | "global" | "search";
  writable: boolean;
  save: (edit: (draft: DNSConfig) => void) => Promise<void>;
}) {
  const items: Setting[] =
    section === "magic"
      ? [
          { key: "dns.magic_dns", value: String(config.magic_dns) },
          { key: "dns.base_domain", value: config.base_domain },
          { key: "dns.override_local_dns", value: String(config.override_local_dns) },
        ]
      : section === "global"
        ? [{ key: "dns.nameservers.global", value: JSON.stringify(config.nameservers.global) }]
        : [{ key: "dns.search_domains", value: JSON.stringify(config.search_domains) }];
  return (
    <SettingsList
      group={
        section === "magic"
          ? "MagicDNS"
          : section === "global"
            ? "Global nameservers"
            : "Search domains"
      }
      items={items}
      writable={writable}
      save={(item, value) =>
        save((draft) => {
          switch (item.key) {
            case "dns.magic_dns":
              draft.magic_dns = value === "true";
              break;
            case "dns.base_domain":
              draft.base_domain = value.trim();
              break;
            case "dns.override_local_dns":
              draft.override_local_dns = value === "true";
              break;
            case "dns.nameservers.global":
              draft.nameservers.global = JSON.parse(value);
              break;
            case "dns.search_domains":
              draft.search_domains = JSON.parse(value);
              break;
          }
        })
      }
    />
  );
}
