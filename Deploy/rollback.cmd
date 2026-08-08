@echo off
setlocal EnableDelayedExpansion

REM =====================================================================
REM FinvQuant Windows 本地回滚脚本（rollback.cmd）
REM ---------------------------------------------------------------------
REM 功能：升级后若服务异常，回滚到上一版本镜像并重建容器。
REM   默认：回滚到 GHCR 上一发布版本（FINV_IMAGE_TAG 指向旧 tag）；
REM   也可：回滚到本地备份镜像（如 docker tag 旧镜像后指定 --tag <镜像>）。
REM 用法：  Deploy\rollback.cmd                （按 .env 的 FINV_IMAGE_TAG 拉取并重建）
REM         Deploy\rollback.cmd --tag v0.1.0   （回滚到指定镜像 tag）
REM 前提：  仓库根目录执行；数据库迁移不可自动回滚，详见 Win11DockerUpgrade.md 第 5 节。
REM =====================================================================

set "ROOT=%~dp0.."
pushd "%ROOT%" || (echo [ERROR] 无法进入仓库根目录 %ROOT% & exit /b 1)

set "COMPOSE_CMD=docker compose -f Deploy\docker-compose.yml --env-file Deploy\.env"

echo.
echo ============================================================
echo   FinvQuant 回滚脚本
echo ============================================================
echo.

echo [1/3] 确认目标镜像 tag（.env 的 FINV_IMAGE_TAG）...
for /f "tokens=1,* delims==" %%a in ('findstr /i "FINV_IMAGE_TAG" Deploy\.env 2^>nul') do set "TAG=%%b"
if not defined TAG set "TAG=latest"
echo   目标 FINV_IMAGE_TAG=!TAG!
echo   如需回滚到指定版本，请先修改 Deploy\.env 的 FINV_IMAGE_TAG（如 v0.1.0）后重新执行本脚本

echo [2/3] 拉取目标镜像 ...
%COMPOSE_CMD% pull finvquant
if errorlevel 1 (echo [ERROR] 拉取镜像失败 & popd & exit /b 1)

echo [3/3] 重建并启动容器（postgres / redis 不重建，数据保留）...
%COMPOSE_CMD% up -d --force-recreate finvquant
if errorlevel 1 (echo [ERROR] 容器启动失败 & popd & exit /b 1)

echo.
echo 验证：docker ps --filter "name=finvquant" 与 http://localhost:16002
echo.
echo ??  注意：
echo   1. 数据库迁移不可自动回滚。若本次回滚是因为迁移失败导致服务不可用，
echo      应先修复迁移脚本发布新镜像，而非直接回滚（详见 Win11DockerUpgrade.md 第 5 节）。
echo   2. 完整手册见 Deploy\DeployUpgradeGuide.md
echo.

popd
endlocal
exit /b 0
