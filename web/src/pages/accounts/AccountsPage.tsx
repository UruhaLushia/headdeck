import { t } from "../../lib/i18n";
import { Button } from "@heroui/react";
import { Plus } from "lucide-react";
import { useState } from "react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { Badge, DataTable, Empty, Page } from "../../components/Page";
import { useCollection } from "../../hooks/useCollection";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import type { Account, Role, User } from "../../lib/types";
import { IdentityDialog } from "./IdentityDialog";
import { AccountBinding } from "./AccountBinding";
import { AccountSecurity } from "./AccountSecurity";

export function AccountsPage({ permissions }: { permissions: string[] }) {
  const resource = useResource<Account[]>("/api/accounts");
  const users = useCollection<User>("users");
  const { openForm, dialogs } = useDialogs();
  const [identities, setIdentities] = useState<Account>();
  const [binding, setBinding] = useState<Account>();
  const [security, setSecurity] = useState<Account>();
  const writable = permissions.includes("accounts.write");
  function edit(account?: Account) {
    const availableUsers = users.items.filter(
      (user) =>
        !(resource.data ?? []).some(
          (item) => item.id !== account?.id && item.headscale_user_id === user.id,
        ),
    );
    openForm({
      title: account ? t("Edit account") : t("Create account"),
      fields: [
        ...(!account
          ? [
              { name: "username", label: "Username", values: [""] },
              { name: "password", label: "Password", values: [""], secret: true },
            ]
          : []),
        {
          name: "label",
          label: "Display name",
          values: [account?.label ?? ""],
          required: true,
        },
        {
          name: "role",
          label: "Role",
          values: [account?.role ?? "pending"],
          choices: ["pending", "member", "admin", "owner"],
        },
        ...(!account
          ? [
              {
                name: "user",
                label: "Headscale user",
                values: [""],
                choices: ["", ...availableUsers.map((user) => user.id)],
                choiceLabels: {
                  "": t("Unlinked"),
                  ...Object.fromEntries(availableUsers.map((user) => [user.id, user.name])),
                },
              },
            ]
          : []),
        {
          name: "enabled",
          label: "Allow sign-in",
          values: [String(!account?.disabled)],
          control: "switch",
        },
      ],
      onSubmit: async (values) => {
        const body: Account = {
          id: account?.id ?? "",
          label: values.label[0],
          role: values.role[0] as Role,
          disabled: values.enabled[0] !== "true",
          headscale_user_id: account?.headscale_user_id ?? values.user?.[0],
        };
        if (account) await api.put(`/api/accounts/${account.id}`, body);
        else
          await api.post("/api/accounts", {
            ...body,
            username: values.username[0],
            password: values.password[0],
          });
        await resource.refresh();
      },
    });
  }
  const accounts = resource.data ?? [];
  return (
    <Page
      title="Access accounts"
      error={resource.error}
      loading={resource.loading}
      refresh={resource.refresh}
      actions={
        writable && (
          <Button onPress={() => edit()}>
            <Plus size={16} />
            {t("Add account")}
          </Button>
        )
      }
    >
      {!accounts.length ? (
        <Empty>{resource.loading ? t("Loading accounts…") : t("No access accounts yet.")}</Empty>
      ) : (
        <DataTable columns={["Account", "Role", "Headscale user", "Status", ""]}>
          {accounts.map((account) => (
            <tr key={account.id}>
              <td>
                <p className="font-medium">{account.label}</p>
                {account.username && <p className="text-xs text-muted">{account.username}</p>}
              </td>
              <td data-label={t("Role")}>
                <Badge>{t(account.role)}</Badge>
              </td>
              <td data-label={t("Headscale user")} className="text-muted">
                {users.items.find((user) => user.id === account.headscale_user_id)?.name ||
                  t("Unlinked")}
              </td>
              <td data-label={t("Status")}>
                <Badge tone={account.disabled ? "neutral" : "success"}>
                  {account.disabled ? t("Account suspended") : t("Account active")}
                </Badge>
              </td>
              <td>
                <ActionsMenu
                  label={t("Manage {0}", account.label)}
                  actions={[
                    ...(writable
                      ? [
                          {
                            label: "Edit account",
                            onSelect: () => edit(account),
                          },
                          { label: "Headscale binding", onSelect: () => setBinding(account) },
                          { label: "Security", onSelect: () => setSecurity(account) },
                          {
                            label: "Set password",
                            onSelect: () =>
                              openForm({
                                title: "Set password",
                                fields: [
                                  ...(!account.headscale_user_id
                                    ? [
                                        {
                                          name: "username",
                                          label: "Username",
                                          values: [account.username || ""],
                                          required: true,
                                        },
                                      ]
                                    : []),
                                  {
                                    name: "password",
                                    label: "New password",
                                    values: [""],
                                    secret: true,
                                    required: true,
                                  },
                                ],
                                onSubmit: async (values) => {
                                  await api.put(`/api/accounts/${account.id}/password`, {
                                    username: account.headscale_user_id
                                      ? account.username
                                      : values.username?.[0],
                                    password: values.password[0],
                                  });
                                  await resource.refresh();
                                },
                              }),
                          },
                        ]
                      : []),
                    {
                      label: "Login identities",
                      onSelect: () => setIdentities(account),
                    },
                  ]}
                />
              </td>
            </tr>
          ))}
        </DataTable>
      )}
      {dialogs}
      {security && <AccountSecurity account={security} onClose={() => setSecurity(undefined)} />}
      {binding && (
        <AccountBinding
          account={binding}
          accounts={accounts}
          users={users.items}
          refresh={resource.refresh}
          onClose={() => setBinding(undefined)}
        />
      )}
      {identities && (
        <IdentityDialog
          account={identities}
          writable={writable}
          onClose={() => setIdentities(undefined)}
        />
      )}
    </Page>
  );
}
