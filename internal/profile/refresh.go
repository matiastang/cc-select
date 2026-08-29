package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
)

// refresh.go 是热切后的选择器刷新原语（specs/002 US3 / research D3）。
//
// route switch 成功后，经继承的 $CLAUDE_CONFIG_DIR 定位**发射 profile** 的
// settings.json（热切后它仍指向发射目录——写 profiles/<新 provider>/ 运行中
// 会话毫无感知），把 modelPicker/model 改写为新 provider 的清单。
//
// 语义（与构建期注入互补）：
//   - 非空 plan → modelPicker = 新清单；model = 新主模型；availableModels 幂等追加；
//   - 空 plan（目标 provider 无模型变量）→ 移除注入键，显示回落内置目录；
//   - 其余字段一律字段级保留（CC 的 /model 选择、permissions、env 都不触碰）；
//   - 原子写（同目录 temp + rename，0600）。
//
// 调用方约定（internal/cli/route.go）：本函数返回错误时路由切换**照常成功**——
// 路由表是 Mode P 真值，选择器显示是增强。

// RefreshPicker 把 settingsPath 的 modelPicker/model 刷新为 plan 的状态。
// settingsPath 通常 = $CLAUDE_CONFIG_DIR/settings.json。
//
// 并发安全（SC-005）：CC 自身也是本文件的写者（/model 选择、permissions 等
// 落盘，research D6）。采用读-改-写 + 写前字节比对：基线在窗口内被并发修改
// 则整轮重读重合并（至多 refreshMaxAttempts 轮），耗尽仍冲突则返回错误且
// **不写**——绝不落过期快照（0 字段丢失 / 0 覆盖丢失）。
func RefreshPicker(settingsPath string, plan config.ModelPlan) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf(i18n.T("profile.refreshRead"), err)
	}
	for attempt := 1; ; attempt++ {
		stale, err := refreshAttempt(settingsPath, data, plan)
		if err != nil {
			return err
		}
		if !stale {
			return nil
		}
		if attempt >= refreshMaxAttempts {
			return fmt.Errorf(i18n.T("profile.refreshConflict"), refreshMaxAttempts, settingsPath)
		}
		data, err = os.ReadFile(settingsPath)
		if err != nil {
			return fmt.Errorf(i18n.T("profile.refreshRead"), err)
		}
	}
}

const refreshMaxAttempts = 3

// refreshBeforeWriteHook 是并发写者模拟点（SC-005 测试注入），生产恒为 nil。
var refreshBeforeWriteHook func()

// refreshAttempt 以 data 为基线做字段级合并并原子写回；返回 stale=true 表示
// 写前检测到基线已被并发修改（本轮未写）。
func refreshAttempt(settingsPath string, data []byte, plan config.ModelPlan) (bool, error) {
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf(i18n.T("profile.refreshParse"), err)
	}

	if len(plan.Entries) == 0 {
		// 目标 provider 无模型变量 → 清除注入态（显示回落内置目录）。
		delete(m, "modelPicker")
		delete(m, "model")
	} else {
		m["modelPicker"] = buildPickerBlock(plan)
		if plan.Main != "" {
			m["model"] = plan.Main
		} else {
			// 无主模型时残留的 model 是上一个 provider 的 id（评审 finding 4）。
			delete(m, "model")
		}
		appendAvailableModels(m, plan)
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return false, fmt.Errorf(i18n.T("profile.refreshSerialize"), err)
	}
	if refreshBeforeWriteHook != nil {
		refreshBeforeWriteHook()
	}
	cur, err := os.ReadFile(settingsPath)
	if err != nil {
		return false, fmt.Errorf(i18n.T("profile.refreshRead"), err)
	}
	if !bytes.Equal(cur, data) {
		return true, nil
	}
	return false, writeFileAtomic(settingsPath, out)
}

// appendAvailableModels 把 plan 中尚未在白名单的 id 追加进 availableModels
// （白名单外的 modelPicker 行会被 CC Dropped——research D5）。键不存在则不动。
// 注入与刷新共用，保证两处行为一致。
func appendAvailableModels(m map[string]any, plan config.ModelPlan) {
	am, ok := m["availableModels"].([]any)
	if !ok {
		return
	}
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

// writeFileAtomic 同目录临时文件 + rename 原子写入（0600）。
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cc-select-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后移除为空操作
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
