import { Button, Input } from "@heroui/react";
import { Trash2 } from "lucide-react";
import { t } from "../lib/i18n";

export function ListEditor({
  label,
  values,
  onChange,
  disabled = false,
}: {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-2">
      {[...values, ""].map((value, index) => (
        <div key={index} className="flex items-center gap-2">
          <Input
            aria-label={`${label} ${index + 1}`}
            autoComplete="off"
            className="min-w-0 flex-1 rounded-xl border border-border shadow-none"
            disabled={disabled}
            value={value}
            onChange={(event) => {
              const next = [...values];
              next[index] = event.target.value;
              onChange(next);
            }}
          />
          <div className="flex size-9 shrink-0 items-center justify-center">
            {index < values.length && (
              <Button
                aria-label={t("Remove {0} {1}", label, index + 1)}
                isIconOnly
                size="sm"
                variant="tertiary"
                isDisabled={disabled}
                onPress={() => onChange(values.filter((_, i) => i !== index))}
              >
                <Trash2 size={16} />
              </Button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
