# 数据库修改检查清单（DbChangeCommitPreCheckListSpec）

> 所属：FinvQuant 交付检查清单 · 存放：`Docs/CheckList/`
> 用法：本清单**只列本类特有项**。通用项（范围授权、编译测试、提交纪律、红线）不重复，
> 按 [`CommitPreCheckListSpec.md`](CommitPreCheckListSpec.md) §4 路由取；合并侧复核见 [`MergeCommitPreCheckListSpec.md`](MergeCommitPreCheckListSpec.md)。
> 适用：新增迁移、改表结构、批量改数据、清洗/删除数据。
> 关联：[`DbSchemaSpec.md`](../DevSpec/DbSchemaSpec.md) **§0 红线**（表存在性与归属不得擅自变更）、[`DataDictMapping/`](../DataDictMapping/)、[`FileNamingSpec.md`](../DevSpec/FileNamingSpec.md)。

## 1. 授权（先过这一条，再谈其它）

| 编号 | 检查项 | 判据 |
|------|--------|------|
| DB1 | 删表/改名/换 schema/合并同名表有明确授权 | 用户在本轮**点名**的原话或 `Docs/RequirementList.md` 条目；否则一律不做 |
| DB2 | 迁移文件只新增 | `git diff --stat Deploy/Migrations` 只有 `A` 项，无 `M`/`D` |
| DB3 | 文件命名与序号 | `V300xxx__小写蛇形描述.sql`，序号连续不跳号；一脚本一件事 |

## 2. 脚本写法

| 编号 | 检查项 | 判据 |
|------|--------|------|
| DB4 | 单事务 | 脚本内 `BEGIN`/`COMMIT` 包裹全部语句 |
| DB5 | 幂等 | `IF EXISTS`/`IF NOT EXISTS`/`ON CONFLICT DO NOTHING`；复跑两次结果一致 |
| DB6 | 种子不覆盖用户值 | 一律 `ON CONFLICT DO NOTHING`，禁用 `DO UPDATE` 写种子 |
| DB7 | 删除/清洗有源表非空守卫 | 语句含 `AND EXISTS (SELECT 1 FROM 源表 LIMIT 1)`，避免空库误删种子 |
| DB8 | 数据修复按业务键直接落值 | 按 `ftmm_symbol`/主键把目标列写成目标口径，不只改标记列再断言 |
| DB9 | 收敛校验只断结构性事实 | 只断言"计划删除集是否清空""重复 0 组""与键表差异 0"；行数类计数一律 `RAISE NOTICE` |
| DB10 | 无物理外键 | 迁移后 `fk = 0`（跨域引用只走程序层） |
| DB11 | 不写生成列 | 不向 `usc_lower`、`ftmm_symbol` 等插入/更新 |
| DB12 | 命名与 schema 正确 | 时间列 `dt_` 前缀、字段动词原形（禁 `-ed`）；表名连 schema 核对（`futu.` ≠ `cdeu.`） |
| DB13 | 大小写不敏感唯一用生成列 + 部分唯一索引 | `UNIQUE (col_lower) WHERE status='ACTIVE'`，不靠 `lower()` 表达式索引散落各处 |

## 3. 验证（缺一不可）

```bash
# 空库全量迁移（最关键：一次性通过，且种子仍在）
go test ./internal/... -run TestMigrateIntegration -count=1
# 主库/克隆库补迁移 + 版本号前进 + 幂等复跑
psql -c "SELECT max(version) FROM public.schema_version;"
psql -c "SELECT count(*) FROM pg_constraint WHERE contype='f';"   # 必须为 0
```

| 编号 | 检查项 | 判据 |
|------|--------|------|
| DB14 | 空库全量迁移通过 | 迁移链不中止；种子行数完整 |
| DB15 | 真实库补迁移通过 | `schema_version` 等于新迁移号；服务不再重启循环 |
| DB16 | 复跑幂等 | 同一脚本第二次执行无报错、无重复数据 |
| DB17 | 大表操作可控 | 分批/带条件，避免长事务锁表；预估耗时写进迁移注释 |
| DB18 | 改前有备份 | 归档表或 `Deploy/Backup/` 产物，能定位到具体行 |
| DB19 | 回滚路径写明 | 迁移注释或 PR 里写清怎么退（反向 SQL / 恢复备份 / 重跑旧版） |
| DB20 | 文档同步 | `DbSchemaSpec`、`DataDictMapping/` 中受影响的表/字段同步更新 |

## 4. 本类高频失手（逐条都是真事故）

| 事故 | 对应条目 |
|------|----------|
| 删除类清洗在全新空库误删两万多行种子 | DB7 |
| 只改标记列就断言目标列，远端仍是旧值 → 启动失败、服务重启循环 | DB8 |
| 收敛校验用绝对行数阈值，把"刻意保留"判成未清理 → 迁移中止 | DB9 |
| 迁移残留物理外键，与仓库约定冲突 | DB10 |
| 显式写生成列报错 | DB11 |
| WSL 下 `psql -f D:\...` 找不到文件；SQL 注释含反引号破坏 Go raw string | DB14（用 `docker exec -i psql < file`） |

## 5. 条目新增规则

同 [`CommitPreCheckListSpec.md`](CommitPreCheckListSpec.md) §8。新增事故先记 `R` 编号，再回填本表。
