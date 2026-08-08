@echo off
setlocal EnableDelayedExpansion

REM =====================================================================
REM FinvQuant Windows 版本检查与升级脚本（upgrade.cmd）
REM ---------------------------------------------------------------------
REM 功能：检查远端 GHCR 是否有比当前本地部署更新的镜像；如有则拉取并按需
REM       升级部署；全程输出关键要素与结果日志。
REM   ①拉取远端镜像最新版本信息（pull，幂等） → ②对比本地运行版本
REM   → ③若远端更新则执行升级部署（存量 DB/Redis/配置保留） → ④验证。
REM 用法：  Deploy\upgrade.cmd            （检查并按需升级到 latest）
REM         Deploy\upgrade.cmd --skip-backup   （跳过数据库备份，应急用）
REM         Deploy\upgrade.cmd --force    （即使版本相同也强制重建部署）
REM 前提：  仓库根目录执行；已安装 Docker Desktop；网络可达 ghcr.io。
REM 说明：  升级不重建 postgres/redis（配置未变则数据不丢）；数据库迁移由
REM        服务端启动自动应用（幂等）。回滚见 Deploy\rollback.cmd。
REM =====================================================================

set "ROOT=%~dp0.."
pushd "%ROOT%" || (echo [ERROR] 无法进入仓库根目录 %ROOT% & exit /b 1)

set "COMPOSE_CMD=docker compose -f Deploy\docker-compose.yml --env-file Deploy\.env"
set "IMG=ghcr.io/acanx/finvquant"
set "DO_BACKUP=1"
set "FORCE=0"
for %%A in (%*) do (
    if /I "%%A"=="--skip-backup" set "DO_BACKUP=0"
    if /I "%%A"=="--force"       set "FORCE=1"
)

REM ---------- 0. 前置检查 ----------
echo.
echo ============================================================
echo   FinvQuant 版本检查与升级脚本
echo   工作目录：%ROOT%
echo ============================================================
echo.
echo [1/8] 前置检查 ...
docker version >nul 2>&1 || (echo [ERROR] Docker 未运行，请先启动 Docker Desktop & exit /b 1)
if not exist "Deploy\.env" (
    echo [ERROR] 缺少 Deploy\.env，请先运行 Deploy\deploy.cmd 生成
    exit /b 1
)
docker ps --filter "name=finvquant" --format "{{.Names}}" | findstr /i "finvquant" >nul
if errorlevel 1 (
    echo [ERROR] 未检测到运行中的 finvquant 容器。首次部署请用 Deploy\deploy.cmd，或先启动服务
    popd & exit /b 1
)
echo [OK] Docker 可用，Deploy\.env 存在，finvquant 容器在运行。

REM ---------- 1. 读取当前本地运行版本 ----------
echo.
echo [2/8] 读取当前本地运行版本 ...
set "CUR_IMGID="
for /f "tokens=*" %%i in ('docker inspect finvquant --format "{{.Image}}" 2^>nul') do set "CUR_IMGID=%%i"
set "CUR_TAG="
for /f "tokens=1,* delims==" %%a in ('findstr /i "FINV_IMAGE_TAG" Deploy\.env 2^>nul') do set "CUR_TAG=%%b"
if not defined CUR_TAG set "CUR_TAG=latest"
set "CUR_SHORT=!CUR_IMGID:~7,12!"
echo   - 本地运行容器镜像 ID : !CUR_IMGID!（简写 !CUR_SHORT!）
echo   - 配置的目标 tag      : %CUR_TAG%
echo   - 容器状态            :
docker ps --filter "name=finvquant" --format "    {{.Names}}: {{.Status}}  (uptime {{.RunningFor}})"
echo   - 依赖服务            :
docker ps --filter "name=postgres" --format "    {{.Names}}: {{.Status}}"
docker ps --filter "name=redis" --format "    {{.Names}}: {{.Status}}"

