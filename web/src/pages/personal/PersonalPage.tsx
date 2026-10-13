import { Link2, LockKeyhole, MonitorSmartphone, UserRound } from "lucide-react";
import { Tabs } from "@heroui/react";
import { useEffect, useState } from "react";
import { Page, Panel } from "../../components/Page";
import { SecurityMethods } from "../../components/SecurityMethods";
import type { Security, Setup } from "../../components/SecurityMethods";
import { SecurityDialog } from "../../components/SecurityDialog";
import { useResource } from "../../hooks/useResource";
import { useDialogs } from "../../hooks/useDialogs";
import { t } from "../../lib/i18n";
import type { LoginMethods } from "../auth/Login";
import { PasswordForm } from "./PasswordForm";
import { SessionDevices } from "./SessionDevices";
import { Profile } from "./Profile";

const tabs = [
  { id: "profile", label: "Profile", icon: UserRound },
  { id: "security", label: "Password & security", icon: LockKeyhole },
  { id: "linked", label: "Linked accounts", icon: Link2 },
  { id: "devices", label: "Signed-in devices", icon: MonitorSmartphone },
] as const;
type Tab = (typeof tabs)[number]["id"];

export function PersonalPage() {
  const resource = useResource<Security>("/api/account/security");
  const methods = useResource<LoginMethods>("/api/auth/methods");
  const [tab, setTab] = useState<Tab>(() =>
    new URLSearchParams(location.search).has("account_security") ? "linked" : "profile",
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(() =>
    new URLSearchParams(location.search).has("binding_error")
      ? t("Account binding failed")
      : undefined,
  );
  const [totp, setTOTP] = useState<{ mode: "totp" | "disable"; setup?: Setup }>();
  const { openConfirm, dialogs } = useDialogs();
  useEffect(() => {
    if (new URLSearchParams(location.search).has("account_security"))
      history.replaceState(null, "", location.pathname + "#/personal");
  }, []);
  async function run(action: () => Promise<void>) {
    if (pending) return;
    setPending(true);
    setError(undefined);
    try {
      await action();
    } catch (reason) {
      setError(reason instanceof Error ? t(reason.message) : t("Request failed"));
    } finally {
      setPending(false);
    }
  }
  const data = resource.data;
  return (
    <Page
      title="Personal settings"
      error={error || resource.error || methods.error}
      loading={resource.loading}
    >
      <Tabs
        className="max-w-6xl gap-6"
        selectedKey={tab}
        onSelectionChange={(key) => setTab(key as Tab)}
      >
        <Tabs.ListContainer>
          <Tabs.List aria-label={t("Personal settings")}>
            {tabs.map(({ id, label, icon: Icon }) => (
              <Tabs.Tab key={id} id={id}>
                <Icon size={16} />
                {t(label)}
                <Tabs.Indicator />
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="profile" className="view-enter space-y-5">
          {data && <Profile key={data.account.label} data={data} refresh={resource.refresh} />}
        </Tabs.Panel>
        <Tabs.Panel id="security" className="view-enter space-y-5">
          {data && (
            <>
              <PasswordForm data={data} refresh={resource.refresh} />
              <Panel title="Security" flush>
                <SecurityMethods
                  data={data}
                  providers={methods.data}
                  pending={pending}
                  run={run}
                  refresh={resource.refresh}
                  section="mfa"
                  showMain={() => {}}
                  showPassword={() => {}}
                  showTOTP={(setup) => setTOTP({ mode: setup ? "totp" : "disable", setup })}
                  confirm={(title, action) =>
                    openConfirm({
                      title: t(title),
                      description: t("Confirm {0}?", t(title)),
                      onConfirm: action,
                    })
                  }
                />
              </Panel>
            </>
          )}
        </Tabs.Panel>
        <Tabs.Panel id="linked" className="view-enter space-y-5">
          {data && (
            <Panel title="Linked accounts" flush>
              <SecurityMethods
                data={data}
                providers={methods.data}
                pending={pending}
                run={run}
                refresh={resource.refresh}
                section="linked"
                showMain={() => {}}
                showPassword={() => {}}
                showTOTP={() => {}}
                confirm={(title, action) =>
                  openConfirm({
                    title: t(title),
                    description: t("Confirm {0}?", t(title)),
                    onConfirm: action,
                  })
                }
              />
            </Panel>
          )}
        </Tabs.Panel>
        <Tabs.Panel id="devices" className="view-enter space-y-5">
          <SessionDevices />
        </Tabs.Panel>
      </Tabs>
      {totp && (
        <SecurityDialog
          initialMode={totp.mode}
          initialSetup={totp.setup}
          onUpdated={resource.refresh}
          onClose={() => setTOTP(undefined)}
        />
      )}
      {dialogs}
    </Page>
  );
}
