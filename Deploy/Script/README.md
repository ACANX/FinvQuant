# Script 目录说明（快照包构建 + 一键下载）

> 所属：FinvQuant 部署 · 存放：`Deploy/Script/`
> 上级目录总览：[`../README.md`](../README.md)
> 用途：① 从源码构建**快照二进制包**（Ubuntu 26.04 / Windows 11），供 `feat/integration` 集成分支验证与本地联调使用；
> ② 不构建、直接从 GitHub Releases **一键下载**匹配本机 OS/架构的最新程序（含快照版）并解压（见 §13）。

## 1. 文件与职责

| 文件 | 职责 |
|---|---|
| `PackSnapshot.sh` | **打包逻辑的唯一实现**。CI 的 `snapshot-binaries` job 与本地出包都调用它 |
| `PackSnapshot.cmd` | Windows 薄封装：只负责找到 Git Bash 的 `bash.exe` 并转调 `PackSnapshot.sh`，**不含任何打包逻辑** |
| `Download.sh` | **一键下载逻辑的唯一实现**：检测 OS/架构 → 选版本（正式版/快照）→ 下载 + SHA256 校验 → 安全解压到指定目录（见 §13） |
| `Download.cmd` | Windows 薄封装：定位 Git Bash 后转调 `Download.sh`，不含下载逻辑 |
| `MigratePublicKlineToCdeu.sql` | **一次性**数据迁移：`public.finv_quote_secu_kline_min` → `cdeu.finv_quote_secu_kline_min`（只读源表、幂等、列名映射）；说明见 [`../../Docs/FinvQuant/MigratePublicKlineToCdeu.md`](../../Docs/FinvQuant/MigratePublicKlineToCdeu.md) |

```
PackSnapshot.cmd ──(定位 bash)──▶ PackSnapshot.sh ──▶ Dist/ 三个包
                                        ▲
GitHub Actions (snapshot-binaries) ─────┘
```

**改打包规则（包内文件、命名、压缩方式）只改 `PackSnapshot.sh`**，`.cmd` 与 CI 自动跟随，避免两边漂移。
**下载/校验/解压规则只改 `Download.sh`**，`Download.cmd` 仅转调。

## 2. 快速开始

```bash
# Linux / WSL / macOS：一次出全部三个包
bash Deploy/Script/PackSnapshot.sh
```

```bat
REM Windows（cmd 或 PowerShell 均可）
Deploy\Script\PackSnapshot.cmd
```

产物：

```
Dist/
├── FinvQuant-v0.1.1.00000.2610241530-ubuntu-amd64.tar.gz   14M
├── FinvQuant-v0.1.1.00000.2610241530-ubuntu-amd64.tar.gz.sha256
├── FinvQuant-v0.1.1.00000.2610241530-ubuntu-arm64.tar.gz   12M
├── FinvQuant-v0.1.1.00000.2610241530-ubuntu-arm64.tar.gz.sha256
├── FinvQuant-v0.1.1.00000.2610241530-windows-amd64.zip           14M
└── FinvQuant-v0.1.1.00000.2610241530-windows-amd64.zip.sha256
```

## 3. 参数