REM ---------- 2. 拉取远端镜像（幂等，获取最新版本信息） ----------
echo.
echo [3/8] 拉取远端镜像最新版本（%IMG%:%CUR_TAG%）...
REM 网络波动容错：pull 强制联网检查远端 manifest，偶发 EOF/超时，
REM 自动重试最多 3 次（每次间隔 5s）后再判失败。
set /a PULL_TRY=0
:pull_retry
    set /a PULL_TRY+=1
    if !PULL_TRY! gtr 1 (
        echo   [重试] 第 !PULL_TRY!/3 次尝试拉取（网络波动时常见，自动重试）...
        timeout /t 5 /nobreak >nul
    )
    %COMPOSE_CMD% pull finvquant
    if not errorlevel 1 goto :pull_ok
    if !PULL_TRY! lss 3 goto :pull_retry
    echo [ERROR] 拉取远端镜像连续 3 次失败。
    echo   - 网络问题：ghcr.io 不可达 / 网络波动（可用 docker pull %IMG%:%CUR_TAG% 手工重试）
    echo   - 认证问题：需登录 GHCR（docker login ghcr.io）
    echo   - 本地已有镜像时也可直接执行：docker compose -f Deploy\docker-compose.yml --env-file Deploy\.env up -d
    popd & exit /b 1
:pull_ok
set "REMOTE_ID="
for /f "tokens=*" %%i in ('docker image inspect %IMG%:%CUR_TAG% --format "{{.Id}}" 2^>nul') do set "REMOTE_ID=%%i"
set "REMOTE_SHORT=!REMOTE_ID:~7,12!"
echo   - 远端（最新）镜像 ID : !REMOTE_ID!（简写 !REMOTE_SHORT!）
echo   - manifest 完整内容 :
REM 完整输出 docker manifest inspect 原始 JSON（不做任何处理/解析）
docker manifest inspect %IMG%:%CUR_TAG%

REM ---------- 识别镜像版本号（v{VERSION}-YYYYMMDDHHMM）与构建时间 ----------
REM docker 无"列出远端 tag"命令，用 digest 匹配探测（见 resolve-image-version.ps1）：
REM   探测候选 tag，命中 latest digest 即确认版本号；探测不到则退化显示构建时间。
REM 实现：ps1 的 -Friendly 模式直接输出单行友好信息，cmd 仅透传显示（规避 cmd
REM   多行解析 / for-f 变量 / 花括号 / 延迟展开等陷阱）。
set "VER_FILE="
for /f "tokens=*" %%v in ('type VERSION 2^>nul') do set "VER_FILE=%%v"
if not defined VER_FILE set "VER_FILE=0.1.0"
powershell -NoProfile -ExecutionPolicy Bypass -Command "& 'Deploy\resolve-image-version.ps1' -Image '%IMG%' -Version '%VER_FILE%' -Friendly"

REM ---------- 3. 版本对比 ----------
echo.
echo [4/8] 版本对比 ...
echo   - 本地运行 : !CUR_SHORT!
echo   - 远端最新 : !REMOTE_SHORT!
if not defined REMOTE_ID (
    echo [WARN] 无法获取远端镜像 ID，按需升级将跳过
    goto :verify
)
if "!CUR_IMGID!"=="!REMOTE_ID!" (
    if "%FORCE%"=="1" (
        echo   [判断] 版本相同（已是最新），因 --force 强制重建部署
        set "NEED_UPGRADE=1"
    ) else (
        echo   [判断] 版本相同（已是最新版本），无需升级。
        echo   [OK] 当前本地部署已是最新版本，退出。
        goto :done
    )
) else (
    echo   [判断] 远端有更新版本（!CUR_SHORT! -^> !REMOTE_SHORT!），执行升级部署。
    set "NEED_UPGRADE=1"
)

