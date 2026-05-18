# MagicGateway

企业 DeepSeek API 网关 —— 透传代理 + 用户管理 + Token 用量统计。

## 背景

DeepSeek 提供 Anthropic 兼容端点 (`https://api.deepseek.com/anthropic`)，
Claude Code 可直接使用。但企业只有一个 API key，需要网关来：
1. 为每个员工分配独立的虚拟 API key
2. 透传请求到 DeepSeek，同时记录每人 token 用量
3. 提供 Web 管理界面，按日/周/月/年统计用量

## 技术栈

- **Go 1.25** + SQLite (modernc.org/sqlite, 纯 Go, CGO_ENABLED=0) — 单二进制部署
- **JWT** (golang-jwt) — 用户登录鉴权
- **bcrypt** — 密码哈希
- **Go embed** — 前端静态资源内嵌
- **交叉编译** — `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` Mac → Linux x86_64

## 项目结构

```
MagicGatewayApp/           # ★ 部署包，可直接复制到服务器
├── main.go                # 入口
├── config/config.go       # YAML 配置
├── proxy/proxy.go         # SSE 流式透传代理
├── auth/auth.go           # JWT + API key 验证
├── store/store.go         # SQLite CRUD
├── handler/               # HTTP handlers
│   ├── auth_handler.go    # 登录/注册
│   ├── key_handler.go     # Key 管理
│   ├── stats_handler.go   # 统计查询
│   └── admin_handler.go   # 管理员功能
├── web/                   # 前端页面（Go embed）
│   ├── login.html
│   ├── dashboard.html
│   └── admin.html
├── bench/                   # 延迟压测工具
│   ├── bench.go             #   单次请求计时程序
│   └── run.sh               #   A/B 对比脚本
├── deploy/
│   └── magicgateway.service  # systemd unit
├── config.yaml
└── Makefile
```

## 前端

当前：原生 HTML + CSS + JavaScript，无框架，Go embed 内嵌。
原因：3 个页面（登录/用户面板/管理面板）均为 CRUD 表格 + 表单，
原生 JS 足够，无需引入 npm + Vite 构建链。

后续迁移 Vue：当页面和交互复杂度增长时，将 `web/` 改为 Vue 3 + Vite 项目，
构建产物放在 `web/dist/`，Go embed 路径调整即可，API 层不变。

### 登录页 (`login.html`)

- 居中大标题 "MagicGateway" + 副标题 "企业 DeepSeek API 网关"
- 底部版权声明：版权归深圳美丽魔方机器人有限公司所有
- 背景：5 个 AI 公司真实 logo 水印斜向排列（OpenAI / Claude / DeepSeek / Gemini / Qwen）
  - SVG 路径来自 Wikipedia 和 simple-icons 官方仓库
  - 保留原始品牌颜色，透明度 14%，间距 140px
  - 720×720 循环平铺

### 用户面板 (`dashboard.html`)

卡片顺序：Token 用量排行 → 我的 API Keys → 我的用量统计

- 柱状图样式：`align-items:stretch` + `justify-content:flex-end`（**重要**：不能
  用 `flex-end`，否则柱状图百分比高度无法解析）
- 柱状图悬停：CSS 即时气泡显示 token 数量（`data-tip` 属性 + `::after` 伪元素）
- Token 格式化：<1万 原始数字，1万~1亿 X.XX万，>1亿 X亿XXXX万
- Key 创建后显示平台切换指南（**Linux / Mac** | **Windows** 两个标签）
  - Linux/Mac：shell 函数（添加到 `~/.zshrc` / `~/.bashrc`）+ 复制按钮
  - Windows：两种配置方式，方法一（推荐）PowerShell Profile 函数，方法二 settings.json，每步独立复制按钮
  - 两个模板均动态填入当前网关 URL 和新创建的 Key
- Header 右侧「修改密码」按钮

### 用户面板交互说明

- 「修改密码」使用页面内 Modal（非 `prompt()`），三个字段：当前密码 / 新密码 / 确认新密码，含实时校验（空值、长度、一致性）

### 管理面板 (`admin.html`)

四个标签页：概览 / 用户管理 / Key 管理 / 统计查询

- 用户管理：每行有「重置密码」按钮，管理员可为任意用户直接设置新密码
- Key 管理：可启用已吊销的 Key
- Key 创建后显示平台切换指南，同 dashboard（**Linux / Mac** | **Windows** 两个标签，含 PowerShell Profile 函数和 settings.json 两种方法）
- 「重置密码」和「删除用户」均使用页面内 Modal（非 `prompt()` / `confirm()`）；删除 Modal 显示用户名确认，操作按钮为红色危险样式

