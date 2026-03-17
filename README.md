# Todo

基于 Go + PostgreSQL 的单日聚焦任务系统。

这套项目现在已经不是最早的演示型 MVP，而是一版可实际使用的 Web 应用，重点是：

- 多用户
- PostgreSQL 完整约束和迁移
- 单日视图
- `todo / schedule / ddl` 三类任务
- 管理员审批注册
- PWA 安装
- 前台 SSE 实时同步

## 当前定位

这个系统追求的是“低干扰”。

首页默认只看某一天真正需要出现的任务，不做复杂的多栏工作台。  
日常使用的主路径是：

1. 打开当天页面
2. 只处理今天需要出现的任务
3. 完成、撤销、延期
4. 用简化输入面板继续新增

## 核心模型

系统里有三种任务：

- `todo`
  - 普通待办
  - 没有固定出现日期
  - 会一直显示，直到完成
- `schedule`
  - 某天发生的日程
  - 只在对应那一天显示
  - 支持单次和批量创建
- `ddl`
  - 有截止时间的任务
  - 只会在“创建当天到截止当天”之间显示
  - 在截止当天按小时 / 分钟倒计时

每条任务都有 `1-5` 的重要等级。  
当前排序规则是：

- 先按重要等级排序
- 再按时间紧迫度排序
- 如果 DDL 已进入“当天按小时/分钟倒计时”的阶段，会优先顶到最前

## 已实现能力

### 账户与权限

- 多用户注册 / 登录 / 登出
- 首个注册账号自动成为 admin
- 后续注册账号必须由 admin 审批后才能登录
- admin 可批准或直接拒绝并删除待审批账号
- 基于 HttpOnly Cookie 的会话

### 任务与交互

- 单日聚焦视图
- `昨天 / 今天 / 明天 / 后天` 快速切换
- 自定义滚轮式日期选择器
- 完成、撤销、延期
- `更多` 抽屉查看当前视图日期下的已完成任务
- 已完成列表只显示“这一天完成的任务”
- 金句空状态

### 输入方式

- `Todo / 日程 / DDL` 显式创建
- 星级重要度录入
- `schedule` 支持：
  - 单次创建
  - 批量创建
  - 批量模式下按起始日期、截止日期、周一到周日展开
- `ddl` 支持精确到分钟
- `短信` 独立大输入框
- `ICS` 文件导入

### 解析与导入

- 中文日期基础解析
- 快递短信批量解析为 `todo`
- ICS 导入为 `schedule`
- ICS 只保留 `SUMMARY` 作为标题

### 实时与安装

- PWA，可在 Chrome 中安装为应用
- 前台 SSE 实时同步
- 手机或桌面切回前台时会自动补同步

## 技术栈

- 后端：Go
- 路由：`chi`
- 数据库：PostgreSQL
- 前端：服务端模板 + 原生 JS + CSS
- 实时同步：SSE
- PWA：`manifest + service worker + icons`

## 项目结构

```text
cmd/server            HTTP 启动入口
db/migrations         PostgreSQL schema 迁移
deploy/synology       群晖部署文件
internal/config       配置加载
internal/database     迁移执行器
internal/domain       核心领域模型
internal/repository   数据访问层
internal/service      业务逻辑、解析、导入
internal/web          HTTP handler、模板数据、SSE
scripts               辅助脚本
web/templates         HTML 模板
web/static            CSS / JS / PWA 资源
```

## 数据库设计

核心表：

- `app_users`
  - 用户账号
  - 显示名
  - 角色
  - 审批状态
- `user_sessions`
  - 登录会话
  - 过期时间
  - 最近活跃时间
- `ingestion_sources`
  - 输入来源
  - 原始文本 / ICS
  - 校验和
  - metadata
- `tasks`
  - 统一任务主表
  - 三类任务共用
  - `importance`
  - `scheduled_for / deadline / completed_at`
- `task_events`
  - 创建
  - 导入
  - 完成
  - 恢复
  - 延期

数据库层当前已做的事情：

- 枚举类型
- 任务类型约束
- 完成状态一致性约束
- `importance` 取值约束 `1-5`
- `source_id + user_id` 复合归属约束
- `metadata / payload` 强制 JSON object
- 用户名、显示名、会话边界约束
- 用户维度索引
- 活跃任务索引
- ICS 去重索引
- 事件时间索引
- `updated_at` 自动更新时间

数据库是这个项目里约束最完整的一层，当前设计目标就是“先把 PG 做稳，再逐步打磨交互”。

## 本地运行

### 方式一：直接运行

1. 启动 PostgreSQL
2. 复制并修改环境变量

```bash
cp .env.example .env
```

3. 启动服务

```bash
go run ./cmd/server
```

访问：

- `http://localhost:8080`

首次使用时：

- 访问 `http://localhost:8080/register`
- 首个账号会自动成为 admin

### 方式二：Docker Compose

```bash
docker compose up --build
```

访问：

- `http://localhost:8080`

## 环境变量

