import type { ErrorBody, Node, PreAuthKey, Route, Session, Status, User } from "./types";

import { appURL } from "./url";
const WRITE_HEADER = { "X-Requested-With": "headdeck" };

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, body: ErrorBody) {
    super(body.message || `request failed with ${status}`);
    this.name = "ApiError";
    this.code = body.code;
    this.status = status;
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (method !== "GET") Object.assign(headers, WRITE_HEADER);
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const response = await fetch(appURL(path), {
    method,
    headers,
    signal: AbortSignal.any([AbortSignal.timeout(20000), ...(signal ? [signal] : [])]),
    credentials: "same-origin",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (response.status === 204) return undefined as T;

  const text = await response.text();
  let parsed: unknown = undefined;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = undefined;
    }
  }

  if (!response.ok) {
    if (response.status === 401 && path !== "/api/session")
      window.dispatchEvent(new Event("headdeck:unauthenticated"));
    if (response.status === 403 && path !== "/api/session")
      window.dispatchEvent(new Event("headdeck:session-updated"));
    const envelope = parsed as { error?: ErrorBody } | undefined;
    const errorBody = envelope?.error ?? {
      code: "INTERNAL",
      message: text || response.statusText,
    };
    throw new ApiError(response.status, errorBody);
  }
  return parsed as T;
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>("GET", path, undefined, signal),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, body),
  delete: <T>(path: string, body?: unknown) => request<T>("DELETE", path, body),
  session: () => request<Session>("GET", "/api/session"),
  login: (token: string) => request<Session>("POST", "/api/session", { token }),
  logout: () => request<void>("DELETE", "/api/session"),

  status: (refresh = false) => request<Status>("GET", `/api/status${refresh ? "?refresh=1" : ""}`),
  nodes: () => request<{ nodes: Node[] }>("GET", "/api/nodes"),
  users: () => request<{ users: User[] }>("GET", "/api/users"),
  routes: () => request<{ routes: Route[] }>("GET", "/api/routes"),
  preAuthKeys: () => request<{ keys: PreAuthKey[] }>("GET", "/api/preauth-keys"),
};
