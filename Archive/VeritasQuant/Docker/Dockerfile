# syntax=docker/dockerfile:1
# VeritasQuant 服务端应用镜像（多阶段构建）。
#
# 目标环境：Windows 11 + Docker Desktop（WSL2 后端）。
# 该镜像仅用于模拟盘（PAPER）与券商仿真（SIMULATION）实验；
# 实盘交易默认禁用（TechSpec 13 阶段 5 gate 未通过前不得启用）。
#
# 阶段 1：builder —— 在隔离环境构建 wheel（不携带构建工具链到运行时）。
FROM python:3.13-slim AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /build

# 只复制构建元数据以利用层缓存；业务代码随后复制。
COPY pyproject.toml README.md ./
COPY src ./src

# 安装构建后端并构建 wheel；构建产物进入 /build/dist。
RUN python -m pip install --upgrade pip setuptools wheel \
    && python -m pip wheel --no-deps --wheel-dir /build/dist .

# ---------------------------------------------------------------------------
# 阶段 2：runtime —— 只携带运行时依赖与 wheel，非 root 运行。
FROM python:3.13-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    VQ_HOME=/opt/veritasquant

# 创建非 root 运行用户（容器内以 10001 运行，宿主映射友好）。
RUN groupadd --gid 10001 vq \
    && useradd --uid 10001 --gid 10001 --home-dir "$VQ_HOME" --create-home vq

WORKDIR "$VQ_HOME"

# 先装运行时依赖（利用层缓存）；再装应用 wheel。
COPY --from=builder /build/dist/*.whl /tmp/wheels/
RUN python -m pip install --upgrade pip setuptools \
    && python -m pip install /tmp/wheels/*.whl \
    && rm -rf /tmp/wheels

# 资源目录（错误码目录等打包数据随 wheel 安装，无需额外 COPY）。

# 容器内挂载点：运行产物、数据导入目录（由 compose 或 -v 提供）。
RUN mkdir -p "$VQ_HOME/runtime" "$VQ_HOME/data" \
    && chown -R vq:vq "$VQ_HOME"

USER vq

# 健康检查：默认只验证进程存活（容器编排层再做 readiness）。
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:18000/health/live', timeout=2).status==200 else 1)" || exit 1

EXPOSE 18000

# 默认入口：API 服务（--serve 才真正启动）。可用 docker compose 覆盖为
# vq-trading-worker / vq-scheduler-service 等其他 console script。
# 使用 12000 以后的端口，避开 8000/8080 等常用端口。
ENTRYPOINT ["vq-api-server"]
CMD ["--serve", "--host", "0.0.0.0", "--port", "18000"]
