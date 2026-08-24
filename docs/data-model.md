# 数据模型

本文描述 SQLite 逻辑模型。字段类型、索引和约束在数据库迁移阶段细化。

服务启动时启用外键约束、WAL 日志模式和合理的 busy timeout。应用使用单个连接池协调写入；当前使用规模不到十人，不需要引入独立数据库服务。

## 1. 实体关系

```text
accounts 1 ── * characters
accounts 1 ── * campaigns (keeper_id)

campaigns * ── * characters
        via campaign_characters

characters 1 ── * character_versions
campaigns  1 ── * campaign_blocks
campaigns  1 ── * dice_rolls
characters 0..1 ── * dice_rolls
dice_rolls 1 ── * notification_deliveries
```

## 2. 账户

### `accounts`

- `id`
- `username`（唯一）
- `display_name`
- `password_hash`
- `role`: `admin | user`
- `status`: `active | disabled`
- `must_change_password`
- `password_changed_at`
- `created_at`、`updated_at`

不保存明文密码。登录会话单独存储并支持服务端撤销。

## 3. 人物卡

### `characters`

- `id`
- `owner_account_id`
- `kind`: `investigator | npc`
- `ruleset`: 首版固定 `coc7`
- `status`: `draft | active | retired | deceased | archived`
- `name`
- `avatar_asset_id`
- `current_version`
- `sheet_data`：SQLite JSON 文本，保存结构化人物卡当前状态
- `created_at`、`updated_at`、`archived_at`

关系和权限等稳定字段独立成列；规则字段集中在有版本定义的 `sheet_data` 中，兼顾查询能力与后续字段扩展。需要查询的常用值应提取为普通列或使用 SQLite JSON 函数建立索引，不依赖 PostgreSQL 专有类型。

`sheet_data` 按区域组织，而不是保存任意表单：

```text
schema_version
profile
attributes
derived_values
conditions
skills[]
weapons[]
possessions[]
finances
backstory
mythos
notes
```

### `character_versions`

- `id`
- `character_id`
- `version`（人物卡内单调递增）
- `parent_version`
- `actor_account_id`
- `source_campaign_id`（可空）
- `change_kind`: `edit | generation | restore | import | system`
- `message`（可空）
- `changed_paths`：字段路径摘要
- `diff_data`：用于快速展示的结构化差异
- `snapshot_data`：该版本完整人物卡快照
- `created_at`

`(character_id, version)` 唯一。版本和当前人物卡更新必须处于同一数据库事务。

前端提交 `base_version`。若与服务器当前版本不一致，服务端返回冲突，不静默覆盖他人的修改。用户可刷新差异后重新应用。

### 自动保存合并

客户端使用稳定的 `edit_session_id` 标识一次编辑意图，服务端可在短时间窗内合并同一操作者、同一人物卡和同一编辑会话的连续文本更新。

数据库始终保证当前状态安全落盘；展示层的逻辑版本可以合并。数值变动、随机属性生成、批量操作和恢复操作不合并。

## 4. 团本

### `campaigns`

- `id`
- `keeper_account_id`
- `title`
- `summary`
- `cover_asset_id`
- `ruleset`: `coc7`
- `era`
- `status`: `preparing | active | completed | archived`
- `created_at`、`updated_at`、`archived_at`

### `campaign_blocks`

- `id`
- `campaign_id`
- `title`
- `content`：编辑器结构化内容
- `visibility`: `public | keeper`
- `sort_order`
- `created_at`、`updated_at`

### `campaign_characters`

- `id`
- `campaign_id`
- `character_id`
- `role`: `investigator | npc`
- `visibility`: `hidden | summary | public`
- `joined_at`、`left_at`

同一人物卡在同一团本内只允许一个有效挂靠。关联表不存储人物卡状态。

## 5. 骰子

### `dice_rolls`

- `id`
- `request_id`（幂等键）
- `actor_account_id`
- `character_id`（可空）
- `campaign_id`（可空）
- `visibility`: `public | keeper | test`
- `roll_kind`: `check | damage | expression | attribute_generation`
- `label`
- `expression`
- `request_data`
- `result_data`：每颗骰子、合计和 CoC 判定
- `created_at`

随机数和规则判定均由 Go 后端生成。重投创建新记录并通过 `reroll_of_id` 关联原记录。

## 6. QQ 通知

### `campaign_notification_settings`

- `campaign_id`
- `provider`: `disabled | console | qq_official`
- `target_reference`
- `event_settings`
- `secret_reference`（只引用服务端密钥，不保存密钥内容）
- `updated_at`

### `notification_deliveries`

- `id`
- `dice_roll_id`
- `provider`
- `status`: `pending | sending | sent | retryable | failed | skipped`
- `attempt_count`
- `next_attempt_at`
- `provider_message_id`
- `last_error_code`
- `created_at`、`sent_at`

投骰事务只写入投骰和待发送任务，不等待外部 QQ API。后台工作器异步发送，保证 QQ 故障不会阻塞跑团。

## 7. 图片资源

### `assets`

- `id`
- `owner_account_id`
- `storage_provider`
- `storage_key`
- `original_name`
- `media_type`
- `size_bytes`
- `checksum`
- `created_at`

开发期使用本地存储实现，业务层通过存储接口访问，为以后切换对象存储保留空间。

## 8. SQLite 运维约束

- 默认文件为 `.data/coc.db`，可以通过 `COC_DATABASE_PATH` 修改；
- 数据库迁移由 Go 服务启动时执行，并记录 schema 版本；
- 启用 `foreign_keys`、`journal_mode=WAL` 和 `busy_timeout`；
- 写事务保持短小，网络调用和 QQ 推送不得发生在数据库事务内；
- 版本快照可能增大数据库，需要在归档和备份时观察文件大小，但不清理历史；
- 正式环境定期生成一致性备份，并实际测试恢复流程。
