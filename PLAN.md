# MagicGateway - 企业 DeepSeek API 网关

## Context

公司申请了企业 DeepSeek 账号，目标是让公司每位员工都能用上企业 DeepSeek
（替代 Claude Code 默认的 Anthropic 后端），并统计每人日/周/月/年 token 用量。

### 决策过程

1. **初始方案：LiteLLM 网关** → 以为需要 Anthropic → OpenAI 协议转换
2. **关键发现：DeepSeek 已有 Anthropic 端点** → 实测 `https://api.deepseek.com/anthropic` 可直接用
3. **最终方案：Go 透传代理 + 完整 Web 管理系统**

核心流程：管理员创建用户 → 用户登录后可申请多个 API key → 在 Claude Code
中配置使用 → 网关透传请求到 DeepSeek，同时记录每个 key 的 token 用量。

### 约束

- 规模：<20 人小团队
- 开发机：Mac (ARM)
- 部署机：x86_64 Linux + 双 4090 GPU 服务器
- 部署方式：Mac 交叉编译 Linux 二进制，直接 `./gateway` 运行
- DeepSeek 端点：Anthropic 兼容（无需协议转换）
- 技术栈：Go + SQLite + 内嵌 Web 前端

## 项目目录结构

整个项目在 `MagicGatewayApp/` 文件夹内，可直接打包复制到服务器部署：

```
MagicGateway/                          # 工作区根目录
├── CLAUDE.md                          # 开发文档（不部署）
├── PLAN.md                            # 本文件（不部署）
│
└── MagicGatewayApp/                   # ★ 部署包，可直接复制
    ├── main.go                        # 入口
    ├── go.mod / go.sum
    ├── config/
    │   └── config.go                  # YAML 配置解析
    ├── proxy/
    │   └── proxy.go                   # SSE 流式透传代理
    ├── auth/
    │   └── auth.go                    # JWT 鉴权 + API key 验证
    ├── store/
    │   └── store.go                   # SQLite（用户/Key/用量 CRUD）
    ├── handler/
    │   ├── auth_handler.go            # 登录/注册接口
    │   ├── key_handler.go             # API key 管理接口
    │   ├── stats_handler.go           # 统计接口
    │   └── admin_handler.go           # 管理员接口
    ├── web/
    │   ├── login.html                 # 登录/注册页
    │   ├── dashboard.html             # 普通用户面板
    │   └── admin.html                 # 管理员面板
    ├── deploy/
    │   └── magicgateway.service       # systemd unit 文件
    ├── config.yaml                    # 配置文件模板
    ├── Makefile
    └── README.md                      # 部署说明
```

## 架构

```
员工 Claude Code               MagicGateway (Go :8080)            DeepSeek API
┌──────────────────┐    ┌──────────────────────────────────┐    ┌──────────────────┐
│ ANTHROPIC_BASE_  │    │                                  │    │ api.deepseek.com │
│ URL=gateway:8080 │──▶│ /v1/messages (API key 鉴权)       │──▶│ /anthropic/v1/   │
│ ANTHROPIC_API_   │    │   → 验证虚拟 key → 替换企业 key   │    │ messages         │
│ KEY=sk-xxx       │    │   → 透传请求                      │    │                  │
└──────────────────┘    │   → 流式转发响应                  │    └──────────────────┘
                        │   → 提取 usage 写入 SQLite        │
                        │                                  │
      浏览器             │  Web 管理界面                    │
┌──────────────────┐    │  /login      → 登录/注册          │
│ 管理员/用户      │──▶│  /dashboard  → 用户面板            │
│ 登录管理        │    │  /admin      → 管理员面板          │
└──────────────────┘    └──────────────────────────────────┘
```

## Web 系统设计

### 角色与权限

