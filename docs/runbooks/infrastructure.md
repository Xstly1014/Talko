# 部署、告警与恢复操作手册

## 当前运行边界

本机社区与审核 API 已加载本轮代码，开发环境继续使用内嵌 worker；容器环境禁用内嵌 worker，运行独立 worker。不可在同一部署中遗漏 worker。社区与审核前端独立构建和代理，只在各自服务内导航。

部署目录是单机试运行候选方案，尚未完成容器启动验收。绑定地址为 127.0.0.1，邮件使用 Mailpit，不是公网生产环境。没有 TLS、真实 SMTP、云账号或域名时，不得邀请用户连接公网服务。Docker Desktop 服务端当前无法启动，不能把 Compose 配置校验当成实际启动成功。

## Linux / WSL 容器启动

1. Docker Engine 与 Compose 可用，执行 bash scripts/check-runtime.sh。容量需要实测；本机运行成功不证明云主机容量足够。
2. 复制 deploy/example.env 到 deploy/.env；本机生成独立随机密码。当前 MySQL DSN 使用原始密码拼接，要求随机十六进制，避免特殊字符破坏 DSN。限制环境文件与备份读取权限。
3. 执行 docker compose -f deploy/compose.yaml config --quiet，禁止将含凭证的完整 config 输出上传到工单。
4. 执行 docker compose -f deploy/compose.yaml up -d --build。迁移为独立 job，应用依赖其成功；MySQL、Prometheus、Grafana、Alertmanager 使用持久卷。
5. 社区 http://localhost:5183；审核 http://localhost:5184；Mailpit http://localhost:8035；Prometheus http://localhost:9090；Grafana http://localhost:3000；Alertmanager http://localhost:9093。这些地址只有栈启动后才可用，与现有 5173 / 5174 开发服务隔离。
6. 审核账号通过受信任运维命令开通，不提供公开注册，不把密码写入镜像或参数。数据库与指标端口不发布到公网。

Nginx 本轮只有基本请求大小、代理超时与 SPA 路由配置。正式入口仍需 TLS、访问控制、真实客户端地址与可信代理边界；当前进程内限流在代理后可能按代理地址合并，不能当作公网风控完成。

## 飞书机器人

实现依据：[飞书自定义机器人说明](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot)。选用启用签名校验的机器人。密钥只在本机环境文件配置，不发到聊天或提交 Git。

- FEISHU_WEBHOOK：飞书机器人 webhook；FEISHU_SECRET：签名密钥。
- ALERT_TOKEN_FILE：Linux 主机上的绝对路径，文件包含随机 32 字节以上 token。Alertmanager 和适配器挂载同一 secret，适配器不公开主机端口。
- 在 shell 导出 FEISHU_ENABLED=true；使用 docker compose -f deploy/compose.yaml -f deploy/compose.alerts.yaml config --quiet，再用相同组合启动。脚本也会读取此开关。停用时必须重建 Alertmanager 配置，并显式停止旧 alertbridge。
- 基础配置的 local-only receiver 不发送外部通知，只供本机规则和故障观察；不能作为已接通飞书的依据。

适配器只转发告警名、服务、级别与摘要，不转发任意标签或业务正文。请求超时、HTTP 非 200、缺少 code、业务 code 非零均返回 502，由 Alertmanager 重试。使用分组与重复间隔抑制噪音，发送恢复通知。发送成功但响应丢失可能导致重复通知；此通道是至少一次投递，不能承诺恰好一次。Alertmanager 持久卷保存通知日志和静默；长时间整体停机的告警依赖 Prometheus 恢复后重新计算，不是永久事件档案。

本轮用 httptest 接收器验证触发、恢复、错误与取消，不代表真实飞书送达。真实接通后需停 worker 触发告警，再恢复并确认飞书恢复消息，两条都保留截图或消息时间证据。

## 指标与排障

/metrics 仅供内部采集。HTTP 标签为方法、注册路由模板和状态码，不含用户 ID、URL 查询或正文；导出请求数和耗时直方图。数据库错误时指标端点返回 503，不返回虚假的零积压。连接池指标只反映应用连接，不能替代数据库服务器监控。

- MetricsUnavailable：检查进程、数据库与采集网络；worker 独立采集用于发现其停止。
- OutboxDelayed：检查 worker 日志 event_id、DB 锁等待、退避事件，不可直接批量清空队列。
- DeadLetters：按事件检查失败原因，修复后单条审计重放。
- ReviewBacklog：核实审核排班、领取过期、业务提交量，再调整处理能力。
- APIErrorRate / APILatency：按服务和路由定位，联查 request_id、DB 连接等待；当前阈值是试运行初值，非已达成 SLO。

