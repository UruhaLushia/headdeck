import { Button, Modal } from "@heroui/react";
import { dialogStyles } from "../../components/dialogStyles";
import { Badge } from "../../components/Page";
import { t } from "../../lib/i18n";
import { appURL } from "../../lib/url";
import type { Provider } from "./loginProvider";

export function ProviderDetails({
  provider,
  onClose,
  onEdit,
  onDelete,
  pending,
}: {
  provider: Provider;
  onClose: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
  pending: boolean;
}) {
  const details = [
    ["Type", provider.kind.toUpperCase()],
    ["Auto-register", t(provider.auto_register ? "Enabled" : "Disabled")],
    ...(provider.issuer ? [["Issuer", provider.issuer]] : []),
    ["Client ID", provider.client_id || "—"],
    ["Client secret", t(provider.secret_configured ? "Configured" : "Not set")],
    ["Callback URL", appURL(`/api/auth/${provider.id}/callback`)],
    ...(provider.auth_url ? [["Authorization URL", provider.auth_url]] : []),
    ...(provider.token_url ? [["Token URL", provider.token_url]] : []),
    ...(provider.userinfo_url ? [["User info URL", provider.userinfo_url]] : []),
  ];
  return (
    <Modal
      isOpen
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <Modal.Backdrop
        className={dialogStyles.backdrop}
        isDismissable={!pending}
        isKeyboardDismissDisabled={pending}
      >
        <Modal.Container size="lg">
          <Modal.Dialog className={dialogStyles.dialog}>
            {!pending && <Modal.CloseTrigger aria-label={t("Close")} />}
            <Modal.Header className={dialogStyles.header}>
              <Modal.Heading className={dialogStyles.heading}>{provider.name}</Modal.Heading>
              <Badge tone={provider.enabled ? "success" : "neutral"}>
                {provider.enabled ? "Enabled" : "Disabled"}
              </Badge>
            </Modal.Header>
            <Modal.Body className={dialogStyles.body}>
              <dl className="grid grid-cols-1 gap-5 text-sm sm:grid-cols-2">
                {details.map(([label, value]) => (
                  <div key={label} className={label === "Callback URL" ? "sm:col-span-2" : ""}>
                    <dt className="text-xs text-muted">{t(label)}</dt>
                    <dd className="mt-1 break-all font-medium select-text">{value}</dd>
                  </div>
                ))}
              </dl>
            </Modal.Body>
            {(onDelete || onEdit) && (
              <Modal.Footer className={dialogStyles.footer}>
                {onDelete && (
                  <Button variant="danger-soft" isDisabled={pending} onPress={onDelete}>
                    {t("Delete")}
                  </Button>
                )}
                {onEdit && (
                  <Button isPending={pending} isDisabled={pending} onPress={onEdit}>
                    {t("Edit")}
                  </Button>
                )}
              </Modal.Footer>
            )}
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