## 角色权限

| 功能 | 普通用户 | 管理员 |
|------|---------|--------|
| 注册 | ✓ | - |
| 申请/吊销 API key | ✓ (自己的) | ✓ (所有) |
| 查看用量 | ✓ (自己的) | ✓ (所有人的) |
| 修改密码 | ✓ (自己的) | ✓ (重置任意用户) |
| 管理用户 | ✗ | ✓ |

初始管理员：`admin` / `magic2026`

## 关键 API

- `POST /v1/messages` — Claude Code 代理端点（API key 鉴权）
- `POST /api/auth/login|register` — 登录/注册（JWT）
- `GET /api/keys`, `POST /api/keys`, `DELETE /api/keys/:id` — Key 管理
- `GET /api/stats/mine?period=daily|weekly|monthly|yearly` — 个人统计
- `PUT /api/auth/password` — 用户修改密码（需旧密码）
- `GET /api/admin/*` — 管理员接口
- `PUT /api/admin/users/:id/password` — 管理员重置用户密码

## 开发

Mac 上无需安装 Go，全部通过 Docker 容器完成：

```bash
# 开发模式（air 热重载，config.test.yaml）
make dev-watch

# 直接运行
make dev

# 交叉编译 Linux 二进制
make build
```

Makefile 关键变量：
- `GO_IMAGE ?= golang:1.25-alpine`
- `CONFIG ?= config.test.yaml`
- Go 编译缓存挂载到 `.go-cache/` 目录
- Air 配置在 `.air.toml`，监听 Go 文件变更自动重载
- 前端 HTML 变更需 `touch main.go` 触发重载（因 go:embed 不会监听 HTML）

### 调试技巧

- 浏览器硬刷新 Cmd+Shift+R 跳过缓存
- API 测试：`curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"magic2026"}'` 获取 JWT
- 数据库直查：`sqlite3 data/magicgateway.db "SELECT * FROM usage_logs ORDER BY id DESC LIMIT 10;"`
- Docker 内外文件同步有延迟，大改后建议重启容器
- 查看网关日志：每条代理请求自动输出 `[proxy] user=xx model=xx ttfb=xxms total=xxms in=xx out=xx`

### 延迟诊断

Proxy 层每笔请求自动记录 TTFB（Time To First Byte，网关→DeepSeek 首字节时间）：

```
[proxy] user=zhangsan model=deepseek-chat ttfb=320ms total=4521ms in=150 out=480
```

- `ttfb` — 网关到 DeepSeek 的首字节延迟（排队+推理启动）
- `total` — 上游完整耗时（ttfb + 生成时间）

Benchmark A/B 对比工具：

```bash
# 同机测试（消除网络变量，只测网关逻辑开销）
./bench/run.sh <deepseek-key> <proxy-virtual-key> http://localhost:8080 10

# 输出示例：
#   Average overhead: +89.0 ms (+2.0%)   →  正常，网关逻辑开销可忽略
#   Average overhead: +350.0 ms (+12.5%)  →  异常，检查网络/服务器负载
```

延迟构成：
- `< 5%` 网关逻辑开销可忽略，用户"慢感"来自客户端→网关的网络距离
- `5-15%` 正常代理开销，检查网关是否与用户同网段
- `> 15%` 异常，排查：网络 ping 值、服务器 CPU、DNS 解析

## 部署

Ubuntu 服务器不需要 Go 或 Docker，直接运行静态二进制：

```bash
scp build/gateway config.yaml user@server:/home/beautycube/magicgateway/
ssh user@server
cd /home/beautycube/magicgateway
./gateway                                    # 直接运行
sudo cp deploy/magicgateway.service /etc/systemd/system/  # 或 systemd 管理
```

敏感信息（`api_key`、`jwt_secret`）通过环境变量注入，建议写入 `~/.bashrc` 或 `~/.zshrc`：

```bash
echo 'export MAGIC_API_KEY=sk-xxx' >> ~/.bashrc
echo 'export JWT_SECRET=xxx' >> ~/.bashrc
source ~/.bashrc
./gateway
```

systemd 部署时创建 `env` 文件存放环境变量，service 通过 `EnvironmentFile=` 加载，详见 `deploy/magicgateway.service`。

详细方案见：[PLAN.md](PLAN.md)

## 历史变更

2026-05-17 代码审查批次修复（Scan 错误处理、事务、panic recovery、速率限制等）和 2026-05-18 前端 UI 改进，详见 [CHANGELOG.md](CHANGELOG.md)。
