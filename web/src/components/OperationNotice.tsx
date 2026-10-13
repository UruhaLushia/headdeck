import { t } from "../lib/i18n";
import { Button, Spinner } from "@heroui/react";
import { X } from "lucide-react";
import { useLive } from "../hooks/useLive";
import { useResource } from "../hooks/useResource";
import type { Operation } from "../lib/types";

export function OperationNotice({
  operation,
  onClose,
}: {
  operation: Operation;
  onClose: () => void;
}) {
  const resource = useResource<Operation>(`/api/operations/${operation.id}`);
  useLive((event) => {
    if (event.type !== "operations.changed" || event.resource_id !== operation.id) return;
    const next = event.data as Operation | undefined;
    if (next) resource.setData(next);
    else void resource.refresh();
  });
  const current = resource.data ?? operation;
  const active = current.state === "pending" || current.state === "running";
  return (
    <div
      role="status"
      className="fixed right-4 bottom-4 z-20 w-[min(380px,calc(100vw-32px))] rounded-2xl border border-border bg-surface p-4 shadow-xl"
    >
      <div className="flex items-start gap-3">
        {active && <Spinner size="sm" />}
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">
            {active
              ? t("Operation in progress")
              : current.state === "succeeded"
                ? t("Operation completed")
                : t("Operation failed")}
          </p>
          <p className="mt-1 wrap-break-word text-xs text-muted">
            {resource.error || current.error?.message || current.message}
          </p>
        </div>
        <Button
          size="sm"
          isIconOnly
          variant="tertiary"
          aria-label={t("Dismiss operation status")}
          onPress={onClose}
        >
          <X size={15} />
        </Button>
      </div>
    </div>
  );
}
