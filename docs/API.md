# 当前 API 与运行契约

路径统一为 /api/v1。HttpOnly SameSite=Strict cookie 会话，生产要求 Secure，写请求校验 Origin。错误格式：code、message、request_id。状态码：400 参数错误、401 未登录、404 不存在或不可见、409 状态或幂等冲突、429 限流、500 服务异常。

| 能力 | 接口 | 关键约束 |
|---|---|---|
| 身份 | POST /auth/email-code、/auth/register、/auth/login、/auth/logout | 邮箱验证码 10 分钟有效、60 秒重发；注册 password 为 12–72 字节 |
| 资料 | GET /me、/users/{id}；PATCH /me/profile | 修改 nickname、bio、avatar_id；公开资料不含邮箱 |
| 媒体 | POST /media；GET /media/{id} | 原始 JPEG/PNG 二进制，5 MB、1600 万像素限制，重编码；草稿媒体仅本人可见 |
| 草稿 | POST /posts；PATCH /posts/{id}/draft | title、body、category、type、media_ids、opportunity；编辑附 revision_id；创建需 Idempotency-Key |
| 生命周期 | POST /posts/{id}/submit、/withdraw；DELETE /posts/{id} | 提交附 revision_id；审核版本不可覆盖；删除/下架阻断公开读取 |
| 内容与 Feed | GET /posts/{id}、/feed、/me/posts | Feed 支持 q、category、type、author、following=1、saved=1、cursor、limit(1–50) |
| 评论 | GET/POST /posts/{id}/comments；DELETE /comments/{id} | POST body + Idempotency-Key；待审本人可见，通过后公开 |
| 互动 | PUT/DELETE /posts/{id}/reactions/{kind} | like/favorite；联合唯一键幂等 |
| 关系 | PUT/DELETE /users/{id}/follow | 禁止自关注；关注之后的新内容进入收件箱 |
| 举报申诉 | POST /posts/{id}/reports、/appeals；GET /me/reports、/me/appeals | 举报 reason；申诉 task_id + reason；本人读取 |
| 通知 | GET /notifications；POST /notifications/{id}/read | 当前用户所有权校验、事件去重 |

机会字段：publisher、city、compensation、settlement、expires_at（RFC3339）、contact；有效期在未来 3 个月内，公开查询过滤过期机会并返回 risk_disclosure。当前我的内容、评论、通知及举报/申诉最多 100 条，完整分页仍是上线前必做项。

审核命令通过 cmd/moderate 操作，当前没有对公网开放的审核 API。人工审核及举报处理、申诉复核均记录审计与结论。完整 OpenAPI、字段级权限和后续分页契约随下一批交付细化。
