import { t } from "../lib/i18n";
import { Alert, Button, Spinner } from "@heroui/react";
import { RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

export function Page({
  title,
  description,
  children,
  actions,
  error,
  loading,
  refresh,
}: {
  title: ReactNode;
  description?: string;
  children: ReactNode;
  actions?: ReactNode;
  error?: string;
  loading?: boolean;
  refresh?: () => Promise<void>;
}) {
  return (
    <section className="page-view view-enter min-w-0 space-y-5">
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1">
        <h1 className="min-w-0 text-xl font-semibold tracking-tight sm:text-2xl">
          {typeof title === "string" ? t(title) : title}
        </h1>
        <div className="page-actions flex items-center gap-2">
          {loading && <Spinner size="sm" />}
          {actions}
          {refresh && (
            <Button
              variant="tertiary"
              isIconOnly
              aria-label={t("Refresh")}
              onPress={() => void refresh()}
            >
              <RefreshCw size={17} />
            </Button>
          )}
        </div>
        {description && <p className="col-span-2 text-sm text-muted">{t(description)}</p>}
      </div>
      {error && (
        <Alert
          role="alert"
          status="danger"
          className="rounded-xl border border-danger/20 bg-danger-soft shadow-none"
        >
          <Alert.Content>
            <Alert.Description className="wrap-anywhere text-danger">{error}</Alert.Description>
          </Alert.Content>
        </Alert>
      )}
      {children}
    </section>
  );
}

export function Panel({
  title,
  children,
  flush = false,
}: {
  title?: string;
  children: ReactNode;
  flush?: boolean;
}) {
  return (
    <div className="min-w-0 overflow-hidden rounded-2xl border border-border bg-surface">
      {title && (
        <h2 className="border-b border-border px-5 py-3 text-base font-semibold">{t(title)}</h2>
      )}
      {flush ? children : <div className="p-5">{children}</div>}
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-2xl border border-border bg-surface px-5 py-8 text-center text-sm text-muted">
      {typeof children === "string" ? t(children) : children}
    </div>
  );
}
export function DataTable({ columns, children }: { columns: string[]; children: ReactNode }) {
  return (
    <div className="relative min-w-0 overflow-x-auto rounded-2xl border border-border bg-surface">
      <table
        role="table"
        className={`resource-table w-full text-left text-sm ${columns.includes("") ? "resource-table-actions" : ""}`}
      >
        <thead className="border-b border-border text-muted">
          <tr>
            {columns.map((column) => (
              <th key={column} className="font-medium">
                {column ? t(column) : <span className="sr-only">{t("Actions")}</span>}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border">{children}</tbody>
      </table>
    </div>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "success" | "danger" | "neutral";
}) {
  const style = {
    success: "text-success",
    danger: "text-danger",
    neutral: "rounded-md bg-default px-2 py-0.5 text-muted",
  };
  return (
    <span
      className={`inline-flex items-center gap-1.5 text-xs font-medium whitespace-nowrap ${style[tone]}`}
    >
      {tone !== "neutral" && (
        <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-current" />
      )}
      {typeof children === "string" ? t(children) : children}
    </span>
  );
}