| 功能 | 普通用户 | 管理员 |
|------|---------|--------|
| 注册账号 | ✓ | - |
| 登录 | ✓ | ✓ |
| 申请 API key（可多个） | ✓ | ✓ |
| 查看自己的 key | ✓ | ✓ |
| 吊销自己的 key | ✓ | ✓ |
| 查看自己的用量统计 | ✓ | ✓ |
| 管理所有用户 | ✗ | ✓ |
| 管理所有 key | ✗ | ✓ |
| 查看所有人的用量统计 | ✗ | ✓ |

### 初始账号

- 管理员：`admin` / `magic2026`（首次启动自动创建）
- 普通用户：通过注册页面自行注册，无需审批

### 页面规划

**1. 登录页 `/login`**
- 登录表单（用户名 + 密码）
- 注册表单（用户名 + 密码 + 确认密码）
- 登录成功后根据角色跳转到对应面板

**2. 用户面板 `/dashboard`**
- 我的 API Keys 列表（key 前缀、创建时间、状态、最近用量）
- 申请新 Key 按钮
- 吊销 Key 按钮
- 我的用量统计：日/周/月/年切换，表格展示

**3. 管理员面板 `/admin`**
- 概览卡片：总用户数、总 token 消耗（今日/本月）、活跃 key 数
- 用户管理 tab：用户列表、删除用户
- Key 管理 tab：所有 key 列表、按用户筛选、吊销 key、为用户创建 key
- 统计 tab：按用户查看 token 消耗，日/周/月/年切换，表格 + 图表

### 前端技术选型

- 纯 HTML + CSS + 原生 JS，不依赖前端框架
- 原因：当前 3 个页面（登录、用户面板、管理面板）均为表格 + 表单的 CRUD 场景，
  原生 JS 足够胜任，引入 Vue/React 会增加 npm + Vite 构建链，无实际收益
- 图表用 CSS 进度条或内嵌 SVG，不依赖 CDN（内网可用）
- 所有静态资源通过 Go embed 编译进二进制，部署时只有一个文件
- **后续扩展预留**：当管理页面数量增长、UI 交互变得复杂时，可迁移到 Vue 3 + Vite，
  只需将 `web/` 目录改为 Vue 项目，构建产物输出到 `web/dist/`，Go embed 路径对应调整即可

## 数据库设计

```sql
-- 用户表
CREATE TABLE users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,           -- bcrypt
    role            TEXT NOT NULL DEFAULT 'user',  -- 'admin' | 'user'
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- API Keys 表
CREATE TABLE api_keys (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL,
    key_hash    TEXT UNIQUE NOT NULL,         -- SHA256(key)
    key_prefix  TEXT NOT NULL,                -- sk-xxxx 前8位，方便识别
    key_name    TEXT DEFAULT '',              -- 用户自定义名称
    is_active   BOOLEAN DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- 用量日志表
CREATE TABLE usage_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    api_key_id      INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    user_name       TEXT NOT NULL,
    model           TEXT,
    input_tokens    INTEGER DEFAULT 0,
    output_tokens   INTEGER DEFAULT 0,
    request_id      TEXT,
    duration_ms     INTEGER,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_usage_created ON usage_logs(created_at);
CREATE INDEX idx_usage_user   ON usage_logs(user_id);
CREATE INDEX idx_usage_key    ON usage_logs(api_key_id);
```

## API 设计

### 公开接口（无需鉴权）

```
POST /api/auth/login         # 登录，返回 JWT token
POST /api/auth/register      # 注册普通用户
```

### 用户接口（需要 JWT，role=user 或 admin）

```
GET    /api/keys               # 我申请的 key 列表
POST   /api/keys               # 申请新 key { "key_name": "可选名称" }
DELETE /api/keys/:id           # 吊销我的某个 key
GET    /api/stats/mine?period=daily|weekly|monthly|yearly&date=2026-05-15
                               # 我的用量统计
```

### 管理员接口（需要 JWT，role=admin）

