# 对象存储配置（OSSConfig）

> 所属：导航「备份与恢复 → 备份管理 → 对象存储配置」
> 路由：`/Meta/BRS/Backup/Finv/OSSConfig`
> 视图：`Web/src/views/Meta/BRS/Backup/Finv/BrsOssConfigView.vue`
> 菜单 key：`brs-storage`
> 数据表：备份对象存储配置（端点、桶、访问密钥）
> 接口：`/Meta/BRS/Backup/Finv/OSSConfig`
> 关联接口文档：[`Docs/API/BRS/OSSConfig.md`](../../../../API/BRS/OSSConfig.md)

## 1. 用途

维护备份产物的对象存储配置，保存后下一次备份与上传按新配置执行。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 配置表单 | 端点、桶、路径前缀、访问密钥；密钥不回显明文。 |
| 连通性检查 | 保存前可测试连接。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/BRS/Backup/Finv/OSSConfig` | 读取对象存储配置 |
| `POST` | `/Meta/BRS/Backup/Finv/OSSConfig` | 保存对象存储配置 |

## 4. 说明

密钥属敏感信息，页面不回显；修改后需重新执行一次备份验证。
