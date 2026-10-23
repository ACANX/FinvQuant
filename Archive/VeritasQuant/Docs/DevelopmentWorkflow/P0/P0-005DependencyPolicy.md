# P0-005 Python 版本、依赖分组与锁定策略

## 解释器

- 支持基线：Python 3.13+。
- 当前开发、测试、构建和部署默认解释器：Windows 11 上的 `python3`（Python 3.13）。
- CI 必须在 Windows 与 Linux 使用相同的 Python 3.13 版本执行最小基线；新增解释器支持须先更新权威技术方案并通过 Change 记录。

## 依赖分组

- 运行时依赖：`pyproject.toml` 的 `project.dependencies`，镜像到 `Requirements/Runtime.lock`。
- 开发依赖：`project.optional-dependencies.dev`，镜像到 `Requirements/Development.lock`。
- 安全、类型、格式、测试和构建工具只能进入开发依赖，不随运行 wheel 引入。

## 锁定与审阅

1. 使用受控的 Python 3.13 虚拟环境更新依赖解析。
2. 运行 `python3 scripts/VerifyDependencyLocks.py`，确认项目声明与锁文件一致。
3. 对锁文件 diff 审阅版本、许可证、漏洞、传递依赖和升级原因。
4. 在 Windows/Linux CI 从空缓存安装后，保存安装日志、测试报告和 SHA-256。
5. 未经 Change/评审不得手工改写已发布依赖版本或删除锁文件条目。

当前锁文件是本机 Python 3.13 的作者基线；Linux CI 的独立安装结果仍是 P0-006 必需证据。
