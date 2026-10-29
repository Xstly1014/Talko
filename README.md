# Talko

校园图文经验与机会社区，重点建设风险治理、内容生命周期与分发。Go 后端，React / TypeScript 前端。社区与审核工作台作为独立服务启动，业务使用真实 MySQL 持久化。

## 当前本机访问地址

| 服务 | 地址 | 用途 |
| --- | --- | --- |
| 社区 Web | http://localhost:5173 | 游客浏览、注册、发布、评论与个人记录 |
| 审核 Web | http://localhost:5174 | 人员登录、领取审核、举报、申诉与审计 |
| 开发邮箱 | http://localhost:8025 | 查看发往本地 SMTP 的验证码 |
| 社区 API 健康检查 | http://localhost:8080/readyz | 数据库就绪状态 |
| 审核 API 健康检查 | http://localhost:8081/readyz | 数据库就绪状态 |

所有本机服务仅监听回环地址。本机开发审核管理员的账号信息保存在 .tmp/reviewer-account.txt（已被 Git 忽略），请在本机查看。该账号与社区账号独立，不提供审核人员公开注册。

## 启动与检查

需要 Go 1.26+、Node 22.12+（建议 Node 24）、MySQL 8.0+。

1. 执行两个前端的 npm ci：web/community、web/review。
2. 按 .env.example 配置环境变量。Go 不自动读取 .env。
3. 启动 MySQL 后运行 node scripts/local.cjs start。该命令执行迁移，按需后台启动五个进程并等待就绪；已就绪的实例会保留。
4. 执行 node scripts/local.cjs status 查看状态。日志与进程记录保存在 .tmp/，终端返回后服务继续运行。

本机 PATH 中的 Node 22.3 版本不足。可用已经安装的 Node 24 完整路径执行上述命令：C:/Users/Admin/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node.exe。

Docker 可用时执行 docker compose up -d，提供 MySQL 和 Mailpit；此时不要同时启动 cmd/devmail 占用邮件端口，可分别执行以下命令启动四个应用进程。当前机器 Docker 不可用，使用隔离 WSL MySQL 与 cmd/devmail；scripts/isolated-mysql.sh 是本机辅助脚本，不修改已有 MySQL 数据目录，也不属于生产部署方案。

分别前台启动应用：

    node scripts/dev.cjs api
    node scripts/dev.cjs web
    node scripts/dev.cjs review-api
    node scripts/dev.cjs review-web
    node scripts/dev.cjs mail

跨机器或 WSL 的标准 Go 启动方式：设置 MYSQL_DSN、APP_ORIGIN、SMTP_ADDR、SMTP_FROM、REVIEW_ORIGIN，执行 go run ./cmd/community migrate 后，分别运行 go run ./cmd/community、go run ./cmd/review；两个前端分别执行 npm run dev。

## 人员开通

审核人员支持 reviewer、risk、admin 三种角色。受信任运维人员执行 go run ./cmd/review -bootstrap-email operator@example.test -role reviewer，密码从标准输入读取至 EOF，须为 16–72 字节。通过安全的标准输入方式传入，不放进命令参数、仓库或聊天。

reviewer 处理内容；risk 增加举报与申诉；admin 增加操作审计。人员停用后现有会话立即失效。生产人员管理、MFA 与恢复机制仍是上线门禁。

cmd/moderate 继续保留为需要数据库权限的受信任本地维护工具，其 actor 是审计标签；日常审核应使用带身份与角色约束的独立工作台。不得向普通审核人员分发数据库凭证。

## 验证

    node scripts/test.cjs
    node scripts/go.cjs vet ./...
    cd web/community
    npm run build
    cd ../review
    npm run build

测试脚本默认使用独立 talko_test 数据库；也可设置 TEST_MYSQL_DSN。未配置该环境变量时直接 go test 会明确跳过数据库测试，不得将跳过视为验收通过。测试记录见 docs/verification/go-integration-review.txt。

## 规格与实施范围

- docs/spec/community：社区功能与交互规格。
- docs/spec/review：本轮独立审核服务规格与验收边界。
- docs/review-api.md：人员身份、任务与治理 API。
- docs/verification/review-acceptance.md：本轮验收证据。
- docs/IMPLEMENTATION.md：已实现范围、上线缺口与后续优先级。

本轮可在本机完成真实持久化的社区与人工审核闭环，尚未达到公网试运营门禁。
