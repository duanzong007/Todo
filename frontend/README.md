# Todo Frontend

这是 Todo 前端迁移工程。

当前已经进入第四阶段：Vue 版首页、任务管理页和短信导入页。

现阶段目标：

- 用 Vue 重做 `/` 首页核心交互
- 用 Vue 重做 `/me` 管理页的核心交互
- 复用现有登录 Cookie
- 复用 Go 后端任务管理逻辑
- 通过 `/me/data` 读取管理页数据
- 通过 `/dashboard/data` 读取首页完整数据
- 通过 `/me/tasks/apply` 异步提交批量编辑、共享和删除
- 继续复用 `/events` SSE 静默同步
- 用 Vue 重做 `/sms/native` 短信导入页
- 复用安卓壳 `SmsBridge`、本地历史缓存和后端短信识别接口

## 本地运行

先启动原有 Go 服务：

```bash
go run ./cmd/server
```

再启动前端：

```bash
cd frontend
npm install
npm run dev
```

默认 Vite 地址是：

```text
http://127.0.0.1:5173
```

如果后端不是 `http://localhost:8080`，复制 `.env.example` 为 `.env.local` 并修改：

```env
TODO_BACKEND_URL=http://localhost:8080
```

## 当前入口

- Go 服务里的 `/` 已切换为 Vue 版首页
- 旧 Go 模板首页已经删除，不再保留 `/classic`
- Go 服务里的 `/me` 仍是传统管理页，Vue 版保留在 `/me/vue`
- Go 服务里的 `/sms/native` 仍是传统短信导入页，Vue 版保留在 `/sms/native/vue`
- 继续沿用现有 PWA 注册和后端接口
- 不改安卓壳
- 不新增后端业务语义
- Vue 不重新实现任务业务规则

## 验证命令

```bash
npm run typecheck
npm run build
```
