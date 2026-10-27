# 启动 CDEU 工作流（CDEU/Workflow/Run）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.cdeu_*`

手动启动一次完整 CDEU 工作流。服务端创建独立 `MANUAL` 运行实例后，在后台按 `Phase → Process → JOB → Step` 顺序执行所有直接 JOB；导出阶段的 4 个内嵌 JOB 只触发一次 `RUN_DUE_EXPORT_JOBS`。

## POST /CDEU/Workflow/Run

请求体（可选）：

```json
{ "operator": "web" }
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| operator | string | 否 | 操作人，缺省 `web` |

响应：

```json
{
  "code": 202,
  "message": "已启动，后台按阶段顺序执行",
  "data": { "run_id": 2 }
}
```

规则：

- 同一天已有 `PENDING` / `READY` / `RUNNING` 工作流时返回 `409`，避免重复启动；
- 执行顺序：历史行情收集 → 历史行情去重 → 导出决策 → 月度行情导出（含补丁）→ 月度行情上传 → 归档复核；
- 导出 JOB 仍受低峰窗口约束，窗口外快速跳过，其他阶段继续执行；
- 运行状态通过 `GET /CDEU/Overview` 查看；
- 成功启动写一条管理员审计（`job_kind=MANUAL_ADMIN_EDIT`）。

兼容路径：

```text
POST /CDEU/Process/Run
```

行为与 `/CDEU/Workflow/Run` 一致，保留给旧调用方；新前端统一使用 `/CDEU/Workflow/Run`。

错误：

- `409`：已有运行中的 CDEU 工作流；
- `500`（业务码 `2006`）：创建运行或查询定义失败。
