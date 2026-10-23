# 本地开发依赖环境

`Docker/docker-compose.yml` 只提供 PostgreSQL 和 Redis 的临时开发实例。容器不暴露宿主机端口，不挂载卷，且数据库和 Redis 数据目录均使用 `tmpfs`；因此停止并清理后不会保留业务数据或秘密。

启动、等待健康检查并输出状态：

```powershell
python3 scripts/VerifyDevelopmentEnvironment.py --action start
```

停止并删除容器、网络和匿名卷：

```powershell
python3 scripts/VerifyDevelopmentEnvironment.py --action stop
```

该环境仅限本地开发，不能用于模拟盘、券商仿真或实盘。`POSTGRES_HOST_AUTH_METHOD=trust` 只适用于未暴露端口的短生命周期容器；任何持久化、端口映射或凭据接入均须先走 Change 并更新技术方案。
