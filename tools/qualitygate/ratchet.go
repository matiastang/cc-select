package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// knipFileIssue 是 knip JSON 输出的单文件条目（分类数组全部计入问题总数）。
type knipFileIssue struct {
	Binaries                 []json.RawMessage `json:"binaries"`
	Catalog                  []json.RawMessage `json:"catalog"`
	Dependencies             []json.RawMessage `json:"dependencies"`
	DevDependencies          []json.RawMessage `json:"devDependencies"`
	Duplicates               []json.RawMessage `json:"duplicates"`
	EnumMembers              []json.RawMessage `json:"enumMembers"`
	Exports                  []json.RawMessage `json:"exports"`
	Files                    []json.RawMessage `json:"files"`
	NamespaceMembers         []json.RawMessage `json:"namespaceMembers"`
	OptionalPeerDependencies []json.RawMessage `json:"optionalPeerDependencies"`
	Types                    []json.RawMessage `json:"types"`
	Unlisted                 []json.RawMessage `json:"unlisted"`
	Unresolved               []json.RawMessage `json:"unresolved"`
}

// metricCount 按 metric key 从工具 JSON 输出取当前计数值。
func metricCount(metric string, r io.Reader) (int, error) {
	switch {
	case strings.HasPrefix(metric, "jscpd-"):
		return parseJscpdClones(r)
	case metric == "knip-frontend":
		return countKnipIssues(r)
	default:
		return 0, fmt.Errorf("未知 metric %q（支持 jscpd-* 与 knip-frontend）", metric)
	}
}

// parseJscpdClones 从 jscpd JSON 报告读取克隆块数。
// 缺 statistics 视为错误而非 0：空报告按 0 处理会伪装成"棘轮改善"（假绿）。
func parseJscpdClones(r io.Reader) (int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, fmt.Errorf("解析 jscpd JSON: %w", err)
	}
	stRaw, ok := probe["statistics"]
	if !ok {
		return 0, fmt.Errorf("jscpd 报告缺 statistics 字段")
	}
	// stRaw 就是 statistics 对象本身，直接解到 total 层。
	var st struct {
		Total struct {
			Clones int `json:"clones"`
		} `json:"total"`
	}
	if err := json.Unmarshal(stRaw, &st); err != nil {
		return 0, fmt.Errorf("解析 jscpd statistics: %w", err)
	}
	return st.Total.Clones, nil
}

// countKnipIssues 统计 knip JSON 输出的问题总数（所有分类数组长度求和）。
// knip --reporter json 的结构是 {"issues": [<per-file issue>...]}。
func countKnipIssues(r io.Reader) (int, error) {
	var rep struct {
		Issues []knipFileIssue `json:"issues"`
	}
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return 0, fmt.Errorf("解析 knip JSON: %w", err)
	}
	n := 0
	for _, f := range rep.Issues {
		n += len(f.Binaries) + len(f.Catalog) + len(f.Dependencies) + len(f.DevDependencies) +
			len(f.Duplicates) + len(f.EnumMembers) + len(f.Exports) + len(f.Files) +
			len(f.NamespaceMembers) + len(f.OptionalPeerDependencies) + len(f.Types) +
			len(f.Unlisted) + len(f.Unresolved)
	}
	return n, nil
}

// compareCounter 是计数棘轮的纯比较内核（US3 棘轮语义）：
// 现值 > 基线 → 1（劣化）；现值 < 基线 → 0 + 收紧提示；相等 → 0。
func compareCounter(metric string, current, base int, stdout io.Writer) int {
	switch {
	case current > base:
		fmt.Fprintf(stdout, "ratchet %s: 劣化 %d > 基线 %d —— 棘轮只紧不松：消除新增重复/死代码，或在 PR 评审中说明并修正基线\n",
			metric, current, base)
		return 1
	case current < base:
		fmt.Fprintf(stdout, "ratchet %s: %d < 基线 %d ✓ 通过；基线可收紧到 %d（请随本改动更新 quality-baseline.json）\n",
			metric, current, base, current)
		return 0
	default:
		fmt.Fprintf(stdout, "ratchet %s: %d = 基线 %d ✓ 通过\n", metric, current, base)
		return 0
	}
}

// runRatchet 实现 `qualitygate ratchet`（contracts Contract 2）：
//
//	ratchet --baseline <path> --metric <jscpd-go|jscpd-frontend|knip-frontend> [--input <file>]
//
// stdin（或 --input）读对应工具的 JSON 输出，与基线 counters 比较（US3）。
// exit：0 通过 / 1 劣化 / 2 用法或配置错误。
func runRatchet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ratchet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "quality-baseline.json", "基线文件路径")
	metric := fs.String("metric", "", "计数项 key（基线 counters 组，如 jscpd-go）")
	input := fs.String("input", "", "工具 JSON 输出文件（缺省读 stdin）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *metric == "" {
		fmt.Fprintln(stderr, "ratchet: --metric 必填")
		return 2
	}

	baseline, err := LoadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "ratchet: %v\n", err)
		return 2
	}
	base, ok := baseline.Counters[*metric]
	if !ok {
		fmt.Fprintf(stderr, "ratchet: 基线无 counters.%s 条目（新工具接入须显式登记）\n", *metric)
		return 2
	}

	var reader io.Reader = os.Stdin
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintf(stderr, "ratchet: %v\n", err)
			return 2
		}
		defer f.Close()
		reader = f
	}

	current, err := metricCount(*metric, reader)
	if err != nil {
		fmt.Fprintf(stderr, "ratchet: %v\n", err)
		return 2
	}
	return compareCounter(*metric, current, base.Count, stdout)
}
