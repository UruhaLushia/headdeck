import { Button } from "@heroui/react";
import { Check, ChevronRight, Copy, Settings2, UserRound } from "lucide-react";
import { useState } from "react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { Badge, Page, Panel } from "../../components/Page";
import { t } from "../../lib/i18n";
import type { Node } from "../../lib/types";

export function NodeRoles({ node, inline = false }: { node: Node; inline?: boolean }) {
  const routes = [...(node.available_routes ?? []), ...(node.approved_routes ?? [])];
  const exit = routes.some((route) => route === "0.0.0.0/0" || route === "::/0");
  const subnet = routes.some((route) => route !== "0.0.0.0/0" && route !== "::/0");
  if (!exit && !subnet) return null;
  return (
    <div
      className={inline ? "flex shrink-0 gap-1.5 whitespace-nowrap" : "mt-2 flex flex-wrap gap-1.5"}
    >
      {subnet && <Badge>{t("Subnet router")}</Badge>}
      {exit && <Badge>{t("Exit node")}</Badge>}
    </div>
  );
}

function timestamp(value?: string, fallback = "—") {
  if (!value || value.startsWith("0001-")) return fallback;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

export function NodeDetails({
  node,
  actions,
  error,
  loading,
  refresh,
}: {
  node: Node;
  actions: { label: string; onSelect: () => void; danger?: boolean }[];
  error?: string;
  loading: boolean;
  refresh: () => Promise<void>;
}) {
  const [copied, setCopied] = useState<string>();
  const [copyError, setCopyError] = useState(false);
  const name = node.given_name || node.name;
  const routeAction = actions.find((action) => action.label === "Manage routes");
  const settingsActions = actions.filter((action) => action !== routeAction);
  const tags = [...new Set([...(node.forced_tags ?? []), ...(node.valid_tags ?? [])])];
  const approved = node.approved_routes ?? [];
  const available = node.available_routes ?? [];
  const routes = [...new Set([...available, ...approved])];
  const exitRoutes = new Set(["0.0.0.0/0", "::/0"]);
  const routeGroups = [
    { label: "Approved routes", routes: approved.filter((route) => !exitRoutes.has(route)) },
    {
      label: "Awaiting approval",
      routes: available.filter((route) => !exitRoutes.has(route) && !approved.includes(route)),
    },
    { label: "Exit node", routes: routes.filter((route) => exitRoutes.has(route)) },
  ];
  const details = [
    ["Device name", name],
    ["Hostname", node.hostname || node.name || "—"],
    ...(node.os ? [["Operating system", node.os]] : []),
    ...(node.client || node.client_version
      ? [["Client", [node.client, node.client_version].filter(Boolean).join(" ")]]
      : []),
    ["Created at", timestamp(node.created_at)],
    ["Last seen", timestamp(node.last_seen)],
    ["Expiry", timestamp(node.expiry, t("Never"))],
  ];

  async function copy(address: string) {
    try {
      await navigator.clipboard.writeText(address);
      setCopied(address);
      setCopyError(false);
    } catch {
      setCopyError(true);
    }
  }

  return (
    <div className="min-w-0 space-y-4">
      <nav
        aria-label={t("Breadcrumb")}
        className="flex min-w-0 items-center gap-2 text-sm text-muted"
      >
        <a
          href="#/nodes"
          className="shrink-0 rounded hover:text-accent focus-visible:outline-2 focus-visible:outline-accent"
        >
          {t("Devices")}
        </a>
        <ChevronRight size={14} className="shrink-0" aria-hidden />
        <span className="truncate" aria-current="page">
          {name}
        </span>
      </nav>
      <Page
        title={<span className="wrap-anywhere">{name}</span>}
        error={
          error || (copyError ? t("Copy failed. Select and copy the address manually.") : undefined)
        }
        loading={loading}
        refresh={refresh}
        actions={
          <ActionsMenu label={t("Device settings")} actions={settingsActions}>
            <Settings2 size={16} />
            {t("Device settings")}
          </ActionsMenu>
        }
      >
        <Panel flush>
          <div className="grid divide-y divide-border sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] sm:divide-x sm:divide-y-0">
            <div className="flex min-h-20 items-center gap-3 px-5 py-4">
              <UserRound size={19} className="shrink-0 text-muted" />
              <div className="min-w-0">
                <p className="text-xs text-muted">{t("Owner")}</p>
                <p className="mt-1 truncate text-sm font-medium">{node.user.name || "—"}</p>
              </div>
            </div>
            <div className="flex min-h-20 flex-wrap items-center gap-3 px-5 py-4">
              <Badge tone={node.online ? "success" : node.expired ? "danger" : "neutral"}>
                {t(node.online ? "Online" : node.expired ? "Expired" : "Offline")}
              </Badge>
              <NodeRoles node={node} inline />
              {node.ephemeral && <Badge>{t("Ephemeral")}</Badge>}
            </div>
          </div>
        </Panel>

        <section className="space-y-3" aria-label={t("Subnets & routing")}>
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold">{t("Subnets & routing")}</h2>
            {routeAction && (
              <Button size="sm" variant="secondary" onPress={routeAction.onSelect}>
                {t("Manage routes")}
              </Button>
            )}
          </div>
          <Panel flush>
            <div className="grid divide-y divide-border md:grid-cols-3 md:divide-x md:divide-y-0">
              {routeGroups.map((group) => (
                <div key={group.label} className="min-w-0 space-y-3 px-5 py-4">
                  <h3 className="text-xs text-muted">{t(group.label)}</h3>
                  <div className="space-y-2">
                    {group.routes.length ? (
                      group.routes.map((route) => (
                        <div key={route} className="flex flex-wrap items-center gap-2">
                          <span className="font-mono text-sm break-all select-text">{route}</span>
                          {exitRoutes.has(route) && (
                            <Badge tone={approved.includes(route) ? "success" : "neutral"}>
                              {t(approved.includes(route) ? "Approved" : "Awaiting approval")}
                            </Badge>
                          )}
                          {approved.includes(route) && !available.includes(route) && (
                            <Badge>{t("Not advertised")}</Badge>
                          )}
                        </div>
                      ))
                    ) : (
                      <span className="text-sm text-muted">—</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </Panel>
        </section>

        <div className="grid items-start gap-5 xl:grid-cols-2">
          <Panel title="Device details" flush>
            <dl className="divide-y divide-border">
              {details.map(([label, value]) => (
                <div
                  key={label}
                  className="grid min-h-12 grid-cols-[minmax(0,1fr)_minmax(0,2fr)] items-center gap-4 px-5 py-3 text-sm"
                >
                  <dt className="text-muted">{t(label)}</dt>
                  <dd className="min-w-0 wrap-anywhere select-text">{value}</dd>
                </div>
              ))}
            </dl>
          </Panel>
          <div className="min-w-0 space-y-5">
            <Panel title="Addresses" flush>
              <dl className="divide-y divide-border">
                {node.ip_addresses.map((address) => (
                  <div
                    key={address}
                    className="flex min-h-16 items-center justify-between gap-3 px-5 py-3"
                  >
                    <div className="min-w-0">
                      <dt className="text-xs text-muted">
                        {address.includes(":") ? "IPv6" : "IPv4"}
                      </dt>
                      <dd className="mt-1 font-mono text-sm break-all select-text">{address}</dd>
                    </div>
                    <Button
                      isIconOnly
                      size="sm"
                      variant="tertiary"
                      aria-label={t("Copy {0}", address)}
                      onPress={() => void copy(address)}
                    >
                      {copied === address ? <Check size={15} /> : <Copy size={15} />}
                    </Button>
                  </div>
                ))}
                {!node.ip_addresses.length && <div className="px-5 py-4 text-sm text-muted">—</div>}
              </dl>
            </Panel>
            <Panel title="Tags">
              <div className="flex flex-wrap gap-2">
                {tags.length ? (
                  tags.map((tag) => <Badge key={tag}>{tag}</Badge>)
                ) : (
                  <span className="text-sm text-muted">{t("No tags.")}</span>
                )}
              </div>
            </Panel>
          </div>
        </div>
      </Page>
    </div>
  );
}
