# USC 补丁（Patch）机制重构方案

> 编号：**202611-0002**　文档约定见 [README.md](README.md)
>
> 状态：**已落地**（需求 10931，`V300103` + 后端接口 + 前端页面 + JOB；P1 至 P6 全部已定，见 §9）
>
> 关联：[202611-0001-DataBaselineSyncProposal.md](202611-0001-DataBaselineSyncProposal.md)（附录 A 的 USC 编码冲突清单）
>
> 关联需求：10920、10921、10926、10927、10928

## 1. 问题

`cdeu.finv_quote_secu_kline_min` 里存在过时 `usc` 的行情记录，且**会持续产生**：

| 现状 | 数据 |
|------|------|
| 主表 `DISTINCT usc` | 495 个 |
| 其中 6 位旧形态（`SH000688` 这类） | 21 个，每个 1205 行 |
| 其中 4 位新形态（`SH0008` 这类） | 9 个，每个 241 行 |
| 新形态与旧形态同时存在同一指数的有 | 7 组 |

三个特点决定了"跑一次 UPDATE 就完事"行不通：

1. **旧 usc 会反复进来**：上游采集侧短时间不会立刻改口径，每次重新导入都会再写入 `SH000688`、`KC50` 这类旧代码。
2. **别名规则会继续增加**：`SHI` 改为 `SHCI`、`KC50` 改为 `STAR50` 之后还会有新的，规则必须能动态加，不能写死在代码里。
3. **直接改主键会撞主键**：主表主键是 `(ts, usc)`，若目标 usc 在同一 `ts` 已有行，`UPDATE ... SET usc` 直接违反唯一约束。

## 2. 目标

| 目标 | 说明 |
|------|------|
| 规则表持久化 | 别名映射写成表，可增删改、可停用、可审计，不写死在代码或迁移里 |
| 可重复执行 | 每次执行把当前所有生效规则应用到数据上，重复执行无副作用 |
| 主键冲突可控 | 用 upsert 处理 `(ts, usc)` 冲突，冲突有明确取值策略并记账 |
| 可观测 | 每次执行输出：扫描行数、迁移行数、冲突行数、丢弃行数、失败数 |
| 可手工干预 | 前端页面维护规则 + 手动触发 + 查看执行日志 |
| 不阻塞写入 | 不挂 `AFTER INSERT` 触发器（批量导入时逐行触发的代价太高） |

## 3. 规则表设计

新表 `cdeu.finv_quote_cdeu_usc_patch_rule`：

| 字段 | 类型 | 可空 | 默认 | 约束 | 说明 |
|------|------|------|------|------|------|
| `idn` | bigserial | 否 | 自增 | 主键 | 规则序号 |
| `usc_old` | text | 否 | 无 | 唯一（生效规则范围内） | 过时 usc，如 `SH000688` |
| `usc_new` | text | 否 | 无 | 非空 | 目标 usc，如 `SH0688` |
| `match_mode` | text | 否 | `exact` | `exact` 或 `prefix` | 匹配方式，首版只用 `exact` |
| `flag_enable` | char(1) | 否 | `'1'` | `'0'` 或 `'1'` | 是否生效 |
| `priority` | int | 否 | 100 | 非负 | 同一 `usc_old` 多条生效规则时取小者 |
| `note` | text | 否 | `''` | 无 | 规则来源与依据，如"需求 10927" |
| `gmt_create` | timestamptz | 否 | `now()` | 无 | 创建时间 |
| `gmt_update` | timestamptz | 否 | `now()` | 触发器维护 | 更新时间 |

关键约束：

```sql
-- 同一个过时 usc 同时只能有一条生效规则，否则迁移方向不确定
CREATE UNIQUE INDEX uq_finv_quote_cdeu_usc_patch_rule_usc_old
    ON cdeu.finv_quote_cdeu_usc_patch_rule (usc_old)
 WHERE flag_enable = '1';

-- 防止自指（usc_old 与 usc_new 相同会在执行时死循环）
ALTER TABLE cdeu.finv_quote_cdeu_usc_patch_rule
    ADD CONSTRAINT ck_finv_quote_cdeu_usc_patch_rule_not_self CHECK (usc_old <> usc_new);
```

停用而不删除：历史规则把 `flag_enable` 置 `'0'` 保留，便于追溯。

## 4. Patch 执行模型

### 4.1 冲突从哪来