`PackSnapshot.sh` 与 `PackSnapshot.cmd` 参数完全一致（`.cmd` 原样透传）。

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--version <v>` | 由 `--pr` + `--timestamp` + `VERSION` 现算 | 完整快照版本，如 `v0.1.1.00123.2610241530`。给定后 PR 段与时间戳从版本中解析，用于**复现 CI 产物** |
| `--target <t>` | `all` | `linux/amd64`、`linux/arm64`、`windows/amd64`、`all` |
| `--pr <n>` | `00000` | 关联 PR 编号，自动左补零到 5 位；非纯数字按 `00000` |
| `--timestamp <ts>` | 当前 Asia/Shanghai 时间 | 格式 `yymmddHHmm`（两位年） |
| `--base-version <v>` | `VERSION` 文件内容 | 基础版本，如 `0.1.1` |
| `--commit <sha>` | `git rev-parse --short HEAD` | 注入二进制 `main.commit` 与 `BUILD-INFO.txt` |
| `--out <dir>` | `<仓库根>/Dist` | 产物输出目录 |
| `--build-web` | 关 | 强制重新 `npm ci && npm run build` 前端（默认复用 `internal/webui/dist`） |
| `--keep-tmp` | 关 | 保留 `pkg/` 临时目录便于检查包内结构 |
| `-h`, `--help` | — | 打印用法 |

## 4. 版本标识

格式：`v{BASE_VERSION}.{PPPPP}.{yymmddHHmm}`

| 段 | 来源 | 规则 |
|---|---|---|
| `BASE_VERSION` | `VERSION` 文件 | 如 `0.1.1` |
| `PPPPP` | `--pr` 或 CI 从 PR 关联信息取得 | **5 位左补零**；无 PR（纯 push）为 `00000` |
| `yymmddHHmm` | `--timestamp` 或构建时刻 | **两位年** + 月日时分，Asia/Shanghai |

**同一快照的所有产物共用同一个时间戳**：

- CI 里由首个 job `snapshot-meta` 只算一次，经 job outputs 下发给三个平台 job；
- 本地一次 `--target all` 也只用一次解析结果，三个包内 `BUILD-INFO.txt` 的「构建时间戳」完全相同；
- 需要复现 CI 的某次产物，直接把那个版本号传给 `--version`：

```bash
bash Deploy/Script/PackSnapshot.sh --version v0.1.1.00123.2610241530
```

该标识同时用于：git tag、GitHub prerelease 名、包文件名、包内 `VERSION`、二进制 `main.version`（`/API/V1/Version` 可见）。

## 5. 包内结构

### 5.1 Linux 包（`tar.gz`）

```
FinvQuant-v0.1.1.00123.2610241530-ubuntu-arm64/
├── finvquant                 # 静态链接、stripped；amd64 或 aarch64（按包名）
├── Deploy/
│   ├── Migrations/           # 迁移脚本（与仓库结构一致，安装后即 FINV_MIGRATIONS_DIR）
│   ├── Native/               # 宿主直装全套：Install/Upgrade/Uninstall.sh、
│   │                         #   FinvQuant.service、FinvQuant.env.example、
│   │                         #   Nginx.conf.example、README/Install/Upgrade.md
│   └── README.md
├── VERSION                   # 0.1.1.00123.2610241530
├── BUILD-INFO.txt            # 版本/PR/时间戳/commit/平台/构建方式
└── SHA256SUMS                # 包内每个文件的 sha256
```

适用 Ubuntu 26.04（x86_64 / aarch64），部署方式见 `Deploy/Native/Install.md`。

### 5.2 Windows 包（`zip`）

```
FinvQuant-v0.1.1.00123.2610241530-windows-amd64/
├── finvquant.exe
├── Deploy/
│   ├── Migrations/           # 迁移脚本（与仓库结构一致）
│   ├── Docker/               # compose + .env.example + 三个 .cmd + 文档
│   │                         #   注意：不含 Dockerfile（其 build.context 指向仓库根，
│   │                         #   包内无源码；Windows 侧直接 docker compose 拉 GHCR 镜像）
│   └── README.md
├── VERSION
├── BUILD-INFO.txt
└── SHA256SUMS
```

适用 Windows 11 x64。

> **Migrations 位置**：两种包都保持 `Deploy/Migrations/`（与仓库结构一致），解压到
> `/data/App/FinvQuant` 后即 `FINV_MIGRATIONS_DIR=/data/App/FinvQuant/Deploy/Migrations`，
> `Install.sh` 无需再复制。包内文件数随迁移脚本增减变化，以 `SHA256SUMS` 为准。

### 5.3 `SHA256SUMS` 用法

```bash
# 校验包内文件完整性
tar -xzf FinvQuant-xxx-ubuntu-amd64.tar.gz
cd FinvQuant-xxx-ubuntu-amd64 && sha256sum -c SHA256SUMS

