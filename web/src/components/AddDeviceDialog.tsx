import {
  Accordion,
  Alert,
  Button,
  Input,
  Label,
  ListBox,
  Modal,
  Select,
  Switch,
  Tabs,
  TextField,
} from "@heroui/react";
import { Check, Copy } from "lucide-react";
import { useRef, useState } from "react";
import { t } from "../lib/i18n";
import type { User } from "../lib/types";
import { Fields } from "./Fields";
import { dialogStyles } from "./dialogStyles";

const commandTextFields = [
  {
    name: "hostname",
    label: "Device name",
    placeholder: "Device hostname",
  },
  {
    name: "advertise-routes",
    label: "Advertise subnet routes",
    placeholder: "192.168.1.0/24,10.0.0.0/8",
  },
];
const commandChoiceFields = [
  { name: "accept-dns", label: "Use network DNS", choices: ["default", "true", "false"] },
  { name: "accept-routes", label: "Accept subnet routes", choices: ["default", "true", "false"] },
];
const commandChoiceLabels: Record<string, string> = {
  default: "Default",
  true: "Enabled",
  false: "Disabled",
};
const commandSwitchFields = [
  { name: "advertise-exit-node", label: "Offer as an exit node", control: "switch" },
  { name: "ssh", label: "Enable Tailscale SSH", control: "switch" },
];
const commandFields = [...commandTextFields, ...commandChoiceFields, ...commandSwitchFields];

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
  const [options, setOptions] = useState<Record<string, string>>({
    "accept-dns": "default",
    "accept-routes": "default",
  });
  const [copiedCommand, setCopiedCommand] = useState("");
  const [copyError, setCopyError] = useState(false);
  const command = [
    `tailscale up --login-server='${loginServer.replaceAll("'", "'\\''")}'`,
    ...commandFields.flatMap((field) => {
      const value = options[field.name]?.trim();
      if (!value || ("choices" in field && value === "default")) return [];
      if ("control" in field) return value === "true" ? [`--${field.name}`] : [];
      return [`--${field.name}='${value.replaceAll("'", "'\\''")}'`];
    }),
  ].join(" ");
  const copied = copiedCommand === command;

  function changeOption(name: string, value: string) {
    setOptions((current) => ({ ...current, [name]: value }));
    setCopyError(false);
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setCopiedCommand(command);
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
      <pre className="max-h-24 overflow-y-auto rounded-xl border border-border bg-default p-3 text-xs leading-5 whitespace-pre-wrap text-foreground wrap-anywhere select-text">
        <code>{command}</code>
      </pre>
      <Accordion hideSeparator className="rounded-xl border border-border">
        <Accordion.Item id="options">
          <Accordion.Heading>
            <Accordion.Trigger className="min-h-10 px-3 py-2 text-sm text-foreground">
              {t("Additional options")}
              <Accordion.Indicator />
            </Accordion.Trigger>
          </Accordion.Heading>
          <Accordion.Panel>
            <Accordion.Body className="border-t border-border p-0">
              <div className="grid gap-3 p-3 sm:grid-cols-2">
                {commandTextFields.map((field) => (
                  <TextField
                    key={field.name}
                    isDisabled={pending || method !== "command"}
                    value={options[field.name] ?? ""}
                    onChange={(value) => changeOption(field.name, value)}
                    className="min-w-0 gap-1.5"
                  >
                    <Label className="text-xs text-muted">{t(field.label)}</Label>
                    <Input
                      className="h-9 min-w-0 rounded-lg border border-border text-sm shadow-none"
                      placeholder={t(field.placeholder)}
                      autoComplete="off"
                      spellCheck={false}
                    />
                  </TextField>
                ))}
              </div>
              <div className="divide-y divide-border border-t border-border">
                {commandChoiceFields.map((field) => (
                  <Select
                    key={field.name}
                    isDisabled={pending || method !== "command"}
                    value={options[field.name] ?? "default"}
                    onChange={(value) => changeOption(field.name, String(value))}
                    className="flex flex-row items-center justify-between gap-3 px-3 py-1.5"
                  >
                    <Label className="min-w-0 text-sm">{t(field.label)}</Label>
                    <Select.Trigger className="h-8 min-h-8 w-24 shrink-0 items-center rounded-lg border border-border py-0 ps-2 pe-7 text-sm shadow-none">
                      <Select.Value className="text-sm leading-5" />
                      <Select.Indicator />
                    </Select.Trigger>
                    <Select.Popover>
                      <ListBox>
                        {field.choices.map((value) => (
                          <ListBox.Item
                            key={value}
                            id={value}
                            textValue={t(commandChoiceLabels[value])}
                          >
                            {t(commandChoiceLabels[value])}
                            <ListBox.ItemIndicator />
                          </ListBox.Item>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                ))}
                {commandSwitchFields.map((field) => (
                  <Switch
                    key={field.name}
                    isDisabled={pending || method !== "command"}
                    isSelected={options[field.name] === "true"}
                    onChange={(value) => changeOption(field.name, String(value))}
                    className="w-full px-3 py-1.5"
                  >
                    <Switch.Content className="min-h-8 w-full justify-between gap-3">
                      <Label className="text-sm">{t(field.label)}</Label>
                      <Switch.Control className="shrink-0">
                        <Switch.Thumb />
                      </Switch.Control>
                    </Switch.Content>
                  </Switch>
                ))}
              </div>
            </Accordion.Body>
          </Accordion.Panel>
        </Accordion.Item>
      </Accordion>
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
