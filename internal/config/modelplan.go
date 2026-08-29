package config

// modelplan.go 从 provider env 派生 ModelPlan（specs/002 data-model.md 实体 1 / research D1）。
//
// ModelPlan 是「模型选择器注入」与「代理 model 改写」的**唯一共同数据源**——
// 显示与路由必须永远同源，否则出现「能选但不生效」或「生效但不能选」的裂口。
//
// 派生规则：
//   - 槽位取值：ANTHROPIC_MODEL（主）与三个 ANTHROPIC_DEFAULT_*_MODEL；
//   - 值为空或 $keychain: 占位的槽位跳过（模型名不是敏感值，占位无意义）；
//   - Entries = 四槽去重（同 id 首槽位保留），保序 main → opus → sonnet → haiku；
//   - 四槽全空 → 空 plan（调用方据此跳过注入，保持现状显示）。
//
// 纯函数、无 IO：三处消费者（profile 注入 / router 改写 / route 刷新）共用。
type Slot string

const (
	SlotMain   Slot = "main"
	SlotOpus   Slot = "opus"
	SlotSonnet Slot = "sonnet"
	SlotHaiku  Slot = "haiku"
)

// ModelEntry 是注入清单的一行（选择器 options 与代理透传判定的最小单元）。
type ModelEntry struct {
	ID   string
	Slot Slot
}

// ModelPlan 是某 provider 的模型计划。
type ModelPlan struct {
	Main    string
	Opus    string
	Sonnet  string
	Haiku   string
	Entries []ModelEntry
}

// ModelPlanFromEnv 从 provider env 派生 ModelPlan。env 为 nil 或不含任何
// 模型变量时返回空 plan（Entries 为空）。
func ModelPlanFromEnv(env map[string]string) ModelPlan {
	var plan ModelPlan
	seen := map[string]bool{}
	add := func(id string, slot Slot) {
		if id == "" || IsKeychainPlaceholder(id) || seen[id] {
			return
		}
		seen[id] = true
		plan.Entries = append(plan.Entries, ModelEntry{ID: id, Slot: slot})
	}

	plan.Main = slotValue(env["ANTHROPIC_MODEL"])
	plan.Opus = slotValue(env["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	plan.Sonnet = slotValue(env["ANTHROPIC_DEFAULT_SONNET_MODEL"])
	plan.Haiku = slotValue(env["ANTHROPIC_DEFAULT_HAIKU_MODEL"])

	add(plan.Main, SlotMain)
	add(plan.Opus, SlotOpus)
	add(plan.Sonnet, SlotSonnet)
	add(plan.Haiku, SlotHaiku)
	return plan
}

// slotValue 清洗槽位取值：空串与 $keychain: 占位一律视为「未配置」。
// 模型名不是敏感值，占位对注入与改写均无意义（INV-1）。
func slotValue(v string) string {
	if IsKeychainPlaceholder(v) {
		return ""
	}
	return v
}
