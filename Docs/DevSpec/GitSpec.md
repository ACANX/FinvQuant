# Git 提交规范（GitSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：全部 Git 提交信息（commit message）。

## 1. 提交信息格式

- 使用 Conventional Commit 风格前缀，标题格式为 `<type>: 简体中文摘要`。
- 允许的首选 type：`fix`、`feat`、`docs`、`ci`、`test`、`refactor`、`build`、`chore`。

## 2. 语言

- 标题和正文默认使用简体中文；仅当外部协议、固定术语或第三方要求必须使用英文时例外。

## 3. 与变更日志的联动（强制）

- 涉及功能、修复、接口、数据库、配置或部署变更的提交，**必须同时包含 `Docs/CHANGELOG.md`（全大写文件名）的更新**，记录规则见 [`DocSpec.md`](DocSpec.md) 第 4 节。
- 变更日志随功能变更进入同一次提交，不单独拆分为「补写日志」提交；确需拆分时须在提交正文中说明原因。
- 纯文档勘误、格式整理等无可感知影响的提交无需更新变更日志。

## 4. 版本号与构建标识

- 版本号的**单一来源**是项目根 `VERSION` 文件（同时注入后端 `-ldflags`、前端 `package.json`、镜像与二进制包名），不在代码或 CI 中重复定义。
- 正式发布：`VERSION`（如 `0.1.1`）+ git tag `v{VERSION}`（如 `v0.1.1`），镜像 tag 同为该值。
- 分支日常构建（`dev`/`main`/`FinvQuant`）：镜像 tag 用 `v{VERSION}-YYYYMMDDHHMM`（Asia/Shanghai），**不建 git tag、不发 release**。
- **集成分支快照**（`feat/integration`）：标识格式为 `v{VERSION}.{PPPPP}.{yymmddHHmm}`：
  | 段 | 含义 | 规则 |
  |---|---|---|
  | `VERSION` | 基础版本，取自 `VERSION` 文件 | 如 `0.1.1` |
  | `PPPPP` | 关联 PR 编号 | **5 位左补零**；无 PR（纯 push）时 `00000` |
  | `yymmddHHmm` | 构建时间戳（Asia/Shanghai） | 两位年，如 `2610241530` |
  - 该标识同时用作：git tag、GitHub prerelease 名、二进制包文件名、包内 `VERSION`、二进制 `main.version`。
  - **同一快照的所有产物必须共用同一个时间戳**：时间戳由 CI 首个 job（`snapshot-meta`）只算一次，通过 job outputs 传给所有产物 job，禁止各 job 自行 `date`。
  - 快照包产物：Ubuntu 26.04（`tar.gz`，`amd64` + `arm64`）、Windows 11（`zip`，`amd64`）。
  - 快照 release 一律标记 prerelease，不覆盖、不复用已有版本号。
- 命名示例：`finvquant_v0.1.1.00123.2610241530_ubuntu2604_arm64.tar.gz`。
