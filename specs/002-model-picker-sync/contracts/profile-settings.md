# Contract: profile settings.json 注入契约

> 消费方：Claude Code（读取 `<CLAUDE_CONFIG_DIR>/settings.json`）。生产方：cc-select。
> 本契约定义 cc-select 写入的字段形状与不变量；CC 侧语义以官方文档为准（`modelPicker` 需 CC ≥ 2.1.242）。

## 1. 注入形状

```json
{
  "modelPicker": {
    "replaceBuiltInOptions": true,
    "options": [{ "model": "<provider 模型 id>" }]
  },
  "model": "<provider 主模型 id>"
}
```

- `options[]` 每项**只含 `model`**（不注入 `label`/`description`——CC 对不认识的名字回退显示原始 id，真实且零派生逻辑）。
- `replaceBuiltInOptions` 恒为 `true`：第三方 provider 下内置目录行（含误导性定价）必须隐藏。
- `model` 字段**仅 Mode P 注入**；Mode A/B 不写（Mode B 由 `ANTHROPIC_MODEL` env 保证当前模型正确）。

## 2. 不变量

| # | 不变量 | 违反后果 |
|---|---|---|
| INV-1 | 注入内容**不得包含任何密钥/token**（模型名与 keychain 占位均不写入） | 违反 Mode P 无密钥约束（宪法 II） |
| INV-2 | `options[].model` 必须逐一等于 `Provider.Env` 四模型变量的去重值，不多不少 | 显示与代理路由裂口（选了不生效/生效不能选） |
| INV-3 | 写入必须**原子**（临时文件 + rename，沿用 `EnsureRaw` 模式） | CC 读到半个 JSON → 配置损坏 |
| INV-4 | 读-改-写必须**字段级合并**：除 `modelPicker` 与（Mode P 的）`model` 外，既有字段一律保留 | 覆盖 CC/用户字段（FR-004/SC-005） |
| INV-5 | 官方 provider 与四槽全空的 provider：**不产生任何注入** | 官方/未配置场景的显示行为变化 |

## 3. 写入时机与目标

| 时机 | 目标文件 | 写入内容 |
|---|---|---|
| `use`（Mode A/B/P 构建 profile） | `~/.cc-select/profiles/<id>/settings.json` | `modelPicker`（+ Mode P 的 `model`） |
| `route switch`（Mode P 热切） | **`$CLAUDE_CONFIG_DIR/settings.json`**（继承自会话 env） | `modelPicker` + `model` = 新 provider 主模型 |

**关键正确性约束（route switch）**：写入目标是**发射时的 profile 目录**（`$CLAUDE_CONFIG_DIR`），**不是** `profiles/<新 provider>/`。热切后 `CLAUDE_CONFIG_DIR` 仍指向发射 profile——写错文件运行中会话看不到刷新。

- `$CLAUDE_CONFIG_DIR` 未设置 → 跳过刷新（非 CC 会话上下文），静默 no-op。
- 目标文件不存在 / JSON 损坏 → 路由切换**照常成功**，刷新告警输出到 stderr。
- 只写**当前终端**的目录（env 天然 per-terminal），其他终端零触碰（FR-005 / SC-004）。

## 4. 与 CC 自身写入的共存

- CC 在用户 `/model` 确认时向 user settings 写 `model` 字段 → 属预期共存：cc-select 只在 `use` 重建与 `route switch` 时覆写 `model`，且覆写前按 INV-4 保留其他一切字段。
- `model` **不热重载**（CC 启动期键）→ 热切刷新后本会话当前模型标记不即时变化，属已知限制（data-model.md 生命周期节）。
