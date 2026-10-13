import { dialogStyles } from "./dialogStyles";
import { t } from "../lib/i18n";
import { Alert, Button, Modal } from "@heroui/react";
import { useRef, useState } from "react";
import type { Field } from "../lib/types";
import { Fields } from "./Fields";

export interface FormSpec {
  title: string;
  description?: string;
  fields: Field[];
  save?: string;
  onSubmit: (values: Record<string, string[]>) => Promise<void>;
}

export function FormDialog({ form, onClose }: { form: FormSpec; onClose: () => void }) {
  const [values, setValues] = useState(() =>
    Object.fromEntries(form.fields.map((f) => [f.name, f.values])),
  );
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const [error, setError] = useState<string>();
  async function submit() {
    if (submitting.current) return;
    submitting.current = true;
    setPending(true);
    setError(undefined);
    try {
      await form.onSubmit(values);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("Request failed"));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }
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
            <form
              className="contents"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>{t(form.title)}</Modal.Heading>
                {form.description && <p className="text-sm text-muted">{t(form.description)}</p>}
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                <Fields
                  fields={form.fields}
                  values={values}
                  disabled={pending}
                  onChange={(name, value) =>
                    setValues((current) => ({ ...current, [name]: value }))
                  }
                />
                {error && (
                  <Alert status="danger" role="alert">
                    <Alert.Content>
                      <Alert.Description>{error}</Alert.Description>
                    </Alert.Content>
                  </Alert>
                )}
              </Modal.Body>
              <Modal.Footer className={dialogStyles.footer}>
                <Button variant="tertiary" isDisabled={pending} onPress={onClose}>
                  {t("Cancel")}
                </Button>
                <Button type="submit" isPending={pending} isDisabled={pending}>
                  {t(form.save ?? "Save")}
                </Button>
              </Modal.Footer>
            </form>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
