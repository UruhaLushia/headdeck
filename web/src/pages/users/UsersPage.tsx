import { t } from "../../lib/i18n";
import { Button, Input } from "@heroui/react";
import { Plus } from "lucide-react";
import { useState } from "react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { Badge, DataTable, Empty, Page } from "../../components/Page";
import { useCollection } from "../../hooks/useCollection";
import { useDialogs } from "../../hooks/useDialogs";
import { api } from "../../lib/api";
import type { User } from "../../lib/types";
import { UserDetails } from "./UserDetails";

export function UsersPage({ permissions }: { permissions: string[] }) {
  const resource = useCollection<User>("users");
  const { openForm, openConfirm, dialogs } = useDialogs();
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string>();
  const selectedUser = resource.items.find((user) => user.id === selected);
  const writable = permissions.includes("users.write");
  const users = resource.items.filter((user) =>
    [user.name, user.email, user.display_name]
      .join(" ")
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  function actions(user: User) {
    if (!writable) return [];
    return [
      ...(user.provider === "oidc"
        ? []
        : [
            {
              label: "Rename",
              onSelect: () => {
                setSelected(undefined);
                openForm({
                  title: "Rename user",
                  description: user.name,
                  fields: [{ name: "name", label: "Name", values: [user.name], required: true }],
                  onSubmit: async (values) => {
                    await api.post(`/api/users/${user.id}/rename`, { new_name: values.name[0] });
                    await resource.refresh();
                  },
                });
              },
            },
          ]),
      {
        label: "Delete user",
        danger: true,
        onSelect: () => {
          setSelected(undefined);
          openConfirm({
            title: "Delete user",
            description: t("Delete {0}? Detach devices and account bindings first.", user.name),
            destructive: true,
            onConfirm: async () => {
              await api.delete(`/api/users/${user.id}`, { confirm_name: user.name });
              await resource.refresh();
            },
          });
        },
      },
    ];
  }

  function create() {
    openForm({
      title: "Create user",
      fields: [{ name: "name", label: "Name", values: [""], required: true }],
      onSubmit: async (values) => {
        await api.post("/api/users", { name: values.name[0] });
        await resource.refresh();
      },
    });
  }
  return (
    <Page
      title="Users"
      description="Manage users and device ownership."
      error={resource.error}
      loading={resource.loading}
      refresh={resource.refresh}
      actions={
        writable && (
          <Button onPress={create}>
            <Plus size={16} />
            {t("Add user")}
          </Button>
        )
      }
    >
      <Input
        aria-label={t("Search users")}
        placeholder={t("Search users…")}
        className="w-full sm:max-w-sm"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />
      {!users.length ? (
        <Empty>{resource.loading ? t("Loading users…") : t("No users found.")}</Empty>
      ) : (
        <DataTable columns={["User", "Email", "Provider", ""]}>
          {users.map((user) => (
            <tr
              key={user.id}
              className="h-18 cursor-pointer"
              onClick={(event) => {
                if (!(event.target as HTMLElement).closest("button, a, [role=button]"))
                  setSelected(user.id);
              }}
            >
              <td>
                <button
                  type="button"
                  className="max-w-xs truncate text-left font-medium hover:text-accent focus-visible:outline-2 focus-visible:outline-accent"
                  aria-label={t("View {0}", user.display_name || user.name)}
                  title={user.display_name || user.name}
                  onClick={() => setSelected(user.id)}
                >
                  {user.display_name || user.name}
                </button>
                {user.display_name && <p className="text-xs text-muted">{user.name}</p>}
              </td>
              <td data-label={t("Email")} className="text-muted">
                {user.email || "—"}
              </td>
              <td data-label={t("Provider")}>
                <Badge>{user.provider || t("Local")}</Badge>
              </td>
              <td onClick={(event) => event.stopPropagation()}>
                <ActionsMenu label={t("Manage {0}", user.name)} actions={actions(user)} />
              </td>
            </tr>
          ))}
        </DataTable>
      )}
      {selectedUser && (
        <UserDetails
          user={selectedUser}
          actions={actions(selectedUser)}
          onClose={() => setSelected(undefined)}
        />
      )}
      {dialogs}
    </Page>
  );
}
