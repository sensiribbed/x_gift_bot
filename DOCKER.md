# Docker 镜像部署（VPS 无需编译）

仓库：https://github.com/sensiribbed/x_gift_bot

镜像：`ghcr.io/sensiribbed/x_gift_bot:latest`，目标 Linux amd64 / arm64。只有两个架构都通过离线启动测试，Actions 才更新 latest。首次发布状态见仓库 Actions。

配置好域名、业务凭据后，启动命令就是：

```bash
docker compose up -d
```

这是 Compose v2 的官方写法；如果安装的兼容命令叫 docker-compose，可用 `docker-compose up -d`。不要使用旧 Python Compose v1。

## 1. 准备 VPS 和域名

推荐 Ubuntu 24.04 LTS，Docker Engine + Compose v2.20 以上。无需安装 Go、Node.js 或 Caddy。

- 域名 A 记录指向 VPS；只有 IPv6 可达才添加 AAAA。
- 云安全组开放 TCP 80/443；UDP 443 可选（HTTP/3）。保留 SSH 端口。
- 80/443 不能被其他 Web 服务占用。
- Cloudflare 设为 DNS only / 灰云。橙云下本配置会按 Cloudflare 节点 IP 限流。

已有 Docker 可跳过安装。Ubuntu 安装步骤（[官方文档](https://docs.docker.com/engine/install/ubuntu/)）：

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
sudo tee /etc/apt/sources.list.d/docker.sources >/dev/null <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
sudo docker compose version
```

后续命令需要 Docker 使用权限；无权限时 Docker 命令加 sudo，脚本使用 `sudo bash ...`。

## 2. 首次填写配置

```bash
git clone https://github.com/sensiribbed/x_gift_bot.git
cd x_gift_bot
cp .env.example .env
cp bootstrap.example.json bootstrap.json
nano .env
nano bootstrap.json
```

.env 至少修改：

```dotenv
XGIFT_DOMAIN=gift.你的真实域名
ACME_EMAIL=你的真实邮箱
```

域名不带 https://、端口、路径或末尾斜线。其他默认值：

| 变量 | 默认值 |
| --- | --- |
| XGIFT_IMAGE | ghcr.io/sensiribbed/x_gift_bot:latest |
| XGIFT_PAYMENTS_ENABLED | false，确认配置后改 true |
| TZ | Asia/Shanghai |
| COMPOSE_PROJECT_NAME | xgift，决定数据卷名称，不要随意改 |
| CADDY_IMAGE | caddy:2-alpine |

GO_IMAGE 和 NODE_IMAGE 仅用于可选本地构建，VPS 拉取镜像无需修改。

bootstrap.json 是首次导入凭据的文件。替换所有 CHANGE_ME：

- cookies.cookies：自己的 auth_token 和 ct0。
- card：真实卡号、月份 01–12、四位年份、CVC、姓名、账单邮箱、两字母账单国家代码。
- stripe-key：X 结账页面对应的 pk_live_ 公钥。
- proxy 默认直连；需要代理时填 sing-box outbounds。
- api-auth.Authorization：从 X 网页请求头填写以 Bearer 开头的 Authorization；UserAgent 已提供默认值。
- catalog 默认沿用上游。目录中商户、产品、币种、金额必须按实际情况核对。默认 BDT，3/6 个月分别为 30000/60000 最小货币单位，不会自动获取实时价格。

真实凭据不能上传 GitHub；Git 和镜像构建均排除 bootstrap.json。设置权限，让容器 UID 10001 可读：

```bash
sudo chown 10001:10001 bootstrap.json
sudo chmod 600 bootstrap.json
chmod 600 .env
```

域名、X Cookie、银行卡资料不能提供通用默认值，必须自行填写。

## 3. 一条命令运行

```bash
docker compose up -d
```

首次启动自动生成随机保管库密码和管理员密码，并加密导入配置。重启不会重新导入或覆盖已有数据。Caddy 自动申请和续期证书，并将 HTTP 跳转到 HTTPS。

```bash
docker compose ps
docker compose logs --tail=100 app caddy
docker compose exec app cat /data/admin-password
curl --fail https://你的域名/healthz
```

后台为 https://你的域名/admin，用户名 admin。密码只在自己的终端读取。up -d 返回不代表证书已签发，需等待 app 健康并确认公网 HTTPS。

支付默认关闭；核对配置后在 .env 设 XGIFT_PAYMENTS_ENABLED=true，再执行 `docker compose up -d`。开启时原项目会检查支付配置，外部真实可用性仍需核验。

初始化成功并备份后，可把宿主机 bootstrap.json 内容改成 `{}` 清除这份明文输入；保留文件和 UID 10001 可读权限，Compose 仍挂载它。已有 vault 不再读取它。后续修改凭据使用 CLI，不修改 bootstrap。

也可用原项目交互向导：将 bootstrap.json 设为 `{}`，执行 `bash deploy/docker/manage.sh init`。保留 /data/vault-password，站点配置选 y。Docker 使用根目录 .env，不使用向导生成的 /data/site.env。

## 4. 更新与同步原项目

普通镜像更新：

```bash
bash deploy/docker/manage.sh update
```

脚本先下载镜像，成功后停服备份，再同时重建 Caddy 和 app。下载失败不停服，备份失败会尝试恢复原有服务；请选择没有进行中订单的维护窗口。

```bash
bash deploy/docker/manage.sh backup
bash deploy/docker/manage.sh status
bash deploy/docker/manage.sh logs
bash deploy/docker/manage.sh stop
bash deploy/docker/manage.sh up
docker compose run --rm cli check
```

在自己的仓库工作目录同步上游：

```bash
# 已有 upstream 不要重复添加
git remote add upstream https://github.com/mizorewww/x_gift_bot.git
git fetch upstream
git merge upstream/main
# 检查冲突、配置、go.mod 对 Go 版本的要求，提交后推送
git push origin main
```

推送后 Actions 自动构建镜像，失败不会覆盖 latest。上游更改构建依赖、数据格式或接口时仍需检查适配；不要用 GitHub 的 Discard commits 同步，避免丢失 Docker 修改。

服务器获取部署文件变更：先 backup，再 `git pull --ff-only`，然后 update。可将 XGIFT_IMAGE 锁定为 `ghcr.io/sensiribbed/x_gift_bot:sha-完整提交SHA` 或已验证的 digest。

## 5. 备份与迁移

命名卷 app_data 包含数据库和两个密码；caddy_data 包含证书私钥，caddy_config 包含代理状态。不要执行 `docker compose down -v`，它会删除数据。

备份在 backups/时间戳/：volumes.tar.gz（停服一致性备份）、env（环境配置）、source.tar.gz（不含 bootstrap.json、Git 元数据或缓存）、revision.txt（提交号）。备份含解密密钥，需加密保存到服务器之外。

迁移到全新 VPS / 全新数据卷，安全复制备份到 /root/xgift-backup 后：

```bash
mkdir -p /opt/xgift
cd /opt/xgift
tar -xzf /root/xgift-backup/source.tar.gz
cp /root/xgift-backup/env .env
printf '{}\n' > bootstrap.json
sudo chown 10001:10001 bootstrap.json
sudo chmod 600 bootstrap.json
chmod 600 .env
docker compose pull app caddy
docker compose create app
docker compose run --rm --no-deps -T backup -C /snapshot -xzf - < /root/xgift-backup/volumes.tar.gz
docker compose up -d
```

如果要继续 Git 同步，可克隆仓库并检出 revision.txt 中的提交，代替解压源码。归档保留 UID 和 0600 权限，不要用 Windows 解压后逐个拷贝。切换前停掉旧 app 再修改 DNS，避免两个实例处理订单。

旧备份不包含之后产生的兑换状态，已有新订单时不能直接回滚生产库。必须先核对支付状态，在停机维护中恢复到新卷，不要把旧 SQLite 文件与新 WAL 混合。

## 6. 原理与排错

订单进入 review 时，可只读查看已加密保存的错误分类（不会访问 X/Stripe、创建订单或付款）：

```bash
docker compose pull cli
docker compose run --rm --no-deps -T --entrypoint xgift-diagnose cli 日志里的32位订单ID
```

`X_PRODUCT_OR_PRICE_LIST_MISMATCH` 表示 X 返回的商品 ID 或价格列表数量与程序预期不符；`X_PRICE_CURRENCY_OR_PAYMENT_TYPE_MISMATCH` 表示报价、币种或一次性支付类型不符合目录。不要靠猜测修改价格或跳过校验。诊断只打印错误分类，不输出原始响应、Cookie、卡片信息或支付链接。

- app 和 Caddy 共享网络，后端保留 127.0.0.1:8787 监听限制；公网只发布 80/443。Caddy 重写 X-Real-IP，管理 API 关闭。重建 Caddy 要同时重建 app，维护脚本已处理。
- app 使用 UID 10001、只读根文件系统，随机密码文件权限 0600，启动日志不输出密码。
- bootstrap.json is incomplete/invalid：替换占位值并检查 JSON、金额。
- permission denied：检查 bootstrap.json 的所有者 10001 和权限 600，不要 chmod 777。
- 镜像 denied/unauthorized：确认 Actions 首次构建完成、GHCR 包为 Public。GitHub 新包默认 Private，公开仓库不保证镜像可匿名拉取。参见 [GHCR 文档](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)。
- 502/unhealthy：查 app 日志；证书失败检查 DNS、AAAA、安全组、80/443 和 Caddy 日志。
- bootstrap 不会覆盖已有 vault。用 `docker compose run --rm -T cli put --name cookies` 等命令从标准输入更新，再重建 app。
- 部分初始化失败不会自动清空重来；先查日志、修复或恢复，不要删除生产数据试错。
- 本地开发构建：`docker compose -f compose.yaml -f compose.build.yaml build app`。VPS 使用的 compose.yaml 不包含 build。

CI 以合成凭据、无网络容器验证首次启动、healthz、密码权限及重启后密码不变，不发起真实支付。公网 HTTPS 和真实业务需在 VPS 验收。
