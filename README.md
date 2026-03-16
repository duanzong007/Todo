# Todo MVP

基于 Go + PostgreSQL 的极简任务提醒系统 MVP。

当前版本聚焦单用户 Web 使用场景，围绕三类核心任务实现：

- `todo`：长期待办，一直显示，手动完成后消失
- `schedule`：某天发生的日程，只在当天显示，到天自动消失
- `ddl`：有截止日期的任务，每天显示，直到手动完成

## 已实现能力

- 统一输入框录入任务
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
  - 记录输入来源，支持 `manual_text / sms_paste / ics_import`
  - 保存原始内容、摘要、校验和、扩展 metadata
- `tasks`
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
- GIN 元数据索引
- 活跃任务的部分索引
- ICS `uid + scheduled_for` 去重索引
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
