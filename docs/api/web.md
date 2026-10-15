# Web API

Paths below are relative to `server.web.public_url`. With the default Headscale proxy enabled, Web is served at the domain root: `/api/nodes` means `https://example.com/api/nodes`. Headscale's `/api/v1/*`, `/oidc/callback` and device/control endpoints are forwarded to its internal listener without adding Headdeck's API key.

## Authentication and request headers

Web and password login default to disabled. All Web API routes except sign-out return `503` while Headscale login takeover is incomplete; this includes provider callbacks and existing sessions. Initial configuration uses local CLI/TUI. Sessions use the `headdeck_session` Cookie. Login responses set it; browser requests send it automatically. The Cookie is HttpOnly, SameSite Lax and Secure when configured. OAuth binding callbacks can therefore receive the browser session; state, PKCE and flow cookies still validate the callback. Mutations require the request header and origin checks described below. Sessions persist in SQLite and are checked against current account security state.

Every non-GET/HEAD `/api/` request requires `X-Requested-With: headdeck`. A supplied Origin must match the configured public origin. Status SSE also rejects a supplied foreign Origin. These APIs do not accept a Headscale API key as a browser bearer credential.

The ordinary management API denies pending accounts and members without a Headscale binding. Personal account/security endpoints require a valid account session but are separate from network management permissions. Sessions persist across service restarts; their age does not impose an additional limit on personal changes. Password changes verify the current password when one is set, and disabling 2FA requires an authenticator or recovery code.

## Login and sessions

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/api/auth/methods` | Available login methods; no session required |
| POST | `/api/auth/password` | `{"username":"alice","password":"..."}`; `403` when `server.web.password_login` is disabled |
| POST | `/api/auth/totp` | `{"code":"123456"}` after a login challenge; also accepts recovery codes |
| GET | `/api/auth/{provider}/start` | Redirect to the selected provider |
| GET | `/api/auth/{provider}/callback` | Provider callback; validates the login flow |
| POST | `/api/auth/passkey/start` | Begin discoverable Passkey login |
| POST | `/api/auth/passkey/finish` | Submit the WebAuthn assertion |
| POST | `/api/session` | Optional configured token login, body `{"token":"..."}` |
| GET | `/api/session` | Authentication state, identity, permissions and approval state |
| DELETE | `/api/session` | Sign out the current session |
| GET | `/api/sessions` | Current user's login devices |
| DELETE | `/api/sessions/{id}` | Revoke one session |
| DELETE | `/api/sessions` | Revoke all own sessions, including the current one |

Password login returns `{"authenticated":true}` or `{"two_factor":true}`. The latter uses a short-lived `headdeck_auth` Cookie for the TOTP step. Provider login redirects back to the application or its second-factor page. Provider auto-registration uses `server.web.default_role` (`pending` or `member`, default `pending`), independently of Headscale user creation. An unbound public member receives `device_registration:true` in `/api/session`, with no network permissions; a pending device OIDC request can still supply `continue_url`. Headscale creates and binds the user at the first Tailscale device login. Private pending accounts require administrator or owner binding and approval.

Authenticated, approved sessions also receive `login_server`, the public Headscale URL derived from its managed OIDC callback. Web uses it to generate `tailscale up --login-server=…` in the Add device dialog. It is independent of the internal REST API address and does not contain an auth key. Public members can use this dialog before their first Headscale binding.

Sessions default to a seven-day idle lifetime (`server.web.session_ttl`, `168h`). Authenticated same-origin GET requests renew the server expiry and Cookie when less than half the lifetime remains. Event streams do not keep idle sessions alive. Renewal preserves the Cookie value, device ID and original authentication time, and does not create another session. Concurrent renewal cannot restore a revoked or expired session.

Device responses contain `id`, `browser`, `browser_version`, `platform`, `platform_version`, `created_at`, `expires_at` and `current`. Browser and platform details come from the saved User-Agent; unavailable versions are omitted. Session storage allows up to 32 sessions per account and 4096 overall.

## Network resources

| Method | Path | Required permission |
| --- | --- | --- |
| GET | `/api/status` | `system.read`; optional `?refresh=1` |
| GET | `/api/headscale/diagnostics/{section}` | `system.read`; on-demand, read-only Headscale diagnostics |
| GET | `/api/debug/pprof/` and profile paths | `system.read`; Headdeck profiles, diagnostics must be enabled |
| GET | `/api/status/stream` | `system.read`; [status SSE](streams.md#status) |
| GET | `/api/events` | `system.read`; [resource SSE](streams.md#resources) |
| GET | `/api/nodes` | `nodes.read`; optional `?user=alice` for administrators |
| POST | `/api/nodes/register` | `nodes.manage`; `{"key":"hskey-authreq-…","user_id":"1"}`; members register only to their bound user, administrators may specify another owner |
| POST | `/api/nodes/{id}/rename` | `nodes.manage` and ownership |
| POST | `/api/nodes/{id}/expire` | `nodes.manage` and ownership |
| POST | `/api/nodes/{id}/tags` | `nodes.write` |
| POST | `/api/nodes/{id}/owner` | `nodes.write` |
| POST | `/api/nodes/{id}/routes` | `routes.write` |
| DELETE | `/api/nodes/{id}` | `nodes.delete` |
| GET | `/api/users` | `users.read` |
| POST | `/api/users` | `users.write` |
| POST | `/api/users/{id}/rename` | `users.write` |
| DELETE | `/api/users/{id}` | `users.write` |
| GET | `/api/routes` | `routes.read` |
| GET | `/api/preauth-keys` | `keys.read`; optional `?user=alice` for administrators |
| POST | `/api/preauth-keys` | `keys.write` and ownership |
| POST | `/api/preauth-keys/{id}/expire` | `keys.write` and ownership |
| DELETE | `/api/preauth-keys/{id}` | `keys.write` and ownership |

Collection responses are `{"nodes":[],"revision":1}`, `{"users":[],"revision":1}`, `{"routes":[]}` and `{"keys":[]}`. Member lists are filtered to their Headscale binding and use revision `0` rather than the global revision. Web has no separate `GET /api/nodes/{id}` route; details use the node collection.

The Web device detail page is available at `#/nodes/{id}`. It shares the device collection cache with the list and supports direct links, reloads and browser back/forward navigation. An inaccessible or removed device shows an unavailable state without requesting another user's data.

