# 后端 API 与模块

## 1. 总体结构

Go 后端采用模块化单体。业务模块之间通过明确服务接口协作，不在首版引入微服务。

SQLite 连接和迁移由后端进程直接管理。存储层仍通过接口与业务层隔离，但首版不为切换其他数据库增加不必要的抽象。

```text
auth          登录、会话、修改密码
accounts      管理员账号管理
characters    人物卡当前状态、权限、自动保存
history       版本、差异、比较和恢复
campaigns     团本、内容块、挂靠和可见性
rules/coc7    属性、派生值、技能和检定规则
dice          表达式解析、随机数、结果持久化
notifications 通知任务和 QQ 适配器
assets        图片元数据和存储适配器
```

API 使用 `/api/v1` 前缀。错误返回稳定的机器码和可展示消息；所有时间使用 UTC 传输，前端按用户时区显示。

## 2. 认证与账号

```text
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me
PUT    /api/v1/auth/password

GET    /api/v1/admin/accounts
POST   /api/v1/admin/accounts
PATCH  /api/v1/admin/accounts/:id
POST   /api/v1/admin/accounts/:id/reset-password
```

采用服务端 Session 和安全 Cookie。修改密码、停用账号和管理员重置密码时撤销相关会话。

## 3. 人物卡与历史

```text
GET    /api/v1/characters
POST   /api/v1/characters
GET    /api/v1/characters/:id
GET    /api/v1/characters/:id/campaigns
PATCH  /api/v1/characters/:id
POST   /api/v1/characters/:id/archive
POST   /api/v1/characters/:id/clone

GET    /api/v1/characters/:id/versions
GET    /api/v1/characters/:id/versions/:version
GET    /api/v1/characters/:id/compare?from=&to=
POST   /api/v1/characters/:id/restore/:version
POST   /api/v1/characters/:id/generate-attributes
POST   /api/v1/characters/:id/age-adjustment
POST   /api/v1/characters/:id/occupation
PUT    /api/v1/characters/:id/skill-allocation
POST   /api/v1/characters/:id/skill-growth
```

人物卡更新请求包含：

```json
{
  "baseVersion": 37,
  "editSessionId": "client-generated-id",
  "sourceCampaignId": "optional-id",
  "message": "目睹神话生物",
  "changes": [
    {"op": "replace", "path": "/conditions/sanity/current", "value": 49}
  ]
}
```

服务端只接受允许的字段路径和操作，执行规则校验、权限检查和乐观锁。成功后返回最新人物卡、版本号及规范化后的派生值。

## 4. 团本

```text
GET    /api/v1/campaigns
POST   /api/v1/campaigns
GET    /api/v1/campaigns/:id
PATCH  /api/v1/campaigns/:id

POST   /api/v1/campaigns/:id/blocks
PATCH  /api/v1/campaigns/:id/blocks/:blockId
DELETE /api/v1/campaigns/:id/blocks/:blockId
PUT    /api/v1/campaigns/:id/blocks/order

POST   /api/v1/campaigns/:id/characters
PATCH  /api/v1/campaigns/:id/characters/:characterId
DELETE /api/v1/campaigns/:id/characters/:characterId
```

读取团本详情时由后端按调用者权限过滤 KP 内容与隐藏 NPC，避免将秘密数据发给前端后再隐藏。

## 5. 骰子 API

```text
POST   /api/v1/rolls/check
POST   /api/v1/rolls/push
POST   /api/v1/rolls/reroll
POST   /api/v1/rolls/expression
POST   /api/v1/rolls/:id/reroll
GET    /api/v1/rolls
GET    /api/v1/campaigns/:id/rolls
```

所有创建投骰接口要求客户端提供 `requestId`，网络重试时返回同一结果，不重新生成随机数。

组合骰表达式首版语法：

```text
expression := term (("+" | "-") term)*
term       := dice | integer
dice       := [count] "d" sides
```

首版支持正整数骰数和面数、加减及空格；限制最大骰数、最大面数和表达式长度。括号可在后续加入，避免首版解析器复杂度无必要增加。

## 6. 通知接口

```go
type DiceNotifier interface {
    SendDiceResult(ctx context.Context, event DiceRollEvent) (DeliveryResult, error)
}
```

实现：

- `DisabledNotifier`
- `ConsoleNotifier`
- `QQOfficialBotNotifier`（部署阶段接入）

只有公开且符合团本通知设置的投骰会创建发送任务。KP 暗骰和测试骰在通知层强制跳过，即使客户端提交错误参数也不会外发。

管理接口预留：

```text
GET    /api/v1/campaigns/:id/notification-settings
PUT    /api/v1/campaigns/:id/notification-settings
POST   /api/v1/campaigns/:id/notification-settings/test
GET    /api/v1/campaigns/:id/notification-deliveries
POST   /api/v1/notification-deliveries/:id/retry
```

## 7. 图片

```text
POST   /api/v1/assets
GET    /api/v1/assets/:id
DELETE /api/v1/assets/:id
```

服务端校验实际文件类型、大小和访问权限，不信任扩展名。富文本只引用资源 ID，不直接保存任意外部 HTML。
