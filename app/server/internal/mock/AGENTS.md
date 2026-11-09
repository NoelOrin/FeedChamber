# Mock 测试替身

内存测试替身（test doubles），用于单元测试。包名 `mock`。

## 实现

| Mock | 说明 |
|------|------|
| `UploadRepository` | 内存 `map[string]*model.Upload`，支持 Create/FindByID/ListByUser/Delete，可注入 SetFindErr/SetDeleteErr |
| `StorageProvider` | 跟踪 PutKeys/DeleteKeys 调用，可配置 PutErr/DeleteErr |
| `SettingService` | 内存 `map[string]string`，支持 Get/GetAll/Set/BatchSet，Builder 模式 `WithSetting(key, value)` |
| `TrustRepository` | 内存 `repository.TrustRepository`：配置四表 + 人工调整；指标由测试直接灌（`Metrics` / `Daily` / `Streak` / `ActiveDays` / `Upheld`），`SaveLevelSnapshot` 记录最后一次调用供断言 |

> 2026-10-29：项目更名为 Feed Chamber（Go module `feedchamber`、npm `@feedchamber/*`、镜像 `feedchamber-*`）。

## 浏览记录 / 续读位置（2026-11-09）

- `TopicReadRepository` 补齐 `RecordView` / `DeleteByUserAndPost`（幂等）/ `DeleteByUser`；两路写入共用 `applyTopicRead` 骨架，只写自己负责的列、空游标保留旧值（与 `topic_read_sql.go` 的 `AssignmentColumns` 语义对齐，真正的列裁剪由 `repository/topic_read_test.go` 打在内存 sqlite 上）。
- `PostRepository` 补 `ListReadByUser`（此处仅返回空壳，历史出口的真实行为由 service 层测试跑 sqlite 覆盖）——接口新增方法后不补这个 stub，所有用到该 mock 的测试都会编译不过。
- **修掉 `ListByUser` 的前缀误匹配**：key 形状是 `userID + "\x00" + postID`，旧写法用裸前缀匹配会让 `u1` 命中 `u10\x00p`（把别人的记录算进来）。现统一 `strings.HasPrefix(key, userID+"\x00")`，`DeleteByUser` 同用带分隔符的 prefix。
