# 上游建议：`Nginx.conf.example` 需要支持「分阶段 TLS」模板

> 用途：向上游（FinvQuant 仓库）提交 Issue / PR 的需求说明。
> 现状模板：`Deploy/Native/Nginx.conf.example`（安装后为 `<PREFIX>/Deploy/Nginx.conf.example`）。
> 本文档可直接作为 Issue 正文提交。

---

## 建议 Issue 标题

**`Deploy/Native/Nginx.conf.example` 应默认按「HTTP-only 阶段一 + 注释式 TLS 阶段二」组织，避免无证书环境下开箱即失败**

## 背景与问题

当前模板默认按「已具备域名 + 有效证书 + 443/HTTP2 已放通」编写：

1. 第一个 `server` 块直接 `return 301 https://$host$request_uri;`；
2. 第二个 `server` 块 `listen 443 ssl;` + `http2 on;`，并引用
   `/etc/letsencrypt/live/<域名>/fullchain.pem`。

在没有证书、443 未放通、也没有域名的首次部署场景下：

- 直接 `cp` 模板后执行 `nginx -t` 会因**证书文件不存在**而失败；
- 即使跳过校验，80 端口的请求会被 301 到不可用的 443，**服务完全打不开**；
- 用户必须手工删除/注释大段配置才能用 IP 访问，容易改错（例如漏删跳转块、误删 ACME 目录）。

## 期望行为

模板应做到：

1. **默认可用**：`cp` 后 `nginx -t` 通过，`http://<服务器IP>` 直接访问前端（反代 16002）与 API（`/API/` → 16001）；
2. **不依赖域名**：`server_name _;` + `listen 80 default_server;`，无需先申请域名；
3. **TLS 配置整段注释**：443/HTTP2/证书/80→443 跳转放在文件末尾，以逐行 `# ` 注释形式提供；
4. **取消注释即可切换**：注释块内含启用步骤说明，不涉及跨块删除之外的额外改动；
5. **保留 ACME 目录**：阶段一的 80 块保留 `/.well-known/acme-challenge/`，便于后续 `certbot --webroot` 申请证书；
6. **安装脚本不覆盖已改配置**：`Install.sh` / `Upgrade.sh` 重新拷贝模板时不应覆盖用户已修改的 `/etc/nginx/...` 文件（当前只拷贝到 `<PREFIX>/Deploy/`，不直接写 `/etc`，行为正确，建议在文档中明确）。

## 建议的具体改动

| # | 位置 | 改动 |
|---|---|---|
| 1 | 文件头注释 | 增加「阶段一（HTTP/80，默认启用）/ 阶段二（HTTPS/443 + HTTP/2，注释）」说明与切换步骤 |
| 2 | 阶段一 `server` 块 | 新增：`listen 80 default_server;`、`listen [::]:80 default_server;`、`server_name _;`；把前端/API/健康检查/上传大小/超时/安全头全部放这里 |
| 3 | 原「HTTP → HTTPS 跳转」块 | 移入文件末尾【块 A】并**整段 `# ` 注释**，含 ACME 目录与 301 跳转 |
| 4 | 原「HTTPS 主站」块 | 移入文件末尾【块 B】并**整段 `# ` 注释**，含 `ssl_certificate(_key)`、`http2 on;`、TLS 版本/加密套件、HSTS（可选） |
| 5 | 注释块内 | 增加启用步骤：申请证书 → 取消注释 → 删除阶段一 → 改 `server_name`/证书路径 → `nginx -t && systemctl reload nginx` |
| 6 | HTTP/2 写法 | 注明 nginx ≥ 1.25.1 用 `http2 on;`，旧版本用 `listen 443 ssl http2;` |
| 7 | 相关文档 | `Install.md` §6、`README.md` 文件表中「80→443 跳转」的描述改为「阶段一 HTTP/80（默认）+ 阶段二 HTTPS（注释）」 |

## 验收标准

```bash
# 1) 模板本身可通过语法检查（无证书、无域名环境下）
sudo cp Deploy/Native/Nginx.conf.example /etc/nginx/sites-available/finvquant-test
# 仅把 listen 80 default_server 改为 listen 18080 避免冲突后：
sudo nginx -t -c /dev/stdin <<'EOF'
events {}
http { include /etc/nginx/sites-available/finvquant-test; }
EOF

# 2) 默认配置不含未注释的 443 / ssl_certificate / return 301 https
grep -vE '^\s*#' Deploy/Native/Nginx.conf.example | grep -cE 'listen\s+443|ssl_certificate|return 301 https'   # 期望 0

# 3) 默认配置包含 80 default_server 与 IP 直连能力
grep -c 'listen 80 default_server;' Deploy/Native/Nginx.conf.example          # 期望 ≥ 1
grep -c 'server_name _;'            Deploy/Native/Nginx.conf.example          # 期望 ≥ 1

# 4) 注释块内的 443 配置完整可启用（行数与关键指令齐全）
grep -E '^#\s+listen 443 ssl;'      Deploy/Native/Nginx.conf.example          # 期望命中
grep -E '^#\s+ssl_certificate\s'    Deploy/Native/Nginx.conf.example          # 期望命中
grep -E '^#\s+http2 on;'            Deploy/Native/Nginx.conf.example          # 期望命中

# 5) 端到端：按阶段一部署后 IP 可访问
curl -s -o /dev/null -w '%{http_code}\n' http://<服务器IP>/API/V1/Health/Ready   # 期望 200
```

## 兼容性

- 纯模板与文档改动，**不影响**已有按旧模板部署的站点（用户已落到 `/etc/nginx/` 的配置不会被覆盖）；
- 不改变 FinvQuant 监听地址（仍为 `127.0.0.1:16001` / `127.0.0.1:16002`）；
- 对 nginx 版本要求不变（1.18+ 均可用；`http2 on;` 仅出现在注释块中并附带旧版本写法说明）。

## 参考实现

本仓库已按上述方案调整：

- 模板：`Deploy/Native/Nginx.conf.example`
- 操作手册：`Docs/FinvQuant/Nginx-Deploy.md`
