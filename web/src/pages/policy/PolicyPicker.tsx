import { Button, Checkbox, Input, Label } from "@heroui/react";
import { Search, X } from "lucide-react";
import { useState } from "react";
import { t } from "../../lib/i18n";
import type { PolicyOption } from "./policyOptions";
import { optionLabel, validAddress } from "./policyOptions";

export function PolicyPicker({
  label,
  options,
  values,
  onChange,
  disabled,
  addresses = false,
  custom = false,
}: {
  label: string;
  options: PolicyOption[];
  values: string[];
  onChange: (values: string[]) => void;
  disabled: boolean;
  addresses?: boolean;
  custom?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [extra, setExtra] = useState("");
  const [error, setError] = useState(false);
  const chosen = [...new Set(values.filter(Boolean))];
  const resolved = options.flatMap((option) => {
    const aliases = option.aliases?.filter((alias) => chosen.includes(alias)) ?? [];
    return aliases.length ? [...new Set(aliases)].map((value) => ({ ...option, value })) : [option];
  });
  const known = new Map(resolved.map((option) => [option.value, option]));
  const all: PolicyOption[] = [
    ...resolved,
    ...chosen
      .filter((value) => !known.has(value))
      .map((value) => ({ value, label: value, kind: "Other configured values" })),
  ];
  const matches = all.filter((option) =>
    `${option.label} ${option.value} ${option.description ?? ""} ${t(option.kind)}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  const kinds = [...new Set(matches.map((option) => option.kind))];
  return (
    <fieldset className="min-w-0 space-y-3" disabled={disabled}>
      <legend className="mb-2 text-sm font-medium">{t(label)}</legend>
      {chosen.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {chosen.map((value) => (
            <Button
              key={value}
              size="sm"
              variant="secondary"
              className="h-auto max-w-full py-1.5"
              isDisabled={disabled}
              aria-label={t("Remove {0}", optionLabel(resolved, value))}
              onPress={() => onChange(values.filter((v) => v !== value))}
            >
              <span className="min-w-0 wrap-anywhere">{optionLabel(resolved, value)}</span>
              <X size={13} className="shrink-0" />
            </Button>
          ))}
        </div>
      )}
      <div className="overflow-hidden rounded-xl border border-border">
        <div className="flex items-center gap-2 border-b border-border px-3">
          <Search size={15} className="shrink-0 text-muted" />
          <Input
            aria-label={t("Search {0}", t(label))}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            disabled={disabled}
            className="min-w-0 flex-1 rounded-none border-0 bg-transparent px-0 text-base shadow-none sm:text-sm"
            placeholder={t("Search…")}
          />
        </div>
        <div className="max-h-64 overflow-y-auto overscroll-contain p-2 sm:max-h-52">
          {kinds.map((kind) => (
            <div key={kind} className="space-y-0.5">
              <p className="px-2 pt-2 pb-1 text-xs text-muted">{t(kind)}</p>
              {matches
                .filter((option) => option.kind === kind)
                .map((option) => (
                  <Checkbox
                    key={option.value}
                    aria-label={optionLabel([option], option.value)}
                    isDisabled={disabled}
                    isSelected={chosen.includes(option.value)}
                    onChange={(selected) =>
                      onChange(
                        selected
                          ? [...values.filter(Boolean), option.value]
                          : values.filter((v) => v !== option.value),
                      )
                    }
                    className="w-full rounded-lg px-2 py-2 hover:bg-default"
                  >
                    <Checkbox.Content className="w-full items-center">
                      <Checkbox.Control className="shrink-0">
                        <Checkbox.Indicator />
                      </Checkbox.Control>
                      <div className="min-w-0">
                        <Label className="block wrap-anywhere">{option.label}</Label>
                        {option.description && option.description !== option.label && (
                          <span className="block text-xs text-muted wrap-anywhere">
                            {option.description}
                          </span>
                        )}
                      </div>
                    </Checkbox.Content>
                  </Checkbox>
                ))}
            </div>
          ))}
          {!matches.length && <p className="p-3 text-sm text-muted">{t("No matching options.")}</p>}
        </div>
      </div>
      {(addresses || custom) && (
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Input
              aria-label={t(addresses ? "IP address or prefix" : "Custom SSH username")}
              placeholder={addresses ? "192.168.1.0/24" : "ubuntu"}
              value={extra}
              disabled={disabled}
              onChange={(event) => {
                setExtra(event.target.value);
                setError(false);
              }}
              className="min-w-0 flex-1 rounded-xl border border-border text-base shadow-none sm:text-sm"
            />
            <Button
              size="sm"
              variant="secondary"
              isDisabled={disabled || !extra.trim()}
              onPress={() => {
                const value = extra.trim();
                if (addresses && !validAddress(value)) {
                  setError(true);
                  return;
                }
                if (!chosen.includes(value)) onChange([...values.filter(Boolean), value]);
                setExtra("");
              }}
            >
              {t("Add")}
            </Button>
          </div>
          {error && (
            <p role="alert" className="text-xs text-danger">
              {t("Enter a valid IP address or subnet.")}
            </p>
          )}
        </div>
      )}
    </fieldset>
  );
}
