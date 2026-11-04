# Docs/Issue — Issue 文档目录

> 用途：存放**面向上游依赖或第三方系统的 Issue 正文**（可直接复制粘贴提交），
> 与 `Docs/Proposal/`（面向本项目决策的需求提案）区分开。

## 1. 目录约定

| 目录 | 是否入库 | 放什么 |
|------|----------|--------|
| `Docs/Issue/` | ✅ 跟踪 | 已提交/已外发的 Issue 正文，命名形如 `ISSUE-O-0001-MetaExtCustomHeaderSupport.md` |
| `Docs/Issue/Outside/` | ❌ 不跟踪（见 `.gitignore`） | 未外发、本地草稿或涉及内部环境信息的 Issue 草稿 |

## 2. 文件命名与编号

格式：**`ISSUE-O-NNNN-大驼峰标题.md`**

| 段 | 规则 |
|----|------|
| `ISSUE-O-` | **固定前缀**（全大写），所有 Issue 文档一律带（含 `Outside/` 草稿），不做变动 |
| `NNNN` | **本目录内**四位顺序号，从 `0001` 起，不足四位**左补 0**（`0001`、`0042`、`1234`）；只增不减、不得重号、**不得复用**（即使原文件被删除或改名） |
| `大驼峰标题` | 英文 PascalCase，主题 + 对象清晰可辨，如 `MetaExtCustomHeaderSupport` |

示例：

```
Docs/Issue/ISSUE-O-0001-MetaExtCustomHeaderSupport.md
Docs/Issue/ISSUE-O-0002-RedisClusterTimeoutBehavior.md
Docs/Issue/Outside/ISSUE-O-0003-LocalDraftIssue.md
```

规则要点：

- 编号在 `Docs/Issue/` 与其 `Outside/` 子目录**共用一套计数**：外发后从 `Outside/` 移到 `Docs/Issue/` 时**编号保持不变**，不重新编号。
- 新增前先核对已用编号（`Outside/` 也要算），以下命令应输出空：

  ```bash
  ls Docs/Issue Docs/Issue/Outside 2>/dev/null | grep -E '^ISSUE-O-[0-9]{4}-' | cut -d- -f3 | sort | uniq -d
  ```

- 下一个可用编号 = 现有最大编号 + 1；当前已用：`0001`。
- 不使用中文文件名；编号与 `Docs/Proposal/`（`yyyyMM-NNNN-`）、`Docs/Refactor/`（`yyMM-NNN-`）三套编号互相独立、互不干扰。

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
