export type Role = "pending" | "member" | "admin" | "owner";

export interface ErrorBody {
  code: string;
  message: string;
  request_id?: string;
  retryable?: boolean;
}

export interface Session {
  authenticated: boolean;
  continue_url?: string;
  device_registration?: boolean;
  login_server?: string;
  subject?: string;
  label?: string;
  avatar_url?: string;
  headscale_user_id?: string;
  role?: Role;
  permissions?: string[];
}

export interface HeadscaleStatus {
  configured: boolean;
  base_url?: string;
  reachable: boolean;
  version?: string;
  api_key_prefix?: string;
  api_key_expiry?: string;
  api_key_expires_in?: string;
  api_key_expired?: boolean;
  last_error?: ErrorBody;
  latency_ms?: number;
}

export interface Status {
  server: {
    state: string;
    rss_bytes: number | null;
    version: string;
    commit: string;
    platform: string;
    uptime: string;
    socket: string;
    pid: number;
    event_subscribers: number;
  };
  principal: { kind: string; id: string; role: Role; permissions: string[] };
  headscale: HeadscaleStatus;
  service_control: {
    supported: boolean;
    driver?: string;
    unit?: string;
    message?: string;
    state?: {
      active_state: string;
      sub_state: string;
      main_pid: number;
      rss_bytes: number | null;
      uptime?: string;
      can_reload: boolean;
    };
  };
  authorization: {
    path: string;
    users: number;
    missing_file: boolean;
    error?: string;
  };
  protocol_version: number;
}

export interface Ref {
  id?: string;
  name?: string;
}

export interface Node {
  id: string;
  name: string;
  hostname?: string;
  created_at?: string;
  os?: string;
  client?: string;
  client_version?: string;
  ephemeral?: boolean;
  given_name: string;
  user: Ref;
  ip_addresses: string[];
  online: boolean;
  last_seen?: string;
  expiry?: string;
  expired: boolean;
  register_method?: string;
  forced_tags?: string[];
  available_routes?: string[];
  valid_tags?: string[];
  approved_routes?: string[];
}

export interface User {
  id: string;
  name: string;
  provider?: string;
  provider_identifier?: string;
  display_name?: string;
  email?: string;
  created_at?: string;
}

export interface Route {
  node_id: string;
  node_name?: string;
  user?: Ref;
  prefix: string;
  available: boolean;
  approved: boolean;
  enabled: boolean;
}

export interface PreAuthKey {
  id: string;
  key?: string;
  user: Ref;
  reusable: boolean;
  ephemeral: boolean;
  used: boolean;
  acl_tags?: string[];
  expiration?: string;
  expired: boolean;
}

export interface Operation {
  id: string;
  state: "pending" | "running" | "succeeded" | "failed";
  message?: string;
  error?: ErrorBody;
  steps?: { name: string; message?: string; state: string }[];
}
export interface Setting {
  key: string;
  value: string;
  secret?: boolean;
  configured?: boolean;
}
export interface Settings {
  items: Setting[];
  reload_required?: boolean;
  writable?: boolean;
  pending?: boolean;
  revision?: string;
}
export interface DNSConfig {
  magic_dns: boolean;
  base_domain: string;
  override_local_dns: boolean;
  nameservers: { global: string[]; split: Record<string, string[]> };
  search_domains: string[];
  extra_records: { name: string; type: string; value: string }[];
}
export interface DNSSettings {
  config: DNSConfig;
  revision: string;
  writable: boolean;
  pending: boolean;
  restart_required: boolean;
  external_records: boolean;
}
export interface PolicySettings {
  policy: string;
  mode: string;
  revision: string;
  writable: boolean;
  pending: boolean;
}
export interface PolicyItem {
  section: string;
  key: string;
  index: number;
  add: boolean;
  title: string;
  summary: string;
}
export interface Field {
  name: string;
  label: string;
  values: string[];
  choiceLabels?: Record<string, string>;
  choices?: string[] | null;
  multi?: boolean;
  secret?: boolean;
  required?: boolean;
  help?: string;
  control?: "switch";
}
export interface PolicyObjects {
  settings: PolicySettings;
  items: { item: PolicyItem; fields: Field[] }[];
}
export interface Account {
  username?: string;
  id: string;
  label: string;
  role: Role;
  disabled: boolean;
  headscale_user_id?: string;
}
export interface Identity {
  issuer: string;
  subject: string;
  account_id?: string;
}
export interface Collection<T> {
  revision: number;
  nodes?: T[];
  users?: T[];
}
export interface Diff<T> {
  base_revision: number;
  revision: number;
  upsert?: T[];
  removed?: string[];
}
export interface LiveEvent {
  resource_id?: string;
  instance?: string;
  id: number;
  type: string;
  revision: number;
  data?: unknown;
}
