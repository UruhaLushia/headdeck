import { Dropdown } from "@heroui/react";
import { Monitor, Moon, Sun } from "lucide-react";
import { t } from "../lib/i18n";
import { setTheme, useTheme } from "../lib/theme";

export function ThemeMenu() {
  const theme = useTheme();
  const options = [
    { id: "light", label: "Light", icon: Sun },
    { id: "dark", label: "Dark", icon: Moon },
    { id: "auto", label: "Follow system", icon: Monitor },
  ] as const;
  const Icon = options.find((option) => option.id === theme)!.icon;
  return (
    <Dropdown>
      <Dropdown.Trigger
        className="rounded-xl p-2.5 text-muted transition-colors hover:bg-default"
        aria-label={t("Appearance")}
      >
        <Icon size={18} />
      </Dropdown.Trigger>
      <Dropdown.Popover placement="bottom end" className="min-w-40 rounded-xl">
        <Dropdown.Menu
          selectionMode="single"
          selectedKeys={[theme]}
          onAction={(key) => setTheme(key === "light" || key === "dark" ? key : "auto")}
        >
          {options.map(({ id, label, icon: OptionIcon }) => (
            <Dropdown.Item key={id} id={id} textValue={t(label)}>
              <OptionIcon size={16} />
              {t(label)}
              <Dropdown.ItemIndicator />
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}
