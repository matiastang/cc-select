# Contract: 代理 model 路由契约（Mode P 请求体改写）

> 消费方：Mode P 会话（Claude Code 发往 `ANTHROPIC_BASE_URL=127.0.0.1:<port>` 的请求）。
> 生产方：cc-select 路由 daemon（`ModelRewrite` 中间件）。本契约是请求的**可观察行为**规范，上游 provider 可见的最终 `model` 以本契约为准。

## 1. 改写算法（按序判定，首个命中生效）

对进入转发层的每个 JSON 请求体（非 JSON / 无 `model` 字段 / 超 32 MiB → 原样透传，既有规则不变）：

| # | 条件 | 请求体 `model` 变为 | 说明 |
|---|---|---|---|
| R1 | 值**精确等于**当前路由 provider 清单中的某个 id（`[1m]` 后缀形态照原样参与匹配，清单 id 含则匹配含） | **不变（透传）** | 用户在 `/model` 的选择、CC 从 `model` 设置发出的 id 走这里 |
| R2 | 值含子串 `opus`（不区分大小写） | Opus 槽模型；槽空 → 主模型 | 内置目录/别名形态 |
| R3 | 值含子串 `sonnet` | Sonnet 槽模型；槽空 → 主模型 | 同上 |
| R4 | 值含子串 `haiku` | Haiku 槽模型；槽空 → 主模型 | 后台小任务 id 走这里 |
| R5 | 其余一切 | 主模型 | 与 v1「无条件主模型」兼容（热切后旧 id 的自然归宿） |
| R6 | provider 未配置 `ANTHROPIC_MODEL`（主模型为空） | **不变（透传）** | v1 行为不回归（spec FR-003） |

## 2. 可观察行为示例

当前路由 provider env：`ANTHROPIC_MODEL=glm-5.3[1m]`、`DEFAULT_OPUS=glm-5.2[1m]`、`DEFAULT_HAIKU=glm-5.3-Flash`（无 SONNET）：

| 请求体 `model`（CC 发出） | 转发给 provider 的 `model` | 场景 |
|---|---|---|
| `glm-5.3[1m]` | `glm-5.3[1m]`（透传） | 用户在 /model 选择了它 |
| `glm-5.3-Flash` | `glm-5.3-Flash`（透传） | 用户在 /model 选择了它 |
| `claude-opus-5` / `opus` / `opus[1m]` | `glm-5.2[1m]` | CC 默认/目录形态 |
| `claude-haiku-4-5` / `haiku` | `glm-5.3-Flash` | 后台小任务 |
| `claude-sonnet-5` | `glm-5.3[1m]`（sonnet 槽空 → 主模型） | 目录 sonnet 行 |
| `glm-4.6`（热切前的旧 provider id） | `glm-5.3[1m]` | route switch 后在途/后续请求（= v1 行为） |

## 3. 性质

- **幂等**：改写只依赖「当前路由 provider 的 env」，同一请求在路由不变时结果恒定。
- **零学习成本兼容**：未配置主模型的 provider 全路径透传（R6），与 v1 逐字节一致。
- **清单与注入同源**：R1 的「清单」与 `/model` 显示清单同为 `ModelPlan.Entries`（data-model.md 实体 1），保证 SC-003（选择 100% 生效）。
- **不触碰**：SSE 流式、非 Messages 端点、无 body 请求、header——维持 001 既有透传语义。
