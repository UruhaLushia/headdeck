import type { Field, Setting } from "./types";

const labels: Record<string, string> = {
  "dns.magic_dns": "MagicDNS",
  "dns.base_domain": "Tailnet domain",
  "dns.override_local_dns": "Override local DNS",
  "dns.nameservers.global": "Nameservers",
  "dns.search_domains": "Domains",
  "server.diagnostics": "Debug",
  "features.headscale_service_control": "Service control",
  "features.service_control_driver": "Service driver",
  "features.service_unit": "Service unit",
  "server.log_json": "JSON logging",
  "server.log_level": "Log level",
  "server.web.enabled": "Web interface",
  "server.web.password_login": "Password login",
  "server.web.default_role": "Default registration role",
  "server.web.listen": "Listen address",
  "server.web.public_url": "Public URL",
  "server.web.cookie_secure": "Secure cookies",
  "server.web.session_ttl": "Session duration",
  "server.web.session_key_file": "Session key file",
  "server.web.token_file": "Token file",
  "server.web.tls_cert": "TLS certificate",
  "server.web.tls_key": "TLS key",
  "server.web.trusted_proxies": "Trusted proxies",
  "server.web.oidc.enabled": "Single sign-on",
  "server.web.oidc.name": "Provider name",
  "server.web.oidc.issuer": "Provider URL",
  "server.web.oidc.client_id": "Client ID",
  "server.web.oidc.client_secret": "Client secret",
  "server.web.oidc.auto_register": "Auto-register",
  "server.web.oidc.disable_token_login": "Disable token login",
  "server.web.github.enabled": "GitHub login",
  "server.web.github.client_id": "Client ID",
  "server.web.github.client_secret": "Client secret",
  "server.web.github.auto_register": "Auto-register",
  "server.web.github.disable_token_login": "Disable token login",
  "hs.metrics_listen_addr": "Metrics address",
  "hs.prefixes.v4": "IPv4 prefix",
  "hs.prefixes.v6": "IPv6 prefix",
  "hs.prefixes.allocation": "Address allocation",
  "hs.disable_check_updates": "Disable update checks",
  "hs.log.level": "Log level",
  "hs.log.format": "Log format",
  "hs.logtail.enabled": "Client log collection",
  "hs.taildrop.enabled": "Taildrop",
  "hs.auto_update.enabled": "Client auto-update",
  "hs.node.expiry": "Default node expiry",
  "hs.node.ephemeral.inactivity_timeout": "Ephemeral inactivity timeout",
  "hs.ephemeral_node_inactivity_timeout": "Ephemeral inactivity timeout",
  "hs.derp.urls": "DERP map URLs",
  "hs.derp.auto_update_enabled": "DERP auto-update",
  "hs.derp.update_frequency": "DERP update interval",
  "hs.derp.server.enabled": "Embedded DERP",
  "hs.derp.server.region_id": "DERP region ID",
  "hs.derp.server.region_code": "DERP region code",
  "hs.derp.server.region_name": "DERP region name",
  "hs.derp.server.verify_clients": "DERP client verification",
  "hs.derp.server.stun_listen_addr": "STUN listen address",
  "hs.derp.server.automatically_add_embedded_derp_region": "Add embedded DERP region",
  "hs.derp.server.ipv4": "DERP public IPv4",
  "hs.derp.server.ipv6": "DERP public IPv6",
};
const options: Record<string, string[]> = {
  "server.web.default_role": ["pending", "member"],
  "features.service_control_driver": ["auto", "systemd", "none"],
  "server.log_level": ["debug", "info", "warn", "error"],
  "hs.log.level": ["panic", "fatal", "error", "warn", "info", "debug", "trace"],
  "hs.log.format": ["text", "json"],
  "hs.prefixes.allocation": ["sequential", "random"],
};
export function settingLabel(key: string) {
  return labels[key] ?? key;
}
export function settingGroup(key: string) {
  if (key === "server.diagnostics") return "Web";
  if (key.startsWith("features.")) return "Service control";
  if (key.startsWith("server.web.github.")) return "GitHub login";
  if (key.startsWith("server.web.oidc.")) return "Login";
  if (key.startsWith("server.web.")) return "Web";
  if (key.startsWith("server.log")) return "Logging";
  if (key.startsWith("hs.derp.")) return "DERP";
  if (key.startsWith("hs.prefixes.")) return "IP allocation";
  if (key.startsWith("hs.node.") || key.startsWith("hs.ephemeral")) return "Node lifecycle";
  if (key.startsWith("hs.log")) return "Logging";
  return "Server";
}
export function listSetting(key: string) {
  return (
    key === "hs.derp.urls" ||
    key === "server.web.trusted_proxies" ||
    key === "dns.nameservers.global" ||
    key === "dns.search_domains"
  );
}
export function settingField(item: Setting): Field {
  const field: Field = {
    name: "value",
    label: settingLabel(item.key),
    values: [item.value],
    secret: item.secret,
  };
  if (listSetting(item.key)) {
    field.multi = true;
    field.values = JSON.parse(item.value || "[]") ?? [];
  } else if (options[item.key]) field.choices = options[item.key];
  else if (item.key !== "dns.base_domain" && (item.value === "true" || item.value === "false"))
    field.choices = ["true", "false"];
  if (item.secret) field.help = "This secret is write-only. An empty value clears it.";
  return field;
}
