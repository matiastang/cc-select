// Package presets 的远端源解析（specs/004 US5 / Q19）。
//
// 状态机（contracts/presets-and-usage §1.2）：
//
//	disabled → builtin
//	进程内已解析 → 记忆结果
//	缓存新鲜(<24h) → cache
//	拉取远端(5s,锁定 URL,拒跨 host 重定向) → 校验 → 写缓存 → remote
//	失败 → 缓存(任意年龄) → builtin
package presets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cc-select/cc-select/internal/prefs"
)

// Source 是当前预设来源。
type Source string

const (
	SourceBuiltin Source = "builtin"
	SourceCache   Source = "cache"
	SourceRemote  Source = "remote"
)

// RemotePresetsURL 是远端预设唯一分发地址——硬编码锁定本仓库（FR-014 安全约束：
// 预设决定 token 去向，地址不可配置），与 embed 的 presets.json 同一份文件。
const RemotePresetsURL = "https://raw.githubusercontent.com/matiastang/cc-select/main/internal/presets/presets.json"

const (
	remoteFetchTimeout = 5 * time.Second
	cacheTTL           = 24 * time.Hour
	maxRemoteBody      = 1 << 20 // 1MB：预设清单远小于此，防异常大响应
)

// SourceMeta 描述当前生效的预设来源（API meta 字段的真值结构）。
type SourceMeta struct {
	Source    Source     `json:"source"`
	FetchedAt *time.Time `json:"fetchedAt"`
}

// ResolveOptions 注入解析依赖（测试替身入口）。
type ResolveOptions struct {
	Disabled  bool
	RemoteURL string       // 生产恒为 RemotePresetsURL；测试注入 httptest
	CachePath string       // 生产由 CacheFilePath() 计算
	Client    *http.Client // nil → 默认客户端（带超时与重定向守卫）
	Now       func() time.Time
}

// cacheBundle 是 presets.cache.json 的落盘结构。
type cacheBundle struct {
	SchemaVersion int      `json:"schemaVersion"`
	FetchedAt     string   `json:"fetchedAt"`
	Presets       []Preset `json:"presets"`
}

var resolveState struct {
	mu     sync.Mutex
	done   bool
	result SourceMeta
	forced bool // 用户显式关闭远端后为 true：在飞解析收尾一律按 builtin 落地
}

// CacheFilePath 返回缓存文件路径（~/.cc-select/presets.cache.json，与 providers.json 同目录）。
func CacheFilePath() (string, error) {
	if p := os.Getenv("CC_SELECT_CONFIG"); p != "" {
		return filepath.Join(filepath.Dir(p), "presets.cache.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-select", "presets.cache.json"), nil
}

func defaultResolveOptions(opts *ResolveOptions) *ResolveOptions {
	o := *opts
	if o.RemoteURL == "" {
		o.RemoteURL = RemotePresetsURL
	}
	if o.CachePath == "" {
		if p, err := CacheFilePath(); err == nil {
			o.CachePath = p
		}
	}
	// 安全约束（FR-014）结构化强制：无论注入与否，重定向守卫必须生效——
	// 复制客户端并覆写 CheckRedirect，避免测试/生产注入路径绕过守卫。
	client := o.Client
	if client == nil {
		client = &http.Client{}
	}
	safe := *client
	if safe.Timeout <= 0 || safe.Timeout > remoteFetchTimeout {
		safe.Timeout = remoteFetchTimeout
	}
	safe.CheckRedirect = rejectCrossHostRedirect
	o.Client = &safe
	if o.Now == nil {
		o.Now = time.Now
	}
	return &o
}

// rejectCrossHostRedirect 拒绝跨 host 重定向（预设决定 token 去向，宪法 VI）。
func rejectCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	first, err1 := url.Parse(via[0].URL.String())
	next, err2 := url.Parse(req.URL.String())
	if err1 != nil || err2 != nil {
		return errors.New("presets: unparseable redirect")
	}
	if first.Host != next.Host {
		return fmt.Errorf("presets: cross-host redirect to %s rejected", next.Host)
	}
	return nil
}