以 `SH000688` 改为 `SH0688` 为例：

| usc | 日期 | 行数 |
|-----|------|------|
| `SH000688`（旧） | 20261009 至 20261026 | 1205 |
| `SH0688`（新） | 20261026 | 241 |

20261026 这天两个 usc 都有数据，直接 `UPDATE usc = 'SH0688'` 会在 `(ts, usc)` 上撞唯一约束。

### 4.2 用 upsert 规避

单批执行（按 `ts` 区间切批）：

```sql
WITH src AS (
    SELECT DISTINCT ON (p.usc_new, m.ts)
           p.usc_new AS usc, m.ts, m.date, m.time, m.open, m.high, m.low,
           m.close, m.vwap, m.volume, m.turnover, m.change_price,
           m.change_ratio, m.ext_field, m.last_close, m.remark,
           m.provider, m.gmt_update
      FROM cdeu.finv_quote_secu_kline_min m
      JOIN cdeu.finv_quote_cdeu_usc_patch_rule p
        ON p.usc_old = m.usc
       AND p.flag_enable = '1'
     WHERE m.ts >= $1 AND m.ts < $2
     ORDER BY p.usc_new, m.ts, p.priority
), moved AS (
    INSERT INTO cdeu.finv_quote_secu_kline_min
           (usc, ts, date, time, open, high, low, close, vwap, volume,
            turnover, change_price, change_ratio, ext_field, last_close,
            remark, provider)
    SELECT usc, ts, date, time, open, high, low, close, vwap, volume,
           turnover, change_price, change_ratio, ext_field, last_close,
           remark, provider
      FROM src
    ON CONFLICT (ts, usc) DO UPDATE
       SET open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
           close = EXCLUDED.close, vwap = EXCLUDED.vwap,
           volume = EXCLUDED.volume, turnover = EXCLUDED.turnover,
           change_price = EXCLUDED.change_price,
           change_ratio = EXCLUDED.change_ratio,
           ext_field = EXCLUDED.ext_field, last_close = EXCLUDED.last_close,
           remark = EXCLUDED.remark, provider = EXCLUDED.provider,
           gmt_update = now()
     WHERE cdeu.finv_quote_secu_kline_min.gmt_update < EXCLUDED.gmt_update
    RETURNING 1
)
DELETE FROM cdeu.finv_quote_secu_kline_min m
 USING cdeu.finv_quote_cdeu_usc_patch_rule p
 WHERE m.usc = p.usc_old
   AND p.flag_enable = '1'
   AND m.ts >= $1 AND m.ts < $2;
```

三个要点：

| 要点 | 作用 |
|------|------|
| `DISTINCT ON (p.usc_new, m.ts)` | 防止同一批里两个旧 usc 映射到同一个新 usc 且 `ts` 相同时触发 `ON CONFLICT cannot affect row a second time` |
| `ON CONFLICT ... WHERE gmt_update <` | 冲突时**较新的那份数据胜出**；等于时不覆盖，避免无意义写入 |
| `DELETE` 放在同一语句末尾 | 只删 `usc_old` 的行，刚插入的 `usc_new` 行不匹配删除条件，安全 |

### 4.3 分批与水位

| 项目 | 设计 |
|------|------|
| 分批键 | `ts` 区间，每批 5000 行 |
| 增量水位 | `sys.sys_config` 键 `USC_PATCH_WATERMARK`，记录已处理的最大 `ts` |
| 全量模式 | 忽略水位，从 `min(ts)` 扫到 `max(ts)`，菜单手动触发时可选 |
| 单批事务 | 一批一个事务，失败只回滚该批，进度可续 |

主表按 `ts` 有索引 `idx_finv_quote_usc_ts`，按区间扫不会全表扫。

## 5. 连带影响与处理

`usc` 是多个表的关联维度，改主表必须同步处理：

