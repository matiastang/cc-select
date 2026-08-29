# Quickstart: model-picker-sync 端到端验证

> 前置：Claude Code ≥ 2.1.242（`claude --version`）；两个已配置模型的 provider（下文以 `glm`、`minimax` 为例）；Mode P 已启用（`cc-select mode proxy`）。
> 每个场景给出：操作 → 预期 → 判定依据（对应 spec 验收场景）。

## 场景 1（US1 / SC-001）：启动即见真实模型

```bash
ccs use glm
claude
```

**操作**：会话内输入 `/model`。
**预期**：列表仅含 GLM 已配置模型（如 `glm-5.3[1m]`、`glm-5.2[1m]`、`glm-5.3-Flash`，去重后），无内置 Anthropic 目录行；当前模型行 ✔ 正确；启动横幅显示真实模型 id。
**判定**：`cat ~/.cc-select/profiles/glm/settings.json | jq '.modelPicker, .model'` 可见注入块（Mode P 两者皆有；Mode B 仅 `modelPicker`）。

**对照组**：`ccs use official`（官方 provider）→ `/model` 与现状一致，无注入。

## 场景 2（US2 / SC-003）：选择生效

**操作**：在场景 1 会话的 `/model` 中选择 `glm-5.3-Flash`（`s` 键，仅本会话），发送任意一句话。
**预期**：服务商侧调用记录（GLM 控制台用量明细）显示该请求模型为 `glm-5.3-Flash`；后续多轮保持。
**daemon 侧等价验证**（无控制台权限时）：daemon 调试日志/抓包确认转发请求体 `model` 为 `glm-5.3-Flash` 且未被改写。

**回归对照**：后台小任务（如让 Claude 生成会话标题）发出的 haiku 类 id 仍被映射到 Haiku 槽模型（v1 语义不回归）。

## 场景 3（US3 / SC-002）：热切后选择器跟随

**操作**：在运行中的会话里执行：

```bash
cc-select route switch minimax
```

然后输入 `/model`。
**预期**：**不重启会话**，列表变为 MiniMax 的模型清单；选择其中模型后下一笔请求由该模型服务（MiniMax 侧可验证）。
**判定**：`jq '.modelPicker' $CLAUDE_CONFIG_DIR/settings.json`（在会话内 `!` 前缀执行）显示 MiniMax 清单——注意此文件是**发射 profile**（`profiles/glm/settings.json`），不是 `profiles/minimax/`。

## 场景 4（SC-004）：终端隔离回归

**操作**：终端 A 完成场景 3 后，终端 B（另一 provider）进入 claude 打开 `/model`。
**预期**：终端 B 的列表与路由不受 A 的热切影响；A 的 `route switch` 只改了 A 的 `$CLAUDE_CONFIG_DIR` 文件与 TID 路由条目。

## 场景 5（SC-005 / FR-004）：并发写入不丢字段

**操作**：会话内先 `/model` 确认选一个模型（CC 向 settings.json 写 `model` 字段），紧接着 `cc-select route switch <另一家>`，然后：

```bash
! jq 'keys' $CLAUDE_CONFIG_DIR/settings.json
```

**预期**：`model` 与 `modelPicker` 同时存在且分别为新主模型/新清单；`env`、`permissions` 等其余字段完整；文件 JSON 合法。

## 失败路径抽查

| 操作 | 预期 |
|---|---|
| `CLAUDE_CONFIG_DIR` 未设置的 shell 里 `route switch` | 路由切换成功，无刷新（静默 no-op），功能不报错 |
| 会话内把 settings.json 改坏（非法 JSON）后 `route switch` | 路由切换成功，stderr 告警，显示保持旧样 |
| provider 未配置任何模型变量时 `use` 它 | `/model` 与现状一致，无注入 |
