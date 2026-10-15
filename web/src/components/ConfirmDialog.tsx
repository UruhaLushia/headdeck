import { dialogStyles } from "./dialogStyles";
import { t } from "../lib/i18n";
import { Alert, AlertDialog, Button } from "@heroui/react";
import { useRef, useState } from "react";

export interface ConfirmSpec {
  title: string;
  description: string;
  destructive?: boolean;
  onConfirm: () => Promise<void>;
}
export function ConfirmDialog({ confirm, onClose }: { confirm: ConfirmSpec; onClose: () => void }) {
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const [error, setError] = useState<string>();
  async function submit() {
    if (submitting.current) return;
    submitting.current = true;
    setPending(true);
    setError(undefined);
    try {
      await confirm.onConfirm();
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }
  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <AlertDialog.Backdrop
        className={dialogStyles.backdrop}
        isDismissable={!pending}
        isKeyboardDismissDisabled={pending}
      >
        <AlertDialog.Container>
          <AlertDialog.Dialog className={dialogStyles.dialog}>
            <AlertDialog.Header className={dialogStyles.header}>
              <AlertDialog.Heading className={dialogStyles.heading}>
                {t(confirm.title)}
              </AlertDialog.Heading>
            </AlertDialog.Header>
            <AlertDialog.Body className={dialogStyles.body}>
              <p className="text-sm text-muted">{t(confirm.description)}</p>
              {error && (
                <Alert status="danger">
                  <Alert.Content>
                    <Alert.Description>{error}</Alert.Description>
                  </Alert.Content>
                </Alert>
              )}
            </AlertDialog.Body>
            <AlertDialog.Footer className={dialogStyles.footer}>
              <Button variant="tertiary" isDisabled={pending} onPress={onClose}>
                {t("Cancel")}
              </Button>
              <Button
                variant={confirm.destructive ? "danger" : "primary"}
                isPending={pending}
                isDisabled={pending}
                onPress={() => void submit()}
              >
                {t("Confirm")}
              </Button>
            </AlertDialog.Footer>
          </AlertDialog.Dialog>
        </AlertDialog.Container>
      </AlertDialog.Backdrop>
    </AlertDialog>
  );
}
