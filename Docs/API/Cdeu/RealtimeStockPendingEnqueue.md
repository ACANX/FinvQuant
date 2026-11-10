# 待录入证券入队 / 更新键（RealtimeStockPendingEnqueue）

> 接口（同一资源三个动作）：
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Probe` — **探测**（只读）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue` — **确认入队**（插入）
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Update` — **更新键**（更新）
> 需求：11153（入队）、11154（两步走人工确认）、11155（探测 + 场景化按钮 + 每次写入前复验）、11156（键字段可手工改）
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)（操作列「入队」→「入队 / 更新键」对话框）

## 1. 用途

把待录入证券的**证券键**登记进本地键表 `futu.finv_futu_secu_key`，或修正已有键的非主键字段。
它**不写**主数据表（那是「录入」按钮的事），也不解析 `usc`。

### 1.1 流程（探测 → 人工确认 → 提交）

| 步 | 界面 | 接口 | 是否写库 |
|:--:|------|------|----------|
| ① | 打开对话框（自动探测一次） | `POST .../Enqueue/Probe` | 否 |
| ② | 人工核对 / 修改键字段，必要时点「探测」再确认一次 | `POST .../Enqueue/Probe` | 否 |
| ③ | 键不存在 → 点「确认入队」；键已存在 → 点「更新键」 | `POST .../Enqueue` 或 `POST .../Enqueue/Update` | 是 |

### 1.2 按钮状态规则（需求 11155）

| 探测结果 | 主按钮 | 说明 |
|----------|--------|------|
| 未探测 / 探测中 | 按钮**禁用** | 提示「先点『探测』确认该键是否存在」 |
| 键**不存在** | 「确认入队」可点击 | 插入一行 |
| 键**已存在** | 「更新键」可点击，并给「去『USC 映射规则』」按钮 | 更新非主键字段 |
| `market` / `code` 改动后 | 按钮**禁用** | 主键变了，上次探测失效；必须重新探测 |

服务端在写入前**再验证一次**存在性（入队前查在 + `ON CONFLICT DO NOTHING`；更新时 `WHERE ftmm_symbol` 命中才算成功），
因此即使前端状态过期也不会写错。

## 2. 可写字段（需求 11156）

| 字段 | 可改 | 规则 |
|------|:----:|------|
| `ftmm_symbol` | ❌ | 主键，= `upper(market \|\| '.' \|\| code)`，随 `market` / `code` 自动变化，只读展示 |
| `market` | ✅ | 非空；自动转大写 |
| `code` | ✅ | 非空；原样保留（不做大小写转换） |
| `stock_id` | ✅ | 正整数，默认快照证券编号 |
| `instrument` | ✅ | 必须在键表词表内：`stock` / `etf` / `index` / `sector` / `futures` / `crypto` / `forex` / `market` |
| `status` | ✅ | 必须在键表词表内：`PENDING` / `QUEUED` / `DONE` / `FAILED` / `SKIP`，默认 `PENDING` |
| `flag_enable` | ✅ | `0` / `1`，默认 `1` |
| `priority` | ✅ | `1..1000000`，默认 `10` |
| `usc` | ❌ | 只读返回；入队后由「USC 映射规则」解析 |

> **键表与采集表的 `instrument` 词表不同**：键表是 `sector`（单数），`futu.finv_futu_quote_collect` 是 `sectors`（复数）。
> 写混会让 USC 映射规则的 `instrument` 匹配不到。

## 3. 探测 `POST .../Enqueue/Probe`

### 3.1 请求

```json
{ "stock_id": 89184996156542, "market": "US", "code": "TXCP" }
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `stock_id` | 是 | 快照主键；用于取默认值 |
| `market` / `code` | 否 | 覆盖快照派生值（人工改过的键）；省略则用快照值 |

### 3.2 响应

```json
{
  "code": 0,
  "message": "探测完成",
  "data": {
    "stock_id": 89184996156542,
    "planned": {
      "ftmm_symbol": "US.TXCP",
      "market": "US",
      "code": "TXCP",
      "stock_id": 89184996156542,
      "instrument": "stock",
      "status": "PENDING",
      "flag_enable": 1,
      "priority": 10,
      "usc": ""
    },
    "existed": true,
    "current": {
      "ftmm_symbol": "US.TXCP",
      "market": "US",
      "code": "TXCP",
      "stock_id": 0,
      "instrument": "stock",
      "status": "PENDING",
      "flag_enable": 1,
      "priority": 26,
      "usc": ""
    },
    "hint": "键表已有 US.TXCP（status=PENDING，usc=—），只能用「更新键」改它，不能重复入队"
  }
}
```

| 字段 | 说明 |
|------|------|
| `planned` | 本次将要写入的字段（默认值 + 人工覆盖后的键） |
| `existed` | 键是否已存在（等价于 `current != null`） |
| `current` | 已存在时的现有行（一个字段一列，不再合并展示） |
| `hint` | 直接展示给用户的说明 |

## 4. 确认入队 `POST .../Enqueue`

请求：

```json
{
  "fields": {
    "market": "US", "code": "TXCP", "stock_id": 89184996156542,
    "instrument": "stock", "status": "PENDING", "flag_enable": 1, "priority": 10
  }
}
```

`fields.ftmm_symbol` 可传但会被服务端按 `market` + `code` 重建（不采信）。

响应：

```json
{
  "code": 0,
  "message": "入队完成",
  "data": {
    "ftmm_symbol": "US.TXCP",
    "action": "insert",
    "affected": 1,
    "existed": false,
    "hint": "已入队 US.TXCP（instrument=stock，status=PENDING）；到「USC 映射规则」跑一次计算后，本页可用「手动补录USC」完成同步"
  }
}
```

键已存在时不插入：`message="键表已存在该记录，未重复入队"`、`existed=true`、`affected=0`。

## 5. 更新键 `POST .../Enqueue/Update`

请求体同上（`fields`）。只更新非主键字段：

```sql
UPDATE futu.finv_futu_secu_key
   SET stock_id = $2, instrument = $3, status = $4, flag_enable = $5, priority = $6
 WHERE ftmm_symbol = $1
```

- 主键不可改；要改 `market` / `code` 等于换一条键，请走「确认入队」；
- 记录不存在时 `affected=0` + `hint` 提示改用「确认入队」，**不静默插入**；
- 响应 `action="update"`。

## 6. 幂等与并发

- 入队：先查在，再 `ON CONFLICT (ftmm_symbol) DO NOTHING RETURNING`；并发抢先写入时归为「已存在」而非报错；
- 更新：`UPDATE ... WHERE ftmm_symbol = ...` 天然幂等（重复提交结果一致）；
- 重复点击不会产生第二条键记录。

## 7. 错误码

| HTTP | code | 场景 |
|------|------|------|
| 200 | 0 | 探测成功 / 入队成功或已存在 / 更新成功或记录不存在（`affected=0`） |
| 400 | 400 | `stock_id` 非法或不存在 / `market`·`code` 为空 / `instrument`·`status` 不在词表 / `flag_enable` 非 0·1 / `priority` 越界 |
| 500 | 2006 | 数据库侧失败 |

## 8. 入队之后

1. 键表出现该 `ftmm_symbol` 后，去「USC 映射规则」跑「预演 → 应用」，规则引擎解析出 `usc`；
2. 回本页点「手动补录USC」，`usc` 落到快照表，该行离开待录入清单；
3. 规则也解不出（`ESCALATE`）时补规则条目后再来一遍。
