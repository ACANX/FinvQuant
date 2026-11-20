# FinvQuant Native 部署：Nginx 对外暴露操作手册（阶段一：HTTP/80）

> 所属：FinvQuant 部署文档 · 对应模板：`Deploy/Native/Nginx.conf.example`
> 适用：已用 `Deploy/Native/Install.sh` 完成 Native 安装，但**域名/SSL 证书尚未就绪**，
> 需要先用 `http://<服务器IP>` 把服务对外开放的场景。

## 1. 目标与阶段划分

| 阶段 | 条件 | 入口 | 模板状态 |
|---|---|---|---|
| **阶段一（本手册）** | 无域名、无证书 | `http://<服务器IP>`（80 端口） | 默认启用，开箱即用 |
| 阶段二 | 证书申请完成、443/HTTP2 可放通 | `https://<域名>`（443，HTTP/2） | 模板末尾**整段注释**，取消注释即可 |

> 443 端口与 HTTP/2 需要有效证书 + 安全组/防火墙放通；当前不具备条件，故模板中相关配置保持注释，**不要提前启用**（否则 80 会 301 到不可用的 443，导致服务打不开）。

## 2. 前置条件

1. FinvQuant 服务已运行：

   ```bash
   systemctl status finvquant --no-pager
   curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:16001/API/V1/Health/Ready   # 期望 200
   curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:16002/                       # 期望 200
   ```

2. 已安装 Nginx（Ubuntu/Debian）：

   ```bash
   sudo apt-get update && sudo apt-get install -y nginx
   ```

3. 安全组/防火墙已放通 **80/tcp**（16001/16002 保持只监听 `127.0.0.1`，不需要对外放通）。

## 3. 阶段一：部署 80 端口站点

### 3.1 安装配置

```bash
# 1) 拷贝模板（安装脚本已把模板放在 <PREFIX>/Deploy/Nginx.conf.example）
sudo cp /data/App/FinvQuant/Deploy/Nginx.conf.example /etc/nginx/sites-available/finvquant

# 2) 启用站点（Debian/Ubuntu 的 sites-enabled 机制）
sudo ln -sf /etc/nginx/sites-available/finvquant /etc/nginx/sites-enabled/finvquant

# 3) 移除 Ubuntu/Debian 自带站点：它也声明了 default_server，不移除会冲突
#    （报 "duplicate default server"，或 IP 访问落到 nginx 欢迎页）
sudo rm -f /etc/nginx/sites-enabled/default

# 4) 语法检查 + 重载（不是 restart，避免中断现有连接）
sudo nginx -t && sudo systemctl reload nginx
```

> **阶段一不需要改任何内容**：模板中 `server_name _;` + `listen 80 default_server;`
> 表示不依赖域名、接收所有请求，直接 IP 访问即可。

### 3.2 放通防火墙

```bash
# ufw（Ubuntu 默认）
sudo ufw allow 80/tcp
sudo ufw reload

# 若用 firewalld
sudo firewall-cmd --permanent --add-service=http && sudo firewall-cmd --reload

# 云服务器还需在控制台安全组放通 80/tcp
```

### 3.3 验证

```bash
# 本机
curl -I http://127.0.0.1/

# 从外部机器（把 IP 换成服务器公网/内网地址）
curl -I http://<服务器IP>/
curl -s -o /dev/null -w '%{http_code}\n' http://<服务器IP>/API/V1/Health/Ready   # 期望 200

# 查看访问日志
sudo tail -f /var/log/nginx/access.log
```

浏览器访问 `http://<服务器IP>` 应能看到 FinvQuant 前端登录页。

### 3.4 阶段一配置要点（模板中已包含）

| 配置 | 作用 |
|---|---|
| `listen 80 default_server;` / `listen [::]:80 default_server;` | 接收所有未匹配请求，IP 直连可用 |
| `server_name _;` | 不依赖域名，无需 DNS |
| `location /` → `127.0.0.1:16002` | 前端 SPA（静态资源已内嵌进二进制） |
| `location /API/` → `127.0.0.1:16001` | API 与前端同域，避免 CORS |
| `location = /healthz` | 可选外部探活，转发到 `API/V1/Health/Ready` |
| `client_max_body_size 512m` | 行情文件（MVSV）上传 |
| `proxy_read_timeout 300s` / `proxy_send_timeout 300s` | 大文件导入/导出不中断 |
| `location /.well-known/acme-challenge/` | 保留给阶段二申请 Let's Encrypt 证书使用 |

