package profile

import (
	"encoding/json"
	"fmt"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
)

// modelpicker.go 把 ModelPlan 注入 profile settings.json（specs/002 contracts/profile-settings.md）。
//
// 注入是 profile 构建的收尾步（Mode A/B/P 三个写路径共用）：
//
//   - modelPicker{replaceBuiltInOptions:true, options:[{model:id}...]}——行只含 model，
//     无 label/description（CC 对不认识的名字回退显示原始 id，research D7）；
//   - model 字段仅 Mode P 注入（withModel=true）：Mode P 的 profile env 无 ANTHROPIC_MODEL，
//     CC 启动时解析 model 设置定位当前模型——不注入则 ✔/横幅停在全局继承的目录别名上（research D8）；
//   - availableModels 白名单存在时追加缺失 id（否则行被 CC Dropped，research D5）；
//   - 空 plan（四槽全空）→ 原样返回（INV-5：官方 provider / 无模型变量 provider 零注入）。
//
// 字段级合并（INV-4）：除 modelPicker 与（Mode P 的）model 外，既有字段一律保留。
// 本文件是纯函数（无 IO）；文件读写与原子写在调用方（EnsureRaw / RefreshPicker）。
//
// 与代理改写的同源约束：本注入与 internal/router 的改写判定共用 config.ModelPlanFromEnv
// 的派生结果——「能选的」永远等于「能生效的」（002 data-model 实体 1）。

// buildPickerBlock 构造 modelPicker 配置块（US3 刷新路径复用同一原语，防两处漂移）。
func buildPickerBlock(plan config.ModelPlan) map[string]any {
	options := make([]map[string]string, 0, len(plan.Entries))
	for _, e := range plan.Entries {
		options = append(options, map[string]string{"model": e.ID})
	}
	return map[string]any{
		"replaceBuiltInOptions": true,
		"options":               options,
	}
}

// injectModelPicker 把 plan 注入已合并的 settings.json 字节流。
// withModel 仅 Mode P 为 true（注入 model=<主模型>）；空 plan 时原样返回。
func injectModelPicker(data []byte, plan config.ModelPlan, withModel bool) ([]byte, error) {
	if len(plan.Entries) == 0 {
		return data, nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf(i18n.T("profile.parseMergedSettings"), err)
	}

	m["modelPicker"] = buildPickerBlock(plan)
	if withModel && plan.Main != "" {
		m["model"] = plan.Main
	}

	// availableModels 白名单追加（幂等）：白名单外的行会被 CC Dropped（research D5）。
	if am, ok := m["availableModels"].([]any); ok {
		seen := map[string]bool{}
		for _, v := range am {
			if s, ok := v.(string); ok {
				seen[s] = true
			}
		}
		for _, e := range plan.Entries {
			if !seen[e.ID] {
				am = append(am, e.ID)
				seen[e.ID] = true
			}
		}
		m["availableModels"] = am
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf(i18n.T("profile.serializeMergedSettings"), err)
	}
	return out, nil
}
