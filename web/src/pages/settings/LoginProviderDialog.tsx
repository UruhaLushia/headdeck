import { Button, Modal } from "@heroui/react";
import { useState } from "react";
import { Fields } from "../../components/Fields";
import { dialogStyles } from "../../components/dialogStyles";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import { appURL } from "../../lib/url";
import type { Field } from "../../lib/types";
import type { Provider } from "./loginProvider";

const field = (name: string, label: string, extra: Partial<Field> = {}): Field => ({
  name,
  label,
  values: [],
  ...extra,
});

export function ProviderDialog({
  provider,
  onClose,
  onSave,
}: {
  provider: Provider;
  onClose: () => void;
  onSave: () => Promise<void>;
}) {
  const [values, setValues] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(
      Object.entries(provider).map(([key, value]) => [
        key,
        Array.isArray(value) ? value : [String(value ?? "")],
      ]),
    ),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const kind = values.kind?.[0] || "oidc";
  const fields = [
    ...(!provider.id ? [field("id", "ID", { required: true })] : []),
    field("name", "Name", { required: true }),
    ...(!provider.id ? [field("kind", "Type", { choices: ["oidc", "github", "oauth"] })] : []),
    ...(kind === "oidc" ? [field("issuer", "Issuer", { required: true })] : []),
    field("client_id", "Client ID", { required: true }),
    field("client_secret", "Client secret", { secret: true }),
    ...(kind === "oauth"
      ? [
          field("auth_url", "Authorization URL", { required: true }),
          field("token_url", "Token URL", { required: true }),
          field("userinfo_url", "User info URL", { required: true }),
          field("scopes", "Scopes", { multi: true }),
          field("subject_claim", "Subject claim"),
          field("name_claim", "Name claim"),
          field("avatar_claim", "Avatar claim"),
        ]
      : []),
    field("enabled", "Enabled", { control: "switch" }),
    field("auto_register", "Auto-register", { control: "switch" }),
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
            <form
              className="contents"
              onSubmit={async (event) => {
                event.preventDefault();
                if (pending) return;
                setPending(true);
                setError(undefined);
                try {
                  const body = Object.fromEntries(
                    Object.entries(values).map(([key, value]) => [
                      key,
                      key === "scopes"
                        ? value.filter(Boolean)
                        : ["enabled", "auto_register"].includes(key)
                          ? value[0] === "true"
                          : value[0],
                    ]),
                  );
                  delete body.secret_configured;
                  await api.put(`/api/login-providers/${provider.id || values.id[0]}`, body);
                  await onSave();
                  onClose();
                } catch (reason) {
                  setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
                } finally {
                  setPending(false);
                }
              }}
            >
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>
                  {t(provider.id ? "Edit provider" : "Add provider")}
                </Modal.Heading>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                <Fields
                  fields={fields}
                  values={values}
                  disabled={pending}
                  onChange={(name, value) =>
                    setValues((current) => ({ ...current, [name]: value }))
                  }
                />
                {(provider.id || values.id?.[0]) && (
                  <div>
                    <p className="mb-1 text-xs text-muted">{t("Callback URL")}</p>
                    <p className="break-all select-text font-mono text-xs">
                      {appURL(`/api/auth/${provider.id || values.id[0]}/callback`)}
                    </p>
                  </div>
                )}
                {error && (
                  <p role="alert" className="text-sm text-danger">
                    {error}
                  </p>
                )}
              </Modal.Body>
              <Modal.Footer className={dialogStyles.footer}>
                <Button variant="tertiary" isDisabled={pending} onPress={onClose}>
                  {t("Cancel")}
                </Button>
                <Button type="submit" isPending={pending} isDisabled={pending}>
                  {t("Save")}
                </Button>
              </Modal.Footer>
            </form>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
