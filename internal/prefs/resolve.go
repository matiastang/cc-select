package prefs

// ResolveMode 按三级优先级解析最终生效的隔离模式（高 → 低）：
//
//  1. oneOff    —— 一次性覆盖（如 cc-select use --mode，仅本次，不落盘）
//  2. provider  —— 该 provider 的 per-provider 覆盖（providers.json 的 Provider.IsolationMode）
//  3. global    —— 全局默认（prefs.json 的 IsolationMode）
//  4. 都未设置  —— DefaultMode（ModeSettingsOnly）
//
// 空串表示「未指定」，逐级回退。调用方负责把各来源的值传入；本函数是纯逻辑、易测。
func ResolveMode(oneOff, provider, global Mode) Mode {
	switch {
	case oneOff != "":
		return oneOff
	case provider != "":
		return provider
	case global != "":
		return global
	default:
		return DefaultMode
	}
}