最常用的是下面这些：

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `APP_ADDR` | HTTP 监听地址 | `:8080` |
| `APP_TIMEZONE` | 应用时区 | `Asia/Shanghai` |
| `AUTO_MIGRATE` | 启动时自动跑迁移 | `true` |
| `MIGRATIONS_DIR` | 迁移目录 | `db/migrations` |
| `ICS_IMPORT_HORIZON_DAYS` | ICS 向未来展开的天数 | `180` |
| `MAX_UPLOAD_SIZE_BYTES` | 上传大小限制 | `4194304` |
| `DATABASE_URL` | 主数据库连接串 | `postgres://todo:todo@localhost:5432/todo?sslmode=disable` |
| `QUOTES_DATABASE_URL` | 金句数据库连接串，可留空 | 空 |
| `SESSION_COOKIE_NAME` | Cookie 名称 | `todo_session` |
| `SESSION_TTL_HOURS` | 会话有效时长 | `720` |
| `SESSION_SECURE_COOKIE` | HTTPS 下建议设为 `true` | `false` |
| `ALLOW_REGISTRATION` | 是否允许注册 | `true` |

## 输入与解析说明

### 手动创建

当前主要推荐显式创建，而不是依赖自然语言“猜”。

- `Todo`
  - 标题
  - 星级
- `日程`
  - 标题
  - 星级
  - 单次 / 批量
- `DDL`
  - 标题
  - 星级
  - 日期时间

### 日程批量创建

`schedule` 的批量模式支持：

- 起始日期
- 截止日期
- 周一到周日选择

规则是“包含起始日期和截止日期”。

### 短信解析

短信入口是独立大输入框。  
当前重点支持快递取件短信，解析结果全部写成 `todo`，默认 `2 星`。

例如会提取成：

- `驿站：140-1-3005`
- `9号柜 466412`
- `5号柜 055151`

支持一次性粘贴很多条短信，就算中间没有空行，也会按 `【...】` 头识别。

### ICS 导入

ICS 导入结果全部按 `schedule` 处理。

当前策略：

- 使用 `SUMMARY` 作为标题
- 不保留多余备注
- 支持常见周期展开
- 按 `uid + 日期` 去重

## 视图规则

### 单日视图

首页只显示当前查看日期需要出现的任务。

### DDL 显示规则

DDL 当前是按“查看日期”计算的，不是死盯真实今天。

规则包括：

- 创建前不显示
- 创建当天开始显示
- 截止当天仍显示
- 截止后不再显示
- 如果查看的就是截止当天：
  - 当天且是现实今天，显示小时 / 分钟倒计时
  - 当天但不是现实今天，显示“今天”

### 已完成

“更多”里的已完成列表是“当前查看日期完成的任务”，不是全局历史完成记录。

## 实时同步

当前实时同步只针对“前台打开的页面 / PWA”。

实现方式：

- 后端通过 SSE 向当前用户广播任务变更
- 前端静默拉取最新 snapshot
- 不整页重载
- 页面切回前台时会补同步

说明：

- 前台打开时，可以接近秒同步
- 退到后台后，系统可能会挂起连接
- 后台再切回前台，会自动补一次同步

## PWA

当前已经具备：

- `manifest.webmanifest`
- `service worker`
- `favicon / apple-touch-icon / 192 / 512 / maskable` 图标
- Chrome 安装为 app

如果你替换了根目录的 `logo.png`，重新生成图标：

```bash
./scripts/generate_pwa_assets.sh
```

### PWA 更新说明

如果你更新了前端资源或 service worker，已经安装过的 PWA 可能还在使用旧缓存。  
最稳的做法是：

1. 彻底关闭已安装的 PWA
2. 重新打开一次
3. 如仍异常，清站点数据或重新安装

## 群晖部署

如果你要从 macOS 导出给群晖 `x86_64 / amd64` 使用，直接看：

- `deploy/synology/README.md`

已经准备好的内容包括：

- 强制导出 `linux/amd64` 的镜像脚本
- 可直接导入群晖的镜像 tar
- 群晖内置 PostgreSQL 方案
- 外部 PostgreSQL 方案
- 环境变量模板

如果你只是让群晖跑应用、数据库仍用外部 PG，那么通常只需要导入：

- `dist/synology-amd64/todo-app-synology-amd64.tar`

注意：镜像里已经包含应用、模板、静态资源、PWA 资源和迁移文件；数据库数据和环境变量不在镜像里。

## 验证

当前常用检查：

```bash
go test ./...
```

前端脚本也建议在改动后检查：

```bash
node --check web/static/task-cards.js
node --check web/static/focus-page.js
node --check web/static/composer-panel.js
node --check web/static/realtime-sync.js
node --check web/static/sw.js
```

## 当前限制

当前这版已经可用，但仍然有几个明确边界：

- 手机端交互不是主战场，桌面端更成熟
- 自然语言解析只做基础支持，不追求“像 AI 一样理解”
- 后台推送目前只做到“切回前台补同步”，没有系统级 push
- 前端交互经过大量细调，后续仍可能继续收敛

## 后续方向

如果继续往下做，比较自然的顺序是：

- 继续优化移动端模式
- 给短信 / ICS 做更明确的反馈和导入结果页
- 增加 JSON API
- 做更完整的管理员能力
- 做真正的后台推送或通知

## License

MIT，见 `LICENSE`。
