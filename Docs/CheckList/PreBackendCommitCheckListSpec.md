# 后端改动检查清单（PreBackendCommitCheckListSpec）

> 所属：FinvQuant 交付检查清单 · 存放：`Docs/CheckList/`
> 用法：本清单**只列本类特有项**。通用项（范围授权、编译测试、提交纪律、红线）不重复，
> 按 [`PreCommitCheckListSpec.md`](PreCommitCheckListSpec.md) §4 路由取；合并侧复核见 [`PreMergeCommitCheckListSpec.md`](PreMergeCommitCheckListSpec.md)。
> 适用：改动 `cmd/`、`internal/`（Go 服务端、定时任务、外部客户端）。
> 关联：[`ApiSpec.md`](../DevSpec/ApiSpec.md)（响应信封/异常映射）、[`ErrorCodeSpec.md`](../DevSpec/ErrorCodeSpec.md)（错误码号段）、[`LogSpec.md`](../DevSpec/LogSpec.md)（日志与可观测性）。

## 1. 命令

```bash
go build ./... && go vet ./...
go test ./internal/<改动包>/... -count=1
go test ./internal/docs/ -count=1        # 改了接口/文档时
```

## 2. 清单

| 编号 | 检查项 | 判据 |
|------|--------|------|
| BE1 | 编译与 `go vet` 通过 | 退出码 0，无被 `//nolint` 掩盖的新告警 |
| BE2 | 新逻辑有测试或书面豁免 | 有 `_test.go` 用例；无 `t.Skip` 新增 |
| BE3 | 错误映射合规 | 对外走 `ApiSpec` 信封 + `ErrorCodeSpec` 号段；`SQLSTATE`、内部堆栈不外泄 |
| BE4 | 外部调用带超时与上下文 | HTTP/DB 调用传 `ctx` 与超时；不裸 `http.Get` |
| BE5 | URL 参数编码与分片 | 拼进 URL 的每个值都做百分号编码；大列表除个数上限外按字符预算分片 |
| BE6 | `ON CONFLICT` 目标列在 INSERT 列表里 | 否则命中不了、必撞自然键（`SQLSTATE 23505`） |
| BE7 | 不写生成列 | INSERT/UPDATE 列表不含 `usc_lower`、`ftmm_symbol` 等生成列 |
| BE8 | 幂等 | 手动触发、定时触发、重试走同一链路，重复执行结果一致 |
| BE9 | 资源释放 | `rows`/`resp.Body`/文件句柄在 `defer` 中关闭 |
| BE10 | 并发安全 | 共享 map/缓存有锁或并发安全结构；不并发写同一 map |
| BE11 | 配置不硬编码 | 连接串、表名、cron 走环境变量或 `sys_config` |
| BE12 | 日志合规 | 级别与字段符合 `LogSpec`；不打印密钥、令牌、全量请求体 |
| BE13 | 接口文档同步 | 改了接口就同步 `Docs/API/*.md` + `*.yml`，`go test ./internal/docs/` 通过 |
| BE14 | 结构变更联动 | 涉及表结构/数据口径的，走 [`PreDbChangeCommitCheckListSpec.md`](PreDbChangeCommitCheckListSpec.md) |
| BE15 | 大表查询有索引支撑 | 新增查询给出 `EXPLAIN`；不新增无索引全表扫描 |
| BE16 | 影响面自查 | 定时任务/常驻循环改动要说明首次执行时间与错峰，避免启动即打满 |

## 3. 本类高频失手

| 事故 | 对应条目 |
|------|----------|
| 外部 REST 过滤值未编码，`&`/`,` 被截断 → HTTP 400（前面批次正常，后面才炸） | BE5 |
| `ON CONFLICT (主键)` 但 INSERT 不含主键 → 编辑必撞唯一键 | BE6 |
| 错误直接把 `SQLSTATE` 透传前端 | BE3 |

## 4. 条目新增规则

同 [`PreCommitCheckListSpec.md`](PreCommitCheckListSpec.md) §8。
