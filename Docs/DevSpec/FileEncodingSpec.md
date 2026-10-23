# 文件编码规范（FileEncodingSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：全部文本文件的编码约定（UTF-8 与脚本编码）。

## 1. 文本编码

- 所有新建或修改的文本文件优先使用 **UTF-8** 编码；不得因本地编辑器或终端默认代码页而将中文内容写成系统区域编码。

## 2. Windows 批处理（.bat / .cmd）

- 对包含中文的 `.bat` 或 `.cmd` 文件：使用 UTF-8 编码，**首个有效命令必须为 ASCII 的 `@chcp 65001 >nul`**，且其前不得出现中文输出、中文注释或其他非 ASCII 内容；避免依赖调用终端的默认代码页。

## 3. Bash 脚本（.sh）

- 对包含中文的 `.sh` 或 Bash 文件：保持 **ASCII shebang**，并使用 UTF-8 编码；涉及中文输入、输出或文件名的命令须在目标环境验证 locale/终端编码，不能假定 CI 或远程主机已正确配置。

## 4. 换行符（跨平台一致，强制）

- 仓库内所有文本文件**统一以 LF 存储、以 LF 检出**，Windows / Linux / macOS 三端工作区保持一致；不得依赖 `core.autocrlf` 的自动转换去「消化」行尾差异。
- 该策略由仓库根目录 [`.gitattributes`](../../.gitattributes) 固化，主要条款：
  - `* text=auto eol=lf`：所有文本默认 LF；
  - `*.cmd`、`*.bat`、`*.ps1`：`eol=crlf`（cmd.exe 对 LF 的 `goto` / 标签解析不稳定）；
  - `*.sh`：`eol=lf`（WSL / Linux 容器内 shebang 需要）；
  - 二进制资源（图片、压缩包、字体等）：`binary`，禁止换行转换。
- 本地 Git 配置建议：`git config core.autocrlf false`、`git config core.eol lf`；`.gitattributes` 的属性**优先于**该配置，即使他人误设 `core.autocrlf=true` 也不会产生 CRLF 提交。
- 提交前若出现整库「假修改」（大量文件仅行尾不同），不要逐个改回，按以下步骤统一修正：
  1. `git add --renormalize .`（按属性重算索引）；
  2. 删除工作区待刷新文件后 `git checkout -- .`，使工作区按 LF 重新检出；
  3. 用 `git ls-files --eol` 确认除 `*.cmd` / `*.bat` / `*.ps1` 外无 `w/crlf`。