```
GET    /api/admin/users                 # 所有用户列表
DELETE /api/admin/users/:id             # 删除用户
GET    /api/admin/keys                  # 所有 key 列表（支持 ?user_id= 筛选）
POST   /api/admin/keys                  # 为指定用户创建 key { "user_id": 1, "key_name": "" }
DELETE /api/admin/keys/:id              # 吊销任意 key
GET    /api/admin/stats/:user_id?period=daily&date=2026-05-15
                                        # 查看指定用户用量
GET    /api/admin/stats/overview        # 全局概览 { total_users, today_tokens, month_tokens }
```

### 代理接口（API key 鉴权，非 JWT）

```
POST /v1/messages              # Anthropic Messages API 代理
                               # Header: Authorization: Bearer sk-xxx
                               # 或: x-api-key: sk-xxx
```

## API key 格式

虚拟 key 格式：`sk-magic-{16位随机hex}`

- 生成时产生完整 key，**仅在创建时返回一次明文**，后续无法查看
- 数据库只存 SHA256 哈希 + 前8位前缀（如 `sk-magic-a1b2c3d4`）
- 用户忘记 key 只能吊销后重新申请

## 代理核心流程

`POST /v1/messages` 处理逻辑：

1. 从 Header 提取 API key → SHA256 → 查 `api_keys` 表
2. Key 不存在或已吊销 → 返回 401
3. Key 有效 → 获取 user_id, user_name
4. 记录请求开始时间
5. 创建到 `https://api.deepseek.com/anthropic/v1/messages` 的请求
6. 替换 Authorization header 为企业 key
7. **流式转发**：读取 DeepSeek 响应 → 逐块写回客户端
8. 从 SSE 流的最后一个 event 中提取 `usage` 数据
9. 写入 `usage_logs` 表
10. 如果是非流式请求，直接从响应 body 提取 usage

## 本地开发（Mac，无需安装 Go）

全部通过 Docker 容器完成，不污染 Mac 环境：

```bash
cd MagicGatewayApp

# 开发模式：代码挂载进容器，go run 直接跑（修改代码后重启容器即可）
docker run --rm -it -p 8080:8080 \
  -v $(pwd):/app -w /app \
  golang:1.25-alpine sh -c "go run ."

# 编译 Linux 二进制
docker run --rm -v $(pwd):/app -w /app \
  golang:1.25-alpine sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o build/gateway ."
```

- Go 工具链只存在于 Docker 镜像内，容器停止后 Mac 上不留任何痕迹
- 代码在 Mac 上编辑，运行/编译在容器内完成
- 容器内是 Linux 环境，与最终部署的 Ubuntu 服务器一致

## 部署方案（Ubuntu 22.04 服务器）

Go 静态编译的二进制不依赖任何系统库，不需要在服务器上安装 Go 或 Docker。

### 编译 + 打包（在 Mac 上完成）

```bash
cd MagicGatewayApp

# 编译 + 打包（在 Docker 容器内交叉编译）
make build-linux package
# → build/magicgateway.tar.gz 包含：gateway + config.yaml + systemd unit
```

Makefile 封装这些命令：

```makefile
dev:          # Docker 开发模式（localhost:8080）
build-linux:  # Docker 内交叉编译 Linux 二进制
package:      # 打包 tar.gz
```

### 部署到服务器（Ubuntu 22.04）

```bash
# 1. 复制打包文件到服务器
scp build/magicgateway.tar.gz user@server:/home/beautycube/

# 2. 在服务器上解压
ssh user@server
mkdir -p /home/beautycube/magicgateway && cd /home/beautycube/magicgateway
tar xzf ../magicgateway.tar.gz

# 3. 修改配置（填入企业 DeepSeek key 和随机 JWT secret）
vim config.yaml

# 4. 直接启动（不需要 Docker、Go 或任何运行时）
./gateway
# 监听 :8080，SQLite 自动创建在 ./data/ 目录
```

### systemd 服务（推荐用于生产）

