# Requirements

Headdeck provides terminal and browser management for self-hosted Headscale. This document defines product scope and acceptance criteria. See [Architecture](architecture.md) and [API](api/README.md) for implementation details.

## Roles and access

| Role | Scope |
| --- | --- |
| `pending` | Waits for approval; cannot read network resources or perform management operations |
| `member` | Views the bound Headscale user, devices, routes and authentication keys; can rename or expire owned devices and manage owned keys |
| `admin` | Manages network resources, DNS, policies, Headscale configuration and services; cannot manage other Headdeck accounts |
| `owner` | Includes administrator access and manages accounts, identity bindings, login providers and Web settings |

A member Web account needs a Headscale binding before accessing management pages. The server enforces permissions and ownership independently of visible UI controls. Local Unix authorization and Web accounts are separate; changes to local authorization require sudo.

## Features

| Area | Requirements |
| --- | --- |
| Overview | Separate daemon and Headscale status, version, uptime and memory; network resource counts |
| Devices | Details, names, owners, tags, expiry and route approval; subnet and exit-node badges belong to the device |
| Users | Headscale user management and binding existing users to Headdeck accounts |
| Authentication keys | Create, expire and delete; return usable key material only on creation |
| DNS | MagicDNS, domain, global and split nameservers, search domains and additional records |
| Access control | Visual forms for access rules, grants, SSH rules, groups, tag owners, host aliases and automatic approvals; searchable user, group, tag, device and subnet selection, with separate protocol and port controls |
| Configuration | Edit supported Headscale settings, save drafts, apply or discard; detect external changes |
| Service control | Start, stop, restart and reload Headscale; track asynchronous results |
| Account security | Passwords, multiple OAuth/OIDC identities, GitHub, Passkey, TOTP, recovery codes and device sessions |
| Headscale login | Headdeck OIDC issuer backed by approved accounts and explicit Headscale bindings; required for Web |
| Headscale connection | Discover the local listener and Unix management socket; no API key or manual connection settings |

## Identity binding

- Headdeck accounts have stable, independent account IDs.
- Headscale bindings use user IDs. A bound account's username follows the Headscale username.
- External identities use `issuer + subject`; one identity belongs to one account. Display names and email addresses do not automatically merge accounts.
- GitHub login names provide usernames, nicknames provide display labels, and numeric identifiers identify the external identity.
- An account may have multiple identities; a Headscale user may be bound to only one Headdeck account.
- Provider settings control Headdeck account registration. `server.web.default_role` is `pending` by default for private deployments; public deployments use `member`. Automatic registration never grants `admin` or `owner`, and changing the default does not change existing roles.
- Headdeck account registration and Headscale user registration are separate. A new member has no Headscale user until the first Tailscale device OIDC login. Headscale creates that user, and Headdeck binds it using `<Headdeck issuer>/<account ID>`. Pending accounts require an owner to bind an existing Headscale user and approve access.
- Provider username collisions receive a unique suffix instead of failing registration or merging accounts. Missing or invalid usernames receive a generated name. Existing Headscale users are never claimed by matching a name or email.
- Disabling accounts, changing privileges or rebinding users must prevent old sessions from retaining previous access.

## Interaction

The TUI supports mouse, arrow keys and focus navigation. Wide terminals use a sidebar and multiple overview columns; narrow terminals use named horizontal navigation and a single column.

The Web UI uses HeroUI 3 with consistent forms, lists, details and dialogs. Both interfaces support English and Simplified Chinese; Web supports light, dark and system themes.

Edit individual settings inline and open details for multi-field objects. Use selection controls for fixed values and row editors for lists. Deletion requires a confirmation dialog without typed-name confirmation. UI text should explain only what is needed to complete the action.

Use cached data immediately when navigating and refresh it in the background. Resource updates should send differences instead of repeated snapshots. Overview status uses SSE with a real sample every 15 seconds; clients advance displayed uptime locally between samples and correct it on the next sample. Stop the overview stream when leaving the page or hiding the browser tab.

## Acceptance criteria

Enforce access rules for direct API requests, unbound accounts and other users' resources. Separate saving from applying configuration, reject conflicting external changes and report rollback results. Check both systemd state and API health before reporting a successful start.

Bound event history, subscriber queues, authentication state, caches and operation records. Release connections and tasks on disconnect, unmount and daemon reload. Persist sessions across reloads while supporting per-device revocation.

## Current limits

Service control currently targets Linux/systemd. Missing upstream device fields, such as platform, must not be inferred. Web and password login default to disabled. Configure login through CLI/TUI before enabling Web. Headscale must use Headdeck's HTTPS OIDC issuer and matching account bindings; Dex is not part of the target deployment. The TUI can administer account security, while browser ceremonies such as Passkey enrollment remain Web operations.

Web lives at the domain root and automatically takes over Headscale OIDC and HTTP forwarding. Configure one public URL in Headdeck; Headscale's public URL follows it, and the internal upstream is read from Headscale's listener configuration. The HTTPS reverse proxy forwards all traffic to Headdeck, which serves its own UI/API and proxies Headscale HTTP requests and control-protocol upgrades. Headscale remains on a separate loopback listener and enforces its own API and device authentication.
