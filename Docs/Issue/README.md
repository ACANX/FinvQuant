# Docs/Issue — Issue 文档目录

> 用途：存放**面向上游依赖或第三方系统的 Issue 正文**（可直接复制粘贴提交），
> 与 `Docs/Proposal/`（面向本项目决策的需求提案）区分开。

## 1. 目录约定

| 目录 | 是否入库 | 放什么 |
|------|----------|--------|
| `Docs/Issue/` | ✅ 跟踪 | 已提交/已外发的 Issue 正文，命名用描述性大驼峰，如 `MetaExtCustomHeaderSupport.md` |
| `Docs/Issue/Outside/` | ❌ 不跟踪（见 `.gitignore`） | 未外发、本地草稿或涉及内部环境信息的 Issue 草稿 |

## 2. 文件命名

- 使用**描述性大驼峰**（PascalCase）英文名，主题 + 对象清晰可辨：
  `MetaExtCustomHeaderSupport.md`、`RedisClusterTimeoutBehavior.md`。
- 不使用中文文件名，不使用日期编号前缀（编号规则只适用于 `Docs/Proposal/` 与 `Docs/Refactor/`）。

## 3. 文档必备内容

每份 Issue 至少包含：

| 小节 | 必填内容 |
|------|----------|
| 状态栏 | 文首引用块：状态（待提交/已提交/已修复/已关闭）、目标仓库与版本、关联需求编号 |
| 问题陈述 | 用可核对的事实描述现象，能贴数据/日志就贴 |
| 现状核对 | 上游源码位置（文件 + 行号）、当前行为、为什么调用方无法自行规避 |
| 期望方案 | 首选方案与备选方案，含 API 签名、参考实现片段 |
| 验收标准 | 每条都可执行核对（命令或测试用例） |
| 待确认决策点 | 编号列出，每条给选项与建议 |
| 落地回填 | Issue 关闭后回填上游版本号与 FinvQuant 侧的替换提交 |

## 4. 与其他文档的关系

| 文档 | 关系 |
|------|------|
| `Docs/Proposal/` | 本项目内部决策用；面向上游的诉求放本目录 |
| `Docs/CHANGELOG.md` | Issue 外发/关闭时按 `DocSpec.md` §4 记一条；**未入库的 `Outside/` 草稿不记** |
| `Docs/RequirementList.md` | 上游采纳并在本仓库落地后，才登记正式需求编号 |
