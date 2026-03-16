# Todo MVP

基于 Go + PostgreSQL 的极简任务提醒系统 MVP。

当前版本已升级为多用户 Web 使用场景，围绕三类核心任务实现：

- `todo`：长期待办，一直显示，手动完成后消失
- `schedule`：某天发生的日程，只在当天显示，到天自动消失
- `ddl`：有截止日期的任务，每天显示，直到手动完成

## 已实现能力

- 统一输入框录入任务
- 多用户注册 / 登录 / 登出
- 新用户注册后需由 admin 审批，审批通过后才能登录
- 基于 Cookie 的会话管理
- 每个用户仅能访问自己的任务和导入数据
- 中文文本规则解析
- 快递短信解析为持久 Todo
- ICS 文件导入为 Schedule
- DDL / Todo 完成
- DDL / Schedule 延期
- Web Dashboard 展示 `今天 / DDL / Todo`
- PostgreSQL 迁移、约束、索引、事件日志

## 项目结构

```text
cmd/server            HTTP 启动入口
db/migrations         PostgreSQL schema 迁移
internal/config       配置加载
internal/database     迁移执行器
internal/domain       核心领域模型
internal/repository   数据访问层
internal/service      解析、导入、业务服务
internal/web          Web 处理器
web/templates         HTML 模板
web/static            CSS
```

## 数据库设计

核心表：

- `ingestion_sources`
  - 记录所属用户
  - 记录输入来源，支持 `manual_text / sms_paste / ics_import`
  - 保存原始内容、摘要、校验和、扩展 metadata
- `app_users`
  - 存储用户账号、显示名、密码哈希、角色、审批状态
- `user_sessions`
  - 存储登录会话、到期时间、最后活跃时间
- `tasks`
  - 所有任务归属到具体用户
  - 统一任务主表
  - 通过约束保证三类任务与日期字段的组合合法
  - `metadata JSONB` 用于承载解析结果、ICS 字段、后续扩展
- `task_events`
  - 记录创建、导入、完成、延期等事件
  - 方便后续做审计、统计、回放

已补齐：

- PostgreSQL enum
- 任务类型约束
- 状态与完成时间一致性约束
- `schedule` 不能被标记为完成，`todo` 不能被延期
- 任务 `source_id + user_id` 复合外键，保证任务和输入来源归属同一用户
- 在历史数据已回填完成时，将 `tasks.user_id / ingestion_sources.user_id` 提升为真正的 `NOT NULL`
- `metadata / payload` 强制为 JSON object
- 用户名格式、显示名长度、会话有效期等数据库级校验
- 首个账号自动成为已审批 admin，后续账号默认进入待审批队列
- GIN 元数据索引
- 活跃任务的部分索引
- 用户维度的活跃任务索引
- 用户维度的 ICS `uid + scheduled_for` 去重索引
- 用户维度的来源校验和索引、任务来源索引、事件时间索引
- `updated_at` 触发器

## 本地运行

### 方式一：直接运行

1. 启动 PostgreSQL。
2. 配置环境变量，参考 `.env.example`。
3. 运行：

```bash
go run ./cmd/server
```

访问：`http://localhost:8080`

首次使用时，先访问 `http://localhost:8080/register` 注册首个账号。首个账号会自动成为 admin 并立即可用；后续注册账号需要由 admin 在 `/admin/users` 审批后才能登录。

### 方式二：Docker Compose

```bash
docker compose up --build
```

访问：`http://localhost:8080`

## 文本解析规则

当前 MVP 已支持的典型输入：

- `买电池` -> `todo`
- `明天上课` -> `schedule`
- `下午签到` -> `schedule`
- `周五交作业` -> `ddl`
- `【菜鸟驿站】取件码 384923` -> `todo`

支持的日期表达以常见中文场景为主：

- `今天 / 明天 / 后天`
- `周一 ... 周日`
- `本周X / 下周X`
- `3月20号 / 2026-03-20 / 2026/03/20`

## 多用户认证

当前默认开启注册，相关环境变量：

- `SESSION_COOKIE_NAME`
- `SESSION_TTL_HOURS`
- `SESSION_SECURE_COOKIE`
- `ALLOW_REGISTRATION`

认证方式：

- 注册：`/register`
- 登录：`/login`
- 登出：`POST /logout`
- 用户审批：`GET /admin/users`、`POST /admin/users/{id}/approve`、`POST /admin/users/{id}/reject`
- 会话：HttpOnly Cookie + 数据库存储的哈希 token

旧版单用户模式遗留的无归属数据，会在首个注册账号创建时自动归属给该账号。

## ICS 导入说明

MVP 版支持：

- 单次事件
- `FREQ=DAILY`
- `FREQ=WEEKLY`
- `FREQ=MONTHLY`
- 常见 `BYDAY / COUNT / UNTIL / EXDATE`

导入时会按 `ICS_IMPORT_HORIZON_DAYS` 预展开未来日程，并以 `ics_uid + scheduled_for` 去重。

## 验证

已完成：

- `go test ./...`

## 下一步建议

- 增加 JSON API，给 CLI / Bot / App 复用
- 增加后台定时任务，处理导入刷新和旧日程归档
- 引入更完整的自然语言日期解析
- 增加任务详情、批量操作、筛选能力
- 增加管理员能力、邮箱验证、密码重置
