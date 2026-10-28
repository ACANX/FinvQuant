# 一次性数据迁移：`public.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`

> 所属：FinvQuant 部署文档 · 脚本：`Deploy/Script/MigratePublicKlineToCdeu.sql`
> 性质：**一次性任务**，不进入 `Deploy/Migrations/` 迁移链；上线验证通过后可注释/删除。

## 1. 背景

目标服务器上的 `finv` 库为多服务共用库，其中 `public.finv_quote_secu_kline_min`
是 FinvQuant 早期版本建在 `public` 下的历史分钟 K 线表。新版本运行时只读写
`cdeu.finv_quote_secu_kline_min`（代码常量见 `internal/meta/finv/quote/dedup/store.go:19`），
因此需要把这段历史数据一次性搬入 `cdeu`。

脚本对源表**只读**（仅 `SELECT`），不改名、不删除、不 `TRUNCATE`，源表保持原样。

## 2. 表结构与列映射

| 旧表（`public`） | 新表（`cdeu`） | 处理方式 |
|---|---|---|
| `secu_code` | `usc` | 改名映射 |
| `prev_close` | `last_close` | 改名映射 |
| `market_code` | —（无） | **丢弃**（新表以 `usc` 绑定市场） |
| `ts`, `date`, `time`, `open`, `high`, `low`, `close`, `paocd`, `volume`, `turnover`, `ext_field`, `remark`, `gmt_create`, `gmt_update` | 同名 | 原样复制（保留原 `gmt_*`） |
| — | `change_price`, `change_ratio`, `provider` | 源表没有，不复制（保留目标默认值 `NULL`） |

主键差异：

- 旧：`PRIMARY KEY (ts, market_code, secu_code)`
- 新：`PRIMARY KEY (ts, usc)`

因此同一 `(ts, usc)` 在旧表可能有多个 `market_code` 行，脚本按 `gmt_update`
保留最新一条（`DISTINCT ON`），并输出 `WARNING` 提示重复组数。

> 若服务器上的 `public` 表已被改造过（已有 `usc` / `last_close` 列），脚本自动识别
> 并直接同名复制，无需改脚本。

## 3. 前置条件

1. **已完成数据库迁移**：`cdeu.finv_quote_secu_kline_min` 已存在（迁移 `V100001–V100080` 跑完）：

   ```bash
   sudo -u postgres psql -d finv -tAc "select to_regclass('cdeu.finv_quote_secu_kline_min')"
   ```

2. **已备份数据库**（脚本只写 `cdeu`，但大数据量导入仍建议先备份）：

   ```bash
   pg_dump -Fc -d finv -f /data/App/FinvQuant/Backup/finv_before_kline_migrate_$(date +%F).dump
   ```

3. 建议**低峰期**执行；源表较大时会持有较长时间的读锁（不阻塞查询，但会阻塞 DDL）。

## 4. 执行

```bash
# 脚本随安装包位于 <PREFIX>/Deploy/Script/
psql -h 127.0.0.1 -p 5432 -U finv -d finv \
     -v ON_ERROR_STOP=1 \
     -f /data/App/FinvQuant/Deploy/Script/MigratePublicKlineToCdeu.sql
```

典型输出：

```
NOTICE:  列映射：usc,ts,date,"time",open,high,low,close,paocd,volume,turnover,ext_field,last_close,remark,gmt_create,gmt_update
NOTICE:  源列取值：s.secu_code,s.ts,s.date,s."time",...
NOTICE:  迁移前：源表 2812345 行，目标表 0 行；源表重复 (ts, usc) 组数 0
NOTICE:  迁移后：目标表 2812345 行（新增 2812345 行，跳过已存在 0 行）
NOTICE:  校验：源表仍未在目标表落地的行数 = 0
NOTICE:  迁移完成（幂等：重复执行不会重复插入）
```

要点：

- **幂等**：重复执行时全部命中 `ON CONFLICT (ts, usc) DO NOTHING`，只输出「新增 0 行」；
- **目标已有数据不会丢**：只插入缺失的 `(ts, usc)`，不覆盖 `cdeu` 中运行期写入的数据；
- **任一步异常整体回滚**（单事务），包括最后的两道计数校验。

## 5. 验证

```bash
sudo -u postgres psql -d finv <<'SQL'
-- 1) 目标表行数
select count(*) as cdeu_rows from cdeu.finv_quote_secu_kline_min;

-- 2) 源表行数不变（只读验证）
select count(*) as public_rows from public.finv_quote_secu_kline_min;

-- 3) 源表数据是否全部落地（应为 0）
select count(*) as missing
  from public.finv_quote_secu_kline_min s
 where not exists (select 1 from cdeu.finv_quote_secu_kline_min d
                    where d.ts = s.ts and d.usc = s.secu_code);   -- 源表已是 usc 时改成 s.usc

-- 4) 抽样核对映射（secu_code → usc、prev_close → last_close）
select d.usc, d.ts, d.last_close, s.prev_close, d.close, s.close
  from cdeu.finv_quote_secu_kline_min d
  join public.finv_quote_secu_kline_min s
    on s.ts = d.ts and s.secu_code = d.usc
 order by d.ts desc
 limit 5;
SQL
```

## 6. 回滚

脚本只向 `cdeu` 追加数据，回滚 = 删除本次导入的行（按源表键匹配）：

```sql
-- 源表为旧结构（secu_code）
delete from cdeu.finv_quote_secu_kline_min d
 where exists (select 1 from public.finv_quote_secu_kline_min s
                where s.ts = d.ts and s.secu_code = d.usc);

-- 源表已是新结构（usc）
delete from cdeu.finv_quote_secu_kline_min d
 where exists (select 1 from public.finv_quote_secu_kline_min s
                where s.ts = d.ts and s.usc = d.usc);
```

> 若 `cdeu` 中已有运行期导入的数据，回滚会把「与源表同键」的行一并删掉；
> 执行前先确认影响行数（把 `delete` 换成 `select count(*)` 预览）。

## 7. 注意事项

| 项 | 说明 |
|---|---|
| 只读源表 | 全程不对 `public.finv_quote_secu_kline_min` 执行 `UPDATE/DELETE/TRUNCATE/ALTER` |
| 新列默认值 | `change_price` / `change_ratio` / `provider` 为 `NULL`；如需回填，另写一次性脚本（可基于 `close - last_close` 计算 `change_price`） |
| `market_code` 丢弃 | 新表不再按市场分键；如业务需要市场信息，可通过 `cdeu.finv_quote_cdeu_usc` 关联获取 |
| 大表耗时 | 单事务、单条 `INSERT ... SELECT`；280 万行约数十秒。若超过 1000 万行，建议拆分为按年批处理（`PROCEDURE` + 分批 `COMMIT`） |
| 锁 | `SET LOCAL lock_timeout = '15s'`：有长事务占用源表时快速失败，避免长时间等待 |
| 幂等 | 可安全重复执行；重复执行只做一次全表 `NOT EXISTS` 校验 |
| 作废 | 验证通过后可将本脚本注释/删除；它**不在** `Deploy/Migrations/` 中，不影响版本化升级 |
