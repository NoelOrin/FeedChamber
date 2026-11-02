---
title: MCP Integration
description: Call 15 tools over POST /mcp with an API key, and connect the site as an MCP client for external moderation
---

`POST /mcp` lets AI clients (Claude Desktop, Cursor, Cline) read and write site content and
customization directly. It speaks JSON-RPC 2.0, **skips** the forum envelope, and authenticates
with an API key only — and the site can act as an MCP client in the other direction.

Two switches must be on or `/mcp` refuses everything: `mcp_enabled` and `api_key_enabled` (both
default `false`; the second is the credential-level kill switch). `mcp_disabled_tools` is a tool
disable list, not a third switch: empty means default tool availability, which has reads on and
writes off. Create a key whose scopes cover the tools you need — its owner needs those
permissions too, since a scope is a ceiling, not a substitute.

## Connecting

The endpoint lives at the **app root** (`https://your-host/mcp`) and takes `POST`; `OPTIONS`
returns 204. `Authorization: Bearer ak_...` is equivalent to the header below.

```json
{ "mcpServers": { "feedchamber": { "type": "http", "url": "https://forum.example.com/mcp",
  "headers": { "X-API-Key": "ak_..." } } } }
```

## Protocol shape

Requests and replies are standard JSON-RPC 2.0. Supported methods are `initialize`,
`tools/list`, `tools/call`, plus the no-result `notifications/initialized` and `ping`; even
`initialize` requires a credential. The negotiated version is `2025-06-18` and `serverInfo` is
`{"name":"feedchamber","version":"1.0.0"}`. `tools/list` returns only tools the caller may use
**and** that are enabled, sorted by name — a missing tool is usually permissions or a disabled
tool, not a bug.

## Two error layers

Protocol problems use the JSON-RPC `error` object; business failures come back as a **tool-level**
result with `content[].text` and `isError: true`, so clients can tell the two apart. Codes:
`-32700` / `-32600` invalid JSON or JSON-RPC, `-32601` / `-32602` unknown method or tool /
invalid parameters, `-32010` tool exists but is **disabled** (deliberately distinct from
"unknown"), `-32011` denied by permission, ban or maintenance, `-32012` over the rate-limit
budget.

## The 15 tools

| Tool | Purpose | Permission | Writes |
|------|---------|------------|--------|
| `search_posts` | search posts, respecting visibility | `post:read` | |
| `get_post` | read a post with its body | `post:read` | |
| `list_spaces` | list the space tree | `post:read` | |
| `list_tags` | list tags (paged) | `post:read` | |
| `get_user_profile` | read a public profile | `user:read` | |
| `list_snippets` | list all customization snippets | `customize:read` | |
| `create_post` | create a post (status follows moderation) | `post:create` | ● |
| `create_comment` | comment or reply | `comment:create` | ● |
| `like_post` | like or unlike (toggle) | `content:like` | ● |
| `favorite_post` | favorite or unfavorite (toggle) | `favorite:self` | ● |
| `upsert_snippet` | write a CSS / JS / HTML snippet | `customize:write` | ● |
| `set_theme_tokens` | override theme tokens | `customize:write` | ● |
| `upsert_nav_item` | write a nav item | `customize:write` | ● |
| `upsert_home_module` | write a home module | `customize:write` | ● |
| `upsert_custom_page` | write a custom page at `/p/{slug}` | `customize:write` | ● |

Write tools are **disabled by default** — enable them under *MCP tools* (`/admin/mcp`).
`mcp_disabled_tools` stores a **delta over the default**:
empty means reads on and writes off, `search_posts` disables that read tool, `create_post` keeps
it off, `-create_post` **explicitly enables** it. The minus form exists so clearing the list
returns to the safe state instead of opening every write path. Toggle one tool via
`POST /api/v1/admin/mcp/tools/status` with `{"name":"create_post","enabled":true}`
(`setting:manage`); it persists immediately.

## Authorization

- **Credential**: `ak_` prefix, bcrypt, expiry, revocation; the `ban` role is rejected, muted
  users keep reads but lose writes, and maintenance allows admin writes only.
- **Permissions**: scope **and** role; `customize:publish` is granted by key scope alone and is
  never inherited from a role.
- **Rate limit**: the key's `rate_limit`, or 120 requests per minute.

The `customize:publish` exception is deliberate: admins hold it, so inheriting it would let any
key an admin creates apply JS immediately. A key with `customize:write` writing JS / HTML / nav /
home modules / custom pages gets a draft instead, with `applied: false` and a `hint`.

Tool calls run **as the caller**, so visibility, ownership and gate rules match HTTP exactly:
authors read their own pending posts, `login` / `level` spaces are evaluated by level, and there
is no moderation bypass — `create_post` still follows `moderation_mode`.

Results are truncated at `mcp_max_result_bytes` (default 8192) with a marker appended, and every
call is audited under the `mcp.tool.` prefix (caller, tool, outcome, 200-character summary); a
failed audit write logs a warning but never blocks the call.

## Acting as an MCP client

Register external servers (admin *External MCP*, `/admin/mcp-servers`) under
`moderation_mode=external` via `POST /api/v1/admin/mcp-servers/{list|create|update|delete|test}`
(`setting:manage`); `test` handshakes and lists tools.

| Constraint | Behaviour |
|------------|-----------|
| Transport / SSRF | Streamable HTTP only; `http(s)`, no loopback or private addresses |
| Redirects / auth headers | redirects **not followed**; headers stored as JSON, only key names returned |
| Timeout / size | 3 s per call; a 1 MB response cap that errors instead of truncating |

Set `purpose` to `moderation` to put a server in the review path and `moderation_tool` to name
the tool (empty means `check_content`); verdicts may be `{"action":"allow"}` or
`{"content":[{"type":"text","text":"allow"}]}`.

**Fallback**: a server not registered for moderation, or one that fails, falls back to the
existing HTTP moderation contract — silently in the first case, with a warning log in the second.
Neither blocks publishing.

## Verification and failures

`curl -X POST https://your-host/mcp -H 'X-API-Key: ak_...' -d
'{"jsonrpc":"2.0","id":1,"method":"tools/list"}'` returning names (not `-32011`) means the
credential and switches are fine; calls appear in the audit log as `mcp.tool.<name>`.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `-32011 disabled` / `-32010` | a switch is off / the tool is disabled | enable both switches; enable the tool |
| `-32601` / short list | wrong name or missing permissions | check `tools/list`, then both layers |
