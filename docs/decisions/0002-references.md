# 技术依据

- Go 标准 HTTP 路由与请求路径参数：https://pkg.go.dev/net/http
- MySQL 事务队列锁与 SKIP LOCKED：https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html
- Vite 运行环境要求：https://vite.dev/guide/

当前选择标准库 HTTP，数据库锁定队列用于本地事务性投递；按实际规模演进，不以组件数量作为项目成熟度。
