import { Button } from "@heroui/react";
import { Fingerprint } from "lucide-react";
import { useState } from "react";
import { t } from "../../lib/i18n";
import { AuthLayout } from "./AuthLayout";
import { LoginCredentials } from "./LoginCredentials";
import { useResource } from "../../hooks/useResource";
import { appURL } from "../../lib/url";
import { signInWithPasskey } from "../../lib/passkeys";

export interface LoginMethods {
  password: boolean;
  token: boolean;
  passkey: boolean;
  providers: { id: string; name: string; issuer: string }[];
}
export function Login({ onSuccess }: { onSuccess: () => Promise<void> }) {
  const methods = useResource<LoginMethods>("/api/auth/methods");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const passkeyEnabled = methods.data?.passkey && typeof PublicKeyCredential !== "undefined";
  const credentialsEnabled = methods.data?.password || methods.data?.token;
  const twoFactor = new URLSearchParams(location.search).has("two_factor");
  return (
    <AuthLayout>
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold tracking-tight">{t("Sign in to Headdeck")}</h1>
        {(credentialsEnabled || twoFactor) && (
          <LoginCredentials
            passwordEnabled={methods.data?.password ?? false}
            tokenEnabled={methods.data?.token ?? false}
            onSuccess={onSuccess}
          />
        )}
        {(credentialsEnabled || twoFactor) &&
          (!!methods.data?.providers.length || passkeyEnabled) && (
            <div className="flex items-center gap-4 text-xs text-muted">
              <span className="h-px flex-1 bg-border" />
              {t("Or")}
              <span className="h-px flex-1 bg-border" />
            </div>
          )}
        <div className="space-y-3">
          {methods.data?.providers.map((provider) => (
            <Button
              key={provider.id}
              fullWidth
              variant="outline"
              onPress={() => location.assign(appURL(`/api/auth/${provider.id}/start`))}
            >
              {t("Sign in with {0}", t(provider.name))}
            </Button>
          ))}
          {passkeyEnabled && (
            <Button
              fullWidth
              variant="outline"
              isPending={pending}
              isDisabled={pending}
              onPress={async () => {
                setPending(true);
                setError(undefined);
                try {
                  await signInWithPasskey();
                  await onSuccess();
                } catch (reason) {
                  setError(reason instanceof Error ? t(reason.message) : t("Sign-in failed"));
                } finally {
                  setPending(false);
                }
              }}
            >
              <Fingerprint size={17} />
              {t("Use Passkey")}
            </Button>
          )}
        </div>
        {(error || methods.error || new URLSearchParams(location.search).has("login_error")) && (
          <p role="alert" className="text-sm text-danger">
            {error || methods.error || t("Sign-in failed")}
          </p>
        )}
      </div>
    </AuthLayout>
  );
}