Debug features are disabled by default (`server.diagnostics=false`). Administrators and owners can enable them under Settings → Headdeck → Web; the same setting is available in the TUI and through `PATCH /api/settings`. The setting is persisted. The Debug page (`#/diagnostics`) displays a disabled state until enabled in settings. While disabled, the page makes no diagnostic requests, the chart library is not loaded, and all diagnostic endpoints return `403` without reading upstream or serving cached snapshots.

When enabled, Headdeck reads Headscale's private `metrics_listen_addr`. Supported sections are `metrics`, `overview`, `batcher`, `nodestore`, `routes`, `derp`, `policy-manager` and `goroutine` (collapsed stacks). Responses contain `section`, `collected_at` (RFC 3339) and `format` (`text` or `json`). Debug responses include `content`; metrics responses include only chart `samples`, with timestamps and numeric `values`. Raw Prometheus text is neither returned nor displayed. Unsupported sections return `400`; unavailable listeners or sections show an error. Requests have a three-second timeout and a 4 MiB upstream response limit. Successful snapshots are shared for ten seconds, fetched only on access or refresh; there is no periodic polling.

The daemon retains up to 120 actual metric snapshots from the last hour in memory; cache hits do not add samples. History resets when Headdeck reloads or restarts, or a changed Headscale process start time is observed. Labeled series are summed for each displayed metric, and non-finite values are omitted. ECharts renders these snapshots, leaving missing values as gaps. CPU time is cumulative seconds, not CPU utilization.

The Headscale diagnostic target comes from the local Headscale configuration, with wildcard bindings resolved to loopback. No management key, browser Cookie or forwarding headers are sent upstream. Headscale configuration dumps, registration caches, forced GC, CPU profiling, trace and arbitrary debug URLs are not exposed. Members and pending accounts cannot access or enable diagnostics. Debug output is rendered as text, never as upstream HTML.

Headdeck's own Go pprof interface is available at `/api/debug/pprof/` after enabling the same diagnostic switch. It requires an administrator/owner Web session, including when accessed with profiling tools. Standard profile paths include `goroutine`, `heap`, `allocs`, `block`, `mutex`, `threadcreate`, CPU `profile`, execution `trace`, `cmdline` and `symbol`. Use `?debug=1` for text snapshots, or omit it to download binary profiles. The optional `seconds` parameter must be an integer from 1 to 30; CPU profiling defaults to 30 seconds and tracing to one second. There is no automatic profiling or change to runtime sampling rates. Responses are not cached. This API profiles Headdeck itself, independently of the Headscale debug views.

Manual device registration uses the pending registration key from the device sign-in URL (`hskey-authreq-…` on current Headscale, Node Key on older versions). It does not accept a pre-auth key or create a Headscale user. Omitting `user_id` uses the session's Headscale binding. A member cannot select another user; administrators and owners may select an existing user. Successful registration publishes the normal device update event.

## Settings, configuration and services

DNS switches, the tailnet domain, global nameservers and search domains use the same inline controls as Settings. Split DNS rules and custom records keep multi-field detail dialogs. Edits save to the DNS draft; the existing Apply action activates the draft and restarts Headscale only when required.

The Access control page uses searchable selection lists populated from the existing user, device and route APIs and structured policy objects. Users are identified by their stable Headscale provider identifier when available; existing username/email references are retained until explicitly changed. Groups and tags are selected by name. ACL destinations have independent port controls, while grants expose protocol/port service rows. SSH targets, login usernames and reauthentication settings use dedicated controls. New group and tag names receive the required prefix automatically. Existing values outside the option lists remain selected and removable. Saving still uses the revision-checked policy object API, preserves unedited advanced fields and creates a draft for the normal Apply workflow.

