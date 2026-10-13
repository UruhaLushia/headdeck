import { t } from "../../lib/i18n";
import type { Node, PolicyObjects, Route, User } from "../../lib/types";

export interface PolicyOption {
  value: string;
  label: string;
  kind: string;
  description?: string;
  aliases?: string[];
}

export type PolicyEntry = PolicyObjects["items"][number];

export function aclDestination(value: string) {
  const separator = value.lastIndexOf(":");
  if (separator >= 0) {
    return {
      target: value.slice(0, separator).replace(/^\[(.*)\]$/, "$1"),
      ports: value.slice(separator + 1),
    };
  }
  return { target: value, ports: "*" };
}

export function withPorts(target: string, ports: string) {
  const address = target.split("/")[0];
  return `${address.includes(":") && !/^(tag|group|autogroup):/.test(target) && !target.includes("@") ? `[${target}]` : target}:${ports}`;
}

export function validPorts(value: string) {
  return (
    value === "*" ||
    value.split(",").every((part) => {
      if (!/^\d+(?:-\d+)?$/.test(part)) return false;
      const [start, end = start] = part.split("-").map(Number);
      return start >= 0 && start <= end && end <= 65535;
    })
  );
}

export function validAddress(value: string, subnet = false) {
  const [address, bits, extra] = value.split("/");
  let size = 32;
  if (address.includes(":")) {
    if (!URL.canParse(`http://[${address}]/`)) return false;
    size = 128;
  } else if (
    !/^\d+\.\d+\.\d+\.\d+$/.test(address) ||
    address.split(".").some((part) => Number(part) > 255 || (part.length > 1 && part[0] === "0"))
  ) {
    return false;
  }
  return (
    extra === undefined &&
    (bits === undefined ? !subnet : /^\d+$/.test(bits) && Number(bits) <= size)
  );
}

export function policyOptions(
  entries: PolicyEntry[],
  users: User[],
  nodes: Node[],
  routes: Route[],
) {
  const options = new Map<string, PolicyOption>();
  for (const user of users) {
    const identifier = user.provider_identifier || user.name || user.email;
    if (!identifier) continue;
    const value = identifier.includes("@") ? identifier : `${identifier}@`;
    const label = user.display_name || user.name || user.email || identifier;
    const aliases = [user.name, user.email ?? ""]
      .filter(Boolean)
      .map((alias) => (alias.includes("@") ? alias : `${alias}@`));
    options.set(value, {
      value,
      label,
      kind: "Users",
      description: user.email || user.name,
      aliases,
    });
  }
  for (const { item, fields } of entries) {
    if (item.add) continue;
    if (item.section === "groups" || item.section === "tagOwners") {
      options.set(item.key, {
        value: item.key,
        label: item.key.replace(/^(group|tag):/, ""),
        kind: item.section === "groups" ? "Groups" : "Tags",
      });
    } else if (item.section === "hosts") {
      options.set(item.key, {
        value: item.key,
        label: item.key,
        kind: "Host aliases",
        description: fields.find((f) => f.name === "value")?.values[0],
      });
    }
  }
  for (const node of nodes) {
    for (const address of node.ip_addresses)
      options.set(address, {
        value: address,
        label: node.given_name || node.name,
        kind: "Devices",
        description: address,
      });
    for (const tag of node.valid_tags ?? [])
      if (!options.has(tag))
        options.set(tag, { value: tag, label: tag.replace(/^tag:/, ""), kind: "Tags" });
    for (const prefix of node.available_routes ?? [])
      options.set(prefix, { value: prefix, label: prefix, kind: "Subnets" });
  }
  for (const route of routes)
    options.set(route.prefix, { value: route.prefix, label: route.prefix, kind: "Subnets" });
  for (const [value, label] of [
    ["*", "All devices and addresses"],
    ["autogroup:member", "User-owned devices"],
    ["autogroup:tagged", "Tagged devices"],
    ["autogroup:self", "Own devices"],
    ["autogroup:internet", "Internet"],
    ["autogroup:danger-all", "All addresses, including outside the tailnet"],
  ])
    options.set(value, { value, label: t(label), kind: "Built-in targets" });
  return [...options.values()];
}

export function optionLabel(options: PolicyOption[], value: string) {
  const option = options.find(
    (candidate) => candidate.value === value || candidate.aliases?.includes(value),
  );
  if (option?.kind === "Devices") return `${option.label} · ${option.description}`;
  return option?.label ?? value;
}

export function targetOptions(options: PolicyOption[], section: string, name: string) {
  if (section === "groups") return options.filter((o) => o.kind === "Users");
  if (section === "tagOwners")
    return options.filter((o) => o.kind === "Users" || o.kind === "Groups");
  if (section.startsWith("autoApprovers."))
    return options.filter((o) => ["Users", "Groups", "Tags"].includes(o.kind));
  if (section === "ssh") {
    if (name === "dst")
      return options.filter((o) => o.kind === "Tags" || o.value === "autogroup:self");
    return options.filter(
      (o) =>
        ["Users", "Groups", "Tags"].includes(o.kind) ||
        o.value === "autogroup:member" ||
        o.value === "autogroup:tagged",
    );
  }
  return options.filter((o) =>
    name === "src"
      ? !["autogroup:self", "autogroup:internet"].includes(o.value)
      : o.value !== "autogroup:danger-all",
  );
}