```ini
# deploy/magicgateway.service
[Unit]
Description=MagicGateway - DeepSeek API Gateway
After=network.target

[Service]
Type=simple
WorkingDirectory=/home/beautycube/magicgateway
ExecStart=/home/beautycube/magicgateway/gateway
Restart=always
RestartSec=5
Environment=TZ=Asia/Shanghai

# 安全加固
NoNewPrivileges=yes
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

安装服务：

```bash
sudo cp deploy/magicgateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now magicgateway
sudo systemctl status magicgateway
```

### 更新流程

```bash
# Mac 上重新编译 + 打包
make build-linux package

# 复制到服务器
scp build/magicgateway.tar.gz user@server:/home/beautycube/

# 服务器上更新
ssh user@server
cd /home/beautycube/magicgateway
sudo systemctl stop magicgateway
tar xzf ../magicgateway.tar.gz
sudo systemctl start magicgateway
```

### 目录结构（服务器上）

```
/home/beautycube/magicgateway/
├── gateway             # 二进制
├── config.yaml         # 配置文件
├── deploy/
│   └── magicgateway.service
└── data/
    └── magicgateway.db # SQLite 数据库（自动创建）
```

## 配置文件

```yaml
# config.yaml
server:
  port: 8080
  jwt_secret: change-me-to-a-random-string  # JWT 签名密钥

deepseek:
  base_url: https://api.deepseek.com/anthropic
  api_key: sk-your-enterprise-key

database:
  path: ./data/magicgateway.db

admin:
  default_username: admin
  default_password: magic2026  # 首次启动自动创建，启动后建议修改
```

## 实现步骤

### 步骤 1：项目骨架
- 创建 `MagicGatewayApp/` 目录结构
- 初始化 Go module
- 编写 Makefile（build / build-linux / package）

### 步骤 2：config + store 包
- YAML 配置解析
- SQLite schema 自动迁移
- 用户/Key/用量 的 CRUD 操作
- 初始 admin 账号创建逻辑

### 步骤 3：auth 包
- bcrypt 密码哈希/验证
- JWT 生成/验证
- API key 生成（随机串 + SHA256）
- 中间件：JWT 鉴权、Admin 角色检查

### 步骤 4：handler 层
- 登录/注册接口
- Key 管理接口（用户 + 管理员）
- 统计接口（日/周/月/年聚合查询）
- 管理员接口（用户管理）

### 步骤 5：proxy 包
- SSE 流式透传核心
- API key 鉴权
- usage 提取和写入

### 步骤 6：Web 前端
- login.html（登录 + 注册）
- dashboard.html（用户面板 + 个人统计）
- admin.html（管理面板 + 全局统计）

### 步骤 7：main.go 组装
- 路由注册
- 中间件链
- 内嵌静态资源
- 初始数据创建

### 步骤 8：部署配置
- systemd unit 文件 `deploy/magicgateway.service`
- README.md 部署说明
- Makefile `package` 目标（打包 tar.gz）

## 技术要点与风险

1. **SSE 流式代理**：逐块转发，从 message_stop event 提取 usage
2. **JWT 安全性**：生产环境务必修改 jwt_secret
3. **API key 明文仅展示一次**：创建时返回，之后只显示前缀
4. **SQLite 并发**：<20 人 + WAL 模式完全够用
5. **跨平台构建**：Mac ARM → Linux x86_64 通过 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` 交叉编译
6. **密码安全**：bcrypt 哈希，初始密码建议首次登录后修改

## 验证方法

1. `go build` 在 Mac 上编译通过，`make build-linux` 交叉编译成功
2. 启动后浏览器访问 `http://localhost:8080/login`，注册用户、申请 key
3. 用 curl 模拟 Claude Code 请求，验证代理 + 统计
4. 管理员面板查看用量统计，切换日/周/月/年
5. 在 Ubuntu 22.04 服务器上通过 systemd 部署，验证开机自启
