@echo off
setlocal EnableDelayedExpansion

REM =====================================================================
REM FinvQuant Windows 一键部署脚本（deploy.cmd）
REM ---------------------------------------------------------------------
REM 功能：首次/重新在 Windows 11 Docker 上部署 FinvQuant（All-in-One）。
REM   步骤：①生成/核对 Deploy\.env 环境变量 → ②启动 docker compose 三个服务
REM         → ③等待健康 → ④验证（版本/存活/迁移/数据目录）。
REM 用法：  Deploy\deploy.cmd
REM   可选：Deploy\deploy.cmd --no-env    （跳过 .env 生成环节，直接启动）
REM 前提：  仓库根目录执行；已安装 Docker Desktop；网络可达 ghcr.io。
REM 说明：  镜像默认从 GHCR 拉取（ghcr.io/acanx/finvquant:latest）。
REM        数据库/Redis 数据持久化在宿主机（见 .env 中 *_DATA_DIR 与命名卷）。
REM =====================================================================

set "ROOT=%~dp0.."
pushd "%ROOT%" || (echo [ERROR] 无法进入仓库根目录 %ROOT% & exit /b 1)

set "COMPOSE_CMD=docker compose -f Deploy\docker-compose.yml --env-file Deploy\.env"
for /f "tokens=1,* delims==" %%a in ('findstr /i "FINV_IMAGE_TAG" Deploy\.env 2^>nul') do set "IMG_TAG=%%b"
if not defined IMG_TAG set "IMG_TAG=latest"
set "DO_ENV=1"
for %%A in (%*) do ( if /I "%%A"=="--no-env" set "DO_ENV=0" )

echo.
echo ============================================================
echo   FinvQuant 一键部署脚本
echo   工作目录：%ROOT%
echo ============================================================
echo.

REM ---------- 0. 前置检查 ----------
echo [1/5] 前置检查 ...
docker version >nul 2>&1 || (echo [ERROR] Docker 未运行，请先启动 Docker Desktop & exit /b 1)
echo [OK] Docker 可用。

REM ---------- 1. 生成 / 核对 .env ----------
echo.
if "%DO_ENV%"=="1" (
    echo [2/5] 环境变量文件（.env）检查 ...
    if exist "Deploy\.env" (
        echo   [OK] Deploy\.env 已存在，将沿用现有配置。
        echo   - 如需重新生成（恢复默认），请先手动删除 Deploy\.env 后重新执行本脚本
        echo   - 关键配置项见下方核对：
    ) else (
        echo   [提示] 未找到 Deploy\.env，将从 Deploy\.env.example 生成默认配置。
        if not exist "Deploy\.env.example" (echo [ERROR] 缺少 Deploy\.env.example，无法生成 & popd & exit /b 1)
        copy /Y "Deploy\.env.example" "Deploy\.env" >nul
        echo   [OK] 已生成 Deploy\.env（从 .env.example 复制）
        echo   - ??  请核对以下关键项，必要时编辑 Deploy\.env 后重新执行：
    )
    echo   ── 关键配置核对 ────────────────────────────────
    for /f "tokens=1,* delims==" %%a in ('findstr /i "^FINV_PG_USER= ^FINV_PG_PASSWORD= ^FINV_PG_DATABASE= ^FINV_PG_EXPOSE_PORT= ^FINV_REDIS_EXPOSE_PORT= ^FINV_IMAGE_TAG= ^FINV_CONTAINER_NAME= ^FINV_PROJECT_NAME=" Deploy\.env 2^>nul') do echo    %%a=%%b
    echo   ────────────────────────────────────────────────
    echo   - 数据库数据目录：在 Deploy\.env 中 FINV_PG_DATA_DIR（Windows 示例 D:\Dev\Docker\HostFileSystem\FinvQuant\PostgreSQL）
    echo   - Redis 数据：命名卷 finvquant_finvquant-redisdata（容器重建不丢）
    echo   - 镜像来源：ghcr.io/acanx/finvquant:!IMG_TAG!
    echo.
) else (
    echo [2/5] 跳过 .env 生成（--no-env），要求 Deploy\.env 已存在
    if not exist "Deploy\.env" (echo [ERROR] 缺少 Deploy\.env，请先运行 Deploy\deploy.cmd 生成 & popd & exit /b 1)
)

REM ---------- 2. 拉取镜像 ----------
echo [3/5] 拉取镜像（ghcr.io/acanx/finvquant:!IMG_TAG!）...
REM 网络波动容错：自动重试最多 3 次（每次间隔 5s）
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
    echo [ERROR] 拉取镜像连续 3 次失败，请检查 ghcr.io 网络可达性
    popd & exit /b 1
:pull_ok
echo [OK] 镜像已就绪。

REM ---------- 3. 启动 compose（首次创建全部服务） ----------
echo.
echo [4/5] 启动 Docker Compose 服务（postgres + redis + finvquant）...
%COMPOSE_CMD% up -d
if errorlevel 1 (
    echo [ERROR] 服务启动失败，查看日志：%COMPOSE_CMD% logs --tail=100
    popd & exit /b 1
)
echo [OK] 服务已启动。

REM ---------- 4. 等待健康 + 验证 ----------
echo.
echo [5/5] 等待服务健康（最长 120s）...
set /a WAIT=0
:waitloop
    if !WAIT! geq 24 (
        echo [WARN] 健康检查超时，请手动查看：%COMPOSE_CMD% ps
        goto :verify
    )
    set "ALLHEALTHY=1"
    for /f "tokens=*" %%s in ('docker inspect finvquant postgres redis --format "{{.Name}}={{.State.Health.Status}}" 2^>nul') do (
        echo   %%s
        echo %%s | findstr /v "=healthy" >nul && set "ALLHEALTHY=0"
    )
    if "!ALLHEALTHY!"=="1" goto :healthy
    timeout /t 5 /nobreak >nul
    set /a WAIT+=1
    goto :waitloop
:healthy
echo [OK] 全部服务健康。

:verify
echo.
echo 验证部署结果 ...
echo   - 服务端版本接口：
curl -s http://localhost:16001/API/V1/Version 2>nul & echo.
echo   - 存活探针：
curl -s http://localhost:16001/API/V1/Health/Live 2>nul & echo.
echo   - 容器状态：
docker ps --filter "name=finvquant" --filter "name=postgres" --filter "name=redis" --format "  {{.Names}}: {{.Status}}"
echo   - 数据库迁移记录（最新 3 条）：
docker exec fq-postgres psql -U finvquant -d finvquant -tc "SELECT version FROM schema_version ORDER BY version::int DESC LIMIT 3;" 2>nul

echo.
echo ============================================================
echo   部署完成！
echo   - 前端控制台：http://localhost:16002
echo   - 服务端 API ：http://localhost:16001/API/V1/Health/Live
echo   - 日常升级：Deploy\upgrade.cmd
echo ============================================================
echo.

popd
endlocal
exit /b 0
