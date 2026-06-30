package profile

import (
	"encoding/json"
	"fmt"
)

// mergeSettings 把全局 ~/.claude/settings.json 与 provider env 合并为 profile settings.json
// （Mode B 用）。规则（已定：整体替换 env 块）：
//
//   - 解析 globalJSON 为 map[string]any，保留所有未知字段（permissions/hooks/model/theme…一个不丢）；
//   - 把 env 键的值整体替换为 env（非深合并，全局原有 env 丢弃）；
//   - 序列化返回（键按字母序，可接受的外观变化）。
//
// globalJSON 为空时，等价于 {"env": env}。解析失败返回错误（调用方决定是否降级）。
func mergeSettings(globalJSON []byte, env map[string]string) ([]byte, error) {
	m := map[string]any{}
	if len(globalJSON) > 0 {
		if err := json.Unmarshal(globalJSON, &m); err != nil {
			return nil, fmt.Errorf("解析全局 settings.json: %w", err)
		}
	}
	// 整体替换 env 块。
	m["env"] = env
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化合并 settings: %w", err)
	}
	return out, nil
}
