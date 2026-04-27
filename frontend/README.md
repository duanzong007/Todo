# Todo Frontend

这是 Todo 前端迁移第一阶段的独立工程。

当前目标不是替换现有 Go 模板页面，而是验证：

- Vue + Vite 工程结构
- 与现有 Go 后端的接口边界
- 登录 Cookie 复用
- `/dashboard/snapshot` 数据读取
- `/events` SSE 实时通道

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

## 当前边界

- 不接管生产页面
- 不注册 PWA
- 不改安卓壳
- 不新增后端业务语义
- 只读取现有后端接口

## 验证命令

```bash
npm run typecheck
npm run build
```
