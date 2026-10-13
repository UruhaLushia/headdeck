import { Button, Modal } from "@heroui/react";
import { Trash2 } from "lucide-react";
import { dialogStyles } from "../../components/dialogStyles";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import type { Account } from "../../lib/types";

interface Security {
  totp: boolean;
  passkeys: { id: string; name: string }[];
}
export function AccountSecurity({ account, onClose }: { account: Account; onClose: () => void }) {
  const resource = useResource<Security>(`/api/accounts/${account.id}/security`);
  const { openConfirm, dialogs } = useDialogs();
  function confirm(title: string, action: string, id?: string) {
    openConfirm({
      title: t(title),
      description: t("Confirm {0}?", t(title)),
      onConfirm: async () => {
        await api.post(`/api/accounts/${account.id}/security`, { action, id });
        await resource.refresh();
      },
    });
  }
  return (
    <>
      <Modal
        isOpen
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
      >
        <Modal.Backdrop className={dialogStyles.backdrop}>
          <Modal.Container>
            <Modal.Dialog className={dialogStyles.dialog}>
              <Modal.CloseTrigger aria-label={t("Close")} />
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>{t("Security")}</Modal.Heading>
                <p className="mt-1 text-sm text-muted">{account.label}</p>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                <section className="space-y-3">
                  <h3 className="text-sm font-medium">Passkey</h3>
                  {!resource.data?.passkeys.length && (
                    <p className="text-sm text-muted">{t("Not set")}</p>
                  )}
                  {resource.data?.passkeys.map((key) => (
                    <div key={key.id} className="flex items-center justify-between gap-4 text-sm">
                      <span>{key.name}</span>
                      <Button
                        isIconOnly
                        size="sm"
                        variant="tertiary"
                        aria-label={t("Delete Passkey")}
                        onPress={() => confirm("Delete Passkey", "passkey.delete", key.id)}
                      >
                        <Trash2 size={16} />
                      </Button>
                    </div>
                  ))}
                </section>
                <div className="flex items-center justify-between border-t border-border pt-4">
                  <span className="text-sm">2FA</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    isDisabled={!resource.data?.totp}
                    onPress={() => confirm("Reset 2FA", "totp.reset")}
                  >
                    {t("Reset")}
                  </Button>
                </div>
                <div className="flex items-center justify-between border-t border-border pt-4">
                  <span className="text-sm">{t("Signed-in devices")}</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onPress={() => confirm("Sign out everywhere", "sessions.revoke")}
                  >
                    {t("Sign out everywhere")}
                  </Button>
                </div>
                {resource.error && (
                  <p role="alert" className="text-sm text-danger">
                    {resource.error}
                  </p>
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