# 校验下载到的包本身
sha256sum -c FinvQuant-xxx-ubuntu-amd64.tar.gz.sha256
```

## 6. 环境要求

| 依赖 | 必需性 | 说明 |
|---|---|---|
| Go 1.25+ | 必需 | 脚本自动探测 `go` 或 `go.exe` |
| 前端产物 `internal/webui/Dist/index.html` | 必需 | 缺失时用 `--build-web` 重新构建（需要 Node 24 + npm） |
| bash | 必需 | Windows 侧由 `.cmd` 负责定位（Git for Windows / MSYS2 / Cygwin） |
| `zip` 或替代 | 仅 Windows 包 | 按可用性回退，见 §7 |
| `tar`、`sha256sum`、`find`、`sort` | 必需 | Linux 原生 / Git Bash 均自带 |

## 7. 跨环境差异（脚本已内部处理）

| 环境 | 识别方式 | 环境变量传递 | 路径转换 |
|---|---|---|---|
| native（Linux / macOS） | `uname -s` 非 MSYS/MINGW | 直接 | 不需要 |
| WSL | `WSL_DISTRO_NAME` 或 `/proc/version` 含 microsoft | **必须经 `WSLENV`**（否则 Windows 版 go 忽略 `GOOS`，产出 PE 文件） | 输出路径用 `wslpath -w` |
| Git Bash / MSYS2 / Cygwin | `uname -s` 为 MSYS*/MINGW*/CYGWIN* | 直接可用 | 输出路径用 `cygpath -w` |

打 zip 时的后端回退顺序（缺一个自动换下一个）：

```
zip  →  Windows 自带 tar.exe（bsdtar，Win10+）  →  powershell.exe  →  python3
                    ↑ 生成标准正斜杠路径                    ↑ Compress-Archive 会写反斜杠
                                                              路径，跨平台解压会错乱，故排后
```

全都没有时报错并列出四种可选方式。

## 8. 常见用法配方

```bash
# 只出 Windows 包（本机验证用）
bash Deploy/Script/PackSnapshot.sh --target windows/amd64

# 只出 Ubuntu arm64（比如只要部署到 ARM 服务器）
bash Deploy/Script/PackSnapshot.sh --target linux/arm64

# 模拟「关联 PR #123 的快照」
bash Deploy/Script/PackSnapshot.sh --pr 123

# 复现 CI 产物并保留 pkg/ 便于查看包内结构
bash Deploy/Script/PackSnapshot.sh --version v0.1.1.00123.2610241530 --keep-tmp

# 前端未构建过
bash Deploy/Script/PackSnapshot.sh --build-web
```

## 9. 产物校验

```bash
# 1) 架构确认
tar -xzOf Dist/*_amd64.tar.gz  "*_amd64/finvquant"   | head -c 20 | od -An -tx1 | tail -1
#   amd64 → 02 00 3e 00       (EM_X86_64)
tar -xzOf Dist/*_arm64.tar.gz  "*_arm64/finvquant"   | head -c 20 | od -An -tx1 | tail -1
#   arm64 → 02 00 b7 00       (EM_AARCH64)
unzip -p Dist/*_windows_amd64.zip "*/finvquant.exe"    | head -c 2  | od -An -c | head -1
#   PE    → M   Z

# 2) 三包时间戳一致性
for f in Dist/*.tar.gz; do tar -xzOf "$f" --wildcards '*/BUILD-INFO.txt' | grep 构建时间戳; done
unzip -p Dist/*.zip "*/BUILD-INFO.txt" | grep 构建时间戳

