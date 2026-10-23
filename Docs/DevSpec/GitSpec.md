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
