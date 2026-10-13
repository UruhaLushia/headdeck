# Local IPC API

IPC is HTTP over a Unix Socket, default `/run/headdeck/ipc.sock`. The daemon reads peer credentials and checks the connecting UID against `/etc/headdeck/users.json`. It does not trust user IDs, roles, passwords or identity headers supplied by the caller.

The application protocol version is `1`. Clients negotiate through `POST /api/v1/system/hello`; authorization is still checked on later requests and long-lived streams. `/ping` is public. No browser Cookie or client-side signing key is used.

```sh
curl --unix-socket /run/headdeck/ipc.sock http://headdeck.local/ping
curl --unix-socket /run/headdeck/ipc.sock \
  -H 'Content-Type: application/json' \
  -d '{"client":"example","client_version":"0.1.0","protocol_min":1,"protocol_max":1}' \
  http://headdeck.local/api/v1/system/hello
```

Run these as an authorized local user, with Host `headdeck.local` and no Origin or Referer. The handshake response includes the selected version, server version, principal, role and permitted features. Adding/removing local users uses `hdkt auth` and sudo, not an HTTP endpoint.

## Endpoints

Unless noted, requests use the [shared mutation bodies](README.md#shared-mutation-bodies) and role policy from [Requirements](../requirements.md#roles-and-access).

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/ping` | Service identity |
| POST | `/api/v1/system/hello` | Protocol and feature negotiation |
| GET | `/api/v1/system/status` | Daemon and Headscale status; optional `?refresh=1` |
| GET | `/api/v1/system/headscale` | Headscale service state |
| POST | `/api/v1/system/headscale/{action}` | Start, stop, restart or reload; accepts optional `{"reason":"..."}` |
| GET | `/api/v1/operations` | Retained operation list |
| GET | `/api/v1/operations/{id}` | Operation result |
| GET | `/api/v1/events` | Resource SSE; requires `system.read` |
| GET | `/api/v1/nodes` | Node snapshot; optional `?user=alice` |
| GET | `/api/v1/nodes/{id}` | Node details |
| POST | `/api/v1/nodes/{id}/rename` | Rename node |
| POST | `/api/v1/nodes/{id}/user` | Move node to a Headscale user |
| POST | `/api/v1/nodes/{id}/tags` | Replace forced tags |
| POST | `/api/v1/nodes/{id}/expire` | Expire node |
| DELETE | `/api/v1/nodes/{id}` | Delete node |
| GET | `/api/v1/routes` | Route list |
| GET / POST | `/api/v1/nodes/{id}/routes` | Read or replace route approvals |
| GET / POST | `/api/v1/users` | User snapshot or create user |
| POST | `/api/v1/users/{id}/rename` | Rename user |
| DELETE | `/api/v1/users/{id}` | Delete user |
| GET / POST | `/api/v1/preauth-keys` | List or create keys; GET accepts `?user=alice` |
| POST | `/api/v1/preauth-keys/{id}/expire` | Expire key |
| DELETE | `/api/v1/preauth-keys/{id}` | Delete key |
| GET / PATCH | `/api/v1/settings` | Read or update Headdeck settings |
| GET / PATCH / DELETE | `/api/v1/headscale/config` | Read, save or discard a managed configuration draft |
| POST | `/api/v1/headscale/config/apply` | Apply managed configuration |
| POST | `/api/v1/headscale/login/apply` | Retry automatic Headscale management; local owner only |
| POST | `/api/v1/headscale/login/recover` | Recover an interrupted login transaction; local owner only |
| GET / PUT / DELETE | `/api/v1/dns` | Read, save or discard DNS draft |
| POST | `/api/v1/dns/apply` | Apply DNS |
| GET / PUT / DELETE | `/api/v1/policy` | Read, save or discard raw policy draft |
| POST | `/api/v1/policy/check` | Validate policy, body `{"policy":"...","revision":"..."}` |
| POST | `/api/v1/policy/apply` | Apply policy |
| GET / POST | `/api/v1/accounts` | List or create Headdeck accounts; owner only |
| PUT | `/api/v1/accounts/{id}` | Update account; owner only |
| GET / POST / DELETE | `/api/v1/accounts/{id}/identities` | List, bind or detach identities; owner only |
| PUT | `/api/v1/accounts/{id}/password` | Set account username/password; owner only |
| GET / POST | `/api/v1/accounts/{id}/security` | Read security or perform reset/revocation; owner only |
| GET | `/api/v1/login-providers` | Custom providers; owner only |
| PUT / DELETE | `/api/v1/login-providers/{id}` | Save or delete a custom provider; owner only |

Account lists use `{"accounts":[]}`; identity lists use `{"bindings":[]}`. These differ from Web arrays. IPC provider listing returns custom definitions only; built-in provider settings belong to `/api/v1/settings`.

Raw policy save sends `{"revision":"...","policy":"..."}`. Applying drafts additionally requires `service.manage`. Setting updates containing `server.web.*` require owner access. Node ownership mutation is named `/user` in IPC and `/owner` in Web.

Headscale management starts automatically after the Web listener opens. Login apply/recover remain local maintenance endpoints and have no request body. They return `202` with an operation; inspect its final state rather than treating acceptance as completion. Apply rejects incomplete user mappings before stopping Headscale. See [the takeover sequence](../architecture.md#headscale-oidc-provider).

IPC does not expose personal browser login, Passkey enrollment or device-session routes. The TUI uses owner administration routes for credential resets and revocation.

## SSE example

```sh
curl -N --unix-socket /run/headdeck/ipc.sock \
  -H 'Last-Event-ID: 42' \
  http://headdeck/api/v1/events
```

Use a timeout-free HTTP client for streams, cancel it when the UI closes, and follow the [replay and revision rules](streams.md#resources). The built-in Go client is in [internal/ipc/client](../../internal/ipc/client).
