# Talko

面向校园的图文经验与机会社区。Go 后端和 React/TypeScript 社区前端，重点建设风控与内容生命周期/分发。首期包含审核后公开的评论。

## 开发环境

要求 Go 1.26+、Node 22.12+（本次使用 24.22.1）、MySQL 8.0+。建议在 WSL 配置开发工具；Docker 可用时先运行 `docker compose up -d`，本地邮件 UI 为 http://localhost:8025。

复制 .env.example 并在终端载入环境变量。Go 不自动读取 .env，不要将真实凭证提交仓库。

```sh
export MYSQL_DSN='talko:talko_local@tcp(127.0.0.1:3307)/talko?parseTime=true&multiStatements=true&charset=utf8mb4&loc=UTC'
export APP_ORIGIN='http://localhost:5173'
export SMTP_ADDR='127.0.0.1:1025'
export SMTP_FROM='noreply@talko.local'
go run ./cmd/community migrate
go run ./cmd/community
# 独立终端
cd web/community
npm ci
npm run dev
```

若 Docker 不可用，可使用自己的 MySQL。cmd/devmail 是仅监听本机的开发 SMTP 邮箱，与 Mailpit 二选一启动，不应用于生产：`go run ./cmd/devmail`。

Windows 当前机器可以使用 Node 24 执行 scripts/run.cjs run ./cmd/community migrate、scripts/dev.cjs api、scripts/dev.cjs web；scripts/go.cjs 将 Go 缓存放进项目 .tmp。scripts/isolated-mysql.sh 是本机 Docker 不可用时的专用 WSL 辅助脚本，默认仅本机监听，不属于生产部署方案。

## 人工审核

```sh
go run ./cmd/moderate -action list
go run ./cmd/moderate -action decide -id TASK_ID -actor reviewer@example.test -decision approve -reason '人工核验通过'
go run ./cmd/moderate -action reports
go run ./cmd/moderate -action resolve-report -id REPORT_ID -actor risk@example.test -decision hide -reason '违规内容下架'
go run ./cmd/moderate -action appeals
go run ./cmd/moderate -action resolve-appeal -id APPEAL_ID -actor risk@example.test -reason '复核结论与下一步说明'
```

审核 CLI 需要数据库权限，只供受信任开发/运维人员使用。actor 是审计标签；正式审核后台还需人员认证和角色控制。

## 验证

```sh
export TEST_MYSQL_DSN='talko:talko_local@tcp(127.0.0.1:3307)/talko_test?parseTime=true&multiStatements=true&charset=utf8mb4&loc=UTC'
go test -count=1 -v ./...
go vet ./...
cd web/community && npm run build
```

TEST_MYSQL_DSN 未设置时数据库测试明确跳过；不得把跳过视作业务链路通过。本次实际集成测试结果见 docs/verification/go-integration.txt。

## 文档

- docs/spec/community：校园社区规格与验收草案。
- docs/decisions：架构取舍及环境说明。
- docs/IMPLEMENTATION.md：已实现范围与真实试运营前的缺口。

这是持续开发的首个社区版本，当前尚未达到真实试运营上线条件。