社区与审核进程导出的是同一数据库业务队列，跨服务查看时不要把业务积压相加。Grafana 已配置数据源及业务、请求、数据库连接池看板；只有实际启动并查到采样数据才算看板验收完成。当前没有集中日志查询、分布式追踪、宿主机/容器 exporter 或 MySQL 服务端 exporter，不能标为完整可观测平台。

## 死信重放

受信任运维人员配置 MYSQL_DSN，执行：

    go run ./cmd/outbox -id <事件ID> -actor <运维身份> -reason <不少于5字的处理理由>

仅未处理且已进入死信的事件允许重放；事务锁保护，重复操作拒绝。重置当前退避状态，保留 outbox_failures 历史并写 audit_log。actor 是维护审计标签，身份可信性由数据库访问权限保障；该命令不能作为普通审核人员接口。首次失败退避 2 秒，随后指数增加，第 8 次失败进入死信；整体数据库故障或事务被数据库整体回滚时，不消耗业务重试计数，按进程循环重试并触发采集不可用告警。

## 备份与恢复

    bash deploy/bin/backup.sh
    bash deploy/bin/restore-check.sh /绝对路径/talko-时间.sql.gz

脚本依赖 bash、gzip、sha256sum、mysql 容器与 Compose。备份含媒体 BLOB，使用事务一致快照、压缩完整性检查、SHA256 和临时文件原子改名。非事务表和并发 DDL 不在一致性保证范围内。备份目录默认 Git 忽略、权限收紧；同盘备份不能抵抗主机丢失。

恢复只新建 talko_restore_ 时间命名数据库，保留失败现场，校验表、关键记录数和部分孤儿关系。不自动覆盖源库，不自动删除恢复库；演练结束后由运维核实数据库名与证据后清理。额外执行登录、审核证据、图片读取、分发幂等与通知检查，才算业务恢复完成。

本轮另在原生 WSL MySQL 做真实备份与隔离恢复，规范化完整 SQL 一致，见 verification/native-restore.txt。该库媒体记录数为零，不能据此声称图片恢复已验证；Compose 脚本本身尚未执行。CI 验收脚本包含独立二进制 canary，待运行证明。首期暂定每日备份、保留 7 份与异机加密副本；需在云资源选定后落地定时任务、恢复时长测量与 RPO/RTO 验收，当前未自动启用。

## 发布与回滚

镜像发布参考：[GitHub 发布 Docker 镜像](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images)。手动运行 release-images 工作流，先通过 Go、前端和容器验收，再将六个镜像推送 GHCR，使用完整提交 SHA 标签及 provenance。工作流尚未在远端执行，不声明镜像已发布。依赖镜像目前固定版本标签，首次验证后应记录 digest；供应链扫描和依赖升级流程仍待补齐。

导出 IMAGE_PREFIX（如 ghcr.io/xstly1014/talko）和 RELEASE（完整提交 SHA），执行 bash deploy/bin/release.sh。脚本互斥发布、拉取镜像、等待数据库、已有业务库先备份、每次新容器迁移、重建应用、检查健康及基础匿名读/未授权登录响应，并保存版本和镜像 ID。任一失败即退出，无自动数据库降级。发布记录中的基础冒烟不替代完整登录发布审核评论验收。

应用回滚先评估旧版本与当前表结构兼容性，导出旧 RELEASE 与 SCHEMA_COMPATIBLE=yes，再执行 bash deploy/bin/rollback.sh。只更新应用且不执行依赖或迁移；保留当前数据库。不得用恢复旧备份充当自动回滚，以免覆盖新写入。破坏性迁移必须另行设计维护窗口与数据处理方案。尚无云主机，因此没有配置 SSH 自动部署或云发布审批环境。

## 验收入口

本地 go test 与 vet 的结果见 verification/infrastructure-tests.txt。bash deploy/bin/acceptance.sh 只运行在独立 staging 或 CI 栈：启动栈、用 promtool/amtool 校验、停止 worker 并验证规则进入 firing、恢复采集与告警、备份隔离恢复及 BLOB canary。该脚本会短暂停止 staging worker，不得误用于生产。配置文件校验通过不等于该演练通过；CI 文件存在不等于 CI 已通过。
