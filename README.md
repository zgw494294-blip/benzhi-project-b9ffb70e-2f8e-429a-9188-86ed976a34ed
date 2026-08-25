# 舞台吊挂装台验收与演出放行系统

本项目面向剧场舞台机械技师、载荷试验记录员和演出技术负责人，将装台批次、吊点构件、载荷试验、偏差整改、技术复核与演出放行凭据收束为一条可追溯流程。系统由单个 Go 服务提供响应式浏览器工作台和同源 JSON API，数据保存在本地 SQLite。

## 业务流程

1. 创建装台验收批次，登记场馆、舞台区域、计划演出时间和负责人；草稿阶段可继续维护这些信息。
2. 登记吊点、葫芦序列号、钢丝绳规格、额定载荷和计划载荷。系统校验编号唯一，并要求计划载荷不超过额定载荷的 80%。
3. 锁定待测清单后逐点记录目标载荷、实测载荷、保持时长和位移。不合格试验必须同步登记偏差并阻断送审。
4. 提交整改证据后执行关联复测。失败复测会保留为新的试验记录并继续阻断；合格复测关闭偏差。
5. 全部吊点最新试验合格且偏差关闭后，由技术负责人批准。系统保存不可变技术复核记录并冻结规范化配置摘要。
6. 技术负责人签发递增序号的不可变放行凭据，可按批次或序号查询，并可重新计算 SHA-256 摘要核验状态。

所有写请求都携带 `expectedVersion` 和 `idempotencyKey`。版本不一致返回 `409 version_conflict`；相同操作的安全重试会返回首次提交结果，并设置 `Idempotency-Replayed: true`。

## 架构与数据

- `cmd/server`：监听配置、依赖装配、服务生命周期和有界 HTTP selfcheck。
- `internal/domain`：验收聚合、状态机、载荷规则、冻结摘要和完整性校验。
- `internal/application`：用例编排、版本与幂等契约、查询服务。
- `internal/store`：SQLite 迁移、事务仓储、审计与启动完整性检查。
- `internal/httpapi`：版本化 JSON API、输入限制、错误映射和响应协商。
- `internal/webui`：内嵌的原生 HTML、CSS 和 JavaScript 工作台。

SQLite 默认文件为 `rigging-release.db`，启动时启用外键和 WAL，并执行可重复迁移。冻结快照、技术复核、放行凭据、载荷试验和审计事件由数据库触发器保护为只追加记录。

## 构建与运行

要求 Go 1.22 或更高版本。

```text
go build ./cmd/server
go run ./cmd/server -addr=127.0.0.1:19137
```

浏览器访问 `http://127.0.0.1:19137/`。默认监听地址是 `127.0.0.1:19137`，可通过 `-addr=127.0.0.1:<port>` 修改。未显式传入 `-addr` 时，也可把 `PORT` 设置为 1024 至 65535 的端口号，服务会绑定 `127.0.0.1:<PORT>`。服务拒绝非回环地址。

可用 `-db=<path>` 指定 SQLite 文件。生产启动不应使用 `-selfcheck`。

## 测试与自检

```text
go test ./...
go run ./cmd/server -addr=127.0.0.1:19137 -selfcheck
```

selfcheck 使用临时内存数据库，但会先真实绑定指定监听地址，再通过公开 HTTP API 完成创建、吊点登记、锁定、失败试验、偏差整改、合格复测、批准冻结、凭据签发、摘要核验和幂等重放，随后主动关闭服务并退出。

## HTTP API

API 基础路径为 `/api/v1`，请求和响应使用 `application/json`，单个请求体上限为 1 MiB。

- `GET /batches`、`POST /batches`：按 `ownerName`、`status`、`stageZone`、`from`/`to`（RFC3339）组合查询和创建批次；列表返回覆盖率、未测吊点、未关闭偏差、失败尝试和风险标记，并支持 `limit` 与稳定 `cursor` 分页。
- `GET /batches/{batchId}`、`PATCH /batches/{batchId}`：查询和维护草稿批次。
- `POST /batches/{batchId}/points`（或 `/points/batch`）、`POST /batches/{batchId}/lock`：逐条或以 `points`/`items` 数组原子登记吊点；提交 `precheck: true` 可只返回逐行校验和承载汇总，不写入数据库。
- `POST /batches/{batchId}/tests`：记录首次试验或普通试验。
- `POST /batches/{batchId}/deviations/{deviationId}/remediation`：提交整改证据。
- `POST /batches/{batchId}/deviations/{deviationId}/retest`：执行关联复测。
- `POST /batches/{batchId}/approval`：技术复核并冻结配置。
- `POST /batches/{batchId}/credential`：签发不可变放行凭据。
- `GET /batches/{batchId}/verification`：重算并核验批次凭据摘要。
- `GET /batches/{batchId}/audit`：查询只追加审计轨迹。
- `GET /batches/{batchId}/tests`：按吊点编号和 `result` 查询稳定的逐点试验历史与趋势汇总。
- `POST /batches/{batchId}/deviations/{deviationId}/update`（或 `/escalate`、`/confirm`）：升级偏差等级并确认责任人。
- `GET /batches/{batchId}/approval-preview`：返回批准前的清单差异摘要和阻断原因。
- `GET /credentials/{serial}`：按递增序号查询凭据。
