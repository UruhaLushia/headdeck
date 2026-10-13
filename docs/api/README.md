# API reference

Headdeck exposes a [Web API](web.md) and a [local IPC API](ipc.md). Both call the same service layer, but their authentication, paths and some response shapes differ. [Streaming](streams.md) describes their SSE contracts.

## Conventions

Requests and responses use JSON unless a route is a login redirect, OIDC endpoint or SSE stream. Use `Content-Type: application/json` for JSON bodies. Decoders reject unknown fields and trailing JSON values. The ordinary JSON body limit is 1 MiB; WebAuthn and OIDC routes have their own smaller limits.

IDs are strings in Headdeck DTOs, including numeric Headscale IDs. Timestamps use RFC 3339; durations use Go duration strings such as `2h14m5s`. Configuration revisions are opaque strings; collection revisions and event IDs are integers. Do not interchange them.

Success responses expose the route's object or collection directly, without a common `data` wrapper. Ordinary mutations return `200`; accepted asynchronous operations return `202` with an Operation. Session deletion returns `204`.

Errors have this shape:

```json
{
  "error": {
    "code": "CONFLICT",
    "message": "DNS settings changed; refresh before saving"
  }
}
```

`request_id`, `retryable` and `details` may also be present. Do not parse the message to decide client behavior.

| HTTP status | Common error codes |
| --- | --- |
| 400 | `BAD_REQUEST`, `CLIENT_CONFIG_ERROR`, `HEADSCALE_NOT_CONFIGURED` |
| 401 | `UNAUTHENTICATED` |
| 403 | `PERMISSION_DENIED`, `PEER_NOT_AUTHORIZED` |
| 404 | `NOT_FOUND` |
| 409 | `CONFLICT`, `OPERATION_IN_PROGRESS` |
| 413 | `PAYLOAD_TOO_LARGE` |
| 426 | `PROTOCOL_INCOMPATIBLE` |
| 429 | `RATE_LIMITED` |
| 501 | `UNSUPPORTED`, `SERVICE_CONTROL_UNAVAILABLE`, `HEADSCALE_VERSION_UNSUPPORTED` |
| 502 | `HEADSCALE_UNAUTHORIZED` |
| 503 | `HEADSCALE_UNAVAILABLE`, `DAEMON_UNAVAILABLE`, `DEPENDENCY_UNAVAILABLE` |
| 504 | `REQUEST_TIMEOUT` |
| 500 | `INTERNAL` |

The complete mapping is in [protocol/errors.go](../../internal/ipc/protocol/errors.go).

## Shared mutation bodies

| Operation | Body |
| --- | --- |
| Rename device or user | `{"new_name":"new-name"}` |
| Change device owner | `{"user_id":"10"}` |
| Set device tags | `{"tags":["tag:server"]}` |
| Approve device routes | `{"routes":["192.168.1.0/24"]}`; an empty list clears approvals |
| Delete device | `{}` or `{"confirm_name":"device-name"}` |
| Create Headscale user | `{"name":"alice","display_name":"Alice","email":"alice@example.com"}`; the last two fields are optional |
| Delete Headscale user | `{"confirm_name":"alice"}` |
| Create authentication key | `{"user":"alice","reusable":false,"ephemeral":false,"expiration":"2027-01-01T00:00:00Z","acl_tags":[]}` |
| Update Headdeck settings | `{"changes":{"server.log_level":"warn"}}` |
| Update managed Headscale settings | `{"revision":"current-revision","values":{"prefixes.allocation":"sequential"}}` |
| Apply a configuration draft | `{"revision":"current-revision"}` |
| Bind or detach identity | `{"issuer":"https://identity.example.com","subject":"stable-subject"}` |
| Reset account security | `{"action":"totp.reset"}`, `{"action":"sessions.revoke"}` or `{"action":"passkey.delete","id":"credential-id"}` |

Clients supply the current user name for user deletion; the UI only asks for confirmation and does not require typing it. An existing account binding prevents deleting that Headscale user until detached.

Route approval is a replacement set, not an addition. Only advertised routes can be approved. Lists omit usable authentication key material; creation returns it once.

## Accounts and providers

An Account contains `id`, `label`, `username`, `role`, `disabled` and optional `headscale_user_id`. Account creation may also include `password`. Updates use the existing account ID in the path. A Headscale binding overrides the submitted username with the current Headscale username. Headscale bindings and external identities are unique across accounts.

A LoginProvider contains `id`, `name`, `kind`, `enabled`, `auto_register`, `issuer`, `client_id` and optional `client_secret`. Kinds are `github`, `oidc` and `oauth`. Generic OAuth also uses `auth_url`, `token_url`, `userinfo_url`, `scopes`, `subject_claim`, `name_claim` and `avatar_claim`.

Provider listing omits secrets and may return `secret_configured`. Updating an existing custom provider with an empty secret preserves its previous secret. Its kind and issuer cannot be changed in place; generic OAuth identity endpoints and the subject claim also require a new provider. IDs `github` and `oidc` are reserved for built-in settings; configure these through the settings API. Custom providers are limited to 16. Auto-registration uses `server.web.default_role` (`pending` or `member`) and does not create a Headscale user until device login.

## Configuration drafts and operations

DNS reads return `config`, `revision`, `writable`, `pending`, `restart_required` and `external_records`. DNS saves send `{"revision":"...","config":{...}}`; the config contains `magic_dns`, `base_domain`, `override_local_dns`, `nameservers` (`global` and `split`), `search_domains` and `extra_records` (`name`, `type`, `value`).

Managed configuration reads return `items`, `revision`, `writable` and `pending`. Policy reads return `policy`, `mode`, `revision`, `writable` and `pending`. Save preserves a draft; apply requires its latest revision. A `409` requires refreshing or discarding the conflicting draft rather than blindly retrying.

Managed settings include `hs.prefixes.v4`, `hs.prefixes.v6` (IPv4/IPv6 CIDRs) and `hs.prefixes.allocation` (`sequential` or `random`). At least one address family must remain configured. Applying prefix changes restarts Headscale; existing device addresses are not rewritten by Headdeck.

Headscale OIDC and HTTP forwarding are managed automatically whenever Web is enabled, after the Web listener starts. Its client ID is `headscale`, its callback is `<public_url>/oidc/callback`, and its client secret is generated once and stored in the settings database. `server.web.oidc_server.*`, `server.web.headscale_proxy` and `headscale.*` are not exposed or editable through the settings API. Headscale's `server_url` follows `server.web.public_url`; its loopback upstream is read from `listen_addr`. These two Headscale fields are not editable as managed settings. The daemon uses the configured local `unix_socket` for gRPC management and needs no API credential. External OAuth/OIDC providers are configured independently as Headdeck login providers.

An Operation contains `id`, `kind`, `state`, `target`, timestamps, `message`, optional `error` and `steps`. States are `pending`, `running`, `succeeded`, `failed`, `timeout` and `unknown`. Read the operation endpoint or watch `operations.changed` until completion. Records are bounded and do not survive daemon reloads; `404` does not establish whether the underlying operation succeeded.

Authoritative DTOs are in [internal/ipc/protocol](../../internal/ipc/protocol), [accounts](../../internal/auth/accounts/store.go) and [provider definitions](../../internal/auth/accounts/providers.go).
