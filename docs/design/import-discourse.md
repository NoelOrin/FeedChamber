# Discourse JSON 导入（MVP）

## 用法

```bash
cd app/server
go run . import-discourse --dir /path/to/discourse-export
```

可选：

```bash
go run . import-discourse --dir /path/to/export --source discourse
```

## 输入文件

导出目录需包含：

| 文件 | 映射 |
|------|------|
| `users.json` | → `users` + `credentials` |
| `categories.json` | → `spaces` |
| `topics.json` | → `posts` |
| `posts.json` | → `contents`（首帖为 root content，后续为回复） |

JSON 可为数组，或带一层对象包装（importer 会尝试解析首个数组字段）。

## 字段映射

| Discourse | Feed Chamber |
|-----------|-----------|
| user.id | 新 ULID，`import_maps` 保留 external_id |
| category | space |
| topic | post |
| post #1 | content root |
| replies | contents（parent/root） |

密码：随机生成 + `meta must_change_password:{userID}=true`（若 meta 表可用）。

## 幂等

表 `import_maps`：`(source, entity_type, external_id) → local_id`。

重复执行同一目录不会重复创建已映射实体。

## 限制

- MVP 仅 SQL 后端（sqlite/mysql/postgres）
- 不导入附件、权限组、主题标签、私信
- 邮箱缺失时使用 `imported+{id}@example.invalid`