// ResetForTest 恢复内置预设并清空进程内解析状态。
// 仅供跨包测试（internal/web）隔离远端热替换对全局 store 的污染。
func ResetForTest() {
	var b Bundle
	_ = json.Unmarshal(embeddedPresets, &b)
	reloadStore(b)
	resolveState.mu.Lock()
	resolveState.done = false
	resolveState.result = SourceMeta{}
	resolveState.forced = false
	resolveState.mu.Unlock()
}

// SetRemoteDisabled 在用户切换远端预设开关后即时生效（contracts/api.md §2.2）：
// 关闭 → 立即回落内置并定格 meta；开启 → 清除记忆态，下次 EnsureResolved/Refresh
// 按缓存 TTL 重新解析（未过期缓存直接复用）。
func SetRemoteDisabled(disabled bool) {
	if disabled {
		reloadBuiltin()
		resolveState.mu.Lock()
		resolveState.forced = true
		resolveState.done = true
		resolveState.result = SourceMeta{Source: SourceBuiltin}
		resolveState.mu.Unlock()
	} else {
		resolveState.mu.Lock()
		resolveState.forced = false
		resolveState.done = false
		resolveState.result = SourceMeta{}
		resolveState.mu.Unlock()
	}
}

// Meta 返回当前生效的来源（未解析过 = builtin）。API meta 字段真值。
func Meta() SourceMeta {
	resolveState.mu.Lock()
	defer resolveState.mu.Unlock()
	if !resolveState.done {
		return SourceMeta{Source: SourceBuiltin}
	}
	return resolveState.result
}

// DefaultOptions 构造生产解析选项：Disabled 读用户偏好（远端预设开关）。
func DefaultOptions() *ResolveOptions {
	disabled := false
	if p, err := prefs.Load(); err == nil {
		disabled = p.RemotePresetsDisabled
	}
	return &ResolveOptions{Disabled: disabled}
}

// EnsureResolved 执行一次（进程内）源解析；后续调用直接返回记忆结果。
// CLI 与 Web 各自进程首调用生效（FR-015 共享同一逻辑）。
func EnsureResolved(ctx context.Context, opts *ResolveOptions) SourceMeta {
	o := defaultResolveOptions(opts)

	resolveState.mu.Lock()
	if resolveState.done {
		meta := resolveState.result
		resolveState.mu.Unlock()
		return meta
	}
	resolveState.mu.Unlock()

	meta := resolve(ctx, o)
	resolveState.mu.Lock()
	if resolveState.forced {
		// 解析在飞期间用户关闭了远端（NEW-3 竞态）：丢弃结果并回滚 store——
		// disable 可能落在 resolve 内部 reloadStore(remote) 之后，必须恢复 builtin。
		meta = SourceMeta{Source: SourceBuiltin}
	}
	resolveState.done = true
	resolveState.result = meta
	forced := resolveState.forced
	resolveState.mu.Unlock()
	if forced {
		reloadBuiltin()
	}
	return meta
}

// Refresh 忽略 TTL 强制拉取远端（用户手动触发）。成功 → 热替换 + 写缓存；
// 失败/禁用 → 不动当前 store，返回当前生效 meta + warning（fetch_failed|invalid_schema|disabled）。
func Refresh(ctx context.Context, opts *ResolveOptions) (SourceMeta, string) {
	o := defaultResolveOptions(opts)

	if func() bool {
		resolveState.mu.Lock()
		defer resolveState.mu.Unlock()
		return resolveState.forced
	}() || o.Disabled {
		reloadBuiltin()
		return SourceMeta{Source: SourceBuiltin}, "disabled"
	}

	data, err := fetchRemote(ctx, o)
	if err != nil {
		return currentMeta(o, SourceBuiltin), "fetch_failed"
	}
	if err := Validate(data); err != nil {
		return currentMeta(o, SourceBuiltin), "invalid_schema"
	}

	now := o.Now().UTC()
	b := bundleFrom(data, now)

	// 提交前锁内复查 forced（NEW-3a）：刷新期间用户可能已关闭远端——
	// 此时不得 reloadStore(remote)/写 meta，回落 builtin 并报 disabled。
	resolveState.mu.Lock()
	if resolveState.forced {
		resolveState.mu.Unlock()
		reloadBuiltin()
		return SourceMeta{Source: SourceBuiltin}, "disabled"
	}
	reloadStore(b.Bundle)
	meta := SourceMeta{Source: SourceRemote, FetchedAt: &now}
	resolveState.done = true
	resolveState.result = meta
	resolveState.mu.Unlock()

	writeCache(o, b)
	return meta, ""
}

