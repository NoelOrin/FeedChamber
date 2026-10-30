---
title: 下载 openapi.json
description: 完整 OpenAPI 规格（由 handler 注解生成）
sidebar:
  order: 4
---

完整规格由 handler 上的 swag 注解经 `make swag` 生成，运行时也可在 `/swagger/` 浏览。

<div class="download-card">

### openapi.json

313 个 path / 314 个操作，Swagger 2.0 格式。

[下载 openapi.json](/openapi.json)

</div>

也可以直接拉运行中实例的：

```bash
curl -s http://localhost:8080/swagger/doc.json -o openapi.json
```

:::note[为什么站上只放下载，不做在线浏览]
313 个端点全量渲染会让页面很重，且规格随代码天天变。
按域读的指南见 [按域查端点](/api/domains/)；需要精确字段时再查这份 JSON。
:::
