# SSE streams

Both stream families use ordinary authenticated GET requests with `Content-Type: text/event-stream`. No WebSocket upgrade or bidirectional messages are used. Reverse proxies must forward streamed responses without buffering; the server sets `X-Accel-Buffering: no`.

Status and resource streams share a daemon-wide limit of 64 connections. Each frame has a renewed write deadline. Streams close on request cancellation, lost authorization or a lifetime limit of at most 30 minutes. Web lifetime is also bounded by session expiry. Clients reconnect rather than keeping abandoned connections alive.

## Status

Web endpoint: `/api/status/stream`. It requires `system.read`. Initial delivery uses `snapshot`:

```text
event: snapshot
data: {"server":{"state":"running","uptime":"2h14m5s"},"headscale":{"configured":true,"reachable":true}}

```

The example is abbreviated; the initial payload contains the same fields as `/api/status`. Later `diff` frames contain only changed top-level fields:

```text
event: diff
data: {"server":{"state":"running","uptime":"2h14m20s"}}

```

Merge top-level fields into the last snapshot; nested objects are replacements, not recursive patches. A removed field is sent as `null`. Real sampling occurs every 15 seconds. Clients locally advance uptime from the last sample and recalibrate when another real sample arrives. They do not synthesize memory, online state or health values. Stopped service uptime does not advance.

Status frames have no replay cursor. Every connection starts with a new snapshot. Browser EventSource handles transient reconnects; on authorization failures the application checks `/api/session`. The frontend closes this stream outside the overview page or while hidden. Manual refresh reconnects for an immediate sample. TUI status uses ordinary IPC status requests every 15 seconds with local uptime rendering.

## Resources

Endpoints: Web `/api/events`, IPC `/api/v1/events`; both require `system.read`. Event scopes are filtered by the authenticated principal's permissions. Members use ownership-filtered GET requests on visits and manual refreshes, without an event connection or resource polling. Role or Headscale binding changes close the Web stream.

| Event | Meaning |
| --- | --- |
| `hello` | Allowed scopes and daemon instance ID |
| `cursor` | Resume position, including events filtered out of this connection |
| `nodes.changed`, `users.changed` | Collection differences when available, otherwise invalidation |
| `keys.changed`, `routes.changed` | Resource invalidation; refresh the corresponding collection |
| `operations.changed` | An operation changed; data may contain its current result |
| `dns.changed`, `policy.changed`, `config.changed` | Draft or applied configuration changed; refresh that resource |
| `accounts.changed`, `providers.changed` | Account or login provider changed; owner-only invalidation |
| `headscale.recovered` | Refresh health-dependent state |
| `resync` | History expired or subscriber overflowed; fetch fresh snapshots |
| `bye` | Planned end of stream; reconnect |

Regular event data uses this envelope:

```text
event: nodes.changed
data: {"id":43,"type":"nodes.changed","timestamp":"2026-10-15T00:00:00Z","revision":43,"data":{"base_revision":7,"revision":8,"upsert":[],"removed":["12"]}}
id: 43

```

The outer revision belongs to the event; the inner revision belongs to the collection. Apply `upsert` by stable ID and remove IDs in `removed`, then replace the local collection revision. Apply only if `base_revision` equals the client's current collection revision; otherwise fetch a new collection snapshot. Ignore differences already covered by a newer snapshot. Missing difference data means invalidation, not an empty change.

Resume using the `Last-Event-ID` header. Initial subscription and replay are acquired atomically. Record cursor frames as well as visible events. A daemon instance change or `resync` invalidates the replay assumption; refresh snapshots and update the resume cursor. Browser EventSource preserves its cursor during automatic retries; an explicitly new connection does not inherit it automatically.

The hub retains at most 256 events and 8 MiB of history. Event data is limited to 512 KiB; oversized differences fall back to invalidation. Slow consumers are disconnected when their queues overflow. A cursor heartbeat is sent every 20 seconds. A connected stream does not imply a permanent or durable event log.
