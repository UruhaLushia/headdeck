import { Button } from "@heroui/react";
import { Plus, ChevronRight } from "lucide-react";
import { useState } from "react";
import { CategoryPanel } from "../../components/Categories";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { useResourceCache } from "../../hooks/ResourceProvider";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import { settingField } from "../../lib/settings";
import type { Setting, Settings } from "../../lib/types";
import { ProviderDialog } from "./LoginProviderDialog";
import { ProviderDetails } from "./LoginProviderDetails";
import type { Provider } from "./loginProvider";

function builtin(provider: Provider) {
  return ["oidc", "github"].includes(provider.id);
}

export function LoginProviders({ writable }: { writable: boolean }) {
  const resource = useResource<Provider[]>("/api/login-providers");
  const cache = useResourceCache();
  const [editing, setEditing] = useState<Provider>();
  const [selected, setSelected] = useState<Provider>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const { openForm, openConfirm, dialogs } = useDialogs();
  async function edit(provider: Provider) {
    if (!writable || pending) return;
    if (!builtin(provider)) {
      setSelected(undefined);
      setEditing(provider);
      return;
    }
    setPending(true);
    setError(undefined);
    try {
      const settings = await api.get<Settings>("/api/settings");
      const prefix = `server.web.${provider.id}.`;
      const keys = [
        "name",
        "issuer",
        "client_id",
        "client_secret",
        "enabled",
        "auto_register",
        "disable_token_login",
      ];
      const items = keys.flatMap((key) =>
        settings.items.filter((item) => item.key === prefix + key),
      );
      openForm({
        title: "Edit provider",
        fields: items.map((item) => ({
          ...settingField(item),
          name: item.key,
          label: item.key.endsWith(".enabled")
            ? "Enabled"
            : item.key.endsWith(".auto_register")
              ? "Auto-register"
              : settingField(item).label,
          help: undefined,
          ...(item.value === "true" || item.value === "false"
            ? { control: "switch" as const, choices: undefined }
            : {}),
        })),
        onSubmit: (values) => saveBuiltin(provider, items, values),
      });
      setSelected(undefined);
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      setPending(false);
    }
  }
  async function saveBuiltin(
    provider: Provider,
    items: Setting[],
    values: Record<string, string[]>,
  ) {
    const prefix = `server.web.${provider.id}.`;
    const changes = Object.fromEntries(
      items
        .filter((item) => !item.secret || values[item.key][0] !== "")
        .map((item) => [item.key, values[item.key][0]]),
    );
    const saved = await api.patch<Settings>("/api/settings", { changes });
    cache?.set("/api/settings", saved);
    const secretConfigured = saved.items.find(
      (setting) => setting.key === prefix + "client_secret",
    )?.configured;
    resource.setData((current) =>
      current?.map((item) =>
        item.id === provider.id
          ? {
              ...item,
              name: changes[prefix + "name"] ?? item.name,
              issuer: changes[prefix + "issuer"] ?? item.issuer,
              client_id: changes[prefix + "client_id"] ?? item.client_id,
              enabled: changes[prefix + "enabled"] === "true",
              auto_register: changes[prefix + "auto_register"] === "true",
              secret_configured: secretConfigured,
            }
          : item,
      ),
    );
  }
  return (
    <>
      <CategoryPanel
        title="Login providers"
        actions={
          writable && (
            <Button
              size="sm"
              variant="secondary"
              onPress={() =>
                setEditing({
                  id: "",
                  name: "",
                  kind: "oidc",
                  enabled: true,
                  auto_register: false,
                  issuer: "",
                  client_id: "",
                  client_secret: "",
                })
              }
            >
              <Plus size={15} />
              {t("Add")}
            </Button>
          )
        }
      >
        {resource.data?.map((provider) => (
          <button
            type="button"
            key={provider.id}
            className="flex w-full items-center justify-between gap-3 rounded-xl px-3 py-3 text-left transition-colors hover:bg-surface-secondary focus-visible:outline-2 focus-visible:outline-accent"
            onClick={() => setSelected(provider)}
          >
            <div className="min-w-0">
              <p className="text-sm font-medium">{provider.name}</p>
              <p className="text-xs text-muted">
                {provider.kind.toUpperCase()} · {t(provider.enabled ? "Enabled" : "Disabled")}
              </p>
            </div>
            <ChevronRight size={17} className="shrink-0 text-muted" />
          </button>
        ))}
        {(resource.error || error) && (
          <p role="alert" className="px-3 text-sm text-danger">
            {resource.error || error}
          </p>
        )}
      </CategoryPanel>
      {selected && (
        <ProviderDetails
          provider={selected}
          pending={pending}
          onClose={() => setSelected(undefined)}
          onEdit={writable ? () => void edit(selected) : undefined}
          onDelete={
            writable && !builtin(selected)
              ? () => {
                  const provider = selected;
                  setSelected(undefined);
                  openConfirm({
                    title: "Delete provider",
                    description: provider.name,
                    destructive: true,
                    onConfirm: async () => {
                      await api.delete(`/api/login-providers/${provider.id}`);
                      await resource.refresh();
                    },
                  });
                }
              : undefined
          }
        />
      )}
      {editing && (
        <ProviderDialog
          provider={editing}
          onClose={() => setEditing(undefined)}
          onSave={resource.refresh}
        />
      )}
      {dialogs}
    </>
  );
}