REM ---------- 4. 升级前备份 ----------
if "!NEED_UPGRADE!"=="1" (
    if "%DO_BACKUP%"=="1" (
        echo.
        echo [5/8] 升级前备份 ...
        if not exist "Deploy\backup" mkdir "Deploy\backup"
        set "BK=Deploy\backup\finvquant_%date:~0,4%%date:~5,2%%date:~8,2%_%time:~0,2%%time:~3,2%%time:~6,2%.dump"
        set "BK=!BK: =0!"
        echo   备份 .env -^> Deploy\.env.bak
        copy /Y "Deploy\.env" "Deploy\.env.bak" >nul
        echo   备份数据库（pg_dump，含表结构与数据）...
        docker exec fq-postgres pg_dump -U finvquant -d finvquant -F c -f /tmp/finvquant_backup.dump >nul 2>&1
        if errorlevel 1 (
            echo   [WARN] 数据库备份失败（可能 PG 未就绪），继续升级；如后续迁移异常请手动备份
        ) else (
            docker cp fq-postgres:/tmp/finvquant_backup.dump "!BK!" >nul 2>&1
            echo   [OK] 数据库备份完成：!BK!
        )
    ) else (
        echo.
        echo [5/8] 跳过备份（--skip-backup）
    )

    REM ---------- 5. 升级部署（仅重建 finvquant，保留 DB/Redis） ----------
    echo.
    echo [6/8] 执行升级部署 ...
    echo   - 重建服务：仅 finvquant（postgres / redis 配置未变，不重建、数据不丢）
    echo   - 存量数据：PG 数据目录绑定挂载 + Redis 命名卷，升级全程保留
    echo   - 环境配置：沿用 Deploy\.env（未修改）
    echo   - 数据库迁移：新版本服务启动时自动应用（幂等，见 schema_version 表）
    echo.
    echo   执行命令：%COMPOSE_CMD% up -d --force-recreate finvquant
    %COMPOSE_CMD% up -d --force-recreate finvquant
    if errorlevel 1 (
        echo [ERROR] 升级部署失败，查看日志：%COMPOSE_CMD% logs --tail=100 finvquant
        echo   如需回滚：Deploy\rollback.cmd
        popd & exit /b 1
    )
    echo [OK] finvquant 容器已重建为新版本。

    REM ---------- 6. 等待健康 ----------
    echo.
    echo [7/8] 等待服务健康检查（最长 90s）...
    set /a WAIT=0
    :waitloop
        if !WAIT! geq 18 (
            echo [WARN] 健康检查超时，请手动查看：%COMPOSE_CMD% logs --tail=100 finvquant
            goto :verify
        )
        for /f "tokens=*" %%s in ('docker inspect finvquant --format "{{.State.Health.Status}}" 2^>nul') do set "HSTATUS=%%s"
        if "!HSTATUS!"=="healthy" goto :healthy
        timeout /t 5 /nobreak >nul
        set /a WAIT+=1
        goto :waitloop
    :healthy
    echo [OK] 容器健康（healthy）。
)

REM ---------- 7. 验证 ----------
:verify
echo.
echo [8/8] 验证升级结果 ...
echo   - 服务端版本接口：
curl -s http://localhost:16001/API/V1/Version 2>nul & echo.
echo   - 存活探针：
curl -s http://localhost:16001/API/V1/Health/Live 2>nul & echo.
echo   - 容器状态：
docker ps --filter "name=finvquant" --format "  {{.Names}}: {{.Status}}"
echo   - 数据库迁移记录（最新 3 条）：
docker exec fq-postgres psql -U finvquant -d finvquant -tc "SELECT version FROM schema_version ORDER BY version::int DESC LIMIT 3;" 2>nul

:done
echo.
echo ============================================================
echo   本次升级操作完成。
echo   - 前端控制台：http://localhost:16002
echo   - 服务端 API ：http://localhost:16001/API/V1/Health/Live
echo   - 完整手册：Deploy\DeployUpgradeGuide.md
echo ============================================================
echo.

popd
endlocal
exit /b 0
