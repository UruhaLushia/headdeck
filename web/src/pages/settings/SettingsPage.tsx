import { t } from "../../lib/i18n";
import { Tabs } from "@heroui/react";
import { useState } from "react";
import { DraftActions } from "../../components/DraftActions";
import { Page } from "../../components/Page";
import { SettingsList } from "./SettingsList";
import { LoginProviders } from "./LoginProviders";
import { CategoryLayout, CategoryNav } from "../../components/Categories";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import { settingGroup } from "../../lib/settings";
import type { Operation, Setting, Settings } from "../../lib/types";

export function SettingsPage({
  permissions,
  onOperation,
}: {
  permissions: string[];
  onOperation: (operation: Operation) => void;
}) {
  const [mode, setMode] = useState<"headdeck" | "headscale">("headdeck");
  const [selectedGroup, setSelectedGroup] = useState<string>();
  const path = mode === "headscale" ? "/api/headscale/config" : "/api/settings";
  const resource = useResource<Settings>(path);
  const settings = resource.data;
  const showProviders = mode === "headdeck" && permissions.includes("accounts.read");
  const items = settings?.items.filter(
    (item) =>
      !showProviders ||
      (!item.key.startsWith("server.web.oidc.") && !item.key.startsWith("server.web.github.")),
  );
  const writable =
    permissions.includes("settings.write") && (mode === "headdeck" || !!settings?.writable);
  const groups: string[] = [
    ...new Set(items?.map((item) => settingGroup(item.key))),
    ...(showProviders ? ["Login providers"] : []),
  ].toSorted((a, b) => {
    const order = [
      "Server",
      "IP allocation",
      "DERP",
      "Node lifecycle",
      "Web",
      "Login providers",
      "Service control",
      "Login",
      "GitHub login",
      "Logging",
    ];
    return order.indexOf(a) - order.indexOf(b);
  });
  const group = selectedGroup && groups.includes(selectedGroup) ? selectedGroup : groups[0];
  async function save(item: Setting, value: string) {
    const body =
      mode === "headscale"
        ? { values: { [item.key]: value }, revision: settings?.revision }
        : { changes: { [item.key]: value } };
    resource.setData(await api.patch<Settings>(path, body));
  }
  return (
    <Page
      title="Settings"
      description="Manage Headdeck and Headscale settings."
      error={resource.error}
      loading={resource.loading}
      refresh={resource.refresh}
    >
      <Tabs
        variant="secondary"
        className="gap-5"
        selectedKey={mode}
        onSelectionChange={(key) => {
          setMode(key as "headdeck" | "headscale");
          setSelectedGroup(undefined);
        }}
      >
        <Tabs.ListContainer>
          <Tabs.List aria-label={t("Settings")}>
            <Tabs.Tab id="headdeck">
              Headdeck
              <Tabs.Indicator />
            </Tabs.Tab>
            <Tabs.Tab id="headscale">
              Headscale
              <Tabs.Indicator />
            </Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        {(["headdeck", "headscale"] as const).map((value) => (
          <Tabs.Panel key={value} id={value} className="min-w-0 space-y-5">
            {mode === value && (
              <>
                {mode === "headscale" && settings && (
                  <DraftActions
                    pending={!!settings.pending}
                    writable={!!settings.writable}
                    canEdit={permissions.includes("settings.write")}
                    discard={() => api.delete(path)}
                    refresh={resource.refresh}
                    onOperation={onOperation}
                    apply={
                      permissions.includes("service.manage")
                        ? () =>
                            api.post<Operation>(`${path}/apply`, {
                              revision: settings.revision,
                            })
                        : undefined
                    }
                  />
                )}
                {settings && group && (
                  <CategoryLayout>
                    <CategoryNav
                      label="Setting categories"
                      groups={[
                        { items: groups.map((category) => ({ id: category, label: category })) },
                      ]}
                      selected={group}
                      onSelect={setSelectedGroup}
                    />
                    <div key={`${mode}:${group}`} className="view-enter min-w-0">
                      {group === "Login providers" ? (
                        <LoginProviders writable={permissions.includes("accounts.write")} />
                      ) : (
                        <SettingsList
                          items={items?.filter((item) => settingGroup(item.key) === group) ?? []}
                          group={group}
                          writable={writable}
                          save={save}
                        />
                      )}
                    </div>
                  </CategoryLayout>
                )}
              </>
            )}
          </Tabs.Panel>
        ))}
      </Tabs>
    </Page>
  );
}
