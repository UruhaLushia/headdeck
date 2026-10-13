import { Alert, Button } from "@heroui/react";
import { useState } from "react";
import { Plus } from "lucide-react";
import { AddDeviceDialog } from "../../components/AddDeviceDialog";
import { AuthLayout } from "./AuthLayout";
import { t } from "../../lib/i18n";

export function PendingAccess({
  label,
  deviceRegistration,
  loginServer,
  onSignOut,
}: {
  label?: string;
  deviceRegistration?: boolean;
  loginServer?: string;
  onSignOut: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [adding, setAdding] = useState(false);
  return (
    <AuthLayout>
      <section className="space-y-6">
        <div className="space-y-3">
          <h1 className="text-2xl font-semibold">
            {t(deviceRegistration ? "Connect a device" : "Waiting for approval")}
          </h1>
          {label && <p className="font-medium">{label}</p>}
        </div>
        {error && (
          <Alert status="danger">
            <Alert.Content>
              <Alert.Description>{error}</Alert.Description>
            </Alert.Content>
          </Alert>
        )}
        {deviceRegistration && loginServer && (
          <Button fullWidth onPress={() => setAdding(true)}>
            <Plus size={16} />
            {t("Add device")}
          </Button>
        )}
        <Button
          fullWidth
          variant="secondary"
          onPress={() => {
            location.hash = "/personal";
          }}
        >
          {t("Personal settings")}
        </Button>
        <Button
          fullWidth
          variant="secondary"
          isPending={busy}
          isDisabled={busy}
          onPress={async () => {
            setBusy(true);
            setError(undefined);
            try {
              await onSignOut();
            } catch (reason) {
              setError(reason instanceof Error ? reason.message : String(reason));
            } finally {
              setBusy(false);
            }
          }}
        >
          {t("Sign out")}
        </Button>
      </section>
      {adding && loginServer && (
        <AddDeviceDialog loginServer={loginServer} onClose={() => setAdding(false)} />
      )}
    </AuthLayout>
  );
}
