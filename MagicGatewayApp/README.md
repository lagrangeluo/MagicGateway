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
# 1. 创建测试配置（绕过校验）
cat > config.test.yaml << 'EOF'
server:
  port: 8080
  jwt_secret: dev-secret-do-not-use-in-production
deepseek:
  base_url: https://api.deepseek.com/anthropic
  api_key: sk-test-key
database:
  path: ./data/magicgateway.db
admin:
  default_username: admin
  default_password: magic2026
EOF

# 2. 启动热重载
make dev-watch

# 3. 修改代码（.go / .html），保存后自动重启
# 4. 浏览器访问 http://localhost:8080/login 测试
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
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  magicgateway:latest

docker logs magicgateway
# 输出: MagicGateway starting on :8080
```

## 部署到 Linux 服务器

### 方式 A：Docker 部署

```bash
# Mac 上构建 Linux x86_64 镜像
docker build --platform linux/amd64 -t magicgateway:latest .

# 导出并复制到服务器
docker save magicgateway:latest | gzip > magicgateway.tar.gz
scp magicgateway.tar.gz config.yaml user@server:/home/beautycube/magicgateway/

# 服务器上导入并启动
ssh user@server
cd /home/beautycube/magicgateway
docker load < magicgateway.tar.gz
docker run -d --name magicgateway \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  magicgateway:latest
```

### 方式 B：二进制部署

```bash
# Mac 上交叉编译 Linux 二进制
make build-linux

# 复制到服务器直接运行
scp build/gateway config.yaml deploy/magicgateway.service user@server:/home/beautycube/magicgateway/
ssh user@server
cd /home/beautycube/magicgateway
vim config.yaml   # 填入 DeepSeek API key + 修改 JWT secret
./gateway

# 或注册 systemd 服务
sudo cp deploy/magicgateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now magicgateway
```

## 配置

```yaml
# config.yaml
server:
  port: 8080
  jwt_secret: <随机字符串>        # 务必修改

deepseek:
  base_url: https://api.deepseek.com/anthropic
  api_key: sk-<你的企业key>       # 务必修改

database:
  path: ./data/magicgateway.db

admin:
  default_username: admin
  default_password: magic2026      # 首次启动后建议修改
```

## 使用方式

1. 浏览器访问 `http://<服务器>:8080/login`
2. 管理员登录：admin / magic2026
3. 普通用户注册后登录，申请 API Key
4. 在 Claude Code 中配置：
   ```bash
   export ANTHROPIC_BASE_URL=http://<服务器>:8080
   export ANTHROPIC_API_KEY=sk-magic-<你的key>
   ```
