@chcp 65001 >nul
@echo off
REM =====================================================================
REM FinvQuant 一键下载（Windows 封装）
REM
REM 真正的下载/校验/解压逻辑在 Download.sh（唯一实现，Linux 与 Windows 共用）。
REM 本脚本只负责找到可用的 bash（Git Bash / MSYS2 / Cygwin）并转调。
REM
REM 用法（与 .sh 完全一致）：
REM   Deploy\Script\Download.cmd [选项]
REM
REM   -d, --dir <目录>      解压目标目录（Windows 默认 <当前目录>\FinvQuant）
REM   -c, --channel <渠道>  auto（默认，最新含快照）/ release / snapshot / pr
REM   -v, --version <版本>  指定 tag
REM   -r, --repo <o/r>      GitHub 仓库（默认 ACANX/FinvQuant）
REM   -t, --token <token>   访问令牌（默认取 GH_TOKEN / GITHUB_TOKEN）
REM   -o, --out <目录>      下载缓存目录（默认临时目录）
REM       --base-url <url>  下载源根地址（默认 https://github.com）
REM       --keep-top        保留包内顶层目录
REM       --no-extract      只下载、不解压
REM       --force           目标目录非空时继续
REM       --no-verify       跳过 SHA256 校验
REM       --dry-run         只打印将要下载的地址
REM       --list            列出最近 10 个版本
REM   -h, --help            帮助
REM
REM 示例：
REM   Deploy\Script\Download.cmd
REM   Deploy\Script\Download.cmd -d D:\FinvQuant
REM   Deploy\Script\Download.cmd -c release -d D:\FinvQuant
REM
REM 依赖：Git for Windows（自带 Git Bash 与 unzip）；curl 已随 Windows 提供。
REM       C:\Windows\System32\bash.exe 是 WSL 转发器，无法处理 Windows 路径，本脚本会跳过它。
REM =====================================================================
setlocal EnableExtensions EnableDelayedExpansion

set "SCRIPT_DIR=%~dp0"
set "SH=%SCRIPT_DIR%Download.sh"

if not exist "%SH%" (
    echo [dl] 错误：未找到 %SH%
    echo      请确认从仓库内的 Deploy\Script\ 目录运行本脚本。
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
    for %%k in ("!GITDIR!..") do set "GITROOT=%%~fk"
    if exist "!GITROOT!\bin\bash.exe" set "BASH=!GITROOT!\bin\bash.exe"
)

REM 2) Git for Windows 常见安装位置
if not defined BASH if exist "%ProgramFiles%\Git\bin\bash.exe" set "BASH=%ProgramFiles%\Git\bin\bash.exe"
if not defined BASH if exist "%ProgramFiles(x86)%\Git\bin\bash.exe" set "BASH=%ProgramFiles(x86)%\Git\bin\bash.exe"
if not defined BASH if exist "%LOCALAPPDATA%\Programs\Git\bin\bash.exe" set "BASH=%LOCALAPPDATA%\Programs\Git\bin\bash.exe"

REM 3) PATH 中的其它 bash（MSYS2 / Cygwin），跳过 System32 与 SysWOW64
if not defined BASH (
    for /f "delims=" %%i in ('where bash 2^>nul') do (
        if not defined BASH (
            echo %%i | find /i "System32" >nul
            if errorlevel 1 (
                echo %%i | find /i "SysWOW64" >nul
                if errorlevel 1 set "BASH=%%i"
            )
        )
    )
)

if not defined BASH (
    echo [dl] 错误：未找到可用的 bash
    echo      本脚本复用 Download.sh 的下载逻辑，需要以下之一：
    echo        1^) Git for Windows（含 Git Bash）—— 推荐
    echo        2^) MSYS2 / Cygwin
    echo      注意：C:\Windows\System32\bash.exe 是 WSL 转发器，不能用于本脚本。
    echo      也可直接在 WSL / Linux 中执行：
    echo        bash Deploy/Script/Download.sh
    exit /b 1
)

echo [dl] 使用 bash : %BASH%
"%BASH%" --noprofile --norc "%SH%" %*
exit /b %ERRORLEVEL%
