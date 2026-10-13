import { t } from "../../lib/i18n";
import { Button } from "@heroui/react";
import { Cpu, Network, Server, Users } from "lucide-react";
import { Badge, Page, Panel } from "../../components/Page";
import { useCollection } from "../../hooks/useCollection";
import { useDialogs } from "../../hooks/useDialogs";
import { useStatus } from "../../hooks/useStatus";
import { api } from "../../lib/api";
import type { Node, Operation, User } from "../../lib/types";

function memory(value?: number | null) {
  return value == null ? "—" : `${(value / 1048576).toFixed(1)} MiB`;
}

export function OverviewPage({
  permissions,
  onOperation,
}: {
  permissions: string[];
  onOperation: (operation: Operation) => void;
}) {
  const resource = useStatus();
  const nodes = useCollection<Node>("nodes");
  const users = useCollection<User>("users");
  const { openConfirm, dialogs } = useDialogs();
  const status = resource.data;
  const control = status?.service_control;
  const state = control?.state;
  const reachable = !!status?.headscale.reachable;
  const cards = [
    {
      label: "Devices",
      value: nodes.items.length,
      caption: t("{0} online", nodes.items.filter((node) => node.online).length),
      icon: Network,
    },
    {
      label: "Users",
      value: users.items.length,
      caption: "Network users",
      icon: Users,
    },
    {
      label: "Headscale memory",
      value: memory(state?.rss_bytes),
      caption: state?.uptime ? t("Uptime {0}", state.uptime) : t("Process memory"),
      icon: Server,
    },
    {
      label: "Daemon memory",
      value: memory(status?.server.rss_bytes),
      caption: status?.server.uptime ? t("Uptime {0}", status.server.uptime) : t("Process memory"),
      icon: Cpu,
    },
  ];
  return (
    <Page
      title="Overview"
      description="Your network at a glance."
      error={resource.error}
      refresh={resource.refresh}
    >
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {cards.map(({ label, value, caption, icon: Icon }) => (
          <Panel key={label}>
            <div className="mb-4 flex items-center justify-between text-sm text-muted">
              <span>{t(label)}</span>
              <Icon size={17} />
            </div>
            <p className="text-3xl font-semibold tracking-tight">{value}</p>
            <p className="mt-2 text-xs text-muted">{t(caption)}</p>
          </Panel>
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel>
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold">Headscale</h2>
            <Badge tone={reachable ? "success" : "danger"}>
              {state?.active_state || (reachable ? t("Running") : t("Unavailable"))}
            </Badge>
          </div>
          <dl className="my-5 grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-muted">{t("Version")}</dt>
              <dd className="mt-1 font-medium">{status?.headscale.version || "—"}</dd>
            </div>
            <div>
              <dt className="text-muted">API</dt>
              <dd className="mt-1 font-medium">{reachable ? t("Connected") : t("Unavailable")}</dd>
            </div>
          </dl>
          {status?.headscale.last_error && (
            <p className="mb-4 text-sm text-danger">{status.headscale.last_error.message}</p>
          )}
          {control?.supported && permissions.includes("service.manage") && (
            <div className="flex flex-wrap gap-2">
              {["start", "stop", "restart", ...(state?.can_reload ? ["reload"] : [])].map(
                (action) => (
                  <Button
                    key={action}
                    size="sm"
                    variant={action === "restart" ? "primary" : "secondary"}
                    onPress={() =>
                      openConfirm({
                        title: t(`${action[0].toUpperCase()}${action.slice(1)} Headscale`),
                        description:
                          action === "stop"
                            ? t(
                                "Devices will not receive network updates while Headscale is stopped.",
                              )
                            : t("Change the state of the Headscale service."),
                        destructive: action === "stop",
                        onConfirm: async () => {
                          onOperation(await api.post<Operation>(`/api/headscale/${action}`));
                        },
                      })
                    }
                  >
                    {t(action[0].toUpperCase() + action.slice(1))}
                  </Button>
                ),
              )}
            </div>
          )}
        </Panel>
        <Panel>
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold">{t("Headdeck daemon")}</h2>
            <Badge tone={status ? "success" : "danger"}>
              {status ? t("Running") : t("Unavailable")}
            </Badge>
          </div>
          <dl className="my-5 grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-muted">{t("Version")}</dt>
              <dd className="mt-1 font-medium">{status?.server.version || "—"}</dd>
            </div>
            <div>
              <dt className="text-muted">{t("Uptime")}</dt>
              <dd className="mt-1 font-medium">{status?.server.uptime || "—"}</dd>
            </div>
          </dl>
        </Panel>
      </div>
      {dialogs}
    </Page>
  );
}
