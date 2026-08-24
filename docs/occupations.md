# CoC 7版职业目录

职业使用两个相互独立的 JSON 文件：

- `data/rules/coc7/occupations.official.json`：项目维护的官方职业，只读使用；
- `.data/rules/coc7/occupations.custom.json`：用户自己的职业，首次启动时自动创建。

后端启动时先校验两个文件，再合并职业列表。官方 ID 必须以 `official.` 开头，自定义 ID 必须以 `custom.` 开头，自定义职业不能覆盖官方职业。

文件结构：

```json
{
  "schemaVersion": 1,
  "occupations": []
}
```

职业条目格式示例（演示数据，不代表官方职业规则）：

```json
{
  "id": "custom.example",
  "name": "示例职业",
  "eras": ["modern"],
  "creditRating": { "min": 10, "max": 60 },
  "skillPointFormulas": [
    {
      "label": "EDU × 2 + INT × 2",
      "terms": [
        { "attribute": "edu", "multiplier": 2 },
        { "attribute": "int", "multiplier": 2 }
      ]
    }
  ],
  "fixedSkills": ["图书馆使用", "侦查"],
  "choiceGroups": [
    {
      "count": 1,
      "skills": ["魅惑", "话术", "恐吓", "说服"]
    }
  ],
  "freeChoiceCount": 2,
  "description": "自定义职业示例"
}
```

如果一个职业允许从多种属性公式中选择，可以在 `skillPointFormulas` 中添加多个公式。人物创建时记录所选公式索引以及职业快照。

启动校验包括：

- schema 版本；
- ID 命名空间和全局唯一性；
- 职业名称与适用年代；
- 信用评级范围必须在 0～99；
- 技能点公式只能引用 CoC 八项属性；
- 公式倍率和技能选择数量必须合理；
- 未知字段会被拒绝，避免拼写错误被静默忽略。

自定义文件属于运行数据，需要和 `.data/coc.db`、上传资源一起备份。

