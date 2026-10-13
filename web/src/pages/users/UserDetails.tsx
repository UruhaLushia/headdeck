import { Button, Modal } from "@heroui/react";
import { dialogStyles } from "../../components/dialogStyles";
import { t } from "../../lib/i18n";
import type { User } from "../../lib/types";

export function UserDetails({
  user,
  actions,
  onClose,
}: {
  user: User;
  actions: { label: string; onSelect: () => void; danger?: boolean }[];
  onClose: () => void;
}) {
  const created = user.created_at ? new Date(user.created_at) : undefined;
  const details = [
    ["Name", user.name],
    ["Email", user.email || "—"],
    ["Provider", user.provider || t("Local")],
    [
      "Created at",
      created && !Number.isNaN(created.getTime()) && !user.created_at?.startsWith("0001-")
        ? created.toLocaleString()
        : "—",
    ],
  ];
  return (
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
              <Modal.Heading className={`${dialogStyles.heading} wrap-break-word pr-6`}>
                {user.display_name || user.name}
              </Modal.Heading>
              <p className="text-sm text-muted">{t("User details")}</p>
            </Modal.Header>
            <Modal.Body className={dialogStyles.body}>
              <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
                {details.map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-xs text-muted">{t(label)}</dt>
                    <dd className="mt-1 wrap-break-word font-medium select-text">{value}</dd>
                  </div>
                ))}
              </dl>
            </Modal.Body>
            {actions.length > 0 && (
              <Modal.Footer className={dialogStyles.footer}>
                {actions.map((action) => (
                  <Button
                    key={action.label}
                    size="sm"
                    variant={action.danger ? "danger-soft" : "secondary"}
                    onPress={action.onSelect}
                  >
                    {t(action.label)}
                  </Button>
                ))}
              </Modal.Footer>
            )}
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
