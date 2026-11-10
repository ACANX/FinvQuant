# USC 映射规则（UscMapping）

> 菜单 key：`quote-realtime-usc-mapping`
> 路由：`/Meta/Finv/Quote/Realtime/UscMapping`
> 所属：导航「行情 → 实时行情 → USC 映射规则」（紧跟「证券参数同步」之后）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/UscMappingView.vue`
> 关联需求：11072（建表 + 后端预演/应用）、11073（页面与菜单）
> 关联接口：[`API/UscMap.md`](../../../API/UscMap.md)、契约 [`API/UscMap.yml`](../../../API/UscMap.yml)
> 关联数据表：`futu.finv_futu_usc_rule` / `futu.finv_futu_usc_rule_item` / `futu.finv_futu_usc_map` /
> `futu.finv_futu_usc_rule_log` / `archive.finv_usc_reuse_snapshot`（迁移 `V300157`）

## 1. 用途

维护「富途代码 → Finv 内部证券代码（usc）」的映射规则，并把规则算出的结果落到
`futu.finv_futu_usc_map`、回写 `futu.finv_futu_secu_key.usc`。

- 规则改完**先预演**，看差异与实际落库口径一致后再应用；
- 应用是异步运行（`run_id` + 日志），同一时刻只允许一个；
- 定时同步默认关闭（`sys_config 20015` 为空），需要时由运维填回 cron。

## 2. 页面结构

### 2.1 头部

| 控件 | 说明 |
|------|------|
| 规则版本 / 条目数 / 键表匹配率 / 定时同步 | `GET .../UscMap/Status` 的快照 |
| 刷新 | 重新拉状态、规则、条目、日志 |

### 2.2 Tab 1「规则与条目」

| 区域 | 内容 |
|------|------|
| 左：规则层 | 9 条规则（`rule_no` / 名称 / 类型 / priority / 作用域 / 条目数 / `rule_desc`），点击切换右侧条目 |
| 右：条目层 | 按规则过滤的条目表：市场 / 品种 / 代码 / 正则 / 动作 / 取值 / 状态 / 说明 / 出处；支持新增、编辑、停用 |

- 保存时 `item_desc`（条目说明）**必填**，否则后端拒绝；
- 停用不删行：置 `RETIRED` 并写失效日，可再编辑回 `ACTIVE`；
- `status` / `scope_env` / 生效区间都可在对话框里改；
- 编辑对话框的**动作 / 状态 / 品种**是下拉（品种可自由填写，兼容历史值），选项一律按
  「`值[中文]`」风格显示，如下表。

| 字段 | 选项（`中文[值]`） |
|------|--------------------|
| 动作 `action` | `取证券代码[CODE]`、`取富途全码[FTMM]`、`取固定字面量[LITERAL]`、`按正则替换[REWRITE]`、`沪市指数四位码[SH4]`；留空 = 用规则默认动作 |
| 状态 `status` | `生效[ACTIVE]`、`待生效[PENDING]`、`已停用[RETIRED]` |
| 品种 `instrument` | `股票[stock]`、`ETF / 交易所交易基金[etf]`、`指数[index]`、`板块 / 行业分类[sector]`、`大盘 / 市场整体[market]`、`加密货币[crypto]`、`外汇[forex]`、`期货[futures]`；留空 = 通配任意品种 |

条目列表里这三列同样显示成 `中文[值]`（未知值原样显示，避免信息丢失）。

### 2.3 Tab 2「预演与应用」

| 操作 | 行为 |
|------|------|
| 预演差异 | `GET /Diff/Preview`：返回新增/更新/退市/复用/未变/冲突升级计数 + `preview_token`（响应只带前 200 行） |
| 明细分页 | `GET /Diff/Page`：按类别（全部/新增/更新/退市）分页浏览**完整明细**；每页 50 条，页大小上限 200。明细缓存在服务端，30 分钟或重新预演后失效 |
| 立即应用 | 只有预演出差异才可用；弹窗填原因后 `POST /Sync/Run`（带令牌），随后 800ms 轮询运行快照 |
| 立即执行一次 | 定时任务的「手动跑一遍」：内部仍是预演取令牌 → 确认 → 应用，与定时执行内容完全相同 |
| 冲突 | 预演报冲突（若有）时应用会被后端拒绝，需先修规则 |

页面最下方是「**定时自动执行**」卡片（`GET/POST /Schedule`）：只决定上面这套动作多久自动跑一次，
不是额外步骤；可改 cron（6 段）或一键关闭，并显示下次执行时间。

明细列含义：`insert`=结果表没有、`update`=`usc` 变了、`retire`=键表已无该证券（退市）、
`reuse_of_ftmm`=这个 `usc` 上一个占用者。

### 2.4 Tab 3「审计日志」

`finv_futu_usc_rule_log` 倒序分页：时间 / 对象（`rule_item` / `map`）/ 动作 / 主键 / 操作人 / 原因 / 变更摘要。

## 3. 注意事项

- 应用会**真实改写** `futu.finv_futu_secu_key.usc`；生产库执行前先把预演明细交业务确认；
- 首次应用（结果表为空）时「新增」等于键表行数（当前 73,212），并会改写约 1.68 万行键表 `usc`——
  这正是默认关闭定时同步的原因（迁移 `V300158`）；
- `usc` 复用会先把下游历史行归档到 `archive.finv_usc_reuse_snapshot`，归档失败则整次回滚。
