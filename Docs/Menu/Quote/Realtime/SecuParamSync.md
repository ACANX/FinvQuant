# 证券参数同步（SecuParamSync）

> 菜单 key：`quote-realtime-secu-param-sync`
> 路由：`/Meta/Finv/Quote/Realtime/SecuParamSync`
> 所属：导航「行情 → 实时行情 → 证券参数同步」（紧跟「缺省汇率」之后）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/SecuParamSyncView.vue`
> 关联需求：11042（菜单与页面）、11043（同步实现）、11044（源表名可配 20014）；连接配置 20013（需求 11041）
> 关联接口：[`API/Cdeu/SecuParamSync.md`](../../../API/Cdeu/SecuParamSync.md)
> 关联数据表：目标 `futu.finv_futu_secu_key`（V300137 建表 / V300139 补 `stock_id`+`usc` / V300141 改名）；
> 源 Supabase `public.finv_quote_secu_key`（表名可配，默认与上游同名）

## 1. 用途

把上游 Supabase 的**证券键表**（`ftmm_symbol` 键映射 + 采集状态）分批增量同步到本地
`futu.finv_futu_secu_key`，供后续富途行情采集按 `ftmm_symbol` 取数。

- 只由本页**按钮手动触发**，没有定时任务；
- 单向只读上游 → 写本地：不删除本地行，不清空本地专有列 `stock_id` / `usc`；
- 连接配置（`PROJECT_ID|APIKEY`）在系统配置 **20013** 里维护，本页只展示状态，不回显完整 key；
- 源表名存系统配置 **20014**，默认 `finv_quote_secu_key`，在本页可直接改。

## 2. 页面结构

### 2.1 头部操作

| 控件 | 说明 |
|------|------|
| 刷新状态 | `GET .../SecuKey/Status` 重新读配置与目标表行数（不访问 Supabase） |

### 2.2 连接与目标表（第一张卡片）

| 区块 | 展示 |
|------|------|
| Supabase 连接（20013） | `已配置`（success chip）/ `未配置`（warning chip）；已配置时显示项目 ID 与脱敏 key 前缀（`APIKEY sb_publi…（已脱敏）`）；未配置时显示 `config_error` 原文 |
| 本地目标表 | `futu.finv_futu_secu_key` + `共 N 行`（tonal chip）+ 最近变更时间（`max(gmt_update)`） |

### 2.3 同步参数（第二张卡片）

| 控件 | 说明 |
|------|------|
| Supabase 源表名 | 文本输入，默认 `finv_quote_secu_key`；可写 `schema.table` 两段，只允许字母/数字/下划线；与已生效值不同时才允许保存 |
| 每批行数 | 下拉 `500 / 1000 / 2000 / 5000`，默认 1000 |
| 保存源表名 | `POST .../SecuKey/Config` 写系统配置 20014，成功后 toast 与状态刷新 |
| 立即同步 | `POST .../SecuKey/Sync`（超时 15 分钟）；20013 未配置时按钮禁用；同步中按钮 loading 并提示「请勿关闭页面」 |

### 2.4 同步结果（第三张卡片，仅同步后出现）

顶部摘要：`源表 <表名> · 共 N 行 · M 批 · 耗时`；
chip 行：`拉取 / 新增 / 更新 / 未变 / 跳过（有才显示）/ status 归一（有才显示）`；
下方表格为逐批日志：`批次 / offset / 拉取 / 新增 / 更新 / 跳过 / 耗时`。

### 2.5 说明卡片（页面最底部唯一「说明」卡片，ExplainCardSpec）

三段：**表口径**（源/目标表、参与同步的列、`stock_id`/`usc` 不同步）、
**增量口径**（缺则插入、变了才更新、未变不改写、不删除本地行）、
**配置与安全**（20013 格式与脱敏、20014 需保存才生效、`status` 非法归一为 `PENDING`）。

## 3. 交互细节

| 场景 | 表现 |
|------|------|
| 页面打开 | 自动 `GET Status`；失败弹 error toast 并在页面显示 error alert |
| 20013 未配置 | 状态显示「未配置」；点「立即同步」弹 warning toast 并直接返回（不发请求） |
| 源表名未改动 | 「保存源表名」按钮禁用（避免无意义写入） |
| 同步成功 | success toast（新增/更新/未变条数）+ 结果卡片 + 自动刷新状态 |
| 同步失败 | error alert 展示 `message` 原文（含上游 `PGRST205` / 鉴权提示）+ error toast |
| 同步进行中 | 「保存源表名」禁用，避免与进行中的同步口径不一致 |

## 4. 与其他页面的关系

- 「缺省汇率」之后新增本菜单（需求 11042 指定位置）；
- 本页只写 `futu.finv_futu_secu_key`，不改 `cdeu.*` 快照表，也不触发任何采集；
- 后续按 `ftmm_symbol` 采集行情时以本表为键来源（`status`/`priority`/`url` 供采集侧使用）。
