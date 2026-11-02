---
title: Site Settings
description: Site name and URL, SEO metadata, registration modes, pagination, maintenance mode, and announcements
---

Site settings are keys in the `settings` table and apply immediately — no environment
variables, no restart (one pagination exception below). Admin entry: `/admin/settings`, with
five tabs. This page covers identity, SEO, registration, pagination, maintenance, announcements.

## Before you start

- Your account needs `setting:manage` and access to `/admin`; see
  [Authentication and Security](/configuration/authentication/).
- Raw access: `POST /api/v1/admin/settings` reads all, `/{key}/update` writes one, `/batch` many.

## Site identity

`/admin/settings?tab=site` → "Site Information".

| Key | Default | Purpose |
|-----|---------|---------|
| `site_name` | `Feed Chamber` | Site name; used in navigation, title suffix, and the TOTP issuer |
| `site_description` | `NoelOrin 的个人博客` | One-line description; fallback for `seo_description` |
| `site_logo` | empty | Logo URL; fallback for `og_image` |
| `site_url` | empty | Canonical base for sitemap, robots.txt, canonical links, og:url |

Set `site_url` to the full public address (with scheme, no trailing slash). Empty is allowed:
crawler output falls back to `http://localhost:8080` and the startup log warns.

## SEO and verification

Each field falls back to site identity, so filling fewer is valid:

| Key | Falls back to | Purpose |
|-----|---------------|---------|
| `seo_title` | `site_name` | Default site title, also the suffix on child page titles |
| `seo_description` | `site_description` | Search snippet and social card description |
| `seo_keywords` | seeded value | Comma-separated keywords meta tag |
| `og_image` | `site_logo` | `og:image` and Twitter card image |

`google_site_verification` and `baidu_site_verification` sit here too: paste each webmaster
console token and the matching meta tag is emitted. Both are public settings — they end up in
the HTML anyway. Whether other users' profiles are indexed is controlled by `user_profile_indexable`
(default `false`, emits `noindex`).

## Registration

`/admin/settings?tab=user` → "Users and Registration".

| Key | Default | Notes |
|-----|---------|-------|
| `register_enabled` | `true` | When off, registration is rejected with code 1013 |
| `register_mode=all` | default | Username or email, at least one |
| `register_mode=email` | — | Email required; a blank username comes from before `@` |
| `register_mode=username` | — | Username required |
| `register_mode=invite` | — | Valid invite code required; create codes in `/admin/invites` |
| `register_mode=approve` | — | Account starts as `pending_user` until approved |

Approval uses the row actions in `/admin/users` (or
`POST /api/v1/admin/users/{id}/approve_registration`): the role becomes `user` and the
account's sessions are revoked, so the user must sign in again.

## Pagination

`/admin/settings?tab=content` → "Posting and Comments" holds the pagination keys.

| Key | Default | Notes |
|-----|---------|-------|
| `page_default_size` | `20` | Default page size |
| `page_max_size` | `100` | Ceiling; larger requests fall back to the default |
| `article_page_size` | `20` | Posts per page in front-end lists |

:::caution[They apply at different times]
`article_page_size` is read from public settings on every page load, so a refresh is enough.
`page_default_size` and `page_max_size` are loaded into the server at startup — restart first.
:::

## Maintenance mode

Also under `?tab=content`.

| Key | Default | Notes |
|-----|---------|-------|
| `site_maintenance` | `false` | Non-admin writes return 5005 (HTTP 503) |
| `site_maintenance_message` | `站点维护中，请稍后再试` | Message shown to users |

The allow-list is deliberate: `admin` is unaffected, read-only browsing (GET plus read RPCs
such as `/list`, `/search`, `/detail`) keeps working, and health checks, login, registration,
session refresh, password recovery, `/swagger`, and OAuth/OIDC endpoints stay up.

## Announcements

A separate module at `/admin/announcements`, not a settings key. Publish and unpublish are
separate actions; with `announcement_auth_required=true`, reading requires a login.

| Field | Notes |
|-------|-------|
| Title / content | Content is Markdown |
| Display type | `1` toast, `2` top banner, `3` modal, `4` iframe embed (content holds the URL) |
| Position / sort | Six toast positions (default `top-right`); pinned items come first |
| Expires at | Stops showing after expiry |

## Verify

```bash
# Only the public subset is exposed; saved values show up here
curl -s -X POST http://localhost:8080/api/v1/settings/public \
  | jq '.data | {site_name, register_mode, site_url}'
```

Reload `?tab=site` to confirm the fields fill back in; check the home page title, social image,
and the domain in `/robots.txt` and `/sitemap.xml`.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| sitemap still uses localhost | Batch save failed, or no restart | Check the batch call returned `code: 0`; restart |
| Name or description does not change | Cached server-rendered page | Hard refresh; trust the API response |
| Invite mode keeps asking for a code | No invite code exists | Create one under `/admin/invites` |
