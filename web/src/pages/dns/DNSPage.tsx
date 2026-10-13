import { t } from "../../lib/i18n";
import { Button } from "@heroui/react";
import { Pencil } from "lucide-react";
import { useState } from "react";
import { DNSLists } from "./DNSLists";
import { DNSGeneral } from "./DNSGeneral";
import { CategoryLayout, CategoryNav } from "../../components/Categories";
import { DraftActions } from "../../components/DraftActions";
import { Page } from "../../components/Page";
import { useDialogs } from "../../hooks/useDialogs";
import { useResource } from "../../hooks/useResource";
import { api } from "../../lib/api";
import type { DNSConfig, DNSSettings, Field, Operation } from "../../lib/types";

type DNSSection = "magic" | "global" | "split" | "search" | "records";
const clean = (values: string[]) => values.map((value) => value.trim()).filter(Boolean);

export function DNSPage({
  permissions,
  onOperation,
}: {
  permissions: string[];
  onOperation: (operation: Operation) => void;
}) {
  const resource = useResource<DNSSettings>("/api/dns");
  const [section, setSection] = useState<DNSSection>("magic");
  const { openForm, openConfirm, dialogs } = useDialogs();
  const settings = resource.data;
  const writable = !!settings?.writable && permissions.includes("dns.write");
  const config = settings?.config;
  async function save(edit: (draft: DNSConfig) => void) {
    if (!settings) return;
    const draft = structuredClone(settings.config);
    edit(draft);
    resource.setData(
      await api.put<DNSSettings>("/api/dns", {
        config: draft,
        revision: settings.revision,
      }),
    );
  }
  function form(
    title: string,
    fields: Field[],
    edit: (draft: DNSConfig, values: Record<string, string[]>) => void,
  ) {
    openForm({
      title,
      fields,
      onSubmit: (values) => save((draft) => edit(draft, values)),
    });
  }
  function remove(title: string, edit: (draft: DNSConfig) => void) {
    openConfirm({
      title: "Delete DNS entry",
      description: t("Remove {0} from the DNS draft?", title),
      destructive: true,
      onConfirm: () => save(edit),
    });
  }
  function split(domain = "") {
    form(
      domain ? t("Edit split DNS") : t("Add split DNS"),
      [
        { name: "domain", label: "Domain", values: [domain], required: true },
        {
          name: "servers",
          label: "Nameservers",
          values: config?.nameservers.split[domain] ?? [],
          multi: true,
        },
      ],
      (draft, values) => {
        const next = values.domain[0].trim();
        if (next !== domain && next in draft.nameservers.split)
          throw new Error("This domain already exists.");
        delete draft.nameservers.split[domain];
        draft.nameservers.split[next] = clean(values.servers);
      },
    );
  }
  function record(index = -1) {
    const entry = config?.extra_records[index];
    form(
      entry ? t("Edit DNS record") : t("Add DNS record"),
      [
        {
          name: "name",
          label: "Name",
          values: [entry?.name ?? ""],
          required: true,
        },
        {
          name: "address",
          label: "IP address",
          values: [entry?.value ?? ""],
          required: true,
        },
      ],
      (draft, values) => {
        const value = values.address[0].trim();
        const next = {
          name: values.name[0].trim(),
          type: value.includes(":") ? "AAAA" : "A",
          value,
        };
        if (index < 0) draft.extra_records.push(next);
        else draft.extra_records[index] = next;
      },
    );
  }
  const editButton = (label: string, edit: () => void) =>
    writable && (
      <Button size="sm" variant="tertiary" isIconOnly aria-label={label} onPress={edit}>
        <Pencil size={15} />
      </Button>
    );
  return (
    <Page
      title="DNS"
      description="Configure MagicDNS, resolvers and custom records."
      error={resource.error}
      loading={resource.loading}
      refresh={resource.refresh}
    >
      {settings && (
        <DraftActions
          pending={settings.pending}
          writable={settings.writable}
          canEdit={permissions.includes("dns.write")}
          restart={settings.restart_required}
          discard={() => api.delete("/api/dns")}
          refresh={resource.refresh}
          onOperation={onOperation}
          apply={
            permissions.includes("service.manage")
              ? () =>
                  api.post<Operation>("/api/dns/apply", {
                    revision: settings.revision,
                  })
              : undefined
          }
        />
      )}
      {config && (
        <CategoryLayout>
          <CategoryNav
            label="Setting categories"
            groups={[
              {
                items: [
                  { id: "magic", label: "MagicDNS" },
                  { id: "global", label: "Global nameservers" },
                  { id: "split", label: "Split DNS" },
                  { id: "search", label: "Search domains" },
                  { id: "records", label: "DNS records" },
                ],
              },
            ]}
            selected={section}
            onSelect={(next) => setSection(next as DNSSection)}
          />
          <div key={section} className="view-enter min-w-0">
            {section === "split" || section === "records" ? (
              <DNSLists
                config={config}
                section={section}
                writable={writable}
                editButton={editButton}
                split={split}
                record={record}
                remove={remove}
              />
            ) : (
              <DNSGeneral config={config} section={section} writable={writable} save={save} />
            )}
          </div>
        </CategoryLayout>
      )}
      {dialogs}
    </Page>
  );
}
