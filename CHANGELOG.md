# Changelog

## [0.1.1] — 2026-05-18

### 前端 UI

- **Token 用量排行**：周期切换从下拉菜单改为按钮标签（今日/本周/本月/今年），默认打开本周。dashboard.html 和 admin.html 同步更新。
- **客户端配置指南**：dashboard 新增独立卡片，常态化展示 Linux/Mac 和 Windows 的 Claude Code 配置指南，含 shell 函数和 settings.json 模板，API Key 位置使用占位符。
- **switchPlatform 作用域隔离**：新增 `scope` 参数（`kpanel` / `gpanel`），解决 Key 创建弹窗和配置指南卡片中多组 platform-tab 的切换冲突。
- **用量统计标签作用域**：`switchStats` / `switchRanking` 选择器从全局 `.stats-tab` 改为 `#statsTabs` / `#rankTabs` 作用域限定，避免排行和统计两组标签互相干扰。

### 文档

- **CHANGELOG.md**：新建独立变更日志，将 CLAUDE.md 中"待优化一览"表格和"架构决策"章节移入，CLAUDE.md 精简为项目参考文档。
- **CLAUDE.md**：移除已修复问题清单，改为引用 CHANGELOG.md。

### 工程化

- **版本管理**：新增 `VERSION` 文件（当前 0.1.0）为唯一版本源，语义化版本号。Makefile 构建时通过 `-ldflags "-X main.Version=..."` 注入版本号。
- **Git 工作流**：会话末由用户决定提交时机，发布时创建 annotated tag 并推送。

## 2026-05-17

### Bug 修复（代码审查批次）

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| S1 | `main.go` | 启动日志打印管理员明文密码 | 改为仅打印用户名 |
| B1 | `store/store.go` `GetOverview()` | 4 个 Scan 错误全部丢弃，管理概览恒为 0 | 每个 Scan 均检查错误并返回 |
| B2 | `store/store.go` `GetKeyLastUsed()` | Scan 错误未检查，Key 使用数据静默丢失 | 非 ErrNoRows 错误写日志 |
| B3 | `store/store.go` `GetBreakdown()` | 多处 Scan 错误未检查，统计图表数据不可信 | 每处 Scan 均检查并返回错误 |
| P1 | `store/store.go` | `time.Parse` 错误被丢弃，非法 date 导致结果错乱 | 解析失败直接 return error |
| B4 | `store/store.go` `DeleteUser()` | 三步删除无事务 | 用 `db.Begin()` 包裹，失败自动 Rollback |
| B5 | `proxy/proxy.go` `extractUsage()` | 流式请求 input_tokens 永远为 0 | 改为从 `message_start` 读取，同时累加 cache tokens |
| P3 | `proxy/proxy.go` | HTTP 客户端超时 10 分钟 | 改为 5 分钟 |
| P4 | 全 handler | 无全局 panic recovery | 新增 `handler.RecoverMiddleware`，打印 stack trace |

### 安全加固

| # | 位置 | 问题 | 处理 |
|---|------|------|------|
| S2 | 全项目 | 无 CSRF 保护 | N/A — Bearer header 天然防 CSRF |
| S3 | `handler/auth_handler.go` | 登录无速率限制 | 新增 `handler/ratelimit.go`，IP 维度 10 次失败锁定 5 分钟 |
| S4 | `proxy/proxy.go` `writeProxyError()` | 错误消息字符串拼入 JSON | 改用 `json.NewEncoder` 序列化 |
| S5 | `config/config.go` | JWT Secret 无最小长度限制 | 加 `len < 32` 检查，启动时报错 |

### 代码质量

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| P2 | `main.go` | 日志文件无 `defer f.Close()` | 已加 `defer f.Close()` |
| A1 | `handler/key_handler.go` | `json.Decode` 错误被忽略 | 出错返回 400 |
| A2 | `handler/admin_handler.go` | 多 admin 场景互删 | N/A — 已有 role=admin 删除拦截 |

### 架构评估

- **多 API Key 分流**：评估后暂不实施。DeepSeek 限流为 account 级别，同账户多 key 共享配额，轮询无效。未来方向：多账户 key pool / 429 退避重试 / 令牌桶限流 / 多供应商 fallback。
