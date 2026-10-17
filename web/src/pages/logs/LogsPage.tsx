import { Input, ListBox, Modal, Select, Tabs } from "@heroui/react";
import { useState } from "react";
import { dialogStyles } from "../../components/dialogStyles";
import { DataTable, Empty, Page } from "../../components/Page";
import { JsonCode } from "../../components/JsonCode";
import { useResource } from "../../hooks/useResource";
import { t } from "../../lib/i18n";

interface LogEntry {
  time: string;
  level: string;
  message: string;
}

interface LogSnapshot {
  source: string;
  collected_at: string;
  entries: LogEntry[];
}

const levels = [
  { id: "all", label: "All levels" },
  { id: "error", label: "Error" },
  { id: "warn", label: "Warning" },
  { id: "info", label: "Info" },
  { id: "debug", label: "Debug" },
];

const colors: Record<string, string> = {
  error: "text-danger",
  warn: "text-warning",
  info: "text-accent",
  debug: "text-muted",
};

function formatLog(entry: LogEntry): string {
  const metadata = { time: entry.time, level: entry.level };
  try {
    const data: unknown = JSON.parse(entry.message);
    if (data && typeof data === "object" && !Array.isArray(data)) {
      return JSON.stringify({ ...metadata, ...data }, null, 2);
    }
  } catch {
    // Service-manager messages and older application logs can be plain text.
  }
  return JSON.stringify({ ...metadata, msg: entry.message }, null, 2);
}

export function LogsPage() {
  const [source, setSource] = useState("headdeck");
  const [level, setLevel] = useState("all");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<LogEntry>();
  const resource = useResource<LogSnapshot>(`/api/logs/${source}`);
  const query = search.trim().toLocaleLowerCase();
  const entries = resource.data?.entries.filter(
    (entry) =>
      (level === "all" || entry.level === level) &&
      (!query || entry.message.toLocaleLowerCase().includes(query)),
  );

  return (
    <Page
      title="Logs"
      description="View the latest 200 service logs, newest first."
      error={resource.error ? t(resource.error) : undefined}
      loading={resource.loading}
      refresh={resource.refresh}
    >
      <Tabs
        variant="secondary"
        selectedKey={source}
        onSelectionChange={(key) => setSource(String(key))}
        className="gap-5"
      >
        <Tabs.ListContainer>
          <Tabs.List aria-label={t("Log source")}>
            {["headdeck", "headscale"].map((id) => (
              <Tabs.Tab key={id} id={id} className="w-auto">
                {id === "headdeck" ? "Headdeck" : "Headscale"}
                <Tabs.Indicator />
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </Tabs.ListContainer>
        {["headdeck", "headscale"].map((id) => (
          <Tabs.Panel key={id} id={id} className="min-w-0 space-y-5 p-0">
            {source === id && (
              <>
                <div className="flex flex-wrap items-center gap-3">
                  <Input
                    aria-label={t("Search logs")}
                    placeholder={t("Search logs…")}
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    className="w-full sm:max-w-sm"
                  />
                  <Select
                    aria-label={t("Log level")}
                    value={level}
                    onChange={(value) => setLevel(String(value ?? "all"))}
                    className="w-full sm:w-40"
                  >
                    <Select.Trigger>
                      <Select.Value />
                      <Select.Indicator />
                    </Select.Trigger>
                    <Select.Popover>
                      <ListBox>
                        {levels.map((item) => (
                          <ListBox.Item key={item.id} id={item.id} textValue={t(item.label)}>
                            {t(item.label)}
                            <ListBox.ItemIndicator />
                          </ListBox.Item>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                </div>
                {!!entries?.length && (
                  <DataTable columns={["Time", "Log level", "Message"]}>
                    {entries.map((entry, index) => (
                      <tr key={`${entry.time}-${index}`}>
                        <td data-label={t("Time")} className="align-top sm:whitespace-nowrap">
                          <time dateTime={entry.time} className="text-muted tabular-nums">
                            {new Date(entry.time).toLocaleString()}
                          </time>
                        </td>
                        <td data-label={t("Log level")} className="align-top sm:min-w-24">
                          <span
                            className={`inline-flex items-center gap-1.5 text-xs font-medium whitespace-nowrap ${colors[entry.level] ?? "text-muted"}`}
                          >
                            <span
                              aria-hidden
                              className="size-1.5 shrink-0 rounded-full bg-current"
                            />
                            {t(
                              levels.find((item) => item.id === entry.level)?.label ?? entry.level,
                            )}
                          </span>
                        </td>
                        <td
                          data-label={t("Message")}
                          className="mobile-table-wide w-full min-w-0 align-top sm:max-w-0"
                        >
                          <button
                            type="button"
                            aria-haspopup="dialog"
                            onClick={() => setSelected(entry)}
                            className="block w-full cursor-pointer truncate rounded-sm text-left font-mono focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
                          >
                            {entry.message}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </DataTable>
                )}
                {resource.data && !entries?.length && <Empty>No matching logs.</Empty>}
                {resource.data && (
                  <p className="text-xs text-muted">
                    {t(
                      "{0} logs · Updated {1}",
                      entries?.length ?? 0,
                      new Date(resource.data.collected_at).toLocaleTimeString(),
                    )}
                  </p>
                )}
              </>
            )}
          </Tabs.Panel>
        ))}
      </Tabs>
      <Modal
        isOpen={!!selected}
        onOpenChange={(open) => {
          if (!open) setSelected(undefined);
        }}
      >
        <Modal.Backdrop className={dialogStyles.backdrop}>
          <Modal.Container size="lg">
            <Modal.Dialog className={dialogStyles.dialog}>
              <Modal.CloseTrigger aria-label={t("Close")} />
              <Modal.Header className={dialogStyles.header}>
                <Modal.Heading className={dialogStyles.heading}>{t("Log details")}</Modal.Heading>
              </Modal.Header>
              <Modal.Body className={dialogStyles.body}>
                {selected && <JsonCode code={formatLog(selected)} />}
              </Modal.Body>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal>
    </Page>
  );
}
