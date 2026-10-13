import { Alert, Button, Input, Label, Modal, Spinner, Tabs, TextField } from "@heroui/react";
import { useEffect, useRef, useState } from "react";
import { dialogStyles } from "../../components/dialogStyles";
import { t } from "../../lib/i18n";
import { ACLDestinations, GrantServices, PolicySelect, protocols } from "./PolicyControls";
import { PolicyPicker } from "./PolicyPicker";
import type { PolicyEntry, PolicyOption } from "./policyOptions";
import { aclDestination, targetOptions, validAddress, validPorts } from "./policyOptions";
import { policyTitle, sectionLabel } from "./PolicySections";

export function PolicyEditor({
  entry,
  options,
  loading,
  catalogError,
  sshUsers,
  onSave,
  onClose,
}: {
  entry: PolicyEntry;
  options: PolicyOption[];
  loading: boolean;
  catalogError?: string;
  sshUsers: string[];
  onSave: (values: Record<string, string[]>) => Promise<void>;
  onClose: () => void;
}) {
  const [values, setValues] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(entry.fields.map((field) => [field.name, field.values ?? []])),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const [narrow, setNarrow] = useState(() => window.matchMedia("(width < 640px)").matches);
  const [targetTab, setTargetTab] = useState("src");
  useEffect(() => {
    const media = window.matchMedia("(width < 640px)");
    const update = () => setNarrow(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  const submitting = useRef(false);
  const section = entry.item.section;
  const rules = ["acls", "grants", "ssh"].includes(section);
  const prefix = section === "groups" ? "group:" : section === "tagOwners" ? "tag:" : "";
  function change(name: string, next: string[]) {
    setValues((current) => ({ ...current, [name]: next }));
    setError(undefined);
  }
  async function submit() {
    if (submitting.current || loading || catalogError) return;
    const next = Object.fromEntries(
      entry.fields.map((field) => [
        field.name,
        field.multi
          ? (values[field.name] ?? []).map((value) => value.trim()).filter(Boolean)
          : [(values[field.name]?.[0] ?? "").trim()],
      ]),
    );
    if (rules && (!next.src?.length || !next.dst?.length)) {
      setError(t("Choose at least one source and destination."));
      return;
    }
    if (section === "acls" && next.dst.some((value) => !validPorts(aclDestination(value).ports))) {
      setError(t("Enter ports between 0 and 65535, or a valid range."));
      return;
    }
    if (section === "grants") {
      if (entry.item.add && !next.ip?.length) {
        setError(t("Add at least one service."));
        return;
      }
      if (
        next.ip?.some(
          (value) => !validPorts(value.includes(":") ? value.slice(value.indexOf(":") + 1) : value),
        )
      ) {
        setError(t("Enter ports between 0 and 65535, or a valid range."));
        return;
      }
    }
    if (
      section === "acls" &&
      !["", "tcp", "udp", "sctp", "6", "17", "132"].includes(next.proto[0]) &&
      next.dst.some((value) => aclDestination(value).ports !== "*")
    ) {
      setError(t("This protocol requires all ports."));
      return;
    }
    if (section === "ssh" && !next.users?.length) {
      setError(t("Choose at least one SSH username."));
      return;
    }
    if (
      section === "ssh" &&
      next.src.some((value) => value.startsWith("tag:") || value === "autogroup:tagged") &&
      (next.action[0] === "check" || next.dst.some((value) => !value.startsWith("tag:")))
    ) {
      setError(t("Tagged sources require tagged destinations and the Allow action."));
      return;
    }
    if (!rules && section !== "autoApprovers.exitNode") {
      const name = next.name?.[0] ?? "";
      if (!name) {
        setError(t("Name is required."));
        return;
      }
      if (prefix) next.name = [name.startsWith(prefix) ? name : `${prefix}${name}`];
      if (section === "autoApprovers.routes" && !validAddress(name, true)) {
        setError(t("Enter a valid subnet prefix."));
        return;
      }
      if (section === "hosts" && !validAddress(next.value?.[0] ?? "")) {
        setError(t("Enter a valid IP address or subnet."));
        return;
      }
    }
    submitting.current = true;
    setPending(true);
    setError(undefined);
    try {
      await onSave(next);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }
  const nameLabel =
    section === "groups" ? "Group name" : section === "tagOwners" ? "Tag name" : "Name";
  const sshOptions = [
    { value: "autogroup:nonroot", label: t("Any non-root user"), kind: "SSH users" },
    { value: "root", label: "root", kind: "SSH users" },
    ...sshUsers
      .filter((value) => value && value !== "autogroup:nonroot" && value !== "root")
      .map((value) => ({ value, label: value, kind: "SSH users" })),
  ];
  const sources = (
    <PolicyPicker
      label="Sources"
      options={targetOptions(options, section, "src")}
      values={values.src ?? []}
      onChange={(next) => change("src", next)}
      disabled={pending}
      addresses={section !== "ssh"}
    />
  );
  const destinations =
    section === "acls" ? (
      <ACLDestinations
        options={targetOptions(options, section, "dst")}
        values={values.dst ?? []}
        onChange={(next) => change("dst", next)}
        disabled={pending}
      />
    ) : (
      <PolicyPicker
        label="Destinations"
        options={targetOptions(options, section, "dst")}
        values={values.dst ?? []}
        onChange={(next) => change("dst", next)}
        disabled={pending}
        addresses={section !== "ssh"}
      />
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
        <Modal.Container size="lg" className="policy-editor-container">
          <Modal.Dialog
            className={`${dialogStyles.dialog} policy-editor ${rules ? "sm:max-w-208" : ""}`}
          >
            {!pending && <Modal.CloseTrigger aria-label={t("Close")} />}
            <form
              noValidate
              className="contents"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>
                  {entry.item.add
                    ? t("Add {0}", t(sectionLabel(section)))
                    : t("Edit {0}", policyTitle(entry.item))}
                </Modal.Heading>
                <p className="hidden text-sm text-muted sm:block">
                  {t("Select users, devices and services to build this policy.")}
                </p>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                {loading && <Spinner size="sm" aria-label={t("Loading")} />}
                {catalogError && (
                  <p role="alert" className="text-sm text-danger">
                    {catalogError}
                  </p>
                )}
                {!rules &&
                  section !== "autoApprovers.exitNode" &&
                  (section === "autoApprovers.routes" ? (
                    <PolicyAddress
                      label="Subnet"
                      value={values.name?.[0] ?? ""}
                      options={options.filter((option) => option.kind === "Subnets")}
                      disabled={pending}
                      onChange={(value) => change("name", [value])}
                    />
                  ) : (
                    <TextField
                      isRequired
                      isDisabled={pending}
                      value={(values.name?.[0] ?? "").replace(new RegExp(`^${prefix}`), "")}
                      onChange={(value) => change("name", [value])}
                    >
                      <Label>{t(nameLabel)}</Label>
                      <Input
                        autoComplete="off"
                        className="rounded-xl border border-border shadow-none"
                      />
                    </TextField>
                  ))}
                {section === "ssh" && (
                  <PolicySelect
                    label="Action"
                    value={values.action?.[0] || "accept"}
                    options={[
                      { value: "accept", label: "accept" },
                      { value: "check", label: "check" },
                    ]}
                    onChange={(value) => change("action", [value])}
                    disabled={pending}
                  />
                )}
                {rules &&
                  (narrow ? (
                    <Tabs
                      className="policy-target-tabs min-w-0 gap-4"
                      selectedKey={targetTab}
                      onSelectionChange={(key) => setTargetTab(String(key))}
                    >
                      <Tabs.ListContainer>
                        <Tabs.List aria-label={t("Rule targets")} className="w-full">
                          <Tabs.Tab id="src" className="flex-1 gap-2">
                            {t("Sources")}{" "}
                            <span className="text-xs tabular-nums text-muted">
                              {values.src?.filter(Boolean).length || 0}
                            </span>
                            <Tabs.Indicator />
                          </Tabs.Tab>
                          <Tabs.Tab id="dst" className="flex-1 gap-2">
                            {t("Destinations")}{" "}
                            <span className="text-xs tabular-nums text-muted">
                              {values.dst?.filter(Boolean).length || 0}
                            </span>
                            <Tabs.Indicator />
                          </Tabs.Tab>
                        </Tabs.List>
                      </Tabs.ListContainer>
                      <div className="grid min-w-0">
                        <Tabs.Panel
                          id="src"
                          shouldForceMount
                          className="col-start-1 row-start-1 min-w-0 p-0 data-inert:invisible"
                        >
                          {sources}
                        </Tabs.Panel>
                        <Tabs.Panel
                          id="dst"
                          shouldForceMount
                          className="col-start-1 row-start-1 min-w-0 p-0 data-inert:invisible"
                        >
                          {destinations}
                        </Tabs.Panel>
                      </div>
                    </Tabs>
                  ) : (
                    <div className="grid min-w-0 items-start gap-5 sm:grid-cols-2">
                      {sources}
                      {destinations}
                    </div>
                  ))}
                {section === "acls" && (
                  <PolicySelect
                    label="Protocol"
                    value={values.proto?.[0] || "any"}
                    options={[...new Set([...protocols, values.proto?.[0] ?? ""])].map((value) => ({
                      value: value || "any",
                      label: value ? value.toUpperCase() : "Any protocol",
                    }))}
                    onChange={(value) => change("proto", [value === "any" ? "" : value])}
                    disabled={pending}
                  />
                )}
                {section === "grants" && (
                  <GrantServices
                    values={values.ip ?? []}
                    onChange={(next) => change("ip", next)}
                    disabled={pending}
                  />
                )}
                {section === "ssh" && (
                  <>
                    <PolicyPicker
                      label="SSH users"
                      options={[
                        ...new Map(sshOptions.map((option) => [option.value, option])).values(),
                      ]}
                      values={values.users ?? []}
                      onChange={(next) => change("users", next)}
                      disabled={pending}
                      custom
                    />
                    {values.action?.[0] === "check" && (
                      <PolicySelect
                        label="Check period (optional)"
                        value={values.checkPeriod?.[0] || "default"}
                        options={[
                          ...new Set([
                            "",
                            "1h",
                            "12h",
                            "24h",
                            "168h",
                            values.checkPeriod?.[0] ?? "",
                          ]),
                        ].map((value) => ({
                          value: value || "default",
                          label:
                            value === "1h"
                              ? "1 hour"
                              : value === "12h"
                                ? "12 hours"
                                : value === "24h"
                                  ? "1 day"
                                  : value === "168h"
                                    ? "7 days"
                                    : value || "Default",
                        }))}
                        onChange={(value) =>
                          change("checkPeriod", [value === "default" ? "" : value])
                        }
                        disabled={pending}
                      />
                    )}
                  </>
                )}
                {!rules &&
                  (section === "hosts" ? (
                    <PolicyAddress
                      label="IP address or prefix"
                      value={values.value?.[0] ?? ""}
                      options={options.filter(
                        (option) => option.kind === "Devices" || option.kind === "Subnets",
                      )}
                      onChange={(value) => change("value", [value])}
                      disabled={pending}
                    />
                  ) : (
                    <PolicyPicker
                      label={entry.fields.find((field) => field.name === "value")!.label}
                      options={targetOptions(options, section, "value")}
                      values={values.value ?? []}
                      onChange={(next) => change("value", next)}
                      disabled={pending}
                    />
                  ))}
                {error && (
                  <Alert
                    status="danger"
                    role="alert"
                    className="rounded-xl border border-danger/20 bg-danger-soft shadow-none"
                  >
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
                <Button
                  type="submit"
                  isDisabled={pending || loading || !!catalogError}
                  isPending={pending}
                >
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

function PolicyAddress({
  label,
  value,
  options,
  onChange,
  disabled,
}: {
  label: string;
  value: string;
  options: PolicyOption[];
  onChange: (value: string) => void;
  disabled: boolean;
}) {
  const [editingCustom, setCustom] = useState<boolean>();
  const custom = editingCustom ?? !options.some((option) => option.value === value);
  return (
    <div className="space-y-2">
      <PolicySelect
        label={label}
        value={custom ? "custom" : value}
        options={[
          ...options.map((option) => ({
            value: option.value,
            label:
              option.label === option.value ? option.label : `${option.label} · ${option.value}`,
          })),
          { value: "custom", label: "Custom address or subnet" },
        ]}
        disabled={disabled}
        onChange={(next) => {
          setCustom(next === "custom");
          if (next !== "custom") onChange(next);
        }}
      />
      {custom && (
        <Input
          aria-label={t(label)}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          disabled={disabled}
          required
          autoComplete="off"
          placeholder="192.168.1.0/24"
          className="rounded-xl border border-border shadow-none"
        />
      )}
    </div>
  );
}
