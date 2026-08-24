# COC7版人物卡

面向小型朋友团体的《克苏鲁的呼唤》第 7 版跑团网站。

当前仓库已经进入工程阶段。已确认的核心原则：

- React + TypeScript 前端、Go 单体后端、SQLite 数据库。
- 封闭式账户系统，不开放注册。
- 人物卡是全站唯一实体；团本只挂靠人物卡，不复制团本专属状态。
- 人物卡使用所见即所得编辑，并保存从创建开始的完整版本历史。
- 骰子结果由后端生成；QQ 官方机器人作为可替换通知通道后续接入。
- 第一阶段优先保证信息结构和操作流程，视觉风格可在功能稳定后替换。

设计文档：

- [产品与交互设计](docs/product-design.md)
- [权限设计](docs/permissions.md)
- [数据模型](docs/data-model.md)
- [后端 API 与模块](docs/api-design.md)
- [开发路线](docs/roadmap.md)
- [职业 JSON 格式](docs/occupations.md)

## 本地开发

建议使用当前稳定版 Go，以及 Node.js 22 或更新版本。SQLite 数据库由 Go 服务直接管理，不需要安装或启动独立数据库服务。

```bash
cp .env.example .env
npm --prefix web install
make dev-api
```

另开终端启动前端：

```bash
make dev-web
```

访问 `http://localhost:5173`。当前工程骨架的健康检查位于：

```text
GET http://localhost:8080/api/v1/health/live
GET http://localhost:8080/api/v1/health/ready
```

首次使用需要先创建一个系统管理员。密码不能为空，命令只在创建时读取环境变量，不会写入配置文件：

```bash
COC_ADMIN_PASSWORD='请替换为初始密码' make create-admin
```

也可以指定用户名和显示名称：

```bash
COC_ADMIN_PASSWORD='请替换为初始密码' \
  go run ./cmd/admin create -username keeper -display-name '守秘人'
```

服务会自动创建 `.data/coc.db` 并执行数据库迁移。正式部署时应设置 `COC_COOKIE_SECURE=true`，确保 Session Cookie 仅通过 HTTPS 发送。

当前已提供的账户 API：

```text
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
PUT  /api/v1/auth/password
GET  /api/v1/admin/accounts
POST /api/v1/admin/accounts
POST /api/v1/admin/accounts/:id/reset-password
PATCH /api/v1/admin/accounts/:id/status
POST /api/v1/admin/accounts/:id/revoke-sessions
GET  /api/v1/admin/system/status
GET  /api/v1/admin/audit-logs
GET  /api/v1/admin/backups
POST /api/v1/admin/backups
GET  /api/v1/admin/backups/:name
POST /api/v1/admin/backups/validate

GET   /api/v1/characters
POST  /api/v1/characters
GET   /api/v1/characters/:id
GET   /api/v1/characters/:id/campaigns
PATCH /api/v1/characters/:id
GET   /api/v1/characters/:id/versions
GET   /api/v1/characters/:id/versions/:version
GET   /api/v1/characters/:id/compare?from=:version&to=:version
POST  /api/v1/characters/:id/restore/:version
POST  /api/v1/characters/:id/generate-attributes
POST  /api/v1/characters/:id/age-adjustment
GET   /api/v1/rules/coc7/occupations
POST  /api/v1/characters/:id/occupation
PUT   /api/v1/characters/:id/skill-allocation
POST  /api/v1/characters/:id/skill-growth
POST  /api/v1/characters/:id/copy
PATCH /api/v1/characters/:id/status

POST  /api/v1/rolls/check
POST  /api/v1/rolls/push
POST  /api/v1/rolls/reroll
POST  /api/v1/rolls/expression
GET   /api/v1/rolls

GET   /api/v1/campaigns
POST  /api/v1/campaigns
GET   /api/v1/campaigns/:id
PATCH /api/v1/campaigns/:id
PATCH /api/v1/campaigns/:id/cover
GET   /api/v1/campaigns/:id/blocks
POST  /api/v1/campaigns/:id/blocks
PATCH /api/v1/campaigns/:id/blocks/:blockId
DELETE /api/v1/campaigns/:id/blocks/:blockId
POST  /api/v1/campaigns/:id/blocks/:blockId/move
POST  /api/v1/campaigns/:id/assets
GET   /api/v1/campaigns/:id/assets/:assetId
GET   /api/v1/campaigns/:id/characters
GET   /api/v1/campaigns/:id/rolls
POST  /api/v1/campaigns/:id/characters
PATCH /api/v1/campaigns/:id/characters/:characterId
DELETE /api/v1/campaigns/:id/characters/:characterId
GET   /api/v1/campaigns/:id/notifications
PUT   /api/v1/campaigns/:id/notifications
GET   /api/v1/campaigns/:id/notification-deliveries
```

运行验证：

```bash
make test
make check
```

## 从完整备份恢复

恢复必须在 API 服务停止后执行。命令会先为当前数据创建一份新的安全备份，再校验并替换数据库、图片和自定义职业文件：

```bash
make restore BUNDLE=/绝对路径/coc-日期.tar.gz
```

恢复成功后再重新启动 API。不要在服务运行期间执行恢复命令。
