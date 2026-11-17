# P1-040 账户与分账户隔离证据

已定义强类型 `AccountScopeV1`，缺失 `account_id` 无法通过模型校验；`AccountStateRouterV1` 仅允许对已注册账户进行状态变更，并为每个账户保存独立资源桶。跨账户汇总或调拨未提供隐式接口。

```powershell
python3 -m pytest tests\unit\accounts\test_routing.py -q
```
