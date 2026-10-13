import { Button, Input, Label, TextField } from "@heroui/react";
import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { Panel } from "../../components/Page";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";
import type { Security } from "../../components/SecurityMethods";

function PasswordInput({
  label,
  value,
  onChange,
  current = false,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  current?: boolean;
  disabled: boolean;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <TextField
      className="grid gap-2 sm:grid-cols-[9rem_minmax(0,1fr)] sm:items-center sm:gap-4"
      isRequired
      isDisabled={disabled}
      value={value}
      onChange={onChange}
    >
      <Label>{t(label)}</Label>
      <div className="relative w-full max-w-xl">
        <Input
          className="h-10 w-full rounded-xl border border-border pr-12 shadow-none"
          type={visible ? "text" : "password"}
          autoComplete={current ? "current-password" : "new-password"}
        />
        <Button
          isIconOnly
          size="sm"
          variant="tertiary"
          className="absolute top-1/2 right-1 -translate-y-1/2"
          aria-label={t(visible ? "Hide password" : "Show password")}
          onPress={() => setVisible(!visible)}
        >
          {visible ? <EyeOff size={16} /> : <Eye size={16} />}
        </Button>
      </div>
    </TextField>
  );
}

export function PasswordForm({ data, refresh }: { data: Security; refresh: () => Promise<void> }) {
  const [username, setUsername] = useState(data.account.username || "");
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState(false);
  return (
    <Panel title={data.password ? "Change password" : "Set password"} flush>
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          if (pending) return;
          setError(undefined);
          setSaved(false);
          if (password !== confirmation) {
            setError(t("Passwords do not match"));
            return;
          }
          setPending(true);
          try {
            await api.put("/api/account/password", {
              username,
              password,
              current_password: current,
            });
            setCurrent("");
            setPassword("");
            setConfirmation("");
            await refresh();
            setSaved(true);
          } catch (reason) {
            setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
          } finally {
            setPending(false);
          }
        }}
      >
        <div className="space-y-3 px-5 py-4">
          <TextField
            className="grid gap-2 sm:grid-cols-[9rem_minmax(0,1fr)] sm:items-center sm:gap-4"
            isRequired
            isDisabled={pending}
            value={username}
            onChange={setUsername}
          >
            <Label>{t("Username")}</Label>
            <Input
              className="h-10 w-full max-w-xl rounded-xl border border-border shadow-none"
              autoComplete="username"
              readOnly={!!data.account.headscale_user_id}
            />
          </TextField>
          {data.password && (
            <PasswordInput
              label="Current password"
              value={current}
              onChange={setCurrent}
              current
              disabled={pending}
            />
          )}
          <PasswordInput
            label="New password"
            value={password}
            onChange={setPassword}
            disabled={pending}
          />
          <PasswordInput
            label="Confirm new password"
            value={confirmation}
            onChange={setConfirmation}
            disabled={pending}
          />
          {error && (
            <p role="alert" className="text-sm text-danger">
              {error}
            </p>
          )}
          {saved && (
            <p role="status" className="text-sm text-success">
              {t("Saved")}
            </p>
          )}
        </div>
        <div className="flex justify-end border-t border-border px-5 py-3">
          <Button type="submit" isPending={pending} isDisabled={pending}>
            {t(data.password ? "Change password" : "Set password")}
          </Button>
        </div>
      </form>
    </Panel>
  );
}
