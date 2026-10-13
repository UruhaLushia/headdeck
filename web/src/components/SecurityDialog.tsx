import { Button, Modal } from "@heroui/react";
import { ArrowLeft } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { t } from "../lib/i18n";
import type { Field } from "../lib/types";
import type { LoginMethods } from "../pages/auth/Login";
import { dialogStyles } from "./dialogStyles";
import { Fields } from "./Fields";
import { SecurityMethods } from "./SecurityMethods";
import type { Security, Setup } from "./SecurityMethods";

type Mode = "main" | "password" | "totp" | "disable" | "recovery" | "confirm";

export function SecurityDialog({
  onClose,
  initialMode = "main",
  initialSetup,
  onUpdated,
}: {
  onClose: () => void;
  initialMode?: "main" | "totp" | "disable";
  initialSetup?: Setup;
  onUpdated?: () => Promise<void>;
}) {
  const [data, setData] = useState<Security>();
  const [providers, setProviders] = useState<LoginMethods>();
  const [mode, setMode] = useState<Mode>(initialMode);
  const [values, setValues] = useState<Record<string, string[]>>({});
  const [setup, setSetup] = useState<Setup | undefined>(initialSetup);
  const [codes, setCodes] = useState<string[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(() =>
    new URLSearchParams(location.search).has("binding_error") ? t("Account binding failed") : "",
  );
  const [confirmation, setConfirmation] = useState<{
    title: string;
    action: () => Promise<void>;
  }>();
  async function refresh() {
    setData(await api.get<Security>("/api/account/security"));
    await onUpdated?.();
  }
  useEffect(() => {
    let active = true;
    void Promise.all([
      api.get<Security>("/api/account/security"),
      api.get<LoginMethods>("/api/auth/methods"),
    ])
      .then(([security, methods]) => {
        if (active) {
          setData(security);
          setProviders(methods);
        }
      })
      .catch((reason: Error) => {
        if (active) setError(t(reason.message));
      });
    return () => {
      active = false;
    };
  }, []);
  async function run(action: () => Promise<void>) {
    if (pending) return;
    setPending(true);
    setError(undefined);
    try {
      await action();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      setPending(false);
    }
  }
  function confirm(title: string, action: () => Promise<void>) {
    setConfirmation({ title, action });
    setError(undefined);
    setMode("confirm");
  }
  const fields: Field[] =
    mode === "password"
      ? [
          { name: "username", label: "Username", values: [], required: true },
          ...(data?.password
            ? [
                {
                  name: "current_password",
                  label: "Current password",
                  values: [],
                  required: true,
                  secret: true,
                },
              ]
            : []),
          { name: "password", label: "New password", values: [], required: true, secret: true },
        ]
      : [{ name: "code", label: "Verification code", values: [], required: true }];
  const title =
    mode === "main"
      ? "Security"
      : mode === "password"
        ? "Password"
        : mode === "recovery"
          ? "Recovery codes"
          : mode === "confirm"
            ? (confirmation?.title ?? "Confirm")
            : "2FA";
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
              <div className="flex items-center gap-2">
                {mode !== "main" && (
                  <Button
                    isIconOnly
                    size="sm"
                    variant="tertiary"
                    aria-label={t("Back")}
                    isDisabled={pending}
                    onPress={() => {
                      if (initialMode !== "main") {
                        onClose();
                        return;
                      }
                      setMode("main");
                      setError(undefined);
                      setCodes([]);
                      setSetup(undefined);
                    }}
                  >
                    <ArrowLeft size={17} />
                  </Button>
                )}
                <Modal.Heading className={dialogStyles.heading}>{t(title)}</Modal.Heading>
              </div>
            </Modal.Header>
            <Modal.Body className={dialogStyles.body}>
              {mode === "main" && data && (
                <SecurityMethods
                  data={data}
                  providers={providers}
                  pending={pending}
                  run={run}
                  refresh={refresh}
                  confirm={confirm}
                  showMain={() => setMode("main")}
                  showPassword={() => {
                    setValues({ username: [data.account.username || ""], password: [""] });
                    setMode("password");
                  }}
                  showTOTP={(next) => {
                    setValues({ code: [""] });
                    setSetup(next);
                    setMode(next ? "totp" : "disable");
                  }}
                />
              )}
              {mode === "totp" && setup && (
                <div className="space-y-3 text-center">
                  <img
                    src={setup.qr}
                    alt={t("Authenticator QR code")}
                    className="mx-auto size-48 rounded-xl"
                  />
                  <p className="break-all select-text font-mono text-xs">{setup.secret}</p>
                </div>
              )}
              {["password", "totp", "disable"].includes(mode) && (
                <Fields
                  fields={fields}
                  values={values}
                  disabled={pending}
                  onChange={(name, value) =>
                    setValues((current) => ({ ...current, [name]: value }))
                  }
                />
              )}
              {mode === "recovery" && (
                <div className="grid grid-cols-2 gap-2 select-text font-mono text-xs">
                  {codes.map((code) => (
                    <p key={code}>{code}</p>
                  ))}
                </div>
              )}
              {mode === "confirm" && (
                <p className="text-sm">{t("Confirm {0}?", t(confirmation?.title || ""))}</p>
              )}
              {error && (
                <p role="alert" className="text-sm text-danger">
                  {error}
                </p>
              )}
            </Modal.Body>
            {mode !== "main" && (
              <Modal.Footer className={dialogStyles.footer}>
                {mode === "recovery" && (
                  <Button
                    onPress={() =>
                      void run(async () => {
                        await navigator.clipboard.writeText(codes.join("\n"));
                      })
                    }
                  >
                    {t("Copy")}
                  </Button>
                )}
                {["password", "totp", "disable", "confirm"].includes(mode) && (
                  <Button
                    isPending={pending}
                    isDisabled={pending}
                    variant={mode === "confirm" || mode === "disable" ? "danger" : "primary"}
                    onPress={() =>
                      void run(async () => {
                        if (mode === "password") {
                          await api.put("/api/account/password", {
                            username: values.username?.[0],
                            password: values.password?.[0],
                            current_password: values.current_password?.[0],
                          });
                          setValues({});
                          await refresh();
                          setMode("main");
                        }
                        if (mode === "disable") {
                          await api.delete("/api/account/totp", { code: values.code?.[0] });
                          await refresh();
                          if (initialMode !== "main") {
                            onClose();
                            return;
                          }
                          setMode("main");
                        }
                        if (mode === "totp" && setup) {
                          const result = await api.post<{ recovery_codes: string[] }>(
                            "/api/account/totp/finish",
                            { id: setup.id, code: values.code?.[0] },
                          );
                          setCodes(result.recovery_codes);
                          setSetup(undefined);
                          await refresh();
                          setMode("recovery");
                        }
                        if (mode === "confirm") await confirmation?.action();
                      })
                    }
                  >
                    {t(mode === "password" ? "Save" : "Confirm")}
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
