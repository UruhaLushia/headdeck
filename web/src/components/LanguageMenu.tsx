import { Dropdown } from "@heroui/react";
import { Languages } from "lucide-react";
import { setLocale, t, useLocale } from "../lib/i18n";

export function LanguageMenu({ compact = false }: { compact?: boolean }) {
  const locale = useLocale();
  return (
    <Dropdown>
      <Dropdown.Trigger
        className={`flex items-center gap-2 rounded-xl text-sm text-muted transition-colors hover:bg-default ${compact ? "p-2.5" : "px-3 py-2"}`}
        aria-label={t("Language")}
      >
        <Languages size={18} /> {!compact && (locale === "zh-CN" ? "简体中文" : "English")}
      </Dropdown.Trigger>
      <Dropdown.Popover placement="bottom end" className="min-w-36 rounded-xl">
        <Dropdown.Menu
          selectionMode="single"
          selectedKeys={[locale]}
          onAction={(key) => setLocale(key === "zh-CN" ? "zh-CN" : "en")}
        >
          <Dropdown.Item id="zh-CN" textValue="简体中文">
            简体中文
            <Dropdown.ItemIndicator />
          </Dropdown.Item>
          <Dropdown.Item id="en" textValue="English">
            English
            <Dropdown.ItemIndicator />
          </Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}
