// qualitygate：代码质量门禁工具（specs/005-quality-gates，需求 R12）。
//
// 职责（contracts/quality-gates.md Contract 2）：
//   - filelines：Go 产品代码文件行数检查
//   - filter：   linter 输出减去基线（存量违规豁免，棘轮载体）
//   - ratchet：  jscpd/knip 计数棘轮
//   - coverage： Go 覆盖率下限
//
// 本文件实现基线的加载与匹配（data-model.md Entity 1）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// BaselineVersion 是当前基线文件格式版本；格式变更必须升版本并同步迁移逻辑。
const BaselineVersion = 1

// Finding 是一条位置型违规：基线条目与工具输出共用同一结构。
// 基线条目的 Message 语义是"前缀"（容忍实测数值变化，如复杂度从 26 降到 24）。
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message,omitempty"`
}

// Counter 是一条计数型棘轮（jscpd 克隆数、knip 问题数）。
type Counter struct {
	Count  int    `json:"count"`
	Metric string `json:"metric,omitempty"`
}

// FileLinesConfig 承载文件行数阈值（FR-003，Go 产品代码）。
type FileLinesConfig struct {
	GoMax int `json:"go-max"`
}

// Baseline 是 quality-baseline.json 的内存表示：存量违规显式豁免 +
// 计数棘轮 + 覆盖率阈值 + 文件行数阈值的单一真值（FR-002/009/010）。
type Baseline struct {
	Version   int                  `json:"version"`
	Comment   string               `json:"_comment"`
	Findings  map[string][]Finding `json:"findings"`
	Counters  map[string]Counter   `json:"counters"`
	Coverage  map[string]float64   `json:"coverage"`
	FileLines FileLinesConfig      `json:"fileLines"`
}

// LoadBaseline 读取并校验基线文件。任何结构问题（版本不符、字段非法）都返回错误——
// 基线损坏时必须让门禁显式失败，而不是静默放行（spec Edge Cases「假绿」禁令）。
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取基线: %w", err)
	}

	var b Baseline
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("解析基线 %s: %w", path, err)
	}

	if b.Version != BaselineVersion {
		return nil, fmt.Errorf("基线版本 %d 不受支持（want %d）", b.Version, BaselineVersion)
	}

	for tool, findings := range b.Findings {
		for i, f := range findings {
			if f.File == "" {
				return nil, fmt.Errorf("findings.%s[%d].file 为空", tool, i)
			}
			if f.Rule == "" {
				return nil, fmt.Errorf("findings.%s[%d].rule 为空", tool, i)
			}
			if f.Line < 0 {
				return nil, fmt.Errorf("findings.%s[%d].line 为负", tool, i)
			}
		}
	}

	for key, c := range b.Counters {
		if c.Count < 0 {
			return nil, fmt.Errorf("counters.%s.count 为负", key)
		}
	}

	for key, v := range b.Coverage {
		if v < 0 || v > 100 {
			return nil, fmt.Errorf("coverage.%s = %v 越界（0-100）", key, v)
		}
	}

	return &b, nil
}

// matches 判断工具输出的候选违规是否命中该基线条目。
// 匹配语义（contracts Contract 3）：file/line/rule 精确（基线 line=0 表示整文件级，
// 匹配任意行）；message 非空时按前缀匹配（容忍行文中的实测数值变化）。
func (f Finding) matches(c Finding) bool {
	if f.File != c.File || f.Rule != c.Rule {
		return false
	}
	if f.Line != 0 && f.Line != c.Line {
		return false
	}
	if f.Message != "" && !strings.HasPrefix(c.Message, f.Message) {
		return false
	}
	return true
}

// HasFinding 判断某工具的候选违规是否被基线豁免（存量违规，US3）。
// 未知 tool 组（无条目）一律不命中——新工具接入必须显式登记基线组。
func (b *Baseline) HasFinding(tool string, c Finding) bool {
	for _, f := range b.Findings[tool] {
		if f.matches(c) {
			return true
		}
	}
	return false
}