| 对象 | 影响 | 处理方式 |
|------|------|----------|
| `cdeu.finv_quote_cdeu_usc` | 旧 usc 的登记行存在、新 usc 可能不存在 | 从旧行复制生成新行（`ON CONFLICT DO NOTHING`），旧行保留不删（10920 的"永不删除"原则），标记 `note` |
| `cdeu.finv_quote_cdeu_quote_maturity_mon` | 按 usc 统计的成熟度会分裂成两份 | patch 完成后对涉及的 usc 触发一次重算（复用 `REFRESH_QUOTE_MATURITY` 的补全逻辑） |
| CDEU 归档产物（本地 `Full/`、`Incr/`、远端 key） | 旧 usc 的已导出文件会成为孤儿 | 不改历史文件；由 `CLEAN_EXPORT_STORAGE` 与保留策略自然淘汰；需要立即重刷时手工触发对应月份的导出 |
| BRS 增量分片 | 分片键是日期，与 usc 无关 | 无需处理 |
| 富途映射表 `futu.finv_futu_mapping_security` | 若映射到旧 usc，下次采集又会写入旧 usc | patch 后同步更新该表的 `finv_usc` |

最后一条是关键：**只要上游映射还指向旧 usc，patch 就是长期的、反复的**——这也正是本机制必须做成"规则表 + 定期执行"而不是一次性迁移的原因。

## 6. 页面与接口

### 6.1 菜单

放在「行情 → 收剔导传ETL」分组下，与「归档主数据与规则」并列：

| 菜单项 | 路径 | 说明 |
|--------|------|------|
| USC 补丁规则 | `/Meta/Finv/Quote/Cdeu/Usc/Patch` | 规则维护 + 手动执行 + 执行日志 |

### 6.2 页面

单页三块：

| 区块 | 内容 |
|------|------|
| 规则列表 | 表格展示 `usc_old`、`usc_new`、`match_mode`、`flag_enable`、`priority`、`note`、`gmt_update`；行内启用/停用、编辑、删除 |
| 手动执行 | 模式选择（增量或全量）、批大小、开始按钮；运行中显示进度（已处理 `ts` 区间、迁移行数、冲突行数） |
| 执行日志 | 复用现有 JOB 运行记录与日志（`run` 与 `run_log`），展示每批结果 |

### 6.3 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/List` | 规则列表（分页，查询参数小驼峰） |
| POST | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Save` | 新增或更新规则 |
| POST | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Enable` | 启用或停用 |
| POST | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Delete` | 删除规则 |
| POST | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Run` | 触发执行，写入 JOB 队列 |
| GET | `/API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/Preview` | 试算：返回受影响的 usc 与行数，不写库 |

JSON 体字段一律 `snake_case`（`ApiSpec` §3）。

### 6.4 JOB

复用 CDEU 现有 JOB 框架，新增一种：

| 项目 | 值 |
|------|-----|
| `Kind` | `PATCH_OUTDATED_USC` |
| 默认调度 | 每天一次，`DailyAt: "05:00"`（可配）；选 05:00 是为了在凌晨导入与 00:xx 的统计决策之后、06:xx 的清理之前执行 |
| 单次行为 | 从水位开始，按 `ts` 分批扫到 `max(ts)`，每批一次 upsert + delete |
| 日志内容 | 每批一行：`ts 区间、扫描行数、迁移行数、冲突覆盖行数、删除行数、耗时` |

## 7. 与现有机制的关系

| 现有机制 | 关系 |
|----------|------|
| `DiscoverUsc`（需求 10920） | patch 后主表的 `DISTINCT usc` 变了，下一轮发现会把 `usc_new` 补进登记表；`usc_old` 不会被新登记 |
| `RefreshSecurityMetadata`（10920） | 只刷新已登记 usc 的元数据，patch 本身不触碰 |
| `BackfillUscDefaults` | 新登记的 `usc_new` 走同一套推导，无需额外逻辑 |
| `finv_quote_cdeu_usc_day_incr_row_tpl` | `usc_new` 的 `day_expect_rows` 由模板匹配或沿用旧 usc 的值 |
| 一次性迁移（`V300098` 至 `V300102`） | 只处理"存量里此刻就存在的错"；本机制处理"以后还会再来的错"，两者不冲突 |

## 8. 执行顺序建议

| 步骤 | 内容 | 交付物 |
|------|------|--------|
| 1 | 建规则表与索引 | 迁移 `V300103` |
| 2 | 页面字段变更说明补进 `Docs/API` | 接口文档 |
| 3 | 后端：规则 CRUD + 试算接口 | 接口 |
| 4 | 后端：patch 执行器（分批 upsert + 水位）与 JOB 注册 | JOB |
| 5 | 前端：规则维护页 + 手动执行 + 日志 | 页面 |
| 6 | 初始规则：把附录 A 已确认的映射写进规则表并停用，等业务启用 | 数据 |

