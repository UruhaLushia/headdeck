import { Button, Input, Label, ListBox, Select, TextField } from "@heroui/react";
import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { t } from "../../lib/i18n";
import { PolicyPicker } from "./PolicyPicker";
import type { PolicyOption } from "./policyOptions";
import { aclDestination, optionLabel, withPorts } from "./policyOptions";

export const protocols = [
  "",
  "tcp",
  "udp",
  "icmp",
  "ipv6-icmp",
  "sctp",
  "gre",
  "esp",
  "ah",
  "igmp",
  "ipv4",
  "ip-in-ip",
  "egp",
  "igp",
  "fc",
];

export function PolicySelect({
  label,
  value,
  options,
  onChange,
  disabled,
  hideLabel = false,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  disabled: boolean;
  hideLabel?: boolean;
}) {
  return (
    <Select
      className="min-w-0"
      isDisabled={disabled}
      value={value}
      onChange={(next) => onChange(String(next ?? ""))}
    >
      <Label className={hideLabel ? "sr-only" : undefined}>{t(label)}</Label>
      <Select.Trigger className="min-w-0 rounded-xl border border-border shadow-none">
        <Select.Value className="min-w-0 truncate" />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover>
        <ListBox>
          {options.map((option) => (
            <ListBox.Item key={option.value} id={option.value} textValue={t(option.label)}>
              {t(option.label)}
              <ListBox.ItemIndicator />
            </ListBox.Item>
          ))}
        </ListBox>
      </Select.Popover>
    </Select>
  );
}

const portChoices = [
  { value: "*", label: "All ports" },
  { value: "22", label: "SSH · 22" },
  { value: "53", label: "DNS · 53" },
  { value: "80", label: "HTTP · 80" },
  { value: "443", label: "HTTPS · 443" },
  { value: "80,443", label: "HTTP + HTTPS · 80,443" },
  { value: "custom", label: "Custom ports" },
];

function Ports({
  value,
  onChange,
  disabled,
  compact = false,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  compact?: boolean;
}) {
  const [custom, setCustom] = useState(!portChoices.some((option) => option.value === value));
  return (
    <div className="min-w-0 space-y-2">
      <PolicySelect
        label="Ports"
        hideLabel={compact}
        value={custom ? "custom" : value}
        options={portChoices}
        disabled={disabled}
        onChange={(next) => {
          setCustom(next === "custom");
          if (next !== "custom") onChange(next);
        }}
      />
      {custom && (
        <TextField isRequired isDisabled={disabled} value={value} onChange={onChange}>
          <Input
            aria-label={t("Custom ports")}
            placeholder="8080,8443,9000-9010"
            className="rounded-xl border border-border shadow-none"
          />
        </TextField>
      )}
    </div>
  );
}

export function ACLDestinations({
  options,
  values,
  onChange,
  disabled,
}: {
  options: PolicyOption[];
  values: string[];
  onChange: (values: string[]) => void;
  disabled: boolean;
}) {
  const rows = values.filter(Boolean).map(aclDestination);
  return (
    <div className="space-y-3">
      <PolicyPicker
        label="Destinations"
        options={options}
        values={rows.map((row) => row.target)}
        disabled={disabled}
        addresses
        onChange={(targets) => {
          const current = new Set(rows.map((row) => row.target));
          onChange([
            ...values.filter((value) => targets.includes(aclDestination(value).target)),
            ...targets
              .filter((target) => !current.has(target))
              .map((target) => withPorts(target, "*")),
          ]);
        }}
      />
      {rows.map((row, index) => (
        <div
          key={`${row.target}:${index}`}
          className="grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)] items-center gap-3 rounded-xl border border-border p-3"
        >
          <span className="text-sm font-medium wrap-anywhere">
            {optionLabel(options, row.target)}
          </span>
          <Ports
            compact
            value={row.ports}
            disabled={disabled}
            onChange={(ports) =>
              onChange(
                values
                  .filter(Boolean)
                  .map((value, i) => (i === index ? withPorts(row.target, ports) : value)),
              )
            }
          />
        </div>
      ))}
    </div>
  );
}

export function GrantServices({
  values,
  onChange,
  disabled,
}: {
  values: string[];
  onChange: (values: string[]) => void;
  disabled: boolean;
}) {
  const entries = values.filter(Boolean);
  return (
    <fieldset className="min-w-0 space-y-3" disabled={disabled}>
      <legend className="mb-2 text-sm font-medium">{t("Allowed services")}</legend>
      {entries.map((entry, index) => {
        const separator = entry.indexOf(":");
        const protocol = separator < 0 ? "" : entry.slice(0, separator);
        const ports = separator < 0 ? entry : entry.slice(separator + 1);
        const update = (nextProtocol: string, nextPorts: string) =>
          onChange(
            entries.map((value, i) =>
              i === index ? `${nextProtocol ? `${nextProtocol}:` : ""}${nextPorts}` : value,
            ),
          );
        return (
          <div
            key={index}
            className="flex min-w-0 items-start gap-2 rounded-xl border border-border p-3"
          >
            <div className="grid min-w-0 flex-1 gap-3 sm:grid-cols-2">
              <PolicySelect
                label="Protocol"
                value={protocol || "any"}
                options={[...new Set([...protocols, protocol])].map((value) => ({
                  value: value || "any",
                  label: value ? value.toUpperCase() : "Any protocol",
                }))}
                disabled={disabled}
                onChange={(next) => update(next === "any" ? "" : next, ports)}
              />
              <Ports
                value={ports}
                onChange={(next) => update(protocol, next)}
                disabled={disabled}
              />
            </div>
            <Button
              isIconOnly
              size="sm"
              variant="tertiary"
              aria-label={t("Remove service {0}", index + 1)}
              isDisabled={disabled}
              onPress={() => onChange(entries.filter((_, i) => i !== index))}
              className="mt-6"
            >
              <Trash2 size={15} />
            </Button>
          </div>
        );
      })}
      <Button
        size="sm"
        variant="secondary"
        isDisabled={disabled}
        onPress={() => onChange([...entries, "tcp:443"])}
      >
        <Plus size={15} />
        {t("Add service")}
      </Button>
    </fieldset>
  );
}
