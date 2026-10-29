# 审核服务 API

开发地址 http://localhost:8081，Web http://localhost:5174。Web 通过同源 /review-api/v1 代理。生产需独立受保护域名与 HTTPS。

## 身份与错误

人员会话名 talko_review_session，HttpOnly、SameSite=Strict、8 小时；生产强制 Secure。社区会话不能用于审核服务。每次读取人员状态与角色，停用即时生效。写请求校验 Origin；JSON 禁止未知字段与多对象。错误为 message、request_id，响应同时提供 X-Request-ID。401 为会话无效，403 为权限或自复核限制，409 为业务状态冲突。

## 接口

| 方法与路径（前缀 /review-api/v1） | 权限 | 输入或行为 |
| --- | --- | --- |
| POST /auth/login | 公开 | email、password；每 IP 每分钟最多 10 次 |
| POST /auth/logout | 人员 | 删除当前会话 |
| GET /me | 人员 | id、email、role |
| GET /summary | 人员 | 待审、已处理、待举报、待申诉数量 |
| GET /tasks | 人员 | status=pending/decided/cancelled、kind=post/comment/profile、page |
| GET /tasks/{id} | 人员 | 具体不可变版本、图片、机会字段、评论关联正文、领取与已有决定 |
| POST /tasks/{id}/claim | 人员 | 领取/续期 15 分钟；其他人员有效领取时返回 409 |
| POST /tasks/{id}/decision | 人员 | decision=approve/reject、reason（2–400 字）、advertising（图文广告标识） |
| GET /media/{id} | 人员 | 待审核与留证图片，仅已认证人员可访问 |
| GET /reports | risk/admin | 图文与评论案件列表、page |
| GET /reports/{id}?kind=post/comment | risk/admin | 举报理由、原始证据、当前状态、已处理信息 |
| POST /reports/{id}/resolve | risk/admin | kind=post/comment、action=hide/dismiss、reason |
| GET /appeals | risk/admin | 原决定与处理结果、新任务编号、page |
| POST /appeals/{id}/resolve | risk/admin | action=uphold/reopen、reason；原审核员及作者不得复核 |
| GET /audit | admin | 决策与处置审计、page |

列表每页 40 条，返回 items、page、has_more；页码从 1 开始，非法页码返回 400。任务按提交时间升序，案件与审计按创建时间降序。任务搜索目前针对当前页，界面明确显示该范围。

## 社区侧新增接口

- PATCH /api/v1/me/profile：返回 202，生成 profile_changes 和任务，公开资料不直接变化；GET /api/v1/me 包含最新 profile_change。
- POST /api/v1/comments/{id}/reports：已公开评论可举报，同一人员同一评论已有待处理案件时返回 409。
- GET /api/v1/me/comment-reports：本人的评论举报及结果。
- GET /api/v1/me/review-decisions：本人的所有类型审核任务，可对驳回决定提交既有 /api/v1/appeals。
- /api/v1/me/reports、/me/appeals、/notifications 和 /posts/{id}/comments 支持 page，返回 has_more。

## 状态与一致性

审核领取以数据库互斥保护领取和决策校验，最终领域事务仍再次检查对象与版本。举报原始版本引用在提交举报的事务中保存，后续编辑不会替换证据。申诉重开在同一事务中校验旧版本有效性、重新进入 pending、创建新任务、保存复核记录与 Outbox；原任务结论保持不变。消息异步产生，最终公开读取以数据库当前业务状态为准。

迁移按文件编号顺序执行。MySQL DDL 不提供完整事务回滚；迁移采用可重复建表、条件索引变更与幂等数据校正，失败时需要查看日志并恢复后重跑，生产仍需备份与迁移演练。
