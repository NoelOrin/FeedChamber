# Mock 测试替身

内存测试替身（test doubles），用于单元测试。包名 `mock`。

## 实现

| Mock | 说明 |
|------|------|
| `UploadRepository` | 内存 `map[string]*model.Upload`，支持 Create/FindByID/ListByUser/Delete，可注入 SetFindErr/SetDeleteErr |
| `StorageProvider` | 跟踪 PutKeys/DeleteKeys 调用，可配置 PutErr/DeleteErr |
| `SettingService` | 内存 `map[string]string`，支持 Get/GetAll/Set/BatchSet，Builder 模式 `WithSetting(key, value)` |
| `TrustRepository` | 内存 `repository.TrustRepository`：配置四表 + 人工调整；指标由测试直接灌（`Metrics` / `Daily` / `Streak` / `ActiveDays` / `Upheld`），`SaveLevelSnapshot` 记录最后一次调用供断言 |
