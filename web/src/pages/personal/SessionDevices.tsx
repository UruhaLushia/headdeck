import { Button } from "@heroui/react";
import { Monitor, MonitorSmartphone, Smartphone } from "lucide-react";
import { useState } from "react";
import { useResource } from "../../hooks/useResource";
import { useDialogs } from "../../hooks/useDialogs";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import { Panel } from "../../components/Page";

interface LoginDevice {
  id: string;
  browser: string;
  browser_version?: string;
  platform?: string;
  platform_version?: string;
  created_at: string;
  current: boolean;
}
const signedOut = () => window.dispatchEvent(new Event("headdeck:unauthenticated"));

export function SessionDevices() {
  const resource = useResource<LoginDevice[]>("/api/sessions");
  const { openConfirm, dialogs } = useDialogs();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState<string>();
  async function remove(device: LoginDevice) {
    setPending(device.id);
    setError(undefined);
    try {
      await api.delete(`/api/sessions/${device.id}`);
      if (device.current) signedOut();
      else await resource.refresh();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      setPending(undefined);
    }
  }
  return (
    <Panel title="Signed-in devices" flush>
      {(error || resource.error) && (
        <p role="alert" className="px-5 pt-4 text-sm text-danger">
          {error || resource.error}
        </p>
      )}
      <div className="divide-y divide-border">
        {resource.data?.map((device) => {
          const Icon = ["Android", "iOS"].includes(device.platform ?? "")
            ? Smartphone
            : device.platform
              ? Monitor
              : MonitorSmartphone;
          return (
            <div
              key={device.id}
              className="flex min-h-20 items-center justify-between gap-4 px-5 py-2.5"
            >
              <div className="flex min-w-0 items-center gap-3">
                <Icon size={20} className="shrink-0 text-muted" />
                <div className="min-w-0">
                  <p
                    className="truncate text-sm font-medium"
                    title={`${t(device.browser)} ${device.browser_version ?? ""}`}
                  >
                    {t(device.browser)} {device.browser_version}
                  </p>
                  <p className="mt-0.5 flex h-4 items-center gap-2 text-xs text-muted">
                    <span
                      className="truncate"
                      title={`${device.platform ?? ""} ${device.platform_version ?? ""}`}
                    >
                      {device.platform} {device.platform_version}
                    </span>
                    {device.current && (
                      <span className="shrink-0 text-accent">{t("This device")}</span>
                    )}
                  </p>
                  <p
                    className="mt-0.5 truncate text-xs text-muted"
                    title={new Date(device.created_at).toLocaleString()}
                  >
                    {new Date(device.created_at).toLocaleString()}
                  </p>
                </div>
              </div>
              <Button
                size="sm"
                variant="tertiary"
                isDisabled={!!pending}
                isPending={pending === device.id}
                onPress={() => remove(device)}
              >
                {t("Sign out")}
              </Button>
            </div>
          );
        })}
      </div>
      <div className="flex justify-end border-t border-border px-5 py-3">
        <Button
          variant="secondary"
          isDisabled={!!pending}
          onPress={() =>
            openConfirm({
              title: t("Sign out everywhere"),
              description: t("Confirm {0}?", t("Sign out everywhere")),
              onConfirm: async () => {
                await api.delete("/api/sessions");
                signedOut();
              },
            })
          }
        >
          {t("Sign out everywhere")}
        </Button>
      </div>
      {dialogs}
    </Panel>
  );
}
