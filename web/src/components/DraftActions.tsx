import { t } from "../lib/i18n";
import { Button } from "@heroui/react";
import { useDialogs } from "../hooks/useDialogs";
import type { Operation } from "../lib/types";

export function DraftActions({
  pending,
  writable,
  canEdit = true,
  apply,
  discard,
  refresh,
  onOperation,
  restart = true,
}: {
  pending: boolean;
  writable: boolean;
  canEdit?: boolean;
  apply?: () => Promise<Operation>;
  discard: () => Promise<unknown>;
  refresh: () => Promise<void>;
  onOperation: (operation: Operation) => void;
  restart?: boolean;
}) {
  const { openConfirm, dialogs } = useDialogs();
  if (!pending && writable) return null;
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-warning/20 bg-warning-soft p-4 text-sm">
      <p>
        {writable
          ? t("You have unapplied changes.")
          : t("Configuration is read-only. Authorize management from the terminal.")}
      </p>
      {pending && canEdit && (
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="tertiary"
            onPress={() =>
              openConfirm({
                title: "Discard changes",
                description: "All changes in this draft will be discarded.",
                onConfirm: async () => {
                  await discard();
                  await refresh();
                },
              })
            }
          >
            {t("Discard")}
          </Button>
          {writable && apply && (
            <Button
              size="sm"
              onPress={() =>
                openConfirm({
                  title: "Apply changes",
                  description: restart
                    ? t("Headscale will restart briefly to apply this configuration.")
                    : t("Apply this configuration to the running network."),
                  onConfirm: async () => {
                    onOperation(await apply());
                    await refresh();
                  },
                })
              }
            >
              {t("Apply")}
            </Button>
          )}
        </div>
      )}
      {dialogs}
    </div>
  );
}
