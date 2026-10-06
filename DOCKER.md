# XGift Lite：Docker 部署

只提供 **检测赠送资格** 和 **手动付款链接**。保留上游的链接验证、排队、15 分钟付款窗口和缓存；没有兑换码、管理后台、银行卡配置或自动扣款任务。

- 镜像：`ghcr.io/sensiribbed/x_gift_bot:lite`
- 支持 Linux amd64 / arm64（包括你的 ARM VPS）。
- Caddy 自动申请、续期 HTTPS 证书。
- 上游源代码保留用于合并；镜像只运行 `xgift-lite`，配置工具是 `xgift-config`。
- 旧完整版 `:latest` 不随精简版更新，保留用于过渡。不要沿用旧 `.env` 的镜像地址。

## 首次部署

先安装 Docker Engine 和 Compose v2 插件，确认 `docker compose version` 正常。见 [Docker 官方安装文档](https://docs.docker.com/engine/install/ubuntu/)。

将域名 A 记录指向 VPS；仅在 IPv6 可正常访问时配置 AAAA。开放公网 TCP 80 和 TCP 443，UDP 443 可选。首次申请证书建议 DNS 直连服务器。

```bash
git clone https://github.com/sensiribbed/x_gift_bot.git
cd x_gift_bot
cp .env.example .env
cp bootstrap.example.json bootstrap.json
nano .env
nano bootstrap.json
```

`.env` 必填域名、邮箱；其他默认值可保留：

```dotenv
XGIFT_DOMAIN=你的域名
ACME_EMAIL=你的邮箱
XGIFT_IMAGE=ghcr.io/sensiribbed/x_gift_bot:lite
HTTP_PORT=80
```

`COMPOSE_PROJECT_NAME=xgift` 决定持久化卷名称，部署后不要随意更改。

`bootstrap.json` 填写：

| 字段 | 内容 |
| --- | --- |
| `cookies.cookies` | 当前 X 登录会话的 `auth_token`、`ct0` |
| `api-auth.Authorization` | X 请求使用的完整 `Bearer …` |
| `api-auth.UserAgent` | 与登录会话一致的浏览器 User-Agent |
| `stripe-key` | X 结账使用的 `pk_live_…` 公钥；用于验证付款链接，不能省略，不是你自己的 Stripe 账户公钥 |
| `catalog` | 真实商户、币种、产品和金额，须与当前 X 返回的套餐一致 |
| `proxy` | 默认 `direct`，无需添加之前失败的孟加拉代理 |

模板套餐为 3 个月 BDT 300、6 个月 BDT 600，金额使用最小单位（30000 / 60000）。这是可编辑示例，不保证你的账号、地区当前仍是此价格。此前的 `X_PRICE_CURRENCY_OR_PAYMENT_TYPE_MISMATCH` 仍应核对真实套餐，精简版不会跳过价格验证。

设置文件权限，然后一条命令启动：

```bash
sudo chown 10001:10001 bootstrap.json
sudo chmod 600 bootstrap.json
docker compose up -d
```

访问 `https://你的域名`。首次启动自动创建加密数据和随机密钥，无管理员密码，不用填写银行卡，也没有自动付款开关。生成链接后需在 Stripe 页面自行付款。

```bash
docker compose ps
docker compose logs --tail=100 app caddy
```

`app` 应为 healthy。健康检查只表示本地配置库和服务可用，不表示 X 登录会话或网络一定有效。

### 80 端口已占用

你的 VPS 之前出现过此情况：在 `.env` 设置 `HTTP_PORT=8080`，不用修改 Compose。公网 TCP 443 必须直接到达此 Caddy，以便通过 TLS-ALPN-01 申请证书。使用 **HTTPS 标准 443** 访问；`http://域名:8080` 会跳转 HTTPS。公网 80 上的其他服务不会自动为本站转发请求。如果 443 也被占用，需要让现有反向代理接管，不能只随意更改 HTTPS 端口。

## 从之前完整版升级（保留数据）

先在旧目录备份并停止服务：

```bash
cd ~/x_gift_bot
bash deploy/docker/manage.sh backup
docker compose stop app caddy
```

保留你改过的 Compose 文件，避免拉取时冲突：

```bash
cp compose.yaml compose.before-lite.yaml
git restore compose.yaml
git pull --ff-only
```

如果还有其他自定义修改，先备份并处理 Git 提示，不要强制覆盖。若手工创建过 `docker-compose.yml`，也将其改名留存；新版统一使用 `compose.yaml`。

编辑原 `.env`：镜像改为 `ghcr.io/sensiribbed/x_gift_bot:lite`，添加 `HTTP_PORT=8080`（你的端口情况），保留原域名、邮箱、`COMPOSE_PROJECT_NAME`。删除旧 `XGIFT_PAYMENTS_ENABLED` 配置。

```bash
docker compose pull app caddy
docker compose up -d --force-recreate
```

沿用原数据卷及密钥。已有 vault 时不再读取 bootstrap，不需要重新填写登录信息。原卡信息和兑换记录留在加密卷中但精简入口不会使用；旧订单不会自动恢复付款。若原 vault 保存了失败代理，按下一节改为 direct。不要执行 `docker compose down -v`，它会删除数据和证书。

## 修改登录信息、套餐、代理

`bootstrap.json` 只在空卷首次启动时导入。之后编辑文件不会自动覆盖现有配置；用以下命令更新需要的项（VPS 需安装 jq）：

```bash
docker compose stop app
jq '.cookies' bootstrap.json | docker compose run --rm --no-deps -T cli --name cookies put
jq '."api-auth"' bootstrap.json | docker compose run --rm --no-deps -T cli --name api-auth put
jq -r '."stripe-key"' bootstrap.json | docker compose run --rm --no-deps -T cli --name stripe-key put
jq '.catalog' bootstrap.json | docker compose run --rm --no-deps -T cli --name catalog put
printf '%s\n' '{"outbounds":[{"type":"direct","tag":"direct"}]}' | docker compose run --rm --no-deps -T cli --name proxy put
docker compose up -d
```

只执行需要修改的项。配置工具不输出密钥内容，也不提供付款命令。保留 bootstrap 文件作为 Compose secret 来源；内容及备份不要上传 GitHub。

## 以后同步原项目更新

在自己的 GitHub 仓库 **Actions → Sync upstream into Lite → Run workflow** 手动触发。它合并原项目 main，再触发构建。独立的 `RunLite`、页面入口、Docker CMD 保证上游新增页面、后台路由和自动任务不会直接出现在精简站点。

两种 CPU 都必须通过 Go 测试和离线容器测试才更新 `:lite`。测试检查禁用完整版路由、初始化、持久化和手动流程。若合并冲突或接口不兼容，工作流报错，需要适配后重试，不会发布失败镜像。请同时确认 **Sync** 和 **Build and publish Lite Docker image** 都成功。

构建成功后在 VPS 执行：

```bash
docker compose pull app
docker compose up -d
```

带备份更新：`bash deploy/docker/manage.sh update`。如果更新涉及 Compose 或部署脚本，还需 `git pull --ff-only`；Git 不会覆盖 `.env`、bootstrap 和数据卷。

固定版本可设置 `XGIFT_IMAGE=ghcr.io/sensiribbed/x_gift_bot:lite-sha-完整提交SHA`。移动标签不会在 VPS 自动拉取，需要上述 pull。

## 备份、迁移、恢复

```bash
bash deploy/docker/manage.sh backup
```

备份在 `backups/时间戳/`，包含停止服务后的数据卷、证书、`.env` 和源码归档。数据库和 `vault-password` 必须一起迁移。另行安全复制 `bootstrap.json`（故意不放入源码归档），整份备份保存到服务器外。

新 VPS 安装 Docker，将 `source.tar.gz` 解压到空项目目录，把备份 `env` 复制为 `.env`，恢复 bootstrap 文件（UID 10001、权限 600）。在项目目录执行：

```bash
docker compose run --rm --no-deps -T backup -C /snapshot -xzf - < /安全路径/volumes.tar.gz
docker compose up -d
```

修改 DNS 指向新服务器，并停掉旧站。此恢复流程面向空卷，不要覆盖运行中的数据库；已有部署先停服务、备份当前卷并确认恢复目标。

## 开发验证

```bash
npm ci
npm run check
node frontend/scripts/build-lite.mjs
go test ./internal/site ./internal/checkout ./cmd/xgift-config
docker compose -f compose.yaml -f compose.build.yaml build
```

Go 使用 Linux 文件锁，容器及 CI 为 Linux 测试环境。前端 mock 预览：`node frontend/scripts/preview-lite.mjs`，只使用合成数据，不连接 X 或 Stripe。
