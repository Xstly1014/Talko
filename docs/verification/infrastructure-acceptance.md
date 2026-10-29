# 基础设施实施与验收记录

日期：2026-10-29。本轮未上线公网。

| 项目 | 实际结果 | 证据与限制 |
| --- | --- | --- |
| 原有社区与审核 MySQL 业务回归 | 通过 | infrastructure-tests.txt，未跳过数据库测试 |
| 毒消息隔离、8 次失败死信、通知幂等 | 通过 | worker 集成测试 |
| 延迟下架、延迟发布、重复发布更新 | 通过 | 当前内容状态收敛的数据库回归 |
| 重放审计、重复重放拒绝、保留失败历史 | 通过 | worker 集成测试 |
| 指标 DB 故障返回 503、路由标签去 ID | 通过 | observability 测试 |
| 飞书触发/恢复、鉴权、业务拒绝、HTTP 错误、取消 | 本地契约测试通过 | httptest 接收器；真实飞书未接通 |
| Go vet 与全量编译 | 通过 | 本机执行 |
| 所有 Bash 脚本语法 | 通过 | Git Bash -n；未等同执行成功 |
| 基础 Compose 配置 | 通过 | Docker Compose config --quiet；未启动容器 |
| 新代码与迁移加载 | 通过 | 本机社区/审核已重载，schema version 5，两个 /metrics 返回 200 |
| 原生 MySQL 备份隔离恢复 | 通过 | native-restore.txt，完整规范化 SQL SHA256 一致；图片数为零 |
| Docker 服务端 | 未通过 | docker info 返回 Docker Desktop is unable to start；日志定位 WSL update required；用户已授权升级 |
| 容器栈、Grafana 实际看板、告警故障注入 | 未执行 | 受 Docker 服务端阻塞；CI 已定义，尚未跑过 |
| Compose 备份脚本与 BLOB canary | 未执行 | acceptance.sh 已定义；原生演练不替代脚本验证 |
| 镜像构建、GHCR 发布、云发布/回滚 | 未执行 | 工作流与脚本已写；不声明镜像存在或已部署 |
| 宿主机/容器/MySQL 服务端 exporter、集中日志、追踪 | 未实现 | 不属于本轮已交付可观测能力 |
| 自动备份、异机副本、恢复时长/RPO/RTO | 未完成 | 待云资源与任务调度落地 |

## 当前可访问服务

社区 http://localhost:5173；审核 http://localhost:5174；开发邮箱 http://localhost:8025。社区 API http://localhost:8080/readyz、审核 API http://localhost:8081/readyz 与两个 /metrics 已验证可用。

5173 / 5174 是现有开发服务。5183 / 5184、3000、9090、9093 是容器候选配置地址，目前不能按已启动对外交付。

## 上线门禁

修复容器运行环境后执行部署手册与容器验收；配置飞书并验证触发及恢复消息；补足图片恢复、异机备份、真实邮件、TLS/入口安全与权限管理。账户恢复、人员生命周期、数据治理与容量/安全测试仍沿用既有上线门禁，不能因基础设施文件已提交而解除。
