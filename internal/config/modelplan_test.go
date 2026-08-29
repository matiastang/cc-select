package config

import "testing"

// ModelPlanFromEnv 派生规则全表（specs/002 data-model.md 实体 1 / research D1）：
// 四槽派生、Entries 保序去重、keychain 占位跳过、全空 = 空 plan（不注入信号）。
func TestModelPlanFromEnv(t *testing.T) {
	t.Run("四槽齐全：Entries 保序 main→opus→sonnet→haiku", func(t *testing.T) {
		env := map[string]string{
			"ANTHROPIC_MODEL":                "glm-5.3[1m]",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":   "glm-5.2[1m]",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.3[1m]",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "glm-5.3-Flash",
			"ANTHROPIC_BASE_URL":             "https://example.com",
			"ANTHROPIC_AUTH_TOKEN":           "secret",
		}
		plan := ModelPlanFromEnv(env)
		if plan.Main != "glm-5.3[1m]" || plan.Opus != "glm-5.2[1m]" ||
			plan.Sonnet != "glm-5.3[1m]" || plan.Haiku != "glm-5.3-Flash" {
			t.Fatalf("槽位值错误: %+v", plan)
		}
		want := []ModelEntry{
			{ID: "glm-5.3[1m]", Slot: SlotMain},
			{ID: "glm-5.2[1m]", Slot: SlotOpus},
			{ID: "glm-5.3-Flash", Slot: SlotHaiku},
		}
		if len(plan.Entries) != len(want) {
			t.Fatalf("Entries 数量 = %d, want %d（main 与 sonnet 同值应去重）: %+v", len(plan.Entries), len(want), plan.Entries)
		}
		for i, e := range want {
			if plan.Entries[i] != e {
				t.Errorf("Entries[%d] = %+v, want %+v", i, plan.Entries[i], e)
			}
		}
	})

	t.Run("同 id 多槽去重：首槽位保留", func(t *testing.T) {
		env := map[string]string{
			"ANTHROPIC_MODEL":                "glm-5.3[1m]",
			"ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.3[1m]",
		}
		plan := ModelPlanFromEnv(env)
		if len(plan.Entries) != 1 || plan.Entries[0].Slot != SlotMain {
			t.Fatalf("去重后应只剩 main 槽一条: %+v", plan.Entries)
		}
	})

	t.Run("keychain 占位跳过（模型名不是敏感值，INV-1）", func(t *testing.T) {
		env := map[string]string{
			"ANTHROPIC_MODEL":               "$keychain:cc-select:glm:ANTHROPIC_MODEL",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-Flash",
		}
		plan := ModelPlanFromEnv(env)
		if plan.Main != "" {
			t.Fatalf("占位不应进入 Main: %q", plan.Main)
		}
		if len(plan.Entries) != 1 || plan.Entries[0].ID != "glm-5.3-Flash" {
			t.Fatalf("Entries 应只含 haiku 槽: %+v", plan.Entries)
		}
	})

	t.Run("部分槽位为空", func(t *testing.T) {
		env := map[string]string{
			"ANTHROPIC_MODEL":               "glm-5.3[1m]",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3-Flash",
		}
		plan := ModelPlanFromEnv(env)
		if plan.Opus != "" || plan.Sonnet != "" {
			t.Fatalf("空槽位应保持空: %+v", plan)
		}
		if len(plan.Entries) != 2 {
			t.Fatalf("Entries = %+v", plan.Entries)
		}
	})

	t.Run("四槽全空：空 plan（不注入信号）", func(t *testing.T) {
		plan := ModelPlanFromEnv(map[string]string{
			"ANTHROPIC_BASE_URL":   "https://example.com",
			"ANTHROPIC_AUTH_TOKEN": "secret",
		})
		if plan.Main != "" || plan.Opus != "" || plan.Sonnet != "" || plan.Haiku != "" {
			t.Fatalf("应全部为空: %+v", plan)
		}
		if len(plan.Entries) != 0 {
			t.Fatalf("Entries 应为空: %+v", plan.Entries)
		}
	})

	t.Run("nil env 不 panic", func(t *testing.T) {
		plan := ModelPlanFromEnv(nil)
		if len(plan.Entries) != 0 {
			t.Fatalf("nil env 应得空 plan: %+v", plan)
		}
	})
}
