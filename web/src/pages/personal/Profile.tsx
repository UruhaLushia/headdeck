import { Button, Input, Label, TextField } from "@heroui/react";
import { useState } from "react";
import { Panel } from "../../components/Page";
import type { Security } from "../../components/SecurityMethods";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";

export function Profile({ data, refresh }: { data: Security; refresh: () => Promise<void> }) {
  const [label, setLabel] = useState(data.account.label);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  return (
    <Panel title="Profile" flush>
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          if (pending) return;
          setPending(true);
          setError(undefined);
          try {
            await api.put("/api/account/profile", { label });
            await refresh();
            window.dispatchEvent(new Event("headdeck:session-updated"));
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
            value={label}
            onChange={setLabel}
          >
            <Label>{t("Display name")}</Label>
            <Input
              className="h-10 w-full max-w-xl rounded-xl border border-border shadow-none"
              autoComplete="nickname"
            />
          </TextField>
          <dl className="space-y-3 text-sm">
            {[
              ["Username", data.account.username || t("Not set")],
              ["Role", t(data.account.role)],
            ].map(([name, value]) => (
              <div
                key={name}
                className="grid min-h-10 grid-cols-[6rem_minmax(0,1fr)] items-center gap-4 sm:grid-cols-[9rem_minmax(0,1fr)]"
              >
                <dt className="text-muted">{t(name)}</dt>
                <dd className="min-w-0 wrap-anywhere">{value}</dd>
              </div>
            ))}
          </dl>
          {error && (
            <p role="alert" className="text-sm text-danger">
              {error}
            </p>
          )}
        </div>
        <div className="flex justify-end border-t border-border px-5 py-3">
          <Button
            type="submit"
            isPending={pending}
            isDisabled={pending || label.trim() === data.account.label}
          >
            {t("Save")}
          </Button>
        </div>
      </form>
    </Panel>
  );
}
