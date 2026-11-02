---
title: 调用示例
description: 从登录到发帖、评论、上传、订阅 SSE、后台封禁的完整 curl 与 TypeScript 示例
---

这一页是可以直接跑的一条链路：登录拿 sid → 读列表 → 发帖 → 评论 → 上传 → 读通知 →
订阅实时流 → 管理端封禁与改权限。字段名都取自实际实现，可以照抄。前置条件是服务在
`http://localhost:8080` 启动、有普通用户与管理员账号、本机有 `curl` 与 `jq`。

## 1. 登录并读列表

```bash
BASE=http://localhost:8080/api/v1

USER_TOKEN=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"login":"alice","password":"alice-password"}' | jq -r '.data.access_token')

curl -s -X POST "$BASE/posts/list" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"page":1,"page_size":20,"sort_by":"latest"}' | jq '.data | {total}'
```

`login` 可以是用户名或邮箱，登录限流 10 次/分钟。响应是 `data: {items,total}`；筛选
字段有 `tag_name`、`author_id`、`space_id`、`space_slug`、`status`、`sort_by`。
**分页字段是 `page_size`**，不是 `pageSize`。

## 2. 发帖与评论

```bash
POST_ID=$(curl -s -X POST "$BASE/posts/create" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"第一帖","content":"正文支持 Markdown","space_id":""}' | jq -r '.data.id')

curl -s -X POST "$BASE/posts/$POST_ID/contents/create" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d '{"body":"第一条评论","parent_id":""}' | jq '.data.id'
```

发帖字段是 `title`、`content`、`space_id`（下划线，不是 `spaceId`），成功返回 201。
`status` 由审核模式决定：`async` 下通常 `approved`，`manual` 下是 `pending`（只有作者
与审核者能读到）。发帖限流 10 次/分钟，评论 30 次/分钟，回复时 `parent_id` 填父评论。

## 3. 上传

路径取决于存储后端：`s3` 用预签名直传，`local`（默认）只能 multipart。默认只允许
`image/jpeg`、`image/png`、`image/gif`、`image/webp`，上限 `upload_max_size`（10 MB）。

```bash
PRESIGN=$(curl -s -X POST "$BASE/upload/presign" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"file_name":"logo.png","content_type":"image/png"}')

UPLOAD_ID=$(echo "$PRESIGN" | jq -r '.data.uploadId')

# 按 data.fields 的键值把文件 POST 到 data.url（对象存储直传）
curl -s -X POST "$(echo "$PRESIGN" | jq -r '.data.url')" \
  -F 'key=…' -F 'policy=…' -F 'file=@./logo.png'

curl -s -X POST "$BASE/upload/confirm" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d "{\"upload_id\":\"$UPLOAD_ID\"}" | jq '.data'
```

本地存储则用 `POST $BASE/upload` 加表单字段 `file` 一次传完。

:::caution[预签名响应是 camelCase，确认请求是 snake_case]
预签名返回 `uploadId`，而 `/upload/confirm` 的入参是 `upload_id`。照抄字段名最容易在
这一步踩空，报 `2001`。
:::

## 4. 通知与实时流

```bash
curl -s -X POST "$BASE/notifications/list" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"page":1,"page_size":20,"type":"","read":""}' | jq '.data.total'
curl -s -X POST "$BASE/notifications/unread_count" -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' -d '{}' | jq '.data.count'
curl -N -X POST "$BASE/notifications/stream" \
  -H "Authorization: Bearer $USER_TOKEN" -H 'Accept: text/event-stream'
```

`type` 与 `read` 留空表示不过滤。未读数接口返回 `{"count":n}`，SSE 推的是
`{"unread":n}`——字段名不同，别复用同一个解析器。详见 [实时事件](/api/realtime/)。

## 5. 管理端：封禁与改权限

```bash
ADMIN_TOKEN=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"admin-password"}' | jq -r '.data.access_token')

# 永久封禁；临时禁言改用 duration_minutes（分钟）
curl -s -X POST "$BASE/user/$USER_ID/banned" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"reason":"刷屏","is_permanent":true}'
# 改角色会吊销该用户全部会话
curl -s -X POST "$BASE/user/$USER_ID/role/update" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"moderator"}'
# 覆盖某个角色的权限集合
curl -s -X POST "$BASE/permission/sync" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"role":"moderator","permissions":["post:read","post:pin","report:manage"]}'
```

封禁要 `user:update`，改权限要 `role:manage`。`is_permanent` 为 false 时必须给
`duration_minutes` 或 `expires_at`。

## TypeScript 版本

```ts
const BASE = '/api/v1';
let sid: string | null = null;

async function call<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(sid ? { Authorization: `Bearer ${sid}` } : {}),
    },
    credentials: 'include', // 浏览器里 HttpOnly cookie 也算认证
    body: JSON.stringify(body ?? {}),
  });
  if (res.status === 401) {
    // 401 → 刷新取新 sid 后重试一次；新 sid 生效即旧值失效
    const r = await fetch(`${BASE}/auth/refresh`, { method: 'POST', body: '{}' });
    const j = await r.json();
    if (j.code !== 0) throw new Error('session expired');
    sid = j.data.access_token;
    return call<T>(path, body);
  }
  const json = await res.json();
  if (json.code !== 0) throw Object.assign(new Error(json.msg), { code: json.code });
  return json.data as T;
}

sid = (await call<{ access_token: string }>('/auth/login', { login: 'alice', password: 'p' })).access_token;
await call('/posts/list', { page: 1, page_size: 20 });
const post = await call<{ id: string }>('/posts/create', { title: '示例', content: '正文', space_id: '' });
await call(`/posts/${post.id}/contents/create`, { body: '第一条评论', parent_id: '' });
await call('/notifications/unread_count');
const { uploadId } = await call<{ uploadId: string }>('/upload/presign', {
  file_name: 'logo.png',
  content_type: 'image/png',
});
await call('/upload/confirm', { upload_id: uploadId }); // snake_case
```

SSE 不能用 `call`：它返回的是流，要单独读 `response.body`。

## 验证

发帖后 `/posts/list` 的 `total` 应加一；封禁生效后该用户的写请求返回 404（不是 403）。

## 常见失败

| 现象 | 原因 | 处理 |
|------|------|------|
| `2001` 且提示内容为空 | 没发 body 或字段名写错 | 至少发 `{}`；核对 `space_id` / `page_size` |
| 发帖后列表看不到 | 审核模式 `manual`，帖子处于 `pending` | 管理员过审，或改用 `async` |
| 上传确认 `2001` | 传了 `uploadId` 而不是 `upload_id` | 按 snake_case 传 |
| 刷新后 `1014` | 用了旧 sid | 采纳 `/auth/refresh` 返回的新 token |
| 写接口 `429` | 发帖 10 次/分、评论 30 次/分 | 退避重试 |
