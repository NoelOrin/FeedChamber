---
title: Site Customization
description: Theme token overrides, custom CSS/JS/HTML snippets, navigation items, home modules, and custom pages — and the rules that govern them
---

Site customization changes appearance and structure without code, across five areas: **theme
tokens**, **snippets** (CSS / JS / HTML), **navigation items**, **home modules**, and **custom
pages**. All of them reach the front end through one public aggregate,
`POST /api/v1/customization/public`, and live in their own tables rather than `settings` keys —
per-item toggles, per-item audit, and conditional indexing all need rows.

## Before you start

- `customize:read` to view, `customize:write` to edit, `customize:publish` for publish actions.
- Pages: `/admin/customize/theme`, `/snippets`, `/nav`, `/home`, `/pages`; custom pages are at
  `/p/{slug}`.

## Theme tokens

`/admin/customize/theme`. An override is identified by **(token, mode)**, so one token can hold
separate light and dark values: `all` (default) writes to both `:root` and `.dark`, `light` only
to `:root`, and `dark` only to `.dark`.

Only CSS custom property names (`--xxx`) and values without `; { } < >` or newlines, up to 512
characters, are accepted. The result is a single `<style id="fc-theme-overrides">` block placed
after global styles, so it wins. This layer is limited to colours and radii on purpose — complex
selectors belong in a CSS snippet.

## Snippets

`/admin/customize/snippets`. Each snippet has a kind, a position, content, and an enabled flag.

| Kind | Default position | Rendered as |
|------|------------------|-------------|
| `css` | `body_end` | A `<style>` block |
| `js` | `head` | A `<script>` block |
| `html` | `body_end` (**fixed**) | A container `div`; raw HTML in `head` parses before `<body>` |

`head` snippets are emitted in the document head and apply on first paint; `body_end` snippets
render just before `</body>`, after the page content. Both affect only the public site —
**`/admin` never receives snippets**, since it shares the origin. Enabling a JS or HTML snippet
needs `customize:publish`, because that executes code for every visitor.

## Navigation items

`/admin/customize/nav`. Fields: `label`, `href`, `icon`, `sort`, `enabled`, `visibility`,
`min_level`. Visibility `public` shows to everyone including guests, `login` only to signed-in
users, and `level` to those at or above `min_level` (which must be > 0).

Items render in the sidebar's "Navigation" group. **An empty table renders nothing** and the
built-in navigation stays, so clearing the config never leaves the site without navigation.

## Home modules

`/admin/customize/home`. Kinds are `latest` and `hot` (config `{"limit":5}`, range 1–20), `spaces`
(no config), and `html` (config `{"html":"<div>...</div>"}`). Fields also include `title`, `sort`,
`enabled`, and the same `visibility` / `min_level` as navigation.

`config` must be valid JSON and falls back to per-kind defaults otherwise; `html` rejects empty
content. Modules render at the top of `/timeline` in `sort` order, and with nothing configured no
block appears at all.

## Custom pages

`/admin/customize/pages`, served at `/p/{slug}` with Markdown content. Fields: `slug` (unique, no
`/ ? # &` or spaces, no collision with top-level routes such as `admin`, `api`, `login`),
`title`, `content`, and `status` (`draft` / `published`).

A draft returns **404, not 403**, so it does not advertise its existence. Publish and unpublish
use `POST /api/v1/admin/customization/pages/{id}/publish`.

## Graded application: machine writes force drafts

| Source | Behaviour |
|--------|-----------|
| Operator in the admin UI | Written exactly as requested |
| Machine credential (API key, OAuth client, MCP) | High-risk content is **forced to draft or disabled** |

High-risk means `js` / `html` snippets, navigation items, `html` modules, and custom pages —
navigation because `href` may be a `javascript:` URL. CSS snippets, tokens, and other module
kinds are unaffected. A credential holding `customize:publish` skips the rule; the check reads
that credential's scopes, never inherited roles. The response says what happened:

```json
{ "snippet": { "...": "..." }, "applied": false, "reason": "机器凭证未持 customize:publish 权限，已保存为草稿，需在后台启用" }
```

## Audit and rollback

Every write records an audit entry (`/admin/audit-logs`) named like `customize.snippet.update` or
`customize.page.publish`. The detail is a **change summary** — old and new length plus the first
120 characters — enough to find what changed without bloating the table.

There is no one-click revert. Roll back per area: disable a snippet (fastest), delete or clear
theme tokens, edit navigation and module fields back, or unpublish a page. Stop the bleeding
first, then confirm timing in the audit log.

## Verify

```bash
# Public aggregate: readable anonymously, all sections present
curl -s -X POST http://localhost:8080/api/v1/customization/public -d '{}' \
  | jq '{nav: (.data.nav|length), modules: (.data.home_modules|length)}'
# Custom page: published returns data, draft returns 404
curl -s http://localhost:8080/api/v1/pages/about | jq '.code'
```

In the browser, editing a token changes `<style id="fc-theme-overrides">`; `head` snippets appear
in `<head>`, and `body_end` ones at the end of the page source with `fc-custom-*` ids.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| A JS snippet does not run | Not enabled, or stored as a draft by a machine write | Turn it on in `/admin/customize/snippets` |
| A machine call returns `applied: false` | Missing `customize:publish` scope | Expected; enable by hand |
| `/admin` styling changed too | Injection is impossible there — admin receives no snippets | Check extensions or global CSS |
| Token save rejected | Name is not `--xxx`, or the value has `; { } < >` | Fix it, or use a CSS snippet |
| A custom page 404s | Still `draft` | Call the publish endpoint |
| Slug rejected | Reserved word or an existing page | Pick another slug |
