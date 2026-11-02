---
title: Realtime (SSE)
description: The notification and generic SSE endpoints, event formats, the 15-second keepalive, client reconnection with 401 refresh, and proxy read timeouts
---

Realtime pushes use Server-Sent Events in one direction only. Both endpoints authenticate with a
session sid but need different permissions, `notification:self` and `realtime:self`.

| Endpoint | What it pushes |
|----------|----------------|
| `POST /api/v1/notifications/stream` | unread notification count |
| `POST /api/v1/realtime/stream` | generic event envelopes with channel subscriptions |

Both are `POST`, so `EventSource` is unusable — use `fetch` and read `response.body`, which is
what `@feedchamber/sse-client` does.

## The notification stream

```bash
curl -N -X POST http://localhost:8080/api/v1/notifications/stream \
  -H "Authorization: Bearer $TOKEN" -H 'Accept: text/event-stream'
```

You get the current unread count immediately, then one push per change:

```
data: {"unread":3}
: keepalive
data: {"unread":4}
```

The field is `unread`, unlike the `{"count":n}` from `POST /notifications/unread_count` — keep
two parsers.

An idle stream emits no bytes, so any intermediary with a read timeout cuts it (nginx defaults
`proxy_read_timeout` to 60 s). The handler therefore writes the comment `: keepalive` every
**15 seconds**; clients must ignore comment lines, and the frontend parser only reacts to
`data: ` lines, so it can never surface as an empty event. If the KV subscription fails (say
NATS is briefly down) the endpoint degrades to polling every `sse_polling_interval` seconds
(default 5) instead of dropping the connection.

## The generic stream

```bash
curl -N -X POST http://localhost:8080/api/v1/realtime/stream \
  -H "Authorization: Bearer $TOKEN" -H 'Accept: text/event-stream' \
  -H 'Content-Type: application/json' -d '{"channels":["notif:01J…"]}'
```

Only your own channel `notif:<your user id>` is accepted; other names are silently dropped, and
if none survive you fall back to your own. Events carry an `id:`, e.g. `id: 8` followed by
`data: {"type":"message","channel":"heartbeat","payload":{"ts":1793000000},"id":8}`.

:::note[Two different keepalives]
The notification stream keeps alive with an SSE **comment**; the generic stream sends its heartbeat
as a **regular event** with `channel: "heartbeat"`.
:::

## Client: reconnection and 401 refresh

`SSEClient` options include `url`, `method` (default `POST`), `withCredentials`, `refreshToken`,
`onData`, `maxRetries` (default 5) and `resumeOnVisible`. Backoff is exponential — 3 s, ×1.5,
capped at 30 s — and resets on any successful connection. On a 401 it calls `refreshToken()`
once: success reconnects, failure clears local auth state and stops.

:::caution[Refresh rotates the credential]
`/auth/refresh` revokes the old sid. Bearer clients must adopt the new token, otherwise the
reconnect is rejected with `1014` and looks like "it never connects".
:::

```ts
const client = new SSEClient({
  url: '/api/v1/notifications/stream',
  method: 'POST',
  withCredentials: true,          // HttpOnly cookie auth; JS cannot read the token
  refreshToken: () => tryRefreshToken(),
  onData: (d) => typeof d.unread === 'number' && setUnread(d.unread),
});
client.start();  // stop() on unmount
```

## The 60-second proxy problem

nginx's default `proxy_read_timeout` cuts an idle stream at roughly 61 s, so the browser
reconnects — a hiccup every minute. Deployments give both streams their own location:

```nginx
location ~ "^/api/v1/(notifications|realtime)/stream$" {
    proxy_pass http://backend:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering    off;
    proxy_cache        off;
    proxy_read_timeout 3600s;
}
```

`proxy_buffering off` matters as much as the timeout: with buffering on, nginx accumulates small
events and pushes arrive in batches. The regex location wins over the `/api/v1/` prefix, and with
the 15 s keepalive in place this block is a backstop, not the only mechanism.

## Multiple tabs

The server does not limit connections per user and there is no cross-tab deduplication: three
tabs mean three connections and three reconnection storms. Keep **one per tab** — subscribe at
the layout level, `stop()` on unmount, pause background tabs with `resumeOnVisible`. For a single
site-wide connection, elect a leader tab and broadcast over `BroadcastChannel`.

## Verification and failures

Connect with `curl -sN --max-time 3`: the unread count arrives at once, a reply to your post
pushes a second frame, and a `: keepalive` appears after 15 idle seconds. The
`feedchamber_sse_connections{channel="notification"}` gauge should drop back on disconnect.

| Symptom | Cause | Fix |
|---------|-------|-----|
| drops every ~60 s | proxy read timeout or missing keepalive | check the SSE location and buffering |
| immediate 401 / 403 | missing credential or permission | send the cookie or Bearer |
| events arrive in batches | a buffering intermediary | disable `proxy_buffering` |
| gives up after retries | `maxRetries` reached | check NATS, or raise the cap |
