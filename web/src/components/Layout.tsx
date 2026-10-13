import { t } from "../lib/i18n";
import { Drawer } from "@heroui/react";
import { LanguageMenu } from "./LanguageMenu";
import { AccountMenu } from "./AccountMenu";
import { ThemeMenu } from "./ThemeMenu";
import {
  Globe2,
  KeyRound,
  LayoutDashboard,
  Menu,
  Activity,
  Network,
  Settings2,
  ShieldCheck,
  Users,
  UserRoundCog,
} from "lucide-react";
import { useEffect, useState } from "react";
import type { ReactNode } from "react";

export const pages = [
  {
    id: "overview",
    label: "Overview",
    icon: LayoutDashboard,
    permission: "system.read",
  },
  { id: "nodes", label: "Devices", icon: Network, permission: "nodes.read" },
  { id: "users", label: "Users", icon: Users, permission: "users.read" },
  { id: "keys", label: "Auth keys", icon: KeyRound, permission: "keys.read" },
  { id: "dns", label: "DNS", icon: Globe2, permission: "dns.read" },
  {
    id: "policy",
    label: "Access control",
    icon: ShieldCheck,
    permission: "policy.read",
  },
  {
    id: "accounts",
    label: "Access accounts",
    icon: UserRoundCog,
    permission: "accounts.read",
  },
  {
    id: "diagnostics",
    label: "Debug",
    icon: Activity,
    permission: "system.read",
  },
  {
    id: "settings",
    label: "Settings",
    icon: Settings2,
    permission: "settings.read",
  },
] as const;
export type PageID = (typeof pages)[number]["id"] | "personal";

export function Layout({
  page,
  permissions,
  role,
  label,
  avatarURL,
  onSignOut,
  preload,
  children,
}: {
  page: PageID;
  permissions: string[];
  role: string;
  label?: string;
  avatarURL?: string;
  onSignOut: () => Promise<void>;
  preload: (page: PageID) => Promise<void>;
  children: ReactNode;
}) {
  const [mobile, setMobile] = useState(false);
  useEffect(() => {
    if (!mobile) return;
    const desktop = window.matchMedia("(min-width: 1024px)");
    const close = () => setMobile(false);
    const resize = () => {
      if (desktop.matches) close();
    };
    window.addEventListener("hashchange", close);
    desktop.addEventListener("change", resize);
    return () => {
      window.removeEventListener("hashchange", close);
      desktop.removeEventListener("change", resize);
    };
  }, [mobile]);
  const allowed = pages.filter((item) => permissions.includes(item.permission));
  const nav = (
    <nav aria-label={t("Main navigation")} className="space-y-5 px-3 py-4">
      {[
        { label: "Network", ids: ["overview", "nodes", "users", "keys", "dns", "policy"] },
        { label: "Administration", ids: ["accounts", "diagnostics", "settings"] },
      ].map((group) => {
        const items = allowed.filter((item) => group.ids.includes(item.id));
        if (!items.length) return null;
        return (
          <div key={group.label}>
            <p className="mb-2 px-3 text-xs text-muted">{t(group.label)}</p>
            <div className="space-y-1">
              {items.map(({ id, label: pageLabel, icon: Icon }) => (
                <a
                  key={id}
                  href={`#/${id}`}
                  aria-current={id === page ? "page" : undefined}
                  onClick={() => setMobile(false)}
                  onPointerEnter={() => void preload(id)}
                  onFocus={() => void preload(id)}
                  className={`flex min-h-11 items-center gap-3 rounded-xl px-3 py-2 text-sm transition-colors lg:min-h-9 lg:rounded-lg ${id === page ? "bg-accent-soft font-medium text-accent" : "text-muted hover:bg-default hover:text-foreground"}`}
                >
                  <Icon size={18} strokeWidth={1.7} className="shrink-0" />
                  <span className="min-w-0 truncate">{t(pageLabel)}</span>
                </a>
              ))}
            </div>
          </div>
        );
      })}
    </nav>
  );
  return (
    <div className="min-h-dvh bg-background">
      <aside className="fixed inset-y-0 top-14 left-0 hidden w-56 flex-col overflow-y-auto border-r border-border bg-surface lg:flex">
        {nav}
      </aside>
      <header className="fixed inset-x-0 top-0 z-30 flex h-14 items-center justify-between border-b border-border bg-surface px-4">
        <div className="flex items-center gap-2">
          <Drawer isOpen={mobile} onOpenChange={setMobile}>
            <Drawer.Trigger
              aria-label={t("Open navigation")}
              className="flex size-10 items-center justify-center rounded-xl bg-default text-foreground outline-none focus-visible:ring-2 focus-visible:ring-accent lg:hidden"
            >
              <Menu size={20} />
            </Drawer.Trigger>
            <Drawer.Backdrop className="navigation-backdrop">
              <Drawer.Content placement="left">
                <Drawer.Dialog
                  aria-label={t("Main navigation")}
                  className="navigation-drawer"
                  style={{ touchAction: "pan-y" }}
                >
                  <Drawer.Header className="m-0 flex min-h-14 shrink-0 flex-row items-center justify-between border-b border-border px-4 pt-[env(safe-area-inset-top)]">
                    <Drawer.Heading className="text-base font-semibold">Headdeck</Drawer.Heading>
                    <Drawer.CloseTrigger
                      aria-label={t("Close navigation")}
                      className="static size-9 rounded-lg"
                    />
                  </Drawer.Header>
                  <Drawer.Body className="m-0 min-h-0 flex-1 overflow-y-auto overscroll-contain p-0 pb-[env(safe-area-inset-bottom)]">
                    {nav}
                  </Drawer.Body>
                </Drawer.Dialog>
              </Drawer.Content>
            </Drawer.Backdrop>
          </Drawer>
          <a
            href={`#/${allowed[0]?.id ?? "overview"}`}
            className="text-sm font-semibold tracking-tight"
          >
            Headdeck
          </a>
        </div>
        <div className="flex items-center gap-2">
          <LanguageMenu compact />
          <ThemeMenu />
          <AccountMenu label={label} role={role} avatarURL={avatarURL} onSignOut={onSignOut} />
        </div>
      </header>
      <main className="min-w-0 px-4 pt-20 pb-6 sm:px-6 lg:ml-56">
        <div className="mx-auto w-full max-w-360">{children}</div>
      </main>
    </div>
  );
}