初始规则建议（先以停用状态入库，逐条启用）：

| `usc_old` | `usc_new` | 依据 |
|-----------|-----------|------|
| `SH000680` | `SH0680` | 附录 A.3.1 |
| `SH000688` | `SH0688` | 附录 A.3.1 |
| `SH000888` | `SH0888` | 附录 A.3.1 |
| `SH000002` | `SH0002` | 附录 A.3.1 |
| `SH000003` | `SH0003` | 附录 A.3.1 |
| `SH000004` | `SH0004` | 附录 A.3.1 |
| `SH000007` | `SH0007` | 附录 A.3.1 |
| `SH000008` | `SH0008` | 附录 A.3.1 |
| `SH000009` | `SH0009` | 附录 A.3.1 |
| `SH000010` | `SH0010` | 附录 A.3.1 |
| `SH000011` | `SH0011` | 附录 A.3.1 |
| `SH000012` | `SH0012` | 附录 A.3.1 |
| `SH000015` | `SH0015` | 附录 A.3.1 |
| `SH000016` | `SH0016` | 附录 A.3.1 |
| `SH000017` | `SH0017` | 附录 A.3.1 |
| `SH000039` | `SH0039` | 附录 A.3.1 |
| `SH000041` | `SH0041` | 附录 A.3.1 |
| `SH000043` | `SH0043` | 附录 A.3.1 |
| `SH000132` | `SH0132` | 附录 A.3.1 |
| `SH000159` | `SH0159` | 附录 A.3.1 |
| `SH000300` | `SH0300` | 附录 A.3.1 |
| `SH000903` | `SH0903` | 附录 A.3.1 |
| `SH000922` | `SH0922` | 附录 A.3.1 |
| `SH000980` | `SH0980` | 附录 A.3.1 |
| `SH000985` | `SH0985` | 附录 A.3.1 |
| `000680` | 不适用 | 该 usc 属深市个股，不改 |
| `000985` | 不适用 | 该 usc 属深市个股，不改 |

## 9. 待确认决策点

| 编号 | 决策点 | 选项 | 建议 |
|------|--------|------|------|
| P1 | 主键冲突时保留哪份数据 | ① 较新者胜（按 `gmt_update`） ② 已存在的目标行胜 ③ 字段逐列 `COALESCE` 合并 | ① 较新者胜（按 `gmt_update`）—— 已定 |
| P2 | 登记表 `finv_quote_cdeu_usc` 的旧 usc 行 | ① 保留不删 ② 停用标记 ③ 删除 | ① 保留不删 —— 已定 |
| P3 | 已导出的旧 usc 归档文件 | ① 保留，靠保留策略淘汰 ② 立即标记待重刷 ③ 立即删除 | ① 保留，靠保留策略淘汰 —— 已定 |
| P4 | 触发方式 | ① 定时 JOB（每天一次）加菜单手动 ② 仅菜单手动 ③ 定时加导入后钩子 | ① 定时加手动 —— 已定 |
| P5 | 规则匹配方式 | ① 首版只做精确匹配 ② 同时支持前缀 ③ 支持正则 | ① 首版只做精确匹配 —— 已定 |
| P6 | 默认调度间隔 | 每天一次（2026-11-02 已定），`Kind: PATCH_OUTDATED_USC`、`DailyAt: "05:00"` | 已定 |

## 10. 风险与边界

| 风险 | 处置 |
|------|------|
| 规则配错把 A 证券的数据搬到 B 证券 | 规则保存前跑试算接口，展示受影响 usc 与行数；启用前人工确认 |
| 两个旧 usc 映射到同一新 usc，合并后行数对不上 | upsert 逐 `ts` 去重，日志单独统计"合并丢弃行数" |
| 全量执行扫大表拖慢库 | 强制分批，单批 5000 行，批间可让出；默认只跑增量 |
| 规则成环（A 改 B、B 改 A） | 建表时禁止自指；执行前检测环，发现环拒绝执行 |
| 上游映射不改，patch 无限重复 | 同步更新 `futu.finv_futu_mapping_security`，并在 JOB 日志里统计"重复迁移行数"，持续偏高说明上游待修 |
| 删数据违规 | 本机制的 `DELETE` 只删已成功 upsert 到目标 usc 的旧行，且与插入在同一语句内；不满足 `DbSchemaSpec` §0 的任何禁止项（不删表、不改结构、不改 schema） |
