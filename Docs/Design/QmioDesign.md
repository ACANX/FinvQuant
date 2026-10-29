# qmio 数据搬运工具设计

**文档版本**：v1.0.0
**状态**：生效
**生效日期**：2026-10-29
**最后更新**：2026-10-29
**负责人**：@ACANX
**适用版本**：FinvQuant 0.1.4

---

## 变更日志

| 版本 | 日期 | 变更内容 | 变更人 |
|---|---|---|---|
| v1.0.0 | 2026-10-29 | 初始版本：export / import / verify 三个子命令的输入、处理、输出与失败语义，MVSV-2 数据契约、安全与运维约束 | @ACANX |

---

## 0. 定位与读者

本文描述 **`qmio`（Quick Market IO）** 的设计与实现：一个不依赖主服务的命令行数据搬运工具，用于把 `cdeu.finv_quote_secu_kline_min` 整表导出成 MVSV-2 文件、再导入到另一个库，以及校验文件完整性。

读者：

- 需要给测试环境灌数、做联调或压测的运维/开发；
- 需要修改导入语义（冲突策略、批量、gmt 处理）的开发人员；
- 排查「导入后行数不对 / 时间戳变空 / 校验失败」的人员。

配套文档：

- 使用手册（参数、示例）：[Qmio.md](../FinvQuant/Qmio.md)
- 文件格式契约：[MvsvFileFormat.md](../DataFormat/MvsvFileFormat.md) §5「MVSV-2」
- 设计前身（导出/导入方案的取舍）：[2610-010-CdeuKlineDumpImport.md](../Refactor/2610-010-CdeuKlineDumpImport.md)
- 数据来源与归档链路：[CdeuWorkflowDesign.md](CdeuWorkflowDesign.md)

## 1. 目标与边界

| 目标 | 说明 |
|---|---|
| 整表可搬运 | 一次导出整张主表（或按 `--where` 过滤的子集），单文件即可移植 |
| 不打扰在线服务 | 导出走 `REPEATABLE READ` + `READ ONLY` 快照事务，不阻塞写入，导出期间数据一致 |
| 可重放 | 默认 `upsert`，重复导入同一文件结果不变（幂等） |
| 可校验 | 每个导出文件配一份 `.sha256`，导入/校验前自动比对 |
| 零依赖 | 单二进制，无需启动 `finvquant` 服务；连接参数可从 `finvquant.env` 自动发现 |

**不在范围内**：

- **不做增量同步**：没有水位、没有差值计算，每次都是「文件即数据集」；增量属于归档链路（`JOB_SCAN_PATCH`）的职责。
- **不做远程传输**：只落地到本地目录，上传远端是 CDEU 归档（`JOB_UPLOAD_FILES`）的职责，本工具不碰 uploader。
- **不做 MVSV-1 导出**：MVSV-1 是单证券行情交换格式，由行情导入/上传链路使用；本工具只**读** MVSV-2（`Format != "MVSV-2"` 直接报错）。
- 不写 `quote.` / `dedup.` / `export.` 三张同名表，只写 `cdeu.` 主表（`DbSchemaSpec` §0 的隔离约束）。

## 2. 术语

| 术语 | 含义 |
|---|---|
| MVSV-2 | 多证券单文件格式，数据区含 `usc` 列，列顺序由 `# Field` 声明（详见格式文档 §5） |
| 快照事务 | `IsoLevel=RepeatableRead` + `AccessMode=ReadOnly`，保证 `count(*)` 与数据读取来自同一时点 |
| `upsert` | 冲突键 `(usc, ts)`，冲突时用文件值覆盖业务列 |
| `append` | 冲突键已存在则跳过（`ON CONFLICT DO NOTHING`） |
| `replace` | 先 `TRUNCATE` 目标表再整表插入，破坏性操作 |
| `gmt_*` | `gmt_create` / `gmt_update` 两列；`--no-gmt` 时文件为 17 列，否则 19 列 |

## 3. 总体架构

