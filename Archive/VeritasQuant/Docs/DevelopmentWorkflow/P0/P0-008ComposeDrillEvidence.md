# P0-008 Docker Compose 技术演练证据

## 结果

- 演练创建时间：`2026-07-31T09:31:27Z`。
- 清理复查时间：`2026-07-31T09:33:58.014Z`。
- 结论：临时 PostgreSQL 与 Redis 环境的配置、启动、健康检查和清理均成功。
- 限制：本记录是自动化技术证据，不能替代独立人类 SRE/QA 验收。

## 执行命令与退出码

在仓库根目录使用 Python 3.13 执行：

| 命令 | 退出码 | 结果 |
| --- | ---: | --- |
| `docker compose -f Docker/docker-compose.yml config --quiet` | 0 | 配置有效。 |
| `python3 scripts/VerifyDevelopmentEnvironment.py --action start` | 0 | `postgresql` 和 `redis` 均进入 `running (healthy)`。 |
| `python3 scripts/VerifyDevelopmentEnvironment.py --action check` | 0 | Compose 状态和容器 inspect 均成功。 |
| `python3 scripts/VerifyDevelopmentEnvironment.py --action stop` | 0 | 项目容器、网络和卷均被删除。 |

## 镜像与隔离检查

| 服务 | 固定镜像摘要 | 隔离结果 |
| --- | --- | --- |
| PostgreSQL | `postgres@sha256:5660c2cbfea50c7a9127d17dc4e48543eedd3d7a41a595a2dfa572471e37e64c` | `5432/tcp` 未发布到主机；无 mounts；`/var/lib/postgresql/data` 使用 tmpfs。 |
| Redis | `redis@sha256:e7723ff73d963f5cc6d9c4643ea3d989527a402a319239054e9472a7fb9219a2` | `6379/tcp` 未发布到主机；无 mounts；`/data` 使用 tmpfs。 |

停止后，按 Compose 项目 label 复查的容器、网络和卷数量均为 `0`。未接触任何非本项目容器。

## 2026-08-01 独立复核快照

- 复核时间：`2026-08-01T20:18:00Z`。
- 复核对象：`Docker/docker-compose.yml`、`scripts/VerifyDevelopmentEnvironment.py`、`tests/contract/test_p0_engineering_baseline.py` 与 `Docker/DevelopmentEnvironment.md`。
- Compose 顶层无 `volumes`；`postgresql`（`postgres:16.4-alpine`）与 `redis`（`redis:7.4-alpine`）均未发布宿主端口、无 mounts，数据目录使用 `tmpfs`，且均配置健康检查。
- `VerifyDevelopmentEnvironment.py`：`start` 使用 `up --detach --wait --remove-orphans`（等待健康检查），`stop` 使用 `down --volumes --remove-orphans`（清理容器、网络与匿名卷）；Docker 不可用时明确失败，不回退到本机服务。
- 契约测试 `P0-008-001`（`test_development_compose_is_ephemeral_and_has_health_checks`）通过；`.github/workflows/Ci.yml` 的单元/契约/打包测试步骤覆盖该测试。
- 演练证据（2026-07-31 自动化执行）与上述静态配置一致；镜像摘要已在演练时记录，运行时按 tag 拉取。

本快照证明 P0-008 验收标准（一条命令启动并通过健康检查；停止后无遗留秘密或业务数据）的实现证据与自动化演练相符。独立人类 SRE 对原始运行日志的复核仍作为 M0 Gate 前的治理行动保留，不替代或改变本工作项的验收。
