import { Button, Input, Label, ListBox, Modal, Select, TextField } from "@heroui/react";
import { useState } from "react";
import { dialogStyles } from "../../components/dialogStyles";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import type { Account, User } from "../../lib/types";

export function AccountBinding({
  account,
  accounts,
  users,
  onClose,
  refresh,
}: {
  account: Account;
  accounts: Account[];
  users: User[];
  onClose: () => void;
  refresh: () => Promise<void>;
}) {
  const [selected, setSelected] = useState(account.headscale_user_id || "");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const available = users.filter(
    (user) =>
      !accounts.some((item) => item.id !== account.id && item.headscale_user_id === user.id),
  );
  const username = users.find((user) => user.id === selected)?.name || account.username || "";
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
        <Modal.Container>
          <Modal.Dialog className={dialogStyles.dialog}>
            {!pending && <Modal.CloseTrigger aria-label={t("Close")} />}
            <Modal.Header className={dialogStyles.header}>
              <Modal.Heading className={dialogStyles.heading}>
                {t("Headscale binding")}
              </Modal.Heading>
              <p className="mt-1 text-sm text-muted">{account.label}</p>
            </Modal.Header>
            <Modal.Body className={dialogStyles.body}>
              <Select
                value={selected}
                isDisabled={pending}
                onChange={(value) => setSelected(String(value ?? ""))}
              >
                <Label>{t("Headscale user")}</Label>
                <Select.Trigger className="rounded-xl border border-border shadow-none">
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    <ListBox.Item id="" textValue={t("Unlinked")}>
                      {t("Unlinked")}
                      <ListBox.ItemIndicator />
                    </ListBox.Item>
                    {available.map((user) => (
                      <ListBox.Item key={user.id} id={user.id} textValue={user.name}>
                        {user.name}
                        <ListBox.ItemIndicator />
                      </ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
              <TextField value={username} isReadOnly>
                <Label>{t("Username")}</Label>
                <Input className="rounded-xl border border-border shadow-none" />
              </TextField>
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
              <Button
                isPending={pending}
                isDisabled={pending || selected === (account.headscale_user_id || "")}
                onPress={async () => {
                  setPending(true);
                  setError(undefined);
                  try {
                    await api.put(`/api/accounts/${account.id}`, {
                      ...account,
                      headscale_user_id: selected,
                    });
                    await refresh();
                    onClose();
                  } catch (reason) {
                    setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
                  } finally {
                    setPending(false);
                  }
                }}
              >
                {t("Save")}
              </Button>
            </Modal.Footer>
          </Modal.Dialog>
        </Modal.Container>
      </Modal.Backdrop>
    </Modal>
  );
}
