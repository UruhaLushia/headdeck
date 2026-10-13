import { Spinner } from "@heroui/react";
import { lazy, Suspense, useState } from "react";
import { CategoryLayout, CategoryNav } from "../../components/Categories";
import { Empty, Page, Panel } from "../../components/Page";
import { useResource } from "../../hooks/useResource";
import { t } from "../../lib/i18n";
import type { Settings } from "../../lib/types";
import { appURL } from "../../lib/url";
import type { MetricSample } from "./MetricChart";

const MetricChart = lazy(() =>
  import("./MetricChart").then((module) => ({ default: module.MetricChart })),
);

interface Diagnostic {
  section: string;
  collected_at: string;
  format: "text" | "json";
  content?: string;
  samples?: MetricSample[];
}

const views = [
  { id: "metrics", label: "Metrics" },
  { id: "overview", label: "State overview" },
  { id: "batcher", label: "Control connections" },
  { id: "nodestore", label: "Node store" },
  { id: "routes", label: "Primary routes" },
  { id: "derp", label: "DERP map" },
  { id: "policy-manager", label: "Policy manager" },
  { id: "goroutine", label: "Goroutine stacks" },
];

const metrics = [
  { name: "go_goroutines", label: "Goroutines", unit: "count" },
  { name: "process_resident_memory_bytes", label: "Process memory", unit: "bytes" },
  { name: "go_memstats_heap_alloc_bytes", label: "Heap memory", unit: "bytes" },
  { name: "process_cpu_seconds_total", label: "CPU time", unit: "seconds" },
  { name: "process_open_fds", label: "Open file descriptors", unit: "count" },
  { name: "http_requests_inflight", label: "In-flight HTTP requests", unit: "count" },
  { name: "headscale_nodestore_nodes", label: "Stored devices", unit: "count" },
  { name: "headscale_nodestore_queue_depth", label: "Write queue depth", unit: "count" },
];

export function DiagnosticsPage() {
  const [section, setSection] = useState("metrics");
  const settings = useResource<Settings>("/api/settings");
  const enabled = settings.data?.items.some(
    (item) => item.key === "server.diagnostics" && item.value === "true",
  );
  const resource = useResource<Diagnostic>(
    enabled ? `/api/headscale/diagnostics/${section}` : null,
  );
  const data = resource.data;
  return (
    <Page
      title="Debug"
      description="Inspect runtime metrics and network state."
      error={settings.error ?? (enabled ? resource.error : undefined)}
      loading={settings.loading || resource.loading}
      refresh={enabled ? resource.refresh : undefined}
    >
      {settings.data && !enabled && <Empty>Debug is disabled.</Empty>}
      {enabled && (
        <p className="text-sm">
          <a
            href={appURL("/api/debug/pprof/")}
            target="_blank"
            rel="noreferrer"
            className="text-accent hover:underline"
          >
            {t("Headdeck debug (pprof)")}
          </a>
        </p>
      )}
      {enabled && (
        <CategoryLayout>
          <CategoryNav
            label="Debug"
            groups={[{ items: views.slice(0, 1) }, { label: "Debug views", items: views.slice(1) }]}
            selected={section}
            onSelect={setSection}
          />
          <div className="min-w-0 space-y-4">
            {section === "metrics" ? (
              <div className="space-y-3">
                <p className="text-xs text-muted">
                  {t("Trends use collected snapshots from the last hour. Refresh to add a sample.")}
                </p>
                <Suspense fallback={<Spinner size="sm" />}>
                  <div className="grid gap-4 xl:grid-cols-2">
                    {metrics.map((metric) => (
                      <MetricChart key={metric.name} {...metric} samples={data?.samples ?? []} />
                    ))}
                  </div>
                </Suspense>
              </div>
            ) : (
              <Panel title="Debug output">
                <div className="space-y-4">
                  {data ? (
                    <>
                      <p className="text-xs text-muted">
                        {t("Collected at {0}", new Date(data.collected_at).toLocaleTimeString())}
                      </p>
                      <pre
                        className="max-h-[65dvh] overflow-auto rounded-xl border border-border bg-background p-3 text-xs leading-5"
                        tabIndex={0}
                        aria-label={t("Debug output")}
                      >
                        {data.format === "json"
                          ? JSON.stringify(JSON.parse(data.content!), null, 2)
                          : data.content}
                      </pre>
                    </>
                  ) : (
                    <p className="py-8 text-center text-sm text-muted">
                      {t(resource.loading ? "Loading" : "Debug data is unavailable")}
                    </p>
                  )}
                  <p className="text-xs text-muted">{t("Refresh to update the snapshot.")}</p>
                </div>
              </Panel>
            )}
          </div>
        </CategoryLayout>
      )}
    </Page>
  );
}
