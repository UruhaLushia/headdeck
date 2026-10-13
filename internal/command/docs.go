package command

const docsText = `Headdeck IPC

HTTP over /run/headdeck/ipc.sock; application protocol version 1.
GET /ping is public. All protected requests authenticate SO_PEERCRED UID
against the root-controlled user allowlist and check the user's role.
No client keys, signatures, timestamp windows or nonce cache are required.

hdkt auth add <user> [--role owner|admin|member|pending]
hdkt auth remove <user>
hdkt auth list
These commands invoke sudo when needed. Passwords stay with sudo.

POST /api/v1/system/hello        negotiate version and permitted features
GET  /api/v1/system/status       daemon and Headscale status
GET  /api/v1/events              scoped SSE resource diffs and notifications
GET  /api/v1/settings            admin-only, secrets omitted
PATCH /api/v1/settings           atomic {"changes":{"key":"value"}}, automatically reloads

hdkt settings get [key]
hdkt settings set key value
hdkt settings set server.web.public_url https://ts.example.com

/etc/headdeck/users.json         root:headdeck 0640
/var/lib/headdeck/settings.db    headdeck:headdeck 0600, SQLite
/run/headdeck/ipc.sock           headdeck:headdeck 0666, UID authorization

`
