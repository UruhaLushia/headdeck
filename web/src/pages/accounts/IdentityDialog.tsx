import { dialogStyles } from "../../components/dialogStyles";
import { t } from "../../lib/i18n";
import { Button, Modal } from "@heroui/react";
import { Plus, Trash2 } from "lucide-react";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import type { Account, Identity } from "../../lib/types";

export function IdentityDialog({
  account,
  writable,
  onClose,
}: {
  account: Account;
  writable: boolean;
  onClose: () => void;
}) {
  const path = `/api/accounts/${account.id}/identities`;
  const resource = useResource<Identity[]>(path);
  const { openForm, openConfirm, dialogs } = useDialogs();
  return (
    <>
      <Modal
        isOpen
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
      >
        <Modal.Backdrop className={dialogStyles.backdrop}>
          <Modal.Container size="lg">
            <Modal.Dialog className={dialogStyles.dialog}>
              <Modal.CloseTrigger aria-label={t("Close")} />
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>
                  {t("Login identities")}
                </Modal.Heading>
                <p className="text-sm text-muted">{account.label}</p>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                <p className="text-sm text-muted">
                  {t(
                    "An identity belongs to exactly one account. Email addresses are not used to merge accounts.",
                  )}
                </p>
                {resource.error && <p className="text-sm text-danger">{resource.error}</p>}
                {resource.data?.map((identity) => (
                  <div
                    key={`${identity.issuer}:${identity.subject}`}
                    className="flex items-center justify-between gap-3 rounded-lg border border-border p-3"
                  >
                    <div className="min-w-0">
                      <p className="break-all text-sm font-medium">{identity.issuer}</p>
                      <p className="mt-1 break-all text-xs text-muted">{identity.subject}</p>
                    </div>
                    {writable && (
                      <Button
                        isIconOnly
                        size="sm"
                        variant="tertiary"
                        aria-label={t("Unbind identity")}
                        onPress={() =>
                          openConfirm({
                            title: "Unbind login identity",
                            description:
                              "This identity will no longer be associated with this account.",
                            destructive: true,
                            onConfirm: async () => {
                              await api.delete(path, {
                                issuer: identity.issuer,
                                subject: identity.subject,
                              });
                              await resource.refresh();
                            },
                          })
                        }
                      >
                        <Trash2 size={15} />
                      </Button>
                    )}
                  </div>
                ))}
                {!resource.data?.length && (
                  <p className="py-4 text-center text-sm text-muted">
                    {resource.loading ? t("Loading identities…") : t("No login identities bound.")}
                  </p>
                )}
                {writable && (
                  <Button
                    size="sm"
                    variant="secondary"
                    onPress={() =>
                      openForm({
                        title: "Bind login identity",
                        fields: [
                          {
                            name: "issuer",
                            label: "Issuer",
                            values: [""],
                            required: true,
                          },
                          {
                            name: "subject",
                            label: "Subject",
                            values: [""],
                            required: true,
                          },
                        ],
                        onSubmit: async (values) => {
                          await api.post(path, {
                            issuer: values.issuer[0],
                            subject: values.subject[0],
                          });
                          await resource.refresh();
                        },
                      })
                    }
                  >
                    <Plus size={15} />
                    {t("Bind identity")}
                  </Button>
                )}
              </Modal.Body>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal>
      {dialogs}
    </>
  );
}
