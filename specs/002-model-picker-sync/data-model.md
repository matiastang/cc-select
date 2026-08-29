# Data Model: model-picker-sync

> 来源：`spec.md` Key Entities + FR-001/003/004。本特性**不引入新的持久化存储文件**——所有产物都写入既有的 profile `settings.json`（CC 消费的契约文件）或由既有 env 派生。

## 实体 1：ModelPlan（provider 派生模型计划）

由 `Provider.Env` **纯函数派生**（不落盘、无 schema 变更，对应 spec Assumption「Q11 env 派生」）。

```text
ModelPlan
├── Main    string            // env["ANTHROPIC_MODEL"]（空 = 未配置主模型）
├── Opus    string            // env["ANTHROPIC_DEFAULT_OPUS_MODEL"]
├── Sonnet  string            // env["ANTHROPIC_DEFAULT_SONNET_MODEL"]
├── Haiku   string            // env["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
└── Entries []ModelEntry      // 去重后的注入清单（保序：Main, Opus, Sonnet, Haiku）

ModelEntry
├── ID   string   // 模型 id 原文（如 "glm-5.3[1m]"）
└── Slot Slot     // 来源槽位（main | opus | sonnet | haiku）
```

**派生规则（验证约束）**：

- 值为空或 `$keychain:` 占位的槽位跳过（模型名不是敏感值，占位无意义——沿用 `config.IsKeychainPlaceholder` 判定）。
- 同 id 多槽位出现时去重，**首槽位保留**（`ANTHROPIC_MODEL` 最优先）。
- 四槽全空 → `Entries` 为空 → **不注入**（spec FR-001 官方 provider / 无模型变量 provider 路径）。

**消费者（三处共用这一份派生）**：

| 消费方 | 用途 |
|---|---|
| profile 构建（`Sync`/`SyncProxy`） | 注入 `modelPicker`（+ Mode P 的 `model` 字段） |
| `route switch` 刷新 | 重写当前终端 settings.json 的 `modelPicker`/`model` |
| 代理改写（`ModelRewrite`） | 槽位映射表 + 清单透传判定 |

单一派生点是本设计的关键约束：**注入显示与代理路由必须永远同源**，否则出现「能选但不生效」或「生效但不能选」的裂口。

## 实体 2：profile settings.json 注入产物

写入既有 profile `settings.json` 的字段（Mode B/P 经 mergeSettings 合并路径；Mode A 经 syncFull）：

```json
{
  "env": { "...": "..." },
  "modelPicker": {
    "replaceBuiltInOptions": true,
    "options": [{ "model": "glm-5.3[1m]" }, { "model": "glm-5.2[1m]" }, { "model": "glm-5.3-Flash" }]
  },
  "model": "glm-5.3[1m]"
}
```

**字段所有权矩阵**（谁写、谁保留——FR-004/005 的竞态语义）：

| 字段 | 写者 | 规则 |
|---|---|---|
| `env` | cc-select（mergeSettings 整体替换，现状不变） | 每次 `use` 重建 |
| `modelPicker` | cc-select | 每次 `use` 重建 + 每次 `route switch` 刷新；CC 不碰 |
| `model`（Mode P） | cc-select 注入主模型；CC 在 `/model` 确认时覆写 | `route switch` 刷新时重置为新主模型；`use` 重建时重新注入（CC 的 `/model` 选择不跨 `use` 存活——与现状一致，文档化） |
| `model`（Mode B） | 不注入（`ANTHROPIC_MODEL` env 优先级高于 `model`，env 已保证 ✔ 正确） | 维持现状 |
| 其余字段（permissions/hooks/availableModels…） | 继承自全局 / CC / 用户 | mergeSettings 未知字段保留（现状不变） |

**Mode 差异矩阵**：

| | 注入 modelPicker | 注入 `model` | 数据源 |
|---|---|---|---|
| Mode A | ✅ | ❌ | `Provider.Env` |
| Mode B | ✅ | ❌（env 优先 sufficient） | `Provider.Env`（= profile env 真值） |
| Mode P | ✅ | ✅（覆盖全局继承的 `opus[1m]`，否则 CC 内部认知停在目录别名，✔ 落错行） | `providers.json` 的 `Provider.Env`（**不是** profile env——profile 只有代理 BASE_URL） |

## 实体 3：槽位映射规则（代理改写表）

请求体 `model` 的分类决策（FR-003 的精确化，逐请求执行）：

```text
1. model ∈ 当前路由 provider 的 Entries.ID（精确匹配）→ 原样转发
2. model 含 "opus"（大小写不敏感）→ Opus 槽；含 "sonnet" → Sonnet 槽；含 "haiku" → Haiku 槽
   → 槽位变量非空则替换为该值；槽位变量为空 → 回落 Main
3. 其余（含无法分类的未知 id）→ Main
4. Main 为空（provider 未配 ANTHROPIC_MODEL）→ 原样透传（v1 行为不回归）
```

**设计要点**：

- 规则 1 优先于规则 2——provider 清单匹配是权威，避免 provider id 恰好含 "sonnet" 之类子串的误判。
- 规则 2 的子串分类覆盖 CC 实际会发的全部形态（完整 id `claude-opus-*`、别名 `opus`、`opus[1m]`、后台任务的 haiku id）。
- 规则 3 的「未知 → Main」与 v1「无条件 → Main」兼容：热切后会话继续发旧 provider 的 id 时，行为与今天完全一致。
- 32 MiB 上限保护、非 JSON 透传等既有防护原样保留。

## 生命周期：modelPicker 状态机

```text
absent ──use(首次,有模型变量)──▶ injected(provider A 清单)
injected ──route switch(B)──▶ refreshed(B 清单)          [仅写发起终端的 CLAUDE_CONFIG_DIR]
refreshed ──use(任意)──▶ regenerated(use 目标清单)        [重建=回到 provider 配置真值]
任意状态 ──CC /model Enter──▶ model 字段被 CC 改写        [modelPicker 不受影响]
```

- 刷新失败（文件损坏/不可写/CLAUDE_CONFIG_DIR 未设置）→ **路由切换照常成功**，仅打印警告（路由表是 Mode P 真值，显示是增强）。
- `model` 字段**不热重载**（CC 官方明确的三个启动期键之一）→ 热切刷新后，本会话的「当前模型」标记与 Default 行显示保持旧值，直至用户在 `/model` 选择或重启。列表（modelPicker）热重载已由 2026-08-29 实验证实。
