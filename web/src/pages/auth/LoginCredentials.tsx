import { Button, Input, Label, TextField } from "@heroui/react";
import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { api } from "../../lib/api";
import { t } from "../../lib/i18n";

export function LoginCredentials({
  passwordEnabled,
  tokenEnabled,
  onSuccess,
}: {
  passwordEnabled: boolean;
  tokenEnabled: boolean;
  onSuccess: () => Promise<void>;
}) {
  const [useToken, setUseToken] = useState(false);
  const tokenMode = tokenEnabled && (!passwordEnabled || useToken);
  const [twoFactor, setTwoFactor] = useState(() =>
    new URLSearchParams(location.search).has("two_factor"),
  );
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [passwordVisible, setPasswordVisible] = useState(false);
  const [code, setCode] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const ready = twoFactor ? !!code.trim() : !!password && (tokenMode || !!username.trim());
  return (
    <form
      className="space-y-4"
      onSubmit={async (event) => {
        event.preventDefault();
        if (pending || !ready) return;
        setPending(true);
        setError(undefined);
        try {
          if (twoFactor) {
            await api.post("/api/auth/totp", { code });
          } else if (tokenMode) {
            await api.login(password);
          } else {
            const result = await api.post<{ two_factor?: boolean }>("/api/auth/password", {
              username,
              password,
            });
            setPassword("");
            if (result.two_factor) {
              setTwoFactor(true);
              return;
            }
          }
          history.replaceState(null, "", location.pathname + location.hash);
          await onSuccess();
        } catch (reason) {
          setError(reason instanceof Error ? t(reason.message) : t("Sign-in failed"));
          if (twoFactor) {
            setTwoFactor(false);
            setCode("");
            history.replaceState(null, "", location.pathname);
          }
        } finally {
          setPending(false);
        }
      }}
    >
      {twoFactor ? (
        <TextField
          isRequired
          validationBehavior="aria"
          isDisabled={pending}
          value={code}
          onChange={(value) => {
            setCode(value);
            setError(undefined);
          }}
        >
          <Label>{t("Verification code")}</Label>
          <Input autoComplete="one-time-code" autoFocus spellCheck={false} />
        </TextField>
      ) : (
        <>
          {!tokenMode && (
            <TextField
              isRequired
              validationBehavior="aria"
              isDisabled={pending}
              value={username}
              onChange={(value) => {
                setUsername(value);
                setError(undefined);
              }}
            >
              <Label>{t("Username")}</Label>
              <Input autoComplete="username" autoCapitalize="none" spellCheck={false} />
            </TextField>
          )}
          <TextField
            isRequired
            validationBehavior="aria"
            isDisabled={pending}
            value={password}
            onChange={(value) => {
              setPassword(value);
              setError(undefined);
            }}
          >
            <Label>{t(tokenMode ? "Access token" : "Password")}</Label>
            <div className="relative">
              <Input
                className="w-full pr-12"
                type={passwordVisible ? "text" : "password"}
                autoComplete={tokenMode ? "off" : "current-password"}
              />
              <Button
                type="button"
                isIconOnly
                size="sm"
                variant="ghost"
                className="auth-password-toggle absolute top-1/2 right-1 -translate-y-1/2"
                aria-label={t(passwordVisible ? "Hide password" : "Show password")}
                isDisabled={pending}
                onPress={() => setPasswordVisible(!passwordVisible)}
              >
                {passwordVisible ? <EyeOff size={17} /> : <Eye size={17} />}
              </Button>
            </div>
          </TextField>
        </>
      )}
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
      <Button type="submit" fullWidth isPending={pending} isDisabled={pending || !ready}>
        {t(twoFactor ? "Verify" : "Sign in")}
      </Button>
      {passwordEnabled && tokenEnabled && !twoFactor && (
        <Button
          type="button"
          fullWidth
          size="sm"
          variant="tertiary"
          onPress={() => {
            setUseToken(!useToken);
            setPassword("");
            setPasswordVisible(false);
            setError(undefined);
          }}
        >
          {t(tokenMode ? "Use password" : "Use access token")}
        </Button>
      )}
    </form>
  );
}
