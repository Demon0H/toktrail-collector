# 当前部署

正式入口 https://tokenrank.demonh.site，DNS A 记录指向 60.205.3.96。

应用位于 NAT 主机 /opt/dh-tokenrank，服务 dh-tokenrank.service，以专用用户运行，监听内部 8081；数据库 /var/lib/dh-tokenrank/dh.sqlite。

公网机新增独立 Nginx 站点 /etc/nginx/sites-available/dh-tokenrank，证书 /etc/letsencrypt/live/tokenrank.demonh.site/。HTTP 80 跳转 HTTPS，独立 ACME webroot /var/www/dh-tokenrank-acme。已有 party-certbot-renew.timer 调用 certbot renew，现有 deploy hook 检查并重载 Nginx，新证书自动参与续期；未修改 www、miband 站点。

回源走 dh-tokenrank-tunnel.service 的专用 SSH 隧道：127.0.0.1:18081 → NAT 主机 127.0.0.1:8081，通过 hz-dx-1.dabaiyun.net:40020 连接。专用密钥仅允许转发到此目标，锁定服务端主机公钥，凭据不纳入仓库。应用只信任本机隧道传入的 X-Forwarded-For；Nginx 覆盖为真实访问者 IP。

IP 查询使用 HTTPS ipwho.is，内存缓存 15 分钟，不写入数据库，失败显示未知。最多 1000 条缓存，第三方额度或网络失败不影响接入。

8081→40001 保留为临时 HTTP 预览；正式入口始终使用 HTTPS。无需额外 40008 或 DNS TXT。Caddyfile 只是空白主机备选配置，勿覆盖现有 Nginx。

检查：应用机 systemctl status dh-tokenrank；公网机 systemctl status dh-tokenrank-tunnel nginx、nginx -t；curl -f https://tokenrank.demonh.site/api/health。备份 SQLite 前停止应用或使用在线备份工具。

GitHub OAuth 凭据仅保存于 NAT 主机 /etc/dh-tokenrank.env（root 0600），通过独立 systemd drop-in 加载，不进入仓库。回调 https://tokenrank.demonh.site/auth/github/callback；使用 state + PKCE，只读取公开身份。网站使用 Secure/HttpOnly/SameSite 会话 cookie 和 CSRF token，不持久化 GitHub access token。首页只展示真实榜单，/setup 先一句话再手动接入，均要求 GitHub 登录。登录后默认公开，可撤回。旧的 HTTP 40001 入口自动跳转正式 HTTPS 域名。
