import { t } from "../../lib/i18n";
import { Button } from "@heroui/react";
import { Plus } from "lucide-react";
import { useState } from "react";
import { DraftActions } from "../../components/DraftActions";
import { Page } from "../../components/Page";
import { CategoryLayout, CategoryPanel } from "../../components/Categories";
import { useDialogs } from "../../hooks/useDialogs";
import { useCollection } from "../../hooks/useCollection";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import type { Node, Operation, PolicyObjects, Route, User } from "../../lib/types";
import { PolicyEditor } from "./PolicyEditor";
import { policyOptions } from "./policyOptions";
import type { PolicyEntry } from "./policyOptions";
import { PolicyList } from "./PolicyList";
import { PolicySections, policyTitle, sectionLabel } from "./PolicySections";

export function PolicyPage({
  permissions,
  onOperation,
}: {
  permissions: string[];
  onOperation: (operation: Operation) => void;
}) {
  const resource = useResource<PolicyObjects>("/api/policy/objects");
  const users = useCollection<User>("users");
  const nodes = useCollection<Node>("nodes");
  const routes = useResource<{ routes: Route[] }>("/api/routes");
  const { openConfirm, dialogs } = useDialogs();
  const [editor, setEditor] = useState<{ entry: PolicyEntry; revision: string }>();
  const [selected, setSelected] = useState("acls");
  const settings = resource.data?.settings;
  const writable = !!settings?.writable && permissions.includes("policy.write");
  const items = resource.data?.items ?? [];
  const section = items.some(({ item }) => item.section === selected)
    ? selected
    : (items[0]?.item.section ?? selected);
  const entries = items.filter(({ item }) => item.section === section && !item.add);
  const add = items.find(({ item }) => item.section === section && item.add);
  const options = policyOptions(items, users.items, nodes.items, routes.data?.routes ?? []);
  function edit(entry: PolicyObjects["items"][number]) {
    setEditor({ entry, revision: settings!.revision });
  }
  function remove(entry: PolicyObjects["items"][number]) {
    openConfirm({
      title: "Delete policy item",
      description: t("Remove {0} from this policy draft?", policyTitle(entry.item)),
      destructive: true,
      onConfirm: async () => {
        await api.post("/api/policy/objects", {
          item: entry.item,
          values: {},
          revision: settings?.revision,
          remove: true,
        });
        await resource.refresh();
      },
    });
  }
  return (
    <Page
      title="Access control"
      description="Define who can reach your devices, routes and services."
      error={resource.error ?? users.error ?? nodes.error ?? routes.error}
      loading={resource.loading}
      refresh={async () => {
        await Promise.all([resource.refresh(), users.refresh(), nodes.refresh(), routes.refresh()]);
      }}
    >
      {settings && (
        <DraftActions
          pending={settings.pending}
          writable={settings.writable}
          canEdit={permissions.includes("policy.write")}
          restart={settings.mode === "file"}
          discard={() => api.delete("/api/policy")}
          refresh={resource.refresh}
          onOperation={onOperation}
          apply={
            permissions.includes("service.manage")
              ? () =>
                  api.post<Operation>("/api/policy/apply", {
                    revision: settings.revision,
                  })
              : undefined
          }
        />
      )}
      {settings && (
        <CategoryLayout>
          <PolicySections selected={section} items={items} onSelect={setSelected} />
          <div key={section} className="view-enter min-w-0">
            <CategoryPanel
              title={sectionLabel(section)}
              actions={
                writable &&
                add && (
                  <Button size="sm" variant="secondary" onPress={() => edit(add)}>
                    <Plus size={15} />
                    {t("Add")}
                  </Button>
                )
              }
            >
              <PolicyList
                section={section}
                entries={entries}
                writable={writable}
                edit={edit}
                remove={remove}
                options={options}
              />
            </CategoryPanel>
          </div>
        </CategoryLayout>
      )}
      {dialogs}
      {editor && (
        <PolicyEditor
          entry={editor.entry}
          options={options}
          loading={users.loading || nodes.loading || routes.loading}
          catalogError={users.error ?? nodes.error ?? routes.error}
          sshUsers={items
            .filter(({ item }) => item.section === "ssh")
            .flatMap(({ fields }) => fields.find((field) => field.name === "users")?.values ?? [])}
          onClose={() => setEditor(undefined)}
          onSave={async (values) => {
            await api.post("/api/policy/objects", {
              item: editor.entry.item,
              values,
              revision: editor.revision,
              remove: false,
            });
            await resource.refresh();
          }}
        />
      )}
    </Page>
  );
}
