# 前端迁移接口边界

## 目标

这份文档记录 Vue 前端逐步迁移时，需要复用或补齐的后端接口。

原则：Vue 负责页面状态和交互，业务规则仍然在 Go 后端。

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

### 短信导入页接口

```http
GET /sms/native
GET /sms/native/classic
GET /sms/native/data
POST /tasks/parse-sms/native
POST /tasks/parse-sms/native-paste
```

用途：

- `/sms/native` 当前是 Vue 短信导入页入口
- `/sms/native/classic` 是旧 Go 模板回退页
- `/sms/native/data` 给 Vue 读取当前用户、返回路径和时区
- `/tasks/parse-sms/native` 复用后端短信识别逻辑导入壳层读取的短信
- `/tasks/parse-sms/native-paste` 复用后端短信识别逻辑导入手动粘贴短信

本地状态：

- 新短信缓存继续使用 `todo-native-sms-current-v1:{userID}`
- 历史记录继续使用 `todo-native-sms-history-v1:{userID}`
- Vue 迁移不改变已有历史记录兼容性

### 管理页接口

```http
GET /me
GET /me/data
POST /me/tasks/apply
```

用途：

- `/me` 当前是 Vue 管理页入口
- `/me/classic` 是 Go 模板回退页
- `/me/data` 给 Vue 管理页读取任务、筛选器、分页和共享用户
- `/me/tasks/apply` 给 Vue 管理页异步提交编辑、共享和删除

提交方式：

- 继续使用 `FormData`
- 请求头带 `X-Requested-With: fetch`
- 成功返回 JSON 消息
- 失败返回 JSON 错误
- 旧模板表单提交仍然保持重定向行为

缓存：

- `/me/data` 设置 `Cache-Control: no-store`

## 阶段结论

第一阶段已完成工程与接口验证。

第二阶段新增了管理页 JSON 边界：

- `/me/data`
- `/me/tasks/apply`
- `/events`

第三阶段新增了短信页 JSON 边界：

- `/sms/native/data`
- `/tasks/parse-sms/native`
- `/tasks/parse-sms/native-paste`

这几个接口足够支撑 Vue 版管理页：

- 登录态复用
- 筛选分页
- 批量选择
- 批量编辑
- 共享
- 物理删除
- SSE 静默同步