# 3) zip 内路径必须是正斜杠（反斜杠会在 Linux 解压出错）
unzip -l Dist/*.zip | grep -c '\\\\'   # 期望 0
```

## 10. 常见问题

| 现象 | 原因与处理 |
|---|---|
| `.cmd` 报 `No such file or directory: D:\...\PackSnapshot.sh` | 选到了 `C:\Windows\System32\bash.exe`（WSL 转发器，把 Windows 路径当 Linux 路径）。脚本已主动跳过；若仍出现，确认 `git.exe` 在 PATH 中以便推导 `<GitRoot>\bin\bash.exe` |
| `.cmd` 报「未找到可用的 bash」 | 未装 Git for Windows / MSYS2。装 Git for Windows 后重试，或直接在 WSL 执行 `bash Deploy/Script/PackSnapshot.sh` |
| WSL 下产物是 PE 文件（`file` 显示 PE32+） | `GOOS` 未传进 Windows 版 go。脚本已用 `WSLENV` 处理；手工调用时不要直接 `GOOS=linux go.exe build` |
| 报「缺少命令：zip」/ zip 相关失败 | Git Bash 无 `zip`，脚本会回退到 Windows 自带 `tar.exe`；Windows 10 以下无 `tar.exe`，可装 `zip` 或用 WSL |
| zip 内路径是 `dir\sub\file` | 用了 PowerShell 的 `Compress-Archive`（历史 bug）。脚本优先选 `zip` 或 bsdtar，不会走到 PowerShell，除非前两者都不可用 |
| 报「缺少前端产物 internal/webui/Dist/index.html」 | 先 `bash Deploy/Script/PackSnapshot.sh --build-web`，或手工 `cd Web && npm ci && npm run build` |
| 报「版本格式不合法」 | `--version` 必须严格为 `vX.Y.Z.PPPPP.yymmddHHmm`（PR 段 5 位、时间戳 10 位） |
| 包内二进制名异常（如 `finvquant''`） | 历史 bug（`target_meta` 空字段被 `read` 丢弃）已修复；若复现请升级脚本 |
| `Dist/` 被误提交 | `.gitignore` 已忽略 `Dist/`、`pkg/`、`.tmp-snap/` |

## 11. 与 CI 的关系

`.github/workflows/CI.yml` 中：

| job | 作用 |
|---|---|
| `snapshot-meta` | 计算 `v{VERSION}.{PPPPP}.{yymmddHHmm}`，输出 `version`/`base_version`/`pr`/`timestamp`/`commit` |
| `snapshot-binaries` | 矩阵 `linux/amd64`、`linux/arm64`、`windows/amd64`，每格执行 `bash Deploy/Script/PackSnapshot.sh --version ... --target ...`，上传 `Dist/*` |
| `snapshot-release` | 汇总三个包，以快照版本为 tag 建 **prerelease** |

CI 调用方式（与本地完全相同，只多传 `--version` 与 `--commit`）：

```yaml
bash Deploy/Script/PackSnapshot.sh \
  --version "${SNAPSHOT_VERSION}" \
  --target "${{ matrix.goos }}/${{ matrix.goarch }}" \
  --commit "${{ needs.snapshot-meta.outputs.commit }}" \
  --out dist
```

## 12. 维护约定

- 新增平台：在 `PackSnapshot.sh` 的 `target_meta()` 与 `TARGETS` 里加一行，同步 CI 的 `matrix.include`，并在本文件 §2/§5 补产物说明。
- 调整包内文件：只改 `PackSnapshot.sh` 的装配段，`.cmd` 与 CI 无需改动。
- 新增参数：同步 `.cmd` 头部注释与本文件 §3。
- 产物格式（`tar.gz` / `zip`）变化时，同步 CI 的 `upload-artifact` 与 `snapshot-release` 的附件路径。

## 13. 一键下载与解压（Download.sh / Download.cmd）

不构建、不装 Go：直接按**本机操作系统与 CPU 架构**从 GitHub Releases 拉取官方发行包，校验后解压到指定目录。
默认取**最新版本（含快照/构建包）**，也可只取正式版。

### 13.1 快速开始

```bash
# Linux（默认解压到 Native 安装根目录 /data/App/FinvQuant）
sudo bash Deploy/Script/Download.sh

# 指定目录 + 只取正式版
bash Deploy/Script/Download.sh -c release -d /opt/FinvQuant

# 指定某个版本（可省略前导 v）
bash Deploy/Script/Download.sh -v v0.1.3.00357.2610281256 -d /tmp/FinvQuant

# 只下载不安装（保留压缩包与 .sha256）
bash Deploy/Script/Download.sh --no-extract -o ./Dist

# 先看会下载什么
bash Deploy/Script/Download.sh --dry-run
bash Deploy/Script/Download.sh --list          # 最近 10 个版本及类型
```

```bat
REM Windows（Git Bash 透传，默认解压到 .\FinvQuant）
Deploy\Script\Download.cmd -d D:\FinvQuant
```

### 13.2 版本渠道（`-c/--channel`）

| 渠道 | 选中条件 | 典型 tag | 稳定性 |
|---|---|---|---|
| `auto`（默认） | API/feed 中的最新一条（含 prerelease） | 任意 | 可能是快照，**不保证稳定** |
| `release` | 正式版（semver 三段，非 prerelease） | `v0.1.3` | 稳定，仅 `main` 合并后存在 |
| `snapshot` / `prerelease` | 最新 prerelease（快照包或 PR 构建包） | `v0.1.3.00357.2610281256` | 集成验证产物 |
| `pr` | 最新 PR 构建包（四段） | `v0.1.3.00357` | 单个 PR 的构建产物 |

> 当前仓库尚无正式版 release 时，`-c release` 会明确报错并提示先用 `--list` 查看。

### 13.3 平台匹配

| `uname -s` / `uname -m` | 选中资产 | 备注 |
|---|---|---|
| Linux + `x86_64` | `...-ubuntu-amd64.tar.gz` | Debian/Ubuntu 走 Native 安装；其它发行版静态二进制可直接运行 |
| Linux + `aarch64` | `...-ubuntu-arm64.tar.gz` | 同上 |
| Windows（MINGW/MSYS/CYGWIN）+ `x86_64` | `...-windows-amd64.zip` | 包内为 Docker 部署编排 |
| macOS / 其它架构 | — | 明确报错（官方暂未发布），需从源码构建 |

### 13.4 校验与解压行为

1. **资产级校验**：下载 `<资产>.sha256` 后用 `sha256sum -c` 校验压缩包（`--no-verify` 可跳过）。
2. **包内校验**：解压后校验 `SHA256SUMS`（包内每个文件的 sha256）。
3. **安全解压**：解压前扫描包内路径，含绝对路径或 `..` 直接拒绝；先解到临时目录再落盘，失败不留半成品。
4. **落盘方式**：默认**铺平**（包内顶层目录内容直接进目标目录，与仓库结构的 `finvquant` + `Deploy/` 一致）；`--keep-top` 保留 `FinvQuant-<版本>-<os>-<arch>/`。
5. **不误覆盖**：目标目录非空时报错退出；`--force` 才继续（同名文件覆盖、同名目录合并，**不删除**目标里已有的多余文件）。

### 13.5 常用参数

| 参数 | 说明 |
|---|---|
| `-d, --dir <目录>` | 解压目标目录（Linux 默认 `/data/App/FinvQuant`；Windows 默认 `.\FinvQuant`） |
| `-c, --channel <渠道>` | `auto` / `release` / `snapshot` / `pr` |
| `-v, --version <版本>` | 指定 tag（不联网解析列表，便于下载历史版本/镜像） |
| `-o, --out <目录>` | 下载缓存目录（默认临时目录，完成即清理） |
| `--base-url <url>` | 下载源根地址，默认 `https://github.com`（可指向内网镜像/加速代理） |
| `-r, --repo <o/r>` | 仓库，默认 `ACANX/FinvQuant` |
| `-t, --token <token>` | 私有仓库或规避 API 限流；也可用 `GH_TOKEN` / `GITHUB_TOKEN` |
| `--list` / `--dry-run` | 列版本 / 只打印下载地址与目标 |
| `--keep-top` / `--no-extract` / `--force` / `--no-verify` | 见 §13.4 |

> 无 token 时走公开 `releases.atom`（近 10 条，含 prerelease）；有 token 时走 Releases API（近 100 条，排除 draft）。

### 13.6 下载后直接安装（Linux）

```bash
sudo bash Deploy/Script/Download.sh -d /data/App/FinvQuant
# 安装根目录默认即 /data/App/FinvQuant，可省略 --prefix
sudo bash /data/App/FinvQuant/Deploy/Native/Install.sh
```

包内已含 `finvquant`、`Deploy/Migrations`、`Deploy/Native` 全套脚本，`Install.sh` 会补齐用户/目录/数据库/Redis/systemd 并启动服务（见 [`../Native/Install.md`](../Native/Install.md)）。

### 13.7 与 PackSnapshot 的关系

| 维度 | `PackSnapshot.sh` | `Download.sh` |
|---|---|---|
| 方向 | 源码 → 发行包（构建/出包） | 发行包 → 本机（下载/解压） |
| 依赖 | Go 1.25、Node、zip、git | curl、tar/unzip、sha256sum |
| 产物 | `Dist/FinvQuant-*.{tar.gz,zip}` | 解压后的程序目录 |
| 依赖关系 | 不依赖 Download | 依赖 PackSnapshot 定义的**命名规则**（`FinvQuant-<版本>-<os>-<arch>`） |

> 命名规则变更时**两处都要同步**：`PackSnapshot.sh` 的装配段负责产出，`Download.sh` 的 `ASSET` / `TOP_DIR` 负责取用。

### 13.8 常见问题

| 现象 | 处理 |
|---|---|
| `获取版本列表失败` | 网络不通或仓库名错误；可加 `-v <版本> --base-url <镜像>` 绕过列表 |
| `渠道 release 下没有可用版本` | 尚无正式版 release（`main` 未发布）；用默认 `auto` 或 `-c snapshot` |
| 下载很慢/中断 | 用 `--base-url` 指向镜像；或 `-o` 缓存后重跑（已下好的同名文件会跳过） |
| `目标目录非空` | 换 `-d` 目录，或确认后加 `--force` |
| Windows 提示未找到 bash | 安装 Git for Windows（含 Git Bash），不要用 `C:\Windows\System32\bash.exe`（WSL 转发器） |
| `SHA256 校验失败` | 文件损坏或镜像被篡改，删除缓存后重下；勿用 `--no-verify` 绕过 |
