import { Alert, Button, Spinner } from "@heroui/react";
import { Suspense, useEffect, useState } from "react";
import { Layout } from "./components/Layout";
import { OperationNotice } from "./components/OperationNotice";
import { LiveProvider } from "./hooks/useLive";
import { ResourceProvider } from "./hooks/ResourceProvider";
import { usePageNavigation } from "./hooks/usePageNavigation";
import { api } from "./lib/api";
import { t, useLocale } from "./lib/i18n";
import type { Operation, Session } from "./lib/types";
import { Login } from "./pages/auth/Login";
import { PendingAccess } from "./pages/auth/PendingAccess";

export default function App() {
  useLocale();
  const [session, setSession] = useState<Session>();
  const [personal, setPersonal] = useState(
    () =>
      location.hash === "#/personal" ||
      new URLSearchParams(location.search).has("account_security"),
  );
  useEffect(() => {
    const navigate = () => setPersonal(location.hash === "#/personal");
    window.addEventListener("hashchange", navigate);
    return () => window.removeEventListener("hashchange", navigate);
  }, []);
  const waiting = !!session?.authenticated && !session.permissions?.length;
  const forceLogin = new URLSearchParams(location.search).has("sign_in");
  useEffect(() => {
    if (!forceLogin && session?.authenticated && session.continue_url)
      location.assign(session.continue_url);
  }, [session, forceLogin]);
  async function load() {
    const url = new URL(location.href);
    url.searchParams.delete("sign_in");
    history.replaceState(null, "", url.pathname + url.search + url.hash);
    try {
      setSession(await api.session());
    } catch {
      setSession({ authenticated: false });
    }
  }
  useEffect(() => {
    const controller = new AbortController();
    let loading = false;
    const refresh = async () => {
      if (loading || controller.signal.aborted) return;
      loading = true;
      try {
        const next = await api.get<Session>("/api/session", controller.signal);
        if (!controller.signal.aborted) setSession(next);
      } catch {
        if (!controller.signal.aborted)
          setSession((current) => current ?? { authenticated: false });
      } finally {
        loading = false;
      }
    };
    void refresh();
    const interval = waiting ? window.setInterval(refresh, 5000) : undefined;
    window.addEventListener("focus", refresh);
    window.addEventListener("headdeck:session-updated", refresh);
    const expired = () => setSession({ authenticated: false });
    window.addEventListener("headdeck:unauthenticated", expired);
    const update = (event: Event) => setSession((event as CustomEvent<Session>).detail);
    window.addEventListener("headdeck:session", update);
    return () => {
      controller.abort();
      window.clearInterval(interval);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("headdeck:session-updated", refresh);
      window.removeEventListener("headdeck:unauthenticated", expired);
      window.removeEventListener("headdeck:session", update);
    };
  }, [waiting]);
  if (!session)
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner />
      </div>
    );
  if (!session.authenticated || forceLogin) return <Login onSuccess={load} />;
  const signOut = async () => {
    await api.logout();
    setSession({ authenticated: false });
  };
  if (!session.permissions?.length && !personal)
    return (
      <PendingAccess
        label={session.label}
        deviceRegistration={session.device_registration}
        loginServer={session.login_server}
        onSignOut={signOut}
      />
    );
  return (
    <LiveProvider
      enabled={session.permissions?.includes("system.read") ?? false}
      key={JSON.stringify([
        session.subject,
        session.role,
        session.headscale_user_id,
        session.permissions,
      ])}
    >
      <ResourceProvider>
        <Dashboard session={session} onSignOut={signOut} />
      </ResourceProvider>
    </LiveProvider>
  );
}

function Dashboard({ session, onSignOut }: { session: Session; onSignOut: () => Promise<void> }) {
  const [operation, setOperation] = useState<Operation>();
  const permissions = session.permissions ?? [];
  const { page, nodeID, Component, error, retry, preload } = usePageNavigation(permissions);
  const shared = {
    permissions,
    nodeID,
    loginServer: session.login_server,
    onOperation: setOperation,
  };
  return (
    <Layout
      page={page}
      permissions={permissions}
      role={session.role ?? ""}
      label={session.label}
      avatarURL={session.avatar_url}
      onSignOut={onSignOut}
      preload={preload}
    >
      {error && (
        <Alert status="danger" className="mb-4">
          <Alert.Content>
            <Alert.Description>{error}</Alert.Description>
          </Alert.Content>
          <Button size="sm" variant="secondary" onPress={retry}>
            {t("Retry")}
          </Button>
        </Alert>
      )}
      <div className="page-content">
        <Suspense
          fallback={
            <div className="grid min-h-64 place-items-center">
              <Spinner />
            </div>
          }
        >
          {Component ? (
            <Component key={page} {...shared} />
          ) : (
            !error && (
              <div className="grid min-h-64 place-items-center">
                <Spinner />
              </div>
            )
          )}
        </Suspense>
      </div>
      {operation && (
        <OperationNotice
          key={operation.id}
          operation={operation}
          onClose={() => setOperation(undefined)}
        />
      )}
    </Layout>
  );
}