## 4. 常见问题

| 现象 | 排查 |
|---|---|
| `http://<IP>` 打不开 / 连接超时 | 云安全组是否放通 80；`sudo ss -lntp \| grep :80` 是否有 nginx 监听 |
| 502 Bad Gateway | `systemctl status finvquant`；`curl 127.0.0.1:16002` 是否 200；确认 `FINV_SERVER_HOST=127.0.0.1` |
| 页面能开但接口 404 | 访问路径应以 `/API/` 开头；确认 `location /API/` 未被其它站点配置覆盖 |
| 上传大文件 413 | 确认 `client_max_body_size 512m;` 在 `server` 块内 |
| 刷新子路由 404 | 模板 `location /` 反代到 16002 由后端处理 SPA 路由，不要改成静态 `root` |
| `nginx -t` 报 `duplicate default server` | Ubuntu/Debian 自带的 `/etc/nginx/sites-enabled/default` 已声明 `default_server`：`sudo rm -f /etc/nginx/sites-enabled/default` 后重新 `nginx -t` |
| `http://<IP>` 显示 nginx 欢迎页 | 同上：请求落到了自带 `default` 站点，移除该软链后 reload |
| 想改用 8080 端口 | 把阶段一里两处 `80` 改成 `8080`，安全组放通 8080，访问 `http://<IP>:8080` |

## 5. 阶段二：证书就绪后切换到 HTTPS/443 + HTTP/2

1. **申请证书**（示例：certbot，Webroot 方式复用 80 端口的 ACME 目录）：

   ```bash
   sudo apt-get install -y certbot
   sudo certbot certonly --webroot -w /var/www/html -d finvquant.example.com
   ```

2. **改模板**（`/etc/nginx/sites-available/finvquant`）：
   - 删除（或整段注释）「阶段一」的 `server { listen 80 default_server; ... }` 块；
   - 取消文件末尾【块 A】（80→443 跳转 + ACME）与【块 B】（443 ssl + http2）的注释；
   - `server_name finvquant.example.com;` 改成真实域名；
   - `ssl_certificate` / `ssl_certificate_key` 改成实际证书路径；
   - 若 nginx < 1.25.1：把【块 B】的 `http2 on;` 删掉，改为 `listen 443 ssl http2;`。

3. **放通 443**：

   ```bash
   sudo ufw allow 443/tcp && sudo ufw reload
   # 云控制台安全组同步放通 443/tcp
   ```

4. **校验并重载**：

   ```bash
   sudo nginx -t && sudo systemctl reload nginx
   curl -I https://finvquant.example.com/
   curl -s -o /dev/null -w '%{http_code} %{http_version}\n' https://finvquant.example.com/   # 期望 200 2
   ```

5. **（可选）启用 HSTS**：确认全站 HTTPS 正常后，取消【块 B】中 `Strict-Transport-Security` 的注释。

## 6. 回滚

```bash
# 回到阶段一（HTTP/80）
sudo rm -f /etc/nginx/sites-enabled/finvquant
sudo cp /data/App/FinvQuant/Deploy/Nginx.conf.example /etc/nginx/sites-available/finvquant
sudo ln -sf /etc/nginx/sites-available/finvquant /etc/nginx/sites-enabled/finvquant
sudo nginx -t && sudo systemctl reload nginx
```

## 7. 与 FinvQuant 服务的关系

- `finvquant` 由 systemd 管理，**只监听 `127.0.0.1:16001`（API）与 `127.0.0.1:16002`（前端）**，不对公网暴露；
- Nginx 是唯一对外入口（80 或 443），因此：
  - 升级/重启 FinvQuant 时 Nginx 无需变更；
  - 需要换端口/域名时只改 Nginx 配置；
  - 不要把 `FINV_SERVER_HOST` 改成 `0.0.0.0`，否则绕过 Nginx 直接暴露内部端口。
