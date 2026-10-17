import type { ComponentType } from "react";
import type { PageID } from "../components/Layout";
import type { Operation } from "./types";

const pageLoaders = {
  personal: () =>
    import("../pages/personal/PersonalPage").then((m) => ({ default: m.PersonalPage })),
  overview: () =>
    import("../pages/overview/OverviewPage").then((m) => ({ default: m.OverviewPage })),
  nodes: () => import("../pages/nodes/NodesPage").then((m) => ({ default: m.NodesPage })),
  users: () => import("../pages/users/UsersPage").then((m) => ({ default: m.UsersPage })),
  keys: () => import("../pages/keys/KeysPage").then((m) => ({ default: m.KeysPage })),
  dns: () => import("../pages/dns/DNSPage").then((m) => ({ default: m.DNSPage })),
  policy: () => import("../pages/policy/PolicyPage").then((m) => ({ default: m.PolicyPage })),
  accounts: () =>
    import("../pages/accounts/AccountsPage").then((m) => ({ default: m.AccountsPage })),
  settings: () =>
    import("../pages/settings/SettingsPage").then((m) => ({ default: m.SettingsPage })),
  diagnostics: () =>
    import("../pages/diagnostics/DiagnosticsPage").then((m) => ({ default: m.DiagnosticsPage })),
  logs: () => import("../pages/logs/LogsPage").then((m) => ({ default: m.LogsPage })),
};

type PageComponent = ComponentType<{
  permissions: string[];
  nodeID?: string;
  loginServer?: string;
  onOperation: (operation: Operation) => void;
}>;
const components = new Map<PageID, PageComponent>();
const pending = new Map<PageID, Promise<PageComponent>>();

export const getPage = (page: PageID) => components.get(page);

export function loadPage(page: PageID): Promise<PageComponent> {
  const component = components.get(page);
  if (component) return Promise.resolve(component);
  const existing = pending.get(page);
  if (existing) return existing;
  const promise = pageLoaders[page]()
    .then((module) => {
      components.set(page, module.default);
      return module.default;
    })
    .finally(() => {
      pending.delete(page);
    });
  pending.set(page, promise);
  return promise;
}