// resolve 实现状态机主体（无记忆语义）。
// meta 语义（contracts/api.md §1）：remote=本次拉取或新鲜缓存(<24h)；cache=已过期
// 但仍采用的缓存；builtin=embed 兜底。凡采用某 bundle 都必须 reloadStore 生效。
func resolve(ctx context.Context, o *ResolveOptions) SourceMeta {
	if o.Disabled {
		reloadBuiltin()
		return SourceMeta{Source: SourceBuiltin}
	}
	if cb, ok := readCache(o); ok && o.Now().Sub(cb.fetchedAt) < cacheTTL {
		reloadStore(Bundle{SchemaVersion: SchemaVersion, Presets: cb.bundle.Presets})
		return SourceMeta{Source: SourceRemote, FetchedAt: &cb.fetchedAt}
	}

	data, err := fetchRemote(ctx, o)
	if err == nil {
		if verr := Validate(data); verr == nil {
			now := o.Now().UTC()
			b := bundleFrom(data, now)
			reloadStore(b.Bundle)
			writeCache(o, b)
			return SourceMeta{Source: SourceRemote, FetchedAt: &now}
		}
	}
	// 拉取/校验失败：缓存（任意年龄）兜底，再不济 builtin。
	if cb, ok := readCache(o); ok {
		reloadStore(Bundle{SchemaVersion: SchemaVersion, Presets: cb.bundle.Presets})
		return SourceMeta{Source: SourceCache, FetchedAt: &cb.fetchedAt}
	}
	return SourceMeta{Source: SourceBuiltin}
}

func reloadBuiltin() {
	var b Bundle
	_ = json.Unmarshal(embeddedPresets, &b)
	reloadStore(b)
}

func fetchRemote(ctx context.Context, o *ResolveOptions) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.RemoteURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("presets: remote status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteBody))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// readCache 读取并校验缓存；损坏/缺失返回 ok=false。
func readCache(o *ResolveOptions) (struct {
	bundle    cacheBundle
	fetchedAt time.Time
}, bool) {
	var out struct {
		bundle    cacheBundle
		fetchedAt time.Time
	}
	data, err := os.ReadFile(o.CachePath)
	if err != nil {
		return out, false
	}
	var cb cacheBundle
	if err := json.Unmarshal(data, &cb); err != nil {
		return out, false
	}
	if cb.SchemaVersion != SchemaVersion || len(cb.Presets) == 0 {
		return out, false
	}
	raw, err := json.Marshal(Bundle{SchemaVersion: SchemaVersion, Presets: cb.Presets})
	if err != nil || Validate(raw) != nil {
		return out, false
	}
	at, err := time.Parse(time.RFC3339, cb.FetchedAt)
	if err != nil {
		return out, false
	}
	out.bundle = cb
	out.fetchedAt = at
	return out, true
}

type loadedBundle struct {
	Bundle
	FetchedAt time.Time
}

func bundleFrom(data []byte, at time.Time) loadedBundle {
	var b Bundle
	_ = json.Unmarshal(data, &b)
	return loadedBundle{Bundle: b, FetchedAt: at}
}

func writeCache(o *ResolveOptions, b loadedBundle) {
	if o.CachePath == "" {
		return
	}
	cb := cacheBundle{
		SchemaVersion: SchemaVersion,
		FetchedAt:     b.FetchedAt.Format(time.RFC3339),
		Presets:       b.Presets,
	}
	data, err := json.MarshalIndent(cb, "", "  ")
	if err != nil {
		return
	}
	// 原子写 0600（宪法 VI）：temp + rename，与 providers/prefs 同语义。
	tmp := o.CachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, o.CachePath)
}

// currentMeta 返回「当前生效源」（契约 §2.1：Refresh 失败也如实报告）：
// 已解析 → 记忆结果；未解析 → store 即 builtin。
func currentMeta(*ResolveOptions, Source) SourceMeta {
	resolveState.mu.Lock()
	defer resolveState.mu.Unlock()
	if resolveState.done {
		return resolveState.result
	}
	return SourceMeta{Source: SourceBuiltin}
}
