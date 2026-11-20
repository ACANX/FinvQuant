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
- **三种发布包**（CI 中 `snapshot-binaries` / `pr-binaries` / `release-binaries` 三个同级平行 job）：

  | 包类型 | 触发场景 | 版本 / git tag | 发布方式 |
  |---|---|---|---|
  | 快照包 `snapshot` | push 代码到分支（非 `main`，且提交未关联已合并 PR） | `v{VERSION}.{PPPPP}.{yymmddHHmm}` | prerelease；时间戳保证同一 PR 多次 push 唯一 |
  | 构建包 `pr` | PR 合并（push 的提交关联到已合并 PR） | `v{VERSION}.{PPPPP}` | prerelease；**B 策略**：同名 release/tag 已存在时删除后重建 |
  | 正式版包 `release` | 代码合并到 `main`（push 到 `main`） | `v{VERSION}` | 正式 release（`--latest`）；tag 已存在则跳过，需先 bump `VERSION` |

  段含义：
  | 段 | 含义 | 规则 |
  |---|---|---|
  | `VERSION` | 基础版本，取自 `VERSION` 文件 | 如 `0.1.3` |
  | `PPPPP` | 关联 PR 编号 | **5 位左补零**；无 PR（纯 push）时 `00000` |
  | `yymmddHHmm` | 构建时间戳（Asia/Shanghai） | 两位年，仅快照包使用 |

  - 版本标识同时用作：git tag、GitHub release 名、二进制包文件名、包内 `VERSION`、二进制 `main.version`。
  - **同一快照的所有产物必须共用同一个时间戳**：由 `snapshot-meta` 只算一次，通过 job outputs 传给 `snapshot-binaries`，禁止各 job 自行 `date`。
  - 包产物：Ubuntu 26.04（`tar.gz`，`amd64` + `arm64`）、Windows 11（`zip`，`amd64`）；包内 `BUILD-INFO.txt` 标注包类型。
  - 三种包场景互斥：提交已关联已合并 PR 时只出构建包（`snapshot-meta` 判定后输出空版本并跳过），同一次合并不会同时产出快照包与构建包。
- 产物命名：`FinvQuant-{包版本}-{ubuntu|windows}-{amd64|arm64}.{tar.gz|zip}`，产物目录 `Dist/`。
  - 命名示例：`FinvQuant-v0.1.3.00357-ubuntu-amd64.tar.gz`（构建包）、`FinvQuant-v0.1.3.00357.2610250004-ubuntu-arm64.tar.gz`（快照包）、`FinvQuant-v0.1.3-windows-amd64.zip`（正式版包）。
  - release 标题：`{包版本}-FinvQuant`（如 `v0.1.3.00357-FinvQuant`），git tag 仍为 `{包版本}`。
