import { t } from "../../lib/i18n";
import { Button, Input } from "@heroui/react";
import { Plus, RefreshCw } from "lucide-react";
import { useLayoutEffect, useRef, useState } from "react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { AddDeviceDialog } from "../../components/AddDeviceDialog";
import { Badge, DataTable, Empty, Page } from "../../components/Page";
import { useCollection } from "../../hooks/useCollection";
import { useDialogs } from "../../hooks/useDialogs";
import { api } from "../../lib/api";
import type { Node, User } from "../../lib/types";
import { NodeDetails, NodeRoles } from "./NodeDetails";

export function NodesPage({
  permissions,
  loginServer,
  nodeID,
}: {
  permissions: string[];
  loginServer?: string;
  nodeID?: string;
}) {
  const resource = useCollection<Node>("nodes");
  const users = useCollection<User>("users");
  const { openForm, openConfirm, dialogs } = useDialogs();
  const [search, setSearch] = useState("");
  const [adding, setAdding] = useState(false);
  const listScroll = useRef(0);
  const selectedNode = resource.items.find((node) => node.id === nodeID);
  useLayoutEffect(() => {
    window.scrollTo(0, nodeID ? 0 : listScroll.current);
  }, [nodeID]);
  const can = (permission: string) => permissions.includes(permission);
  const nodes = resource.items.filter((node) =>
    [node.given_name, node.name, node.user.name, ...node.ip_addresses, ...(node.forced_tags ?? [])]
      .join(" ")
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  function actions(node: Node) {
    const name = node.given_name || node.name;
    const out = [];
    const mutate = async (action: string, body?: unknown) => {
      await api.post(`/api/nodes/${node.id}/${action}`, body);
      await resource.refresh();
    };
    if (can("nodes.manage"))
      out.push({
        label: "Rename",
        onSelect: () =>
          openForm({
            title: "Rename device",
            description: name,
            fields: [
              {
                name: "name",
                label: "Device name",
                values: [name],
                required: true,
              },
            ],
            onSubmit: (values) => mutate("rename", { new_name: values.name[0] }),
          }),
      });
    if (can("nodes.write"))
      out.push(
        {
          label: "Edit tags",
          onSelect: () =>
            openForm({
              title: "Edit tags",
              description: name,
              fields: [
                {
                  name: "tags",
                  label: "Tags",
                  multi: true,
                  values: node.forced_tags ?? [],
                },
              ],
              onSubmit: (values) => mutate("tags", { tags: values.tags.filter((v) => v.trim()) }),
            }),
        },
        {
          label: "Change owner",
          onSelect: () =>
            openForm({
              title: "Change owner",
              description: name,
              fields: [
                {
                  name: "user",
                  label: "Owner",
                  values: [node.user.id ?? ""],
                  choices: users.items.map((user) => user.id),
                  choiceLabels: Object.fromEntries(users.items.map((user) => [user.id, user.name])),
                },
              ],
              onSubmit: (values) => mutate("owner", { user_id: values.user[0] }),
            }),
        },
      );
    if (can("nodes.manage"))
      out.push({
        label: "Expire device",
        onSelect: () =>
          openConfirm({
            title: "Expire device",
            description: `${name} will need to sign in again.`,
            onConfirm: () => mutate("expire"),
          }),
      });
    if (can("routes.write"))
      out.push({
        label: "Manage routes",
        onSelect: () =>
          openForm({
            title: "Approve routes",
            description: name,
            fields: [
              {
                name: "routes",
                label: "Approved routes",
                values: node.approved_routes ?? [],
                multi: true,
                choices: [
                  ...new Set([...(node.available_routes ?? []), ...(node.approved_routes ?? [])]),
                ],
                choiceLabels: Object.fromEntries(
                  (node.approved_routes ?? [])
                    .filter((route) => !node.available_routes?.includes(route))
                    .map((route) => [route, t("{0} (not advertised)", route)]),
                ),
                help:
                  node.available_routes?.length || node.approved_routes?.length
                    ? "Select advertised routes to approve. Uncheck to revoke approval."
                    : "No advertised routes.",
              },
            ],
            onSubmit: (values) =>
              mutate("routes", {
                routes: values.routes.filter((v) => v.trim()),
              }),
          }),
      });
    if (can("nodes.delete"))
      out.push({
        label: "Delete device",
        danger: true,
        onSelect: () =>
          openConfirm({
            title: "Delete device",
            description: t("Delete {0}? This cannot be undone.", name),
            destructive: true,
            onConfirm: async () => {
              await api.delete(`/api/nodes/${node.id}`, { confirm_name: name });
              if (nodeID === node.id) location.hash = "/nodes";
              await resource.refresh();
            },
          }),
      });
    return out;
  }
  if (nodeID) {
    return (
      <>
        {selectedNode ? (
          <NodeDetails
            key={selectedNode.id}
            node={selectedNode}
            actions={actions(selectedNode)}
            error={resource.error}
            loading={resource.loading}
            refresh={resource.refresh}
          />
        ) : (
          <Page
            title="Device details"
            error={resource.error}
            loading={resource.loading}
            refresh={resource.refresh}
          >
            <a href="#/nodes" className="text-sm text-accent hover:underline">
              {t("Back to devices")}
            </a>
            <Empty>
              {resource.data === undefined && !resource.error
                ? t("Loading devices…")
                : t("Device not found.")}
            </Empty>
          </Page>
        )}
        {dialogs}
      </>
    );
  }
  return (
    <Page
      title="Devices"
      description="Manage the devices connected to your network."
      error={resource.error}
      loading={resource.loading}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Input
          aria-label={t("Search devices")}
          placeholder={t("Search devices, users or tags…")}
          className="w-full sm:max-w-sm"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="page-actions ml-auto flex items-center gap-2">
          <Button
            variant="tertiary"
            isIconOnly
            aria-label={t("Refresh")}
            onPress={() => void resource.refresh()}
          >
            <RefreshCw size={17} />
          </Button>
          {loginServer && (
            <Button onPress={() => setAdding(true)}>
              <Plus size={16} />
              {t("Add device")}
            </Button>
          )}
        </div>
      </div>
      {!nodes.length ? (
        <Empty>{resource.loading ? t("Loading devices…") : t("No devices found.")}</Empty>
      ) : (
        <DataTable columns={["Device", "Owner", "Addresses", "Status", ""]}>
          {nodes.map((node) => (
            <tr
              key={node.id}
              className="h-18 cursor-pointer"
              onClick={(event) => {
                if (
                  !(event.target instanceof Element) ||
                  event.target.closest("button, a, [role=button]")
                )
                  return;
                listScroll.current = window.scrollY;
                location.hash = `/nodes/${node.id}`;
              }}
            >
              <td className="sm:min-w-64">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1 sm:flex-nowrap">
                  <a
                    href={`#/nodes/${node.id}`}
                    className="min-w-0 max-w-xs truncate rounded text-left font-medium hover:text-accent focus-visible:outline-2 focus-visible:outline-accent"
                    onClick={() => {
                      listScroll.current = window.scrollY;
                    }}
                    aria-label={t("View {0}", node.given_name || node.name)}
                    title={node.given_name || node.name}
                  >
                    {node.given_name || node.name}
                  </a>
                  <NodeRoles node={node} inline />
                </div>
                <p
                  className="mt-1 max-w-sm truncate text-xs text-muted"
                  title={node.forced_tags?.join(" · ") || node.hostname || node.name}
                >
                  {node.forced_tags?.join(" · ") || node.hostname || node.name}
                </p>
              </td>
              <td data-label={t("Owner")}>{node.user.name ?? "—"}</td>
              <td
                data-label={t("Addresses")}
                className="mobile-table-wide font-mono text-xs text-muted sm:whitespace-nowrap"
              >
                <div className="space-y-1">
                  {node.ip_addresses.length
                    ? node.ip_addresses.map((address) => <p key={address}>{address}</p>)
                    : "—"}
                </div>
              </td>
              <td data-label={t("Status")}>
                <Badge tone={node.online ? "success" : node.expired ? "danger" : "neutral"}>
                  {node.online ? t("Online") : node.expired ? t("Expired") : t("Offline")}
                </Badge>
              </td>
              <td onClick={(event) => event.stopPropagation()}>
                <ActionsMenu
                  label={t("Manage {0}", node.given_name || node.name)}
                  actions={actions(node)}
                />
              </td>
            </tr>
          ))}
        </DataTable>
      )}
      {dialogs}
      {adding && loginServer && (
        <AddDeviceDialog
          loginServer={loginServer}
          users={can("nodes.write") ? users.items : undefined}
          onRegister={
            can("nodes.manage")
              ? async (key, userID) => {
                  await api.post("/api/nodes/register", { key, user_id: userID });
                  await resource.refresh();
                }
              : undefined
          }
          onClose={() => setAdding(false)}
        />
      )}
    </Page>
  );
}
