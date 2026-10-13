import { Alert, Avatar, Dropdown } from "@heroui/react";
import { LogOut, UserRound } from "lucide-react";
import { useState } from "react";
import { t } from "../lib/i18n";

export function AccountMenu({
  label,
  role,
  avatarURL,
  onSignOut,
}: {
  label?: string;
  role: string;
  avatarURL?: string;
  onSignOut: () => Promise<void>;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const name = label || t(role);
  const avatar = (
    <Avatar size="sm" color="accent" variant="soft">
      {avatarURL && (
        <Avatar.Image key={avatarURL} src={avatarURL} alt="" referrerPolicy="no-referrer" />
      )}
      <Avatar.Fallback>{Array.from(name)[0]?.toUpperCase() || "H"}</Avatar.Fallback>
    </Avatar>
  );
  async function signOut() {
    setPending(true);
    setError(undefined);
    try {
      await onSignOut();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("Request failed"));
    } finally {
      setPending(false);
    }
  }
  return (
    <>
      <Dropdown>
        <Dropdown.Trigger
          className="rounded-full p-1 transition-colors hover:bg-default focus-visible:outline-2 focus-visible:outline-accent"
          aria-label={t("Account menu")}
        >
          {avatar}
        </Dropdown.Trigger>
        <Dropdown.Popover placement="bottom end" className="w-60 rounded-xl">
          <div className="flex min-w-0 items-center gap-3 border-b border-border px-3 py-3">
            {avatar}
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{name}</p>
              <p className="mt-0.5 text-xs text-muted">{t(role)}</p>
            </div>
          </div>
          <Dropdown.Menu
            aria-label={t("Account menu")}
            disabledKeys={pending ? ["logout"] : []}
            onAction={(key) => {
              if (key === "personal") location.hash = "/personal";
              if (key === "logout") void signOut();
            }}
          >
            <Dropdown.Item id="personal" textValue={t("Personal settings")}>
              <UserRound size={16} />
              {t("Personal settings")}
            </Dropdown.Item>
            <Dropdown.Item id="logout" textValue={t("Sign out")} className="text-danger">
              <LogOut size={16} />
              {t("Sign out")}
            </Dropdown.Item>
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
      {error && (
        <Alert status="danger" className="absolute top-16 right-4 max-w-sm">
          <Alert.Content>
            <Alert.Description>{error}</Alert.Description>
          </Alert.Content>
        </Alert>
      )}
    </>
  );
}
