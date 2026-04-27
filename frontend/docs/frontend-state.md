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

- 先做 Vue 版管理页
- 选中集合、筛选条件、分页状态都放在同一个页面级 store 中
- 弹窗作为独立组件

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

## 第一阶段结论

当前最需要组件化的不是视觉，而是状态边界。

迁移优先级：

1. `/me` 管理页
2. `/sms/native` 短信导入页
3. 首页任务区
