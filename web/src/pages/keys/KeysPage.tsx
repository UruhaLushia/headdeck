import { dialogStyles } from "../../components/dialogStyles";
import { t } from "../../lib/i18n";
import { Button, Input, Modal } from "@heroui/react";
import { Copy, Plus } from "lucide-react";
import { useState } from "react";
import { ActionsMenu } from "../../components/ActionsMenu";
import { Badge, DataTable, Empty, Page } from "../../components/Page";
import { useCollection } from "../../hooks/useCollection";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import type { PreAuthKey, User } from "../../lib/types";

export function KeysPage({ permissions }: { permissions: string[] }) {
  const resource = useResource<{ keys: PreAuthKey[] }>("/api/preauth-keys");
  const users = useCollection<User>("users");
  const { openForm, openConfirm, dialogs } = useDialogs();
  const [created, setCreated] = useState<PreAuthKey>();
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const writable = permissions.includes("keys.write");
  function create() {
    openForm({
      title: "Create auth key",
      fields: [
        {
          name: "user",
          label: "User",
          values: [users.items[0]?.id ?? ""],
          choices: users.items.map((user) => user.id),
          choiceLabels: Object.fromEntries(users.items.map((user) => [user.id, user.name])),
        },
        {
          name: "expiry",
          label: "Expiration",
          values: ["24h"],
          help: "Use a duration such as 24h or 90d, or a date and time.",
        },
        {
          name: "reusable",
          label: "Reusable",
          values: ["false"],
          choices: ["false", "true"],
        },
        {
          name: "ephemeral",
          label: "Ephemeral",
          values: ["false"],
          choices: ["false", "true"],
        },
        ...(permissions.includes("policy.write")
          ? [{ name: "tags", label: "Tags", values: [], multi: true }]
          : []),
      ],
      onSubmit: async (values) => {
        const key = await api.post<PreAuthKey>("/api/preauth-keys", {
          user: values.user[0],
          expiration: values.expiry[0],
          reusable: values.reusable[0] === "true",
          ephemeral: values.ephemeral[0] === "true",
          acl_tags: (values.tags ?? []).filter((value) => value.trim()),
        });
        await resource.refresh();
        setCreated(key);
        setCopied(false);
        setCopyError(false);
      },
    });
  }
  const keys = resource.data?.keys ?? [];
  return (
    <Page
      title="Auth keys"
      description="Register devices without interactive sign-in."
      error={resource.error}
      loading={resource.loading}
      refresh={resource.refresh}
      actions={
        writable && (
          <Button isDisabled={!users.items.length} onPress={create}>
            <Plus size={16} />
            {t("Create key")}
          </Button>
        )
      }
    >
      {!keys.length ? (
        <Empty>{resource.loading ? t("Loading keys…") : t("No auth keys yet.")}</Empty>
      ) : (
        <DataTable columns={["Key", "User", "Type", "Expires", ""]}>
          {keys.map((key) => (
            <tr key={key.id}>
              <td className="font-mono text-xs sm:whitespace-nowrap">{key.key || key.id}</td>
              <td data-label={t("User")}>{key.user.name || "—"}</td>
              <td data-label={t("Type")}>
                <Badge>
                  {key.ephemeral ? t("Ephemeral") : key.reusable ? t("Reusable") : t("Single use")}
                </Badge>
              </td>
              <td
                data-label={t("Expires")}
                className="mobile-table-wide text-muted sm:whitespace-nowrap"
              >
                {key.expired
                  ? t("Expired")
                  : key.expiration
                    ? new Date(key.expiration).toLocaleString()
                    : "—"}
              </td>
              <td>
                <ActionsMenu
                  label={t("Manage auth key")}
                  actions={
                    !writable
                      ? []
                      : [
                          {
                            label: "Expire key",
                            onSelect: () =>
                              openConfirm({
                                title: "Expire key",
                                description:
                                  "This key will no longer allow new devices to register.",
                                onConfirm: async () => {
                                  await api.post(`/api/preauth-keys/${key.id}/expire`);
                                  await resource.refresh();
                                },
                              }),
                          },
                          {
                            label: "Delete key",
                            danger: true,
                            onSelect: () =>
                              openConfirm({
                                title: "Delete key",
                                description: "Remove this auth key permanently?",
                                destructive: true,
                                onConfirm: async () => {
                                  await api.delete(`/api/preauth-keys/${key.id}`);
                                  await resource.refresh();
                                },
                              }),
                          },
                        ]
                  }
                />
              </td>
            </tr>
          ))}
        </DataTable>
      )}
      {dialogs}
      {created && (
        <Modal
          isOpen
          onOpenChange={(open) => {
            if (!open) setCreated(undefined);
          }}
        >
          <Modal.Backdrop className={dialogStyles.backdrop}>
            <Modal.Container>
              <Modal.Dialog className={dialogStyles.dialog}>
                <Modal.CloseTrigger aria-label={t("Close")} />
                <Modal.Header className={dialogStyles.header}>
                  <Modal.Heading className={dialogStyles.heading}>
                    {t("Your new auth key")}
                  </Modal.Heading>
                </Modal.Header>
                <Modal.Body className={dialogStyles.body}>
                  <p className="text-sm text-muted">
                    {t("Copy this key now. It will only be shown once.")}
                  </p>
                  <Input aria-label={t("New auth key")} readOnly value={created.key ?? ""} />
                  {copyError && (
                    <p role="alert" className="text-sm text-danger">
                      {t("Copy failed. Select and copy the key manually.")}
                    </p>
                  )}
                </Modal.Body>
                <Modal.Footer className={dialogStyles.footer}>
                  <Button
                    onPress={async () => {
                      if (created.key) {
                        try {
                          await navigator.clipboard.writeText(created.key);
                          setCopied(true);
                          setCopyError(false);
                        } catch {
                          setCopyError(true);
                        }
                      }
                    }}
                  >
                    <Copy size={16} />
                    {copied ? t("Copied") : t("Copy key")}
                  </Button>
                </Modal.Footer>
              </Modal.Dialog>
            </Modal.Container>
          </Modal.Backdrop>
        </Modal>
      )}
    </Page>
  );
}