| Method | Path | Required permission |
| --- | --- | --- |
| GET / PATCH | `/api/settings` | `settings.read` / `settings.write`; Web settings additionally require `accounts.read` / `accounts.write` |
| GET / PATCH / DELETE | `/api/headscale/config` | `settings.read` for GET, `settings.write` for draft changes |
| POST | `/api/headscale/config/apply` | `settings.write` and `service.manage` |
| GET / PUT / DELETE | `/api/dns` | `dns.read` for GET, `dns.write` for draft changes |
| POST | `/api/dns/apply` | `dns.write` and `service.manage` |
| GET | `/api/policy` | `policy.read` |
| GET | `/api/policy/objects` | `policy.read`; structured editor fields |
| POST | `/api/policy/objects` | `policy.write`; structured edit |
| DELETE | `/api/policy` | `policy.write`; discard draft |
| POST | `/api/policy/apply` | `policy.write` and `service.manage` |
| POST | `/api/headscale/{action}` | `service.manage`; start, stop, restart or reload |
| GET | `/api/operations/{id}` | `operations.read` |

Headdeck settings reads return `items` and `reload_required`; secrets are masked. A successful settings update automatically requests daemon reload after responding, with no separate apply endpoint.

File paths (`token_file`, `session_key_file`, TLS files and the OIDC signing key) and `features.service_unit` belong to local installation configuration. They are excluded from editable settings and rejected by PATCH, including for owners.

`GET /api/policy/objects` returns `settings` and `items`. Each entry contains an `item` locator and `fields` with name, label, values, choices and multi-value metadata. Submit `{"revision":"...","item":{...},"values":{"src":["group:team"],"dst":["*:443"]},"remove":false}` using the locator returned by GET. The Web API does not expose raw policy PUT/check endpoints; these exist in IPC.

## Account administration

All routes below require administrator or owner access (`accounts.read` or `accounts.write`). Responses from the list and identity endpoints are arrays.

| Method | Path | Behavior |
| --- | --- | --- |
| GET / POST | `/api/accounts` | List or create an account |
| PUT | `/api/accounts/{id}` | Update role, label, disabled state and Headscale binding |
| GET / POST / DELETE | `/api/accounts/{id}/identities` | List, bind or detach an external identity |
| PUT | `/api/accounts/{id}/password` | Set username/password; Headscale binding constrains username |
| GET / POST | `/api/accounts/{id}/security` | Inspect security or perform a reset/revocation action |
| GET | `/api/login-providers` | Built-in and custom provider definitions, with secrets omitted |
| PUT / DELETE | `/api/login-providers/{id}` | Save or delete a custom provider |

Accounts have no DELETE endpoint; use disabled state to block login. See [shared payloads](README.md#accounts-and-providers).

## Personal account security

| Method | Path | Body or result |
| --- | --- | --- |
| PUT | `/api/account/profile` | `{"label":"Alice"}` |
| GET | `/api/account/security` | Account, password flag, identities, Passkeys and TOTP flag |
| PUT | `/api/account/password` | `{"username":"alice","password":"new-password","current_password":"old-password"}` |
| POST | `/api/account/providers/{provider}/bind` | Returns `{"url":"..."}`; follow it to verify and bind |
| DELETE | `/api/account/identities` | Identity issuer/subject |
| POST | `/api/account/totp/start` | Returns setup `id`, `secret` and QR data URL |
| POST | `/api/account/totp/finish` | `{"id":"setup-id","code":"123456"}` |
| DELETE | `/api/account/totp` | `{"code":"123456"}` |
| POST | `/api/account/passkeys/start` | Returns ceremony `id` and WebAuthn `options` |
| POST | `/api/account/passkeys/finish/{id}` | WebAuthn registration response |
| DELETE | `/api/account/passkeys/{id}` | Remove a credential |

Only profile and security reads omit the recent-login requirement. WebAuthn options and responses follow the browser credential APIs, with binary fields serialized as base64url. Follow [web/src/lib/passkeys.ts](../../web/src/lib/passkeys.ts) for the client encoding.

Passkey display names are resolved from the verified credential's AAGUID using the [Passkeys Developer registry](https://github.com/passkeydeveloper/passkey-authenticator-aaguids). Custom names are preserved; unknown or anonymized authenticators retain a generic name. This metadata is used only for display.

## Headscale OIDC

Discovery is at `<public_url>/oidc/.well-known/openid-configuration`. Read its advertised authorization, token, userinfo and signing-key URLs instead of constructing them. Discovery and signing keys remain available during setup so Headscale can start. Authorization completion and token validation require takeover readiness and an approved account. Bound accounts require an exact Headscale identity match; an unbound member may register a new user only when the default role is `member`. Sign-in uses the Headdeck login page; `prompt=login` does not silently reuse an existing session. This is an authorization-code provider for the configured Headscale client, not a password-grant endpoint. See [Architecture](../architecture.md#headscale-oidc-provider).
