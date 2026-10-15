import { Button } from "@heroui/react";
import { Fingerprint, Link2, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { Badge } from "./Page";
import { api } from "../lib/api";
import { t } from "../lib/i18n";
import { registerPasskey } from "../lib/passkeys";
import type { Account } from "../lib/types";
import type { LoginMethods } from "../pages/auth/Login";
export interface Security {
  account: Account & { username?: string };
  password: boolean;
  totp: boolean;
  identities: { issuer: string; subject: string }[];
  passkeys: { id: string; name: string }[];
}
export interface Setup {
  id: string;
  secret: string;
  qr: string;
}

export function SecurityMethods({
  data,
  providers,
  pending,
  run,
  refresh,
  confirm,
  showMain,
  showPassword,
  showTOTP,
  section = "all",
}: {
  data: Security;
  providers?: LoginMethods;
  pending: boolean;
  run: (action: () => Promise<void>) => Promise<void>;
  refresh: () => Promise<void>;
  confirm: (title: string, action: () => Promise<void>) => void;
  showMain: () => void;
  showPassword: () => void;
  showTOTP: (setup?: Setup) => void;
  section?: "all" | "linked" | "mfa";
}) {
  const rowClass = section === "all" ? "py-2.5" : "px-5 py-2.5";
  return (
    <div className="divide-y divide-border">
      {section === "all" && (
        <section className="py-4">
          <h3 className="mb-2 text-sm font-medium">{t("Password")}</h3>
          <div className="flex items-center justify-between gap-3">
            <span className="text-sm text-muted">{data.account.username || t("Not set")}</span>
            <Button
              size="sm"
              variant="secondary"
              isDisabled={pending}
              onPress={() => {
                showPassword();
              }}
            >
              {t(data.password ? "Change" : "Set password")}
            </Button>
          </div>
        </section>
      )}
      {section !== "mfa" && (
        <section>
          {section === "all" && (
            <h3 className="mb-2 text-sm font-medium">{t("Linked accounts")}</h3>
          )}
          {!data.identities.length && (
            <p className={`${rowClass} text-sm text-muted`}>{t("Not set")}</p>
          )}
          <div className="divide-y divide-border">
            {data.identities.map((identity) => (
              <div
                key={`${identity.issuer}:${identity.subject}`}
                className={`flex min-h-16 items-center justify-between gap-4 text-sm ${rowClass}`}
              >
                <Link2 size={20} className="shrink-0 text-muted" />
                <div className="min-w-0 flex-1 space-y-1">
                  <p className="truncate font-medium">
                    {providers?.providers.find((provider) => provider.issuer === identity.issuer)
                      ?.name ||
                      (identity.issuer === "https://github.com" ? "GitHub" : "OIDC / OAuth")}
                  </p>
                  <p className="truncate text-xs text-muted">{identity.subject}</p>
                </div>
                <Button
                  isIconOnly
                  size="sm"
                  variant="tertiary"
                  isDisabled={pending}
                  aria-label={t("Unlink")}
                  onPress={() =>
                    confirm("Unlink account", async () => {
                      await api.delete("/api/account/identities", {
                        issuer: identity.issuer,
                        subject: identity.subject,
                      });
                      await refresh();
                      showMain();
                    })
                  }
                >
                  <Trash2 size={16} />
                </Button>
              </div>
            ))}
          </div>
          {providers?.providers.length ? (
            <div className={`flex flex-wrap justify-end gap-2 border-t border-border ${rowClass}`}>
              {providers?.providers.map((provider) => (
                <Button
                  key={provider.id}
                  size="sm"
                  variant="secondary"
                  isDisabled={pending}
                  onPress={() =>
                    void run(async () => {
                      const result = await api.post<{ url: string }>(
                        `/api/account/providers/${provider.id}/bind`,
                      );
                      location.assign(result.url);
                    })
                  }
                >
                  <Plus size={14} />
                  {provider.name}
                </Button>
              ))}
            </div>
          ) : null}
        </section>
      )}
      {section !== "linked" && (
        <section>
          <div className={`flex min-h-16 items-center justify-between gap-4 ${rowClass}`}>
            <h3 className="flex items-center gap-3 text-sm font-medium">
              <Fingerprint size={20} className="shrink-0 text-muted" />
              Passkey
            </h3>
            {providers?.passkey && typeof PublicKeyCredential !== "undefined" && (
              <Button
                size="sm"
                variant="secondary"
                isDisabled={pending}
                onPress={() =>
                  void run(async () => {
                    await registerPasskey();
                    await refresh();
                  })
                }
              >
                <Plus size={14} />
                {t("Add")}
              </Button>
            )}
          </div>
          {data.passkeys.length > 0 && (
            <div
              className={`mb-4 divide-y divide-border overflow-hidden rounded-xl border border-border ${section === "all" ? "" : "mx-5"}`}
            >
              {data.passkeys.map((key, index) => (
                <div
                  key={key.id}
                  className="flex min-h-16 items-center justify-between gap-4 px-4 py-2.5 text-sm"
                >
                  <div className="min-w-0">
                    <span className="wrap-anywhere font-medium">
                      {key.name === "Passkey" ? t("Passkey {0}", index + 1) : key.name}
                    </span>
                  </div>
                  <Button
                    isIconOnly
                    size="sm"
                    variant="tertiary"
                    isDisabled={pending}
                    aria-label={t("Delete")}
                    onPress={() =>
                      confirm("Delete Passkey", async () => {
                        await api.delete(`/api/account/passkeys/${key.id}`);
                        await refresh();
                        showMain();
                      })
                    }
                  >
                    <Trash2 size={16} />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </section>
      )}
      {section !== "linked" && (
        <section className={`flex min-h-16 items-center justify-between gap-4 ${rowClass}`}>
          <div className="flex items-center gap-3">
            <ShieldCheck size={20} className="shrink-0 text-muted" />
            <div className="flex flex-wrap items-center gap-3">
              <h3 className="text-sm font-medium">2FA</h3>
              <Badge tone={data.totp ? "success" : "neutral"}>
                {t(data.totp ? "Enabled" : "Disabled")}
              </Badge>
            </div>
          </div>
          <Button
            size="sm"
            variant="secondary"
            isDisabled={pending}
            onPress={() =>
              void run(async () => {
                if (data.totp) showTOTP();
                else {
                  showTOTP(await api.post<Setup>("/api/account/totp/start"));
                }
              })
            }
          >
            {t(data.totp ? "Disable" : "Enable")}
          </Button>
        </section>
      )}
    </div>
  );
}
