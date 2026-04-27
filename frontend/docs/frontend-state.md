# 现有前端状态梳理

## 首页状态

来源文件：

- `web/static/focus-page.js`
- `web/static/task-cards.js`
- `web/static/date-picker.js`
- `web/static/postpone-picker.js`
- `web/static/composer-panel.js`
- `web/static/realtime-sync.js`

主要状态：

- 当前查看日期
- 任务列表
- 已完成列表
- 金句空状态
- 延期面板开合
- 添加面板开合
- 日期选择器当前值
- 正在提交的后台请求数量
- SSE 同步状态
- 任务改名编辑态

迁移建议：

- 用一个 dashboard store 管理当前日期和 snapshot
- 用组件内部状态管理日期选择器、延期选择器和编辑态
- SSE 只负责触发 snapshot 刷新，不直接修改组件内部细节

## 管理页状态

来源文件：

- `web/static/account-manager.js`

主要状态：

- 筛选器弹窗
- 编辑弹窗
- 共享弹窗
- 选中任务集合
- 分页和显示数量
- 表单提交后的滚动位置恢复
- SSE 触发页面刷新

迁移建议：

- Vue 版管理页已经由页面级状态统一管理
- 选中集合、筛选条件、分页状态集中在 `frontend/src/App.vue`
- 后续如果继续扩复杂编辑器，再拆成独立组件和 store

## 短信导入页状态

来源文件：

- `web/static/native-sms.js`
- `web/static/native-sms-entry.js`

主要状态：

- 新短信列表
- 历史短信列表
- 本地历史记录
- 当前选择集合
- 读取中状态
- 提交中状态
- 粘贴导入弹窗
- 原生短信桥可用性

迁移建议：

- 抽象 `nativeBridge`
- Web 和 Android 壳共用同一套页面状态
- 短信识别继续只走后端接口
- Vue 版短信导入页已接管新短信、历史短信、粘贴导入、本地缓存和提交状态
- 本地历史和缓存 key 沿用旧版，避免升级后记录丢失

## 当前阶段结论

当前最需要组件化的不是视觉，而是状态边界。

迁移优先级：

1. `/me` 管理页，已进入 Vue 版实现
2. `/sms/native` 短信导入页，已进入 Vue 版实现
3. 首页任务区
