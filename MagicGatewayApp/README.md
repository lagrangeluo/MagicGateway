# MagicGateway

企业 DeepSeek API 网关 — 透传代理 + 用户管理 + Token 用量统计。

## 项目结构

```
MagicGatewayApp/
├── main.go              # 入口
├── config/config.go     # YAML 配置解析
├── proxy/proxy.go       # SSE 流式透传代理
├── auth/auth.go         # JWT 鉴权 + API key 验证
├── store/store.go       # SQLite CRUD
├── handler/             # HTTP handlers (auth/key/stats/admin)
├── web/                 # 前端页面 (login/dashboard/admin)
├── deploy/              # systemd unit
├── config.yaml          # 配置文件
├── Dockerfile           # 多阶段构建
├── Makefile
└── README.md
```

## 本地开发（Mac）

Mac 上无需安装 Go。代码通过 volume 挂载进 Docker 容器运行，修改代码后
即时生效。

```bash
# 手动模式：启动后修改代码，Ctrl+C 停掉再运行
make dev

# 热重载模式：修改任何 .go/.html 文件自动重新编译重启
make dev-watch
```

两种模式的区别：

| | make dev | make dev-watch |
|---|---|---|
| 原理 | `go run .` | air 文件监听 + 自动重编译 |
| 改代码后 | Ctrl+C 手动重启 | 自动检测、编译、重启 |
| 首次启动 | 2-3 秒 | 需要额外下载 air（仅首次），后续 2-3 秒 |

### 开发流程

```bash
# 1. 创建测试配置（config.test.yaml 已被 .gitignore 保护）
cat > config.test.yaml << 'EOF'
server:
  port: 8080
  jwt_secret: dev-secret-do-not-use-in-production
deepseek:
  base_url: https://api.deepseek.com/anthropic
  api_key: sk-your-deepseek-key
database:
  path: ./data/magicgateway.db
admin:
  default_username: admin
  default_password: magic2026
EOF

# 2. 将环境变量写入 shell 配置文件（敏感信息不落地到项目文件）
echo 'export MAGIC_API_KEY=sk-你的真实企业key' >> ~/.zshrc   # 或 ~/.bashrc
echo 'export JWT_SECRET=lagrangeluomagicgatewaysecretkey' >> ~/.zshrc
source ~/.zshrc

# 3. 启动热重载
make dev-watch

# 4. 修改代码（.go / .html），保存后自动重启
# 5. 浏览器访问 http://localhost:8080/login 测试
```

## Docker 构建（部署用）

开发完成后构建最终镜像：

```bash
# 构建 Linux x86_64 镜像
make build
```

### 运行容器

```bash
docker run -d --name magicgateway \
  -p 8080:8080 \
  -e MAGIC_API_KEY=sk-你的企业key \
  -e JWT_SECRET=你的JWT密钥 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  magicgateway:latest

docker logs magicgateway
# 输出: MagicGateway v0.1.1 starting on :8080
```

## 部署到 Linux 服务器

### 方式 A：Docker 部署

```bash
# Mac 上构建 Linux x86_64 镜像
docker build --platform linux/amd64 -t magicgateway:latest .

# 导出并复制到服务器
docker save magicgateway:latest | gzip > magicgateway.tar.gz
scp magicgateway.tar.gz config.yaml env.example user@server:/home/beautycube/magicgateway/

# 服务器上导入并启动
ssh user@server
cd /home/beautycube/magicgateway
docker load < magicgateway.tar.gz
docker run -d --name magicgateway \
  -p 8080:8080 \
  -e MAGIC_API_KEY=sk-你的企业key \
  -e JWT_SECRET=你的JWT密钥 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  magicgateway:latest
```

### 方式 B：二进制部署

```bash
# Mac 上交叉编译 Linux 二进制
make build-linux

# 复制到服务器
scp build/gateway config.yaml deploy/magicgateway.service env.example user@server:/home/beautycube/magicgateway/
ssh user@server
cd /home/beautycube/magicgateway

# 从模板创建环境文件，填入真实值
cp env.example env && vim env

# 直接运行
source env && ./gateway

# 或注册 systemd 服务（env 文件路径已写在 service 中，无需编辑 service）
sudo cp deploy/magicgateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now magicgateway
```

## 配置

`config.yaml` 保留非敏感配置（端口、数据库路径等）。敏感值通过环境变量注入，优先级高于配置文件：

| 环境变量 | 对应配置项 | 必填 |
|----------|-----------|------|
| `MAGIC_API_KEY` | `deepseek.api_key` | 是 |
| `JWT_SECRET` | `server.jwt_secret` | 是 |

```yaml
# config.yaml（可安全托管到 git）
server:
  port: 8080
  jwt_secret: change-me-to-a-random-string-at-least-32-chars

deepseek:
  base_url: https://api.deepseek.com/anthropic
  api_key: sk-your-deepseek-key

database:
  path: ./data/magicgateway.db

admin:
  default_username: admin
  default_password: magic2026
```

## 使用方式

1. 浏览器访问 `http://<服务器>:8080/login`
2. 管理员登录：admin / magic2026
3. 普通用户注册后登录，申请 API Key
4. 在 Claude Code 中配置：
   ```bash
   export ANTHROPIC_BASE_URL=http://<服务器>:8080
   export ANTHROPIC_AUTH_TOKEN=sk-magic-<你的key>
   ```
