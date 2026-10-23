@echo off
REM =====================================================================
REM FinvQuant 快照包构建（Windows 封装）
REM
REM 真正的打包逻辑在 PackSnapshot.sh（唯一实现，CI 与本地共用）。
REM 本脚本只负责：找到可用的 bash（Git Bash / MSYS2 / Cygwin）并转调，
REM 避免在 .cmd 里复制一份打包逻辑导致两边漂移。
REM
REM 用法（与 .sh 完全一致）：
REM   Deploy\Script\PackSnapshot.cmd [选项]
REM
REM   --version <v>       快照版本（缺省则用 --pr/--timestamp + VERSION 现算）
REM   --target <t>        linux/amd64 | linux/arm64 | windows/amd64 | all（默认 all）
REM   --pr <n>            关联 PR 编号（默认 00000）
REM   --timestamp <ts>    时间戳 yymmddHHmm（默认当前 Asia/Shanghai 时间）
REM   --base-version <v>  基础版本（默认读 VERSION 文件）
REM   --commit <sha>      提交号（默认 git rev-parse --short HEAD）
REM   --out <dir>         输出目录（默认 <仓库根>\dist）
REM   --build-web         强制重新构建前端
REM   --keep-tmp          保留 pkg\ 临时目录
REM   -h, --help          帮助
REM
REM 示例：
REM   Deploy\Script\PackSnapshot.cmd
REM   Deploy\Script\PackSnapshot.cmd --target windows/amd64
REM   Deploy\Script\PackSnapshot.cmd --version v0.1.1.00123.2610241530
REM
REM 依赖：Git for Windows（自带 Git Bash）；Go 1.25+；打包 Windows 包需 zip
REM       （Git Bash 通常自带）。C:\Windows\System32\bash.exe 是 WSL 转发器，
REM       无法处理 Windows 路径，本脚本会主动跳过它。
REM =====================================================================
setlocal EnableExtensions EnableDelayedExpansion

set "SCRIPT_DIR=%~dp0"
set "SH=%SCRIPT_DIR%PackSnapshot.sh"

if not exist "%SH%" (
    echo [pack] 错误：未找到 %SH%
    echo         请确认从仓库内的 Deploy\Script\ 目录运行本脚本。
    exit /b 1
)

REM ---- 定位真正的 bash（排除 System32\bash.exe 这个 WSL 转发器）----
set "BASH="

REM 1) 由 git.exe 位置推导：<GitRoot>\cmd\git.exe -> <GitRoot>\bin\bash.exe
set "GITDIR="
for /f "delims=" %%i in ('where git 2^>nul') do (
    if not defined GITDIR for %%j in ("%%i") do set "GITDIR=%%~dpj"
)
if defined GITDIR (
    REM %%~fk 会规范化掉 "cmd\.." 这一段，得到 <GitRoot>（无尾部反斜杠）
    for %%k in ("!GITDIR!..") do set "GITROOT=%%~fk"
    if exist "!GITROOT!\bin\bash.exe" set "BASH=!GITROOT!\bin\bash.exe"
)

REM 2) Git for Windows 常见安装位置
if not defined BASH if exist "%ProgramFiles%\Git\bin\bash.exe" set "BASH=%ProgramFiles%\Git\bin\bash.exe"
if not defined BASH if exist "%ProgramFiles(x86)%\Git\bin\bash.exe" set "BASH=%ProgramFiles(x86)%\Git\bin\bash.exe"
if not defined BASH if exist "%LOCALAPPDATA%\Programs\Git\bin\bash.exe" set "BASH=%LOCALAPPDATA%\Programs\Git\bin\bash.exe"

REM 3) PATH 中的其它 bash（MSYS2 / Cygwin），跳 System32 与 SysWOW64
if not defined BASH (
    for /f "delims=" %%i in ('where bash 2^>nul') do (
        if not defined BASH (
            REM find 是字面子串匹配，比 findstr 的正则转义更可靠
            echo %%i | find /i "System32" >nul
            if errorlevel 1 (
                echo %%i | find /i "SysWOW64" >nul
                if errorlevel 1 set "BASH=%%i"
            )
        )
    )
)

if not defined BASH (
    echo [pack] 错误：未找到可用的 bash
    echo         本脚本复用 PackSnapshot.sh 的打包逻辑，需要以下之一：
    echo           1^) Git for Windows（含 Git Bash）—— 推荐
    echo           2^) MSYS2 / Cygwin
    echo         注意：C:\Windows\System32\bash.exe 是 WSL 转发器，不能用于本脚本。
    echo         也可直接在 WSL / Linux 中执行：
    echo           bash Deploy/Script/PackSnapshot.sh
    exit /b 1
)

echo [pack] 使用 bash : %BASH%
"%BASH%" --noprofile --norc "%SH%" %*
exit /b %ERRORLEVEL%