```
qmio export / import / verify
   │
   ├─ main.go       子命令分发、退出码、连接池与 Ping（5 s 超时）
   ├─ envfile.go    未给 --pg-url 时按 4 级候选路径加载 finvquant.env
   ├─ util.go       sha256 计算、.sha256 文件读写（0640）
   ├─ export.go     快照事务 → 流式 SELECT → MVSV-2 写文件 → .sha256
   ├─ import.go     .sha256 校验 → gzip 识别 → MVSV-2 解析 → 分批 INSERT → 单事务提交
   └─ verify.go     .sha256 校验 → 解析计数 → 打印 Format / Count / Field
         │
         └─ internal/meta/finv/quote/mvsv（MVSV-2 读写实现，与归档共用）
```

### 3.1 连接参数解析顺序

```
--pg-url 显式给出
   └─ 否则 ensureEnvLoaded()：FINV_ENV_FILE → <exe 目录>/finvquant.env
        → <exe 目录>/../finvquant.env → /data/App/FinvQuant/finvquant.env
        （已存在的环境变量不覆盖）
        └─ 仍缺 → config.Load() 读取 FINV_PG_* 组装 DSN（sslmode=disable）
```

**为什么需要 envfile 自动加载**：systemd 会用 `EnvironmentFile` 注入配置，但运维手工执行 `qmio` 时没有这些变量——手工跑就报「缺数据库密码」是很差的体验，因此工具自己去找同目录的 `finvquant.env`，且**只填空、不覆盖**已设置的变量。

### 3.2 退出码约定

| 码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 执行失败（连接、解析、校验、写入） |
| 2 | 用法错误（无子命令、未知子命令、缺少 FILE、非法 `--mode`、`replace` 未带 `--force`） |

## 4. 子命令详解

### 4.1 export（导出）

| 项 | 内容 |
|---|---|
| 输入 | `--out DIR`（默认 `/data/App/FinvQuant/Export/CdeuKline`）、`--where SQL`、`--no-gmt`、`--no-gzip`、`--pg-url` |
| 处理 | ① 建池并 Ping → ② 开只读快照事务 → ③ `count(*)` → ④ 流式 `SELECT` 19 列 `ORDER BY usc, date, ts` → ⑤ `mvsv.WriteV2` 写文件 → ⑥ 写 `.sha256` |
| 输出 | `cdeu_kline_YYYYMMDD_HHMMSS.mvsv[.gz]` + 同名 `.sha256`；控制台打印「导出完成：路径（N 行）」 |
| 解决的问题 | 把整表变成可移植、可校验的单文件，供跨环境搬运 |

关键实现选择：

- **数值列一律 `::text`**：主表是 `numeric(20,6)` 等定点类型，转 float 会引入二进制误差，转 text 后按原始精度写入文件（与归档的 MVsv 输出口径一致）。
- **`count(*)` 与数据取自同一快照**：因此文件头 `# Count` 是准确值，导入侧不需要自己数行。
- **`DataScope` 记录过滤条件**（`all` 或 `where: ...`），让文件自解释「装的是哪部分数据」。
- **`Title=CdeuKline_<时间戳>`**：文件名与标题都带时间戳，同名文件不会互相覆盖。
- **目录权限 0750、文件 0640**：导出物是行情数据，默认可读范围收紧。

`--where` 的写法是**直接拼接**进 SQL 的（例：`usc = 'GCMain' AND date >= 20260101`）。这是运维工具的受信输入设计，换来「不用为每个过滤条件写参数解析」；代价是**只允许可信运维使用**，不得把它接进任何自动化管道接收外部输入（见 §6 风险项）。

### 4.2 import（导入）

| 项 | 内容 |
|---|---|
| 输入 | `FILE`、`--mode upsert\|append\|replace`、`--force`、`--batch`（默认 1000）、`--skip-verify`、`--pg-url` |
| 处理 | ① 参数校验 → ② `.sha256` 校验 → ③ gzip 魔数识别 → ④ 流式解析（首行确认 `Format=MVSV-2`，列名取自 `# Field`）→ ⑤ 攒批多值 `INSERT` → ⑥ 单事务提交 |
| 输出 | 控制台打印 `格式 / 文件行数 / 实际影响行数 / mode`；数据库写入 `cdeu.finv_quote_secu_kline_min` |
| 解决的问题 | 把文件可靠地灌进目标库，且重复执行安全（幂等） |

