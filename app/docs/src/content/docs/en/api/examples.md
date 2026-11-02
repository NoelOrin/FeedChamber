---
title: API Examples
description: End-to-end curl and TypeScript examples for login, listing, posting, commenting, uploading, SSE and admin actions
---

One runnable path: log in, list posts, create a post, comment, upload, read notifications,
subscribe to the stream, then ban a user and change permissions. Field names come from the
implementation; prerequisites are a server on `http://localhost:8080`, a user, an admin
account, `curl` and `jq`.

## 1. Log in and list posts

```bash
BASE=http://localhost:8080/api/v1
USER_TOKEN=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"login":"alice","password":"alice-password"}' | jq -r '.data.access_token')

curl -s -X POST "$BASE/posts/list" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"page":1,"page_size":20,"sort_by":"latest"}' | jq '.data | {total}'
```

`login` accepts a username or email (10 per minute). The payload is `data: {items,total}`;
filters include `tag_name`, `author_id`, `space_id`, `sort_by`. **Paging is `page_size`**, not
`pageSize`.

## 2. Create a post and a comment

```bash
POST_ID=$(curl -s -X POST "$BASE/posts/create" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"First post","content":"Markdown body","space_id":""}' | jq -r '.data.id')

curl -s -X POST "$BASE/posts/$POST_ID/contents/create" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d '{"body":"First comment","parent_id":""}' | jq '.data.id'
```

Fields are `title`, `content`, `space_id` (not `spaceId`); success is HTTP 201. `status`
follows the moderation mode: usually `approved` under `async`, `pending` under `manual`
(visible only to the author and reviewers). Posting is limited to 10 per minute, commenting to
30.

## 3. Upload

`s3` takes a presigned direct upload; `local` (the default) takes multipart on `POST /upload`
with the form field `file`. Allowed types default to JPEG/PNG/GIF/WebP, capped at
`upload_max_size` (10 MB).

```bash
PRESIGN=$(curl -s -X POST "$BASE/upload/presign" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"file_name":"logo.png","content_type":"image/png"}')
UPLOAD_ID=$(echo "$PRESIGN" | jq -r '.data.uploadId')

curl -s -X POST "$BASE/upload/confirm" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d "{\"upload_id\":\"$UPLOAD_ID\"}" | jq '.data'
```

Before confirming, POST the file to `data.url` with the keys from `data.fields`.
Note the case change: presign replies `uploadId`, while `/upload/confirm` takes `upload_id`.

## 4. Notifications and the stream

```bash
curl -s -X POST "$BASE/notifications/unread_count" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d '{}' | jq '.data.count'
curl -N -X POST "$BASE/notifications/stream" \
  -H "Authorization: Bearer $USER_TOKEN" -H 'Accept: text/event-stream'
```

Unread returns `{"count":n}` while the stream pushes `{"unread":n}` — different parsers.
See [realtime](/en/api/realtime/).

## 5. Admin: ban and permissions

```bash
ADMIN_TOKEN=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"admin-password"}' | jq -r '.data.access_token')
curl -s -X POST "$BASE/user/$USER_ID/banned" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"reason":"spam","is_permanent":true}'
curl -s -X POST "$BASE/permission/sync" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"role":"moderator","permissions":["post:read","post:pin","report:manage"]}'
```

Banning needs `user:update`, permission changes need `role:manage`; `POST /user/{id}/role/update`
with `{"role":"..."}` also revokes that user's sessions.

## TypeScript

```ts
const BASE = '/api/v1';
let sid: string | null = null;
async function call<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(sid ? { Authorization: `Bearer ${sid}` } : {}) },
    credentials: 'include', // the HttpOnly cookie authenticates too
    body: JSON.stringify(body ?? {}),
  });
  // 401: POST /auth/refresh returns a new sid and kills the old one — adopt it, then retry
  const json = await res.json();
  if (json.code !== 0) throw Object.assign(new Error(json.msg), { code: json.code });
  return json.data as T;
}
sid = (await call<{ access_token: string }>('/auth/login', { login: 'alice', password: 'p' })).access_token;
const post = await call<{ id: string }>('/posts/create', { title: 'Example', content: 'Body', space_id: '' });
await call(`/posts/${post.id}/contents/create`, { body: 'First comment', parent_id: '' });
const { uploadId } = await call<{ uploadId: string }>('/upload/presign', { file_name: 'logo.png', content_type: 'image/png' });
await call('/upload/confirm', { upload_id: uploadId }); // snake_case
```

SSE cannot use `call` — read `response.body` directly.

## Verification and failures

A successful post raises `total` from `/posts/list` by one; a banned user's writes return 404.
`2001` with an empty message means an empty body or a misspelled field, and `confirm` returning
`2001` means `uploadId` was sent instead of `upload_id`.
