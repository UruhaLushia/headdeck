import { Button, Input, ListBox, Select, Switch } from "@heroui/react";
import { useRef, useState } from "react";
import { ListEditor } from "../../components/ListEditor";
import { t } from "../../lib/i18n";
import { listSetting, settingField, settingLabel } from "../../lib/settings";
import type { Setting } from "../../lib/types";

function displayValue(item: Setting) {
  if (listSetting(item.key)) return JSON.stringify(JSON.parse(item.value || "[]") ?? []);
  return item.value;
}

export function SettingRow({
  item,
  group,
  writable,
  save,
}: {
  item: Setting;
  group: string;
  writable: boolean;
  save: (item: Setting, value: string) => Promise<void>;
}) {
  const original = displayValue(item);
  const [draft, setDraft] = useState<string>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const busy = useRef(false);
  const value = draft ?? original;
  const dirty = value !== original;
  const field = settingField(item);
  const label = t(settingLabel(item.key));
  const heading = group === "DERP" ? label.replace(/^DERP\s+/, "") : label;
  const boolean = field.choices?.includes("true") && field.choices.includes("false");
  async function commit(next: string) {
    if (busy.current) return;
    busy.current = true;
    setPending(true);
    setError(undefined);
    try {
      const serialized = listSetting(item.key)
        ? JSON.stringify((JSON.parse(next) as string[]).map((v) => v.trim()).filter(Boolean))
        : next;
      await save(item, serialized);
      setDraft(undefined);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("Request failed"));
    } finally {
      busy.current = false;
      setPending(false);
    }
  }
  function change(next: string) {
    setDraft(next);
    setError(undefined);
  }
  const readonly = item.secret
    ? t(item.configured ? "Configured" : "Not configured")
    : boolean
      ? t(item.value === "true" ? "Enabled" : "Disabled")
      : (listSetting(item.key) ? (JSON.parse(original) as string[]).join("\n") : original) || "—";
  return (
    <div className="grid min-h-14 grid-cols-1 gap-2 px-1 py-3 sm:grid-cols-[minmax(140px,0.7fr)_minmax(0,1fr)] sm:items-start sm:gap-5">
      <span className="pt-2 text-sm text-muted">{heading}</span>
      <div className="min-w-0">
        {!writable ? (
          <span className="block whitespace-pre-line wrap-break-word py-2 text-sm">{readonly}</span>
        ) : boolean ? (
          <Switch
            aria-label={label}
            isSelected={value === "true"}
            isDisabled={pending}
            onChange={(selected) => void commit(String(selected))}
            className="py-2"
          >
            <Switch.Content>
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
            </Switch.Content>
          </Switch>
        ) : field.choices?.length ? (
          <Select
            aria-label={label}
            value={value || null}
            isDisabled={pending}
            onChange={(next) => void commit(String(next ?? ""))}
          >
            <Select.Trigger className="rounded-xl border border-border shadow-none">
              <Select.Value />
              <Select.Indicator />
            </Select.Trigger>
            <Select.Popover>
              <ListBox>
                {field.choices.map((option) => (
                  <ListBox.Item key={option} id={option} textValue={t(option)}>
                    {t(option)}
                    <ListBox.ItemIndicator />
                  </ListBox.Item>
                ))}
              </ListBox>
            </Select.Popover>
          </Select>
        ) : (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (!dirty) return;
              void commit(value);
            }}
            className="space-y-2"
          >
            {listSetting(item.key) ? (
              <ListEditor
                label={label}
                values={JSON.parse(value) as string[]}
                disabled={pending}
                onChange={(next) => change(JSON.stringify(next))}
              />
            ) : (
              <div className="flex items-center gap-2">
                <Input
                  aria-label={label}
                  value={value}
                  disabled={pending}
                  type={item.secret ? "password" : "text"}
                  autoComplete={item.secret ? "new-password" : "off"}
                  placeholder={
                    item.secret ? t(item.configured ? "Configured" : "Not configured") : undefined
                  }
                  onChange={(event) => change(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Escape") {
                      setDraft(undefined);
                      setError(undefined);
                    }
                  }}
                  className="min-w-0 flex-1 rounded-xl border border-border shadow-none"
                />
                {item.secret && item.configured && !dirty && (
                  <Button
                    size="sm"
                    variant="tertiary"
                    isDisabled={pending}
                    onPress={() => void commit("")}
                  >
                    {t("Clear")}
                  </Button>
                )}
              </div>
            )}
            {dirty ? (
              <div className="flex justify-end gap-2">
                <Button
                  size="sm"
                  variant="tertiary"
                  isDisabled={pending}
                  onPress={() => {
                    setDraft(undefined);
                    setError(undefined);
                  }}
                >
                  {t("Cancel")}
                </Button>
                <Button size="sm" type="submit" isPending={pending} isDisabled={pending}>
                  {t("Save")}
                </Button>
              </div>
            ) : null}
          </form>
        )}
        {item.key === "server.web.trusted_proxies" && (
          <p className="mt-2 text-xs text-muted">
            {t("Only accept client IPs forwarded by these proxies.")}
          </p>
        )}
        {error && (
          <p role="alert" className="mt-2 text-sm text-danger">
            {t(error)}
          </p>
        )}
      </div>
    </div>
  );
}