三种模式的 SQL 差异（同一张表，冲突键 `(usc, ts)`）：

| mode | 冲突处理 | 适用场景 |
|---|---|---|
| `upsert`（默认） | `ON CONFLICT (usc,ts) DO UPDATE SET 其余列 = EXCLUDED.*`；文件不含 `gmt_update` 时追加 `gmt_update = now()` | 重复导入、覆盖修正 |
| `append` | `ON CONFLICT (usc,ts) DO NOTHING` | 只补新数据、不动已有行 |
| `replace` | 先 `TRUNCATE` 再纯 `INSERT`（必须 `--force`） | 目标库整表重建（灌数/压测基线） |

关键实现选择：

- **单事务**：解析、分批插入、`TRUNCATE` 全在一个事务里，失败整体回滚——不会出现「导了一半」的半成品状态。
- **gzip 按魔数识别**（`1f 8b`）而非扩展名：文件被改名也能正确解压。
- **列名映射而非列序**：只要 `# Field` 里有该列名就写入，文件多列/少列都不会错位；`usc`、`ts` 缺失时解析阶段即失败。
- **`replace` 三重保护**：`--mode replace` 枚举限制 → 必须 `--force` → 执行前打印目标表名与当前行数。
- **`--batch` 默认 1000**：兼顾往返次数与单条 SQL 体积；小于等于 0 自动回落到 1000。

### 4.3 verify（校验）

| 项 | 内容 |
|---|---|
| 输入 | `FILE`、`--skip-checksum` |
| 处理 | ① `.sha256` 比对 → ② gzip 识别 → ③ 全文件解析，逐行计数 |
| 输出 | `Format` / `Count` / `Field`（列清单）以及「校验通过」 |
| 解决的问题 | 不连数据库也能判断「文件是否完整、格式是否合规」，用于传输后、导入前的门禁 |

**注意边界**：verify 校验的是**文件内部一致性**（哈希、可解析、计数与文件头一致），不校验业务约束（如 `ts` 是否秒级、`volume` 是否非负）——那属于导入后的数据质量核查。

## 5. 数据契约

### 5.1 MVSV-2 文件

| 项 | 规则 |
|---|---|
| `# Format` | 固定 `"MVSV-2"`，import 只接受该值 |
| 列数 | 19 列（含 `gmt_create`、`gmt_update`）或 17 列（`--no-gmt`） |
| 列顺序 | 由 `# Field` 声明；导入按列名映射，不依赖顺序 |
| 必填列 | `usc`、`ts` |
| 数据行 | 顺序为 `ORDER BY usc, date, ts`（导出侧保证，便于人工比对与稳定 diff） |
| 压缩 | gzip（扩展名 `.gz`）或不压缩，按**魔数**识别 |
| 校验文件 | `<文件名>.sha256`，内容为 `<sha256>␠␠<basename>`（与 `sha256sum` 兼容格式），权限 0640 |

### 5.2 列清单（19 列，`--no-gmt` 去掉最后两列）

```
usc, ts, date, time, open, high, low, close, last_close,
volume, turnover, change_price, change_ratio, paocd,
ext_field, remark, provider, gmt_create, gmt_update
```

### 5.3 目标表

- 只写 `cdeu.finv_quote_secu_kline_min`；`quote.` / `dedup.` / `export.` 三张同名表不碰（隔离设计，见 `DbSchemaSpec` §0）。
- 冲突键 `(usc, ts)`：一个证券的一个时间戳只有一行，这是 upsert/append 语义的基础。

## 6. 安全与运维

| 项 | 约束 |
|---|---|
| 数据访问 | 导出用只读快照事务；导入写主表，需相应账号权限 |
| 文件权限 | 目录 `0750`、数据文件 `0640`、校验文件 `0640` |
| 破坏性操作 | `replace` 需 `--mode replace` + `--force`，执行前打印目标表与行数 |
| ⚠ `--where` 直拼 SQL | 仅受信运维手工使用；不得接入接收外部输入的自动化流程 |
| ⚠ gmt 文本解析 | `timeValue` 支持 6 种时间布局；都解析失败时写 `NULL` 且不报错——若发现导入后 `gmt_create` 为空，先检查文件头 `FieldType` 与时间格式 |
| 大文件导入 | 单事务会长时间持有锁：建议低峰期执行，必要时调低 `--batch` 减小单条语句体积 |

