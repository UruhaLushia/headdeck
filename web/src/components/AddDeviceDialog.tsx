import { Alert, Button, Input, Label, Modal, Tabs, TextField } from "@heroui/react";
import { Check, Copy } from "lucide-react";
import { useRef, useState } from "react";
import { t } from "../lib/i18n";
import type { User } from "../lib/types";
import { Fields } from "./Fields";
import { dialogStyles } from "./dialogStyles";

export function AddDeviceDialog({
  loginServer,
  users,
  onRegister,
  onClose,
}: {
  loginServer: string;
  users?: User[];
  onRegister?: (key: string, userID?: string) => Promise<void>;
  onClose: () => void;
}) {
  const [method, setMethod] = useState("command");
  const [key, setKey] = useState("");
  const [userID, setUserID] = useState(users?.length === 1 ? users[0].id : "");
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const [error, setError] = useState<string>();
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const command = `tailscale up --login-server='${loginServer.replaceAll("'", "'\\''")}'`;

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setCopyError(false);
    } catch {
      setCopyError(true);
    }
  }

  async function register() {
    if (!onRegister || submitting.current) return;
    submitting.current = true;
    setPending(true);
    setError(undefined);
    try {
      await onRegister(key.trim(), userID || undefined);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  const commandContent = (
    <div className="space-y-4">
      <p className="text-sm text-muted">{t("Run on the device, then sign in in your browser.")}</p>
      <pre className="rounded-xl border border-border bg-default p-4 text-sm whitespace-pre-wrap text-foreground wrap-anywhere select-text">
        <code>{command}</code>
      </pre>
      {copyError && (
        <p className="text-sm text-danger">
          {t("Copy failed. Select and copy the command manually.")}
        </p>
      )}
    </div>
  );

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
                if (method === "nodekey") void register();
                else void copy();
              }}
            >
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>{t("Add device")}</Modal.Heading>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                {onRegister && (
                  <Tabs
                    className="gap-4"
                    selectedKey={method}
                    onSelectionChange={(value) => {
                      setMethod(String(value));
                      setError(undefined);
                    }}
                  >
                    <Tabs.ListContainer>
                      <Tabs.List aria-label={t("Registration method")} className="w-full">
                        <Tabs.Tab id="command" isDisabled={pending} className="flex-1">
                          {t("Command")}
                          <Tabs.Indicator />
                        </Tabs.Tab>
                        <Tabs.Tab id="nodekey" isDisabled={pending} className="flex-1">
                          {t("Node Key")}
                          <Tabs.Indicator />
                        </Tabs.Tab>
                      </Tabs.List>
                    </Tabs.ListContainer>
                    <div className="grid min-w-0">
                      <Tabs.Panel
                        id="command"
                        shouldForceMount
                        className="static col-start-1 row-start-1 min-w-0 p-0 data-inert:invisible"
                      >
                        {commandContent}
                      </Tabs.Panel>
                      <Tabs.Panel
                        id="nodekey"
                        shouldForceMount
                        className="static col-start-1 row-start-1 min-w-0 space-y-4 p-0 data-inert:invisible"
                      >
                        <TextField
                          isRequired
                          isDisabled={pending || method !== "nodekey"}
                          value={key}
                          onChange={setKey}
                        >
                          <Label>{t("Node Key")}</Label>
                          <Input
                            className="rounded-xl border border-border font-mono shadow-none"
                            placeholder="hskey-authreq-… / nodekey:…"
                            autoComplete="off"
                            spellCheck={false}
                            maxLength={256}
                          />
                        </TextField>
                        {users && users.length > 0 && (
                          <Fields
                            fields={[
                              {
                                name: "user",
                                label: "Owner",
                                values: [userID],
                                required: true,
                                choices: users.map((user) => user.id),
                                choiceLabels: Object.fromEntries(
                                  users.map((user) => [user.id, user.name]),
                                ),
                              },
                            ]}
                            values={{ user: [userID] }}
                            disabled={pending || method !== "nodekey"}
                            onChange={(_, value) => setUserID(value[0] ?? "")}
                          />
                        )}
                        <p className="text-sm text-muted">
                          {t("Use the registration key from the device sign-in URL.")}
                        </p>
                      </Tabs.Panel>
                    </div>
                  </Tabs>
                )}
                {!onRegister && commandContent}
                {error && (
                  <Alert status="danger" role="alert">
                    <Alert.Content>
                      <Alert.Description>{error}</Alert.Description>
                    </Alert.Content>
                  </Alert>
                )}
              </Modal.Body>
              <Modal.Footer className={dialogStyles.footer}>
                {method === "command" ? (
                  <Button type="submit">
                    {copied ? <Check size={16} /> : <Copy size={16} />}
                    {t(copied ? "Copied" : "Copy command")}
                  </Button>
                ) : (
                  <Button
                    type="submit"
                    isPending={pending}
                    isDisabled={pending || !key.trim() || (!!users && !userID)}
                  >
                    {t("Add device")}
                  </Button>
                )}
              </Modal.Footer>
            </form>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
