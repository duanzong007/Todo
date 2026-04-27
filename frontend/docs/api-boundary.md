# 前端迁移接口边界

## 目标

这份文档记录 Vue 前端逐步迁移时，第一批需要复用或补齐的后端接口。

第一阶段只验证读取能力，不改现有业务语义。

## 已可复用接口

### Dashboard Snapshot

```http
GET /dashboard/snapshot?date=YYYY-MM-DD
```

用途：

- 首页当前任务列表
- 当天已完成列表
- 空状态金句

认证：

- 依赖现有 session cookie

缓存：

- 后端已设置 `Cache-Control: no-store`

当前 Vue 验证页已经接入这个接口。

### Realtime Events

```http
GET /events
Accept: text/event-stream
```

用途：

- 多端任务状态同步
- 后续 Vue 页面可以继续复用同一套 SSE 通道

当前 Vue 验证页已经接入这个接口。

### 任务操作接口

```http
POST /tasks/manual
POST /tasks/parse-sms
POST /tasks/parse-sms/native
POST /tasks/parse-sms/native-paste
POST /tasks/{taskID}/rename
POST /tasks/{taskID}/complete
POST /tasks/{taskID}/restore
POST /tasks/{taskID}/postpone
POST /imports/ics
```

用途：

- 后续迁移首页任务区、添加面板、短信页时复用

注意：

- 当前这些接口偏表单提交模型
- Vue 迁移时可以先继续提交 `FormData`
- 后续如果需要更清晰的数据契约，再补 JSON API

### 管理页接口

```http
GET /me
POST /me/tasks/apply
```

用途：

- 当前管理页仍是服务端模板
- 迁移 `/me` 时需要评估是否补 JSON 查询接口

建议：

- 第二阶段迁移 `/me` 前，先补一个只读 JSON 管理接口
- 批量操作可以暂时继续复用 `POST /me/tasks/apply`

## 第一阶段结论

第一阶段可以不改后端业务层。

Vue 工程需要的最小接口是：

- `/dashboard/snapshot`
- `/events`

这两个接口已经足够验证：

- 登录态复用
- 后端数据读取
- 实时同步通道
- Vite 代理配置