### 6.1 常见问题速查

| 现象 | 原因 | 处置 |
|---|---|---|
| 报「仅支持 MVSV-2」 | 传入了 MVSV-1 文件 | 用归档链路产出的 MVSV-2，或改用行情导入接口 |
| 报「sha256 不一致」 | 文件被截断/修改，或与 `.sha256` 不配对 | 重新传输；确认 `.sha256` 与文件同名同目录 |
| 报「replace 会清空目标表，必须显式加 --force」 | 保护机制触发 | 确认目标库正确后加 `--force` |
| 导入影响 0 行 | `append` 模式下数据已存在 | 改用 `upsert` 或 `replace` |
| 导入后 `gmt_*` 为空 | 时间文本无法解析（见上） | 检查文件 `gmt_*` 字段格式 |
| 连接失败 | 未给 `--pg-url` 且未找到 `finvquant.env` | 显式传 `--pg-url`，或设置 `FINV_ENV_FILE` |

## 7. 实现现状与已知限制

| 项 | 状态 |
|---|---|
| `export`：整表/条件导出、gzip、`.sha256`、快照只读 | ✅ |
| `import`：upsert / append / replace、分批、单事务、gzip 魔数识别 | ✅ |
| `verify`：哈希校验、格式/行数/列清单 | ✅ |
| `finvquant.env` 自动发现（4 级候选路径，不覆盖已存在变量） | ✅ |
| 断点续传（中断后从上次位置继续） | ⛔ 未实现：中断即整事务回滚，重跑从头开始 |
| 并行导入 / 多文件批量 | ⛔ 未实现：一次一个文件，串行 |
| 直传对象存储（S3 等） | ⛔ 不做：远端传输属归档链路（BRS / CDEU 上传） |
| 压缩级别可配 | ⛔ 未实现：gzip 使用默认级别 |
| 导入后业务质量校验（`ts` 秒级、`volume` 非负等） | ⛔ 未实现：由数据库约束与归档链路负责 |

## 8. 相关文档

- [Qmio.md](../FinvQuant/Qmio.md) — 使用手册：安装、参数、示例流程
- [MvsvFileFormat.md](../DataFormat/MvsvFileFormat.md) — MVSV-1 / MVSV-2 格式契约（§5 为 MVSV-2）
- [2610-010-CdeuKlineDumpImport.md](../Refactor/2610-010-CdeuKlineDumpImport.md) — 方案取舍（为何选 Go CLI 而非 `pg_dump`）
- [CdeuWorkflowDesign.md](CdeuWorkflowDesign.md) — 数据来源：CDEU 归档链路
- [DbSchemaSpec.md](../DevSpec/DbSchemaSpec.md) — §0 三张同名表的隔离红线

## 9. 源码索引

| 文件 | 职责 |
|---|---|
| `cmd/qmio/main.go` | 子命令分发、用法文本、连接池与 Ping |
| `cmd/qmio/export.go` | 快照事务、列清单与 `::text` 输出、MVSV-2 写入、校验文件 |
| `cmd/qmio/import.go` | 参数校验、gzip 魔数识别、分批 INSERT、三种冲突模式、事务提交 |
| `cmd/qmio/verify.go` | 哈希校验 + 解析计数输出 |
| `cmd/qmio/envfile.go` | `finvquant.env` 候选路径查找与解析（不覆盖已有变量） |
| `cmd/qmio/util.go` | `sha256` 计算与 `.sha256` 文件读写 |
| `internal/meta/finv/quote/mvsv/writer_v2.go` | MVSV-2 写出：`V2Columns`、文件头、数据行格式化 |
| `internal/meta/finv/quote/mvsv/parser_v2.go` | MVSV-2 解析：`Format` 判定、按 `# Field` 列名映射 |
